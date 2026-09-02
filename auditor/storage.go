package auditor

import (
	"cmp"
	"fmt"
	"slices"

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

	ioCheck.AddWidget(ioLoadChart.Widget())
	ioCheck.AddWidget(ioLatencyChart.Widget())
	spaceCheck.AddWidget(spaceChart.Widget())

	seenVolumes := false
	investigated := map[string]bool{}
	ioInvestigated := map[string]bool{}
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
						if !ioInvestigated[i.Name] {
							ioInvestigated[i.Name] = true
							pgIOFindings(i, d, load, ioCheck)
						}
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
							if !investigated[i.Name] {
								investigated[i.Name] = true
								diskUsageFindings(i, capacity, a.w.Ctx, spaceCheck)
							}
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

const diskFindingMinGrowthBytes = 256 * 1024 * 1024
const diskFindingSignificantFraction = 0.02

func diskFindingMinGrowth(capacity float32) float32 {
	minGrowth := float32(diskFindingMinGrowthBytes)
	if capacity > 0 && capacity*diskFindingSignificantFraction > minGrowth {
		minGrowth = capacity * diskFindingSignificantFraction
	}
	return minGrowth
}

func diskUsageFindings(i *model.Instance, capacity float32, ctx timeseries.Context, check *model.Check) {
	switch {
	case i.Postgres != nil:
		pgDiskUsageFindings(i, capacity, ctx, check)
	case i.Mysql != nil:
		mysqlDiskUsageFindings(i, capacity, ctx, check)
	case i.Mongodb != nil:
		mongoDiskUsageFindings(i, capacity, check)
	}
}

const diskFindingMinGrowthRateBytesPerSecond = 16 * 1024 // ~1.35 GB/day

func avgGrowthRate(ts *timeseries.TimeSeries) float32 {
	if ts.IsEmpty() {
		return 0
	}
	var sum float32
	var n int
	iter := ts.Iter()
	for iter.Next() {
		if _, v := iter.Value(); !timeseries.IsNaN(v) {
			sum += v
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float32(n)
}

type diskGrower struct {
	name string
	rate float32 // bytes/second
	size float32 // current total size, 0 if unknown
}

func tableGrowers(noun string, growthRate, size map[model.DbTableKey]*timeseries.TimeSeries) []diskGrower {
	var res []diskGrower
	for k, ts := range growthRate {
		if rate := avgGrowthRate(ts); rate >= diskFindingMinGrowthRateBytesPerSecond {
			res = append(res, diskGrower{noun + " " + k.String(), rate, lastSize(size[k])})
		}
	}
	return res
}

func seriesGrowthRate(ts *timeseries.TimeSeries, ctx timeseries.Context) (rate, size float32) {
	window := float32(ctx.To - ctx.From)
	if window <= 0 {
		return 0, 0
	}
	growth, last := seriesGrowth(ts)
	return growth / window, last
}

func reportGrowers(instanceName string, check *model.Check, growers []diskGrower) int {
	slices.SortFunc(growers, func(a, b diskGrower) int { return cmp.Compare(b.rate, a.rate) })
	growers = growers[:min(3, len(growers))]
	for _, g := range growers {
		if g.size > 0 {
			check.AddDetail("%s: %s is growing at %s/s (%s total)", instanceName, g.name,
				humanize.Bytes(uint64(g.rate)), humanize.Bytes(uint64(g.size)))
		} else {
			check.AddDetail("%s: %s is growing at %s/s", instanceName, g.name, humanize.Bytes(uint64(g.rate)))
		}
	}
	return len(growers)
}

func reportLargest(instanceName, noun string, capacity float32, tableSize map[model.DbTableKey]*timeseries.TimeSeries, dbSize map[string]*timeseries.TimeSeries, check *model.Check) {
	minSize := diskFindingMinGrowth(capacity)
	type item struct {
		name string
		size float32
	}
	var items []item
	for k, ts := range tableSize {
		if s := lastSize(ts); s >= minSize {
			items = append(items, item{noun + " " + k.String(), s})
		}
	}
	if len(items) == 0 {
		for db, ts := range dbSize {
			if s := lastSize(ts); s >= minSize {
				items = append(items, item{"database " + db, s})
			}
		}
	}
	slices.SortFunc(items, func(a, b item) int { return cmp.Compare(b.size, a.size) })
	for _, it := range items[:min(3, len(items))] {
		check.AddDetail("%s: %s is %s", instanceName, it.name, humanize.Bytes(uint64(it.size)))
	}
}

func lastSize(ts *timeseries.TimeSeries) float32 {
	if ts == nil {
		return 0
	}
	if last := ts.Last(); !timeseries.IsNaN(last) {
		return last
	}
	return 0
}
