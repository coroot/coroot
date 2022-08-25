package widgets

import (
	"fmt"
	"github.com/coroot/coroot-focus/model"
)

type Table struct {
	Header []string    `json:"header"`
	Rows   []*TableRow `json:"rows"`
}

func (t *Table) AddRow(cells ...*TableCell) *TableRow {
	r := &TableRow{Cells: cells}
	t.Rows = append(t.Rows, r)
	return r
}

type TableRow struct {
	Cells []*TableCell `json:"cells"`
}

type TableCell struct {
	Icon   *Icon         `json:"icon"`
	Value  string        `json:"value"`
	Tags   []string      `json:"tags"`
	Unit   string        `json:"unit"`
	Status *model.Status `json:"status"`
}

func NewTableCell(value string) *TableCell {
	return &TableCell{Value: value}
}

func (c *TableCell) SetStatus(status model.Status) *TableCell {
	c.Status = &status
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
	c.Tags = append(c.Tags, fmt.Sprintf(format, a...))
	return c
}

type Icon struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}
