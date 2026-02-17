package auditor

import (
	"fmt"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/dustin/go-humanize"
)

func (a *appAuditor) storage() {
	report := a.addReport(model.AuditReportStorage)

	ioCheck := report.CreateCheck(model.Checks.StorageIOLoad)
	spaceCheck := report.CreateCheck(model.Checks.StorageSpace)

	ioLatencyChart := report.GetOrCreateChartGroup("Average I/O latency <selector>, seconds", nil)
	ioLoadChart := report.GetOrCreateChartGroup("I/O load (total latency) <selector>, seconds/second", nil)
	iopsChart := report.GetOrCreateChartGroup("IOPS <selector>", nil)
	bandwidthChart := report.GetOrCreateChartGroup("Bandwidth <selector>, bytes/second", nil)
	ioUtilizationChart := report.GetOrCreateChartGroup("I/O utilization <selector>, %", nil)
	spaceChart := report.GetOrCreateChartGroup("Disk space <selector>, bytes", nil)

	seenVolumes := false
	isK8s := a.app.IsK8s()
	for _, i := range a.app.Instances {
		for _, v := range i.Volumes {
			fullName := i.Name + ":" + v.MountPoint
			if i.Node != nil {
				if isK8s && v.Name.Value() == "" {
					continue
				}
				seenVolumes = true
				if d := i.Node.Disks[v.Device.Value()]; d != nil {
					if ioLatencyChart != nil {
						ioLatencyChart.GetOrCreateChart(v.MountPoint).Feature().AddSeries(i.Name, d.Await)
						ioLatencyChart.
							GetOrCreateChart(i.Name+":"+v.MountPoint).
							AddSeries("read", timeseries.Div(d.ReadTime, d.ReadOps), "blue").
							AddSeries("write", timeseries.Div(d.WriteTime, d.WriteOps), "amber")

					}
					if ioUtilizationChart != nil {
						ioUtilizationChart.GetOrCreateChart(v.MountPoint).AddSeries(i.Name, d.IOUtilizationPercent)
					}
					ioLoad := timeseries.NewAggregate(timeseries.NanSum).Add(d.ReadTime, d.WriteTime).Get()
					if ioLoadChart != nil {
						ioLoadChart.
							GetOrCreateChart(v.MountPoint).
							Feature().
							AddSeries(i.Name, ioLoad)
						ioLoadChart.
							GetOrCreateChart(i.Name+":"+v.MountPoint).
							Stacked().
							AddSeries("read", d.ReadTime, "blue").
							AddSeries("write", d.WriteTime, "amber")
					}
					load := ioLoad.Get().Last()
					if load > ioCheck.Value() {
						ioCheck.SetValue(load)
					}
					if load > ioCheck.Threshold {
						ioCheck.AddItem("%s:%s", i.Name, v.MountPoint)
					}
					if iopsChart != nil {
						iopsChart.GetOrCreateChart(fullName).Stacked().Sorted().
							AddSeries("read", d.ReadOps, "blue").
							AddSeries("write", d.WriteOps, "amber")
					}
					if bandwidthChart != nil {
						bandwidthChart.GetOrCreateChart(fullName).Stacked().Sorted().
							AddSeries("read", d.ReadBytes, "blue").
							AddSeries("written", d.WrittenBytes, "amber")
					}

					latencyMs := model.NewTableCell().SetUnit("ms").SetValue(utils.FormatFloat(d.Await.Last() * 1000))
					ioLoadCell := model.NewTableCell()
					if !timeseries.IsNaN(load) {
						ioLoadCell.SetValue(utils.FormatFloat(load))
					}
					space := model.NewTableCell()
					capacity := v.CapacityBytes.Last()
					usage := v.UsedBytes.Last()
					if usage > 0 && capacity > 0 {
						percentage := usage / capacity * 100
						space.SetValue(fmt.Sprintf(
							"%.0f%% (%s / %s)",
							percentage,
							humanize.Bytes(uint64(usage)),
							humanize.Bytes(uint64(capacity))),
						)
						if percentage > spaceCheck.Value() {
							spaceCheck.SetValue(percentage)
						}
						if percentage > spaceCheck.Threshold {
							spaceCheck.AddItem("%s:%s", i.Name, v.MountPoint)
						}
					}
					report.GetOrCreateTable("Volume", "Latency", "I/O load", "Space", "Device").AddRow(
						model.NewTableCell(fullName),
						latencyMs,
						ioLoadCell,
						space,
						model.NewTableCell(v.Device.Value()).SetUnit(v.Name.Value()),
					)
				}
				if spaceChart != nil {
					spaceChart.GetOrCreateChart(fullName).Stacked().
						AddSeries("used", v.UsedBytes).
						SetThreshold("total", v.CapacityBytes)
				}
			}
		}
	}
	if !seenVolumes {
		a.delReport(model.AuditReportStorage)
	}
}
