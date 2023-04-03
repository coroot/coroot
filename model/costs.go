package model

type Costs struct {
	CPUUsagePerHour      float32
	CPURequestPerHour    float32
	MemoryUsagePerHour   float32
	MemoryRequestPerHour float32
}

func (c *Costs) CPUUsagePerMonth() float32 {
	return c.CPUUsagePerHour * 24 * 30
}

func (c *Costs) MemoryUsagePerMonth() float32 {
	return c.MemoryUsagePerHour * 24 * 30
}

func (c *Costs) CPURequestPerMonth() float32 {
	return c.CPURequestPerHour * 24 * 30
}

func (c *Costs) MemoryRequestPerMonth() float32 {
	return c.MemoryRequestPerHour * 24 * 30
}

func (c *Costs) UsagePerMonth() float32 {
	return (c.MemoryUsagePerHour + c.CPUUsagePerHour) * 24 * 30
}

func (c *Costs) RequestPerMonth() float32 {
	return (c.MemoryRequestPerHour + c.CPURequestPerHour) * 24 * 30
}
