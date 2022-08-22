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

type NanIterator struct {
	startTs   Time
	endTs     Time
	step      Duration
	currentTs Time
}

func (i *NanIterator) Next() bool {
	if i.currentTs == 0 {
		i.currentTs = i.startTs
	} else {
		i.currentTs = i.currentTs.Add(i.step)
	}
	return i.currentTs < i.endTs
}

func (i *NanIterator) Value() (Time, float64) {
	return i.currentTs, NaN
}
