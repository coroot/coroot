package cache

import (
	"bytes"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestChunk(t *testing.T) {
	tmp, err := os.MkdirTemp(os.TempDir(), "")
	require.NoError(t, err)
	defer os.RemoveAll(tmp)

	ts := timeseries.NewNan(timeseries.Context{From: 0, To: 600, Step: 30})
	ts.Set(180, 0.)
	b := &bytes.Buffer{}
	chunk := NewChunk(0, 180, 600, 30, b)

	err = chunk.WriteMetric(model.MetricValues{
		Labels:     model.Labels{"a": "b"},
		LabelsHash: 123,
		Values:     ts,
	})
	require.NoError(t, err)

	cache := &Cache{
		cfg:       Config{Path: tmp},
		byProject: map[db.ProjectId]map[string]*queryData{},
	}
	assert.NoError(t, cache.saveChunk("abc1234", "queryhash", chunk))

	cache.byProject = map[db.ProjectId]map[string]*queryData{}
	assert.NoError(t, cache.initCacheIndexFromDir())

	byProject := cache.byProject["abc1234"]
	var chunkMeta *ChunkMeta
	for _, chunkMeta = range byProject["queryhash"].chunksOnDisk {
		break
	}
	assert.Equal(t, timeseries.Time(0), chunkMeta.startTs)
	assert.Equal(t, timeseries.Duration(600), chunkMeta.duration)
	assert.Equal(t, timeseries.Duration(30), chunkMeta.step)
	assert.Equal(t, timeseries.Time(180), chunkMeta.lastTs)

	chunkFromDisk, err := OpenChunk(chunkMeta)
	assert.NoError(t, err)

	res := map[uint64]model.MetricValues{}
	assert.NoError(t, chunkFromDisk.ReadMetrics(0, 600, 30, res))

	assert.Equal(t, model.Labels{"a": "b"}, res[123].Labels)
	assert.Equal(t,
		"InMemoryTimeSeries(0, 600, 30, [. . . . . . 0 . . . . . . . . . . . . . .])",
		res[123].Values.String(),
	)
}
