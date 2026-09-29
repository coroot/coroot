package clickhouse

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

func (c *Client) GetServicesFromLogs(ctx context.Context, from timeseries.Time) ([]string, error) {
	rows, err := c.Query(ctx, "SELECT DISTINCT ServiceName FROM @@table_otel_logs_service_name_severity_text@@ WHERE LastSeen >= @from",
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []string
	var app string
	for rows.Next() {
		if err = rows.Scan(&app); err != nil {
			return nil, err
		}
		res = append(res, app)
	}
	return res, nil
}

func (c *Client) GetLogSources(ctx context.Context, from timeseries.Time) (otelServices []string, agentLogsFound bool, err error) {
	services, err := c.GetServicesFromLogs(ctx, from)
	if err != nil {
		return nil, false, err
	}
	for _, s := range services {
		if strings.HasPrefix(s, "/") {
			agentLogsFound = true
		} else {
			otelServices = append(otelServices, s)
		}
	}
	return otelServices, agentLogsFound, nil
}

func (c *Client) GetLogsHistogram(ctx context.Context, query LogQuery) ([]model.LogHistogramBucket, error) {
	query, err := c.resolveSeverityTexts(ctx, query)
	if err != nil {
		return nil, err
	}
	where, args := query.filters(nil)
	q := fmt.Sprintf("SELECT SeverityText, max(multiIf(SeverityNumber=0, 0, intDiv(SeverityNumber, 4)+1)), toStartOfInterval(Timestamp, INTERVAL %d second), count(1)", query.Ctx.Step)
	q += " FROM @@table_otel_logs@@"
	q += " WHERE " + strings.Join(where, " AND ")
	q += " GROUP BY 1, 3"
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byText := map[string]*model.LogHistogramBucket{}
	counts := map[string]map[timeseries.Time]uint64{}
	var text string
	var sev int64
	var t time.Time
	var count uint64
	for rows.Next() {
		if err = rows.Scan(&text, &sev, &t, &count); err != nil {
			return nil, err
		}
		text = model.NormalizeSeverityText(text)
		b := byText[text]
		if b == nil {
			b = &model.LogHistogramBucket{SeverityText: text}
			byText[text] = b
			counts[text] = map[timeseries.Time]uint64{}
		}
		b.Severity = max(b.Severity, model.Severity(sev))
		counts[text][timeseries.Time(t.Unix())] += count
	}
	res := make([]model.LogHistogramBucket, 0, len(byText))
	for text, b := range byText {
		b.Timeseries = timeseries.New(query.Ctx.From, query.Ctx.PointsCount(), query.Ctx.Step)
		for t, count := range counts[text] {
			b.Timeseries.Set(t, float32(count))
		}
		res = append(res, *b)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Severity == res[j].Severity {
			return res[i].SeverityText < res[j].SeverityText
		}
		return res[i].Severity < res[j].Severity
	})
	return res, nil
}

func (c *Client) GetLogs(ctx context.Context, query LogQuery) ([]*model.LogEntry, error) {
	query, err := c.resolveSeverityTexts(ctx, query)
	if err != nil {
		return nil, err
	}
	where, args := query.filters(nil)
	cond := strings.Join(where, " AND ")
	limit := fmt.Sprint(query.Limit)
	cutoff := "SELECT min(Timestamp) FROM (SELECT Timestamp FROM @@table_otel_logs@@ WHERE " + cond + " ORDER BY Timestamp DESC LIMIT " + limit + ")"
	q := "SELECT ServiceName, Timestamp, multiIf(SeverityNumber=0, 0, intDiv(SeverityNumber, 4)+1), SeverityText, Body, TraceId, ResourceAttributes, LogAttributes"
	q += " FROM @@table_otel_logs@@"
	q += " WHERE " + cond + " AND Timestamp >= (" + cutoff + ")"
	q += " ORDER BY Timestamp DESC"
	q += " LIMIT " + limit

	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*model.LogEntry
	for rows.Next() {
		var e model.LogEntry
		var sev int64
		if err = rows.Scan(&e.ServiceName, &e.Timestamp, &sev, &e.SeverityText, &e.Body, &e.TraceId, &e.ResourceAttributes, &e.LogAttributes); err != nil {
			return nil, err
		}
		e.Severity = model.Severity(sev)
		e.SeverityText = model.NormalizeSeverityText(e.SeverityText)
		e.ClusterId = c.project.ClusterId()
		e.ClusterName = c.project.Name
		res = append(res, &e)
	}
	return res, nil
}

const maxLogFilterScanWindow = 1 * timeseries.Hour

