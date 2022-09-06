package cache

import (
	"context"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"sort"
)

const (
	RawData ClientOption = iota
)

type ClientOption int

type Client struct {
	cache          *Cache
	projectId      db.ProjectId
	promClient     prom.Client
	scrapeInterval timeseries.Duration
	options        map[ClientOption]bool
}

func (c *Cache) GetCacheClient(p *db.Project, options ...ClientOption) prom.Client {
	cl := &Client{
		cache:          c,
		projectId:      p.Id,
		scrapeInterval: p.Prometheus.RefreshInterval,
		options:        map[ClientOption]bool{},
	}
	for _, o := range options {
		cl.options[o] = true
	}
	return cl
}

func (c *Client) QueryRange(ctx context.Context, query string, from, to timeseries.Time, step timeseries.Duration) ([]model.MetricValues, error) {
	from = from.Truncate(step)
	to = to.Truncate(step)
	c.cache.lock.RLock()
	defer c.cache.lock.RUnlock()

	byProject, ok := c.cache.byProject[c.projectId]
	if !ok {
		return nil, nil
	}
	queryHash := hash(query)
	qData, ok := byProject[queryHash]
	if !ok {
		return nil, nil
	}
	start := from
	end := to
	res := map[uint64]model.MetricValues{}

	for _, chunkInfo := range qData.chunksOnDisk {
		if chunkInfo.startTs > end || chunkInfo.lastTs < start {
			continue
		}
		chunk, err := OpenChunk(chunkInfo)
		if err != nil {
			return nil, err
		}
		err = chunk.ReadMetrics(from, to, step, res)
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

func (c *Client) LastUpdateTime(actualQueries *utils.StringSet) timeseries.Time {
	c.cache.lock.RLock()
	defer c.cache.lock.RUnlock()
	var ts timeseries.Time

	actualHashes := utils.NewStringSet()
	for _, q := range actualQueries.Items() {
		actualHashes.Add(hash(q))
	}

	for queryHash, v := range c.cache.byProject[c.projectId] {
		if !actualHashes.Has(queryHash) {
			continue
		}
		if len(v.chunksOnDisk) == 0 {
			continue
		}
		chunks := make([]*ChunkMeta, 0, len(v.chunksOnDisk))
		for _, chunk := range v.chunksOnDisk {
			chunks = append(chunks, chunk)
		}
		sort.Slice(chunks, func(i, j int) bool {
			return chunks[i].lastTs > chunks[j].lastTs
		})
		lastTs := chunks[0].lastTs
		if ts.IsZero() || lastTs.Before(ts) {
			ts = lastTs
		}
	}
	if !ts.IsZero() {
		ts = ts.Add(-c.scrapeInterval)
	}
	return ts
}

func (c *Cache) GetPromClient(p db.Project) prom.Client {
	client, err := prom.NewApiClient(p.Prometheus.Url, p.Prometheus.TlsSkipVerify)
	if err != nil {
		return NewErrorClient(err)
	}
	return client
}

type ErrorClient struct {
	err error
}

func NewErrorClient(err error) *ErrorClient {
	return &ErrorClient{err: err}
}

func (e ErrorClient) QueryRange(ctx context.Context, query string, from, to timeseries.Time, step timeseries.Duration) ([]model.MetricValues, error) {
	return nil, e.err
}

func (e ErrorClient) LastUpdateTime(actualQueries *utils.StringSet) timeseries.Time {
	return 0
}
