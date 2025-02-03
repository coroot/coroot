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
	"golang.org/x/exp/maps"
)

func (c *Client) GetServicesFromLogs(ctx context.Context, from timeseries.Time) (map[string][]string, error) {
	rows, err := c.Query(ctx, "SELECT DISTINCT ServiceName, SeverityText FROM @@table_otel_logs_service_name_severity_text@@ WHERE LastSeen >= @from",
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := map[string][]string{}
	var app, sev string
	for rows.Next() {
		if err = rows.Scan(&app, &sev); err != nil {
			return nil, err
		}
		res[app] = append(res[app], sev)
	}
	return res, nil
}

func (c *Client) GetServiceLogsHistogram(ctx context.Context, from, to timeseries.Time, step timeseries.Duration, service string, severities []string, search string) (map[string]*timeseries.TimeSeries, error) {
	filters, args := logFilters(from, to, []string{service}, severities, nil, search)
	return c.getLogsHistogram(ctx, filters, args, from, to, step)
}

func (c *Client) GetServiceLogs(ctx context.Context, from, to timeseries.Time, service string, severities []string, search string, limit int) ([]*model.LogEntry, error) {
	filters, args := logFilters(from, to, []string{service}, severities, nil, search)
	return c.getLogs(ctx, filters, args, severities, limit)
}

func (c *Client) GetContainerLogsHistogram(ctx context.Context, from, to timeseries.Time, step timeseries.Duration, containers map[string][]string, severities []string, hashes []string, search string) (map[string]*timeseries.TimeSeries, error) {
	services := maps.Keys(containers)
	filters, args := logFilters(from, to, services, severities, hashes, search)
	return c.getLogsHistogram(ctx, filters, args, from, to, step)
}

func (c *Client) GetContainerLogs(ctx context.Context, from, to timeseries.Time, containers map[string][]string, severities []string, hashes []string, search string, limit int) ([]*model.LogEntry, error) {
	byService := map[string][]*model.LogEntry{}
	for service, ids := range containers {
		filters, args := logFilters(from, to, []string{service}, nil, hashes, search)
		filters = append(filters, "ResourceAttributes['container.id'] IN (@containerId)")
		args = append(args, clickhouse.Named("containerId", ids))
		entries, err := c.getLogs(ctx, filters, args, severities, limit)
		if err != nil {
			return nil, err
		}
		if len(containers) == 1 {
			return entries, nil
		}
		byService[service] = entries
	}
	var res []*model.LogEntry
	for _, entries := range byService {
		res = append(res, entries...)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Timestamp.After(res[j].Timestamp)
	})
	if len(res) > limit {
		return res[:limit], nil
	}
	return res, nil
}

func (c *Client) getLogsHistogram(ctx context.Context, filters []string, args []any, from, to timeseries.Time, step timeseries.Duration) (map[string]*timeseries.TimeSeries, error) {
	q := fmt.Sprintf("SELECT SeverityText, toStartOfInterval(Timestamp, INTERVAL %d second), count(1)", step)
	q += " FROM @@table_otel_logs@@"
	q += " WHERE " + strings.Join(filters, " AND ")
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
			res[sev] = timeseries.New(from, int(to.Sub(from)/step), step)
		}
		res[sev].Set(timeseries.Time(ts.Unix()), float32(count))
	}
	return res, nil
}

func (c *Client) getLogs(ctx context.Context, filters []string, args []any, severities []string, limit int) ([]*model.LogEntry, error) {
	if len(severities) == 0 {
		return nil, nil
	}

	var qs []string
	for _, severity := range severities {
		q := "SELECT Timestamp, SeverityText, Body, TraceId, ResourceAttributes, LogAttributes"
		q += " FROM @@table_otel_logs@@"
		q += " WHERE " + strings.Join(append(filters, fmt.Sprintf("SeverityText = '%s'", severity)), " AND ")
		q += " ORDER BY toUnixTimestamp(Timestamp) DESC LIMIT " + fmt.Sprint(limit)
		qs = append(qs, q)
	}
	q := "SELECT *"
	q += " FROM (" + strings.Join(qs, " UNION ALL ") + ") l"
	q += " ORDER BY Timestamp DESC LIMIT " + fmt.Sprint(limit)

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

func logFilters(from, to timeseries.Time, services []string, severities []string, hashes []string, search string) ([]string, []any) {
	var filters []string
	var args []any

	if len(services) == 1 {
		filters = append(filters, "ServiceName = @serviceName")
		args = append(args, clickhouse.Named("serviceName", services[0]))
	} else {
		filters = append(filters, "ServiceName IN (@serviceName)")
		args = append(args, clickhouse.Named("serviceName", services))
	}

	if len(severities) > 0 {
		filters = append(filters, "SeverityText IN (@severityText)")
		args = append(args, clickhouse.Named("severityText", severities))
	}

	filters = append(filters, "Timestamp BETWEEN @from AND @to")
	args = append(args,
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
	)

	if len(hashes) > 0 {
		filters = append(filters, "LogAttributes['pattern.hash'] IN (@patternHash)")
		args = append(args,
			clickhouse.Named("patternHash", hashes),
		)
	}
	if len(search) > 0 {
		fields := strings.FieldsFunc(search, func(r rune) bool {
			return unicode.IsSpace(r) || (r <= unicode.MaxASCII && !unicode.IsNumber(r) && !unicode.IsLetter(r))
		})
		if len(fields) > 0 {
			var ands []string
			for i, f := range fields {
				set := utils.NewStringSet(f, strings.ToLower(f), strings.ToUpper(f), strings.Title(f))
				var ors []string
				for j, s := range set.Items() {
					name := fmt.Sprintf("token_%d_%d", i, j)
					ors = append(ors, fmt.Sprintf("hasToken(Body, @%s)", name))
					args = append(args, clickhouse.Named(name, s))
				}
				ands = append(ands, fmt.Sprintf("(%s)", strings.Join(ors, " OR ")))
			}
			filters = append(filters, strings.Join(ands, " AND "))
		}
	}
	return filters, args
}
