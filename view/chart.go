package view

import "github.com/coroot/coroot-focus/timeseries"

type ChartType string

type Chart struct {
	Title     string
	Series    []*Series
	Threshold *Series
	Featured  bool
	IsStacked bool
	IsSorted  bool
	IsColumn  bool
}

func NewChart(title string) *Chart {
	return &Chart{Title: title}
}

func (chart *Chart) Stacked() *Chart {
	chart.IsStacked = true
	return chart
}

func (chart *Chart) Sorted() *Chart {
	chart.IsSorted = true
	return chart
}

func (chart *Chart) Column() *Chart {
	chart.IsColumn = true
	chart.IsStacked = true
	return chart
}

func (chart *Chart) AddMany(series []timeseries.Named) *Chart {
	for _, v := range series {
		chart.AddSeries(v.Name, v.Series)
	}
	return chart
}

func (chart *Chart) AddSeries(name string, data timeseries.TimeSeries, color ...string) *Chart {
	if data == nil || data.IsEmpty() {
		return chart
	}
	s := &Series{Name: name, Timeseries: data}
	if len(color) > 0 {
		s.Color = color[0]
	}
	chart.Series = append(chart.Series, s)
	return chart
}

func (chart *Chart) SetThreshold(name string, data timeseries.TimeSeries, aggFunc timeseries.F) *Chart {
	if data == nil {
		return chart
	}
	if chart.Threshold == nil {
		chart.Threshold = &Series{Name: name, Timeseries: timeseries.Aggregate(aggFunc), Color: "black"}
	}
	chart.Threshold.Timeseries.(*timeseries.AggregatedTimeseries).AddInput(data)
	return chart
}

func (chart *Chart) Feature() *Chart {
	chart.Featured = true
	return chart
}

type Series struct {
	Name       string
	Color      string
	Timeseries timeseries.TimeSeries
}

type ChartGroup struct {
	Title  string
	Charts []*Chart
}

func (cg *ChartGroup) GetOrCreateChart(title string) *Chart {
	for _, ch := range cg.Charts {
		if ch.Title == title {
			return ch
		}
	}
	ch := NewChart(title)
	cg.Charts = append(cg.Charts, ch)
	return ch
}
