package chunk

import (
	"bytes"
	"encoding/binary"
	"os"
	"path"
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunk(t *testing.T) {
	tmp := t.TempDir()

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

func TestReadCorruptedBlockMeta(t *testing.T) {
	metrics := []*model.MetricValues{
		{Labels: model.Labels{"a": "b"}, LabelsHash: 111, Values: timeseries.NewWithData(0, 30, []float32{1, 2, 3})},
	}
	var valid bytes.Buffer
	require.NoError(t, Write(&valid, 0, 3, 30, false, metrics))

	var h header
	require.NoError(t, binary.Read(bytes.NewReader(valid.Bytes()), binary.LittleEndian, &h))
	blockMetaOffset := binary.Size(h) + len(metrics)*8

	tests := []struct {
		name   string
		offset int
	}{
		{name: "CompressedTooLarge", offset: blockMetaOffset + 4},
		{name: "DecompressedTooLarge", offset: blockMetaOffset},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corrupted := bytes.Clone(valid.Bytes())
			binary.LittleEndian.PutUint32(corrupted[tt.offset:], ^uint32(0))

			p := path.Join(t.TempDir(), "corrupted.db")
			require.NoError(t, os.WriteFile(p, corrupted, 0o600))

			var err error
			require.NotPanics(t, func() {
				err = Read(p, 0, 3, 30, map[uint64]*model.MetricValues{}, timeseries.FillAny)
			})
			require.ErrorIs(t, err, ErrCorrupted)
		})
	}
}
