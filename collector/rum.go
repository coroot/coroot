package collector

import (
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/ch-go"
	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/coroot/coroot/db"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
	v1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"k8s.io/klog"
)

func isRumRequest(r *http.Request, req *v1.ExportTraceServiceRequest) bool {
	if strings.EqualFold(r.Header.Get(CorootSignalHeader), "rum") {
		return true
	}
	for _, rs := range req.GetResourceSpans() {
		attrs := attributesToMap(rs.GetResource().GetAttributes())
		if lang := attrs[semconv.AttributeTelemetrySDKLanguage]; lang == "webjs" || lang == "javascript" {
			return true
		}
		if attrs["telemetry.sdk.language"] == "webjs" || attrs["telemetry.sdk.language"] == "javascript" {
			return true
		}
	}
	return false
}

func (c *Collector) rumPreflight(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	key := c.findRumKeyByOrigin(origin)
	if key == nil {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	writeCORSHeaders(w, origin, false)
	w.WriteHeader(http.StatusNoContent)
}

func (c *Collector) ingestRum(w http.ResponseWriter, r *http.Request, project *db.Project, key *db.ApiKey, req *v1.ExportTraceServiceRequest, ct string) {
	if key == nil || !key.IsRum() {
		http.Error(w, "rum api key required", http.StatusForbidden)
		return
	}
	origin, ok := allowRumRequest(r, key, rumRequestPath(r, req))
	if !ok {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	writeCORSHeaders(w, origin, false)
	hint := r.Header.Get("X-Coroot-Rum-Hint")
	enrichRumGeo(req, r, project)
	filtered := filterRumRequest(req, key, hint)
	if filtered != nil {
		c.getRumSpansBatch(project).Add(filtered)
	}

	resp := &v1.ExportTraceServiceResponse{}
	var data []byte
	var err error
	if ct == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		data, err = protojson.Marshal(resp)
	} else {
		w.Header().Set("Content-Type", ct)
		data, err = proto.Marshal(resp)
	}
	if err != nil {
		klog.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}

type RumSpansBatch struct {
	limit int
	exec  func(query ch.Query) error

	lock sync.Mutex
	done chan struct{}

	Timestamp          *chproto.ColDateTime64
	TraceId            *chproto.ColStr
	SpanId             *chproto.ColStr
	ParentSpanId       *chproto.ColStr
	TraceState         *chproto.ColStr
	SpanName           *chproto.ColLowCardinality[string]
	SpanKind           *chproto.ColLowCardinality[string]
	ServiceName        *chproto.ColLowCardinality[string]
	ResourceAttributes *chproto.ColMap[string, string]
	SpanAttributes     *chproto.ColMap[string, string]
	Duration           *chproto.ColInt64
	StatusCode         *chproto.ColLowCardinality[string]
	StatusMessage      *chproto.ColStr
	EventsTimestamp    *chproto.ColArr[time.Time]
	EventsName         *chproto.ColArr[string]
	EventsAttributes   *chproto.ColArr[map[string]string]
	LinksTraceId       *chproto.ColArr[string]
	LinksSpanId        *chproto.ColArr[string]
	LinksTraceState    *chproto.ColArr[string]
	LinksAttributes    *chproto.ColArr[map[string]string]
	PagePath           *chproto.ColLowCardinality[string]
	SessionId          *chproto.ColStr

	// events batch columns
	EvTimestamp   *chproto.ColDateTime64
	EvServiceName *chproto.ColLowCardinality[string]
	EvTraceId     *chproto.ColStr
	EvSpanId      *chproto.ColStr
	EvSessionId   *chproto.ColStr
	EvPagePath    *chproto.ColLowCardinality[string]
	EvEventType   *chproto.ColLowCardinality[string]
	EvValue       *chproto.ColFloat64
	EvRating      *chproto.ColLowCardinality[string]
	EvBrowserName *chproto.ColLowCardinality[string]
	EvOsName      *chproto.ColLowCardinality[string]
	EvDeviceType  *chproto.ColLowCardinality[string]
	EvAttributes  *chproto.ColMap[string, string]
}

func NewRumSpansBatch(limit int, timeout time.Duration, exec func(query ch.Query) error) *RumSpansBatch {
	b := &RumSpansBatch{
		limit: limit,
		exec:  exec,
		done:  make(chan struct{}),

		Timestamp:          new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano),
		TraceId:            new(chproto.ColStr),
		SpanId:             new(chproto.ColStr),
		ParentSpanId:       new(chproto.ColStr),
		TraceState:         new(chproto.ColStr),
		SpanName:           new(chproto.ColStr).LowCardinality(),
		SpanKind:           new(chproto.ColStr).LowCardinality(),
		ServiceName:        new(chproto.ColStr).LowCardinality(),
		ResourceAttributes: chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr)),
		SpanAttributes:     chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr)),
		Duration:           new(chproto.ColInt64),
		StatusCode:         new(chproto.ColStr).LowCardinality(),
		StatusMessage:      new(chproto.ColStr),
		EventsTimestamp:    new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano).Array(),
		EventsName:         new(chproto.ColStr).LowCardinality().Array(),
		EventsAttributes:   chproto.NewArray[map[string]string](chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr))),
		LinksTraceId:       new(chproto.ColStr).Array(),
		LinksSpanId:        new(chproto.ColStr).Array(),
		LinksTraceState:    new(chproto.ColStr).Array(),
		LinksAttributes:    chproto.NewArray[map[string]string](chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr))),
		PagePath:           new(chproto.ColStr).LowCardinality(),
		SessionId:          new(chproto.ColStr),

		EvTimestamp:   new(chproto.ColDateTime64).WithPrecision(chproto.PrecisionNano),
		EvServiceName: new(chproto.ColStr).LowCardinality(),
		EvTraceId:     new(chproto.ColStr),
		EvSpanId:      new(chproto.ColStr),
		EvSessionId:   new(chproto.ColStr),
		EvPagePath:    new(chproto.ColStr).LowCardinality(),
		EvEventType:   new(chproto.ColStr).LowCardinality(),
		EvValue:       new(chproto.ColFloat64),
		EvRating:      new(chproto.ColStr).LowCardinality(),
		EvBrowserName: new(chproto.ColStr).LowCardinality(),
		EvOsName:      new(chproto.ColStr).LowCardinality(),
		EvDeviceType:  new(chproto.ColStr).LowCardinality(),
		EvAttributes:  chproto.NewMap[string, string](new(chproto.ColStr).LowCardinality(), new(chproto.ColStr)),
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

