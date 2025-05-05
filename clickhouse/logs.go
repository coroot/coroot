package clickhouse

import (
	"context"
	"fmt"
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

func (c *Client) GetLogsHistogram(ctx context.Context, query LogQuery) (map[string]*timeseries.TimeSeries, error) {
	where, args := query.filters(nil)
	q := fmt.Sprintf("SELECT SeverityText, toStartOfInterval(Timestamp, INTERVAL %d second), count(1)", query.Ctx.Step)
	q += " FROM @@table_otel_logs@@"
	q += " WHERE " + strings.Join(where, " AND ")
	q += " GROUP BY 1, 2"
	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := map[string]*timeseries.TimeSeries{}
	var sev string
	var ts time.Time
	var count uint64
	for rows.Next() {
		if err = rows.Scan(&sev, &ts, &count); err != nil {
			return nil, err
		}
		if res[sev] == nil {
			res[sev] = timeseries.New(query.Ctx.From, query.Ctx.PointsCount(), query.Ctx.Step)
		}
		res[sev].Set(timeseries.Time(ts.Unix()), float32(count))
	}
	return res, nil
}

func (c *Client) GetLogs(ctx context.Context, query LogQuery) ([]*model.LogEntry, error) {
	where, args := query.filters(nil)
	q := "SELECT Timestamp, SeverityText, Body, TraceId, ResourceAttributes, LogAttributes"
	q += " FROM @@table_otel_logs@@"
	q += " WHERE " + strings.Join(where, " AND ")
	q += " ORDER BY Timestamp DESC LIMIT " + fmt.Sprint(query.Limit)

	rows, err := c.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*model.LogEntry
	for rows.Next() {
		var e model.LogEntry
		if err = rows.Scan(&e.Timestamp, &e.Severity, &e.Body, &e.TraceId, &e.ResourceAttributes, &e.LogAttributes); err != nil {
			return nil, err
		}
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
		q = "SELECT DISTINCT SeverityText"
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
	var a string
	for rows.Next() {
		if err = rows.Scan(&a); err != nil {
			return nil, err
		}
		if a == "" {
			continue
		}
		res = append(res, a)
	}
	return res, nil
}

type LogQuery struct {
	Ctx      timeseries.Context
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
		return where, args
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
		for j, a := range attrs {
			var f *[]string
			var expr string
			switch a.Op {
			case "=":
				if name == "Severity" {
					expr = "SeverityText = @%[2]s"
				} else {
					expr = "(LogAttributes[@%[1]s] = @%[2]s OR ResourceAttributes[@%[1]s] = @%[2]s)"
				}
				f = &ors
			case "!=":
				if name == "Severity" {
					expr = "SeverityText != @%[2]s"
				} else {
					expr = "(LogAttributes[@%[1]s] != @%[2]s AND ResourceAttributes[@%[1]s] != @%[2]s)"
				}
				f = &ands
			case "~":
				if name == "Severity" {
					expr = "match(SeverityText, @%[2]s)"
				} else {
					expr = "(match(LogAttributes[@%[1]s], @%[2]s) OR match(ResourceAttributes[@%[1]s], @%[2]s))"
				}
				f = &ors
			case "!~":
				if name == "Severity" {
					expr = "NOT match(SeverityText,  @%[2]s)"
				} else {
					expr = "(NOT match(LogAttributes[@%[1]s], @%[2]s) AND NOT match(ResourceAttributes[@%[1]s], @%[2]s))"
				}
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
