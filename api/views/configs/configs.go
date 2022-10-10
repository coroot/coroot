package configs

import (
	"github.com/coroot/coroot/model"
	"k8s.io/klog"
)

type View struct {
	configs model.CheckConfigs

	Reports []Report `json:"reports"`
}

type Report struct {
	Name   string  `json:"name"`
	Checks []Check `json:"checks"`
}

type Check struct {
	Name                 string          `json:"name"`
	Unit                 model.CheckUnit `json:"unit"`
	Condition            string          `json:"condition"`
	GlobalThreshold      float64         `json:"global_threshold"`
	ProjectThreshold     *float64        `json:"project_threshold"`
	ApplicationOverrides []Application   `json:"application_overrides"`
}

type Application struct {
	Id        model.ApplicationId `json:"id"`
	Threshold float64             `json:"threshold"`
	Details   string              `json:"details"`
}

func Render(configs model.CheckConfigs) *View {
	v := &View{configs: configs}
	cs := model.Checks

	v.addReport("SLO", cs.SLOAvailability, cs.SLOLatency)
	v.addReport("Instances", cs.InstanceAvailability, cs.InstanceRestarts)
	v.addReport("CPU", cs.CPUNode, cs.CPUContainer)
	v.addReport("Memory", cs.MemoryOOM)
	v.addReport("Storage", cs.StorageIO, cs.StorageSpace)
	v.addReport("Network", cs.NetworkRTT)
	v.addReport("Logs", cs.LogErrors)
	v.addReport("Postgres", cs.PostgresAvailability, cs.PostgresLatency, cs.PostgresErrors)
	v.addReport("Redis", cs.RedisAvailability, cs.RedisLatency)

	return v
}

func (v *View) addReport(name string, checks ...model.CheckConfig) {
	r := Report{Name: name}
	for _, c := range checks {
		ch := Check{
			Name:            c.Title,
			Unit:            c.Unit,
			Condition:       c.ConditionFormatTemplate,
			GlobalThreshold: c.DefaultThreshold,
		}
		emptyAppId := model.ApplicationId{}
		for appId, configs := range v.configs.GetByCheck(c.Id) {
			for _, unk := range configs {
				switch cfg := unk.(type) {
				case model.CheckConfigSimple:
					if appId == emptyAppId {
						t := cfg.Threshold
						ch.ProjectThreshold = &t
					} else {
						ch.ApplicationOverrides = append(ch.ApplicationOverrides, Application{
							Id:        appId,
							Threshold: cfg.Threshold,
						})
					}
				case []model.CheckConfigSLOAvailability:
					for _, c := range cfg {
						ch.ApplicationOverrides = append(ch.ApplicationOverrides, Application{
							Id:        appId,
							Threshold: c.ObjectivePercentage,
						})
					}
				case []model.CheckConfigSLOLatency:
					for _, c := range cfg {
						ch.ApplicationOverrides = append(ch.ApplicationOverrides, Application{
							Id:        appId,
							Threshold: c.ObjectivePercentage,
							Details:   "< " + model.FormatLatencyBucket(c.ObjectiveBucket),
						})
					}
				default:
					klog.Warningln("unknown config type")
				}
			}
		}
		r.Checks = append(r.Checks, ch)
	}
	v.Reports = append(v.Reports, r)
}
