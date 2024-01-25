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

	ioCheck := report.CreateCheck(model.Checks.StorageIO)
	spaceCheck := report.CreateCheck(model.Checks.StorageSpace)

	ioLatencyChart := report.GetOrCreateChartGroup("I/O latency <selector>, seconds")
	ioUtilizationChart := report.GetOrCreateChartGroup("I/O utilization <selector>, %")
	iopsChart := report.GetOrCreateChartGroup("IOPS <selector>")
	bandwidthChart := report.GetOrCreateChartGroup("Bandwidth <selector>, bytes/second")
	spaceChart := report.GetOrCreateChartGroup("Disk space <selector>, bytes")

	seenVolumes := false
	for _, i := range a.app.Instances {
		for _, v := range i.Volumes {
			fullName := i.Name + ":" + v.MountPoint
			if i.Node != nil {
				if a.app.IsK8s() && v.Name.Value() == "" {
					continue
				}
				seenVolumes = true
				if d := i.Node.Disks[v.Device.Value()]; d != nil {
					if ioLatencyChart != nil {
						ioLatencyChart.GetOrCreateChart(v.MountPoint).AddSeries(i.Name, d.Await)
					}
					if ioUtilizationChart != nil {
						ioUtilizationChart.GetOrCreateChart(v.MountPoint).AddSeries(i.Name, d.IOUtilizationPercent)
					}
					ioUtilization := d.IOUtilizationPercent.Last()
					if ioUtilization > ioCheck.Value() {
						ioCheck.SetValue(d.IOUtilizationPercent.Last())
					}
					if ioUtilization > ioCheck.Threshold {
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
					ioPercent := model.NewTableCell()
					if !timeseries.IsNaN(ioUtilization) {
						ioPercent.SetValue(fmt.Sprintf("%.0f%%", ioUtilization))
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
					report.GetOrCreateTable("Volume", "Latency", "I/O", "Space", "Device").AddRow(
						model.NewTableCell(fullName),
						latencyMs,
						ioPercent,
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
