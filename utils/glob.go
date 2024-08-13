package utils

import (
	"path/filepath"
)

func GlobValidate(patterns []string) bool {
	for _, p := range patterns {
		if _, err := filepath.Match(p, ""); err != nil {
			return false
		}
	}
	return true
}

func GlobMatch(s string, patterns ...string) bool {
	for _, p := range patterns {
		if ok, err := filepath.Match(p, s); err == nil && ok {
			return true
		}
	}
	return false
}
