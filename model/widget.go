package model

type Widget struct {
	Chart         *Chart         `json:"chart,omitempty"`
	ChartGroup    *ChartGroup    `json:"chart_group,omitempty"`
	Table         *Table         `json:"table,omitempty"`
	DependencyMap *DependencyMap `json:"dependency_map,omitempty"`
	Heatmap       *Heatmap       `json:"heatmap,omitempty"`

	Logs      *Logs      `json:"logs,omitempty"`
	Profiling *Profiling `json:"profiling,omitempty"`
	Tracing   *Tracing   `json:"tracing,omitempty"`

	Width   string   `json:"width,omitempty"`
	DocLink *DocLink `json:"doc_link,omitempty"`
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
	if w.Heatmap != nil {
		w.Heatmap.AddAnnotation(annotations...)
	}
}

type DocLink struct {
	Group string `json:"group"`
	Item  string `json:"item"`
	Hash  string `json:"hash"`
}

func NewDocLink(group, item, hash string) *DocLink {
	return &DocLink{
		Group: group,
		Item:  item,
		Hash:  hash,
	}
}

type RouterLink struct {
	Title  string         `json:"title"`
	Route  string         `json:"name,omitempty"`
	Params map[string]any `json:"params,omitempty"`
	Args   map[string]any `json:"query,omitempty"`
	Hash   string         `json:"hash,omitempty"`
}

func NewRouterLink(title string, route string) *RouterLink {
	return &RouterLink{Title: title, Route: route}
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

type Logs struct {
	ApplicationId ApplicationId `json:"application_id"`
	Check         *Check        `json:"check"`
}

type Profiling struct {
	ApplicationId ApplicationId `json:"application_id"`
}

type Tracing struct {
	ApplicationId ApplicationId `json:"application_id"`
}
