package chunk

import (
	"encoding/binary"
	"io"

	lz4 "github.com/DataDog/golz4"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func readV2(reader io.Reader, header *header, from timeseries.Time, pointsCount int, step timeseries.Duration, dest map[uint64]model.MetricValues) error {
	r := lz4.NewDecompressReader(reader)
	defer r.Close()
	buf := make([]byte, metricMetaSize+8*header.PointsCount)
	dataBuf := make([]float32, header.PointsCount)
	var labelsToRead []*metricMeta
	var maxLabelSize uint32
	var err error
	for i := uint32(0); i < header.DataSizeOrMetricsCount; i++ {
		if _, err = io.ReadFull(r, buf); err != nil {
			return err
		}
		m := metricMeta{
			Hash:       binary.LittleEndian.Uint64(buf),
			MetaOffset: binary.LittleEndian.Uint32(buf[8:]),
			MetaSize:   binary.LittleEndian.Uint32(buf[12:]),
		}
		mv, ok := dest[m.Hash]
		if !ok {
			mv.Values = timeseries.New(from, pointsCount, step)
			labelsToRead = append(labelsToRead, &m)
			if m.MetaSize > maxLabelSize {
				maxLabelSize = m.MetaSize
			}
		}
		for i, v := range asFloats64(buf[metricMetaSize:]) {
			dataBuf[i] = float32(v)
		}
		if !mv.Values.Fill(header.From, header.Step, dataBuf) && !ok {
			continue
		}
		dest[m.Hash] = mv
	}
	if len(labelsToRead) > 0 {
		buf = make([]byte, maxLabelSize)
		offset := uint32(0)
		for _, m := range labelsToRead {
			mv, ok := dest[m.Hash]
			if !ok {
				continue
			}
			toSkip := m.MetaOffset - offset
			if toSkip > 0 {
				if _, err := io.CopyN(io.Discard, r, int64(toSkip)); err != nil {
					return err
				}
			}
			if _, err = io.ReadFull(r, buf[:m.MetaSize]); err != nil {
				return err
			}
			offset = m.MetaOffset + m.MetaSize
			mv.LabelsHash = m.Hash
			mv.Labels = make(model.Labels, 20)
			readLabelsV2(buf[:m.MetaSize], mv.Labels)
			dest[m.Hash] = mv
		}
	}
	return nil
}

func writeLabelsV2(metrics []model.MetricValues, dst io.Writer) error {
	var err error
	for _, m := range metrics {
		for k, v := range m.Labels {
			if _, err = dst.Write(append([]byte(k), byte(0))); err != nil {
				return err
			}
			if _, err = dst.Write(append([]byte(v), byte(0))); err != nil {
				return err
			}
		}
	}
	return nil
}

func readLabelsV2(src []byte, dst model.Labels) {
	var key []byte
	isValue := false
	f := 0
	for i, b := range src {
		if b != 0 {
			continue
		}
		if isValue {
			dst[string(key)] = string(src[f:i])
			isValue = false
		} else {
			key = src[f:i]
			isValue = true
		}
		f = i + 1
	}
}
