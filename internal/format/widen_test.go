package format

import (
	"testing"
	"time"

	"github.com/farrellm/grid/internal/textutil"
)

func TestWidenLeavesWideEnoughAlone(t *testing.T) {
	f := NewInt(5, ' ', "-")
	if got := Widen(f, 3); got != Formatter(f) {
		t.Errorf("Widen to a narrower width gave %#v, want the formatter unchanged", got)
	}
}

// Each kind of column takes its extra space on the side it already pads, so a
// widened column reads the same as a naturally wide one.
func TestWiden(t *testing.T) {
	tests := []struct {
		name  string
		f     Formatter
		v     Value
		width int
		want  string
	}{
		{"int pads left", NewInt(1, ' ', "-"), Int(7), 6, "     7"},
		{"float pads left", NewFloat(1, 1, ' ', "-", ".", "NaN", "∞"), Float(1.5), 8, "     1.5"},
		// WithSize would have widened the exponent to E+0004.
		{"efloat keeps its exponent", NewEFloat(2, 2, "-", ".", "E", "NaN", "∞"), Float(12345), 12, "    1.23E+04"},
		{"string pads right and shows more", NewStr(4, "…", ' ', 1.0, false), String("alphabet"), 10, "alphabet  "},
		{"bool keeps its alignment", NewBool("TRUE", "FALSE", 1, true), Bool(true), 6, "  TRUE"},
		{"time pads right", NewTime("date", "NaN"), Time(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)), 12, "2024-01-02  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Widen(tt.f, tt.width)
			if got := w.Width(); got != tt.width {
				t.Errorf("Width() = %d, want %d", got, tt.width)
			}
			got := w.Format(tt.v)
			if got != tt.want {
				t.Errorf("Format = %q, want %q", got, tt.want)
			}
			if n := textutil.Width(got); n != tt.width {
				t.Errorf("rendered %d columns, want %d", n, tt.width)
			}
		})
	}
}
