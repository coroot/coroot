package prom

import (
	"context"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"sync"
	"time"
)

type QueryStats struct {
	MetricsCount int     `json:"metrics_count"`
	QueryTime    float32 `json:"query_time"`
	Failed       bool    `json:"failed"`
}

func ParallelQueryRange(ctx context.Context, client Client, from, to timeseries.Time, step timeseries.Duration, queries map[string]string, stats map[string]QueryStats) (map[string][]model.MetricValues, error) {
	res := make(map[string][]model.MetricValues, len(queries))
	var lock sync.Mutex
	var lastErr error
	wg := sync.WaitGroup{}
	now := time.Now()
	for queryName, query := range queries {
		wg.Add(1)
		go func(queryName, query string) {
			defer wg.Done()
			metrics, err := client.QueryRange(ctx, query, from, to, step)
			if stats != nil {
				queryTime := float32(time.Since(now).Seconds())
				lock.Lock()
				if queryTime > stats[queryName].QueryTime {
					stats[queryName] = QueryStats{MetricsCount: len(metrics), QueryTime: queryTime, Failed: err != nil}
				}
				lock.Unlock()
			}
			if err != nil {
				lastErr = err
				return
			}
			lock.Lock()
			res[queryName] = metrics
			lock.Unlock()
		}(queryName, query)
	}
	wg.Wait()
	return res, lastErr
}
