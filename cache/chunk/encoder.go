package chunk

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/pierrec/lz4"
)

type Encoder struct {
	valuesData []byte
	metaData   []byte
}

func (e *Encoder) compressValues(src []byte) error {
	l := lz4.CompressBlockBound(len(src))
	e.valuesData = make([]byte, l+4)
	binary.LittleEndian.PutUint32(e.valuesData, uint32(len(src)))
	n, err := lz4.CompressBlock(src, e.valuesData[4:], nil)
	e.valuesData = e.valuesData[:n+4]
	return err
}

func (e *Encoder) compressMeta(src []byte) error {
	l := lz4.CompressBlockBound(len(src))
	e.metaData = make([]byte, l+4)
	binary.LittleEndian.PutUint32(e.metaData, uint32(len(src)))
	n, err := lz4.CompressBlock(src, e.metaData[4:], nil)
	e.metaData = e.metaData[:n+4]
	return err
}

func (e *Encoder) encode(from timeseries.Time, pointsCount int, step timeseries.Duration, metrics []model.MetricValues) error {
	buf := make([]byte, len(metrics)*(16+pointsCount*8))
	valuesBuf := bytes.NewBuffer(buf)
	valuesBuf.Reset()
	metaBuf := &bytes.Buffer{}
	var err error
	offset := uint32(0)
	tmp := make([]float64, pointsCount)

	for _, m := range metrics {
		j, err := json.Marshal(m.Labels)
		if err != nil {
			return err
		}
		l := uint32(len(j))
		v := metricMeta{
			Hash:       m.LabelsHash,
			MetaOffset: offset,
			MetaSize:   l,
		}
		offset += l
		if err = binary.Write(valuesBuf, binary.LittleEndian, v); err != nil {
			return err
		}
		for i := range tmp {
			tmp[i] = timeseries.NaN
		}
		iter := m.Values.Iter()
		to := from.Add(timeseries.Duration(pointsCount-1) * step)
		for iter.Next() {
			t, vv := iter.Value()
			if t > to {
				break
			}
			if t < from {
				continue
			}
			idx := int((t - from) / timeseries.Time(step))
			tmp[idx] = vv
		}
		if _, err = valuesBuf.Write(asBytes(tmp)); err != nil {
			return err
		}
		if _, err = metaBuf.Write(j); err != nil {
			return err
		}
	}
	if err = e.compressValues(valuesBuf.Bytes()); err != nil {
		return err
	}
	if err = e.compressMeta(metaBuf.Bytes()); err != nil {
		return err
	}
	return err
}
