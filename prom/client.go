package prom

import (
	"context"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/utils"
	"time"
)

type Client interface {
	QueryRange(ctx context.Context, query string, from, to time.Time, step time.Duration) ([]model.MetricValues, error)
	LastUpdateTime(set *utils.StringSet) time.Time
}
