package cache

import (
	"context"
	"fmt"
	"github.com/coroot/coroot/cache/chunk"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
)

type Client struct {
	cache           *Cache
	projectId       db.ProjectId
	refreshInterval timeseries.Duration
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
		return nil, fmt.Errorf("unknown project: %s", c.projectId)
	}
	queryHash := hash(query)
	qData, ok := byProject[queryHash]
	if !ok {
		return nil, fmt.Errorf("%w: %s", constructor.ErrUnknownQuery, query)
	}
	start := from
	end := to
	res := map[uint64]model.MetricValues{}
	resPoints := int(to.Sub(from)/step + 1)
	for _, ch := range qData.chunksOnDisk {
		if ch.From > end || ch.From.Add(timeseries.Duration(ch.PointsCount-1)*ch.Step) < start {
			continue
		}
		err := chunk.Read(ch.Path, from, resPoints, step, res)
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
	client, err := prom.NewApiClient(p.Prometheus.Url, user, password, p.Prometheus.TlsSkipVerify, p.Prometheus.ExtraSelector, p.Prometheus.CustomHeaders)
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
