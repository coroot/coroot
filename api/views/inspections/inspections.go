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
	Category             string        `json:"category"`
	GlobalThreshold      float32       `json:"global_threshold"`
	ProjectThreshold     *float32      `json:"project_threshold"`
	ProjectDetails       string        `json:"project_details"`
	ApplicationOverrides []Application `json:"application_overrides"`
}

type Application struct {
	Id        model.ApplicationId `json:"id"`
	Threshold float32             `json:"threshold"`
	Details   string              `json:"details"`
}

func Render(configs model.CheckConfigs) *View {
	v := &View{configs: configs}

	for _, c := range model.GetCheckConfigs() {
		if c.Category == "" {
			continue
		}
		v.addCheck(*c)
	}

	return v
}

func (v *View) addCheck(c model.CheckConfig) {
	ch := Check{
		Check: model.Check{
			Id:                      c.Id,
			Title:                   c.Title,
			Unit:                    c.Unit,
			ConditionFormatTemplate: c.ConditionFormatTemplate,
		},
		Category:        string(c.Category),
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
					if appId.IsZero() {
						t := c.ObjectivePercentage
						ch.ProjectThreshold = &t
					} else {
						ch.ApplicationOverrides = append(ch.ApplicationOverrides, Application{
							Id:        appId,
							Threshold: c.ObjectivePercentage,
						})
					}
				}
			case []model.CheckConfigSLOLatency:
				for _, c := range cfg {
					details := "< " + utils.FormatLatency(c.ObjectiveBucket)
					if appId.IsZero() {
						t := c.ObjectivePercentage
						ch.ProjectThreshold = &t
						ch.ProjectDetails = details
					} else {
						ch.ApplicationOverrides = append(ch.ApplicationOverrides, Application{
							Id:        appId,
							Threshold: c.ObjectivePercentage,
							Details:   details,
						})
					}
				}
			default:
				klog.Warningln("unknown config type")
			}
		}
	}
	v.Checks = append(v.Checks, ch)
}
