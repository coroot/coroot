package model

import (
	"fmt"

	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

type Impact struct {
	AffectedRequestPercentage float32 `json:"percentage"`
}

type IncidentDetails struct {
	AvailabilityBurnRates []BurnRate `json:"availability_burn_rates"`
	LatencyBurnRates      []BurnRate `json:"latency_burn_rates"`
	AvailabilityImpact    Impact     `json:"availability_impact"`
	LatencyImpact         Impact     `json:"latency_impact"`
}

type RCA struct {
	Status            string          `json:"status"`
	Error             string          `json:"error"`
	ShortSummary      string          `json:"short_summary"`
	RootCause         string          `json:"root_cause"`
	ImmediateFixes    string          `json:"immediate_fixes"`
	DetailedRootCause string          `json:"detailed_root_cause_analysis"`
	PropagationMap    *PropagationMap `json:"propagation_map"`
	Widgets           []*Widget       `json:"widgets"`
}

type PropagationMap struct {
	Applications []*PropagationMapApplication `json:"applications"`
}

type PropagationMapApplication struct {
	Id     ApplicationId `json:"id"`
	Icon   string        `json:"icon"`
	Labels Labels        `json:"labels"`
	Status Status        `json:"status"`
	Issues []string      `json:"issues,omitempty"`

	Upstreams   []*PropagationMapApplicationLink `json:"upstreams"`
	Downstreams []*PropagationMapApplicationLink `json:"downstreams"`
}

func (app *PropagationMapApplication) Issue(format string, a ...any) {
	issue := fmt.Sprintf(format, a...)
	for _, i := range app.Issues {
		if i == issue {
			return
		}
	}
	app.Issues = append(app.Issues, issue)
}

type PropagationMapApplicationLink struct {
	Id     ApplicationId    `json:"id"`
	Status Status           `json:"status"`
	Stats  *utils.StringSet `json:"stats"`
}

func (l *PropagationMapApplicationLink) AddIssues(issues ...string) {
	l.Status = CRITICAL
	l.Stats.Add(issues...)
}

type ApplicationIncident struct {
	ApplicationId ApplicationId   `json:"application_id"`
	Key           string          `json:"key"`
	OpenedAt      timeseries.Time `json:"opened_at"`
	ResolvedAt    timeseries.Time `json:"resolved_at"`
	Severity      Status          `json:"severity"`
	Details       IncidentDetails `json:"details"`
	RCA           *RCA            `json:"rca"`
}

func (i *ApplicationIncident) Resolved() bool {
	return !i.ResolvedAt.IsZero()
}

func (i *ApplicationIncident) ShortDescription() string {
	var (
		a, l bool
	)

	if i.RCA != nil && i.RCA.ShortSummary != "" {
		return i.RCA.ShortSummary
	}

	if i.Details.AvailabilityImpact.AffectedRequestPercentage > 0 {
		a = true
	}
	if i.Details.LatencyImpact.AffectedRequestPercentage > 0 {
		l = true
	}
	switch {
	case a && l:
		return "High latency and errors"
	case l:
		return "High latency"
	case a:
		return "Elevated error rate"
	}
	return "SLO violation"
}
