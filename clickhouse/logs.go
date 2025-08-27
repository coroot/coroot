package clickhouse

import (
	"context"
	"fmt"
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

func (c *Client) GetLogsHistogram(ctx context.Context, query LogQuery) ([]model.LogHistogramBucket, error) {
	where, args := query.filters(nil)
	q := fmt.Sprintf("SELECT multiIf(SeverityNumber=0, 0, intDiv(SeverityNumber, 4)+1), toStartOfInterval(Timestamp, INTERVAL %d second), count(1)", query.Ctx.Step)
	q += " FROM @@table_otel_logs@@"
	q += " WHERE " + strings.Join(where, " AND ")
	q += " GROUP BY 1, 2"
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bySeverity := map[int64]*timeseries.TimeSeries{}
	var sev int64
	var t time.Time
	var count uint64
	for rows.Next() {
		if err = rows.Scan(&sev, &t, &count); err != nil {
			return nil, err
		}
		if bySeverity[sev] == nil {
			bySeverity[sev] = timeseries.New(query.Ctx.From, query.Ctx.PointsCount(), query.Ctx.Step)
		}
		bySeverity[sev].Set(timeseries.Time(t.Unix()), float32(count))
	}
	res := make([]model.LogHistogramBucket, 0, len(bySeverity))
	for s, ts := range bySeverity {
		res = append(res, model.LogHistogramBucket{Severity: model.Severity(s), Timeseries: ts})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Severity < res[j].Severity })
	return res, nil
}

func (c *Client) GetLogs(ctx context.Context, query LogQuery) ([]*model.LogEntry, error) {
	where, args := query.filters(nil)
	q := "SELECT ServiceName, Timestamp, multiIf(SeverityNumber=0, 0, intDiv(SeverityNumber, 4)+1), Body, TraceId, ResourceAttributes, LogAttributes"
	q += " FROM @@table_otel_logs@@"
	q += " WHERE " + strings.Join(where, " AND ")
	q += " LIMIT " + fmt.Sprint(query.Limit)

	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*model.LogEntry
	for rows.Next() {
		var e model.LogEntry
		var sev int64
		if err = rows.Scan(&e.ServiceName, &e.Timestamp, &sev, &e.Body, &e.TraceId, &e.ResourceAttributes, &e.LogAttributes); err != nil {
			return nil, err
		}
		e.Severity = model.Severity(sev)
		res = append(res, &e)
	}
	return res, nil
}

func (c *Client) GetLogFilters(ctx context.Context, query LogQuery, name string) ([]string, error) {
	where, args := query.filters(&name)
	var q string
	var res []string
	switch name {
	case "":
		res = append(res, "Severity", "Message")
		q = "SELECT DISTINCT arrayJoin(arrayConcat(mapKeys(LogAttributes), mapKeys(ResourceAttributes)))"
	case "Severity":
		q = "SELECT DISTINCT multiIf(SeverityNumber=0, 0, intDiv(SeverityNumber, 4)+1)"
	case "Message":
		return res, nil
	default:
		q = "SELECT DISTINCT arrayJoin([LogAttributes[@attr], ResourceAttributes[@attr]])"
		args = append(args, clickhouse.Named("attr", name))
	}
	q += " FROM @@table_otel_logs@@"
	q += " WHERE " + strings.Join(where, " AND ")
	q += " ORDER BY 1 LIMIT 1000"
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var s string
	var i int64
	for rows.Next() {
		switch name {
		case "Severity":
			err = rows.Scan(&i)
			s = model.Severity(i).String()
		default:
			err = rows.Scan(&s)
		}
		if err != nil {
			return nil, err
		}
		if s == "" {
			continue
		}
		res = append(res, s)
	}
	return res, nil
}

func (c *Client) GetKubernetesEvents(ctx context.Context, from, to timeseries.Time, limit int) ([]*model.LogEntry, error) {
	q := LogQuery{
		Ctx:     timeseries.NewContext(from, to, 0),
		Filters: []LogFilter{{Name: "service.name", Op: "=", Value: "KubernetesEvents"}},
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
}

type LogFilter struct {
	Name  string `json:"name"`
	Op    string `json:"op"`
	Value string `json:"value"`
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

	where = append(where, "Timestamp BETWEEN @from AND @to")
	args = append(args,
		clickhouse.DateNamed("from", q.Ctx.From.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", q.Ctx.To.ToStandard(), clickhouse.NanoSeconds),
	)

	filters := utils.Uniq(q.Filters)
	var message []string
	byName := map[string][]LogFilter{}
	for _, f := range filters {
		if attr == nil && f.Name == "Message" {
			fields := strings.FieldsFunc(f.Value, func(r rune) bool {
				return unicode.IsSpace(r) || (r <= unicode.MaxASCII && !unicode.IsNumber(r) && !unicode.IsLetter(r))
			})
			message = append(message, fields...)
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
				r1, r2 := model.SeverityFromString(a.Value).Range()
				var f *[]string
				var expr string
				switch a.Op {
				case "=":
					expr = "SeverityNumber BETWEEN @%[1]s AND @%[2]s"
					f = &ors
				case "!=":
					expr = "SeverityNumber NOT BETWEEN @%[1]s AND @%[2]s"
					f = &ands
				default:
					continue
				}
				v1 := fmt.Sprintf("severity_from_%d", j)
				v2 := fmt.Sprintf("severity_to_%d", j)
				*f = append(*f, fmt.Sprintf(expr, v1, v2))
				args = append(args, clickhouse.Named(v1, r1))
				args = append(args, clickhouse.Named(v2, r2))
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
		message = utils.Uniq(message)
		var ands []string
		for i, m := range message {
			set := utils.NewStringSet(m, strings.ToLower(m), strings.ToUpper(m), strings.Title(m))
			var ors []string
			for j, s := range set.Items() {
				name := fmt.Sprintf("token_%d_%d", i, j)
				ors = append(ors, fmt.Sprintf("hasToken(Body, @%s)", name))
				args = append(args, clickhouse.Named(name, s))
			}
			if len(ors) > 0 {
				ands = append(ands, fmt.Sprintf("(%s)", strings.Join(ors, " OR ")))
			}
		}
		if len(ands) > 0 {
			where = append(where, strings.Join(ands, " AND "))
		}
	}

	return where, args
}
