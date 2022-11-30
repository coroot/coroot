package model

import (
	"fmt"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"strconv"
	"strings"
)

type Application struct {
	Id ApplicationId

	Instances []*Instance

	DesiredInstances timeseries.TimeSeries

	Status  Status
	Reports []*AuditReport

	LatencySLIs      []*LatencySLI
	AvailabilitySLIs []*AvailabilitySLI
}

func NewApplication(id ApplicationId) *Application {
	app := &Application{Id: id}
	return app
}

func (app *Application) SLOStatus() Status {
	for _, r := range app.Reports {
		if r.Name == AuditReportSLO {
			return r.Status
		}
	}
	return UNKNOWN
}

func (app *Application) GetInstance(name string) *Instance {
	for _, i := range app.Instances {
		if i.Name == name {
			return i
		}
	}
	return nil
}

func (app *Application) GetOrCreateInstance(name string) *Instance {
	instance := app.GetInstance(name)
	if instance == nil {
		instance = NewInstance(name, app.Id)
		app.Instances = append(app.Instances, instance)
	}
	return instance
}

func (app *Application) Labels() Labels {
	res := Labels{}
	switch app.Id.Kind {
	case ApplicationKindRds:
		res["db"] = fmt.Sprintf(`%s (RDS)`, app.Instances[0].Rds.Engine.Value())
	case ApplicationKindUnknown:
		res["instances"] = strconv.Itoa(len(app.Instances))
	case ApplicationKindExternalService:
		eps := utils.NewStringSet()
		for _, instance := range app.Instances {
			for listen := range instance.TcpListens {
				eps.Add(listen.IP)
			}
		}
		if eps.Len() > 0 {
			name := "external endpoint"
			if eps.Len() > 1 {
				name += "s"
			}
			res[name] = strings.Join(eps.Items(), ", ")
		}
	default:
		res["ns"] = app.Id.Namespace
	}
	for _, i := range app.Instances {
		for _, c := range i.Containers {
			for t := range c.ApplicationTypes {
				if t.IsDatabase() {
					res["db"] = string(t)
				}
				if t.IsQueue() {
					res["queue"] = string(t)
				}
			}
		}
	}
	return res
}

func (app *Application) IsRedis() bool {
	for _, i := range app.Instances {
		if i.Redis != nil {
			return true
		}
	}
	return false
}

func (app *Application) IsPostgres() bool {
	for _, i := range app.Instances {
		if i.Postgres != nil {
			return true
		}
	}
	return false
}

func (app *Application) IsStandalone() bool {
	for _, i := range app.Instances {
		for _, u := range i.Upstreams {
			if u.RemoteInstance != nil && u.RemoteInstance.OwnerId != app.Id && !u.Obsolete() {
				return false
			}
		}
		for _, d := range i.Downstreams {
			if d.Instance != nil && d.Instance.OwnerId != app.Id && !d.Obsolete() {
				return false
			}
		}
	}
	return true
}

func (app *Application) InstrumentationStatus() map[ApplicationType]bool {
	res := map[ApplicationType]bool{}
	for _, i := range app.Instances {
		if i.Pod != nil && i.Pod.IsObsolete() {
			continue
		}
		for t := range i.ApplicationTypes() {
			var instanceInstrumented bool
			switch t {
			case ApplicationTypePostgres:
				instanceInstrumented = i.Postgres != nil
			case ApplicationTypeRedis:
				instanceInstrumented = i.Redis != nil
			default:
				continue
			}
			appInstrumented, visited := res[t]
			res[t] = (appInstrumented || !visited) && instanceInstrumented
		}
	}
	return res
}

func (app *Application) GetClientsConnections() []*Connection {
	var res []*Connection
	for _, i := range app.Instances {
		for _, c := range i.Downstreams {
			if c.Instance.OwnerId == app.Id {
				continue
			}
			res = append(res, c)
		}
	}
	return res
}
