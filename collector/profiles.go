package collector

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"sync"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/coroot/coroot/model"
	"github.com/google/pprof/profile"
	"k8s.io/klog"
)

func (c *Collector) Profiles(w http.ResponseWriter, r *http.Request) {
	project, err := c.getProject(r.Header.Get(ApiKeyHeader))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	query := r.URL.Query()
	var serviceName string
	labels := model.Labels{}
	for k, vs := range query {
		if len(vs) != 1 {
			continue
		}
		v := vs[0]
		if k == "service.name" {
			serviceName = v
			continue
		}
		labels[k] = v
	}
	if serviceName == "" {
		klog.Errorln("service.name is empty")
		http.Error(w, "service.name is empty", http.StatusBadRequest)
		return
	}
	p, err := profile.Parse(r.Body)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c.getProfilesBatch(project).Add(serviceName, labels, p)
}

type ProfilesBatch struct {
	limit int
	exec  func(query ch.Query) error

	lock sync.Mutex
	done chan struct{}

	ServiceName *chproto.ColLowCardinality[string]
	Type        *chproto.ColLowCardinality[string]
	Start       *chproto.ColDateTime64
	End         *chproto.ColDateTime64
	Labels      *chproto.ColMap[string, string]
	Value       *chproto.ColInt64
	StackHash   *chproto.ColUInt64
	Stack       *chproto.ColArr[string]
}

func NewProfilesBatch(limit int, timeout time.Duration, exec func(query ch.Query) error) *ProfilesBatch {
	b := &ProfilesBatch{
		limit: limit,
		exec:  exec,
		done:  make(chan struct{}),

		ServiceName: new(chproto.ColStr).LowCardinality(),
		Type:        new(chproto.ColStr).LowCardinality(),
		Start:       new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano),
		End:         new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano),
		Labels:      chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr)),
		Value:       new(chproto.ColInt64),
		StackHash:   new(chproto.ColUInt64),
		Stack:       new(chproto.ColStr).Array(),
	}

	go func() {
		ticker := time.NewTicker(timeout)
		defer ticker.Stop()
		for {
			select {
			case <-b.done:
				return
			case <-ticker.C:
				b.lock.Lock()
				b.save()
				b.lock.Unlock()
			}
		}
	}()

	return b
}

func (b *ProfilesBatch) Close() {
	b.done <- struct{}{}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.save()
}

func (b *ProfilesBatch) Add(serviceName string, labels model.Labels, p *profile.Profile) {
	b.lock.Lock()
	defer b.lock.Unlock()

	end := time.Unix(0, p.TimeNanos)
	start := end.Add(-time.Duration(p.DurationNanos))

	for i, st := range p.SampleType {
		if st.Type == "" {
			continue
		}
		for _, s := range p.Sample {
			var stack []string
			for _, location := range s.Location {
				for _, line := range location.Line {
					if line.Function == nil {
						continue
					}
					l := fmt.Sprintf("%s %s:%d", line.Function.Name, line.Function.Filename, line.Line)
					stack = append(stack, l)
				}
			}
			stackHash := StackHash(stack)

			b.ServiceName.Append(serviceName)
			b.Type.Append(st.Type)
			b.Start.Append(start)
			b.End.Append(end)
			b.Labels.Append(labels)
			b.Value.Append(s.Value[i])
			b.StackHash.Append(stackHash)
			b.Stack.Append(stack)
		}
	}

	if b.ServiceName.Rows() < b.limit {
		return
	}
	b.save()
}

func (b *ProfilesBatch) save() {
	if b.ServiceName.Rows() == 0 {
		return
	}

	stacksInput := chproto.Input{
		chproto.InputColumn{Name: "ServiceName", Data: b.ServiceName},
		chproto.InputColumn{Name: "Hash", Data: b.StackHash},
		chproto.InputColumn{Name: "LastSeen", Data: b.End},
		chproto.InputColumn{Name: "Stack", Data: b.Stack},
	}
	err := b.exec(ch.Query{Body: stacksInput.Into("@@table_profiling_stacks@@"), Input: stacksInput})
	if err != nil {
		klog.Errorln(err)
	}

	samplesInput := chproto.Input{
		chproto.InputColumn{Name: "ServiceName", Data: b.ServiceName},
		chproto.InputColumn{Name: "Type", Data: b.Type},
		chproto.InputColumn{Name: "Start", Data: b.Start},
		chproto.InputColumn{Name: "End", Data: b.End},
		chproto.InputColumn{Name: "Labels", Data: b.Labels},
		chproto.InputColumn{Name: "StackHash", Data: b.StackHash},
		chproto.InputColumn{Name: "Value", Data: b.Value},
	}
	err = b.exec(ch.Query{Body: samplesInput.Into("@@table_profiling_samples@@"), Input: samplesInput})
	if err != nil {
		klog.Errorln(err)
	}

	for _, i := range stacksInput {
		i.Data.(chproto.Resettable).Reset()
	}
	for _, i := range samplesInput {
		i.Data.(chproto.Resettable).Reset()
	}
}

func StackHash(s []string) uint64 {
	h := fnv.New64a()
	for _, l := range s {
		_, _ = h.Write([]byte(l))
	}
	return h.Sum64()
}
