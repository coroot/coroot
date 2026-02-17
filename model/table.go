package model

import (
	"fmt"
	"sort"

	"github.com/coroot/coroot/timeseries"
)

type Table struct {
	Header []string    `json:"header"`
	Rows   []*TableRow `json:"rows"`

	sorted bool
}

func NewTable(header ...string) *Table {
	return &Table{Header: header}
}

func (t *Table) AddRow(cells ...*TableCell) *TableRow {
	if t == nil {
		return nil
	}
	r := &TableRow{Cells: cells}
	t.Rows = append(t.Rows, r)
	t.SortRows()
	return r
}

func (t *Table) SortRows() {
	if t == nil {
		return
	}
	if t.sorted {
		return
	}
	sort.SliceStable(t.Rows, func(i, j int) bool {
		return t.Rows[i].Cells[0].Value < t.Rows[j].Cells[0].Value
	})
}

func (t *Table) SetSorted() *Table {
	if t == nil {
		return nil
	}
	t.sorted = true
	return t
}

type TableRow struct {
	Id    string       `json:"id"`
	Cells []*TableCell `json:"cells"`
}

func (r *TableRow) SetId(id string) *TableRow {
	if r == nil {
		return nil
	}
	r.Id = id
	return r
}

type Progress struct {
	Percent int    `json:"percent"`
	Color   string `json:"color"`
}

type Bandwidth struct {
	Rx string
	Tx string
}

type TableCell struct {
	Icon       *Icon                  `json:"icon"`
	Value      string                 `json:"value"`
	ShortValue string                 `json:"short_value"`
	Values     []string               `json:"values"`
	Tags       []string               `json:"tags"`
	Unit       string                 `json:"unit"`
	Status     *Status                `json:"status"`
	Link       *RouterLink            `json:"link"`
	Progress   *Progress              `json:"progress"`
	Bandwidth  *Bandwidth             `json:"bandwidth"`
	Chart      *timeseries.TimeSeries `json:"chart"`
	IsStub     bool                   `json:"is_stub"`
	MaxWidth   int                    `json:"max_width"`

	DeploymentSummaries []ApplicationDeploymentSummary `json:"deployment_summaries"`
}

func NewTableCell(values ...string) *TableCell {
	if len(values) == 0 {
		return &TableCell{}
	}
	if len(values) == 1 {
		return &TableCell{Value: values[0]}
	}
	return &TableCell{Values: values}
}

func (c *TableCell) SetStatus(status Status, msg string) *TableCell {
	if c == nil {
		return nil
	}
	c.Status = &status
	c.Value = msg
	return c
}

func (c *TableCell) UpdateStatus(status Status) *TableCell {
	if c == nil {
		return nil
	}
	c.Status = &status
	return c
}

func (c *TableCell) SetValue(value string) *TableCell {
	if c == nil {
		return nil
	}
	c.Value = value
	return c
}

func (c *TableCell) SetShortValue(value string) *TableCell {
	if c == nil {
		return nil
	}
	c.ShortValue = value
	return c
}

func (c *TableCell) SetIcon(name, color string) *TableCell {
	if c == nil {
		return nil
	}
	c.Icon = &Icon{Name: name, Color: color}
	return c
}

func (c *TableCell) SetUnit(unit string) *TableCell {
	if c == nil {
		return nil
	}
	c.Unit = unit
	return c
}

func (c *TableCell) AddTag(format string, a ...any) *TableCell {
	if c == nil {
		return nil
	}
	if format == "" {
		return c
	}
	if len(a) == 0 {
		c.Tags = append(c.Tags, format)
	} else {
		c.Tags = append(c.Tags, fmt.Sprintf(format, a...))
	}
	return c
}

func (c *TableCell) SetProgress(percent int, color string) *TableCell {
	if c == nil {
		return nil
	}
	c.Progress = &Progress{Percent: percent, Color: color}
	return c
}

func (c *TableCell) SetChart(ts *timeseries.TimeSeries) *TableCell {
	if c == nil {
		return nil
	}
	c.Chart = ts
	return c
}

func (c *TableCell) SetStub(format string, a ...any) *TableCell {
	if c == nil {
		return nil
	}
	c.Value = fmt.Sprintf(format, a...)
	c.IsStub = true
	return c
}

func (c *TableCell) SetMaxWidth(w int) *TableCell {
	if c == nil {
		return nil
	}
	c.MaxWidth = w
	return c
}

func (c *TableCell) SetEventsCount(count uint64) *TableCell {
	switch {
	case count < 1:
	case count > 1e6:
		c.Value = fmt.Sprintf("%.1f", float32(count)/1e6)
		c.SetUnit("M")
	case count > 1e3:
		c.Value = fmt.Sprintf("%.1f", float32(count)/1e3)
		c.SetUnit("k")
	default:
		c.Value = fmt.Sprintf("%d", count)
	}
	return c
}

type Icon struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}
