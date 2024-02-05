package chunk

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/buger/jsonparser"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/pierrec/lz4"
)

func readV1(reader io.Reader, chunkSize int, header *header, from timeseries.Time, pointsCount int, step timeseries.Duration, dest map[uint64]model.MetricValues) error {
	compressed := make([]byte, header.DataSizeOrMetricsCount)
	if _, err := io.ReadFull(reader, compressed); err != nil {
		return err
	}
	l := binary.LittleEndian.Uint32(compressed)
	if l == 0 {
		return nil
	}
	data := make([]byte, l)
	if err := decompress(compressed, data); err != nil {
		return nil
	}
	dataReader := bytes.NewReader(data)
	dataBuf := make([]float32, header.PointsCount)
	var meta []byte
	for {
		m := metricMeta{}
		if err := binary.Read(dataReader, binary.LittleEndian, &m); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		offset, _ := dataReader.Seek(0, io.SeekCurrent)
		dataReader.Seek(8*int64(header.PointsCount), io.SeekCurrent)
		mv, ok := dest[m.Hash]
		if !ok {
			mv.Values = timeseries.New(from, pointsCount, step)
		}

		for i, v := range asFloats64(data[offset : offset+int64(header.PointsCount)*8]) {
			dataBuf[i] = float32(v)
		}
		if !mv.Values.Fill(header.From, header.Step, dataBuf) && !ok {
			mv.Values = nil
			continue
		}
		if ok {
			continue
		}
		mv.LabelsHash = m.Hash
		if meta == nil {
			compressed = make([]byte, chunkSize-headerSize-int(header.DataSizeOrMetricsCount))
			if _, err := io.ReadFull(reader, compressed); err != nil {
				return nil
			}
			l := binary.LittleEndian.Uint32(compressed)
			if l == 0 {
				return nil
			}
			meta = make([]byte, l)
			if err := decompress(compressed, meta); err != nil {
				return err
			}
		}
		mv.Labels = make(model.Labels, 20)
		if err := readLabelsV1(meta[m.MetaOffset:m.MetaOffset+m.MetaSize], mv.Labels); err != nil {
			return err
		}
		dest[mv.LabelsHash] = mv
	}
	return nil
}

func readLabelsV1(src []byte, dst model.Labels) error {
	return jsonparser.ObjectEach(src, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		v, err := jsonparser.ParseString(value)
		if err != nil {
			return err
		}
		dst[string(key)] = v
		return nil
	})
}

func decompress(src, dst []byte) error {
	_, err := lz4.UncompressBlock(src[4:], dst)
	return err
}
