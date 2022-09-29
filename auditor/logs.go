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
	report := model.NewAuditReport(a.w.Ctx, "Logs")

	patterns := &model.LogPatterns{
		Title: fmt.Sprintf("Repeated patters from the <var>%s</var>'s log", a.app.Id.Name),
	}
	totalEvents := uint64(0)

	for _, instance := range a.app.Instances {
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
					pattern = &model.LogPatternInfo{
						Level:     string(p.Level),
						Sample:    p.Sample,
						Multiline: p.Multiline,
						Pattern:   p.Pattern,
						Sum:       timeseries.Aggregate(timeseries.NanSum),
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
			pattern.Sum.(*timeseries.AggregatedTimeseries).AddInput(p.Sum)
		}
	}

	eventsBySeverity := model.NewChart(a.w.Ctx, "Events by severity").Column()
	for _, l := range logLevels {
		if l == model.LogLevelError || l == model.LogLevelCritical {
			if v := timeseries.Reduce(timeseries.NanSum, byLevel[l]); v > 0 {
				report.GetOrCreateCheck(model.Checks.Logs.Errors).Inc(v)
			}
		}
		eventsBySeverity.AddSeries(strings.ToUpper(string(l)), byLevel[l], logLevelColors[l])
	}
	report.
		GetOrCreateCheck(model.Checks.Logs.Errors).
		Format(
			`{{.Value}} errors occurred`,
			a.getSimpleConfig(model.Checks.Logs.Errors, 0).Threshold,
		)
	sort.Slice(patterns.Patterns, func(i, j int) bool {
		return patterns.Patterns[i].Events > patterns.Patterns[j].Events
	})
	for _, p := range patterns.Patterns {
		p.Percentage = p.Events * 100 / totalEvents
	}
	report.Widgets = append(report.Widgets, &model.Widget{Chart: eventsBySeverity, Width: "100%"})
	report.Widgets = append(report.Widgets, &model.Widget{LogPatterns: patterns, Width: "100%"})
	a.addReport(report)
}
