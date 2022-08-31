package cache

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/coroot/coroot-focus/db"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/prom"
	"github.com/coroot/coroot-focus/timeseries"
	"github.com/coroot/coroot-focus/utils"
	"github.com/prometheus/client_golang/prometheus"
	"hash/fnv"
	"io"
	"io/ioutil"
	"k8s.io/klog"
	"math"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	chunkSize = time.Hour
)

type Cache struct {
	lock       sync.RWMutex
	data       map[string]*queryData
	db         *db.DB
	promClient prom.Client
	cfg        Config

	pendingCompactions prometheus.Gauge
	compactedChunks    *prometheus.CounterVec

	scrapeInterval time.Duration
}

type ChunkInfo struct {
	path     string
	ts       timeseries.Time
	duration timeseries.Duration
	step     timeseries.Duration
	lastTs   timeseries.Time
}

type queryData struct {
	chunksOnDisk map[string]*ChunkInfo
}

func newQueryData() *queryData {
	return &queryData{
		chunksOnDisk: map[string]*ChunkInfo{},
	}
}

func NewCache(cfg Config, db *db.DB, promClient prom.Client, scrapeInterval time.Duration) (*Cache, error) {
	if err := utils.CreateDirectoryIfNotExists(cfg.Path); err != nil {
		return nil, err
	}

	cache := &Cache{
		cfg:            cfg,
		data:           map[string]*queryData{},
		scrapeInterval: scrapeInterval,
		db:             db,
		promClient:     promClient,

		pendingCompactions: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "coroot_pending_compactions",
			},
		),
		compactedChunks: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "coroot_compacted_chunks_total",
			},
			[]string{"src", "dst"},
		),
	}
	if err := cache.initCacheIndexFromDir(); err != nil {
		return nil, err
	}

	prometheus.MustRegister(cache.pendingCompactions)
	prometheus.MustRegister(cache.compactedChunks)

	//go cache.updater()
	//go cache.gc()
	//go cache.compaction()
	return cache, nil
}

func (c *Cache) initCacheIndexFromDir() error {
	t := time.Now()
	klog.Infoln("loading cache from disk")

	_, err := os.Stat(c.cfg.Path)
	if os.IsNotExist(err) {
		klog.Infof("creating dir %s", c.cfg.Path)
		if err := os.Mkdir(c.cfg.Path, 0755); err != nil {
			return err
		}
	}
	files, err := ioutil.ReadDir(c.cfg.Path)
	if err != nil {
		return err
	}
	for _, chunkFile := range files {
		if !strings.HasSuffix(chunkFile.Name(), ".db") {
			continue
		}
		queryId, chunkInfo, err := getChunkInfo(c.cfg.Path, chunkFile.Name())
		if err != nil {
			klog.Errorln(err)
			continue
		}
		byQuery, ok := c.data[queryId]
		if !ok {
			byQuery = newQueryData()
			c.data[queryId] = byQuery
		}
		byQuery.chunksOnDisk[chunkInfo.path] = chunkInfo
	}
	klog.Infof("cache loaded from disk in %s", time.Since(t))
	return nil
}

func getChunkInfo(cacheDir, filename string) (string, *ChunkInfo, error) {
	parts := strings.Split(strings.TrimSuffix(filename, ".db"), "-")
	if len(parts) != 4 {
		return "", nil, fmt.Errorf("invalid filename %s", filename)
	}
	queryId := parts[0]
	info := &ChunkInfo{
		path: path.Join(cacheDir, filename),
	}
	chunkTsUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", nil, err
	}
	info.ts = timeseries.Time(chunkTsUnix)
	d, err := strconv.ParseInt(parts[2], 10, 32)
	if err != nil {
		return "", nil, err
	}
	info.duration = timeseries.Duration(d)
	step, err := strconv.ParseInt(parts[3], 10, 32)
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

func NewChunkFromInfo(info *ChunkInfo) (*Chunk, error) {
	var err error
	chunk := &Chunk{
		from:     info.ts,
		duration: info.duration,
		to:       info.lastTs,
		step:     info.step,
	}
	if chunk.file, err = os.Open(info.path); err != nil {
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

func hash(query string) string {
	return fmt.Sprintf(`%x`, md5.Sum([]byte(query)))
}

func queryJitter(queryHash string) time.Duration {
	h := fnv.New32a()
	_, _ = h.Write([]byte(queryHash))
	return time.Duration(h.Sum32()%uint32(chunkSize.Minutes())) * time.Minute
}
