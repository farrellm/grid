package format

import (
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
