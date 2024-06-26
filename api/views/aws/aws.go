package aws

import (
	"github.com/coroot/coroot/model"
	"golang.org/x/exp/maps"
)

type View struct {
	Errors    []string   `json:"errors"`
	Instances []Instance `json:"instances"`
}

type Instance struct {
	ApplicationId    model.ApplicationId `json:"application_id"`
	Name             string              `json:"name"`
	Status           string              `json:"status"`
	Engine           string              `json:"engine"`
	EngineVersion    string              `json:"engine_version"`
	InstanceType     string              `json:"instance_type"`
	AvailabilityZone string              `json:"availability_zone"`
}

func Render(w *model.World) *View {
	v := &View{}

	if w == nil {
		return v
	}

	v.Errors = maps.Keys(w.AWS.DiscoveryErrors)

	for _, app := range w.Applications {
		for _, i := range app.Instances {
			ii := Instance{
				ApplicationId: app.Id,
				Name:          i.Name,
			}
			if i.Node != nil {
				ii.InstanceType = i.Node.InstanceType.Value()
				ii.AvailabilityZone = i.Node.AvailabilityZone.Value()
			}
			switch {
			case i.Rds != nil:
				ii.Status = i.Rds.Status.Value()
				ii.Engine = i.Rds.Engine.Value()
				ii.EngineVersion = i.Rds.EngineVersion.Value()
			case i.Elasticache != nil:
				ii.Status = i.Elasticache.Status.Value()
				ii.Engine = i.Elasticache.Engine.Value()
				ii.EngineVersion = i.Elasticache.EngineVersion.Value()
			default:
				continue
			}
			v.Instances = append(v.Instances, ii)
		}
	}

	return v
}
