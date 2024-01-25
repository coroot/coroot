package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

func TestCacheUpdater_calcIntervals(t *testing.T) {
	scrapeInterval := 30 * timeseries.Second

	ts := func(s string) timeseries.Time {
		if s == "" {
			return 0
		}
		res, err := time.Parse("2006-01-02T15:04:05", s)
		assert.NoError(t, err)
		return timeseries.Time(res.Unix())
	}

	calc := func(lastSaved, now string) string {
		jitter := 12 * timeseries.Minute
		return fmt.Sprintf(`%s`, calcIntervals(ts(lastSaved), scrapeInterval, ts(now), jitter))
	}

	assert.Equal(t, // initial fetching
		"[(2020-11-13T09:12:00, 3600, 2020-11-13T10:11:30) (2020-11-13T10:12:00, 3600, 2020-11-13T11:11:30) (2020-11-13T11:12:00, 3600, 2020-11-13T11:48:30)]",
		calc("2020-11-13T09:49:11", "2020-11-13T11:49:11"),
	)
	assert.Equal(t, // two new points
		"[(2020-11-13T11:12:00, 3600, 2020-11-13T11:49:30)]",
		calc("2020-11-13T11:48:30", "2020-11-13T11:50:11"),
	)
	assert.Equal(t, // skipped more than two chunk intervals
		"[(2020-11-13T11:12:00, 3600, 2020-11-13T12:11:30) (2020-11-13T12:12:00, 3600, 2020-11-13T13:11:30) (2020-11-13T13:12:00, 3600, 2020-11-13T13:48:30)]",
		calc("2020-11-13T11:50:00", "2020-11-13T13:49:11"),
	)
	assert.Equal(t, // one new point
		"[(2020-11-13T12:12:00, 3600, 2020-11-13T12:12:30)]",
		calc("2020-11-13T12:12:00", "2020-11-13T12:13:05"),
	)
	assert.Equal(t, // two new points
		"[(2020-11-13T12:12:00, 3600, 2020-11-13T12:12:30)]",
		calc("2020-11-13T12:11:30", "2020-11-13T12:13:05"),
	)
	assert.Equal(t, // re-fetch finished chunk
		"[(2020-11-13T11:12:00, 3600, 2020-11-13T12:11:30)]",
		calc("2020-11-13T12:11:00", "2020-11-13T12:12:05"),
	)
	assert.Equal(t, // re-fetch finished chunk and one new point
		"[(2020-11-13T11:12:00, 3600, 2020-11-13T12:11:30) (2020-11-13T12:12:00, 3600, 2020-11-13T12:12:00)]",
		calc("2020-11-13T12:11:00", "2020-11-13T12:12:35"),
	)
	assert.Equal(t, // re-fetch finished chunk and two new points
		"[(2020-11-13T11:12:00, 3600, 2020-11-13T12:11:30) (2020-11-13T12:12:00, 3600, 2020-11-13T12:12:30)]",
		calc("2020-11-13T12:11:00", "2020-11-13T12:13:05"),
	)
	assert.Equal(t, // too early - nothing to do
		"[]",
		calc("2020-11-13T12:11:30", "2020-11-13T12:12:25"),
	)
}
