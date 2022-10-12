package auditor

import (
	"fmt"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"sort"
	"strings"
)

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

func (a *appAuditor) logs() {
	byHash := map[string]*model.LogPatternInfo{}
	byLevel := map[model.LogLevel]timeseries.TimeSeries{}
	report := a.addReport("Logs")
	check := report.CreateCheck(model.Checks.LogErrors)
	seenContainers := false
	patterns := &model.LogPatterns{
		Title: fmt.Sprintf("Repeated patters from the <var>%s</var>'s log", a.app.Id.Name),
	}
	totalEvents := uint64(0)

	for _, instance := range a.app.Instances {
		if len(instance.Containers) > 0 {
			seenContainers = true
		}
		for level, samples := range instance.LogMessagesByLevel {
			byLevel[level] = timeseries.Merge(byLevel[level], samples, timeseries.NanSum)
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
					pattern = &model.LogPatternInfo{
						Level:     string(p.Level),
						Sample:    p.Sample,
						Multiline: p.Multiline,
						Pattern:   p.Pattern,
						Color:     logLevelColors[p.Level],
						Instances: model.NewChart(a.w.Ctx, "Events by instance").Column(),
					}
					byHash[hash] = pattern
					patterns.Patterns = append(patterns.Patterns, pattern)
				}
			}
			totalEvents += events
			pattern.Events += events
			pattern.Instances.AddSeries(instance.Name, p.Sum)
			pattern.Sum = timeseries.Merge(pattern.Sum, p.Sum, timeseries.NanSum)
		}
	}

	eventsBySeverity := model.NewChart(a.w.Ctx, "Events by severity").Column()
	for _, l := range logLevels {
		if l == model.LogLevelError || l == model.LogLevelCritical {
			if v := timeseries.Reduce(timeseries.NanSum, byLevel[l]); v > 0 {
				check.Inc(int64(v))
			}
		}
		eventsBySeverity.AddSeries(strings.ToUpper(string(l)), byLevel[l], logLevelColors[l])
	}
	sort.Slice(patterns.Patterns, func(i, j int) bool {
		return patterns.Patterns[i].Events > patterns.Patterns[j].Events
	})
	for _, p := range patterns.Patterns {
		p.Percentage = p.Events * 100 / totalEvents
	}
	report.Widgets = append(report.Widgets, &model.Widget{Chart: eventsBySeverity, Width: "100%"})
	report.Widgets = append(report.Widgets, &model.Widget{LogPatterns: patterns, Width: "100%"})

	if !seenContainers {
		check.SetStatus(model.UNKNOWN, "no data")
	}
}
