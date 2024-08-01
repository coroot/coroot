package overview

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type resourceType int

const (
	resourceCpu resourceType = iota + 1
	resourceMemory
)

func (rt resourceType) format(v float32) string {
	if v == 0 {
		return ""
	}
	switch rt {
	case resourceCpu:
		return utils.FormatFloat(v*1000) + "m"
	case resourceMemory:
		s, u := utils.FormatBytes(v)
		return s + u
	default:
		panic("unknown resource type")
	}
}

func (rt resourceType) suggestRequest(usage float32) float32 {
	var step, minUnit float32
	switch rt {
	case resourceCpu:
		minUnit = 0.001
	case resourceMemory:
		minUnit = 1000000
	default:
		panic("unknown resource type")
	}
	usage *= 1.1 // + 10%
	usage /= minUnit
	switch {
	case usage < 10:
		return 10 * minUnit
	case usage < 100:
		step = 10
	default:
		step = 100
	}
	truncated := float32(int64((usage+step)/step)) * step
	return truncated * minUnit
}

type resource struct {
	usage   *timeseries.TimeSeries
	request *timeseries.TimeSeries
}

type instance struct {
	ownerId             model.ApplicationId
	name                string
	cpu                 resource
	memory              resource
	crossAzTrafficCosts float32
	internetEgressCosts float32
	nodePrice           *model.NodePrice
}

func (i *instance) getResource(rt resourceType) resource {
	switch rt {
	case resourceCpu:
		return i.cpu
	case resourceMemory:
		return i.memory
	default:
		panic("unknown resource type")
	}
}
