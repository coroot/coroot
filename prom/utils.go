package prom

import (
	"context"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"sync"
)

func ParallelQueryRange(ctx context.Context, client Client, from, to timeseries.Time, step timeseries.Duration, queries map[string]string) (map[string][]model.MetricValues, error) {
	res := make(map[string][]model.MetricValues, len(queries))
	var lock sync.Mutex
	var lastErr error
	wg := sync.WaitGroup{}
	for queryName, query := range queries {
		wg.Add(1)
		go func(queryName, query string) {
			defer wg.Done()
			metrics, err := client.QueryRange(ctx, query, from, to, step)
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
