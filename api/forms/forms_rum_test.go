package forms

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProjectRumSettingsFormValid(t *testing.T) {
	assert.True(t, (&ProjectRumSettingsForm{ReplaySampleRate: 0.5}).Valid())
	assert.False(t, (&ProjectRumSettingsForm{ReplaySampleRate: 1.5}).Valid())
	assert.False(t, (&ProjectRumSettingsForm{RawTTL: "not-a-duration"}).Valid())
	assert.False(t, (&ProjectRumSettingsForm{RawTTL: "12h", AggregatesTTL: "1d"}).Valid())
	assert.True(t, (&ProjectRumSettingsForm{RawTTL: "7d", AggregatesTTL: "30d"}).Valid())
}
