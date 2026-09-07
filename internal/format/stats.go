package format

import (
	"math"
	"strconv"

	"github.com/farrellm/grid/internal/textutil"
)

// ColumnStats accumulates what choosing a formatter needs to know about a
// column. ngrid computed these with numpy over a materialised array; streaming
// them means the same code serves both a 100-row sample and a whole-file scan.
type ColumnStats struct {
	Count     int
	NullCount int

	AnyNegative bool
	AnyFinite   bool
	MaxAbs      float64 // over finite values only

	MaxStrWidth int

	// Precision is the smallest number of decimal places that represents every
	// value seen to within the tolerance implied by PrecisionMax.
	precision   int
	precisionOK bool
}

// AddNull records a missing value.
func (s *ColumnStats) AddNull() {
	s.Count++
	s.NullCount++
}

// AddNumber records a numeric value.
func (s *ColumnStats) AddNumber(x float64, cfg Config) {
	s.Count++
	if x < 0 {
		s.AnyNegative = true
	}
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return
	}
	s.AnyFinite = true
	if a := math.Abs(x); a > s.MaxAbs {
		s.MaxAbs = a
	}
	if p := minPrecision(x, cfg); p > s.precision {
		s.precision = p
		s.precisionOK = true
	}
}

// AddString records a text value.
func (s *ColumnStats) AddString(v string) {
	s.Count++
	if w := textutil.Width(v); w > s.MaxStrWidth {
		s.MaxStrWidth = w
	}
}

// Precision returns the accumulated precision, clamped to the configured range.
func (s *ColumnStats) Precision(cfg Config) int {
	if !s.precisionOK {
		return cfg.PrecisionMin
	}
	return clamp(cfg.PrecisionMin, s.precision, cfg.PrecisionMax)
}

// minPrecision finds the fewest decimal places at which rounding x leaves a
// residual smaller than half a unit in the last configured place — ngrid's
// progressive search, decided per value rather than per column.
func minPrecision(x float64, cfg Config) int {
	tol := math.Pow(10, -float64(cfg.PrecisionMax)) / 2
	for p := cfg.PrecisionMin; p <= cfg.PrecisionMax; p++ {
		if math.Abs(roundTo(x, p)-x) < tol {
			return p
		}
	}
	return cfg.PrecisionMax
}

// roundTo rounds x to p decimal places with Python's round() semantics.
func roundTo(x float64, p int) float64 {
	r, err := strconv.ParseFloat(strconv.FormatFloat(x, 'f', p, 64), 64)
	if err != nil {
		return x
	}
	return r
}

// digitCount returns how many digits the integral part of x needs.
func digitCount(x float64) int {
	x = math.Abs(x)
	if x == 0 {
		return 1
	}
	return max(int(math.Floor(math.Log10(x)+1)), 1)
}

func clamp(lo, v, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
