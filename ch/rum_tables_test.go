package ch

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func substituteRumDDL(query, cluster string, rumTTL, rumReplayTTL, rumAggTTL int) string {
	query = strings.ReplaceAll(query, "@ttl_rum_replay", fmt.Sprintf("%d", rumReplayTTL))
	query = strings.ReplaceAll(query, "@ttl_rum_agg", fmt.Sprintf("%d", rumAggTTL))
	query = strings.ReplaceAll(query, "@ttl_rum", fmt.Sprintf("%d", rumTTL))
	if cluster != "" {
		query = strings.ReplaceAll(query, "@merge_tree", "ReplicatedMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')")
		query = strings.ReplaceAll(query, "@replacing_merge_tree", "ReplicatedReplacingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')")
		query = strings.ReplaceAll(query, "@summing_merge_tree", "ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')")
		query = strings.ReplaceAll(query, "@aggregating_merge_tree", "ReplicatedAggregatingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')")
		query = strings.ReplaceAll(query, "@on_cluster", "ON CLUSTER "+cluster)
		query = strings.ReplaceAll(query, "@cluster", cluster)
	} else {
		query = strings.ReplaceAll(query, "@merge_tree", "MergeTree()")
		query = strings.ReplaceAll(query, "@replacing_merge_tree", "ReplacingMergeTree()")
		query = strings.ReplaceAll(query, "@summing_merge_tree", "SummingMergeTree()")
		query = strings.ReplaceAll(query, "@aggregating_merge_tree", "AggregatingMergeTree()")
		query = strings.ReplaceAll(query, "@on_cluster", "")
	}
	return query
}

func TestRumTablesDDLPlaceholders(t *testing.T) {
	const (
		rumTTL       = 604800
		rumReplayTTL = 259200
		rumAggTTL    = 2592000
	)
	for _, cluster := range []string{"", "coroot_ch"} {
		t.Run("cluster="+cluster, func(t *testing.T) {
			combined := strings.Join(rumTables, "\n")
			out := substituteRumDDL(combined, cluster, rumTTL, rumReplayTTL, rumAggTTL)
			assert.NotContains(t, out, "@ttl_rum_replay")
			assert.NotContains(t, out, "@ttl_rum_agg")
			assert.NotContains(t, out, "@ttl_rum")
			assert.NotContains(t, out, "@merge_tree")
			assert.NotContains(t, out, "@aggregating_merge_tree")
			assert.NotContains(t, out, "@on_cluster")
			assert.NotContains(t, out, "@")
			assert.Contains(t, out, fmt.Sprintf("%d", rumReplayTTL))
		})
	}
}

func TestRumTableNamesInDDL(t *testing.T) {
	combined := strings.Join(rumTables, "\n")
	for _, name := range rumTableNames {
		assert.Contains(t, combined, name, "rumTables should reference %q", name)
	}
}

func TestRumDistributedTables(t *testing.T) {
	require.Len(t, rumDistributedTables, len(rumTableNames))
	for _, name := range rumTableNames {
		needle := name + "_distributed"
		found := false
		for _, ddl := range rumDistributedTables {
			if strings.Contains(ddl, needle) {
				found = true
				break
			}
		}
		assert.True(t, found, "missing distributed table for %q", name)
	}

	combined := strings.Join(rumDistributedTables, "\n")
	out := substituteRumDDL(combined, "coroot_ch", 604800, 259200, 2592000)
	assert.NotContains(t, out, "@cluster")
	assert.NotContains(t, out, "@")
}
