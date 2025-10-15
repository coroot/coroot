package clickhouse

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/coroot/coroot/ch"
	"github.com/coroot/coroot/config"
	"github.com/coroot/coroot/db"
	"golang.org/x/exp/maps"
	"k8s.io/klog"
)

type SpaceManager struct {
	client    *Client
	cfg       config.ClickHouseSpaceManager
	databases []string
}

func (sm *SpaceManager) CheckAndCleanup(ctx context.Context, cluster string) error {
	klog.Infof("cluster %s", cluster)
	topology, err := sm.client.getClusterTopology(ctx)
	if err != nil {
		return fmt.Errorf("could not get cluster topology: %w", err)
	}

	if len(topology) == 0 {
		klog.Infoln("no cluster topology found, running on single host")
		return sm.runCleanup(ctx, sm.client, cluster)
	}

	for _, node := range topology {
		replicaAddr := net.JoinHostPort(node.HostName, strconv.Itoa(int(node.Port)))
		if err := sm.runCleanupOnReplica(ctx, replicaAddr); err != nil {
			klog.Errorf("cleanup failed for replica %s (shard %d, replica %d): %v",
				replicaAddr, node.ShardNum, node.ReplicaNum, err)
		} else {
			klog.Infof("completed cleanup for replica %s (shard %d, replica %d)",
				replicaAddr, node.ShardNum, node.ReplicaNum)
		}
	}

	return nil
}

func (sm *SpaceManager) runCleanupOnReplica(ctx context.Context, replicaAddr string) error {
	config := NewClientConfig(replicaAddr, sm.client.config.User, sm.client.config.Password)
	config.Protocol = sm.client.config.Protocol
	config.Database = sm.client.config.Database
	config.TlsEnable = sm.client.config.TlsEnable
	config.TlsSkipVerify = sm.client.config.TlsSkipVerify

	client, err := NewClient(config, ch.ClickHouseInfo{})
	if err != nil {
		return fmt.Errorf("failed to create client for replica %s: %w", replicaAddr, err)
	}
	defer client.Close()

	return sm.runCleanup(ctx, client, replicaAddr)
}

func (sm *SpaceManager) runCleanup(ctx context.Context, client *Client, addr string) error {
	klog.Infof("begin cleanup for %s", addr)
	diskInfo, err := client.GetDiskInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get disk info: %w", err)
	}

	if len(diskInfo) == 0 {
		klog.Infoln("no disk information available")
		return nil
	}

	var cleanupNeeded []string
	for _, disk := range diskInfo {
		if disk.Type != "Local" {
			continue
		}

		usedSpace := disk.TotalSpace - disk.FreeSpace
		usagePercent := float64(usedSpace) / float64(disk.TotalSpace) * 100

		klog.Infof("disk \"%s\" usage: %.2f%%, threshold: %d%%", disk.Name, usagePercent, sm.cfg.UsageThresholdPercent)

		if int(usagePercent) > sm.cfg.UsageThresholdPercent {
			klog.Infof("disk %s usage (%.2f%%) exceeds threshold (%d%%), marking for cleanup",
				disk.Name, usagePercent, sm.cfg.UsageThresholdPercent)
			cleanupNeeded = append(cleanupNeeded, disk.Name)
		}
	}

	if len(cleanupNeeded) == 0 {
		return nil
	}

	for _, diskName := range cleanupNeeded {
		if err := sm.cleanupOldestPartitionsFromDisk(ctx, client, diskName); err != nil {
			return fmt.Errorf("failed to cleanup partitions from disk %s: %w", diskName, err)
		}
	}
	return nil
}

