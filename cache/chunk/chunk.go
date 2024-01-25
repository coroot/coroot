package chunk

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"unsafe"

	lz4 "github.com/DataDog/golz4"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

const (
	V1 uint8 = 1
	V2 uint8 = 2
	V3 uint8 = 3
)

type Meta struct {
	Path        string
	From        timeseries.Time
	PointsCount uint32
	Step        timeseries.Duration
	Finalized   bool
}

func (m *Meta) To() timeseries.Time {
	return m.From.Add(timeseries.Duration(m.PointsCount-1) * m.Step)
}

type metricMeta struct {
	Hash       uint64
	MetaOffset uint32
	MetaSize   uint32
}

const metricMetaSize = 16

type header struct {
	Version     uint8
	From        timeseries.Time
	PointsCount uint32
	Step        timeseries.Duration
	Finalized   bool

	DataSizeOrMetricsCount uint32
}

const headerSize = 26

func Write(f io.Writer, from timeseries.Time, pointsCount int, step timeseries.Duration, finalized bool, metrics []model.MetricValues) error {
	var err error
	h := header{
		Version:                V3,
		From:                   from,
		PointsCount:            uint32(pointsCount),
		Step:                   step,
		Finalized:              finalized,
		DataSizeOrMetricsCount: uint32(len(metrics)),
	}
	if err = binary.Write(f, binary.LittleEndian, h); err != nil {
		return err
	}

	zw := lz4.NewWriter(f)
	w := bufio.NewWriter(zw)

	var metaOffset, metaSize int
	buf := make([]float32, pointsCount)
	for i := range metrics {
		metaSize = 0
		for k, v := range metrics[i].Labels {
			metaSize += len(k) + len(v) + 2
		}
		m := metricMeta{
			Hash:       metrics[i].LabelsHash,
			MetaOffset: uint32(metaOffset),
			MetaSize:   uint32(metaSize),
		}
		metaOffset += metaSize
		if err = binary.Write(w, binary.LittleEndian, m); err != nil {
			return err
		}
		for i := range buf {
			buf[i] = timeseries.NaN
		}
		iter := metrics[i].Values.Iter()
		to := from.Add(timeseries.Duration(pointsCount-1) * step)
		for iter.Next() {
			t, v := iter.Value()
			if t > to {
				break
			}
			if t < from {
				continue
			}
			buf[int((t-from)/timeseries.Time(step))] = v
		}
		if _, err = w.Write(asBytes32(buf)); err != nil {
			return err
		}
	}

	if err = writeLabelsV2(metrics, w); err != nil {
		return nil
	}

	if err = w.Flush(); err != nil {
		return err
	}
	if err = zw.Close(); err != nil {
		return err
	}
	return nil
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
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	h := header{}
	if err := binary.Read(reader, binary.LittleEndian, &h); err != nil {
		return err
	}
	switch h.Version {
	case V1:
		return readV1(reader, int(st.Size()), &h, from, pointsCount, step, dest)
	case V2:
		return readV2(reader, &h, from, pointsCount, step, dest)
	case V3:
		return readV3(reader, &h, from, pointsCount, step, dest)
	default:
		return fmt.Errorf("unknown version: %d", h.Version)
	}
}

func asBytes32(f []float32) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(&f[0])), len(f)*4)
}

func asBytes64(f []float64) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(&f[0])), len(f)*8)
}

func asFloats32(b []byte) []float32 {
	return unsafe.Slice((*float32)(unsafe.Pointer(&b[0])), len(b)/4)
}

func asFloats64(b []byte) []float64 {
	return unsafe.Slice((*float64)(unsafe.Pointer(&b[0])), len(b)/8)
}
