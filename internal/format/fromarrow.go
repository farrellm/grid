package format

import (
	"math"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
)

// Accumulate folds one Arrow array into the column's statistics.
func (s *ColumnStats) Accumulate(arr arrow.Array, cfg Config) {
	if arr == nil {
		return
	}
	for i := 0; i < arr.Len(); i++ {
		if arr.IsNull(i) {
			s.AddNull()
			continue
		}
		switch v := valueOf(arr, i); v.Kind {
		case KindString:
			s.AddString(v.S)
		case KindTime:
			s.Count++
		case KindBool:
			s.Count++
		default:
			s.AddNumber(v.Number(), cfg)
		}
	}
}

// valueOf is the subset of Arrow decoding that statistics need; the source
// package holds the full conversion used for rendering.
func valueOf(arr arrow.Array, i int) Value {
	switch a := arr.(type) {
	case *array.Boolean:
		return Bool(a.Value(i))
	case *array.Int8:
		return Int(int64(a.Value(i)))
	case *array.Int16:
		return Int(int64(a.Value(i)))
	case *array.Int32:
		return Int(int64(a.Value(i)))
	case *array.Int64:
		return Int(a.Value(i))
	case *array.Uint8:
		return Int(int64(a.Value(i)))
	case *array.Uint16:
		return Int(int64(a.Value(i)))
	case *array.Uint32:
		return Int(int64(a.Value(i)))
	case *array.Uint64:
		return Int(int64(a.Value(i)))
	case *array.Float32:
		return Float(float64(a.Value(i)))
	case *array.Float64:
		return Float(a.Value(i))
	case *array.String:
		return String(a.Value(i))
	case *array.LargeString:
		return String(a.Value(i))
	default:
		return String(arr.ValueStr(i))
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
		width := clamp(cfg.StrWidthMin, s.MaxStrWidth, cfg.StrWidthMax)
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
