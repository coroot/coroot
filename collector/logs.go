package collector

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/coroot/coroot/db"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	v1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/protobuf/proto"
	"k8s.io/klog"
)

func (c *Collector) Logs(w http.ResponseWriter, r *http.Request) {
	projectId := db.ProjectId(r.Header.Get(ApiKeyHeader))
	_, err := c.getClickhouseClient(projectId)
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

	data, err := io.ReadAll(r.Body)
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

	err = c.saveLogs(r.Context(), projectId, req)
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

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

func (c *Collector) saveLogs(ctx context.Context, projectId db.ProjectId, req *v1.ExportLogsServiceRequest) error {
	colTimestamp := new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano)
	colTraceId := new(chproto.ColStr)
	colSpanId := new(chproto.ColStr)
	colTraceFlags := new(chproto.ColUInt32)
	colSeverityText := new(chproto.ColStr).LowCardinality()
	colSeverityNumber := new(chproto.ColInt32)
	colServiceName := new(chproto.ColStr).LowCardinality()
	colResourceAttributes := chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr))
	colLogAttributes := chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr))
	colBody := new(chproto.ColStr)

	for _, l := range req.GetResourceLogs() {
		var serviceName string
		resourceAttributes := map[string]string{}
		for _, attr := range l.GetResource().GetAttributes() {
			if attr.GetKey() == semconv.AttributeServiceName {
				serviceName = attr.GetValue().GetStringValue()
			}
			resourceAttributes[attr.GetKey()] = attr.GetValue().GetStringValue()
		}
		for _, sl := range l.GetScopeLogs() {
			scopeName := sl.GetScope().GetName()
			scopeVersion := sl.GetScope().GetVersion()
			for _, lr := range sl.GetLogRecords() {
				colTimestamp.Append(time.Unix(0, int64(lr.GetTimeUnixNano())))
				colTraceId.Append(hex.EncodeToString(lr.GetTraceId()))
				colSpanId.Append(hex.EncodeToString(lr.GetSpanId()))
				colTraceFlags.Append(lr.GetFlags())
				colSeverityText.Append(lr.GetSeverityText())
				colSeverityNumber.Append(int32(lr.GetSeverityNumber()))
				colServiceName.Append(serviceName)
				colBody.Append(lr.GetBody().GetStringValue())

				colResourceAttributes.Append(resourceAttributes)
				logAttributes := map[string]string{}
				for _, attr := range lr.GetAttributes() {
					logAttributes[attr.GetKey()] = attr.GetValue().GetStringValue()
				}
				if scopeName != "" {
					logAttributes[semconv.AttributeOtelScopeName] = scopeName
				}
				if scopeVersion != "" {
					logAttributes[semconv.AttributeOtelScopeVersion] = scopeVersion
				}
				colLogAttributes.Append(logAttributes)
			}
		}
	}
	input := chproto.Input{
		{Name: "Timestamp", Data: colTimestamp},
		{Name: "TraceId", Data: colTraceId},
		{Name: "SpanId", Data: colSpanId},
		{Name: "TraceFlags", Data: colTraceFlags},
		{Name: "SeverityText", Data: colSeverityText},
		{Name: "SeverityNumber", Data: colSeverityNumber},
		{Name: "ServiceName", Data: colServiceName},
		{Name: "Body", Data: colBody},
		{Name: "ResourceAttributes", Data: colResourceAttributes},
		{Name: "LogAttributes", Data: colLogAttributes},
	}
	return c.clickhouseDo(ctx, projectId, ch.Query{Body: input.Into("otel_logs"), Input: input})
}
