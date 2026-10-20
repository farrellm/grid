package format

import (
	"math"
	"testing"
	"time"
)

func TestText(t *testing.T) {
	ts := time.Date(2024, 1, 5, 12, 30, 0, 0, time.UTC)
	for _, tt := range []struct {
		v    Value
		want string
	}{
		{Null(), ""},
		{Bool(true), "True"},
		{Bool(false), "False"},
		{Int(-42), "-42"},
		{Float(2.5), "2.5"},
		{Float(1e21), "1e+21"},
		{String("a rather long value, never elided"), "a rather long value, never elided"},
		{Time(ts), "2024-01-05T12:30:00Z"},
	} {
		if got := Text(tt.v); got != tt.want {
			t.Errorf("Text(%+v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestCompare(t *testing.T) {
	t1 := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	nan := Float(math.NaN())
	for _, tt := range []struct {
		a, b Value
		want int
	}{
		{Int(2), Int(10), -1},
		// Exact past 2^53, where both would round to the same float64.
		{Int(1<<53 + 1), Int(1 << 53), 1},
		{Int(2), Float(2.5), -1},
		{Float(3), Int(3), 0},
		{Time(t2), Time(t1), 1},
		{Bool(false), Bool(true), -1},
		{String("b"), String("a"), 1},
		// Kinds that can't be compared order numbers before text.
		{Int(10), String("5x"), -1},
		{String("5x"), Int(9), 1},
		// Missing values come after everything, and tie with each other.
		{Null(), Int(1), 1},
		{String(""), Null(), -1},
		{nan, Float(-1), 1},
		{nan, Null(), 0},
	} {
		if got := Compare(tt.a, tt.b); got != tt.want {
			t.Errorf("Compare(%+v, %+v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
