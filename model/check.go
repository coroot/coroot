package model

import (
	"encoding/json"
	"fmt"
	"k8s.io/klog"
)

type CheckId string

const (
	CheckIdOOM          = "OOM"
	CheckIdNodeCPU      = "Node CPU utilization"
	CheckIdContainerCPU = "Container CPU utilization"
	CheckIdLogErrors    = "Log errors"
	CheckIdRedisStatus  = "Redis status"
	CheckIdRedisLatency = "Redis latency"
)

type Check struct {
	Id      CheckId `json:"id"`
	Status  Status  `json:"status"`
	Message string  `json:"message"`
}

func (ch *Check) SetStatus(status Status, format string, a ...any) {
	ch.Status = status
	ch.Message = fmt.Sprintf(format, a...)
}

type CheckConfigSimple struct {
	Threshold float64 `json:"threshold"`
}

type CheckConfigs map[ApplicationId]map[CheckId]json.RawMessage

func (cc CheckConfigs) getRaw(appId ApplicationId, checkId CheckId) json.RawMessage {
	for _, i := range []ApplicationId{appId, {}} {
		if appConfigs, ok := cc[i]; ok {
			if cfg, ok := appConfigs[checkId]; ok {
				return cfg
			}
		}
	}
	return nil
}

func (cc CheckConfigs) GetSimple(appId ApplicationId, checkId CheckId, defaultThreshold float64) CheckConfigSimple {
	cfg := CheckConfigSimple{Threshold: defaultThreshold}
	raw := cc.getRaw(appId, checkId)
	if raw == nil {
		return cfg
	}
	var v CheckConfigSimple
	if err := json.Unmarshal(raw, &v); err != nil {
		klog.Warningln("failed to unmarshal check config:", err)
		return cfg
	}
	return v
}

//type CheckConfig struct {
//	Threshold    float64 `json:"threshold"`
//	Availability *struct {
//		TotalRequestsQuery  string `json:"total_requests_query"`
//		FailedRequestsQuery string `json:"failed_requests_query"`
//	} `json:"availability;omitempty"`
//	Latency *struct {
//		LatencyHistogramQuery string `json:"latency_histogram_query"`
//		Bucket                string `json:"bucket"`
//	} `json:"latency;omitempty"`
//}
