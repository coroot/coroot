package model

import (
	"github.com/coroot/coroot/timeseries"
)

type ApplicationKind string

const (
	ApplicationKindDeployment      ApplicationKind = "Deployment"
	ApplicationKindStatefulSet     ApplicationKind = "StatefulSet"
	ApplicationKindDaemonSet       ApplicationKind = "DaemonSet"
	ApplicationKindCronJob         ApplicationKind = "CronJob"
	ApplicationKindJob             ApplicationKind = "Job"
	ApplicationKindReplicaSet      ApplicationKind = "ReplicaSet"
	ApplicationKindPod             ApplicationKind = "Pod"
	ApplicationKindStaticPods      ApplicationKind = "StaticPods"
	ApplicationKindUnknown         ApplicationKind = "Unknown"
	ApplicationKindExternalService ApplicationKind = "ExternalService"
	ApplicationKindDatabaseCluster ApplicationKind = "DatabaseCluster"
	ApplicationKindRds             ApplicationKind = "RDS"
	ApplicationKindNode            ApplicationKind = "Node"
)

type Job struct{}

type CronJob struct {
	Schedule          LabelLastValue
	ConcurrencyPolicy LabelLastValue
	StatusActive      timeseries.TimeSeries
	LastScheduleTime  timeseries.TimeSeries
	NextScheduleTime  timeseries.TimeSeries
}

type DaemonSet struct {
	ReplicasDesired timeseries.TimeSeries
}

type ReplicaSet struct {
}

type Deployment struct {
	ReplicasDesired timeseries.TimeSeries
	ReplicaSets     map[string]*ReplicaSet
}

type StatefulSet struct {
	ReplicasDesired timeseries.TimeSeries
	ReplicasUpdated timeseries.TimeSeries
}

type Service struct {
	Name      string
	Namespace string
	ClusterIP string

	Connections []*Connection
}

func (svc *Service) GetDestinationApplicationId() (ApplicationId, bool) {
	apps := map[ApplicationId]bool{}
	for _, c := range svc.Connections {
		if c.RemoteInstance != nil {
			apps[c.RemoteInstance.OwnerId] = true
		}
	}
	if len(apps) == 1 {
		for id := range apps {
			return id, true
		}
	}
	return ApplicationId{}, false
}
