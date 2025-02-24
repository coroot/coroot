package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/coroot/coroot/timeseries"
	"github.com/prometheus/common/model"
)

type Labels map[string]string

func (ls Labels) Hash() uint64 {
	return model.LabelsToSignature(ls)
}

func (ls Labels) String() string {
	if len(ls) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ls))
	for k, v := range ls {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(parts)
	return fmt.Sprintf("{%s}", strings.Join(parts, ","))
}

type NodeContainerId struct {
	NodeId
	ContainerId string
}

type MetricValues struct {
	Labels     Labels
	LabelsHash uint64
	NodeContainerId
	ConnectionKey
	DestIp bool
	Values []*timeseries.TimeSeries
}

func (mv *MetricValues) Get(group, metric string) *timeseries.TimeSeries {
	for i, m := range Metrics[group] {
		if m.Name == metric && i < len(mv.Values) {
			return mv.Values[i]
		}
	}
	return nil
}
