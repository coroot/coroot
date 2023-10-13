package tracing

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/timeseries"
	"golang.org/x/exp/maps"
	"sort"
	"strings"
	"time"
)

func (c *ClickhouseClient) GetServicesFromLogs(ctx context.Context) (map[string][]string, error) {
	q := "SELECT DISTINCT ServiceName, SeverityText"
	q += " FROM " + c.config.LogsTable
	rows, err := c.conn.Query(ctx, q)
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

func (c *ClickhouseClient) GetServiceLogsHistogram(ctx context.Context, from, to timeseries.Time, step timeseries.Duration, service string, severities []string) (map[string]*timeseries.TimeSeries, error) {
	return c.getLogsHistogram(ctx, from, to, step, []string{service}, severities, "")
}

func (c *ClickhouseClient) GetServiceLogs(ctx context.Context, from, to timeseries.Time, service string, severities []string, search string, limit int) ([]*LogEntry, error) {
	return c.getLogs(ctx, from, to, service, severities, nil, search, limit, "")
}

func (c *ClickhouseClient) GetContainerLogsHistogram(ctx context.Context, from, to timeseries.Time, step timeseries.Duration, containers map[string][]string, severities []string) (map[string]*timeseries.TimeSeries, error) {
	return c.getLogsHistogram(ctx, from, to, step, maps.Keys(containers), severities, "")
}

func (c *ClickhouseClient) GetContainerLogs(ctx context.Context, from, to timeseries.Time, containers map[string][]string, severities []string, hashes []string, search string, limit int) ([]*LogEntry, error) {
	byService := map[string][]*LogEntry{}
	for service, ids := range containers {
		entries, err := c.getLogs(ctx, from, to, service, severities, hashes, search, limit,
			"ResourceAttributes['container.id'] IN (@containerId)", clickhouse.Named("containerId", ids),
		)
		if err != nil {
			return nil, err
		}
		if len(containers) == 1 {
			return entries, nil
		}
		byService[service] = entries
	}
	var res []*LogEntry
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

func (c *ClickhouseClient) getLogsHistogram(ctx context.Context, from, to timeseries.Time, step timeseries.Duration, services []string, severities []string, filter string, filterArgs ...any) (map[string]*timeseries.TimeSeries, error) {
	var filters []string
	var args []any
	filters = append(filters, "ServiceName IN @serviceName")
	args = append(args, clickhouse.Named("serviceName", services))
	filters = append(filters, "Timestamp BETWEEN @from AND @to")
	args = append(args,
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
	)
	if len(severities) > 0 {
		filters = append(filters, "SeverityText IN (@severityText)")
		args = append(args, clickhouse.Named("severityText", severities))
	}
	if filter != "" {
		filters = append(filters, filter)
		args = append(args, filterArgs...)
	}

	q := fmt.Sprintf("SELECT SeverityText, toStartOfInterval(Timestamp, INTERVAL %d second), count(1)", step)
	q += " FROM " + c.config.LogsTable
	q += " WHERE " + strings.Join(filters, " AND ")
	q += " GROUP BY 1, 2"
	rows, err := c.conn.Query(ctx, q, args...)
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

func (c *ClickhouseClient) getLogs(ctx context.Context, from, to timeseries.Time, service string, severities []string, hashes []string, search string, limit int, filter string, filterArgs ...any) ([]*LogEntry, error) {
	if len(severities) == 0 {
		return nil, nil
	}

	var filters []string
	var args []any
	filters = append(filters, "ServiceName = @serviceName")
	args = append(args, clickhouse.Named("serviceName", service))
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
		filters = append(filters, "Body ILIKE @search")
		args = append(args,
			clickhouse.Named("search", "%"+search+"%"),
		)
	}

	if filter != "" {
		filters = append(filters, filter)
		args = append(args, filterArgs...)
	}

	var qs []string
	for _, severity := range severities {
		q := "SELECT Timestamp, SeverityText, Body, TraceId, ResourceAttributes, LogAttributes"
		q += " FROM " + c.config.LogsTable
		q += " WHERE " + strings.Join(append(filters, fmt.Sprintf("SeverityText = '%s'", severity)), " AND ")
		q += " ORDER BY toUnixTimestamp(Timestamp) DESC LIMIT " + fmt.Sprint(limit)
		qs = append(qs, q)
	}
	q := "SELECT *"
	q += " FROM (" + strings.Join(qs, " UNION ALL ") + ") l"
	q += " ORDER BY Timestamp DESC LIMIT " + fmt.Sprint(limit)

	rows, err := c.conn.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []*LogEntry
	for rows.Next() {
		var e LogEntry
		if err = rows.Scan(&e.Timestamp, &e.Severity, &e.Body, &e.TraceId, &e.ResourceAttributes, &e.LogAttributes); err != nil {
			return nil, err
		}
		res = append(res, &e)
	}
	return res, nil
}
