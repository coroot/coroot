package timeseries

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func issue694MissingSeries(points int) *TimeSeries {
	return New(0, points, Minute)
}

func issue694Series(values ...float32) *TimeSeries {
	return NewWithData(0, Minute, values)
}

func TestIssue694NanSumAggregation(t *testing.T) {
	tests := []struct {
		name          string
		inputs        func() []*TimeSeries
		want          string
		wantTailEmpty bool
	}{
		{
			name: "one_all_nan_input",
			inputs: func() []*TimeSeries {
				return []*TimeSeries{issue694MissingSeries(3)}
			},
			want:          "TimeSeries(0, 3, 60, [. . .])",
			wantTailEmpty: true,
		},
		{
			name: "two_all_nan_inputs",
			inputs: func() []*TimeSeries {
				return []*TimeSeries{issue694MissingSeries(3), issue694MissingSeries(3)}
			},
			want:          "TimeSeries(0, 3, 60, [0 0 0])",
			wantTailEmpty: false,
		},
		{
			name: "nan_and_defined_zero",
			inputs: func() []*TimeSeries {
				return []*TimeSeries{issue694MissingSeries(3), issue694Series(0, 0, 0)}
			},
			want:          "TimeSeries(0, 3, 60, [0 0 0])",
			wantTailEmpty: false,
		},
		{
			name: "nan_and_positive_values",
			inputs: func() []*TimeSeries {
				return []*TimeSeries{issue694MissingSeries(3), issue694Series(1, 2, 3)}
			},
			want:          "TimeSeries(0, 3, 60, [1 2 3])",
			wantTailEmpty: false,
		},
		{
			name: "multiple_defined_zero_series",
			inputs: func() []*TimeSeries {
				return []*TimeSeries{issue694Series(0, 0, 0), issue694Series(0, 0, 0)}
			},
			want:          "TimeSeries(0, 3, 60, [0 0 0])",
			wantTailEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAggregate(NanSum).Add(tt.inputs()...).Get()
			require.NotNil(t, result)
			t.Logf("result: %s", result)
			assert.Equal(t, tt.want, result.String())
			assert.Equal(t, tt.wantTailEmpty, result.TailIsEmpty())
		})
	}
}

func TestIssue694NanSumOrNaNAggregation(t *testing.T) {
	tests := []struct {
		name          string
		inputs        func() []*TimeSeries
		want          string
		wantTailEmpty bool
	}{
		{
			name: "one_all_nan_input",
			inputs: func() []*TimeSeries {
				return []*TimeSeries{issue694MissingSeries(3)}
			},
			want:          "TimeSeries(0, 3, 60, [. . .])",
			wantTailEmpty: true,
		},
		{
			name: "two_all_nan_inputs",
			inputs: func() []*TimeSeries {
				return []*TimeSeries{issue694MissingSeries(3), issue694MissingSeries(3)}
			},
			want:          "TimeSeries(0, 3, 60, [. . .])",
			wantTailEmpty: true,
		},
		{
			name: "three_all_nan_inputs",
			inputs: func() []*TimeSeries {
				return []*TimeSeries{issue694MissingSeries(3), issue694MissingSeries(3), issue694MissingSeries(3)}
			},
			want:          "TimeSeries(0, 3, 60, [. . .])",
			wantTailEmpty: true,
		},
		{
			name: "nan_and_defined_zero",
			inputs: func() []*TimeSeries {
				return []*TimeSeries{issue694MissingSeries(3), issue694Series(0, 0, 0)}
			},
			want:          "TimeSeries(0, 3, 60, [0 0 0])",
			wantTailEmpty: false,
		},
		{
			name: "nan_and_positive_values",
			inputs: func() []*TimeSeries {
				return []*TimeSeries{issue694MissingSeries(3), issue694Series(1, 2, 3)}
			},
			want:          "TimeSeries(0, 3, 60, [1 2 3])",
			wantTailEmpty: false,
		},
		{
			name: "multiple_defined_zero_series",
			inputs: func() []*TimeSeries {
				return []*TimeSeries{issue694Series(0, 0, 0), issue694Series(0, 0, 0)}
			},
			want:          "TimeSeries(0, 3, 60, [0 0 0])",
			wantTailEmpty: false,
		},
		{
			name: "mixed_defined_and_missing_samples",
			inputs: func() []*TimeSeries {
				return []*TimeSeries{
					issue694Series(NaN, 0, NaN, 2),
					issue694Series(1, NaN, NaN, 3),
					issue694MissingSeries(4),
				}
			},
			want:          "TimeSeries(0, 4, 60, [1 0 . 5])",
			wantTailEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAggregate(NanSumOrNaN).Add(tt.inputs()...).Get()
			require.NotNil(t, result)
			t.Logf("result: %s", result)
			assert.Equal(t, tt.want, result.String())
			assert.Equal(t, tt.wantTailEmpty, result.TailIsEmpty())
		})
	}
}
