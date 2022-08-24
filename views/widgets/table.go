package widgets

type Table struct {
	Header []string
	Rows   []*TableRow
}

type TableRow struct {
	Cells []*TableCell
}

type TableCell struct {
	Type  string
	Value interface{}
	Tags  []string
	Unit  string
}
