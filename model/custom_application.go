package model

import "strings"

type CustomApplication struct {
	InstancePatterns []string `json:"instance_patterns" yaml:"instancePatterns"`
}

func ParseCustomApplicationName(customAppName string) (namespace, name string) {
	if parts := strings.SplitN(customAppName, "/", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", customAppName
}
