package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatImage(t *testing.T) {
	assert.Equal(t, "catalog:0.33", FormatImage("docker.io/organization/catalog:0.33"))
	assert.Equal(t, "img:95048a46", FormatImage("docker.io/orgg/img:95048a46"))
	assert.Equal(t, "package-image@sha256:2d01d1a", FormatImage("repo.io/org/package-image@sha256:2d01d1af064c8cdb32f51406f4148091cd0c87168c41725a62110aae9a6a44b4"))
}
