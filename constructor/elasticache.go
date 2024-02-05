package constructor

import (
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func loadElasticacheMetadata(w *model.World, metrics map[string][]model.MetricValues, pjs promJobStatuses, ecInstancesById map[string]*model.Instance) {
	for _, m := range metrics["aws_elasticache_info"] {
		ecId := m.Labels["ec_instance_id"]
		if ecId == "" {
			continue
		}
		instance := ecInstancesById[ecId]
		if instance == nil {
			var id model.ApplicationId
			instanceParts := strings.SplitN(ecId, "/", 3)
			if len(instanceParts) != 3 {
				continue
			}
			id = model.NewApplicationId("", model.ApplicationKindElasticacheCluster, m.Labels["cluster_id"])
			instance = w.GetOrCreateApplication(id).GetOrCreateInstance(instanceParts[1], nil)
			ecInstancesById[ecId] = instance
			instance.Elasticache = &model.Elasticache{}
		}
		if instance.Node == nil {
			instance.Node = model.NewNode("elasticache:" + instance.Name)
			instance.Node.Name.Update(m.Values, "elasticache:"+instance.Name)
			instance.Node.Instances = append(instance.Node.Instances, instance)
			w.Nodes = append(w.Nodes, instance.Node)
		}
		instance.TcpListens[model.Listen{IP: m.Labels["ipv4"], Port: m.Labels["port"]}] = true
		instance.Elasticache.Engine.Update(m.Values, m.Labels["engine"])
		instance.Elasticache.EngineVersion.Update(m.Values, m.Labels["engine_version"])
		instance.Node.InstanceType.Update(m.Values, m.Labels["instance_type"])
		instance.Node.CloudProvider.Update(m.Values, model.CloudProviderAWS)
		instance.Node.Region.Update(m.Values, m.Labels["region"])
		instance.Node.AvailabilityZone.Update(m.Values, m.Labels["availability_zone"])
	}
}

func (c *Constructor) loadElasticache(w *model.World, metrics map[string][]model.MetricValues, pjs promJobStatuses, ecInstancesById map[string]*model.Instance) {
	for queryName := range QUERIES {
		if !strings.HasPrefix(queryName, "aws_elasticache_") || queryName == "aws_elasticache_info" {
			continue
		}
		for _, m := range metrics[queryName] {
			ecId := m.Labels["ec_instance_id"]
			if ecId == "" {
				continue
			}
			instance := ecInstancesById[ecId]
			if instance == nil {
				continue
			}
			switch queryName {
			case "aws_elasticache_status":
				instance.Elasticache.LifeSpan = merge(instance.Elasticache.LifeSpan, m.Values, timeseries.Any)
				instance.Elasticache.Status.Update(m.Values, m.Labels["status"])
			}
		}
	}
	if c.pricing != nil {
		for _, instance := range ecInstancesById {
			instance.Node.Price = c.pricing.GetNodePrice(instance.Node)
		}
	}
}
