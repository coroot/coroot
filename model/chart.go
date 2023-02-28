package model

import (
	"github.com/coroot/coroot/timeseries"
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

type Chart struct {
	Ctx timeseries.Context `json:"ctx"`

	Title         string       `json:"title"`
	Series        []*Series    `json:"series"`
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

func (chart *Chart) AddMany(series []timeseries.Named) *Chart {
	for _, v := range series {
		chart.AddSeries(v.Name, v.Series)
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
	chart.Series = append(chart.Series, s)
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

type Series struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	Fill  bool   `json:"fill"`

	Data *timeseries.TimeSeries `json:"data"`
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
		for _, s := range ch.Series {
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
