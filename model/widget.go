package model

type Widget struct {
	Chart         *Chart         `json:"chart,omitempty"`
	ChartGroup    *ChartGroup    `json:"chart_group,omitempty"`
	Table         *Table         `json:"table,omitempty"`
	LogPatterns   *LogPatterns   `json:"log_patterns,omitempty"`
	DependencyMap *DependencyMap `json:"dependency_map,omitempty"`
	Profile       *Profile       `json:"profile,omitempty"`
	Heatmap       *Heatmap       `json:"heatmap,omitempty"`

	Width string `json:"width,omitempty"`
}

func (w *Widget) AddAnnotation(annotations ...Annotation) {
	if w.Chart != nil {
		w.Chart.AddAnnotation(annotations...)
	}
	if w.ChartGroup != nil {
		for _, ch := range w.ChartGroup.Charts {
			ch.AddAnnotation(annotations...)
		}
	}
	if w.LogPatterns != nil {
		for _, p := range w.LogPatterns.Patterns {
			if p.Instances != nil {
				p.Instances.AddAnnotation(annotations...)
			}
		}
	}
	if w.Heatmap != nil {
		w.Heatmap.AddAnnotation(annotations...)
	}
}

type RouterLink struct {
	Title  string         `json:"title"`
	Route  string         `json:"name,omitempty"`
	Params map[string]any `json:"params,omitempty"`
	Args   map[string]any `json:"query,omitempty"`
	Hash   string         `json:"hash,omitempty"`
}

func NewRouterLink(title string) *RouterLink {
	return &RouterLink{Title: title}
}

func (l *RouterLink) SetRoute(v string) *RouterLink {
	l.Route = v
	return l
}

func (l *RouterLink) SetParam(k string, v any) *RouterLink {
	if l.Params == nil {
		l.Params = map[string]any{}
	}
	l.Params[k] = v
	return l
}

func (l *RouterLink) SetArg(k string, v any) *RouterLink {
	if l.Args == nil {
		l.Args = map[string]any{}
	}
	l.Args[k] = v
	return l
}

func (l *RouterLink) SetHash(v string) *RouterLink {
	l.Hash = "#" + v
	return l
}

type Profile struct {
	ApplicationId ApplicationId `json:"application_id"`
}
