package widgets

type Dashboard struct {
	Name    string    `json:"name"`
	Widgets []*Widget `json:"widgets"`
}

type Widget struct {
	Chart         *Chart         `json:"chart"`
	ChartGroup    *ChartGroup    `json:"chart_group"`
	Table         *Table         `json:"table"`
	LogPatterns   *LogPatterns   `json:"log_patterns"`
	DependencyMap *DependencyMap `json:"dependency_map"`
}

func (d *Dashboard) GetOrCreateChartGroup(title string) *ChartGroup {
	for _, w := range d.Widgets {
		if cg := w.ChartGroup; cg != nil {
			if cg.Title == title {
				return cg
			}
		}
	}
	cg := &ChartGroup{Title: title}
	d.Widgets = append(d.Widgets, &Widget{ChartGroup: cg})
	return cg
}

func (d *Dashboard) GetOrCreateChartInGroup(title string, chartTitle string) *Chart {
	return d.GetOrCreateChartGroup(title).GetOrCreateChart(chartTitle)
}

func (d *Dashboard) GetOrCreateChart(title string) *Chart {
	for _, w := range d.Widgets {
		if ch := w.Chart; ch != nil {
			if ch.Title == title {
				return ch
			}
		}
	}
	ch := &Chart{Title: title}
	d.Widgets = append(d.Widgets, &Widget{Chart: ch})
	return ch
}

func (d *Dashboard) GetOrCreateDependencyMap() *DependencyMap {
	for _, w := range d.Widgets {
		if w.DependencyMap != nil {
			return w.DependencyMap
		}
	}
	dm := &DependencyMap{}
	d.Widgets = append(d.Widgets, &Widget{DependencyMap: dm})
	return dm
}
