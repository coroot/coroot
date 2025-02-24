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
	if fillFunc == nil {
		fillFunc = timeseries.FillAny
	}
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
		err := chunk.Read(ch.Path, from, resPoints, step, []string{chunk.DefaultMetricName}, res, fillFunc)
		if err != nil {
			return nil, err
		}
	}
	return maps.Values(res), nil
}

func (c *Client) QueryRange2(ctx context.Context, group string, from, to timeseries.Time, step timeseries.Duration) ([]*model.MetricValues, error) {
	metrics := model.Metrics[group]
	size := len(metrics)
	res := map[uint64]*model.MetricValues{}
	for i, metric := range metrics {
		ms, err := c.QueryRange(ctx, metric.Query, from, to, step, metric.FillFunc)
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			mv := res[m.LabelsHash]
			if mv == nil {
				mv = &model.MetricValues{
					Labels:          m.Labels,
					LabelsHash:      m.LabelsHash,
					NodeContainerId: m.NodeContainerId,
					ConnectionKey:   m.ConnectionKey,
					DestIp:          m.DestIp,
					Values:          make([]*timeseries.TimeSeries, size),
				}
				res[m.LabelsHash] = mv
			}
			mv.Values[i] = m.Values[0]
		}
	}
	return maps.Values(res), nil
}

func (c *Client) QueryRange3(ctx context.Context, group string, from, to timeseries.Time, step timeseries.Duration) ([]*model.MetricValues, error) {
	c.cache.lock.RLock()
	defer c.cache.lock.RUnlock()
	projData := c.cache.byProject[c.projectId]
	if projData == nil {
		return nil, fmt.Errorf("unknown project: %s", c.projectId)
	}
	hash := queryHash(group)
	qData := projData.queries[hash]
	if qData == nil {
		return nil, fmt.Errorf("%w: %s", constructor.ErrUnknownQuery, group)
	}
	from = from.Truncate(step)
	to = to.Truncate(step)
	res := map[uint64]*model.MetricValues{}
	resPoints := int(to.Sub(from)/step + 1)

	chunks := maps.Values(qData.chunksOnDisk)
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Created < chunks[j].Created
	})

	fillFunc := timeseries.FillAny // TODO: take from model.Metrics
	metrics := make([]string, len(model.Metrics[group]))
	for i, m := range model.Metrics[group] {
		metrics[i] = m.Name
	}
	for _, ch := range chunks {
		if ch.From > to || ch.To() < from {
			continue
		}
		err := chunk.Read(ch.Path, from, resPoints, step, metrics, res, fillFunc)
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
