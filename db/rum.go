package db

import (
	"fmt"
	"strings"

	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
)

// RumProjectSettings holds project-wide RUM privacy / enrichment options.
type RumProjectSettings struct {
	GeoEnabled       bool             `json:"geo_enabled,omitempty" yaml:"geo_enabled,omitempty"`
	ReplayEnabled    bool             `json:"replay_enabled,omitempty" yaml:"replay_enabled,omitempty"`
	ReplaySampleRate float64          `json:"replay_sample_rate,omitempty" yaml:"replay_sample_rate,omitempty"`
	HostMappings     []RumHostMapping `json:"host_mappings,omitempty" yaml:"host_mappings,omitempty"`
	Retention        *RumRetention    `json:"retention,omitempty" yaml:"retention,omitempty"`
}

// RumRetention overrides global RUM TTL defaults for a project. Zero values mean "use global default".
type RumRetention struct {
	RawTTL        timeseries.Duration `json:"raw_ttl,omitempty" yaml:"raw_ttl,omitempty"`
	ReplayTTL     timeseries.Duration `json:"replay_ttl,omitempty" yaml:"replay_ttl,omitempty"`
	AggregatesTTL timeseries.Duration `json:"aggregates_ttl,omitempty" yaml:"aggregates_ttl,omitempty"`
}

func (r *RumRetention) Validate() error {
	if r == nil {
		return nil
	}
	min := timeseries.Day
	if r.RawTTL > 0 && r.RawTTL < min {
		return fmt.Errorf("raw_ttl must be at least 1d")
	}
	if r.ReplayTTL > 0 && r.ReplayTTL < min {
		return fmt.Errorf("replay_ttl must be at least 1d")
	}
	if r.AggregatesTTL > 0 && r.AggregatesTTL < min {
		return fmt.Errorf("aggregates_ttl must be at least 1d")
	}
	raw := r.RawTTL
	agg := r.AggregatesTTL
	if raw > 0 && agg > 0 && agg < raw {
		return fmt.Errorf("aggregates_ttl must be >= raw_ttl")
	}
	return nil
}

// RumHostMapping maps a browser host pattern to a backend application id string.
type RumHostMapping struct {
	Pattern string `json:"pattern" yaml:"pattern"`
	AppId   string `json:"app_id" yaml:"app_id"`
}

const (
	ApiKeyTypeDefault = ""
	ApiKeyTypeRum     = "rum"
)

// RumApiKey returns a RUM API key as three alphanumeric blocks of length 5,
// e.g. "a7k2m-9xq4p-b3n8w".
func RumApiKey() string {
	const block = 5
	parts := make([]string, 3)
	for i := range parts {
		parts[i] = utils.NanoId(block)
	}
	return strings.Join(parts, "-")
}
