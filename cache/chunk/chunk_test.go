package chunk

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path"
	"testing"
)

func TestChunk(t *testing.T) {
	tmp, err := os.MkdirTemp(os.TempDir(), "")
	require.NoError(t, err)
	defer os.RemoveAll(tmp)

	chunkPath := path.Join(tmp, "chunk.db")

	ts := timeseries.New(0, 10, 30)
	ts.Set(180, 0.)

	err = Write(chunkPath, 0, 10, 30, false, []model.MetricValues{
		{Labels: model.Labels{"a": "b"}, LabelsHash: 123, Values: ts},
		{Labels: model.Labels{"a": "c"}, LabelsHash: 321, Values: ts},
	})
	require.NoError(t, err)

	meta, err := ReadMeta(chunkPath)
	require.NoError(t, err)
	assert.Equal(t, Meta{Path: chunkPath, From: 0, PointsCount: 10, Step: 30, Finalized: false}, *meta)

	res := map[uint64]model.MetricValues{}
	require.NoError(t, Read(chunkPath, 0, 8, 30, res))

	assert.Equal(t, model.Labels{"a": "b"}, res[123].Labels)
	assert.Equal(t, model.Labels{"a": "c"}, res[321].Labels)
	assert.Equal(t,
		"InMemoryTimeSeries(0, 8, 30, [. . . . . . 0 .])",
		res[123].Values.String(),
	)
}
