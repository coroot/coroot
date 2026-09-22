package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCustomApplicationName(t *testing.T) {
	testCases := []struct {
		input             string
		expectedNamespace string
		expectedName      string
	}{
		{
			input:             "",
			expectedNamespace: "",
			expectedName:      "",
		},
		{
			input:             "my-app",
			expectedNamespace: "",
			expectedName:      "my-app",
		},
		{
			input:             "default/my-app",
			expectedNamespace: "default",
			expectedName:      "my-app",
		},
		{
			input:             "prod/backend-api",
			expectedNamespace: "prod",
			expectedName:      "backend-api",
		},
		{
			input:             "custom-ns/team/app",
			expectedNamespace: "custom-ns",
			expectedName:      "team/app",
		},
	}

	for _, tc := range testCases {
		ns, name := ParseCustomApplicationName(tc.input)
		assert.Equal(t, tc.expectedNamespace, ns, "namespace for %s", tc.input)
		assert.Equal(t, tc.expectedName, name, "name for %s", tc.input)
	}
}