func (c *Client) GetLogFilters(ctx context.Context, query LogQuery, name string) ([]string, error) {
	if query.Since.IsZero() {
		if window := query.Ctx.To.Sub(query.Ctx.From); window > maxLogFilterScanWindow {
			query.Ctx.From = query.Ctx.To.Add(-maxLogFilterScanWindow)
		}
	}
	query, err := c.resolveSeverityTexts(ctx, query)
	if err != nil {
		return nil, err
	}
	where, args := query.filters(&name)
	var q string
	var res []string
	orderBy := "ORDER BY 1"
	settings := " SETTINGS max_block_size=2048, max_threads=4"
	switch name {
	case "":
		res = append(res, "Severity", "Message", "TraceId", "Cluster")
		q = "SELECT arrayJoin(arrayConcat(mapKeys(LogAttributes), mapKeys(ResourceAttributes))) AS k"
		orderBy = `GROUP BY 1 HAVING NOT match(k, '\\.\\d+(\\.|$)') ORDER BY count(1) DESC, 1`
	case "Severity":
		q = "SELECT DISTINCT SeverityText"
	case "Message", "TraceId":
		return res, nil
	case "Cluster":
		return []string{c.project.Name}, nil
	default:
		q = "SELECT DISTINCT arrayJoin([LogAttributes[@attr], ResourceAttributes[@attr]])"
		args = append(args, clickhouse.Named("attr", name))
	}
	q += " FROM @@table_otel_logs@@"
	q += " WHERE " + strings.Join(where, " AND ")
	q += " " + orderBy + " LIMIT 1000" + settings
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var s string
	for rows.Next() {
		if err = rows.Scan(&s); err != nil {
			return nil, err
		}
		if name == "Severity" {
			s = model.NormalizeSeverityText(s)
			if slices.Contains(res, s) {
				continue
			}
		}
		if s == "" {
			continue
		}
		res = append(res, s)
	}
	return res, nil
}

func (c *Client) GetKubernetesEvents(ctx context.Context, from, to timeseries.Time, limit int, extraFilters ...LogFilter) ([]*model.LogEntry, error) {
	filters := []LogFilter{{Name: "service.name", Op: "=", Value: "KubernetesEvents"}}
	filters = append(filters, extraFilters...)
	q := LogQuery{
		Ctx:     timeseries.NewContext(from, to, 0),
		Filters: filters,
		Limit:   limit,
	}
	return c.GetLogs(ctx, q)
}

type LogQuery struct {
	Ctx      timeseries.Context
	Source   model.LogSource
	Services []string
	Filters  []LogFilter
	Limit    int
	Since    time.Time

	severityTexts map[string][]string
}

func (c *Client) resolveSeverityTexts(ctx context.Context, q LogQuery) (LogQuery, error) {
	if !slices.ContainsFunc(q.Filters, func(f LogFilter) bool { return f.Name == "Severity" }) {
		return q, nil
	}
	from := q.Ctx.From.ToStandard()
	if !q.Since.IsZero() {
		from = q.Since
	}
	query := "SELECT DISTINCT SeverityText FROM @@table_otel_logs_service_name_severity_text@@ WHERE LastSeen >= @from"
	args := []any{clickhouse.DateNamed("from", from, clickhouse.NanoSeconds)}
	if len(q.Services) > 0 {
		query += " AND ServiceName IN (@services)"
		args = append(args, clickhouse.Named("services", q.Services))
	}
	rows, err := c.Query(ctx, query, args...)
	if err != nil {
		return q, err
	}
	defer rows.Close()
	q.severityTexts = map[string][]string{}
	var text string
	for rows.Next() {
		if err = rows.Scan(&text); err != nil {
			return q, err
		}
		n := model.NormalizeSeverityText(text)
		q.severityTexts[n] = append(q.severityTexts[n], text)
	}
	return q, rows.Err()
}

