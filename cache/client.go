package cache

import (
	"context"
	"fmt"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
)

type Client struct {
	cache           *Cache
	projectId       db.ProjectId
	refreshInterval timeseries.Duration
	promClient      prom.Client
}

func (c *Cache) GetCacheClient(p *db.Project) *Client {
	return &Client{
		cache:           c,
		projectId:       p.Id,
		refreshInterval: p.Prometheus.RefreshInterval,
	}
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

func (c *Client) Ping(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}

func (c *Client) GetTo() (timeseries.Time, error) {
	to, err := c.cache.getMinUpdateTime(c.projectId)
	if err != nil {
		return 0, err
	}
	if to.IsZero() {
		return 0, nil
	}
	return to.Add(-c.refreshInterval), nil
}

func (c *Client) GetStatus() (*Status, error) {
	return c.cache.getStatus(c.projectId)
}

func (c *Cache) getPromClient(p *db.Project) prom.Client {
	user, password := "", ""
	if p.Prometheus.BasicAuth != nil {
		user, password = p.Prometheus.BasicAuth.User, p.Prometheus.BasicAuth.Password
	}
	client, err := prom.NewApiClient(p.Prometheus.Url, user, password, p.Prometheus.TlsSkipVerify)
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

func (e ErrorClient) Ping(ctx context.Context) error {
	return e.err
}
