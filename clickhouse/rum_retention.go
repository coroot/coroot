package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

// RumRetentionTTLs are effective TTLs for each RUM data tier.
type RumRetentionTTLs struct {
	Raw        timeseries.Duration
	Replay     timeseries.Duration
	Aggregates timeseries.Duration
}

// EffectiveRumRetention merges project overrides with global defaults.
func EffectiveRumRetention(project *db.Project, global config.Rum) RumRetentionTTLs {
	out := RumRetentionTTLs{
		Raw:        global.TTL,
		Replay:     global.ReplayTTL,
		Aggregates: global.AggregatesTTL,
	}
	if out.Raw <= 0 {
		out.Raw = 7 * timeseries.Day
	}
	if out.Replay <= 0 {
		out.Replay = out.Raw
	}
	if out.Aggregates <= 0 {
		out.Aggregates = 30 * timeseries.Day
	}
	if project != nil && project.Settings.Rum != nil && project.Settings.Rum.Retention != nil {
		r := project.Settings.Rum.Retention
		if r.RawTTL > 0 {
			out.Raw = r.RawTTL
		}
		if r.ReplayTTL > 0 {
			out.Replay = r.ReplayTTL
		}
		if r.AggregatesTTL > 0 {
			out.Aggregates = r.AggregatesTTL
		}
	}
	if out.Aggregates < out.Raw {
		out.Aggregates = out.Raw
	}
	return out
}

var rumRawTables = []string{"rum_spans", "rum_events"}
var rumReplayTables = []string{"rum_replay_segments"}
var rumAggTables = []string{
	"rum_spans_histogram",
	"rum_web_vitals_hist",
	"rum_service_name",
	"rum_service_edges",
	"rum_page_stats_1m",
	"rum_events_1m",
}

func rumTableTTL(table string, ttls RumRetentionTTLs) timeseries.Duration {
	for _, t := range rumRawTables {
		if t == table {
			return ttls.Raw
		}
	}
	for _, t := range rumReplayTables {
		if t == table {
			return ttls.Replay
		}
	}
	for _, t := range rumAggTables {
		if t == table {
			return ttls.Aggregates
		}
	}
	return 0
}

func allRumTables() []string {
	out := make([]string, 0, len(rumRawTables)+len(rumReplayTables)+len(rumAggTables))
	out = append(out, rumRawTables...)
	out = append(out, rumReplayTables...)
	out = append(out, rumAggTables...)
	return out
}

// RumTableCleanupPriority returns cleanup priority for space manager (lower = drop first).
func RumTableCleanupPriority(table string) int {
	for _, t := range rumReplayTables {
		if t == table {
			return 0
		}
	}
	for _, t := range rumRawTables {
		if t == table {
			return 1
		}
	}
	if strings.HasPrefix(table, "rum_") {
		return 2
	}
	return 3
}

type rumTableTTLInfo struct {
	Name       string
	TTLSeconds uint64
	Exists     bool
}

