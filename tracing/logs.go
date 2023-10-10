package tracing

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/timeseries"
	"strings"
	"time"
)

const (
	MinLogsHistogramStep = timeseries.Minute
)

func (c *ClickhouseClient) GetServiceNamesFromLogs(ctx context.Context) (map[string][]string, error) {
	q := "SELECT DISTINCT ServiceName, SeverityText"
	q += " FROM otel_logs_histogram"
	rows, err := c.conn.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
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

func (c *ClickhouseClient) GetServiceLogsHistogram(ctx context.Context, tsCtx timeseries.Context, service string, severities []string) (map[string]*timeseries.TimeSeries, error) {
	return c.getLogsHistogram(ctx, tsCtx, severities,
		`
			ServiceName = @serviceName
		`,
		clickhouse.Named("serviceName", service),
	)
}

func (c *ClickhouseClient) GetServiceLogs(ctx context.Context, tsCtx timeseries.Context, service string, severities []string, patternHash []string, search string, limit int) ([]*LogEntry, error) {
	return c.getLogs(ctx, tsCtx, severities, patternHash, search, limit,
		`
			ServiceName = @serviceName
		`,
		clickhouse.Named("serviceName", service),
	)
}

func (c *ClickhouseClient) GetContainerLogsHistogram(ctx context.Context, tsCtx timeseries.Context, containerIds []string, severities []string) (map[string]*timeseries.TimeSeries, error) {
	return c.getLogsHistogram(ctx, tsCtx, severities,
		`
			ServiceName = 'coroot-node-agent' AND
			ContainerId IN (@containerIds)
		`,
		clickhouse.Named("containerIds", containerIds),
	)
}

func (c *ClickhouseClient) GetContainerLogs(ctx context.Context, tsCtx timeseries.Context, containerIds []string, severities []string, patternHashes []string, search string, limit int) ([]*LogEntry, error) {
	return c.getLogs(ctx, tsCtx, severities, patternHashes, search, limit,
		`
			ServiceName = 'coroot-node-agent' AND
			LogAttributes['container.id'] IN (@containerIds)
		`,
		clickhouse.Named("containerIds", containerIds),
	)
}

func (c *ClickhouseClient) getLogsHistogram(ctx context.Context, tsCtx timeseries.Context, severities []string, filter string, filterArgs ...any) (map[string]*timeseries.TimeSeries, error) {
	var filters []string
	var args []any

	filters = append(filters, "Timestamp BETWEEN @from AND @to")
	args = append(args,
		clickhouse.DateNamed("from", tsCtx.From.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", tsCtx.To.ToStandard(), clickhouse.NanoSeconds),
	)
	if len(severities) > 0 {
		filters = append(filters, "SeverityText IN (@severityText)")
		args = append(args,
			clickhouse.Named("severityText", severities),
		)
	}
	if filter != "" {
		filters = append(filters, filter)
		args = append(args, filterArgs...)
	}

	tsField := "Timestamp"
	if tsCtx.Step > MinLogsHistogramStep {
		tsField = fmt.Sprintf("toStartOfInterval(Timestamp, INTERVAL %d minute)", tsCtx.Step/timeseries.Minute)
	}
	q := fmt.Sprintf("SELECT SeverityText, %s, sum(EventsCount)", tsField)
	q += " FROM otel_logs_histogram"
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
			res[sev] = timeseries.New(tsCtx.From, int(tsCtx.To.Sub(tsCtx.From)/tsCtx.Step), tsCtx.Step)
		}
		res[sev].Set(timeseries.Time(ts.Unix()), float32(count))
	}
	return res, nil
}

func (c *ClickhouseClient) getLogs(ctx context.Context, tsCtx timeseries.Context, severities []string, patternHashes []string, search string, limit int, filter string, filterArgs ...any) ([]*LogEntry, error) {
	var filters []string
	var args []any

	filters = append(filters, "Timestamp BETWEEN @from AND @to")
	args = append(args,
		clickhouse.DateNamed("from", tsCtx.From.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", tsCtx.To.ToStandard(), clickhouse.NanoSeconds),
	)
	if len(patternHashes) > 0 {
		filters = append(filters, "LogAttributes['pattern.hash'] IN (@patternHash)")
		args = append(args,
			clickhouse.Named("patternHash", patternHashes),
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
		q := "SELECT Timestamp, SeverityText, Body, ResourceAttributes, LogAttributes"
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
		if err = rows.Scan(&e.Timestamp, &e.Severity, &e.Body, &e.ResourceAttributes, &e.LogAttributes); err != nil {
			return nil, err
		}
		res = append(res, &e)
	}
	return res, nil
}