type LogFilter struct {
	Name  string `json:"name"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

func (lf *LogFilter) Matches(value string) bool {
	if lf == nil {
		return true
	}
	switch lf.Op {
	case "=":
		return value == lf.Value
	case "!=":
		return value != lf.Value
	case "~":
		m, _ := regexp.MatchString(lf.Value, value)
		return m
	case "!~":
		m, _ := regexp.MatchString(lf.Value, value)
		return !m

	}
	return false
}

func (q LogQuery) filters(attr *string) ([]string, []any) {
	var where []string
	var args []any

	switch len(q.Services) {
	case 0:
		switch q.Source {
		case model.LogSourceAgent:
			where = append(where, "startsWith(ServiceName, '/')")
		case model.LogSourceOtel:
			where = append(where, "NOT startsWith(ServiceName, '/')")
		}
	case 1:
		where = append(where, "ServiceName = @serviceName")
		args = append(args, clickhouse.Named("serviceName", q.Services[0]))
	default:
		where = append(where, "ServiceName IN (@serviceName)")
		args = append(args, clickhouse.Named("serviceName", q.Services))
	}

	if !q.Since.IsZero() {
		where = append(where, "Timestamp > @since")
		args = append(args,
			clickhouse.DateNamed("since", q.Since, clickhouse.NanoSeconds),
		)
	} else {
		where = append(where, "Timestamp BETWEEN @from AND @to")
		args = append(args,
			clickhouse.DateNamed("from", q.Ctx.From.ToStandard(), clickhouse.NanoSeconds),
			clickhouse.DateNamed("to", q.Ctx.To.ToStandard(), clickhouse.NanoSeconds),
		)
	}

	filters := utils.Uniq(q.Filters)
	var message []string
	var notMessage [][]string
	byName := map[string][]LogFilter{}
	for _, f := range filters {
		if attr == nil && f.Name == "Message" {
			fields := strings.FieldsFunc(f.Value, func(r rune) bool {
				return unicode.IsSpace(r) || (r <= unicode.MaxASCII && !unicode.IsNumber(r) && !unicode.IsLetter(r))
			})
			if f.Op == "not contains" {
				if len(fields) > 0 {
					notMessage = append(notMessage, fields)
				}
			} else {
				message = append(message, fields...)
			}
			continue
		}
		if attr != nil && f.Name == *attr {
			continue
		}
		byName[f.Name] = append(byName[f.Name], f)
	}

	i := 0
	for name, attrs := range byName {
		var ors, ands []string
		switch name {
		case "Severity":
			for j, a := range attrs {
				var f *[]string
				var expr string
				switch a.Op {
				case "=":
					expr = "SeverityText IN (@%s)"
					f = &ors
				case "!=":
					expr = "SeverityText NOT IN (@%s)"
					f = &ands
				default:
					continue
				}
				texts := q.severityTexts[model.NormalizeSeverityText(a.Value)]
				if len(texts) == 0 {
					texts = []string{a.Value}
				}
				v := fmt.Sprintf("severity_%d", j)
				*f = append(*f, fmt.Sprintf(expr, v))
				args = append(args, clickhouse.Named(v, texts))
			}
		case "TraceId":
			for j, a := range attrs {
				var f *[]string
				var expr string
				switch a.Op {
				case "=":
					expr = "TraceId = @%[1]s"
					f = &ors
				default:
					continue
				}
				v := fmt.Sprintf("trace_id_%d", j)
				*f = append(*f, fmt.Sprintf(expr, v))
				args = append(args, clickhouse.Named(v, a.Value))
			}
		case "service.name":
			for j, a := range attrs {
				var f *[]string
				var expr string
				switch a.Op {
				case "=":
					expr = "ServiceName = @%[1]s"
					f = &ors
				case "!=":
					expr = "ServiceName != @%[1]s"
					f = &ands
				case "~":
					expr = "match(ServiceName, @%[1]s)"
					f = &ors
				case "!~":
					expr = "NOT match(ServiceName, @%[1]s)"
					f = &ands
				default:
					continue
				}
				v := fmt.Sprintf("service_name_%d_%d", i, j)
				*f = append(*f, fmt.Sprintf(expr, v))
				args = append(args, clickhouse.Named(v, a.Value))
			}
		default:
			for j, a := range attrs {
				var f *[]string
				var expr string
				switch a.Op {
				case "=":
					expr = "(LogAttributes[@%[1]s] = @%[2]s OR ResourceAttributes[@%[1]s] = @%[2]s)"
					f = &ors
				case "!=":
					expr = "(LogAttributes[@%[1]s] != @%[2]s AND ResourceAttributes[@%[1]s] != @%[2]s)"
					f = &ands
				case "~":
					expr = "(match(LogAttributes[@%[1]s], @%[2]s) OR match(ResourceAttributes[@%[1]s], @%[2]s))"
					f = &ors
				case "!~":
					expr = "(NOT match(LogAttributes[@%[1]s], @%[2]s) AND NOT match(ResourceAttributes[@%[1]s], @%[2]s))"
					f = &ands
				default:
					continue
				}
				n := fmt.Sprintf("attr_name_%d_%d", i, j)
				v := fmt.Sprintf("attr_values_%d_%d", i, j)
				*f = append(*f, fmt.Sprintf(expr, n, v))
				args = append(args, clickhouse.Named(n, name))
				args = append(args, clickhouse.Named(v, a.Value))
			}
		}
		if len(ands) > 0 {
			where = append(where, "("+strings.Join(ands, " AND ")+")")
		}
		if len(ors) > 0 {
			where = append(where, "("+strings.Join(ors, " OR ")+")")
		}
		i++
	}

	if len(message) > 0 {
		if expr := messageTokensExpr(utils.Uniq(message), "token", &args); expr != "" {
			where = append(where, expr)
		}
	}
	for k, tokens := range notMessage {
		if expr := messageTokensExpr(utils.Uniq(tokens), fmt.Sprintf("not_token_%d", k), &args); expr != "" {
			where = append(where, fmt.Sprintf("NOT (%s)", expr))
		}
	}

	return where, args
}

func messageTokensExpr(tokens []string, prefix string, args *[]any) string {
	var ands []string
	for i, m := range tokens {
		set := utils.NewStringSet(m, strings.ToLower(m), strings.ToUpper(m), strings.Title(m))
		var ors []string
		for j, s := range set.Items() {
			name := fmt.Sprintf("%s_%d_%d", prefix, i, j)
			ors = append(ors, fmt.Sprintf("hasToken(Body, @%s)", name))
			*args = append(*args, clickhouse.Named(name, s))
		}
		if len(ors) > 0 {
			ands = append(ands, fmt.Sprintf("(%s)", strings.Join(ors, " OR ")))
		}
	}
	return strings.Join(ands, " AND ")
}
