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

func TestChunkV1(t *testing.T) {
	tmp, err := os.MkdirTemp(os.TempDir(), "")
	require.NoError(t, err)
	defer os.RemoveAll(tmp)

	chunkPath := path.Join(tmp, "chunk.db")

	nan := timeseries.NaN
	ts := timeseries.NewWithData(0, 30, []float64{nan, 1, nan, nan, nan, nan, nan, nan, 2, nan})

	err = WriteV1(chunkPath, 0, 10, 30, false, []model.MetricValues{
		{Labels: model.Labels{"a": "b"}, LabelsHash: 123, Values: ts},
		{Labels: model.Labels{"a": "c"}, LabelsHash: 321, Values: ts},
	})
	require.NoError(t, err)

	meta, err := ReadMeta(chunkPath)
	require.NoError(t, err)
	assert.Equal(t, Meta{Path: chunkPath, From: 0, PointsCount: 10, Step: 30, Finalized: false}, *meta)

	res := map[uint64]model.MetricValues{}
	require.NoError(t, Read(chunkPath, 60, 10, 30, res))

	assert.Equal(t, model.Labels{"a": "b"}, res[123].Labels)
	assert.Equal(t, model.Labels{"a": "c"}, res[321].Labels)
	assert.Equal(t,
		"TimeSeries(60, 10, 30, [. . . . . . 2 . . .])",
		res[123].Values.String(),
	)
}

func TestChunkV2(t *testing.T) {
	tmp, err := os.MkdirTemp(os.TempDir(), "")
	require.NoError(t, err)
	defer os.RemoveAll(tmp)

	chunk1 := path.Join(tmp, "chunk1.db")
	chunk2 := path.Join(tmp, "chunk2.db")

	nan := timeseries.NaN
	data := []float64{nan, 1, nan, nan, 2, nan, nan, nan, 3, nan}
	err = WriteV2(chunk1, 0, 10, 30, false, []model.MetricValues{
		{Labels: model.Labels{"a": "bb"}, LabelsHash: 111, Values: timeseries.NewWithData(0, 30, data)},
		{Labels: model.Labels{"a": "dddd"}, LabelsHash: 333, Values: timeseries.NewWithData(0, 30, data)},
	})
	require.NoError(t, err)
	err = WriteV2(chunk2, 300, 10, 30, false, []model.MetricValues{
		{Labels: model.Labels{"a": "bb"}, LabelsHash: 111, Values: timeseries.NewWithData(300, 30, data)},
		{Labels: model.Labels{"a": "ccc"}, LabelsHash: 222, Values: timeseries.NewWithData(300, 30, data)},
		{Labels: model.Labels{"a": "dddd"}, LabelsHash: 333, Values: timeseries.NewWithData(300, 30, data)},
	})
	require.NoError(t, err)

	meta1, err := ReadMeta(chunk1)
	require.NoError(t, err)
	assert.Equal(t, Meta{Path: chunk1, From: 0, PointsCount: 10, Step: 30, Finalized: false}, *meta1)
	meta2, err := ReadMeta(chunk2)
	require.NoError(t, err)
	assert.Equal(t, Meta{Path: chunk2, From: 300, PointsCount: 10, Step: 30, Finalized: false}, *meta2)

	res := map[uint64]model.MetricValues{}
	require.NoError(t, Read(chunk1, 60, 10, 30, res))
	require.NoError(t, Read(chunk2, 60, 10, 30, res))

	assert.Equal(t, model.Labels{"a": "bb"}, res[111].Labels)
	assert.Equal(t, model.Labels{"a": "ccc"}, res[222].Labels)
	assert.Equal(t, model.Labels{"a": "dddd"}, res[333].Labels)
	assert.Equal(t,
		"TimeSeries(60, 10, 30, [. . 2 . . . 3 . . 1])",
		res[111].Values.String(),
	)
}
