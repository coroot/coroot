package watchers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rca/llm"
)

const alertAITimeout = 20 * time.Second

const logPatternAISystem = `You are an SRE triaging a new log pattern before it pages on-call.

Decide if the team should be notified. Alert only for actionable production problems
(errors that indicate user impact, data loss, or a broken dependency). Do not alert on
health checks, expected retries, debug noise, or one-off messages with no operational value.

Respond with a single JSON object and nothing else:
{"should_alert": true, "explanation": "One or two sentences the on-call can read."}`

const kubernetesEventAISystem = `You are an SRE triaging a Kubernetes Warning event before it pages on-call.

Decide if the team should be notified. Alert only for events that likely need human action
(failed scheduling that will not recover, image pull errors, crash loops, eviction, probe
failures that persist). Do not alert on routine scaling, expected probe noise, or informational
events.

Respond with a single JSON object and nothing else:
{"should_alert": true, "explanation": "One or two sentences the on-call can read."}`

// LocalAlertAI triages log patterns and Kubernetes events with the same OpenAI-compatible
// LLM used for self-hosted RCA. A nil client means AI evaluation is off.
type LocalAlertAI struct {
	llm func() *llm.Client
}

func NewLocalAlertAI(llm func() *llm.Client) *LocalAlertAI {
	return &LocalAlertAI{llm: llm}
}

func (e *LocalAlertAI) Enabled() bool {
	return e != nil && e.llm != nil && e.llm() != nil
}

func (e *LocalAlertAI) LogPatterns() LogPatternEvaluator {
	return logPatternAI{LocalAlertAI: e}
}

func (e *LocalAlertAI) KubernetesEvents() KubernetesEventEvaluator {
	return kubernetesEventAI{LocalAlertAI: e}
}

type logPatternAI struct{ *LocalAlertAI }

func (e logPatternAI) Evaluate(project *db.Project, app *model.Application, severity model.Severity, lp *model.LogPattern) (*LogPatternEvaluation, error) {
	payload := map[string]any{
		"project":  projectName(project),
		"app":      app.Id.String(),
		"severity": severity.String(),
	}
	if lp != nil {
		if lp.Pattern != nil {
			payload["pattern"] = lp.Pattern.String()
		}
		payload["sample"] = truncateForAI(lp.Sample, 2000)
		payload["multiline"] = lp.Multiline
	}
	eval, err := e.evaluate(logPatternAISystem, payload)
	if err != nil {
		return nil, err
	}
	return &LogPatternEvaluation{ShouldAlert: eval.ShouldAlert, Explanation: eval.Explanation}, nil
}

type kubernetesEventAI struct{ *LocalAlertAI }

func (e kubernetesEventAI) Evaluate(project *db.Project, app *model.Application, event *model.LogEntry) (*KubernetesEventEvaluation, error) {
	payload := map[string]any{
		"project": projectName(project),
		"app":     app.Id.String(),
	}
	if event != nil {
		payload["severity"] = event.Severity.String()
		payload["body"] = truncateForAI(event.Body, 2000)
		payload["timestamp"] = event.Timestamp
		payload["attributes"] = event.AllAttributes()
	}
	eval, err := e.evaluate(kubernetesEventAISystem, payload)
	if err != nil {
		return nil, err
	}
	return &KubernetesEventEvaluation{ShouldAlert: eval.ShouldAlert, Explanation: eval.Explanation}, nil
}

func (e *LocalAlertAI) evaluate(system string, payload any) (*llm.Evaluation, error) {
	if !e.Enabled() {
		return nil, fmt.Errorf("no LLM configured")
	}
	user, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), alertAITimeout)
	defer cancel()
	return e.llm().Evaluate(ctx, system, string(user))
}

func projectName(project *db.Project) string {
	if project == nil {
		return ""
	}
	return project.Name
}

func truncateForAI(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
