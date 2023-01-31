package chunk

import (
	"bufio"
	"encoding/binary"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/pierrec/lz4"
	"io/ioutil"
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

const headerSize = 26

func Write(path string, from timeseries.Time, pointsCount int, step timeseries.Duration, finalized bool, metrics []model.MetricValues) error {
	dir, file := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	e := &Encoder{}
	if err := e.encode(from, pointsCount, step, metrics); err != nil {
		return err
	}
	defer e.close()
	h := header{
		Version:     version,
		From:        from,
		PointsCount: uint32(pointsCount),
		Step:        step,
		Finalized:   finalized,
		ValuesSize:  uint32(len(e.valuesData)),
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

func decompress(src, dst []byte) error {
	_, err := lz4.UncompressBlock(src[4:], dst)
	return err
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

	decoder, err := newDecoder(bufio.NewReader(f), int(st.Size()))
	if err != nil {
		return err
	}
	defer decoder.close()
	return decoder.decode(from, pointsCount, step, dest)
}
