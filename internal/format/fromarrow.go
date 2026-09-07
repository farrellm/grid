package format

import (
	"math"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
)

// FromArrow converts one element of an Arrow array into a Value. It is the
// single decoding path: the view renders what it returns and the column
// statistics are gathered from it, so a column can never be sized from one
// reading of a cell and displayed from another.
//
// A nil array, an out-of-range index and a null element all yield a null Value,
// so callers need no bounds check of their own.
func FromArrow(arr arrow.Array, i int) Value {
	if arr == nil || i < 0 || i >= arr.Len() || arr.IsNull(i) {
		return Null()
	}
	// The cases are ordered by how often they come up rather than by width. A
	// type switch over concrete types lowers to a chain of comparisons, so the
	// order is the cost: text, doubles and 64-bit integers are what schema
	// inference produces for delimited input and what most Parquet files hold,
	// and this runs once per cell rendered.
	switch a := arr.(type) {
	case *array.String:
		return String(a.Value(i))
	case *array.Float64:
		return Float(a.Value(i))
	case *array.Int64:
		return Int(a.Value(i))
	case *array.Boolean:
		return Bool(a.Value(i))
	case *array.Int8:
		return Int(int64(a.Value(i)))
	case *array.Int16:
		return Int(int64(a.Value(i)))
	case *array.Int32:
		return Int(int64(a.Value(i)))
	case *array.Uint8:
		return Int(int64(a.Value(i)))
	case *array.Uint16:
		return Int(int64(a.Value(i)))
	case *array.Uint32:
		return Int(int64(a.Value(i)))
	case *array.Uint64:
		return Int(int64(a.Value(i)))
	case *array.Float16:
		return Float(float64(a.Value(i).Float32()))
	case *array.Float32:
		return Float(float64(a.Value(i)))
	case *array.LargeString:
		return String(a.Value(i))
	case *array.Binary:
		return String(string(a.Value(i)))
	case *array.Date32:
		return Time(a.Value(i).ToTime())
	case *array.Date64:
		return Time(a.Value(i).ToTime())
	case *array.Timestamp:
		unit := arrow.Microsecond
		if tt, ok := a.DataType().(*arrow.TimestampType); ok {
			unit = tt.Unit
		}
		return Time(a.Value(i).ToTime(unit))
	case *array.Time32:
		unit := arrow.Second
		if tt, ok := a.DataType().(*arrow.Time32Type); ok {
			unit = tt.Unit
		}
		return Time(a.Value(i).ToTime(unit))
	case *array.Time64:
		unit := arrow.Microsecond
		if tt, ok := a.DataType().(*arrow.Time64Type); ok {
			unit = tt.Unit
		}
		return Time(a.Value(i).ToTime(unit))
	default:
		return String(arr.ValueStr(i))
	}
}

// Accumulate folds one Arrow array into the column's statistics.
//
// Every element of an Arrow array has the same type, so the two that dominate
// real tables are dispatched once here rather than once per cell. The general
// path below decides the same thing per element and gives the same answer; it
// is what every other type takes.
func (s *ColumnStats) Accumulate(arr arrow.Array, cfg Config) {
	if arr == nil {
		return
	}
	switch a := arr.(type) {
	case *array.String:
		for i := range a.Len() {
			if a.IsNull(i) {
				s.AddNull()
			} else {
				s.AddString(a.Value(i))
			}
		}
		return

	case *array.Float64:
		for i := range a.Len() {
			if a.IsNull(i) {
				s.AddNull()
			} else {
				s.AddNumber(a.Value(i), cfg)
			}
		}
		return
	}

	for i := range arr.Len() {
		switch v := FromArrow(arr, i); v.Kind {
		case KindNull:
			s.AddNull()
		case KindString:
			s.AddString(v.S)
		case KindTime, KindBool:
			s.Count++
		default:
			s.AddNumber(v.Number(), cfg)
		}
	}
}

// DefaultFormatter chooses a formatter for a column from its Arrow type and
// the values seen so far. It is a port of ngrid's get_default_formatter.
func DefaultFormatter(dt arrow.DataType, s *ColumnStats, cfg Config) Formatter {
	switch dt.ID() {
	case arrow.BOOL:
		return NewBool("TRUE", "FALSE", 1, true)

	case arrow.INT8, arrow.INT16, arrow.INT32, arrow.INT64,
		arrow.UINT8, arrow.UINT16, arrow.UINT32, arrow.UINT64:
		return NewInt(digitCount(s.MaxAbs), ' ', "-")

	case arrow.FLOAT16, arrow.FLOAT32, arrow.FLOAT64:
		return floatFormatter(s, cfg)

	case arrow.DATE32, arrow.DATE64:
		return NewTime("date", cfg.NaNString)

	case arrow.TIMESTAMP:
		return NewTime(cfg.TimeFormat, cfg.NaNString)

	case arrow.TIME32, arrow.TIME64:
		return NewTime("time", cfg.NaNString)

	default:
		width := min(max(s.MaxStrWidth, cfg.StrWidthMin), cfg.StrWidthMax)
		return NewStr(width, cfg.Ellipsis, ' ', 1.0, false)
	}
}

// floatFormatter picks fixed-point or scientific notation and sizes it, as
// ngrid does for float columns.
func floatFormatter(s *ColumnStats, cfg Config) Formatter {
	sign := "-"
	if !s.AnyNegative {
		sign = ""
	}

	if !s.AnyFinite {
		// Nothing but NaN and infinities; a minimal column still shows them.
		return NewFloat(1, 1, ' ', sign, ".", cfg.NaNString, cfg.InfString)
	}

	precision := s.Precision(cfg)
	maxAbs := s.MaxAbs

	if maxAbs == 0 || (cfg.FixedRangeMin < maxAbs && maxAbs < cfg.FixedRangeMax) {
		return NewFloat(digitCount(maxAbs), precision, ' ', sign, ".",
			cfg.NaNString, cfg.InfString)
	}

	// Very large or very small: size the exponent field to hold its digits.
	expDigits := max(1, int(math.Ceil(math.Log10(math.Floor(math.Abs(math.Log10(maxAbs)))))))
	return NewEFloat(expDigits, precision, sign, ".", "E",
		cfg.NaNString, cfg.InfString)
}
