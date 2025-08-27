package model

import (
	"github.com/coroot/coroot/timeseries"
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
	Status            string    `json:"status"`
	Error             string    `json:"error"`
	ShortSummary      string    `json:"short_summary"`
	RootCause         string    `json:"root_cause"`
	ImmediateFixes    string    `json:"immediate_fixes"`
	DetailedRootCause string    `json:"detailed_root_cause_analysis"`
	Widgets           []*Widget `json:"widgets"`
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
