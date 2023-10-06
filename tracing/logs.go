package tracing

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"strings"
)

var (
	severityToNumber = map[model.LogSeverity][]int{
		model.LogSeverityUnknown:  {0},
		model.LogSeverityDebug:    {1, 2, 3, 4, 5, 6, 7, 8},
		model.LogSeverityInfo:     {9, 10, 11, 12},
		model.LogSeverityWarning:  {13, 14, 15, 16},
		model.LogSeverityError:    {17, 18, 19, 20},
		model.LogSeverityCritical: {21, 22, 23, 24},
	}
	severityNumbers = 0
)

func init() {
	for _, nums := range severityToNumber {
		severityNumbers += len(nums)
	}
}

func (c *ClickhouseClient) GetServiceNamesFromLogs(ctx context.Context) ([]string, error) {
	q := "SELECT DISTINCT ServiceName"
	q += " FROM " + c.config.LogsTable
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

func (c *ClickhouseClient) GetServiceLogs(ctx context.Context, from, to timeseries.Time, service string, severity []model.LogSeverity, patternHash []string, search string, limit int) ([]*LogEntry, error) {
	return c.getLogs(ctx, from, to, severity, patternHash, search, limit,
		`
			ServiceName = @serviceName
		`,
		clickhouse.Named("serviceName", service),
	)
}

func (c *ClickhouseClient) GetContainerLogs(ctx context.Context, from, to timeseries.Time, containerIds []string, severity []model.LogSeverity, patternHash []string, search string, limit int) ([]*LogEntry, error) {
	return c.getLogs(ctx, from, to, severity, patternHash, search, limit,
		`
			ServiceName = 'coroot-node-agent' AND
			LogAttributes['container.id'] IN (@containerIds)
		`,
		clickhouse.Named("containerIds", containerIds),
	)
}

func (c *ClickhouseClient) getLogs(ctx context.Context, from, to timeseries.Time, severity []model.LogSeverity, patternHash []string, search string, limit int, filter string, filterArgs ...any) ([]*LogEntry, error) {
	var filters []string
	var args []any

	filters = append(filters, "Timestamp BETWEEN @from AND @to")
	args = append(args,
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
	)

	if len(severity) > 0 {
		var nums []int
		for _, s := range severity {
			nums = append(nums, severityToNumber[s]...)
		}
		if len(nums) < severityNumbers {
			filters = append(filters, "SeverityNumber IN (@severityNumbers)")
			args = append(args,
				clickhouse.Named("severityNumbers", nums),
			)
		}
	}

	if len(patternHash) > 0 {
		filters = append(filters, "LogAttributes['pattern.hash'] IN (@patternHash)")
		args = append(args,
			clickhouse.Named("patternHash", patternHash),
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

	q := "SELECT Timestamp, SeverityText, Body, ResourceAttributes, LogAttributes"
	q += " FROM " + c.config.LogsTable
	q += " WHERE " + strings.Join(filters, " AND ")
	q += " ORDER BY Timestamp DESC"
	q += " LIMIT " + fmt.Sprint(limit)

	rows, err := c.conn.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
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
