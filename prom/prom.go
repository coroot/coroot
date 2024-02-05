package prom

import (
	"context"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type Querier interface {
	QueryRange(ctx context.Context, query string, from, to timeseries.Time, step timeseries.Duration) ([]model.MetricValues, error)
	GetStep(from, to timeseries.Time) (timeseries.Duration, error)
}
