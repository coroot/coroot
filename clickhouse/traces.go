package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

func (c *Client) GetServicesFromTraces(ctx context.Context) ([]string, error) {
	q := "SELECT DISTINCT ServiceName"
	q += " FROM " + c.config.TracesTable
	rows, err := c.conn.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var res []string
	for rows.Next() {
		var app string
		if err = rows.Scan(&app); err != nil {
			return nil, err
		}
		res = append(res, app)
	}
	return res, nil
}

func (c *Client) GetSpansByServiceName(ctx context.Context, name string, ignoredPeerAddrs []string, tsFrom, tsTo timeseries.Time, durFrom, durTo time.Duration, errors bool, limit int) ([]*model.TraceSpan, error) {
	return c.getSpans(ctx, tsFrom, tsTo, durFrom, durTo, errors, "", "", limit,
		`
			ServiceName = @name AND 
			SpanKind = 'SPAN_KIND_SERVER' AND 
			SpanAttributes['net.sock.peer.addr'] NOT IN (@addrs)
		`,
		clickhouse.Named("name", name),
		clickhouse.Named("addrs", ignoredPeerAddrs),
	)
}

func (c *Client) GetInboundSpans(ctx context.Context, listens []model.Listen, ignoredContainerIds []string, tsFrom, tsTo timeseries.Time, durFrom, durTo time.Duration, errors bool, limit int) ([]*model.TraceSpan, error) {
	if len(listens) == 0 {
		return nil, nil
	}
	ips := utils.NewStringSet()
	for _, l := range listens {
		if l.Port == "0" {
			ips.Add(l.IP)
		}
	}
	var addrs []clickhouse.GroupSet
	for _, l := range listens {
		addrs = append(addrs, clickhouse.GroupSet{Value: []any{l.IP, l.Port}})
	}
	return c.getSpans(ctx, tsFrom, tsTo, durFrom, durTo, errors, "", "Timestamp DESC", limit,
		`
			ServiceName = 'coroot-node-agent' AND 
			(SpanAttributes['net.peer.name'] IN (@ips) OR (SpanAttributes['net.peer.name'], SpanAttributes['net.peer.port']) IN (@addrs)) 
			AND SpanAttributes['container.id'] NOT IN (@containerIds)
		`,
		clickhouse.Named("ips", ips.Items()),
		clickhouse.Named("addrs", addrs),
		clickhouse.Named("containerIds", ignoredContainerIds),
	)
}

