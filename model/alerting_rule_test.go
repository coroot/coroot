package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuiltinAlertingRules(t *testing.T) {
	rules := BuiltinAlertingRules()
	assert.NotEmpty(t, rules)

	for _, rule := range rules {
		t.Run(string(rule.Id), func(t *testing.T) {
			assert.NotEmpty(t, rule.Id, "Id is required")
			assert.NotEmpty(t, rule.Name, "Name is required")
			assert.NotEmpty(t, rule.Source.Type, "Source.Type is required")
			if rule.Source.Type == AlertSourceTypeCheck {
				assert.NotNil(t, rule.Source.Check, "Source.Check is required for check type")
				assert.NotEmpty(t, rule.Source.Check.CheckId, "Source.Check.CheckId is required")
			}
			assert.NotEmpty(t, rule.Selector.Type, "Selector.Type is required")
			assert.True(t, rule.Severity == WARNING || rule.Severity == CRITICAL, "Severity must be WARNING or CRITICAL, got: %s", rule.Severity)
			assert.True(t, rule.Enabled, "Builtin rules should be enabled by default")
			assert.True(t, rule.Builtin, "Builtin flag should be true")
		})
	}
}
