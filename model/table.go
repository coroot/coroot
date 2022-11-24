package model

import (
	"fmt"
	"github.com/coroot/coroot/timeseries"
	"sort"
)

type Table struct {
	Header []string    `json:"header"`
	Rows   []*TableRow `json:"rows"`
}

func (t *Table) AddRow(cells ...*TableCell) *TableRow {
	r := &TableRow{Cells: cells}
	t.Rows = append(t.Rows, r)
	t.SortRows()
	return r
}

func (t *Table) SortRows() {
	sort.SliceStable(t.Rows, func(i, j int) bool {
		return t.Rows[i].Cells[0].Value < t.Rows[j].Cells[0].Value
	})
}

type TableRow struct {
	Cells []*TableCell `json:"cells"`
}

type Progress struct {
	Percent int    `json:"percent"`
	Color   string `json:"color"`
}

type NetInterface struct {
	Name string
	Rx   string
	Tx   string
}

type TableCellLink struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

type TableCell struct {
	Icon          *Icon                 `json:"icon"`
	Value         string                `json:"value"`
	Values        []string              `json:"values"`
	Tags          []string              `json:"tags"`
	Unit          string                `json:"unit"`
	Status        *Status               `json:"status"`
	Link          *TableCellLink        `json:"link"`
	Progress      *Progress             `json:"progress"`
	NetInterfaces []NetInterface        `json:"net_interfaces"`
	Chart         timeseries.TimeSeries `json:"chart"`
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
	c.Status = &status
	c.Value = msg
	return c
}

func (c *TableCell) SetValue(value string) *TableCell {
	c.Value = value
	return c
}

func (c *TableCell) SetIcon(name, color string) *TableCell {
	c.Icon = &Icon{Name: name, Color: color}
	return c
}

func (c *TableCell) SetUnit(unit string) *TableCell {
	c.Unit = unit
	return c
}

func (c *TableCell) AddTag(format string, a ...any) *TableCell {
	if format != "" {
		c.Tags = append(c.Tags, fmt.Sprintf(format, a...))
	}
	return c
}

func (c *TableCell) SetLink(typ, key string) *TableCell {
	c.Link = &TableCellLink{Type: typ, Key: key}
	return c
}

func (c *TableCell) SetProgress(percent int, color string) *TableCell {
	c.Progress = &Progress{Percent: percent, Color: color}
	return c
}

func (c *TableCell) SetChart(ts timeseries.TimeSeries) *TableCell {
	c.Chart = ts
	return c
}

type Icon struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}
