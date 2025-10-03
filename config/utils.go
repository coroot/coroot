package config

import "github.com/prometheus/prometheus/promql/parser"

func IsPrometheusSelectorValid(selector string) bool {
	if selector == "" {
		return true
	}
	_, err := parser.ParseMetricSelector(selector)
	return err == nil
}
