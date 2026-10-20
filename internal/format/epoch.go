package format

import (
	"math"
	"time"
)

// EpochFormatter renders numbers as Unix epoch times.
type EpochFormatter struct {
	// unit is what one count of the number is worth. Zero decides it per value
	// from the value's magnitude, for a column whose formatter does not say
	// how many digits it holds.
	unit time.Duration
	t    *TimeFormatter
}

// NewEpoch creates a formatter for numbers that orig was drawing, rendered by
// t. The unit is chosen from how many integral digits orig was sized for, so a
// column of 13-digit JavaScript timestamps reads as milliseconds.
func NewEpoch(orig Formatter, t *TimeFormatter) *EpochFormatter {
	var unit time.Duration
	switch f := orig.(type) {
	case *IntFormatter:
		unit = epochUnit(f.size)
	case *FloatFormatter:
		unit = epochUnit(f.size)
	}
	return &EpochFormatter{unit: unit, t: t}
}

// epochUnit picks the unit for a count of the given number of digits. Eleven
// digits of seconds reach the year 5138, so anything longer is a finer unit.
func epochUnit(digits int) time.Duration {
	switch {
	case digits <= 11:
		return time.Second
	case digits <= 14:
		return time.Millisecond
	case digits <= 17:
		return time.Microsecond
	default:
		return time.Nanosecond
	}
}

func (f *EpochFormatter) Width() int { return f.t.Width() }

func (f *EpochFormatter) Format(v Value) string {
	unit := f.unit
	switch v.Kind {
	case KindInt:
		if unit == 0 {
			unit = epochUnit(digitCount(float64(v.I)))
		}
		// Split rather than multiply, so a count of seconds cannot overflow
		// the nanoseconds of a Duration.
		per := int64(time.Second / unit)
		sec, frac := v.I/per, v.I%per
		return f.t.Format(Time(time.Unix(sec, frac*int64(unit)).UTC()))
	case KindFloat:
		if math.IsNaN(v.F) || math.IsInf(v.F, 0) {
			break
		}
		if unit == 0 {
			unit = epochUnit(digitCount(v.F))
		}
		sec, frac := math.Modf(v.F * unit.Seconds())
		return f.t.Format(Time(time.Unix(int64(sec), int64(frac*1e9)).UTC()))
	}
	return f.t.Format(Null())
}