func (sm *SpaceManager) cleanupOldestPartitionsFromDisk(ctx context.Context, client *Client, diskName string) error {
	partitions, err := sm.getPartitionsFromDiskOnServer(ctx, client, diskName)
	if err != nil {
		return fmt.Errorf("failed to get partitions from disk %s: %w", diskName, err)
	}

	tablePartitions := make(map[string][]PartitionInfo)
	for _, partition := range partitions {
		tableKey := partition.Database + "." + partition.Table
		tablePartitions[tableKey] = append(tablePartitions[tableKey], partition)
	}

	for tableKey, tablePartitionList := range tablePartitions {
		if len(tablePartitionList) <= sm.cfg.MinPartitions {
			klog.Infof("table %s has %d partitions, keeping minimum %d",
				tableKey, len(tablePartitionList), sm.cfg.MinPartitions)
			continue
		}

		oldestPartition := tablePartitionList[0]
		if err := sm.dropPartition(ctx, client, oldestPartition); err != nil {
			klog.Errorf("failed to drop partition %s from table %s on disk %s: %v",
				oldestPartition.PartitionId, tableKey, diskName, err)
			continue
		}
	}
	return nil
}

func (sm *SpaceManager) dropPartition(ctx context.Context, client *Client, part PartitionInfo) error {
	klog.Infof("dropping partition %s from table %s.%s", part.PartitionId, part.Database, part.Table)

	query := fmt.Sprintf(
		"ALTER TABLE %s.%s DROP PARTITION ID '%s'",
		part.Database,
		part.Table,
		part.PartitionId,
	)

	return client.conn.Exec(ctx, query)
}

func (sm *SpaceManager) getPartitionsFromDiskOnServer(ctx context.Context, client *Client, diskName string) ([]PartitionInfo, error) {
	query := `
		SELECT 
			p.database,
			p.table,
			p.partition_id
		FROM system.parts p
		WHERE p.active = 1 
			AND p.min_time > 0
			AND p.disk_name = ?
			AND p.database IN ?
			AND (p.table LIKE 'otel_%' OR p.table LIKE 'profiling_%')
		ORDER BY p.min_time ASC`

	rows, err := client.conn.Query(ctx, query, diskName, sm.databases)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partitions []PartitionInfo
	for rows.Next() {
		var partition PartitionInfo
		if err := rows.Scan(&partition.Database, &partition.Table, &partition.PartitionId); err != nil {
			return nil, err
		}
		partitions = append(partitions, partition)
	}

	return partitions, rows.Err()
}

func RunSpaceManagerForProjects(ctx context.Context, cfg config.ClickHouseSpaceManager, projects []*db.Project, globalClickHouse *db.IntegrationClickhouse) error {
	type clusterInfo struct {
		config    *db.IntegrationClickhouse
		databases map[string]bool
	}
	clusters := map[string]*clusterInfo{}

	for _, project := range projects {
		cfg := project.ClickHouseConfig(globalClickHouse)
		if cfg == nil {
			continue
		}
		if _, exists := clusters[cfg.Addr]; !exists {
			clusters[cfg.Addr] = &clusterInfo{
				config:    cfg,
				databases: make(map[string]bool),
			}
		}
		if cfg.Database != "" {
			clusters[cfg.Addr].databases[cfg.Database] = true
		}
	}
	if len(clusters) == 0 {
		klog.Infoln("no ClickHouse configurations found")
		return nil
	}
	for addr, cluster := range clusters {
		if err := runSpaceManagerOnCluster(ctx, cfg, cluster.config, maps.Keys(cluster.databases)); err != nil {
			klog.Errorf("failed for cluster %s: %v", addr, err)
		}
	}
	return nil
}

func runSpaceManagerOnCluster(ctx context.Context, managerCfg config.ClickHouseSpaceManager, cfg *db.IntegrationClickhouse, databases []string) error {
	config := NewClientConfig(cfg.Addr, cfg.Auth.User, cfg.Auth.Password)
	config.Protocol = cfg.Protocol
	config.Database = cfg.Database
	config.TlsEnable = cfg.TlsEnable
	config.TlsSkipVerify = cfg.TlsSkipVerify

	client, err := NewClient(config, ch.ClickHouseInfo{})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	cloud, err := client.IsCloud(ctx)
	if err != nil {
		return err
	}
	if cloud {
		klog.Infoln("storage manager is disabled for ClickHouse Cloud")
		return nil
	}

	spaceManager := &SpaceManager{
		client:    client,
		cfg:       managerCfg,
		databases: databases,
	}

	return spaceManager.CheckAndCleanup(ctx, cfg.Addr)
}

type PartitionInfo struct {
	Database    string
	Table       string
	PartitionId string
}
