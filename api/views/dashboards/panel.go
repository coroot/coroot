package dashboards

import (
	"context"
	"fmt"
	"regexp"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"golang.org/x/exp/maps"
)

type PanelData struct {
	Chart *model.Chart `json:"chart,omitempty"`
}

func (ds *Dashboards) PanelData(ctx context.Context, promClients map[string]prom.Client, config db.DashboardPanel, from, to timeseries.Time, step timeseries.Duration) (*PanelData, error) {
	var res PanelData
	switch {
	case config.Source.Metrics != nil:
		for _, q := range config.Source.Metrics.Queries {
			if q.Query == "" {
				continue
			}
			if len(promClients) == 1 && q.DataSource == "" {
				q.DataSource = maps.Keys(promClients)[0]
			}
			pc, ok := promClients[q.DataSource]
			if !ok {
				return nil, fmt.Errorf("invalid datasource")
			}

			mvs, err := pc.QueryRange(ctx, q.Query, prom.FilterLabelsKeepAll, from, to, step)
			if err != nil {
				return nil, err
			}
			for _, mv := range mvs {
				name := q.Legend
				if name != "" {
					for k, v := range mv.Labels {
						if r, _ := regexp.Compile(fmt.Sprintf(`{{\s*%s\s*}}`, k)); r != nil {
							name = r.ReplaceAllString(name, v)
						}
					}
				}
				if name == "" {
					name = mv.Labels.String()
				}
				if name == "" {
					name = q.Query
				}
				if chart := config.Widget.Chart; chart != nil {
					if res.Chart == nil {
						res.Chart = model.NewChart(timeseries.NewContext(from, to, step), "")
					}
					if chart.Stacked {
						res.Chart = res.Chart.Stacked()
					}
					if chart.Display == "bar" {
						res.Chart = res.Chart.Column()
					}
					res.Chart.AddSeries(name, mv.Values)
				}
			}
		}
	}
	return &res, nil
}
