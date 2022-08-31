package application

import (
	"fmt"
	widgets2 "github.com/coroot/coroot-focus/api/views/widgets"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/timeseries"
	"github.com/coroot/coroot-focus/utils"
	"github.com/dustin/go-humanize"
	"math"
)

func storage(ctx timeseries.Context, app *model.Application) *widgets2.Dashboard {
	dash := widgets2.NewDashboard(ctx, "Storage")

	for _, i := range app.Instances {
		for _, v := range i.Volumes {
			fullName := i.Name + ":" + v.MountPoint
			if i.Node != nil {
				if d := i.Node.Disks[v.Device.Value()]; d != nil {
					dash.GetOrCreateChartInGroup("I/O latency <selector>, seconds", v.MountPoint).
						AddSeries(i.Name, d.Await)

					dash.GetOrCreateChartInGroup("I/O utilization <selector>, %", v.MountPoint).
						AddSeries(i.Name, d.IOUtilizationPercent)

					dash.GetOrCreateChartInGroup("IOPS <selector>", fullName).
						Stacked().
						Sorted().
						AddSeries("read", d.ReadOps, "blue").
						AddSeries("write", d.WriteOps, "amber")

					dash.GetOrCreateChartInGroup("Bandwidth <selector>, bytes/second", fullName).
						Stacked().
						Sorted().
						AddSeries("read", d.ReadBytes, "blue").
						AddSeries("written", d.WrittenBytes, "amber")

					latencyMs := widgets2.NewTableCell().SetUnit("ms")
					if d.Await != nil {
						latencyMs.SetValue(utils.FormatFloat(d.Await.Last() * 1000))
					}
					ioPercent := widgets2.NewTableCell()
					if d.IOUtilizationPercent != nil {
						if last := d.IOUtilizationPercent.Last(); !math.IsNaN(last) {
							ioPercent.SetValue(fmt.Sprintf("%.0f%%", last))
						}
					}
					space := widgets2.NewTableCell()
					if v.UsedBytes != nil && v.CapacityBytes != nil {
						capacity := v.CapacityBytes.Last()
						usage := v.UsedBytes.Last()
						if usage > 0 && capacity > 0 {
							space.SetValue(fmt.Sprintf(
								"%.0f%% (%s / %s)",
								usage/capacity*100,
								humanize.Bytes(uint64(usage)),
								humanize.Bytes(uint64(capacity))),
							)
						}
					}
					dash.GetOrCreateTable("Volume", "Latency", "I/O", "Space", "Device").AddRow(
						widgets2.NewTableCell(fullName),
						latencyMs,
						ioPercent,
						space,
						widgets2.NewTableCell(v.Device.Value()).AddTag(v.Name.Value()),
					)
				}
				dash.GetOrCreateChartInGroup("Disk space <selector>, bytes", fullName).
					Stacked().
					AddSeries("used", v.UsedBytes).
					SetThreshold("total", v.CapacityBytes, timeseries.Max)
			}
		}
	}
	return dash
}