func (b *RumSpansBatch) Close() {
	b.done <- struct{}{}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.save()
}

func (b *RumSpansBatch) Add(req *v1.ExportTraceServiceRequest) {
	b.lock.Lock()
	defer b.lock.Unlock()

	for _, rs := range req.GetResourceSpans() {
		var serviceName string
		resourceAttributes := attributesToMap(rs.GetResource().GetAttributes())
		for k, v := range resourceAttributes {
			if k == semconv.AttributeServiceName {
				serviceName = v
			}
		}
		browserName := resourceAttributes["browser.name"]
		osName := resourceAttributes["os.name"]
		deviceType := resourceAttributes["device.type"]
		sessionId := resourceAttributes["session.id"]

		for _, ss := range rs.GetScopeSpans() {
			scopeName := ss.GetScope().GetName()
			scopeVersion := ss.GetScope().GetVersion()
			for _, s := range ss.GetSpans() {
				spanAttributes := attributesToMap(s.GetAttributes())
				if scopeName != "" {
					spanAttributes[semconv.AttributeOtelScopeName] = scopeName
				}
				if scopeVersion != "" {
					spanAttributes[semconv.AttributeOtelScopeVersion] = scopeVersion
				}
				if v := resourceAttributes["service.version"]; v != "" {
					spanAttributes["service.version"] = v
				}
				if v := resourceAttributes["geo.country"]; v != "" {
					spanAttributes["geo.country"] = v
				}
				if v := resourceAttributes["deployment.environment"]; v != "" {
					spanAttributes["deployment.environment"] = v
				}
				if sessionId == "" {
					sessionId = spanAttributes["session.id"]
				}
				rawPagePath := firstNonEmpty(
					spanAttributes["page.path"],
					spanAttributes["page.url.path"],
					spanAttributes["url.path"],
					pathFromURL(spanAttributes["http.url"]),
					pathFromURL(spanAttributes["url.full"]),
				)
				pagePath := normalizePagePath(rawPagePath)
				if rawPagePath != "" && pagePath != rawPagePath {
					spanAttributes["page.path.raw"] = rawPagePath
				}
				if pagePath != "" {
					spanAttributes["page.path"] = pagePath
				}

				ts := time.Unix(0, int64(s.GetStartTimeUnixNano()))
				traceId := hex.EncodeToString(s.GetTraceId())
				spanId := hex.EncodeToString(s.GetSpanId())

				var eventTimestamps []time.Time
				var eventNames []string
				var eventAttributes []map[string]string
				for _, e := range s.GetEvents() {
					eventTimestamps = append(eventTimestamps, time.Unix(0, int64(e.GetTimeUnixNano())))
					eventNames = append(eventNames, e.GetName())
					ea := attributesToMap(e.GetAttributes())
					eventAttributes = append(eventAttributes, ea)
					b.maybeAppendEvent(time.Unix(0, int64(e.GetTimeUnixNano())), serviceName, traceId, spanId, sessionId, pagePath, browserName, osName, deviceType, e.GetName(), ea)
				}

				b.maybeAppendVitalFromSpan(ts, serviceName, traceId, spanId, sessionId, pagePath, browserName, osName, deviceType, s.GetName(), spanAttributes)
				if s.GetStatus().GetCode().String() == "STATUS_CODE_ERROR" {
					b.appendEvent(ts, serviceName, traceId, spanId, sessionId, pagePath, "js_error", 1, "poor", browserName, osName, deviceType, spanAttributes)
				}

				// CWV leaf spans are stored only in rum_events to avoid duplication.
				if isCwvLeafSpanName(s.GetName()) {
					continue
				}

				var linkTraceIds []string
				var linkSpanIds []string
				var linkTraceStates []string
				var linkAttributes []map[string]string
				for _, l := range s.GetLinks() {
					linkTraceIds = append(linkTraceIds, hex.EncodeToString(l.GetTraceId()))
					linkSpanIds = append(linkSpanIds, hex.EncodeToString(l.GetSpanId()))
					linkTraceStates = append(linkTraceStates, l.GetTraceState())
					linkAttributes = append(linkAttributes, attributesToMap(l.GetAttributes()))
				}

				b.Timestamp.Append(ts)
				b.TraceId.Append(traceId)
				b.SpanId.Append(spanId)
				b.ParentSpanId.Append(hex.EncodeToString(s.GetParentSpanId()))
				b.TraceState.Append(s.GetTraceState())
				b.SpanName.Append(s.GetName())
				b.SpanKind.Append(s.GetKind().String())
				b.ServiceName.Append(serviceName)
				b.ResourceAttributes.Append(resourceAttributes)
				b.SpanAttributes.Append(spanAttributes)
				b.Duration.Append(int64(s.GetEndTimeUnixNano() - s.GetStartTimeUnixNano()))
				b.StatusCode.Append(s.GetStatus().GetCode().String())
				b.StatusMessage.Append(s.GetStatus().GetMessage())
				b.EventsTimestamp.Append(eventTimestamps)
				b.EventsName.Append(eventNames)
				b.EventsAttributes.Append(eventAttributes)
				b.LinksTraceId.Append(linkTraceIds)
				b.LinksSpanId.Append(linkSpanIds)
				b.LinksTraceState.Append(linkTraceStates)
				b.LinksAttributes.Append(linkAttributes)
				b.PagePath.Append(pagePath)
				b.SessionId.Append(sessionId)
			}
		}
	}
	if b.Timestamp.Rows() < b.limit {
		return
	}
	b.save()
}

