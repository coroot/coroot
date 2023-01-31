package chunk

import (
	"bytes"
	"encoding/binary"
	"github.com/buger/jsonparser"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/pool"
	"github.com/coroot/coroot/timeseries"
	"io"
	"math"
)

type Decoder struct {
	reader io.Reader
	size   int
	header *header
	meta   []byte
}

func newDecoder(reader io.Reader, size int) (*Decoder, error) {
	d := &Decoder{reader: reader, header: &header{}, size: size}
	if err := binary.Read(d.reader, binary.LittleEndian, d.header); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Decoder) close() {
	if d.meta != nil {
		pool.PutByteArray(d.meta)
	}
}

func (d *Decoder) decode(from timeseries.Time, pointsCount int, step timeseries.Duration, dest map[uint64]model.MetricValues) error {
	compressedValue := pool.GetByteArray(int(d.header.ValuesSize))
	defer pool.PutByteArray(compressedValue)
	if _, err := io.ReadFull(d.reader, compressedValue); err != nil {
		return err
	}

	l := binary.LittleEndian.Uint32(compressedValue)
	if l == 0 {
		return nil
	}
	decompressed := pool.GetByteArray(int(l))
	defer pool.PutByteArray(decompressed)
	if err := decompress(compressedValue, decompressed); err != nil {
		return nil
	}
	valuesReader := bytes.NewReader(decompressed)

	data := make([]float64, int(d.header.PointsCount))

	for {
		m := metric{}
		if err := binary.Read(valuesReader, binary.LittleEndian, &m); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		offset, _ := valuesReader.Seek(0, io.SeekCurrent)
		readFloats(decompressed[offset:], data)
		valuesReader.Seek(8*int64(d.header.PointsCount), io.SeekCurrent)
		mv, ok := dest[m.Hash]
		if !ok {
			mv.Values = timeseries.New(from, pointsCount, step)
		}
		if !mv.Values.Fill(d.header.From, d.header.Step, data) && !ok {
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
	res := model.Labels{}
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
	compressed := pool.GetByteArray(d.size - headerSize - int(d.header.ValuesSize))
	defer pool.PutByteArray(compressed)
	if _, err := io.ReadFull(d.reader, compressed); err != nil {
		return nil
	}
	l := binary.LittleEndian.Uint32(compressed)
	if l == 0 {
		return nil
	}
	d.meta = pool.GetByteArray(int(l))
	return decompress(compressed, d.meta)
}

func readFloats(src []byte, dst []float64) {
	for i := range dst {
		dst[i] = math.Float64frombits(binary.LittleEndian.Uint64(src[i*8:]))
	}
}