func (c *Client) GetParentSpans(ctx context.Context, spans []*model.TraceSpan) ([]*model.TraceSpan, error) {
	traceIds := utils.NewStringSet()
	var ids []clickhouse.GroupSet
	for _, s := range spans {
		if s.ParentSpanId != "" {
			ids = append(ids, clickhouse.GroupSet{Value: []any{s.TraceId, s.ParentSpanId}})
			traceIds.Add(s.TraceId)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return c.getSpans(ctx, 0, 0, 0, 0, false,
		"@traceIds as traceIds, (SELECT min(Start) FROM otel_traces_trace_id_ts WHERE TraceId IN (traceIds)) as start, (SELECT max(End) + 1 FROM otel_traces_trace_id_ts WHERE TraceId IN (traceIds)) as end",
		"", 0,
		"Timestamp BETWEEN start AND end AND TraceId IN (traceIds) AND (TraceId, SpanId) IN (@ids)",
		clickhouse.Named("traceIds", traceIds.Items()),
		clickhouse.Named("ids", ids),
	)
}

func (c *Client) GetSpansByTraceId(ctx context.Context, traceId string) ([]*model.TraceSpan, error) {
	return c.getSpans(ctx, 0, 0, 0, 0, false,
		"(SELECT min(Start) FROM otel_traces_trace_id_ts WHERE TraceId = @traceId) as start, (SELECT max(End) + 1 FROM otel_traces_trace_id_ts WHERE TraceId = @traceId) as end",
		"Timestamp", 0,
		"TraceId = @traceId AND Timestamp BETWEEN start AND end",
		clickhouse.Named("traceId", traceId),
	)
}

func (c *Client) getSpans(ctx context.Context, tsFrom, tsTo timeseries.Time, durFrom, durTo time.Duration, errors bool, with string, orderBy string, limit int, filter string, filterArgs ...any) ([]*model.TraceSpan, error) {
	var filters []string
	var args []any

	if !tsFrom.IsZero() && !tsTo.IsZero() {
		filters = append(filters, "Timestamp BETWEEN @tsFrom AND @tsTo")
		args = append(args,
			clickhouse.DateNamed("tsFrom", tsFrom.ToStandard(), clickhouse.NanoSeconds),
			clickhouse.DateNamed("tsTo", tsTo.ToStandard(), clickhouse.NanoSeconds),
		)
	}

	if filter != "" {
		filters = append(filters, filter)
		args = append(args, filterArgs...)
	}

	switch {
	case durFrom > 0 && durTo > 0 && errors:
		filters = append(filters, "(Duration BETWEEN @durFrom AND @durTo OR StatusCode = 'STATUS_CODE_ERROR')")
	case durFrom == 0 && durTo > 0 && errors:
		filters = append(filters, "(Duration <= @durTo OR StatusCode = 'STATUS_CODE_ERROR')")
	case durFrom > 0 && durTo == 0 && errors:
		filters = append(filters, "(Duration >= @durFrom OR StatusCode = 'STATUS_CODE_ERROR')")
	case durFrom == 0 && durTo == 0 && errors:
		filters = append(filters, "StatusCode = 'STATUS_CODE_ERROR'")
	case durFrom > 0 && durTo > 0 && !errors:
		filters = append(filters, "Duration BETWEEN @durFrom AND @durTo")
	case durFrom == 0 && durTo > 0 && !errors:
		filters = append(filters, "Duration <= @durTo")
	case durFrom > 0 && durTo == 0 && !errors:
		filters = append(filters, "Duration >= @durFrom")
	}
	if durFrom > 0 {
		args = append(args, clickhouse.Named("durFrom", durFrom.Nanoseconds()))
	}
	if durTo > 0 {
		args = append(args, clickhouse.Named("durTo", durTo.Nanoseconds()))
	}

	q := ""
	if with != "" {
		q += "WITH " + with
	}
	q += " SELECT Timestamp, TraceId, SpanId, ParentSpanId, SpanName, ServiceName, Duration, StatusCode, StatusMessage, SpanAttributes, Events.Timestamp, Events.Name, Events.Attributes"
	q += " FROM " + c.config.TracesTable
	q += " WHERE " + strings.Join(filters, " AND ")
	if orderBy != "" {
		q += " ORDER BY " + orderBy
	}
	if limit > 0 {
		q += " LIMIT " + fmt.Sprint(limit)
	}

	rows, err := c.conn.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var res []*model.TraceSpan
	for rows.Next() {
		var s model.TraceSpan
		var eventsTimestamp []time.Time
		var eventsName []string
		var eventsAttributes []map[string]string
		if err = rows.Scan(&s.Timestamp, &s.TraceId, &s.SpanId, &s.ParentSpanId, &s.Name, &s.ServiceName, &s.Duration,
			&s.StatusCode, &s.StatusMessage, &s.Attributes, &eventsTimestamp, &eventsName, &eventsAttributes,
		); err != nil {
			return nil, err
		}
		l := len(eventsTimestamp)
		if l > 0 && l == len(eventsName) && l == len(eventsAttributes) {
			s.Events = make([]model.TraceSpanEvent, l, l)
			for i := range eventsTimestamp {
				s.Events[i].Timestamp = eventsTimestamp[i]
				s.Events[i].Name = eventsName[i]
				s.Events[i].Attributes = eventsAttributes[i]
			}
		}
		res = append(res, &s)
	}
	return res, nil
}
