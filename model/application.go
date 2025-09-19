package model

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"golang.org/x/exp/maps"
)

type Application struct {
	Id ApplicationId

	Custom   bool
	Category ApplicationCategory

	Annotations ApplicationAnnotations

	Instances       []*Instance
	instancesByName map[string][]*Instance

	Downstreams map[ApplicationId]*AppToAppConnection
	Upstreams   map[ApplicationId]*AppToAppConnection

	TrafficStats TrafficStats

	DesiredInstances *timeseries.TimeSeries

	LatencySLIs      []*LatencySLI
	AvailabilitySLIs []*AvailabilitySLI

	Events      []*ApplicationEvent
	Deployments []*ApplicationDeployment
	Incidents   []*ApplicationIncident

	LogMessages map[Severity]*LogMessages

	Status  Status
	Reports []*AuditReport

	Settings *ApplicationSettings

	KubernetesServices []*Service

	DNSRequests          map[DNSRequest]map[string]*timeseries.TimeSeries
	DNSRequestsHistogram map[float32]*timeseries.TimeSeries
}

func NewApplication(id ApplicationId) *Application {
	app := &Application{
		Id:              id,
		Annotations:     ApplicationAnnotations{},
		instancesByName: map[string][]*Instance{},
		LogMessages:     map[Severity]*LogMessages{},
		Upstreams:       map[ApplicationId]*AppToAppConnection{},
		Downstreams:     map[ApplicationId]*AppToAppConnection{},

		DNSRequests:          map[DNSRequest]map[string]*timeseries.TimeSeries{},
		DNSRequestsHistogram: map[float32]*timeseries.TimeSeries{},
	}
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
	for _, i := range app.instancesByName[name] {
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
		instance = NewInstance(name, app)
		app.Instances = append(app.Instances, instance)
		app.instancesByName[name] = append(app.instancesByName[name], instance)
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
	case ApplicationKindUnknown, ApplicationKindDockerSwarmService, ApplicationKindNomadJobGroup:
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
			value := eps.Items()[0]
			if eps.Len() > 1 {
				name += "s"
				value += ",..."
			}
			res[name] = value
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

func (app *Application) IsMysql() bool {
	for _, i := range app.Instances {
		if i.Mysql != nil {
			return true
		}
	}
	return false
}

func (app *Application) IsMemcached() bool {
	for _, i := range app.Instances {
		if i.Memcached != nil {
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

func (app *Application) IsPython() bool {
	for _, i := range app.Instances {
		if i.Python != nil {
			return true
		}
	}
	return false
}

func (app *Application) IsNodejs() bool {
	for _, i := range app.Instances {
		if i.Nodejs != nil {
			return true
		}
	}
	return false
}

func (app *Application) IsStandalone() bool {
	for _, d := range app.Downstreams {
		if d.Application != d.RemoteApplication {
			return false
		}
	}
	for _, u := range app.Upstreams {
		if u.Application != u.RemoteApplication {
			return false
		}
	}
	return true
}

func (app *Application) IsDatabase() bool {
	if app.Id.Kind == ApplicationKindRds || app.Id.Kind == ApplicationKindElasticacheCluster {
		return true
	}
	for t := range app.ApplicationTypes() {
		if t.IsDatabase() {
			return true
		}
	}
	return false
}

func (app *Application) IsQueue() bool {
	for t := range app.ApplicationTypes() {
		if t.IsQueue() {
			return true
		}
	}
	return false
}

func (app *Application) IsK8s() bool {
	switch app.Id.Kind {
	case ApplicationKindCronJob, ApplicationKindJob, ApplicationKindDeployment, ApplicationKindDaemonSet,
		ApplicationKindPod, ApplicationKindReplicaSet, ApplicationKindStatefulSet, ApplicationKindStaticPods,
		ApplicationKindArgoWorkflow, ApplicationKindSparkApplication:
		return true
	}

	for _, i := range app.Instances {
		if i.Pod != nil {
			return true
		}
	}
	return false
}

func (app *Application) hasClientsInAWS() bool {
	for _, d := range app.Downstreams {
		for _, i := range d.Application.Instances {
			if i.Node != nil {
				provider := i.Node.CloudProvider.Value()
				if provider == CloudProviderAWS {
					return true
				}
			}
		}
	}
	return false
}

func (app *Application) AddReport(name AuditReportName, widgets ...*Widget) {
	app.Reports = append(app.Reports, &AuditReport{Name: name, Widgets: widgets})
}

func (app *Application) ApplicationTypes() map[ApplicationType]bool {
	res := map[ApplicationType]bool{}
	for _, i := range app.Instances {
		for t := range i.ApplicationTypes() {
			res[t] = true
		}
	}

	if app.Id.Kind == ApplicationKindExternalService {
		for _, d := range app.Downstreams {
			for p := range d.RequestsCount {
				t := p.ToApplicationType()
				if t == ApplicationTypeUnknown {
					continue
				}
				res[t] = true
			}
		}
	}

	return res
}

func (app *Application) ApplicationType() ApplicationType {
	types := maps.Keys(app.ApplicationTypes())
	if len(types) == 0 {
		return ApplicationTypeUnknown
	}

	if len(types) == 1 {
		return types[0]
	}
	sort.Slice(types, func(i, j int) bool {
		ti, tj := types[i], types[j]
		tiw, tjw := ti.Weight(), tj.Weight()
		if tiw == tjw {
			return ti < tj
		}
		return tiw < tjw
	})
	return types[0]
}

func (app *Application) PeriodicJob() bool {
	switch app.Id.Kind {
	case ApplicationKindJob, ApplicationKindCronJob:
		return true
	}
	for _, i := range app.Instances {
		for _, c := range i.Containers {
			if c.PeriodicSystemdJob {
				return true
			}
		}
	}
	return false
}

func (app *Application) IsCorootComponent() bool {
	for t := range app.ApplicationTypes() {
		if t.IsCorootComponent() {
			return true
		}
	}
	return false
}
