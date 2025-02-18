package constructor

import (
	"strings"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/coroot/logparser"
	"k8s.io/klog"
)

func logMessage(instance *model.Instance, metric *model.MetricValues, pjs promJobStatuses) {
	level := model.LogLevel(metric.Labels["level"])
	msgs := instance.Owner.LogMessages[level]
	if msgs == nil {
		msgs = &model.LogMessages{}
		instance.Owner.LogMessages[level] = msgs
	}
	values := timeseries.Increase(metric.Values, pjs.get(metric.Labels))
	msgs.Messages = merge(msgs.Messages, values, timeseries.NanSum)

	if hash := metric.Labels["pattern_hash"]; hash != "" {
		if msgs.Patterns == nil {
			msgs.Patterns = map[string]*model.LogPattern{}
		}
		p := msgs.Patterns[hash]
		if p == nil {
			for _, pp := range msgs.Patterns {
				if pp.SimilarPatternHashes.Has(hash) {
					p = pp
					break
				}
			}
			if p == nil {
				sample := metric.Labels["sample"]
				pattern := logparser.NewPattern(sample)
				for _, pp := range msgs.Patterns {
					if pattern.WeakEqual(pp.Pattern) {
						p = pp
						p.SimilarPatternHashes.Add(hash)
						break
					}
				}
				if p == nil {
					p = &model.LogPattern{
						Sample:               sample,
						Multiline:            strings.Contains(sample, "\n"),
						Pattern:              pattern,
						SimilarPatternHashes: utils.NewStringSet(hash),
					}
					msgs.Patterns[hash] = p
				}
			}
		}
		p.Messages = merge(p.Messages, values, timeseries.NanSum)
	}
}

func (c *Constructor) loadContainerLogs(metrics map[string][]*model.MetricValues, containers containerCache, pjs promJobStatuses) {
	for _, metric := range metrics["container_log_messages"] {
		v := containers[metric.NodeContainerId]
		if v.instance == nil {
			continue
		}
		logMessage(v.instance, metric, pjs)
	}
}

func (c *Constructor) loadApplicationLogs(w *model.World, metrics map[string][]*model.MetricValues) {
	for _, metric := range metrics[qRecordingRuleApplicationLogMessages] {
		appId, err := model.NewApplicationIdFromString(metric.Labels["application"])
		if err != nil {
			klog.Error(err)
			continue
		}
		app := w.GetApplication(appId)
		if app == nil {
			continue
		}
		if app.LogMessages == nil {
			app.LogMessages = map[model.LogLevel]*model.LogMessages{}
		}
		level := model.LogLevel(metric.Labels["level"])
		msgs := app.LogMessages[level]
		if msgs == nil {
			msgs = &model.LogMessages{}
			app.LogMessages[level] = msgs
		}
		msgs.Messages = merge(msgs.Messages, metric.Values, timeseries.NanSum)
		similar := metric.Labels["similar"]
		if similar == "" {
			continue
		}

		if msgs.Patterns == nil {
			msgs.Patterns = map[string]*model.LogPattern{}
		}
		similarPatterns := strings.Split(similar, " ")
		var p *model.LogPattern
		for _, pp := range msgs.Patterns {
			for _, h := range similarPatterns {
				if pp.SimilarPatternHashes.Has(h) {
					p = pp
					break
				}
			}
			if p != nil {
				break
			}
		}
		if p == nil {
			pattern := logparser.NewPatternFromWords(metric.Labels["words"])
			for _, pp := range msgs.Patterns {
				if pattern.WeakEqual(pp.Pattern) {
					p = pp
					break
				}
			}
			if p == nil {
				p = &model.LogPattern{
					Pattern:              pattern,
					Sample:               metric.Labels["sample"],
					Multiline:            metric.Labels["multiline"] == "true",
					SimilarPatternHashes: utils.NewStringSet(),
				}
				msgs.Patterns[similarPatterns[0]] = p
			}
		}
		p.Messages = merge(p.Messages, metric.Values, timeseries.NanSum)
		p.SimilarPatternHashes.Add(similarPatterns...)
	}
}
