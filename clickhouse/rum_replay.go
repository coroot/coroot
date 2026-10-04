package clickhouse

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/timeseries"
)

type RumReplaySegment struct {
	Timestamp time.Time
	SessionId string
	TraceId   string
	Seq       uint32
	Payload   string
}

// GetRumReplaySegments loads replay blobs for a session (EE; caller must enforce RBAC).
func (c *Client) GetRumReplaySegments(ctx context.Context, service, sessionId string, from, to timeseries.Time, limit int) ([]RumReplaySegment, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	q := `
SELECT Timestamp, SessionId, TraceId, Seq, Payload
FROM @@table_rum_replay_segments@@
WHERE SessionId = @sid
  AND (@svc = '' OR ServiceName = @svc)
  AND Timestamp BETWEEN @from AND @to
ORDER BY Seq, Timestamp
LIMIT @limit`
	rows, err := c.Query(ctx, q,
		clickhouse.Named("sid", sessionId),
		clickhouse.Named("svc", service),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.Named("limit", limit),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []RumReplaySegment
	for rows.Next() {
		var s RumReplaySegment
		if err = rows.Scan(&s.Timestamp, &s.SessionId, &s.TraceId, &s.Seq, &s.Payload); err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	return res, nil
}
