package cloud

import (
	"strings"

	"slices"

	"github.com/coroot/coroot/model"
	"golang.org/x/exp/maps"
)

type View struct {
	Configured bool       `json:"configured"`
	Detected   bool       `json:"detected"`
	Errors     []string   `json:"errors"`
	Instances  []Instance `json:"instances"`
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

func AWS(w *model.World, configured bool) *View {
	v := &View{Configured: configured}
	if w == nil {
		return v
	}
	v.Configured = configured || w.AWS.Configured
	v.Detected = detected(w, model.CloudProviderAWS)
	v.Errors = errors(w.AWS.DiscoveryErrors)
	v.Instances = instances(w, func(i *model.Instance) (model.LabelLastValue, model.LabelLastValue, model.LabelLastValue, bool) {
		switch {
		case i.Rds != nil:
			return i.Rds.Status, i.Rds.Engine, i.Rds.EngineVersion, true
		case i.Elasticache != nil:
			return i.Elasticache.Status, i.Elasticache.Engine, i.Elasticache.EngineVersion, true
		}
		return model.LabelLastValue{}, model.LabelLastValue{}, model.LabelLastValue{}, false
	})
	return v
}

func GCP(w *model.World) *View {
	v := &View{}
	if w == nil {
		return v
	}
	v.Configured = w.GCP.Configured
	v.Detected = detected(w, model.CloudProviderGCP)
	v.Errors = errors(w.GCP.DiscoveryErrors)
	v.Instances = instances(w, func(i *model.Instance) (model.LabelLastValue, model.LabelLastValue, model.LabelLastValue, bool) {
		switch {
		case i.CloudSQL != nil:
			return i.CloudSQL.Status, i.CloudSQL.Engine, i.CloudSQL.EngineVersion, true
		case i.Memorystore != nil:
			return i.Memorystore.Status, i.Memorystore.Engine, i.Memorystore.EngineVersion, true
		}
		return model.LabelLastValue{}, model.LabelLastValue{}, model.LabelLastValue{}, false
	})
	return v
}

func detected(w *model.World, provider string) bool {
	for _, n := range w.Nodes {
		if strings.EqualFold(n.CloudProvider.Value(), provider) {
			return true
		}
	}
	return false
}

func OCI(w *model.World) *View {
	v := &View{}
	if w == nil {
		return v
	}
	v.Configured = w.OCI.Configured
	v.Detected = detected(w, model.CloudProviderOCI)
	v.Errors = errors(w.OCI.DiscoveryErrors)
	v.Instances = instances(w, func(i *model.Instance) (model.LabelLastValue, model.LabelLastValue, model.LabelLastValue, bool) {
		switch {
		case i.OCIDB != nil:
			return i.OCIDB.Status, i.OCIDB.Engine, i.OCIDB.EngineVersion, true
		case i.OCICache != nil:
			return i.OCICache.Status, i.OCICache.Engine, i.OCICache.EngineVersion, true
		}
		return model.LabelLastValue{}, model.LabelLastValue{}, model.LabelLastValue{}, false
	})
	return v
}

func instances(w *model.World, managed func(*model.Instance) (status, engine, version model.LabelLastValue, ok bool)) []Instance {
	var res []Instance
	for _, app := range w.Applications {
		for _, i := range app.Instances {
			status, engine, version, ok := managed(i)
			if !ok {
				continue
			}
			ii := Instance{ApplicationId: app.Id, Name: i.Name, Status: status.Value(), Engine: engine.Value(), EngineVersion: version.Value()}
			if i.Node != nil {
				ii.InstanceType = i.Node.InstanceType.Value()
				ii.AvailabilityZone = i.Node.AvailabilityZone.Value()
			}
			res = append(res, ii)
		}
	}
	return res
}

func errors(m map[string]bool) []string {
	res := maps.Keys(m)
	slices.Sort(res)
	return res
}
