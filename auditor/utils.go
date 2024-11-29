package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type nodeConsumers struct {
	cpu    map[string]model.SeriesData
	memory map[string]model.SeriesData
}

func getNodeConsumers(node *model.Node) *nodeConsumers {
	nc := &nodeConsumers{
		cpu:    map[string]model.SeriesData{},
		memory: map[string]model.SeriesData{},
	}
	for _, i := range node.Instances {
		for _, c := range i.Containers {
			app := i.Owner.Id.Name
			if nc.cpu[app] == nil {
				nc.cpu[app] = timeseries.NewAggregate(timeseries.NanSum)
			}
			if nc.memory[app] == nil {
				nc.memory[app] = timeseries.NewAggregate(timeseries.NanSum)
			}
			nc.cpu[app].(*timeseries.Aggregate).Add(c.CpuUsage)
			nc.memory[app].(*timeseries.Aggregate).Add(c.MemoryRss)
		}
	}
	return nc
}

type nodeConsumersByNode map[string]*nodeConsumers

func (m nodeConsumersByNode) get(node *model.Node) *nodeConsumers {
	name := node.GetName()
	ncs := m[name]
	if ncs == nil {
		ncs = getNodeConsumers(node)
		m[name] = ncs
	}
	return ncs
}

func cpuByModeChart(ch *model.Chart, modes map[string]*timeseries.TimeSeries) {
	ch.Sorted()
	ch.Stacked()
	for _, mode := range []string{"user", "nice", "system", "wait", "iowait", "steal", "irq", "softirq"} {
		v, ok := modes[mode]
		if !ok {
			continue
		}
		var color string
		switch mode {
		case "user":
			color = "blue"
		case "system":
			color = "red"
		case "wait", "iowait":
			color = "orange"
		case "steal":
			color = "black"
		case "irq":
			color = "grey"
		case "softirq":
			color = "yellow"
		case "nice":
			color = "lightGreen"
		}
		ch.AddSeries(mode, v, color)
	}
}
