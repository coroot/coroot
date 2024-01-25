package cache

import (
	"time"

	"github.com/coroot/coroot/timeseries"
)

type Config struct {
	Path       string
	GC         *GcConfig
	Compaction *CompactionConfig
}

type GcConfig struct {
	Interval time.Duration `yaml:"interval"`
	TTL      time.Duration `yaml:"ttl"`
}

type CompactionConfig struct {
	Interval   time.Duration `yaml:"interval"`
	WorkersNum int           `yaml:"workers_num"`

	Compactors []Compactor `yaml:"compactors"`
}

type Compactor struct {
	SrcChunkDuration timeseries.Duration `yaml:"src_chunk_duration_seconds"`
	DstChunkDuration timeseries.Duration `yaml:"dst_chunk_duration_seconds"`
}

var DefaultCompactionConfig = CompactionConfig{
	Interval:   time.Second * 10,
	WorkersNum: 1,
	Compactors: []Compactor{
		{SrcChunkDuration: 3600, DstChunkDuration: 4 * 3600},
		{SrcChunkDuration: 4 * 3600, DstChunkDuration: 12 * 3600},
	},
}