func (c *Client) getRumTableTTLs(ctx context.Context) ([]rumTableTTLInfo, error) {
	tables := allRumTables()
	q := `
SELECT name, extract(
    create_table_query,
    'TTL .+\\+ (INTERVAL \\d+ [A-Z]+|toInterval\\w+\\(\\d+\\))'
) AS ttl_expr
FROM system.tables
WHERE database = currentDatabase()
  AND name IN ?`
	rows, err := c.conn.Query(ctx, q, tables)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byName := map[string]rumTableTTLInfo{}
	for rows.Next() {
		var name string
		var ttlExpr *string
		if err := rows.Scan(&name, &ttlExpr); err != nil {
			return nil, err
		}
		info := rumTableTTLInfo{Name: name, Exists: true}
		if ttlExpr != nil && *ttlExpr != "" {
			info.TTLSeconds = parseTTLToSeconds(*ttlExpr)
		}
		byName[name] = info
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var res []rumTableTTLInfo
	for _, t := range tables {
		if info, ok := byName[t]; ok {
			res = append(res, info)
		}
	}
	return res, nil
}

func ttlNeedsModify(currentSeconds uint64, want timeseries.Duration) bool {
	if want <= 0 {
		return false
	}
	return currentSeconds != uint64(want)
}

func (c *Client) modifyRumTableTTL(ctx context.Context, table string, ttl timeseries.Duration, useLastSeen bool) error {
	col := "Timestamp"
	if useLastSeen {
		col = "LastSeen"
	}
	q := fmt.Sprintf(
		"ALTER TABLE %s MODIFY TTL toDateTime(%s) + toIntervalSecond(%d) SETTINGS materialize_ttl_after_modify = 0",
		table, col, int64(ttl),
	)
	return c.conn.Exec(ctx, q)
}

type rumPartition struct {
	Table       string
	PartitionId string
	MaxDate     time.Time
}

func (c *Client) getExpiredRumPartitions(ctx context.Context, ttls RumRetentionTTLs, now time.Time) ([]rumPartition, error) {
	q := `
SELECT
    table,
    partition_id,
    max(max_time) AS max_t
FROM system.parts
WHERE active = 1
  AND database = currentDatabase()
  AND table IN ?
  AND max_time > 0
GROUP BY table, partition_id
ORDER BY table, max_t`
	rows, err := c.conn.Query(ctx, q, allRumTables())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expired []rumPartition
	for rows.Next() {
		var p rumPartition
		if err := rows.Scan(&p.Table, &p.PartitionId, &p.MaxDate); err != nil {
			return nil, err
		}
		ttl := rumTableTTL(p.Table, ttls)
		if ttl <= 0 {
			continue
		}
		cutoff := now.Add(-ttl.ToStandard())
		// Drop whole day partitions whose newest data is older than cutoff.
		if p.MaxDate.Before(cutoff) {
			expired = append(expired, p)
		}
	}
	return expired, rows.Err()
}

func (c *Client) dropRumPartition(ctx context.Context, p rumPartition) error {
	q := fmt.Sprintf("ALTER TABLE %s DROP PARTITION ID '%s'", p.Table, p.PartitionId)
	return c.conn.Exec(ctx, q)
}

// ApplyRumRetention updates ClickHouse TTLs and drops expired RUM partitions for one project.
func ApplyRumRetention(ctx context.Context, client *Client, project *db.Project, global config.Rum) error {
	if client == nil {
		return nil
	}
	ttls := EffectiveRumRetention(project, global)
	infos, err := client.getRumTableTTLs(ctx)
	if err != nil {
		return fmt.Errorf("list rum table ttls: %w", err)
	}
	for _, info := range infos {
		want := rumTableTTL(info.Name, ttls)
		if !ttlNeedsModify(info.TTLSeconds, want) {
			continue
		}
		useLastSeen := info.Name == "rum_service_name"
		klog.Infof("updating TTL for %s to %s", info.Name, want.ShortString())
		if err := client.modifyRumTableTTL(ctx, info.Name, want, useLastSeen); err != nil {
			klog.Errorf("failed to modify TTL for %s: %v", info.Name, err)
			continue
		}
	}
	expired, err := client.getExpiredRumPartitions(ctx, ttls, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("list expired rum partitions: %w", err)
	}
	for _, p := range expired {
		klog.Infof("dropping expired RUM partition %s from %s", p.PartitionId, p.Table)
		if err := client.dropRumPartition(ctx, p); err != nil {
			klog.Errorf("failed to drop partition %s from %s: %v", p.PartitionId, p.Table, err)
		}
	}
	return nil
}

// RunRumRetentionForProjects applies RUM retention for every non-multicluster project.
func RunRumRetentionForProjects(ctx context.Context, projects []*db.Project, globalClickHouse *db.IntegrationClickhouse, global config.Rum) error {
	for _, project := range projects {
		if project.Multicluster() {
			continue
		}
		if err := ApplyRumRetentionForProject(ctx, project, globalClickHouse, global); err != nil {
			klog.Errorf("rum retention for project %s: %v", project.Id, err)
		}
	}
	return nil
}

// ApplyRumRetentionForProject opens a ClickHouse client and applies retention for one project.
func ApplyRumRetentionForProject(ctx context.Context, project *db.Project, globalClickHouse *db.IntegrationClickhouse, global config.Rum) error {
	cfg := project.ClickHouseConfig(globalClickHouse)
	if cfg == nil {
		return nil
	}
	client, err := NewClient(cfg, project)
	if err != nil {
		return err
	}
	defer client.Close()
	return ApplyRumRetention(ctx, client, project, global)
}
