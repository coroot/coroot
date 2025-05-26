package prom

import (
	"github.com/prometheus/prometheus/promql/parser"
)

func IsSelectorValid(selector string) bool {
	if selector == "" {
		return true
	}
	_, err := parser.ParseMetricSelector(selector)
	return err == nil
}

type FilterLabelsF func(name string) bool

func FilterLabelsKeepAll(name string) bool {
	return true
}

func FilterLabelsDropAll(name string) bool {
	return false
}
