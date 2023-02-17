package model

type Widget struct {
	Chart         *Chart         `json:"chart,omitempty"`
	ChartGroup    *ChartGroup    `json:"chart_group,omitempty"`
	Table         *Table         `json:"table,omitempty"`
	LogPatterns   *LogPatterns   `json:"log_patterns,omitempty"`
	DependencyMap *DependencyMap `json:"dependency_map,omitempty"`
	Profile       *Profile       `json:"profile,omitempty"`

	Width string `json:"width,omitempty"`
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
