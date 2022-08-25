package widgets

import "github.com/coroot/coroot-focus/model"

type Table struct {
	Header []string    `json:"header"`
	Rows   []*TableRow `json:"rows"`
}

func (t *Table) AddRow() *TableRow {
	r := &TableRow{}
	t.Rows = append(t.Rows, r)
	return r
}

type TableRow struct {
	Cells []*TableCell `json:"cells"`
}

func (r *TableRow) Add(cell *TableCell) *TableRow {
	r.Cells = append(r.Cells, cell)
	return r
}

func (r *TableRow) Text(value string, tags ...string) *TableRow {
	r.Cells = append(r.Cells, &TableCell{Value: value, Tags: tags})
	return r
}

func (r *TableRow) WithIcon(value string, icon Icon) *TableRow {
	r.Cells = append(r.Cells, &TableCell{Value: value, Icon: &icon})
	return r
}

func (r *TableRow) Status(value string, status model.Status) *TableRow {
	r.Cells = append(r.Cells, &TableCell{Value: value, Status: &status})
	return r
}

func (r *TableRow) WithUnit(value string, unit string, tags ...string) *TableRow {
	r.Cells = append(r.Cells, &TableCell{Value: value, Unit: unit})
	return r
}

type TableCell struct {
	Icon   *Icon         `json:"icon"`
	Value  string        `json:"value"`
	Tags   []string      `json:"tags"`
	Unit   string        `json:"unit"`
	Status *model.Status `json:"status"`
}

type Icon struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}
