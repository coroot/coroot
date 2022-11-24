package chunk

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"github.com/buger/jsonparser"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	pool "github.com/libp2p/go-buffer-pool"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
)

const version uint8 = 1

type Meta struct {
	Path        string
	From        timeseries.Time
	PointsCount uint32
	Step        timeseries.Duration
	Finalized   bool
}

type metric struct {
	Hash       uint64
	MetaOffset uint32
	MetaSize   uint32
}

type header struct {
	Version     uint8
	From        timeseries.Time
	PointsCount uint32
	Step        timeseries.Duration
	Finalized   bool
	ValuesSize  uint32
}

func Write(path string, from timeseries.Time, pointsCount int, step timeseries.Duration, finalized bool, metrics []model.MetricValues) error {
	dir, file := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	values, meta, err := marshall(from, pointsCount, step, metrics)
	if err != nil {
		return err
	}
	defer pool.Put(values)
	defer pool.Put(meta)

	h := header{
		Version:     version,
		From:        from,
		PointsCount: uint32(pointsCount),
		Step:        step,
		Finalized:   finalized,
		ValuesSize:  uint32(len(values)),
	}
	f, err := ioutil.TempFile(dir, file)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	if err = binary.Write(f, binary.LittleEndian, h); err != nil {
		return err
	}
	if _, err = f.Write(values); err != nil {
		return err
	}
	if _, err = f.Write(meta); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func marshall(from timeseries.Time, pointsCount int, step timeseries.Duration, metrics []model.MetricValues) ([]byte, []byte, error) {
	valuesBuf := &bytes.Buffer{}
	metaBuf := &bytes.Buffer{}
	var err error
	offset := uint32(0)
	for _, m := range metrics {
		j, err := json.Marshal(m.Labels)
		if err != nil {
			return nil, nil, err
		}
		l := uint32(len(j))
		v := metric{
			Hash:       m.LabelsHash,
			MetaOffset: offset,
			MetaSize:   l,
		}
		offset += l
		if err = binary.Write(valuesBuf, binary.LittleEndian, v); err != nil {
			return nil, nil, err
		}
		ts := timeseries.New(from, pointsCount, step)
		ts.CopyFrom(m.Values)
		data := ts.Data()
		if err = binary.Write(valuesBuf, binary.LittleEndian, data); err != nil {
			return nil, nil, err
		}
		if _, err = metaBuf.Write(j); err != nil {
			return nil, nil, err
		}
	}
	values, err := compress(valuesBuf.Bytes())
	if err != nil {
		return nil, nil, err
	}
	meta, err := compress(metaBuf.Bytes())
	if err != nil {
		return nil, nil, err
	}
	return values, meta, err
}

func ReadMeta(path string) (*Meta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := header{}
	if err = binary.Read(f, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	return &Meta{Path: path, From: h.From, PointsCount: h.PointsCount, Step: h.Step, Finalized: h.Finalized}, err
}

func Read(path string, from timeseries.Time, pointsCount int, step timeseries.Duration, dest map[uint64]model.MetricValues) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := bufio.NewReader(f)

	h := &header{}
	if err = binary.Read(reader, binary.LittleEndian, h); err != nil {
		return err
	}
	compressed := pool.Get(int(h.ValuesSize))
	defer pool.Put(compressed)

	if _, err := io.ReadFull(reader, compressed); err != nil {
		return err
	}
	values, err := decompress(compressed)
	if err != nil {
		return err
	}
	defer pool.Put(values)

	valuesReader := bytes.NewReader(values)
	var meta []byte

	data := make([]float64, h.PointsCount)
	to := from.Add(timeseries.Duration(pointsCount-1) * step)
	for {
		m := metric{}
		if err = binary.Read(valuesReader, binary.LittleEndian, &m); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		offset, _ := valuesReader.Seek(0, io.SeekCurrent)
		readFloats(values[offset:], data)
		valuesReader.Seek(8*int64(h.PointsCount), io.SeekCurrent)

		mv, ok := dest[m.Hash]
		if ok {
			var t, tNext timeseries.Time
			for i, v := range data {
				t = h.From.Add(timeseries.Duration(i) * h.Step)
				if t < from || t > to {
					continue
				}
				if t < tNext {
					continue
				}
				tNext = mv.Values.Set(t, v).Add(step)
			}
			continue
		}

		var t, tNext timeseries.Time
		for i, v := range data {
			t = h.From.Add(timeseries.Duration(i) * h.Step)
			if t < from || t > to {
				continue
			}
			if t < tNext {
				continue
			}
			if math.IsNaN(v) {
				continue
			}
			if mv.Values == nil {
				mv.Values = timeseries.New(from, pointsCount, step)
			}
			tNext = mv.Values.Set(t, v).Add(step)
		}
		if mv.Values == nil {
			continue
		}
		mv.LabelsHash = m.Hash
		if meta == nil {
			compressed, err := io.ReadAll(reader)
			if err != nil {
				return nil
			}
			if meta, err = decompress(compressed); err != nil {
				return err
			}
			defer pool.Put(meta)
		}
		mv.Labels = model.Labels{}
		err := jsonparser.ObjectEach(meta[int(m.MetaOffset):int(m.MetaOffset)+int(m.MetaSize)], func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			v, err := jsonparser.ParseString(value)
			if err != nil {
				return err
			}
			mv.Labels[string(key)] = v
			return nil
		})
		if err != nil {
			return err
		}
		dest[mv.LabelsHash] = mv
	}
	return nil
}

func readFloats(src []byte, dst []float64) {
	for i := range dst {
		dst[i] = math.Float64frombits(binary.LittleEndian.Uint64(src[i*8:]))
	}
}
