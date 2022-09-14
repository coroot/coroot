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
}

func NewApplication(id ApplicationId) *Application {
	app := &Application{Id: id}
	return app
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

func (app *Application) IsControlPlane() bool {
	id := app.Id
	if id.Kind == ApplicationKindExternalService && id.Name == "kube-apiserver" {
		return true
	}
	if id.Namespace == "kube-system" || id.Name == "kubelet" {
		return true
	}
	for _, i := range app.Instances {
		if !i.ApplicationTypes()["etcd"] {
			continue
		}
		for _, d := range i.Downstreams {
			if d.Instance != nil || d.Instance.ApplicationTypes()["kube-apiserver"] {
				return true
			}
		}
	}
	if strings.Contains(id.Namespace, "chaos") {
		return utils.NewStringSet("chaos-controller-manager", "chaos-daemon", "chaos-dashboard").Has(id.Name)
	}
	if id.Name == "postgres-operator" {
		return true
	}
	return false
}

func (app *Application) IsMonitoring() bool {
	id := app.Id
	for _, n := range []string{"monitoring", "prometheus", "grafana", "alertmanager", "coroot"} {
		if strings.Contains(id.Namespace, n) || strings.Contains(id.Name, n) {
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
