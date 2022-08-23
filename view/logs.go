package view

import (
	"fmt"
	"github.com/coroot/coroot-focus/model"
	"github.com/coroot/coroot-focus/timeseries"
	"github.com/coroot/logpattern"
	"sort"
	"strings"
)

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

type LogPatterns struct {
	Title    string            `json:"title"`
	Patterns []*LogPatternInfo `json:"patterns"`
}

var (
	logLevelColors = map[model.LogLevel]string{
		"unknown":  "grey-lighten2",
		"debug":    "grey-lighten1",
		"info":     "blue-lighten4",
		"warning":  "orange-lighten1",
		"error":    "red-darken4",
		"critical": "black",
	}
	logLevels = []model.LogLevel{"unknown", "debug", "info", "warning", "error", "critical"}
)

func logs(app *model.Application) *Dashboard {
	byHash := map[string]*LogPatternInfo{}
	byLevel := map[model.LogLevel]timeseries.TimeSeries{}
	dash := &Dashboard{Name: "Logs"}

	patterns := &LogPatterns{
		Title: fmt.Sprintf("Repeated patters from the <var>%s</var>'s log", app.Id.Name),
	}
	totalEvents := uint64(0)

	for _, instance := range app.Instances {
		for level, samples := range instance.LogMessagesByLevel {
			data, ok := byLevel[level]
			if !ok {
				data = timeseries.Aggregate(timeseries.NanSum)
				byLevel[level] = data
			}
			data.(*timeseries.AggregatedTimeseries).AddInput(samples)
		}
		for hash, p := range instance.LogPatterns {
			switch p.Level {
			case model.LogLevelWarning, model.LogLevelError, model.LogLevelCritical:
			default:
				continue
			}
			events := uint64(timeseries.Reduce(timeseries.NanSum, p.Sum))
			if events == 0 {
				continue
			}
			pattern := byHash[hash]
			if pattern == nil {
				for _, pp := range byHash {
					if pp.Pattern.WeakEqual(p.Pattern) {
						pattern = pp
						break
					}
				}
				if pattern == nil {
					pattern = &LogPatternInfo{
						Level:     string(p.Level),
						Sample:    p.Sample,
						Multiline: p.Multiline,
						Pattern:   p.Pattern,
						Sum:       timeseries.Aggregate(timeseries.NanSum),
						Color:     logLevelColors[p.Level],
						Instances: NewChart("Events by instance").Column(),
					}
					byHash[hash] = pattern
					patterns.Patterns = append(patterns.Patterns, pattern)
				}
			}
			totalEvents += events
			pattern.Events += events
			pattern.Instances.AddSeries(instance.Name, p.Sum)
			pattern.Sum.(*timeseries.AggregatedTimeseries).AddInput(p.Sum)
		}
	}

	for _, l := range logLevels {
		dash.GetOrCreateChart("Events by severity").Column().
			AddSeries(strings.ToUpper(string(l)), byLevel[l], logLevelColors[l])
	}
	sort.Slice(patterns.Patterns, func(i, j int) bool {
		return patterns.Patterns[i].Events > patterns.Patterns[j].Events
	})
	for _, p := range patterns.Patterns {
		p.Percentage = p.Events * 100 / totalEvents
	}
	dash.Widgets = append(dash.Widgets, &Widget{LogPatterns: patterns})
	return dash
}
