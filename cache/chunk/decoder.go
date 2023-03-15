package chunk

import (
	"bytes"
	"encoding/binary"
	"github.com/buger/jsonparser"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/pierrec/lz4"
	"io"
)

type Decoder struct {
	reader io.Reader
	size   int
	header *header
	meta   []byte
}

func newDecoder(reader io.Reader, size int, header *header) (*Decoder, error) {
	d := &Decoder{reader: reader, header: header, size: size}
	return d, nil
}

func (d *Decoder) decode(from timeseries.Time, pointsCount int, step timeseries.Duration, dest map[uint64]model.MetricValues) error {
	compressed := make([]byte, d.header.DataSizeOrMetricsCount)
	if _, err := io.ReadFull(d.reader, compressed); err != nil {
		return err
	}
	l := binary.LittleEndian.Uint32(compressed)
	if l == 0 {
		return nil
	}
	decompressed := make([]byte, l)
	if err := decompress(compressed, decompressed); err != nil {
		return nil
	}
	valuesReader := bytes.NewReader(decompressed)

	for {
		m := metricMeta{}
		if err := binary.Read(valuesReader, binary.LittleEndian, &m); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		offset, _ := valuesReader.Seek(0, io.SeekCurrent)
		valuesReader.Seek(8*int64(d.header.PointsCount), io.SeekCurrent)
		mv, ok := dest[m.Hash]
		if !ok {
			mv.Values = timeseries.New(from, pointsCount, step)
		}
		if !mv.Values.Fill(d.header.From, d.header.Step, asFloats(decompressed[offset:offset+int64(d.header.PointsCount)*8])) && !ok {
			mv.Values = nil
			continue
		}
		if ok {
			continue
		}
		mv.LabelsHash = m.Hash
		var err error
		mv.Labels, err = d.readMetricLabels(int(m.MetaOffset), int(m.MetaSize))
		if err != nil {
			return err
		}
		dest[mv.LabelsHash] = mv
	}
	return nil
}

func (d *Decoder) readMetricLabels(offset, size int) (model.Labels, error) {
	if d.meta == nil {
		if err := d.readMetadata(); err != nil {
			return nil, err
		}
	}
	res := make(model.Labels, 20)
	err := jsonparser.ObjectEach(d.meta[offset:offset+size], func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		v, err := jsonparser.ParseString(value)
		if err != nil {
			return err
		}
		res[string(key)] = v
		return nil
	})
	return res, err
}

func (d *Decoder) readMetadata() error {
	compressed := make([]byte, d.size-headerSize-int(d.header.DataSizeOrMetricsCount))
	if _, err := io.ReadFull(d.reader, compressed); err != nil {
		return nil
	}
	l := binary.LittleEndian.Uint32(compressed)
	if l == 0 {
		return nil
	}
	d.meta = make([]byte, l)
	return decompress(compressed, d.meta)
}

func decompress(src, dst []byte) error {
	_, err := lz4.UncompressBlock(src[4:], dst)
	return err
}
