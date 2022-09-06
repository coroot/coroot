package cache

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"io"
	"math"
	"os"
	"path"
	"strconv"
	"strings"
)

type ChunkMeta struct {
	path     string
	startTs  timeseries.Time
	duration timeseries.Duration
	step     timeseries.Duration
	lastTs   timeseries.Time
}

type Chunk struct {
	from     timeseries.Time
	duration timeseries.Duration
	to       timeseries.Time
	buf      *bytes.Buffer
	step     timeseries.Duration
	file     *os.File
	reader   *bufio.Reader
}

func NewChunk(ts, toTs timeseries.Time, duration, step timeseries.Duration, buf *bytes.Buffer) *Chunk {
	buf.Reset()
	_ = binary.Write(buf, binary.LittleEndian, ChunkVersion)
	_ = binary.Write(buf, binary.LittleEndian, toTs)
	return &Chunk{
		from:     ts,
		duration: duration,
		to:       toTs,
		step:     step,
		buf:      buf,
	}
}

func (chunk *Chunk) WriteMetric(v model.MetricValues) error {
	ts := timeseries.NewNan(timeseries.Context{From: chunk.from, To: chunk.to, Step: chunk.step})
	iter := v.Values.Iter()

	var firstPoint, lastPoint timeseries.Time

	for iter.Next() {
		t, v := iter.Value()
		if !math.IsNaN(v) {
			if firstPoint == 0 {
				firstPoint = t
			}
			lastPoint = t
		}
		ts.Set(t, v)
	}

	jsonData, err := json.Marshal(v.Labels)
	if err != nil {
		return err
	}
	tsData, err := ts.ToBinary()
	if err != nil {
		return err
	}
	h := metricHeader{
		FromTs:        firstPoint,
		ToTs:          lastPoint,
		LabelsHash:    v.LabelsHash,
		LabelsJsonLen: int32(len(jsonData)),
		TsDataLen:     int32(len(tsData)),
	}
	if err = binary.Write(chunk.buf, binary.LittleEndian, h); err != nil {
		return err
	}
	if _, err = chunk.buf.Write(jsonData); err != nil {
		return err
	}
	if _, err = chunk.buf.Write(tsData); err != nil {
		return err
	}
	return nil
}

func (chunk *Chunk) Close() {
	if chunk.file != nil {
		chunk.file.Close()
	}
}

func OpenChunk(meta *ChunkMeta) (*Chunk, error) {
	var err error
	chunk := &Chunk{
		from:     meta.startTs,
		duration: meta.duration,
		to:       meta.lastTs,
		step:     meta.step,
	}
	if chunk.file, err = os.Open(meta.path); err != nil {
		return nil, err
	}
	if _, err = chunk.file.Seek(9, io.SeekStart); err != nil {
		return nil, err
	}
	chunk.reader = bufio.NewReader(chunk.file)
	return chunk, err
}

func (chunk *Chunk) ReadMetrics(from, to timeseries.Time, step timeseries.Duration, dest map[uint64]model.MetricValues) error {
	h := metricHeader{}

	for {
		if err := binary.Read(chunk.reader, binary.LittleEndian, &h); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if h.FromTs > to || h.ToTs < from {
			if _, err := chunk.reader.Discard(int(h.LabelsJsonLen + h.TsDataLen)); err != nil {
				return err
			}
			continue
		}
		mv, ok := dest[h.LabelsHash]
		if !ok {
			mv.LabelsHash = h.LabelsHash
			jsonData := make([]byte, int(h.LabelsJsonLen))
			if _, err := io.ReadFull(chunk.reader, jsonData); err != nil {
				return err
			}
			if err := json.Unmarshal(jsonData, &mv.Labels); err != nil {
				return err
			}
			mv.Values = timeseries.NewNan(timeseries.Context{From: from, To: to, Step: step})
			dest[h.LabelsHash] = mv
		} else {
			if _, err := chunk.reader.Discard(int(h.LabelsJsonLen)); err != nil {
				return err
			}
		}
		ts := mv.Values.(*timeseries.InMemoryTimeSeries)
		tsData := make([]byte, h.TsDataLen)
		if _, err := io.ReadFull(chunk.reader, tsData); err != nil {
			return err
		}

		cts, err := timeseries.InMemoryTimeSeriesFromBinary(tsData)
		if err != nil {
			return err
		}
		iter := cts.Iter()
		for iter.Next() {
			t, v := iter.Value()
			if t < from {
				continue
			}
			if t > to {
				continue
			}
			ts.Set(t, v)
		}
	}
	return nil
}

type metricHeader struct {
	FromTs        timeseries.Time
	ToTs          timeseries.Time
	LabelsHash    uint64
	LabelsJsonLen int32
	TsDataLen     int32
}

func readChunkMeta(cacheDir, filename string) (string, *ChunkMeta, error) {
	parts := strings.Split(strings.TrimSuffix(filename, ".db"), "-")
	if len(parts) != 5 {
		return "", nil, fmt.Errorf("invalid filename %s", filename)
	}
	queryId := parts[1]
	info := &ChunkMeta{
		path: path.Join(cacheDir, filename),
	}
	chunkTsUnix, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", nil, err
	}
	info.startTs = timeseries.Time(chunkTsUnix)
	d, err := strconv.ParseInt(parts[3], 10, 32)
	if err != nil {
		return "", nil, err
	}
	info.duration = timeseries.Duration(d)
	step, err := strconv.ParseInt(parts[4], 10, 32)
	if err != nil {
		return "", nil, err
	}
	info.step = timeseries.Duration(step)

	f, err := os.Open(info.path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	var chunkVersion uint8
	if err = binary.Read(f, binary.LittleEndian, &chunkVersion); err != nil {
		return "", nil, fmt.Errorf("can't read chunk version: %w", err)
	}
	var lastTs int64
	if err = binary.Read(f, binary.LittleEndian, &lastTs); err != nil {
		return "", nil, fmt.Errorf("can't read chunk last timestamp: %w", err)
	}
	info.lastTs = timeseries.Time(lastTs)
	return queryId, info, nil
}
