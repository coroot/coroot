package cache

import (
	"context"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/prom"
	"github.com/coroot/coroot-focus/timeseries"
	"github.com/coroot/coroot-focus/utils"
	"sort"
	"time"
)

const (
	RawData ClientOption = iota
)

type ClientOption int

type Client struct {
	cache      *Cache
	promClient prom.Client
	options    map[ClientOption]bool
}

func (c *Cache) GetCacheClient(options ...ClientOption) prom.Client {
	cl := &Client{
		cache:   c,
		options: map[ClientOption]bool{},
	}
	for _, o := range options {
		cl.options[o] = true
	}
	return cl
}

func (c *Client) QueryRange(ctx context.Context, query string, from, to time.Time, step time.Duration) ([]model.MetricValues, error) {
	from = from.Truncate(step)
	to = to.Truncate(step)
	c.cache.lock.RLock()
	defer c.cache.lock.RUnlock()
	queryHash := hash(query)
	qData, ok := c.cache.data[queryHash]
	if !ok {
		return nil, nil
	}
	start := timeseries.Time(from.Unix())
	end := timeseries.Time(to.Unix())
	res := map[uint64]model.MetricValues{}

	for _, chunkInfo := range qData.chunksOnDisk {
		if chunkInfo.ts > end || chunkInfo.lastTs < start {
			continue
		}
		chunk, err := NewChunkFromInfo(chunkInfo)
		if err != nil {
			return nil, err
		}
		err = chunk.ReadMetrics(timeseries.Time(from.Unix()), timeseries.Time(to.Unix()), timeseries.Duration(step.Seconds()), res)
		chunk.Close()
		if err != nil {
			return nil, err
		}
	}
	r := make([]model.MetricValues, 0, len(res))
	for _, mv := range res {
		r = append(r, mv)
	}
	return r, nil
}

func (c *Client) LastUpdateTime(actualQueries *utils.StringSet) time.Time {
	c.cache.lock.RLock()
	defer c.cache.lock.RUnlock()
	var ts time.Time

	actualHashes := utils.NewStringSet()
	for _, q := range actualQueries.Items() {
		actualHashes.Add(hash(q))
	}

	for queryHash, v := range c.cache.data {
		if !actualHashes.Has(queryHash) {
			continue
		}
		if len(v.chunksOnDisk) == 0 {
			continue
		}
		chunks := make([]*ChunkInfo, 0, len(v.chunksOnDisk))
		for _, chunk := range v.chunksOnDisk {
			chunks = append(chunks, chunk)
		}
		sort.Slice(chunks, func(i, j int) bool {
			return chunks[i].lastTs > chunks[j].lastTs
		})
		lastTs := time.Unix(int64(chunks[0].lastTs), 0)
		if ts.IsZero() || lastTs.Before(ts) {
			ts = lastTs
		}
	}
	return ts
}

func maxDuration(d1, d2 time.Duration) time.Duration {
	if d1 >= d2 {
		return d1
	}
	return d2
}
