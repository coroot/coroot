package config

import "github.com/prometheus/prometheus/promql/parser"

var promqlParser = parser.NewParser(parser.Options{})

func IsPrometheusSelectorValid(selector string) bool {
	if selector == "" {
		return true
	}
	_, err := promqlParser.ParseMetricSelector(selector)
	return err == nil
}
