package cache

import (
	"github.com/coroot/coroot/cache/chunk"
	"github.com/coroot/coroot/timeseries"
)

type Config struct {
	Path       string
	GC         *GcConfig
	Compaction *CompactionConfig
}

type GcConfig struct {
	Interval timeseries.Duration `yaml:"interval"`
	TTL      timeseries.Duration `yaml:"ttl"`
}

type CompactionConfig struct {
	Interval   timeseries.Duration `yaml:"interval"`
	WorkersNum int                 `yaml:"workers_num"`

	Compactors []Compactor `yaml:"compactors"`
}

type Compactor struct {
	SrcChunkDuration timeseries.Duration `yaml:"src_chunk_duration_seconds"`
	DstChunkDuration timeseries.Duration `yaml:"dst_chunk_duration_seconds"`
}

var DefaultCompactionConfig = CompactionConfig{
	Interval:   timeseries.Second * 10,
	WorkersNum: 1,
	Compactors: []Compactor{
		{SrcChunkDuration: chunk.Size, DstChunkDuration: timeseries.Hour},
		{SrcChunkDuration: timeseries.Hour, DstChunkDuration: 4 * timeseries.Hour},
		{SrcChunkDuration: 4 * timeseries.Hour, DstChunkDuration: 12 * timeseries.Hour},
	},
}
