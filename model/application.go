package model

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type Application struct {
	Id ApplicationId

	Category ApplicationCategory

	Instances   []*Instance
	Downstreams []*Connection

	DesiredInstances *timeseries.TimeSeries

	LatencySLIs      []*LatencySLI
	AvailabilitySLIs []*AvailabilitySLI

	Events      []*ApplicationEvent
	Deployments []*ApplicationDeployment
	Incidents   []*ApplicationIncident

	Status  Status
	Reports []*AuditReport
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

func (app *Application) GetInstance(name, node string) *Instance {
	for _, i := range app.Instances {
		if i.Name != name {
			continue
		}
		switch app.Id.Kind {
		case ApplicationKindStatefulSet:
			if node == "" || i.NodeName() == "" || i.NodeName() == node {
				return i
			}
		default:
			return i
		}
	}
	return nil
}

func (app *Application) GetOrCreateInstance(name string, node *Node) *Instance {
	nodeName := ""
	if node != nil {
		nodeName = node.GetName()
	}
	instance := app.GetInstance(name, nodeName)
	if instance == nil {
		instance = NewInstance(name, app.Id)
		app.Instances = append(app.Instances, instance)
		if node != nil {
			instance.Node = node
			node.Instances = append(node.Instances, instance)
		}
	}
	return instance
}

func (app *Application) Labels() Labels {
	res := Labels{}
	switch app.Id.Kind {
	case ApplicationKindRds:
		res["db"] = fmt.Sprintf(`%s (RDS)`, app.Instances[0].Rds.Engine.Value())
	case ApplicationKindElasticacheCluster:
		res["db"] = fmt.Sprintf(`%s (EC)`, app.Instances[0].Elasticache.Engine.Value())
	case ApplicationKindUnknown, ApplicationKindDockerSwarmService:
		if app.Id.Namespace != "_" {
			res["ns"] = app.Id.Namespace
		} else {
			res["instances"] = strconv.Itoa(len(app.Instances))
		}
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

func (app *Application) IsMongodb() bool {
	for _, i := range app.Instances {
		if i.Mongodb != nil {
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

func (app *Application) IsJvm() bool {
	for _, i := range app.Instances {
		if len(i.Jvms) > 0 {
			return true
		}
	}
	return false
}

func (app *Application) IsDotNet() bool {
	for _, i := range app.Instances {
		if len(i.DotNet) > 0 {
			return true
		}
	}
	return false
}

func (app *Application) IsStandalone() bool {
	for _, d := range app.Downstreams {
		if d.Instance.OwnerId != app.Id && !d.IsObsolete() {
			return false
		}
	}
	for _, i := range app.Instances {
		for _, u := range i.Upstreams {
			if u.RemoteInstance != nil && u.RemoteInstance.OwnerId != app.Id && !u.IsObsolete() {
				return false
			}
		}
	}
	return true
}

func (app *Application) IsK8s() bool {
	for _, i := range app.Instances {
		if i.Pod != nil {
			return true
		}
	}
	return false
}

func (app *Application) hasClientsInAWS() bool {
	for _, d := range app.Downstreams {
		if d.Instance != nil && d.Instance.Node != nil {
			provider := d.Instance.Node.CloudProvider.Value()
			if provider == CloudProviderAWS {
				return true
			}
		}
	}
	return false
}

func (app *Application) InstrumentationStatus() map[ApplicationType]bool {
	res := map[ApplicationType]bool{}
	for _, i := range app.Instances {
		if i.IsObsolete() {
			continue
		}
		if app.Id.Kind == ApplicationKindExternalService {
			if !app.hasClientsInAWS() {
				continue
			}
			for l := range i.TcpListens {
				switch l.Port {
				case "5432", "3306":
					res[ApplicationTypeRDS] = false
				case "6379", "11211":
					res[ApplicationTypeElastiCache] = false
				}
			}
		}
		for t := range i.ApplicationTypes() {
			var instanceInstrumented bool
			switch t {
			case ApplicationTypePostgres:
				instanceInstrumented = i.Postgres != nil
			case ApplicationTypeRedis, ApplicationTypeKeyDB:
				t = ApplicationTypeRedis
				instanceInstrumented = i.Redis != nil
			case ApplicationTypeMongodb, ApplicationTypeMongos:
				t = ApplicationTypeMongodb
				instanceInstrumented = i.Mongodb != nil
			default:
				continue
			}
			res[t] = res[t] || instanceInstrumented
		}
	}
	return res
}

func (app *Application) GetClientsConnections() map[ApplicationId][]*Connection {
	res := map[ApplicationId][]*Connection{}
	for _, d := range app.Downstreams {
		if d.Instance.OwnerId == app.Id {
			continue
		}
		res[d.Instance.OwnerId] = append(res[d.Instance.OwnerId], d)
	}
	return res
}

func (app *Application) AddReport(name AuditReportName, widgets ...*Widget) {
	app.Reports = append(app.Reports, &AuditReport{Name: name, Widgets: widgets})
}
