package auditor

import (
	"fmt"
	"github.com/coroot/coroot/model"
)

func (a *appAuditor) costs() {
	seenPricing := false
	for _, n := range a.w.Nodes {
		if n.PricePerHour > 0 {
			seenPricing = true
		}
	}
	if !seenPricing {
		return
	}

	report := a.addReport(model.AuditReportCosts)
	costs := model.Costs{}

	for _, i := range a.app.Instances {
		instanceCosts := i.Costs()
		if instanceCosts == nil {
			continue
		}
		costs.CPUUsagePerHour += instanceCosts.CPUUsagePerHour
		costs.CPURequestPerHour += instanceCosts.CPURequestPerHour
		costs.MemoryUsagePerHour += instanceCosts.MemoryUsagePerHour
		costs.MemoryRequestPerHour += instanceCosts.MemoryRequestPerHour

	}
	report.GetOrCreateTable("Application", "Usage", "Request", "CPU usage", "CPU request", "Memory usage", "Memory request").AddRow(
		model.NewTableCell(a.app.Id.Name),
		model.NewTableCell(fmt.Sprintf("$%.2f", costs.UsagePerMonth())).SetUnit("/mo"),
		model.NewTableCell(fmt.Sprintf("$%.2f", costs.RequestPerMonth())).SetUnit("/mo"),
		model.NewTableCell(fmt.Sprintf("$%.2f", costs.CPUUsagePerMonth())).SetUnit("/mo"),
		model.NewTableCell(fmt.Sprintf("$%.2f", costs.CPURequestPerMonth())).SetUnit("/mo"),
		model.NewTableCell(fmt.Sprintf("$%.2f", costs.MemoryUsagePerMonth())).SetUnit("/mo"),
		model.NewTableCell(fmt.Sprintf("$%.2f", costs.MemoryRequestPerMonth())).SetUnit("/mo"),
	)
}
