package cache

import (
	"crypto/md5"
	"fmt"
	"hash/fnv"

	"github.com/coroot/coroot/cache/chunk"
	"github.com/coroot/coroot/constructor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/timeseries"
)

var (
	rrHashes = map[string]bool{}
)

func init() {
	for query := range constructor.RecordingRules {
		rrHashes[queryHash(query)] = true
	}
}

func queryHash(query string) string {
	return fmt.Sprintf(`%x`, md5.Sum([]byte(query)))
}

func chunkJitter(projectId db.ProjectId, queryHash string) timeseries.Duration {
	if rrHashes[queryHash] {
		queryHash = ""
	}
	queryKey := fmt.Sprintf("%s-%s", projectId, queryHash)
	h := fnv.New32a()
	_, _ = h.Write([]byte(queryKey))
	return timeseries.Duration(h.Sum32()%uint32(chunk.Size/timeseries.Minute)) * timeseries.Minute
}

func QueryId(projectId db.ProjectId, query string) (string, timeseries.Duration) {
	h := queryHash(query)
	jitter := chunkJitter(projectId, h)
	return h, jitter
}
