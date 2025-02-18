package cache

import (
	"context"
	"fmt"
	"sort"

	"github.com/coroot/coroot/cache/chunk"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"golang.org/x/exp/maps"
)

func (c *Cache) GetCacheClient(projectId db.ProjectId) *Client {
	return &Client{
		cache:     c,
		projectId: projectId,
	}
}

type Client struct {
	cache     *Cache
	projectId db.ProjectId
}

func (c *Client) QueryRange(ctx context.Context, query string, from, to timeseries.Time, step timeseries.Duration, fillFunc timeseries.FillFunc) ([]*model.MetricValues, error) {
	c.cache.lock.RLock()
	defer c.cache.lock.RUnlock()
	projData := c.cache.byProject[c.projectId]
	if projData == nil {
		return nil, fmt.Errorf("unknown project: %s", c.projectId)
	}
	hash := queryHash(query)
	qData := projData.queries[hash]
	if qData == nil {
		return nil, fmt.Errorf("%w: %s", constructor.ErrUnknownQuery, query)
	}
	from = from.Truncate(step)
	to = to.Truncate(step)
	res := map[uint64]*model.MetricValues{}
	resPoints := int(to.Sub(from)/step + 1)

	chunks := maps.Values(qData.chunksOnDisk)
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Created < chunks[j].Created
	})

	for _, ch := range chunks {
		if ch.From > to || ch.To() < from {
			continue
		}
		err := chunk.Read(ch.Path, from, resPoints, step, res, fillFunc)
		if err != nil {
			return nil, err
		}
	}
	return maps.Values(res), nil
}

func (c *Client) GetStep(from, to timeseries.Time) (timeseries.Duration, error) {
	c.cache.lock.RLock()
	defer c.cache.lock.RUnlock()
	projData := c.cache.byProject[c.projectId]
	if projData == nil {
		return 0, fmt.Errorf("unknown project: %s", c.projectId)
	}

	var step timeseries.Duration
	for _, qData := range projData.queries {
		for _, ch := range qData.chunksOnDisk {
			if ch.From > to || ch.To() < from {
				continue
			}
			if ch.Step > step {
				step = ch.Step
			}
		}
	}
	if step == 0 {
		step = projData.step
	}
	return step, nil
}

func (c *Client) GetTo() (timeseries.Time, error) {
	to, err := c.cache.getMinUpdateTime(c.projectId)
	if err != nil {
		return 0, err
	}

	if to.IsZero() {
		return 0, nil
	}

	c.cache.lock.RLock()
	defer c.cache.lock.RUnlock()
	projData := c.cache.byProject[c.projectId]
	if projData == nil {
		return 0, fmt.Errorf("unknown project: %s", c.projectId)
	}
	step := projData.step

	return to.Add(-step), nil
}

func (c *Client) GetStatus() (*Status, error) {
	return c.cache.getStatus(c.projectId)
}
