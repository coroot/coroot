package collector

import (
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
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
	rows := getLogRows()
	defer putLogRows(rows)
	if err = rows.Unmarshal(data); err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	c.getLogsBatch(project).add(rows)

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

type logsColumns struct {
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

	hexBuf []byte
}

func newLogsColumns() *logsColumns {
	return &logsColumns{
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
}

func (c *logsColumns) append(rows []logRow) {
	for i := range rows {
		r := &rows[i]
		c.Timestamp.Append(time.Unix(0, r.timestamp))
		c.hexBuf = hex.AppendEncode(c.hexBuf[:0], r.traceId)
		c.TraceId.AppendBytes(c.hexBuf)
		c.hexBuf = hex.AppendEncode(c.hexBuf[:0], r.spanId)
		c.SpanId.AppendBytes(c.hexBuf)
		c.TraceFlags.Append(r.traceFlags)
		c.SeverityText.Append(r.severityText)
		c.SeverityNumber.Append(r.severityNumber)
		c.ServiceName.Append(r.serviceName)
		c.ResourceAttributes.AppendKV(r.resourceAttributes)
		c.LogAttributes.AppendKV(r.logAttributes)
		c.Body.Append(r.body)
	}
}

func (c *logsColumns) rows() int {
	return c.Timestamp.Rows()
}

func (c *logsColumns) input() chproto.Input {
	return chproto.Input{
		chproto.InputColumn{Name: "Timestamp", Data: c.Timestamp},
		chproto.InputColumn{Name: "TraceId", Data: c.TraceId},
		chproto.InputColumn{Name: "SpanId", Data: c.SpanId},
		chproto.InputColumn{Name: "TraceFlags", Data: c.TraceFlags},
		chproto.InputColumn{Name: "SeverityText", Data: c.SeverityText},
		chproto.InputColumn{Name: "SeverityNumber", Data: c.SeverityNumber},
		chproto.InputColumn{Name: "ServiceName", Data: c.ServiceName}, //nolint:goconst // column name
		chproto.InputColumn{Name: "ResourceAttributes", Data: c.ResourceAttributes},
		chproto.InputColumn{Name: "LogAttributes", Data: c.LogAttributes},
		chproto.InputColumn{Name: "Body", Data: c.Body},
	}
}

type LogsBatch struct {
	batch[*logsColumns]
}

func NewLogsBatch(limit int, timeout time.Duration, exec func(query ch.Query) error) *LogsBatch {
	b := &LogsBatch{}
	b.start(limit, timeout, "@@table_otel_logs@@", newLogsColumns, exec)
	return b
}

func (b *LogsBatch) add(rows *logRows) {
	b.write(func(c *logsColumns) { c.append(rows.rows) })
}
