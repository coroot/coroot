package watchers

import (
	"context"
	"time"

	"github.com/coroot/coroot/clickhouse"
	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

func StartRumRetention(cfg config.Rum, database *db.DB, globalClickHouse *db.IntegrationClickhouse) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		runRumRetentionOnce(cfg, database, globalClickHouse)
		for range ticker.C {
			runRumRetentionOnce(cfg, database, globalClickHouse)
		}
	}()
}

func runRumRetentionOnce(cfg config.Rum, database *db.DB, globalClickHouse *db.IntegrationClickhouse) {
	projects, err := database.GetProjects()
	if err != nil {
		klog.Errorln("rum retention: failed to get projects:", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := clickhouse.RunRumRetentionForProjects(ctx, maps.Values(projects), globalClickHouse, cfg); err != nil {
		klog.Errorln("rum retention:", err)
	}
}
