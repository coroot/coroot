package timeseries

//func TestInMemoryTimeseries(t *testing.T) {
//	ctx := Context{From: 120, To: 360, Step: 60}
//
//	ts := NewNan(ctx)
//	ts.Set(180, 5)
//	ts.Set(300, 7)
//	ts.Set(360, 10)
//	assert.Equal(t, "InMemoryTimeSeries(120, 360, 60, [. 5 . 7 10])", ts.String())
//
//	compressed, err := ts.ToBinary()
//	assert.NoError(t, err)
//
//	ts1, err := InMemoryTimeSeriesFromBinary(compressed)
//	assert.NoError(t, err)
//	assert.Equal(t, ts.String(), ts1.String())
//}
