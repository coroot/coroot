package collector

import (
	"encoding/hex"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	v1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/protobuf/proto"
	"k8s.io/klog"
)

func (c *Collector) Logs(w http.ResponseWriter, r *http.Request) {
	project, err := c.getProject(r.Header.Get(ApiKeyHeader))
	if err != nil {
		klog.Errorln(err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	contentType := r.Header.Get("Content-Type")
	switch contentType {
	case "application/x-protobuf":
	default:
		http.Error(w, "unsupported content type: "+contentType, http.StatusBadRequest)
		return
	}

	decoder, err := getDecoder(r.Header.Get("Content-Encoding"), r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := io.ReadAll(decoder)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	req := &v1.ExportLogsServiceRequest{}
	err = proto.Unmarshal(data, req)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	c.getLogsBatch(project).Add(req)

	resp := &v1.ExportLogsServiceResponse{}
	w.Header().Set("Content-Type", contentType)
	data, err = proto.Marshal(resp)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}

type LogsBatch struct {
	limit int
	exec  func(query ch.Query) error

	lock sync.Mutex
	done chan struct{}

	Timestamp          *chproto.ColDateTime64
	TraceId            *chproto.ColStr
	SpanId             *chproto.ColStr
	TraceFlags         *chproto.ColUInt32
	SeverityText       *chproto.ColLowCardinality[string]
	SeverityNumber     *chproto.ColInt32
	ServiceName        *chproto.ColLowCardinality[string]
	ResourceAttributes *chproto.ColMap[string, string]
	LogAttributes      *chproto.ColMap[string, string]
	Body               *chproto.ColStr
}

func NewLogsBatch(limit int, timeout time.Duration, exec func(query ch.Query) error) *LogsBatch {
	b := &LogsBatch{
		limit: limit,
		exec:  exec,
		done:  make(chan struct{}),

		Timestamp:          new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano),
		TraceId:            new(chproto.ColStr),
		SpanId:             new(chproto.ColStr),
		TraceFlags:         new(chproto.ColUInt32),
		SeverityText:       new(chproto.ColStr).LowCardinality(),
		SeverityNumber:     new(chproto.ColInt32),
		ServiceName:        new(chproto.ColStr).LowCardinality(),
		ResourceAttributes: chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr)),
		LogAttributes:      chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr)),
		Body:               new(chproto.ColStr),
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

func (b *LogsBatch) Close() {
	b.done <- struct{}{}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.save()
}

func (b *LogsBatch) Add(req *v1.ExportLogsServiceRequest) {
	b.lock.Lock()
	defer b.lock.Unlock()

	for _, l := range req.GetResourceLogs() {
		var serviceName string
		resourceAttributes := attributesToMap(l.GetResource().GetAttributes())
		for k, v := range resourceAttributes {
			if k == semconv.AttributeServiceName {
				serviceName = v
			}
		}
		for _, sl := range l.GetScopeLogs() {
			scopeName := sl.GetScope().GetName()
			scopeVersion := sl.GetScope().GetVersion()
			for _, lr := range sl.GetLogRecords() {
				logAttributes := attributesToMap(lr.GetAttributes())
				if scopeName != "" {
					logAttributes[semconv.AttributeOtelScopeName] = scopeName
				}
				if scopeVersion != "" {
					logAttributes[semconv.AttributeOtelScopeVersion] = scopeVersion
				}
				if int64(lr.GetTimeUnixNano()) < 0 {
					continue
				}
				b.Timestamp.Append(time.Unix(0, int64(lr.GetTimeUnixNano())))
				b.TraceId.Append(hex.EncodeToString(lr.GetTraceId()))
				b.SpanId.Append(hex.EncodeToString(lr.GetSpanId()))
				b.TraceFlags.Append(lr.GetFlags())
				b.SeverityText.Append(lr.GetSeverityText())
				b.SeverityNumber.Append(int32(lr.GetSeverityNumber()))
				b.ServiceName.Append(serviceName)
				b.ResourceAttributes.Append(resourceAttributes)
				b.LogAttributes.Append(logAttributes)
				b.Body.Append(lr.GetBody().GetStringValue())
			}
		}
	}
	if b.Timestamp.Rows() < b.limit {
		return
	}
	b.save()
}

func (b *LogsBatch) save() {
	if b.Timestamp.Rows() == 0 {
		return
	}

	input := chproto.Input{
		chproto.InputColumn{Name: "Timestamp", Data: b.Timestamp},
		chproto.InputColumn{Name: "TraceId", Data: b.TraceId},
		chproto.InputColumn{Name: "SpanId", Data: b.SpanId},
		chproto.InputColumn{Name: "TraceFlags", Data: b.TraceFlags},
		chproto.InputColumn{Name: "SeverityText", Data: b.SeverityText},
		chproto.InputColumn{Name: "SeverityNumber", Data: b.SeverityNumber},
		chproto.InputColumn{Name: "ServiceName", Data: b.ServiceName},
		chproto.InputColumn{Name: "ResourceAttributes", Data: b.ResourceAttributes},
		chproto.InputColumn{Name: "LogAttributes", Data: b.LogAttributes},
		chproto.InputColumn{Name: "Body", Data: b.Body},
	}
	err := b.exec(ch.Query{Body: input.Into("@@table_otel_logs@@"), Input: input})
	if err != nil {
		klog.Errorln(err)
	}
	for _, i := range input {
		i.Data.(chproto.Resettable).Reset()
	}
}
