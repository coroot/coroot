package overview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

type NetworkBandWidth struct {
	Rx int `json:"rx"`
	Tx int `json:"tx"`
}

type Node struct {
	Name                  string            `json:"name"`
	ClusterId             string            `json:"cluster_id"`
	ClusterName           string            `json:"cluster_name"`
	Status                model.Indicator   `json:"status"`
	UptimeMs              int64             `json:"uptime_ms"`
	AvailabilityZone      string            `json:"availability_zone"`
	CloudProvider         string            `json:"cloud_provider"`
	InstanceType          string            `json:"instance_type"`
	Compute               string            `json:"compute"`
	IPs                   []string          `json:"ips"`
	CPUPercent            int               `json:"cpu_percent"`
	MemoryPercent         int               `json:"memory_percent"`
	GPUs                  int               `json:"gpus"`
	TotalNetworkBandWidth int               `json:"total_network_band_width"`
	NetworkBandwidth      *NetworkBandWidth `json:"network_bandwidth"`
}

func renderNodes(w *model.World, project *db.Project) []Node {
	nodes := make([]Node, 0, len(w.Nodes))
	for _, n := range w.Nodes {
		name := n.GetName()
		if name == "" {
			klog.Warningln("empty node name for", n.Id)
			continue
		}
		node := Node{
			Name:          name,
			ClusterId:     n.ClusterId,
			ClusterName:   w.ClusterName(n.ClusterId),
			Status:        model.Indicator{Status: model.OK, Message: "up"},
			InstanceType:  n.InstanceType.Value(),
			Compute:       compute(n),
			GPUs:          len(n.GPUs),
			CloudProvider: strings.ToLower(n.CloudProvider.Value()),
		}
		switch {
		case !n.IsAgentInstalled():
			node.Status = model.Indicator{Status: model.UNKNOWN, Message: "no agent installed"}
		case n.IsDown():
			node.Status = model.Indicator{Status: model.WARNING, Message: "down (no metrics)"}
		}
		if v := n.Uptime.Last(); !timeseries.IsNaN(v) {
			node.UptimeMs = int64(v) * 1000
		}

		if l := n.CpuUsagePercent.Last(); !timeseries.IsNaN(l) {
			node.CPUPercent = int(l)
		}
		if total := n.MemoryTotalBytes.Last(); !timeseries.IsNaN(total) {
			if avail := n.MemoryAvailableBytes.Last(); !timeseries.IsNaN(avail) {
				node.MemoryPercent = int(100 - avail/total*100)
			}
		}

		var rxTotalBytes, txTotalBytes float32
		ips := utils.NewStringSet()
		for _, iface := range n.NetInterfaces {
			if iface.Up.Last() != 1 {
				continue
			}
			if iface.RxBytes.IsEmpty() || iface.TxBytes.IsEmpty() {
				continue
			}
			for _, ip := range iface.Addresses {
				ips.Add(ip)
			}
			if rx := iface.RxBytes.Last(); !timeseries.IsNaN(rx) {
				rxTotalBytes += rx
			}
			if tx := iface.TxBytes.Last(); !timeseries.IsNaN(tx) {
				txTotalBytes += tx
			}
		}
		if rxTotalBytes > 0 || txTotalBytes > 0 {
			node.NetworkBandwidth = &NetworkBandWidth{
				Rx: int(rxTotalBytes * 8),
				Tx: int(txTotalBytes * 8),
			}
			node.TotalNetworkBandWidth = int((rxTotalBytes + txTotalBytes) * 8)
		}
		node.IPs = ips.Items()
		node.AvailabilityZone = n.AvailabilityZone.Value()
		if node.AvailabilityZone == "" {
			node.AvailabilityZone = n.Region.Value()
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func compute(n *model.Node) string {
	c := n.CpuCapacity.Last()
	m := n.MemoryTotalBytes.Last()
	if !timeseries.IsNaN(c) && !timeseries.IsNaN(m) {
		v, u := utils.FormatBytes(m)
		return fmt.Sprintf("%d vCPU / %s%s", int(c), v, u)
	}
	return ""
}

func getNodeTags(n *model.Node) []string {
	var tags []string
	if t := n.InstanceType.Value(); t != "" {
		tags = append(tags, t)
	}
	if l := n.CpuCapacity.Last(); !timeseries.IsNaN(l) {
		tags = append(tags, strconv.Itoa(int(l))+" vCPU")
	}
	if l := n.MemoryTotalBytes.Last(); !timeseries.IsNaN(l) {
		v, u := utils.FormatBytes(l)
		tags = append(tags, v+u)
	}
	return tags
}
