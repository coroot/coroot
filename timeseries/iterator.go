package timeseries

type Iterator interface {
	Next() bool
	Value() (Time, float64)
}

type NilIterator struct{}

func (i *NilIterator) Next() bool {
	return false
}

func (i *NilIterator) Value() (Time, float64) {
	panic("this code should never be called")
	return Time(0), NaN
}
