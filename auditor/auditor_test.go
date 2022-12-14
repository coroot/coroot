package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestCalcRollouts(t *testing.T) {
	var app *model.Application
	createApp := func() {
		app = model.NewApplication(model.NewApplicationId("default", model.ApplicationKindDeployment, "catalog"))
	}
	addInstance := func(name string, rs string, lifeSpan ...float64) {
		i := app.GetOrCreateInstance(name)
		i.Pod = &model.Pod{ReplicaSet: rs}
		i.Pod.LifeSpan = timeseries.NewWithData(1, 1, lifeSpan)
	}
	checkEvents := func(expected string) {
		var actual []string
		for _, e := range calcRollouts(app) {
			assert.Equal(t, EventTypeRollout, e.Type)
			actual = append(actual, e.String())
		}
		assert.Equal(t, expected, strings.Join(actual, ";"))
	}

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 1)
	addInstance("i2", "rs1", 0, 0, 1, 1, 1, 1)
	checkEvents("")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 0, 0, 0)
	addInstance("i2", "rs2", 0, 0, 0, 1, 1, 1)
	checkEvents("4-4")

	createApp()
	addInstance("i1", "rs1", 2, 2, 1, 1, 0, 0)
	addInstance("i2", "rs2", 0, 0, 1, 1, 2, 2)
	checkEvents("3-5")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 0, 0)
	addInstance("i2", "rs2", 0, 0, 0, 0, 1, 1)
	checkEvents("5-5")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 0, 0)
	addInstance("i2", "rs2", 0, 1, 0, 0, 1, 1)
	checkEvents("2-5")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 1, 1)
	addInstance("i2", "rs2", 0, 0, 1, 1, 0, 0)
	checkEvents("3-3;5-5")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 0, 0)
	addInstance("i2", "rs2", 1, 1, 0, 0, 1, 1)
	checkEvents("1-5")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 0)
	addInstance("i2", "rs2", 1, 1, 1, 1, 1, 1)
	checkEvents("1-6")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 1)
	addInstance("i2", "rs2", 1, 1, 1, 1, 1, 1)
	checkEvents("1-")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 1)
	addInstance("i2", "rs2", 0, 0, 0, 1, 1, 1)
	checkEvents("4-")
}
