package prom

import (
	"context"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type Client interface {
	QueryRange(ctx context.Context, query string, from, to timeseries.Time, step timeseries.Duration) ([]model.MetricValues, error)
	LastUpdateTime(set *utils.StringSet) timeseries.Time
}
