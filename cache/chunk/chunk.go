package chunk

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"github.com/DataDog/golz4"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"unsafe"
)

const (
	V1 uint8 = 1
	V2 uint8 = 2
)

type Meta struct {
	Path        string
	From        timeseries.Time
	PointsCount uint32
	Step        timeseries.Duration
	Finalized   bool
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

func WriteV1(path string, from timeseries.Time, pointsCount int, step timeseries.Duration, finalized bool, metrics []model.MetricValues) error {
	dir, file := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	e := &Encoder{}
	if err := e.encode(from, pointsCount, step, metrics); err != nil {
		return err
	}
	h := header{
		Version:                V1,
		From:                   from,
		PointsCount:            uint32(pointsCount),
		Step:                   step,
		Finalized:              finalized,
		DataSizeOrMetricsCount: uint32(len(e.valuesData)),
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
	if _, err = f.Write(e.valuesData); err != nil {
		return err
	}
	if _, err = f.Write(e.metaData); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func WriteV2(path string, from timeseries.Time, pointsCount int, step timeseries.Duration, finalized bool, metrics []model.MetricValues) error {
	dir, file := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	f, err := ioutil.TempFile(dir, file)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	h := header{
		Version:                V2,
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
	buf := make([]float64, pointsCount)
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
		if _, err = w.Write(asBytes(buf)); err != nil {
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
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
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
	default:
		return fmt.Errorf("unknown version: %d", h.Version)
	}
}

func readV1(reader io.Reader, size int, header *header, from timeseries.Time, pointsCount int, step timeseries.Duration, dest map[uint64]model.MetricValues) error {
	decoder, err := newDecoder(reader, size, header)
	if err != nil {
		return err
	}
	return decoder.decode(from, pointsCount, step, dest)
}

func readV2(reader io.Reader, header *header, from timeseries.Time, pointsCount int, step timeseries.Duration, dest map[uint64]model.MetricValues) error {
	r := lz4.NewDecompressReader(reader)
	defer r.Close()
	buf := make([]byte, metricMetaSize+8*header.PointsCount)
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
		if !mv.Values.Fill(header.From, header.Step, asFloats(buf[metricMetaSize:])) && !ok {
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

func asBytes(f []float64) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(&f[0])), len(f)*8)
}

func asFloats(b []byte) []float64 {
	return unsafe.Slice((*float64)(unsafe.Pointer(&b[0])), len(b)/8)
}
