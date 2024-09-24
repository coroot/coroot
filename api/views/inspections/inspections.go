package inspections

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

type View struct {
	configs model.CheckConfigs

	Checks []Check `json:"checks"`
}

type Check struct {
	model.Check
	GlobalThreshold      float32       `json:"global_threshold"`
	ProjectThreshold     *float32      `json:"project_threshold"`
	ApplicationOverrides []Application `json:"application_overrides"`
}

type Application struct {
	Id        model.ApplicationId `json:"id"`
	Threshold float32             `json:"threshold"`
	Details   string              `json:"details"`
}

func Render(configs model.CheckConfigs) *View {
	v := &View{configs: configs}
	cs := model.Checks

	v.addReport(model.AuditReportSLO, cs.SLOAvailability, cs.SLOLatency)
	v.addReport(model.AuditReportInstances, cs.InstanceAvailability, cs.InstanceRestarts)
	v.addReport(model.AuditReportDeployments, cs.DeploymentStatus)
	v.addReport(model.AuditReportCPU, cs.CPUNode, cs.CPUContainer)
	v.addReport(model.AuditReportMemory, cs.MemoryOOM, cs.MemoryLeakPercent)
	v.addReport(model.AuditReportStorage, cs.StorageIOLoad, cs.StorageSpace)
	v.addReport(model.AuditReportNetwork, cs.NetworkRTT)
	v.addReport(model.AuditReportLogs, cs.LogErrors)
	v.addReport(model.AuditReportPostgres, cs.PostgresAvailability, cs.PostgresLatency, cs.PostgresReplicationLag, cs.PostgresConnections)
	v.addReport(model.AuditReportRedis, cs.RedisAvailability, cs.RedisLatency)
	v.addReport(model.AuditReportJvm, cs.JvmAvailability, cs.JvmSafepointTime)
	v.addReport(model.AuditReportMongodb, cs.MongodbAvailability, cs.MongodbReplicationLag)

	return v
}

func (v *View) addReport(name model.AuditReportName, checks ...model.CheckConfig) {
	for _, c := range checks {
		ch := Check{
			Check: model.Check{
				Id:                      c.Id,
				Title:                   string(name) + " / " + c.Title,
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
							Details:   "< " + utils.FormatLatency(c.ObjectiveBucket),
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
