package overview

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceBelongsToNamespaces(t *testing.T) {
	ns := map[string]bool{"twenty-production": true}
	apps := map[string]bool{"twenty-kuberoapp-web": true}

	assert.True(t, serviceBelongsToNamespaces("/k8s/twenty-production/twenty-kuberoapp-web", ns, apps))
	assert.True(t, serviceBelongsToNamespaces("/k8s/twenty-production/twenty-kuberoapp-web-abc12-xyz99/web", ns, apps))
	assert.True(t, serviceBelongsToNamespaces("twenty-kuberoapp-web", ns, apps))
	assert.False(t, serviceBelongsToNamespaces("/k8s/test-production/chi-clickhouse", ns, apps))
	assert.False(t, serviceBelongsToNamespaces("/k8s/kube-system/coredns", ns, apps))
	assert.False(t, serviceBelongsToNamespaces("", ns, apps))
}
