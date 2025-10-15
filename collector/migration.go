package collector

import (
	"context"
	"time"

	"github.com/coroot/coroot/ch"
	"github.com/coroot/coroot/db"
	"github.com/jpillora/backoff"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

func (c *Collector) migrateProjects() {
	b := backoff.Backoff{Factor: 2, Min: time.Minute, Max: 10 * time.Minute}
	for {
		t := time.Now()
		c.projectsLock.Lock()
		projects := maps.Values(c.projects)
		c.projectsLock.Unlock()
		var failed bool
		for _, p := range projects {
			c.migrationDoneLock.Lock()
			done := c.migrationDone[p.Id]
			c.migrationDoneLock.Unlock()
			if done {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			err := c.migrateProject(ctx, p)
			cancel()
			if err != nil {
				klog.Errorf("failed to create or update clickhouse tables for project %s: %s", p.Id, err)
				failed = true
				continue
			}
		}
		if failed {
			d := b.Duration()
			klog.Errorf("clickhouse tables migration failed, next attempt in %s", d.String())
			time.Sleep(d)
			continue
		}
		klog.Infof("clickhouse tables migration done in %s", time.Since(t).Truncate(time.Millisecond))
		return
	}
}

func (c *Collector) migrateProject(ctx context.Context, p *db.Project) error {
	cfg := p.ClickHouseConfig(c.globalClickHouse)
	if cfg == nil {
		return nil
	}
	if cfg.Global {
		initialDbCfg := *cfg
		initialDbCfg.Database = cfg.InitialDatabase
		if initialDbCfg.Database == "" {
			initialDbCfg.Database = "default"
		}
		client, err := ch.NewLowLevelClient(ctx, &initialDbCfg)
		if err != nil {
			return err
		}
		defer client.Close()
		if err = client.CreateDB(ctx, cfg.Database); err != nil {
			return err
		}
	}
	client, err := ch.NewLowLevelClient(ctx, cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	err = client.Migrate(ctx, c.cfg)
	if err == nil {
		c.migrationDoneLock.Lock()
		c.migrationDone[p.Id] = true
		c.migrationDoneLock.Unlock()
	}
	return err
}
