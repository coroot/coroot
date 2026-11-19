package watchers

import (
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/stretchr/testify/assert"
)

func TestK8sEventAppResolver(t *testing.T) {
	w := model.NewWorld(0, 0, 0, 0)
	addApp := func(cluster, ns string, kind model.ApplicationKind, name string, pods ...string) *model.Application {
		app := w.GetOrCreateApplication(model.NewApplicationId(cluster, ns, kind, name), false)
		for _, p := range pods {
			app.GetOrCreateInstance(p, nil).Pod = &model.Pod{}
		}
		return app
	}

	devNats := addApp("c1", "dev", model.ApplicationKindStatefulSet, "nats", "nats-0")
	prodNats := addApp("c1", "prod", model.ApplicationKindStatefulSet, "nats", "nats-0")
	c2DevNats := addApp("c2", "dev", model.ApplicationKindStatefulSet, "nats", "nats-0")

	// the map is built from a randomly ordered map, so check the result is stable
	for i := 0; i < 20; i++ {
		resolve := newK8sEventAppResolver(w)
		assert.Same(t, devNats, resolve("c1", "dev", "Pod", "nats-0"))
		assert.Same(t, prodNats, resolve("c1", "prod", "Pod", "nats-0"))
		assert.Same(t, c2DevNats, resolve("c2", "dev", "Pod", "nats-0"))
		assert.Nil(t, resolve("c1", "staging", "Pod", "nats-0"))

		// events of the application object itself
		assert.Same(t, prodNats, resolve("c1", "prod", "StatefulSet", "nats"))
	}
}
