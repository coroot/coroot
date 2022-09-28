package model

import (
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/logpattern"
)

type LogPatterns struct {
	Title    string            `json:"title"`
	Patterns []*LogPatternInfo `json:"patterns"`
}

type LogPatternInfo struct {
	Pattern *logpattern.Pattern `json:"-"`

	Featured   bool                  `json:"featured"`
	Level      string                `json:"level"`
	Color      string                `json:"color"`
	Sample     string                `json:"sample"`
	Multiline  bool                  `json:"multiline"`
	Sum        timeseries.TimeSeries `json:"sum"`
	Percentage uint64                `json:"percentage"`
	Events     uint64                `json:"events"`
	Instances  *Chart                `json:"instances"`
}
