package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/coroot/coroot/timeseries"
)

type Annotation struct {
	Name string          `json:"name"`
	X1   timeseries.Time `json:"x1"`
	X2   timeseries.Time `json:"x2"`
	Icon string          `json:"icon"`
}

type SeriesData interface {
	IsEmpty() bool
	Get() *timeseries.TimeSeries
	Reduce(timeseries.F) float32
}

type Series struct {
	Name      string `json:"name"`
	Title     string `json:"title,omitempty"`
	Color     string `json:"color,omitempty"`
	Fill      bool   `json:"fill,omitempty"`
	Threshold string `json:"threshold,omitempty"`

	Data  SeriesData `json:"data"`
	Value string     `json:"value"`
}

type SeriesList struct {
	series []*Series

	topN int
	topF timeseries.F

	histogram   []HistogramBucket
	percentiles []float32
}

func (sl SeriesList) IsEmpty() bool {
	return len(sl.series) == 0 && (len(sl.histogram) == 0 || len(sl.percentiles) == 0)
}

func (sl SeriesList) MarshalJSON() ([]byte, error) {
	ss := sl.series
	switch {
	case sl.topN > 0 && sl.topF != nil:
		ss = topN(ss, sl.topN, sl.topF)
	case len(sl.histogram) > 0 && len(sl.percentiles) > 0:
		for _, q := range sl.percentiles {
			ss = append(ss, &Series{
				Name: fmt.Sprintf("p%d", int(q*100)),
				Data: Quantile(sl.histogram, q),
			})
		}
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
	HideLegend    bool         `json:"hide_legend"`
}

func NewChart(ctx timeseries.Context, title string) *Chart {
	return &Chart{Ctx: ctx, Title: title}
}

func (ch *Chart) IsEmpty() bool {
	if ch == nil {
		return true
	}
	return ch.Series.IsEmpty() && (ch.Threshold == nil || ch.Threshold.Data.IsEmpty())
}

func (ch *Chart) Stacked() *Chart {
	if ch == nil {
		return nil
	}
	ch.IsStacked = true
	return ch
}

func (ch *Chart) Sorted() *Chart {
	if ch == nil {
		return nil
	}
	ch.IsSorted = true
	return ch
}

func (ch *Chart) Column() *Chart {
	if ch == nil {
		return nil
	}
	ch.IsColumn = true
	ch.IsStacked = true
	return ch
}

func (ch *Chart) Legend(on bool) *Chart {
	if ch == nil {
		return nil
	}
	ch.HideLegend = !on
	return ch
}

func (ch *Chart) ShiftColors() *Chart {
	if ch == nil {
		return nil
	}
	ch.ColorShift = 1
	return ch
}

func (ch *Chart) AddAnnotation(annotations ...Annotation) *Chart {
	if ch == nil {
		return nil
	}
	ch.Annotations = append(ch.Annotations, annotations...)
	return ch
}

func (ch *Chart) AddSeries(name string, data SeriesData, color ...string) *Chart {
	if ch == nil {
		return nil
	}
	if data.IsEmpty() {
		return ch
	}
	s := &Series{Name: name, Data: data}
	if len(color) > 0 {
		s.Color = color[0]
	}
	ch.Series.series = append(ch.Series.series, s)
	return ch
}

func (ch *Chart) AddMany(series map[string]SeriesData, topN int, topF timeseries.F) *Chart {
	if ch == nil {
		return nil
	}
	for name, data := range series {
		ch.AddSeries(name, data)
	}
	ch.Series.topN = topN
	ch.Series.topF = topF
	return ch
}

func (ch *Chart) PercentilesFrom(histogram []HistogramBucket, percentiles ...float32) *Chart {
	if ch == nil {
		return nil
	}
	ch.Series.histogram = histogram
	ch.Series.percentiles = percentiles
	return ch
}

func (ch *Chart) SetThreshold(name string, data SeriesData) *Chart {
	if ch == nil {
		return nil
	}
	if data.IsEmpty() {
		return ch
	}
	ch.Threshold = &Series{Name: name, Color: "black", Data: data}
	return ch
}

func (ch *Chart) Feature() *Chart {
	if ch == nil {
		return nil
	}
	ch.Featured = true
	return ch
}

type ChartGroup struct {
	ctx    timeseries.Context
	Title  string   `json:"title"`
	Charts []*Chart `json:"charts"`
}

func NewChartGroup(ctx timeseries.Context, title string) *ChartGroup {
	return &ChartGroup{ctx: ctx, Title: title}
}

func (cg *ChartGroup) MarshalJSON() ([]byte, error) {
	autoFeatureChart(cg.Charts)
	return json.Marshal(struct {
		Title  string   `json:"title"`
		Charts []*Chart `json:"charts"`
	}{
		Title:  cg.Title,
		Charts: cg.Charts,
	})
}

func (cg *ChartGroup) GetOrCreateChart(title string) *Chart {
	if cg == nil {
		return nil
	}
	for _, ch := range cg.Charts {
		if ch.Title == title {
			return ch
		}
	}
	ch := NewChart(cg.ctx, title)
	cg.Charts = append(cg.Charts, ch)
	return ch
}

type Heatmap struct {
	Ctx timeseries.Context `json:"ctx"`

	Title  string     `json:"title"`
	Series SeriesList `json:"series"`

	Annotations []Annotation `json:"annotations"`

	DrillDownLink *RouterLink `json:"drill_down_link"`
}

func NewHeatmap(ctx timeseries.Context, title string) *Heatmap {
	return &Heatmap{Ctx: ctx, Title: title}
}

func (hm *Heatmap) AddSeries(name, title string, data SeriesData, threshold string, value string) *Heatmap {
	if hm == nil {
		return nil
	}
	if data.IsEmpty() {
		return hm
	}
	s := &Series{Name: name, Title: title, Data: data, Threshold: threshold, Value: value}
	hm.Series.series = append(hm.Series.series, s)
	return hm
}

func (hm *Heatmap) AddAnnotation(annotations ...Annotation) *Heatmap {
	if hm == nil {
		return nil
	}
	hm.Annotations = append(hm.Annotations, annotations...)
	return hm
}

func (hm *Heatmap) IsEmpty() bool {
	if hm == nil {
		return true
	}
	return hm.Series.IsEmpty()
}

func autoFeatureChart(charts []*Chart) {
	if len(charts) < 2 {
		return
	}
	for _, ch := range charts {
		if ch.Featured {
			return
		}
	}
	type weight struct {
		i int
		w float32
	}
	weights := make([]weight, 0, len(charts))
	for i, ch := range charts {
		var w float32
		for _, s := range ch.Series.series {
			w += s.Data.Reduce(timeseries.NanSum)
		}
		weights = append(weights, weight{i: i, w: w})
	}
	sort.Slice(weights, func(i, j int) bool {
		return weights[i].w > weights[j].w
	})
	if weights[0].w/weights[1].w > 1.2 {
		charts[weights[0].i].Featured = true
	}
}

func topN(ss []*Series, n int, by timeseries.F) []*Series {
	type weighted struct {
		*Series
		weight float32
	}
	sortable := make([]weighted, 0, len(ss))
	for _, s := range ss {
		w := s.Data.Reduce(by)
		if !timeseries.IsNaN(w) {
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
			other.Add(s.Data.Get())
		}
	}
	if otherTs := other.Get(); !otherTs.IsEmpty() {
		res = append(res, &Series{Name: "other", Data: otherTs, Color: "grey"})
	}
	return res
}

func EventsToAnnotations(events []*ApplicationEvent, ctx timeseries.Context) []Annotation {
	if len(events) == 0 {
		return nil
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
		if last == nil || e.Start.Sub(last.start) > 3*ctx.Step {
			a := &annotation{start: e.Start, end: e.End, events: []*ApplicationEvent{e}}
			annotations = append(annotations, a)
			continue
		}
		last.events = append(last.events, e)
		last.end = e.End
	}

	res := make([]Annotation, 0, len(annotations))
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
		res = append(res, Annotation{
			Name: strings.Join(msgs, "<br>"),
			X1:   a.start,
			X2:   a.end,
			Icon: icon,
		})
	}
	return res
}

func IncidentsToAnnotations(incidents []*ApplicationIncident, ctx timeseries.Context) []Annotation {
	res := make([]Annotation, 0, len(incidents))
	for _, i := range incidents {
		if !i.Resolved() {
			i.ResolvedAt = ctx.To
		}
		res = append(res, Annotation{Name: "incident", X1: i.OpenedAt, X2: i.ResolvedAt})
	}
	return res
}
