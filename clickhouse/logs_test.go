package clickhouse

import (
	"context"
	"strings"
	"testing"
	"time"

	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLogConn struct {
	rows      []fakeLogRow
	lastQuery string
}

type fakeLogRow struct {
	serviceName        string
	timestamp          time.Time
	severity           int64
	body               string
	traceId            string
	resourceAttributes map[string]string
	logAttributes      map[string]string
}

func (c *fakeLogConn) Contributors() []string                                            { return nil }
func (c *fakeLogConn) ServerVersion() (*chdriver.ServerVersion, error)                   { return nil, nil }
func (c *fakeLogConn) Select(context.Context, interface{}, string, ...interface{}) error { return nil }
func (c *fakeLogConn) Query(_ context.Context, query string, _ ...interface{}) (chdriver.Rows, error) {
	c.lastQuery = query
	rows := append([]fakeLogRow(nil), c.rows...)
	if strings.Contains(query, "ORDER BY Timestamp DESC") {
		for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
			rows[i], rows[j] = rows[j], rows[i]
		}
	}
	if strings.HasSuffix(query, "LIMIT 2") && len(rows) > 2 {
		rows = rows[:2]
	}
	return &fakeRows{rows: rows}, nil
}
func (c *fakeLogConn) QueryRow(context.Context, string, ...interface{}) chdriver.Row {
	return fakeRow{}
}
func (c *fakeLogConn) PrepareBatch(context.Context, string) (chdriver.Batch, error) { return nil, nil }
func (c *fakeLogConn) Exec(context.Context, string, ...interface{}) error           { return nil }
func (c *fakeLogConn) AsyncInsert(context.Context, string, bool) error              { return nil }
func (c *fakeLogConn) Ping(context.Context) error                                   { return nil }
func (c *fakeLogConn) Stats() chdriver.Stats                                        { return chdriver.Stats{} }
func (c *fakeLogConn) Close() error                                                 { return nil }

type fakeRows struct {
	rows []fakeLogRow
	idx  int
}

func (r *fakeRows) Next() bool {
	return r.idx < len(r.rows)
}

func (r *fakeRows) Scan(dest ...interface{}) error {
	row := r.rows[r.idx]
	r.idx++
	*(dest[0].(*string)) = row.serviceName
	*(dest[1].(*time.Time)) = row.timestamp
	*(dest[2].(*int64)) = row.severity
	*(dest[3].(*string)) = row.body
	*(dest[4].(*string)) = row.traceId
	*(dest[5].(*map[string]string)) = row.resourceAttributes
	*(dest[6].(*map[string]string)) = row.logAttributes
	return nil
}

func (r *fakeRows) ScanStruct(interface{}) error       { return nil }
func (r *fakeRows) ColumnTypes() []chdriver.ColumnType { return nil }
func (r *fakeRows) Totals(...interface{}) error        { return nil }
func (r *fakeRows) Columns() []string                  { return nil }
func (r *fakeRows) Close() error                       { return nil }
func (r *fakeRows) Err() error                         { return nil }

type fakeRow struct{}

func (fakeRow) Err() error                   { return nil }
func (fakeRow) Scan(...interface{}) error    { return nil }
func (fakeRow) ScanStruct(interface{}) error { return nil }

func TestGetLogsReturnsNewestEntriesFirst(t *testing.T) {
	conn := &fakeLogConn{
		rows: []fakeLogRow{
			{serviceName: "checkout", timestamp: time.Unix(10, 0), severity: 9, body: "oldest"},
			{serviceName: "checkout", timestamp: time.Unix(20, 0), severity: 9, body: "middle"},
			{serviceName: "checkout", timestamp: time.Unix(30, 0), severity: 9, body: "newest"},
		},
	}
	c := &Client{
		conn:    conn,
		project: &db.Project{Id: "cluster-1", Name: "cluster-a"},
	}

	res, err := c.GetLogs(context.Background(), LogQuery{
		Ctx:      timeseries.NewContext(0, 60, 15),
		Services: []string{"checkout"},
		Limit:    2,
	})
	require.NoError(t, err)
	require.Len(t, res, 2)

	assert.Contains(t, conn.lastQuery, "ORDER BY Timestamp DESC")
	assert.Equal(t, []string{"newest", "middle"}, []string{res[0].Body, res[1].Body})
	assert.True(t, res[0].Timestamp.After(res[1].Timestamp))
	assert.Equal(t, "cluster-1", res[0].ClusterId)
	assert.Equal(t, "cluster-a", res[0].ClusterName)
}
