package collector

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/google/pprof/profile"
	"k8s.io/klog"
)

func (c *Collector) Profiles(w http.ResponseWriter, r *http.Request) {
	projectId := db.ProjectId(r.Header.Get(ApiKeyHeader))
	_, err := c.getClickhouseClient(projectId)
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

	err = c.saveProfile(r.Context(), projectId, serviceName, labels, p)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (c *Collector) saveProfile(ctx context.Context, projectId db.ProjectId, serviceName string, labels model.Labels, p *profile.Profile) error {
	colServiceName := new(chproto.ColStr).LowCardinality()
	colType := new(chproto.ColStr).LowCardinality()
	colValue := new(chproto.ColInt64)
	colStackHash := new(chproto.ColUInt64)
	colStart := new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano)
	colEnd := new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano)
	colLabels := chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr))
	colStack := new(chproto.ColStr).Array()

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
			colServiceName.Append(serviceName)
			colType.Append(st.Type)
			colStart.Append(start)
			colEnd.Append(end)
			colLabels.Append(labels)
			colValue.Append(s.Value[i])
			colStackHash.Append(StackHash(stack))
			colStack.Append(stack)
		}
	}

	input := chproto.Input{
		{Name: "ServiceName", Data: colServiceName},
		{Name: "Hash", Data: colStackHash},
		{Name: "LastSeen", Data: colEnd},
		{Name: "Stack", Data: colStack},
	}
	err := c.clickhouseDo(ctx, projectId, ch.Query{Body: input.Into("profiling_stacks"), Input: input})
	if err != nil {
		return err
	}
	input = chproto.Input{
		{Name: "ServiceName", Data: colServiceName},
		{Name: "Type", Data: colType},
		{Name: "Start", Data: colStart},
		{Name: "End", Data: colEnd},
		{Name: "Labels", Data: colLabels},
		{Name: "StackHash", Data: colStackHash},
		{Name: "Value", Data: colValue},
	}
	err = c.clickhouseDo(ctx, projectId, ch.Query{Body: input.Into("profiling_samples"), Input: input})
	if err != nil {
		return err
	}
	return nil
}

func StackHash(s []string) uint64 {
	h := fnv.New64a()
	for _, l := range s {
		_, _ = h.Write([]byte(l))
	}
	return h.Sum64()
}
