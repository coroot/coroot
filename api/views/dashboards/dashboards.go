package dashboards

import "github.com/coroot/coroot/db"

type Dashboards struct {
}

type Dashboard struct {
	Id          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Config      *db.DashboardConfig `json:"config,omitempty"`
}

func (ds *Dashboards) List(dashboards []*db.Dashboard) []Dashboard {
	res := make([]Dashboard, 0, len(dashboards))
	for _, d := range dashboards {
		dd := Dashboard{
			Id:          d.Id,
			Name:        d.Name,
			Description: d.Description,
		}
		res = append(res, dd)
	}
	return res
}

func (ds *Dashboards) Dashboard(dashboard *db.Dashboard) Dashboard {
	dd := Dashboard{
		Id:          dashboard.Id,
		Name:        dashboard.Name,
		Description: dashboard.Description,
		Config:      &dashboard.Config,
	}
	return dd
}
