package model

import (
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type ApplicationKind string

const (
	ApplicationKindDeployment         ApplicationKind = "Deployment"
	ApplicationKindStatefulSet        ApplicationKind = "StatefulSet"
	ApplicationKindDaemonSet          ApplicationKind = "DaemonSet"
	ApplicationKindCronJob            ApplicationKind = "CronJob"
	ApplicationKindJob                ApplicationKind = "Job"
	ApplicationKindReplicaSet         ApplicationKind = "ReplicaSet"
	ApplicationKindPod                ApplicationKind = "Pod"
	ApplicationKindStaticPods         ApplicationKind = "StaticPods"
	ApplicationKindUnknown            ApplicationKind = "Unknown"
	ApplicationKindDockerSwarmService ApplicationKind = "DockerSwarmService"
	ApplicationKindExternalService    ApplicationKind = "ExternalService"
	ApplicationKindDatabaseCluster    ApplicationKind = "DatabaseCluster"
	ApplicationKindRds                ApplicationKind = "RDS"
	ApplicationKindElasticacheCluster ApplicationKind = "ElasticacheCluster"
	ApplicationKindNomadJobGroup      ApplicationKind = "NomadJobGroup"
	ApplicationKindArgoWorkflow       ApplicationKind = "Workflow"
	ApplicationKindSparkApplication   ApplicationKind = "SparkApplication"
)

type Job struct{}

type CronJob struct {
	Schedule          LabelLastValue
	ConcurrencyPolicy LabelLastValue
	StatusActive      *timeseries.TimeSeries
	LastScheduleTime  *timeseries.TimeSeries
	NextScheduleTime  *timeseries.TimeSeries
}

type DaemonSet struct {
	ReplicasDesired *timeseries.TimeSeries
}

type ReplicaSet struct {
}

type Deployment struct {
	ReplicasDesired *timeseries.TimeSeries
	ReplicaSets     map[string]*ReplicaSet
}

type StatefulSet struct {
	ReplicasDesired *timeseries.TimeSeries
	ReplicasUpdated *timeseries.TimeSeries
}

const (
	ServiceTypeNodePort     = "NodePort"
	ServiceTypeLoadBalancer = "LoadBalancer"
)

type Service struct {
	Name            string
	Namespace       string
	ClusterIP       string
	Type            LabelLastValue
	EndpointIPs     *utils.StringSet
	LoadBalancerIPs *utils.StringSet
	DestinationApps map[ApplicationId]*Application
}

func (svc *Service) GetDestinationApplication() *Application {
	if len(svc.DestinationApps) == 1 {
		for _, app := range svc.DestinationApps {
			return app
		}
	}
	return nil
}
