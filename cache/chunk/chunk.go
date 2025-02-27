package chunk

import (
	"bufio"
	"bytes"
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
	V4 uint8 = 4

	Size = timeseries.Minute * 10
)

type Meta struct {
	Path        string
	From        timeseries.Time
	PointsCount uint32
	Step        timeseries.Duration
	Finalized   bool
	Created     timeseries.Time
}

func (m *Meta) To() timeseries.Time {
	return m.From.Add(timeseries.Duration(m.PointsCount-1) * m.Step)
}

func (m *Meta) Jitter() timeseries.Duration {
	return m.From.Sub(m.From.Truncate(Size))
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

func ReadMeta(path string) (*Meta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	h := header{}
	if err = binary.Read(f, binary.LittleEndian, &h); err != nil {
		return nil, err
	}
	return &Meta{
		Path:        path,
		From:        h.From,
		PointsCount: h.PointsCount,
		Step:        h.Step,
		Finalized:   h.Finalized,
		Created:     timeseries.TimeFromStandard(stat.ModTime()),
	}, err
}

func Read(path string, from timeseries.Time, pointsCount int, step timeseries.Duration, dest map[uint64]*model.MetricValues, fillFunc timeseries.FillFunc) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	h := header{}
	if err = binary.Read(reader, binary.LittleEndian, &h); err != nil {
		return err
	}
	switch h.Version {
	case V3:
		return readV3(reader, &h, from, pointsCount, step, dest, fillFunc)
	case V4:
		return readV4(reader, &h, from, pointsCount, step, dest, fillFunc)
	default:
		return fmt.Errorf("unknown version: %d", h.Version)
	}
}

func readV3(reader io.Reader, header *header, from timeseries.Time, pointsCount int, step timeseries.Duration, dest map[uint64]*model.MetricValues, fillFunc timeseries.FillFunc) error {
	r := lz4.NewDecompressReader(reader)
	defer r.Close()
	buf := make([]byte, metricMetaSize+4*header.PointsCount)
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
		if mv == nil {
			mv = &model.MetricValues{
				Values: timeseries.New(from, pointsCount, step),
			}
			labelsToRead = append(labelsToRead, &m)
			if m.MetaSize > maxLabelSize {
				maxLabelSize = m.MetaSize
			}
		}
		if !fillFunc(mv.Values, header.From, header.Step, asFloats32(buf[metricMetaSize:])) && !ok {
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
			mv.Labels = make(model.Labels)
			readLabels(buf[:m.MetaSize], mv)
			dest[m.Hash] = mv
		}
	}
	return nil
}

var (
	machineId         = []byte(model.LabelMachineId)
	systemUuid        = []byte(model.LabelSystemUuid)
	containerId       = []byte(model.LabelContainerId)
	destination       = []byte(model.LabelDestination)
	destinationIP     = []byte(model.LabelDestinationIP)
	actualDestination = []byte(model.LabelActualDestination)
)

func metadataSize(mv *model.MetricValues) int {
	s := 0
	if mv.MachineID != "" {
		s += len(machineId) + len(mv.MachineID) + 2
	}
	if mv.SystemUUID != "" {
		s += len(systemUuid) + len(mv.SystemUUID) + 2
	}
	if mv.ContainerId != "" {
		s += len(containerId) + len(mv.ContainerId) + 2
	}
	if mv.Destination != "" {
		label := destination
		if mv.DestIp {
			label = destinationIP
		}
		s += len(label) + len(mv.Destination) + 2
	}
	if mv.ActualDestination != "" {
		s += len(actualDestination) + len(mv.ActualDestination) + 2
	}
	for k, v := range mv.Labels {
		s += len(k) + len(v) + 2
	}
	return s
}

func writeLabels(dst *bufio.Writer, mv *model.MetricValues) error {
	if mv.MachineID != "" {
		if _, err := dst.Write(machineId); err != nil {
			return err
		}
		if err := dst.WriteByte(0); err != nil {
			return err
		}
		if _, err := dst.WriteString(mv.MachineID); err != nil {
			return err
		}
		if err := dst.WriteByte(0); err != nil {
			return err
		}
	}
	if mv.SystemUUID != "" {
		if _, err := dst.Write(systemUuid); err != nil {
			return err
		}
		if err := dst.WriteByte(0); err != nil {
			return err
		}
		if _, err := dst.WriteString(mv.SystemUUID); err != nil {
			return err
		}
		if err := dst.WriteByte(0); err != nil {
			return err
		}
	}
	if mv.ContainerId != "" {
		if _, err := dst.Write(containerId); err != nil {
			return err
		}
		if err := dst.WriteByte(0); err != nil {
			return err
		}
		if _, err := dst.WriteString(mv.ContainerId); err != nil {
			return err
		}
		if err := dst.WriteByte(0); err != nil {
			return err
		}
	}
	if mv.Destination != "" {
		label := destination
		if mv.DestIp {
			label = destinationIP
		}
		if _, err := dst.Write(label); err != nil {
			return err
		}
		if err := dst.WriteByte(0); err != nil {
			return err
		}
		if _, err := dst.WriteString(mv.Destination); err != nil {
			return err
		}
		if err := dst.WriteByte(0); err != nil {
			return err
		}
	}
	if mv.ActualDestination != "" {
		if _, err := dst.Write(actualDestination); err != nil {
			return err
		}
		if err := dst.WriteByte(0); err != nil {
			return err
		}
		if _, err := dst.WriteString(mv.ActualDestination); err != nil {
			return err
		}
		if err := dst.WriteByte(0); err != nil {
			return err
		}
	}
	for k, v := range mv.Labels {
		if _, err := dst.WriteString(k); err != nil {
			return err
		}
		if err := dst.WriteByte(0); err != nil {
			return err
		}
		if _, err := dst.WriteString(v); err != nil {
			return err
		}
		if err := dst.WriteByte(0); err != nil {
			return err
		}
	}
	return nil
}

func readLabels(src []byte, mv *model.MetricValues) {
	var key []byte
	isValue := false
	f := 0
	for {
		i := bytes.IndexByte(src[f:], 0)
		if i == -1 {
			break
		}
		i += f
		if isValue {
			if bytes.Equal(key, machineId) {
				mv.MachineID = string(src[f:i])
			} else if bytes.Equal(key, systemUuid) {
				mv.SystemUUID = string(src[f:i])
			} else if bytes.Equal(key, containerId) {
				mv.ContainerId = string(src[f:i])
			} else if bytes.Equal(key, destination) {
				mv.Destination = string(src[f:i])
			} else if bytes.Equal(key, actualDestination) {
				mv.ActualDestination = string(src[f:i])
			} else if bytes.Equal(key, destinationIP) {
				mv.Destination = string(src[f:i])
				mv.DestIp = true
			} else {
				mv.Labels[string(key)] = string(src[f:i])
			}
			isValue = false
		} else {
			key = src[f:i]
			isValue = true
		}
		f = i + 1
	}
}

func asBytes32[T any](slice []T) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(&slice[0])), len(slice)*4)
}

func asBytes64[T any](slice []T) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(&slice[0])), len(slice)*8)
}

func asFloats32(b []byte) []float32 {
	return unsafe.Slice((*float32)(unsafe.Pointer(&b[0])), len(b)/4)
}

func asUint64(b []byte) []uint64 {
	return unsafe.Slice((*uint64)(unsafe.Pointer(&b[0])), len(b)/8)
}

func asUint32(b []byte) []uint32 {
	return unsafe.Slice((*uint32)(unsafe.Pointer(&b[0])), len(b)/4)
}
