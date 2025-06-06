package model

import (
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type ApplicationAnnotation string

const (
	ApplicationAnnotationCategory                 ApplicationAnnotation = "application_category"
	ApplicationAnnotationCustomName               ApplicationAnnotation = "custom_application_name"
	ApplicationAnnotationSLOAvailabilityObjective ApplicationAnnotation = "slo_availability_objective"
	ApplicationAnnotationSLOLatencyObjective      ApplicationAnnotation = "slo_latency_objective"
	ApplicationAnnotationSLOLatencyThreshold      ApplicationAnnotation = "slo_latency_threshold"
)

var ApplicationAnnotationsList = []ApplicationAnnotation{
	ApplicationAnnotationCategory,
	ApplicationAnnotationCustomName,
	ApplicationAnnotationSLOAvailabilityObjective,
	ApplicationAnnotationSLOLatencyObjective,
	ApplicationAnnotationSLOLatencyThreshold,
}

var ApplicationAnnotationLabels = map[string]ApplicationAnnotation{}

func init() {
	for _, aa := range ApplicationAnnotationsList {
		ApplicationAnnotationLabels["annotation_coroot_com_"+string(aa)] = aa
	}
}

type ApplicationAnnotations map[ApplicationAnnotation]*LabelLastValue

func (aas ApplicationAnnotations) UpdateFromLabels(ls Labels, ts *timeseries.TimeSeries) {
	for n, v := range ls {
		aa := ApplicationAnnotationLabels[n]
		if aa == "" || v == "" {
			continue
		}
		lv := aas[aa]
		if lv == nil {
			lv = &LabelLastValue{}
			aas[aa] = lv
		}
		lv.Update(ts, v)
	}
}

func (app *Application) GetAnnotation(aa ApplicationAnnotation) string {
	if lv := app.Annotations[aa]; lv != nil {
		return lv.Value()
	}
	var instanceAnnotations *utils.StringSet
	for _, i := range app.Instances {
		if i.IsObsolete() {
			continue
		}
		if lv := i.Annotations[aa]; lv != nil {
			if instanceAnnotations == nil {
				instanceAnnotations = utils.NewStringSet()
			}
			instanceAnnotations.Add(lv.Value())
		}
	}
	if instanceAnnotations == nil {
		return ""
	}
	return instanceAnnotations.GetFirst()
}
