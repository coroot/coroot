package config

import (
	"os"
	"regexp"
	"testing"

	"gopkg.in/alecthomas/kingpin.v2"
)

// Flags documented for Coroot Enterprise Edition only.
var eeOnlyFlags = map[string]bool{"license-key": true}

func TestDocumentedFlagsExist(t *testing.T) {
	data, err := os.ReadFile("../docs/docs/configuration/configuration.md")
	if err != nil {
		t.Fatal(err)
	}
	rows := regexp.MustCompile(`(?m)^\|\s*--([a-z][a-z0-9-]*)\s*\|`).FindAllStringSubmatch(string(data), -1)
	if len(rows) == 0 {
		t.Fatal("no flags found in the docs table")
	}
	for _, row := range rows {
		if name := row[1]; !eeOnlyFlags[name] && kingpin.CommandLine.GetFlag(name) == nil {
			t.Errorf("--%s is documented but not defined in config/flags.go", name)
		}
	}
}
