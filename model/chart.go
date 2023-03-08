package model

import (
	"encoding/json"
	"github.com/coroot/coroot/timeseries"
	"math"
	"sort"
	"strings"
)

type ChartType string

type Annotation struct {
	Name string          `json:"name"`
	X1   timeseries.Time `json:"x1"`
	X2   timeseries.Time `json:"x2"`
	Icon string          `json:"icon"`
}

type Series struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	Fill  bool   `json:"fill"`

	Data *timeseries.TimeSeries `json:"data"`
}

type SeriesList struct {
	series []*Series
	topN   int
	topF   timeseries.F
}

func (sl SeriesList) MarshalJSON() ([]byte, error) {
	ss := sl.series
	if sl.topN > 0 && sl.topF != nil {
		ss = topN(ss, sl.topN, sl.topF)
	}
	return json.Marshal(ss)
}

type Chart struct {
	Ctx timeseries.Context `json:"ctx"`

	Title         string       `json:"title"`
	Series        SeriesList   `json:"series"`
	Threshold     *Series      `json:"threshold"`
	Featured      bool         `json:"featured"`
	IsStacked     bool         `json:"stacked"`
	IsSorted      bool         `json:"sorted"`
	IsColumn      bool         `json:"column"`
	ColorShift    int          `json:"color_shift"`
	Annotations   []Annotation `json:"annotations"`
	DrillDownLink *RouterLink  `json:"drill_down_link"`
}

func NewChart(ctx timeseries.Context, title string) *Chart {
	return &Chart{Ctx: ctx, Title: title}
}

func (chart *Chart) IsEmpty() bool {
	return len(chart.Series.series) == 0
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

func (chart *Chart) ShiftColors() *Chart {
	chart.ColorShift = 1
	return chart
}

func (chart *Chart) AddAnnotation(name string, start, end timeseries.Time, icon string) *Chart {
	chart.Annotations = append(chart.Annotations, Annotation{X1: start, X2: end, Name: name, Icon: icon})
	return chart
}

func (chart *Chart) AddEventsAnnotations(events []*ApplicationEvent) *Chart {
	if len(events) == 0 {
		return chart
	}

	type annotation struct {
		start  timeseries.Time
		end    timeseries.Time
		events []*ApplicationEvent
	}
	var annotations []*annotation
	getLast := func() *annotation {
		if len(annotations) == 0 {
			return nil
		}
		return annotations[len(annotations)-1]
	}
	for _, e := range events {
		last := getLast()
		if last == nil || e.Start.Sub(last.start) > 3*chart.Ctx.Step {
			a := &annotation{start: e.Start, end: e.End, events: []*ApplicationEvent{e}}
			annotations = append(annotations, a)
			continue
		}
		last.events = append(last.events, e)
		last.end = e.End
	}

	for _, a := range annotations {
		sort.Slice(a.events, func(i, j int) bool {
			return a.events[i].Type < a.events[j].Type
		})
		icon := ""
		var msgs []string
		for _, e := range a.events {
			i := ""
			switch e.Type {
			case ApplicationEventTypeRollout:
				msgs = append(msgs, "deployment "+e.Details)
				i = "mdi-swap-horizontal-circle-outline"
			case ApplicationEventTypeSwitchover:
				msgs = append(msgs, "switchover "+e.Details)
				i = "mdi-database-sync-outline"
			case ApplicationEventTypeInstanceUp:
				msgs = append(msgs, e.Details+" is up")
				i = "mdi-alert-octagon-outline"
			case ApplicationEventTypeInstanceDown:
				msgs = append(msgs, e.Details+" is down")
				i = "mdi-alert-octagon-outline"
			}
			if icon == "" {
				icon = i
			}
		}
		chart.AddAnnotation(strings.Join(msgs, "<br>"), a.start, a.end, icon)
	}
	return chart
}

func (chart *Chart) AddSeries(name string, data *timeseries.TimeSeries, color ...string) *Chart {
	if data.IsEmpty() {
		return chart
	}
	s := &Series{Name: name, Data: data}
	if len(color) > 0 {
		s.Color = color[0]
	}
	chart.Series.series = append(chart.Series.series, s)
	return chart
}

func (chart *Chart) AddMany(series map[string]*timeseries.TimeSeries, topN int, topF timeseries.F) *Chart {
	for name, data := range series {
		chart.AddSeries(name, data)
	}
	chart.Series.topN = topN
	chart.Series.topF = topF
	return chart
}

func (chart *Chart) SetThreshold(name string, data *timeseries.TimeSeries) *Chart {
	if data == nil {
		return chart
	}
	chart.Threshold = &Series{Name: name, Color: "black", Data: data}
	return chart
}

func (chart *Chart) Feature() *Chart {
	chart.Featured = true
	return chart
}

type ChartGroup struct {
	Title  string   `json:"title"`
	Charts []*Chart `json:"charts"`
}

func (cg *ChartGroup) GetOrCreateChart(ctx timeseries.Context, title string) *Chart {
	for _, ch := range cg.Charts {
		if ch.Title == title {
			return ch
		}
	}
	ch := NewChart(ctx, title)
	cg.Charts = append(cg.Charts, ch)
	return ch
}

func (cg *ChartGroup) AutoFeatureChart() {
	if len(cg.Charts) < 2 {
		return
	}
	type weightedChart struct {
		ch *Chart
		w  float64
	}
	for _, ch := range cg.Charts {
		if ch.Featured {
			return
		}
	}
	charts := make([]weightedChart, 0, len(cg.Charts))
	for _, ch := range cg.Charts {
		var w float64
		for _, s := range ch.Series.series {
			w += s.Data.Reduce(timeseries.NanSum)
		}
		charts = append(charts, weightedChart{ch: ch, w: w})
	}
	sort.Slice(charts, func(i, j int) bool {
		return charts[i].w > charts[j].w
	})
	if charts[0].w/charts[1].w > 1.2 {
		charts[0].ch.Featured = true
	}
}

func topN(ss []*Series, n int, by timeseries.F) []*Series {
	type weighted struct {
		*Series
		weight float64
	}
	sortable := make([]weighted, 0, len(ss))
	for _, s := range ss {
		w := s.Data.Reduce(by)
		if !math.IsNaN(w) {
			sortable = append(sortable, weighted{Series: s, weight: w})
		}
	}
	sort.Slice(sortable, func(i, j int) bool {
		return sortable[i].weight > sortable[j].weight
	})
	res := make([]*Series, 0, n+1)
	other := timeseries.NewAggregate(timeseries.NanSum)
	for i, s := range sortable {
		if (i + 1) < n {
			res = append(res, s.Series)
		} else {
			other.Add(s.Data)
		}
	}
	if otherTs := other.Get(); !otherTs.IsEmpty() {
		res = append(res, &Series{Name: "other", Data: otherTs})
	}
	return res
}
