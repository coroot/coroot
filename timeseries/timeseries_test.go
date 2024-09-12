package timeseries

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFill(t *testing.T) {
	data := []float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	var ts *TimeSeries

	ts = New(0, 5, 15*Second)
	FillAny(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(0, 5, 15, [. 1 2 3 4])", ts.String())
	ts = New(0, 5, 15*Second)
	FillSum(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(0, 5, 15, [. 1 2 3 4])", ts.String())

	ts = New(15, 5, 15*Second)
	FillAny(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(15, 5, 15, [1 2 3 4 5])", ts.String())
	ts = New(15, 5, 15*Second)
	FillSum(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(15, 5, 15, [1 2 3 4 5])", ts.String())

	ts = New(30, 5, 15*Second)
	FillAny(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(30, 5, 15, [2 3 4 5 6])", ts.String())
	ts = New(30, 5, 15*Second)
	FillSum(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(30, 5, 15, [2 3 4 5 6])", ts.String())

	ts = New(60, 5, 15*Second)
	FillAny(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(60, 5, 15, [4 5 6 7 8])", ts.String())
	ts = New(60, 5, 15*Second)
	FillSum(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(60, 5, 15, [4 5 6 7 8])", ts.String())

	ts = New(30, 5, 30*Second)
	FillAny(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(30, 5, 30, [2 4 6 8 10])", ts.String())
	ts = New(30, 5, 30*Second)
	FillSum(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(30, 5, 30, [3 7 11 15 19])", ts.String())

	ts = New(30, 5, 30*Second)
	FillAny(ts, 45, 15*Second, data)
	assert.Equal(t, "TimeSeries(30, 5, 30, [. 2 4 6 8])", ts.String())
	ts = New(30, 5, 30*Second)
	FillSum(ts, 45, 15*Second, data)
	assert.Equal(t, "TimeSeries(30, 5, 30, [. 3 7 11 15])", ts.String())

	ts = New(30, 5, 30*Second)
	FillAny(ts, 60, 15*Second, data)
	assert.Equal(t, "TimeSeries(30, 5, 30, [. 1 3 5 7])", ts.String())
	ts = New(30, 5, 30*Second)
	FillSum(ts, 60, 15*Second, data)
	assert.Equal(t, "TimeSeries(30, 5, 30, [. 1 5 9 13])", ts.String())

	ts = New(60, 5, 30*Second)
	FillAny(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(60, 5, 30, [4 6 8 10 .])", ts.String())
	ts = New(60, 5, 30*Second)
	FillSum(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(60, 5, 30, [7 11 15 19 .])", ts.String())

	ts = New(60, 5, 30*Second)
	FillAny(ts, 45, 15*Second, data)
	assert.Equal(t, "TimeSeries(60, 5, 30, [2 4 6 8 10])", ts.String())
	ts = New(60, 5, 30*Second)
	FillSum(ts, 45, 15*Second, data)
	assert.Equal(t, "TimeSeries(60, 5, 30, [3 7 11 15 19])", ts.String())

	ts = New(60, 5, 30*Second)
	FillAny(ts, 60, 15*Second, data)
	assert.Equal(t, "TimeSeries(60, 5, 30, [1 3 5 7 9])", ts.String())
	ts = New(60, 5, 30*Second)
	FillSum(ts, 60, 15*Second, data)
	assert.Equal(t, "TimeSeries(60, 5, 30, [1 5 9 13 17])", ts.String())

	ts = New(0, 5, 30*Second)
	FillAny(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(0, 5, 30, [. 2 4 6 8])", ts.String())
	ts = New(0, 5, 30*Second)
	FillSum(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(0, 5, 30, [. 3 7 11 15])", ts.String())

	ts = New(0, 5, 30*Second)
	FillAny(ts, 30, 15*Second, data)
	assert.Equal(t, "TimeSeries(0, 5, 30, [. 1 3 5 7])", ts.String())
	ts = New(0, 5, 30*Second)
	FillSum(ts, 30, 15*Second, data)
	assert.Equal(t, "TimeSeries(0, 5, 30, [. 1 5 9 13])", ts.String())

	ts = New(0, 10, 30*Second)
	FillAny(ts, 30, 15*Second, data)
	assert.Equal(t, "TimeSeries(0, 10, 30, [. 1 3 5 7 9 10 . . .])", ts.String())
	ts = New(0, 10, 30*Second)
	FillSum(ts, 30, 15*Second, data)
	assert.Equal(t, "TimeSeries(0, 10, 30, [. 1 5 9 13 17 10 . . .])", ts.String())

	ts = New(0, 10, 30*Second)
	FillAny(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(0, 10, 30, [. 2 4 6 8 10 . . . .])", ts.String())
	ts = New(0, 10, 30*Second)
	FillSum(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(0, 10, 30, [. 3 7 11 15 19 . . . .])", ts.String())

	ts = New(45, 5, 45*Second)
	FillAny(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(45, 5, 45, [3 6 9 10 .])", ts.String())
	ts = New(45, 5, 45*Second)
	FillSum(ts, 15, 15*Second, data)
	assert.Equal(t, "TimeSeries(45, 5, 45, [6 15 24 10 .])", ts.String())

	ts = New(45, 5, 45*Second)
	FillAny(ts, 30, 15*Second, data)
	assert.Equal(t, "TimeSeries(45, 5, 45, [2 5 8 10 .])", ts.String())
	ts = New(45, 5, 45*Second)
	FillSum(ts, 30, 15*Second, data)
	assert.Equal(t, "TimeSeries(45, 5, 45, [3 12 21 19 .])", ts.String())

	data = []float32{1, 2, 3, 4, 5}

	ts = New(60, 5, 30*Second)
	FillAny(ts, 15, 15*Second, data)
	FillAny(ts, 90, 15*Second, data)
	assert.Equal(t, "TimeSeries(60, 5, 30, [4 1 3 5 .])", ts.String())
	ts = New(60, 5, 30*Second)
	FillSum(ts, 15, 15*Second, data)
	FillSum(ts, 90, 15*Second, data)
	assert.Equal(t, "TimeSeries(60, 5, 30, [7 6 5 9 .])", ts.String())
}

func BenchmarkFillAny(b *testing.B) {
	data := make([]float32, 960)
	for i := range data {
		data[i] = float32(i)
	}
	ts := New(1800, 180, 60*Second)
	FillAny(ts, 15, 15*Second, data)
	assert.Equal(b, "TimeSeries(1800, 180, 60, [119 123 127 131 135 139 143 147 151 155 159 163 167 171 175 179 183 187 191 195 199 203 207 211 215 219 223 227 231 235 239 243 247 251 255 259 263 267 271 275 279 283 287 291 295 299 303 307 311 315 319 323 327 331 335 339 343 347 351 355 359 363 367 371 375 379 383 387 391 395 399 403 407 411 415 419 423 427 431 435 439 443 447 451 455 459 463 467 471 475 479 483 487 491 495 499 503 507 511 515 519 523 527 531 535 539 543 547 551 555 559 563 567 571 575 579 583 587 591 595 599 603 607 611 615 619 623 627 631 635 639 643 647 651 655 659 663 667 671 675 679 683 687 691 695 699 703 707 711 715 719 723 727 731 735 739 743 747 751 755 759 763 767 771 775 779 783 787 791 795 799 803 807 811 815 819 823 827 831 835])", ts.String())
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		FillAny(New(1800, 180, 60*Second), 15, 15*Second, data)
	}
}

func BenchmarkFillSum(b *testing.B) {
	data := make([]float32, 960)
	for i := range data {
		data[i] = float32(i)
	}
	ts := New(1800, 180, 60*Second)
	FillSum(ts, 15, 15*Second, data)
	assert.Equal(b, "TimeSeries(1800, 180, 60, [470 486 502 518 534 550 566 582 598 614 630 646 662 678 694 710 726 742 758 774 790 806 822 838 854 870 886 902 918 934 950 966 982 998 1014 1030 1046 1062 1078 1094 1110 1126 1142 1158 1174 1190 1206 1222 1238 1254 1270 1286 1302 1318 1334 1350 1366 1382 1398 1414 1430 1446 1462 1478 1494 1510 1526 1542 1558 1574 1590 1606 1622 1638 1654 1670 1686 1702 1718 1734 1750 1766 1782 1798 1814 1830 1846 1862 1878 1894 1910 1926 1942 1958 1974 1990 2006 2022 2038 2054 2070 2086 2102 2118 2134 2150 2166 2182 2198 2214 2230 2246 2262 2278 2294 2310 2326 2342 2358 2374 2390 2406 2422 2438 2454 2470 2486 2502 2518 2534 2550 2566 2582 2598 2614 2630 2646 2662 2678 2694 2710 2726 2742 2758 2774 2790 2806 2822 2838 2854 2870 2886 2902 2918 2934 2950 2966 2982 2998 3014 3030 3046 3062 3078 3094 3110 3126 3142 3158 3174 3190 3206 3222 3238 3254 3270 3286 3302 3318 3334])", ts.String())
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		FillSum(New(1800, 180, 60*Second), 15, 15*Second, data)
	}
}

func TestIncrease(t *testing.T) {
	x := NewWithData(0, 1, []float32{NaN, 1, 1, 1, 2, 2, 2, NaN, NaN, 10, NaN, 11, 12})
	status := NewWithData(0, 1, []float32{1, 1, 1, 1, 1, 1, 1, NaN, 1, 1, 0, 1, 1})
	assert.Equal(t, "TimeSeries(0, 13, 1, [. 1 0 0 1 0 0 . . 10 . . 1])", Increase(x, status).String())
}

func TestIterFrom(t *testing.T) {
	ts := NewWithData(60, 15, []float32{1, 2, 3})

	iter := ts.IterFrom(30)
	assert.True(t, iter.Next())
	tt, v := iter.Value()
	assert.Equal(t, Time(60), tt)
	assert.Equal(t, float32(1), v)

	iter = ts.IterFrom(60)
	assert.True(t, iter.Next())
	tt, v = iter.Value()
	assert.Equal(t, Time(60), tt)
	assert.Equal(t, float32(1), v)

	iter = ts.IterFrom(70)
	assert.True(t, iter.Next())
	tt, v = iter.Value()
	assert.Equal(t, Time(60), tt)
	assert.Equal(t, float32(1), v)

	iter = ts.IterFrom(75)
	assert.True(t, iter.Next())
	tt, v = iter.Value()
	assert.Equal(t, Time(75), tt)
	assert.Equal(t, float32(2), v)

	iter = ts.IterFrom(90)
	assert.True(t, iter.Next())
	tt, v = iter.Value()
	assert.Equal(t, Time(90), tt)
	assert.Equal(t, float32(3), v)

	iter = ts.IterFrom(100)
	assert.False(t, iter.Next())
}
