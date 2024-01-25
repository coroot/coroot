package watchers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

func TestCalcDeployments(t *testing.T) {
	var app *model.Application
	createApp := func() {
		app = model.NewApplication(model.NewApplicationId("default", model.ApplicationKindDeployment, "catalog"))
	}
	addInstance := func(name string, rs string, lifeSpan ...float32) {
		i := app.GetOrCreateInstance(name, nil)
		i.Pod = &model.Pod{ReplicaSet: rs}
		i.Pod.LifeSpan = timeseries.NewWithData(1, 1, lifeSpan)
	}
	checkDeployments := func(expected string) {
		var actual []string
		for _, d := range calcDeployments(app) {
			actual = append(actual, fmt.Sprintf("%d-%d:%s", d.StartedAt, d.FinishedAt, d.Name))
		}
		assert.Equal(t, expected, strings.Join(actual, ";"))
	}

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 1)
	addInstance("i2", "rs1", 0, 0, 1, 1, 1, 1)
	checkDeployments("")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 0, 0, 0)
	addInstance("i2", "rs2", 0, 0, 0, 1, 1, 1)
	checkDeployments("4-4:rs2")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 0, 0)
	addInstance("i2", "rs2", 0, 0, 1, 1, 1, 1)
	checkDeployments("3-5:rs2")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 0, 0)
	addInstance("i2", "rs2", 0, 0, 0, 0, 1, 1)
	checkDeployments("5-5:rs2")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 0, 0)
	addInstance("i2", "rs2", 0, 1, 0, 0, 1, 1)
	checkDeployments("2-5:rs2")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 1, 1)
	addInstance("i2", "rs2", 0, 0, 1, 1, 0, 0)
	checkDeployments("3-3:rs2;5-5:rs1")

	createApp()
	addInstance("i1", "rs1", 1, 0, 0, 0, 0, 0)
	addInstance("i2", "rs2", 1, 1, 1, 1, 1, 1)
	checkDeployments("")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 0, 0)
	addInstance("i2", "rs2", 1, 1, 0, 0, 1, 1)
	checkDeployments("")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 0)
	addInstance("i2", "rs2", 1, 1, 1, 1, 1, 1)
	checkDeployments("")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 1)
	addInstance("i2", "rs2", 1, 1, 1, 1, 1, 1)
	checkDeployments("")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 1, 1)
	addInstance("i2", "rs2", 1, 1, 1, 1, 0, 0)
	checkDeployments("5-5:rs1")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 1)
	addInstance("i2", "rs2", 0, 0, 0, 1, 1, 1)
	checkDeployments("4-0:rs2")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 1)
	addInstance("i2", "rs2", 0, 0, 1, 1, 0, 0)
	checkDeployments("3-0:rs2;5-5:rs1")
}
