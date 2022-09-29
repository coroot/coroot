package model

import (
	"github.com/coroot/coroot/timeseries"
)

type AuditReport struct {
	ctx     timeseries.Context
	Name    string    `json:"name"`
	Widgets []*Widget `json:"widgets"`
	Checks  []*Check  `json:"checks"`
}

func NewAuditReport(ctx timeseries.Context, name string) *AuditReport {
	return &AuditReport{Name: name, ctx: ctx}
}

type Widget struct {
	Chart         *Chart         `json:"chart"`
	ChartGroup    *ChartGroup    `json:"chart_group"`
	Table         *Table         `json:"table"`
	LogPatterns   *LogPatterns   `json:"log_patterns"`
	DependencyMap *DependencyMap `json:"dependency_map"`

	Width string `json:"width"`
}

func (c *AuditReport) GetOrCreateChartGroup(title string) *ChartGroup {
	for _, w := range c.Widgets {
		if cg := w.ChartGroup; cg != nil {
			if cg.Title == title {
				return cg
			}
		}
	}
	cg := &ChartGroup{Title: title}
	c.Widgets = append(c.Widgets, &Widget{ChartGroup: cg})
	return cg
}

func (c *AuditReport) GetOrCreateChartInGroup(title string, chartTitle string) *Chart {
	return c.GetOrCreateChartGroup(title).GetOrCreateChart(c.ctx, chartTitle)
}

func (c *AuditReport) GetOrCreateChart(title string) *Chart {
	for _, w := range c.Widgets {
		if ch := w.Chart; ch != nil {
			if ch.Title == title {
				return ch
			}
		}
	}
	ch := NewChart(c.ctx, title)
	c.Widgets = append(c.Widgets, &Widget{Chart: ch})
	return ch
}

func (c *AuditReport) GetOrCreateDependencyMap() *DependencyMap {
	for _, w := range c.Widgets {
		if w.DependencyMap != nil {
			return w.DependencyMap
		}
	}
	dm := &DependencyMap{}
	c.Widgets = append(c.Widgets, &Widget{DependencyMap: dm, Width: "100%"})
	return dm
}

func (c *AuditReport) GetOrCreateTable(header ...string) *Table {
	for _, w := range c.Widgets {
		if t := w.Table; t != nil {
			return t
		}
	}
	t := &Table{Header: header}
	c.Widgets = append(c.Widgets, &Widget{Table: t, Width: "100%"})
	return t
}

func (c *AuditReport) AddCheck(id CheckId) *Check {
	ch := &Check{Id: id, Status: OK}
	c.Checks = append(c.Checks, ch)
	return ch
}
