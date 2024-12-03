package overview

import (
	"strconv"
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

func renderNodes(w *model.World) *model.Table {
	nodes := model.NewTable("Node", "Status", "Availability zone", "IP", "CPU", "Memory", "Network")
	unknown := model.NewTable(nodes.Header...)
	for _, n := range w.Nodes {
		name := n.GetName()
		if name == "" {
			klog.Warningln("empty node name for", n.Id)
			continue
		}

		node := model.NewTableCell(name).SetMaxWidth(30)
		node.Link = model.NewRouterLink(name, "overview").
			SetParam("view", "nodes").
			SetParam("id", name)
		for _, t := range getNodeTags(n) {
			node.AddTag(t)
		}

		status := model.NewTableCell().SetStatus(model.OK, "up")
		switch {
		case !n.IsAgentInstalled():
			status.SetStatus(model.UNKNOWN, "no agent installed")
		case n.IsDown():
			status.SetStatus(model.WARNING, "down (no metrics)")
		}
		if v := n.Uptime.Last(); !timeseries.IsNaN(v) {
			status.SetUnit("(" + utils.FormatDurationShort(timeseries.Duration(int64(v)), 1) + ")")
		}

		cpuPercent, memoryPercent := model.NewTableCell(), model.NewTableCell()
		if l := n.CpuUsagePercent.Last(); !timeseries.IsNaN(l) {
			cpuPercent.SetProgress(int(l), "blue")
		}
		if total := n.MemoryTotalBytes.Last(); !timeseries.IsNaN(total) {
			if avail := n.MemoryAvailableBytes.Last(); !timeseries.IsNaN(avail) {
				memoryPercent.SetProgress(int(100-avail/total*100), "deep-purple")
			}
		}

		network := model.NewTableCell()
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
			network.Bandwidth = &model.Bandwidth{
				Rx: utils.HumanBits(rxTotalBytes * 8),
				Tx: utils.HumanBits(txTotalBytes * 8),
			}
		}

		az := n.AvailabilityZone.Value()
		if az == "" {
			az = n.Region.Value()
		}

		table := nodes
		if *status.Status == model.UNKNOWN {
			table = unknown
		}
		table.AddRow(
			node,
			status,
			model.NewTableCell(az).SetMaxWidth(20).SetUnit("("+strings.ToLower(n.CloudProvider.Value())+")"),
			model.NewTableCell(ips.Items()...),
			cpuPercent,
			memoryPercent,
			network,
		)
	}

	nodes.Rows = append(nodes.Rows, unknown.Rows...)

	return nodes
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
