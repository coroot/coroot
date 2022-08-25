package widgets

import "github.com/coroot/coroot-focus/model"

type Table struct {
	Header []string
	Rows   []*TableRow
}

func (t *Table) AddRow() *TableRow {
	r := &TableRow{}
	t.Rows = append(t.Rows, r)
	return r
}

type TableRow struct {
	Cells []*TableCell
}

func (r *TableRow) Text(value string) *TableRow {
	r.Cells = append(r.Cells, &TableCell{Value: value})
	return r
}

func (r *TableRow) WithIcon(value string, icon Icon) *TableRow {
	r.Cells = append(r.Cells, &TableCell{Value: value, Icon: icon})
	return r
}

func (r *TableRow) Status(value string, status model.Status) *TableRow {
	r.Cells = append(r.Cells, &TableCell{Value: value, Status: status})
	return r
}

func (r *TableRow) WithUnit(value string, unit string) *TableRow {
	r.Cells = append(r.Cells, &TableCell{Value: value, Unit: unit})
	return r
}

type TableCell struct {
	Icon   Icon
	Value  string
	Tags   []string
	Unit   string
	Status model.Status
}

type Icon struct {
	Icon  string
	Color string
}
