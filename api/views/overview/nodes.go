package overview

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"net"
	"sort"
	"strconv"
	"strings"
)

func renderNodes(w *model.World) *model.Table {
	table := &model.Table{Header: []string{"Node", "Status", "Availability zone", "IP", "CPU", "Memory", "Network"}}
	for _, n := range w.Nodes {
		node := model.NewTableCell(n.Name.Value())
		node.Link = model.NewRouterLink(n.Name.Value()).SetRoute("node").SetParam("name", n.Name.Value())
		ips := utils.NewStringSet()

		cpuPercent, memoryPercent := model.NewTableCell(), model.NewTableCell()

		for _, t := range getNodeTags(n) {
			node.AddTag(t)
		}
		if l := n.CpuUsagePercent.Last(); !timeseries.IsNaN(l) {
			cpuPercent.SetProgress(int(l), "blue")
		}
		if total := n.MemoryTotalBytes.Last(); !timeseries.IsNaN(total) {
			if avail := n.MemoryAvailableBytes.Last(); !timeseries.IsNaN(avail) {
				memoryPercent.SetProgress(int(100-avail/total*100), "deep-purple")
			}
		}

		status := model.NewTableCell().SetStatus(model.OK, "up")
		if !n.IsUp() {
			status.SetStatus(model.WARNING, "down (no metrics)")
		}

		network := model.NewTableCell()
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
			if ips.Len() == 0 {
				for _, instance := range n.Instances {
					for l := range instance.TcpListens {
						if ip := net.ParseIP(l.IP); ip != nil && !ip.IsLoopback() {
							ips.Add(l.IP)
						}
					}
				}
			}
			network.NetInterfaces = append(network.NetInterfaces, model.NetInterface{
				Name: iface.Name,
				Rx:   utils.HumanBits(iface.RxBytes.Last() * 8),
				Tx:   utils.HumanBits(iface.TxBytes.Last() * 8),
			})
		}
		sort.Slice(network.NetInterfaces, func(i, j int) bool {
			return network.NetInterfaces[i].Name < network.NetInterfaces[j].Name
		})

		if v := n.Uptime.Last(); !timeseries.IsNaN(v) {
			status.SetUnit("(" + utils.FormatDurationShort(timeseries.Duration(int64(v)), 1) + ")")
		}

		table.AddRow(
			node,
			status,
			model.NewTableCell(n.AvailabilityZone.Value()).SetUnit("("+strings.ToLower(n.CloudProvider.Value())+")"),
			model.NewTableCell(ips.Items()...),
			cpuPercent,
			memoryPercent,
			network,
		)
	}
	return table
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
