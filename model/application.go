package model

import (
	"github.com/coroot/coroot-focus/utils"
	"strings"
)

type ApplicationType string

type Application struct {
	Id ApplicationId

	Instances []*Instance
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
	return nil
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
