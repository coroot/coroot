package collector

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
	"k8s.io/klog"
)

type rumReplayPayload struct {
	SessionId   string `json:"session_id"`
	TraceId     string `json:"trace_id"`
	ServiceName string `json:"service_name"`
	Seq         uint32 `json:"seq"`
	Consent     bool   `json:"consent"`
	Payload     string `json:"payload"`
}

func (c *Collector) RumReplay(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		origin := r.Header.Get("Origin")
		key := c.findRumKeyByOrigin(origin)
		if key == nil {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		writeCORSHeaders(w, origin, false)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	project, key, err := c.getProjectAndKey(r.Header.Get(ApiKeyHeader))
	if err != nil || key == nil || !key.IsRum() {
		http.Error(w, "rum api key required", http.StatusForbidden)
		return
	}
	reqPath := ""
	if ref := r.Header.Get("Referer"); ref != "" {
		reqPath = pathFromURL(ref)
	}
	origin, ok := allowRumRequest(r, key, reqPath)
	if !ok {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	writeCORSHeaders(w, origin, false)

	if project.Settings.Rum == nil || !project.Settings.Rum.ReplayEnabled {
		http.Error(w, "session replay is disabled", http.StatusForbidden)
		return
	}

	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRumBodyBytes))
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	var p rumReplayPayload
	if err = json.Unmarshal(data, &p); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !p.Consent || p.SessionId == "" || p.Payload == "" {
		http.Error(w, "consent, session_id and payload required", http.StatusBadRequest)
		return
	}
	if p.ServiceName == "" {
		p.ServiceName = "web-app"
	}

	c.getRumReplayBatch(project).Add(p)
	w.WriteHeader(http.StatusAccepted)
}

type RumReplayBatch struct {
	limit int
	exec  func(query ch.Query) error

	lock sync.Mutex
	done chan struct{}

	Timestamp   *chproto.ColDateTime64
	ServiceName *chproto.ColLowCardinality[string]
	SessionId   *chproto.ColLowCardinality[string]
	TraceId     *chproto.ColStr
	Seq         *chproto.ColUInt32
	Payload     *chproto.ColStr
}

func NewRumReplayBatch(limit int, timeout time.Duration, exec func(query ch.Query) error) *RumReplayBatch {
	b := &RumReplayBatch{
		limit:       limit,
		exec:        exec,
		done:        make(chan struct{}),
		Timestamp:   new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano),
		ServiceName: new(chproto.ColStr).LowCardinality(),
		SessionId:   new(chproto.ColStr).LowCardinality(),
		TraceId:     new(chproto.ColStr),
		Seq:         new(chproto.ColUInt32),
		Payload:     new(chproto.ColStr),
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

func (b *RumReplayBatch) Close() {
	b.done <- struct{}{}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.save()
}

func (b *RumReplayBatch) Add(p rumReplayPayload) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.Timestamp.Append(time.Now())
	b.ServiceName.Append(p.ServiceName)
	b.SessionId.Append(p.SessionId)
	b.TraceId.Append(p.TraceId)
	b.Seq.Append(p.Seq)
	b.Payload.Append(p.Payload)
	if b.Timestamp.Rows() >= b.limit {
		b.save()
	}
}

func (b *RumReplayBatch) save() {
	if b.Timestamp.Rows() == 0 {
		return
	}
	input := chproto.Input{
		{Name: "Timestamp", Data: b.Timestamp},
		{Name: "ServiceName", Data: b.ServiceName},
		{Name: "SessionId", Data: b.SessionId},
		{Name: "TraceId", Data: b.TraceId},
		{Name: "Seq", Data: b.Seq},
		{Name: "Payload", Data: b.Payload},
	}
	err := b.exec(ch.Query{Body: input.Into("@@table_rum_replay_segments@@"), Input: input})
	if err != nil {
		klog.Errorln("rum replay insert:", err)
	}
	b.Timestamp.Reset()
	b.ServiceName.Reset()
	b.SessionId.Reset()
	b.TraceId.Reset()
	b.Seq.Reset()
	b.Payload.Reset()
}
