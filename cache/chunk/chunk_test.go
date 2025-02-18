package chunk

import (
	"os"
	"path"
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunk(t *testing.T) {
	tmp, err := os.MkdirTemp(os.TempDir(), "")
	require.NoError(t, err)
	defer os.RemoveAll(tmp)

	chunk1 := path.Join(tmp, "chunk1.db")
	chunk2 := path.Join(tmp, "chunk2.db")
	f1, err := os.Create(chunk1)
	require.NoError(t, err)
	f2, err := os.Create(chunk2)
	require.NoError(t, err)

	nan := timeseries.NaN
	data := []float32{nan, 1, nan, nan, 2, nan, nan, nan, 3, nan}
	err = Write(f1, 0, 10, 30, false, []*model.MetricValues{
		{Labels: model.Labels{"a": "bb"}, LabelsHash: 111, Values: timeseries.NewWithData(0, 30, data)},
		{Labels: model.Labels{"a": "dddd"}, LabelsHash: 333, Values: timeseries.NewWithData(0, 30, data)},
	})
	require.NoError(t, err)
	err = Write(f2, 300, 10, 30, false, []*model.MetricValues{
		{Labels: model.Labels{"a": "bb"}, LabelsHash: 111, Values: timeseries.NewWithData(300, 30, data)},
		{Labels: model.Labels{"a": "ccc"}, LabelsHash: 222, Values: timeseries.NewWithData(300, 30, data)},
		{Labels: model.Labels{"a": "dddd"}, LabelsHash: 333, Values: timeseries.NewWithData(300, 30, data)},
	})
	require.NoError(t, err)

	meta1, err := ReadMeta(chunk1)
	require.NoError(t, err)
	assert.Equal(t, Meta{Path: chunk1, From: 0, PointsCount: 10, Step: 30, Finalized: false, Created: meta1.Created}, *meta1)
	meta2, err := ReadMeta(chunk2)
	require.NoError(t, err)
	assert.Equal(t, Meta{Path: chunk2, From: 300, PointsCount: 10, Step: 30, Finalized: false, Created: meta2.Created}, *meta2)

	res := map[uint64]*model.MetricValues{}
	require.NoError(t, Read(chunk1, 60, 10, 30, res, timeseries.FillAny))
	require.NoError(t, Read(chunk2, 60, 10, 30, res, timeseries.FillAny))

	assert.Equal(t, model.Labels{"a": "bb"}, res[111].Labels)
	assert.Equal(t, model.Labels{"a": "ccc"}, res[222].Labels)
	assert.Equal(t, model.Labels{"a": "dddd"}, res[333].Labels)
	assert.Equal(t,
		"TimeSeries(60, 10, 30, [. . 2 . . . 3 . . 1])",
		res[111].Values.String(),
	)
}