func vitalValueFromAttrs(attrs map[string]string, vital string) float64 {
	return parseFloat(firstNonEmpty(
		attrs["vital.value"],
		attrs["value"],
		attrs[vital+".value"],
		attrs[vital],
	))
}

func (b *RumSpansBatch) maybeAppendVitalFromSpan(ts time.Time, serviceName, traceId, spanId, sessionId, pagePath, browser, os, device, spanName string, attrs map[string]string) {
	name := strings.ToLower(spanName)
	for _, vital := range []string{"lcp", "inp", "cls", "ttfb", "fcp", "fid"} {
		if name == vital || strings.Contains(name, "webvital."+vital) || strings.Contains(name, "web_vital_"+vital) || attrs["vital.name"] == vital {
			val := vitalValueFromAttrs(attrs, vital)
			rating := firstNonEmpty(attrs["vital.rating"], attrs["rating"], rateVital(vital, val))
			b.appendEvent(ts, serviceName, traceId, spanId, sessionId, pagePath, vital, val, rating, browser, os, device, attrs)
			return
		}
	}
}

func (b *RumSpansBatch) maybeAppendEvent(ts time.Time, serviceName, traceId, spanId, sessionId, pagePath, browser, os, device, eventName string, attrs map[string]string) {
	name := strings.ToLower(eventName)
	for _, vital := range []string{"lcp", "inp", "cls", "ttfb", "fcp", "fid"} {
		if name == vital || strings.Contains(name, vital) {
			val := vitalValueFromAttrs(attrs, vital)
			rating := firstNonEmpty(attrs["vital.rating"], attrs["rating"], rateVital(vital, val))
			b.appendEvent(ts, serviceName, traceId, spanId, sessionId, pagePath, vital, val, rating, browser, os, device, attrs)
			return
		}
	}
	if strings.Contains(name, "exception") || strings.Contains(name, "error") {
		b.appendEvent(ts, serviceName, traceId, spanId, sessionId, pagePath, "js_error", 1, "poor", browser, os, device, attrs)
	}
}

