package timeseries

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/pierrec/lz4"
	"strings"
)

type InMemoryTimeSeries struct {
	ctx  Context
	data []float64
}

func (ts *InMemoryTimeSeries) Range() Context {
	return ts.ctx
}

func (ts *InMemoryTimeSeries) Len() int {
	return len(ts.data)
}

func (ts *InMemoryTimeSeries) Last() float64 {
	if len(ts.data) == 0 {
		return NaN
	}
	return ts.data[len(ts.data)-1]
}

func (ts *InMemoryTimeSeries) Iter() Iterator {
	return &timeseriesIterator{
		ctx:  ts.ctx,
		data: ts.data,
		idx:  -1,
	}
}

func (ts *InMemoryTimeSeries) String() string {
	values := make([]string, 0)
	iter := ts.Iter()
	for iter.Next() {
		_, v := iter.Value()
		values = append(values, Value(v).String())
	}
	return fmt.Sprintf("InMemoryTimeSeries(%d, %d, %d, [%s])", ts.ctx.From, ts.ctx.To, ts.ctx.Step, strings.Join(values, " "))
}

func (ts *InMemoryTimeSeries) Set(t Time, v float64) {
	t = t.Truncate(ts.ctx.Step)
	if t < ts.ctx.From || t > ts.ctx.To {
		return
	}
	ts.data[int((t-ts.ctx.From)/Time(ts.ctx.Step))] = v
}

func NewNan(ctx Context) *InMemoryTimeSeries {
	return New(ctx, NaN)
}

func New(ctx Context, value float64) *InMemoryTimeSeries {
	data := make([]float64, (ctx.To-ctx.From)/Time(ctx.Step)+1)
	for i := range data {
		data[i] = value
	}
	return &InMemoryTimeSeries{
		ctx:  ctx,
		data: data,
	}
}

type timeseriesIterator struct {
	ctx  Context
	data []float64
	idx  int

	t Time
	v float64
}

func (i *timeseriesIterator) Next() bool {
	i.idx++
	t := i.ctx.From.Add(Duration(i.idx) * i.ctx.Step)
	if t > i.ctx.To {
		return false
	}
	i.t = t
	if i.data != nil {
		i.v = i.data[i.idx]
	}
	return true
}

func (i *timeseriesIterator) Value() (Time, float64) {
	return i.t, i.v
}

func (ts *InMemoryTimeSeries) MarshalJSON() ([]byte, error) {
	return MarshalJSON(ts)
}

func (ts *InMemoryTimeSeries) IsEmpty() bool {
	return len(ts.data) == 0
}

func (ts *InMemoryTimeSeries) ToBinary() ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, ts.ctx); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, ts.data); err != nil {
		return nil, err
	}
	res := make([]byte, 8+lz4.CompressBlockBound(buf.Len()))
	binary.PutUvarint(res, uint64(buf.Len()))
	n, err := lz4.CompressBlock(buf.Bytes(), res[8:], nil)
	if err != nil {
		return nil, err
	}
	return res[:n+8], nil
}

func InMemoryTimeSeriesFromBinary(data []byte) (*InMemoryTimeSeries, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("invalid input")
	}
	l, _ := binary.Uvarint(data)
	uncompressed := make([]byte, l)
	if _, err := lz4.UncompressBlock(data[8:], uncompressed); err != nil {
		return nil, err
	}
	r := bytes.NewReader(uncompressed)

	ctx := Context{}
	if err := binary.Read(r, binary.LittleEndian, &ctx); err != nil {
		return nil, err
	}
	ts := NewNan(ctx)
	if err := binary.Read(r, binary.LittleEndian, &ts.data); err != nil {
		return nil, err
	}
	return ts, nil
}
