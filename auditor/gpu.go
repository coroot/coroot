package auditor

import (
	"fmt"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/dustin/go-humanize"
)

type gpuInfo struct {
	instances *utils.StringSet
	node      *model.Node
	gpu       *model.GPU
}

func (a *appAuditor) gpu() {
	report := a.addReport(model.AuditReportGPU)

	table := report.GetOrCreateTable("GPU", "Name", "vRAM", "Node", "Instances")
	usageChart := report.GetOrCreateChart(fmt.Sprintf("GPU usage by <i>%s</i>, %%", a.app.Id.Name), nil)
	memoryUsageChart := report.GetOrCreateChart(fmt.Sprintf("GPU memory usage by <i>%s</i>, %%", a.app.Id.Name), nil)

	relatedGPUs := map[string]*gpuInfo{}

	seenGPUs := false
	for _, i := range a.app.Instances {
		if i.IsObsolete() {
			continue
		}
		total := timeseries.NewAggregate(timeseries.NanSum)
		memory := timeseries.NewAggregate(timeseries.NanSum)
		for uuid, u := range i.GPUUsage {
			seenGPUs = true
			total.Add(u.UsageAverage)
			memory.Add(u.MemoryUsageAverage)

			if i.Node != nil && i.Node.GPUs != nil {
				if gpu := i.Node.GPUs[uuid]; gpu != nil {
					gi := relatedGPUs[uuid]
					if gi == nil {
						gi = &gpuInfo{
							instances: utils.NewStringSet(),
							node:      i.Node,
							gpu:       gpu,
						}
						relatedGPUs[uuid] = gi
					}
					gi.instances.Add(i.Name)
				}
			}
		}
		usageChart.AddSeries(i.Name, total)
		memoryUsageChart.AddSeries(i.Name, memory)
	}
	for uuid, gi := range relatedGPUs {
		mem := model.NewTableCell()
		if last := gi.gpu.TotalMemory.Last(); last > 0 {
			mem.SetValue(humanize.Bytes(uint64(last)))
		}

		node := model.NewTableCell().SetStatus(gi.node.Status(), gi.node.GetName())
		node.Link = model.NewRouterLink(gi.node.GetName(), "overview").
			SetParam("view", "nodes").
			SetParam("id", gi.node.GetName())

		table.AddRow(
			model.NewTableCell(uuid),
			model.NewTableCell(gi.gpu.Name.Value()),
			mem,
			node,
			model.NewTableCell(gi.instances.Items()...),
		)
		report.
			GetOrCreateChartGroup("GPU utilization <selector>, %", nil).
			GetOrCreateChart("average").
			AddSeries(uuid, gi.gpu.UsageAverage).Feature()
		report.
			GetOrCreateChartGroup("GPU utilization <selector>, %", nil).
			GetOrCreateChart("peak").
			AddSeries(uuid, gi.gpu.UsagePeak)
		report.
			GetOrCreateChartGroup("GPU Memory utilization <selector>, %", nil).
			GetOrCreateChart("average").
			AddSeries(uuid, gi.gpu.MemoryUsageAverage).Feature()
		report.
			GetOrCreateChartGroup("GPU Memory utilization <selector>, %", nil).
			GetOrCreateChart("peak").
			AddSeries(uuid, gi.gpu.MemoryUsagePeak).Feature()

		coreChart := report.
			GetOrCreateChartGroup("GPU consumers <selector>, %", nil).
			GetOrCreateChart(uuid).Stacked()
		memChart := report.
			GetOrCreateChartGroup("GPU memory consumers <selector>, %", nil).
			GetOrCreateChart(uuid).Stacked()
		for _, ci := range gi.gpu.Instances {
			if u := ci.GPUUsage[uuid]; u != nil {
				coreChart.AddSeries(ci.Name, u.UsageAverage)
				memChart.AddSeries(ci.Name, u.MemoryUsageAverage)
			}
		}
		report.
			GetOrCreateChart("GPU temperature, ℃", nil).
			AddSeries(uuid, gi.gpu.Temperature)
		report.
			GetOrCreateChart("GPU power, watts", nil).
			AddSeries(uuid, gi.gpu.PowerWatts)
	}

	if !seenGPUs {
		a.delReport(model.AuditReportGPU)
	}
}
