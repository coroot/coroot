package configs

import (
	"github.com/coroot/coroot/model"
	"k8s.io/klog"
)

type View struct {
	configs model.CheckConfigs

	Checks []Check `json:"checks"`
}

type Check struct {
	model.Check
	GlobalThreshold      float64       `json:"global_threshold"`
	ProjectThreshold     *float64      `json:"project_threshold"`
	ApplicationOverrides []Application `json:"application_overrides"`
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

func (v *View) addReport(kind string, checks ...model.CheckConfig) {
	for _, c := range checks {
		ch := Check{
			Check: model.Check{
				Id:                      c.Id,
				Title:                   kind + " / " + c.Title,
				Unit:                    c.Unit,
				ConditionFormatTemplate: c.ConditionFormatTemplate,
			},
			GlobalThreshold: c.DefaultThreshold,
		}
		for appId, configs := range v.configs.GetByCheck(c.Id) {
			for _, unk := range configs {
				switch cfg := unk.(type) {
				case model.CheckConfigSimple:
					if appId.IsZero() {
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
		v.Checks = append(v.Checks, ch)
	}
}