func (b *RumSpansBatch) appendEvent(ts time.Time, serviceName, traceId, spanId, sessionId, pagePath, eventType string, value float64, rating, browser, os, device string, attrs map[string]string) {
	filtered := filterRumEventAttributes(attrs)
	if pagePath != "" {
		filtered["page.path"] = pagePath
	}
	b.EvTimestamp.Append(ts)
	b.EvServiceName.Append(serviceName)
	b.EvTraceId.Append(traceId)
	b.EvSpanId.Append(spanId)
	b.EvSessionId.Append(sessionId)
	b.EvPagePath.Append(pagePath)
	b.EvEventType.Append(eventType)
	b.EvValue.Append(value)
	b.EvRating.Append(rating)
	b.EvBrowserName.Append(browser)
	b.EvOsName.Append(os)
	b.EvDeviceType.Append(device)
	b.EvAttributes.Append(filtered)
}

func (b *RumSpansBatch) save() {
	if b.Timestamp.Rows() > 0 {
		input := chproto.Input{
			{Name: "Timestamp", Data: b.Timestamp},
			{Name: "TraceId", Data: b.TraceId},
			{Name: "SpanId", Data: b.SpanId},
			{Name: "ParentSpanId", Data: b.ParentSpanId},
			{Name: "TraceState", Data: b.TraceState},
			{Name: "SpanName", Data: b.SpanName},
			{Name: "SpanKind", Data: b.SpanKind},
			{Name: "ServiceName", Data: b.ServiceName},
			{Name: "ResourceAttributes", Data: b.ResourceAttributes},
			{Name: "SpanAttributes", Data: b.SpanAttributes},
			{Name: "Duration", Data: b.Duration},
			{Name: "StatusCode", Data: b.StatusCode},
			{Name: "StatusMessage", Data: b.StatusMessage},
			{Name: "Events.Timestamp", Data: b.EventsTimestamp},
			{Name: "Events.Name", Data: b.EventsName},
			{Name: "Events.Attributes", Data: b.EventsAttributes},
			{Name: "Links.TraceId", Data: b.LinksTraceId},
			{Name: "Links.SpanId", Data: b.LinksSpanId},
			{Name: "Links.TraceState", Data: b.LinksTraceState},
			{Name: "Links.Attributes", Data: b.LinksAttributes},
			{Name: "PagePath", Data: b.PagePath},
			{Name: "SessionId", Data: b.SessionId},
		}
		if err := b.exec(ch.Query{Body: input.Into("@@table_rum_spans@@"), Input: input}); err != nil {
			klog.Errorln(err)
		}
		for _, i := range input {
			i.Data.(chproto.Resettable).Reset()
		}
	}
	if b.EvTimestamp.Rows() > 0 {
		input := chproto.Input{
			{Name: "Timestamp", Data: b.EvTimestamp},
			{Name: "ServiceName", Data: b.EvServiceName},
			{Name: "TraceId", Data: b.EvTraceId},
			{Name: "SpanId", Data: b.EvSpanId},
			{Name: "SessionId", Data: b.EvSessionId},
			{Name: "PagePath", Data: b.EvPagePath},
			{Name: "EventType", Data: b.EvEventType},
			{Name: "Value", Data: b.EvValue},
			{Name: "Rating", Data: b.EvRating},
			{Name: "BrowserName", Data: b.EvBrowserName},
			{Name: "OsName", Data: b.EvOsName},
			{Name: "DeviceType", Data: b.EvDeviceType},
			{Name: "Attributes", Data: b.EvAttributes},
		}
		if err := b.exec(ch.Query{Body: input.Into("@@table_rum_events@@"), Input: input}); err != nil {
			klog.Errorln(err)
		}
		for _, i := range input {
			i.Data.(chproto.Resettable).Reset()
		}
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func pathFromURL(u string) string {
	if u == "" {
		return ""
	}
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
		if j := strings.Index(u, "/"); j >= 0 {
			u = u[j:]
		} else {
			return "/"
		}
	}
	if q := strings.IndexAny(u, "?#"); q >= 0 {
		u = u[:q]
	}
	return u
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func rateVital(vital string, value float64) string {
	switch vital {
	case "lcp", "fcp":
		if value <= 2500 {
			return "good"
		}
		if value <= 4000 {
			return "needs-improvement"
		}
		return "poor"
	case "inp", "fid":
		if value <= 200 {
			return "good"
		}
		if value <= 500 {
			return "needs-improvement"
		}
		return "poor"
	case "cls":
		if value <= 0.1 {
			return "good"
		}
		if value <= 0.25 {
			return "needs-improvement"
		}
		return "poor"
	case "ttfb":
		if value <= 800 {
			return "good"
		}
		if value <= 1800 {
			return "needs-improvement"
		}
		return "poor"
	}
	return ""
}
