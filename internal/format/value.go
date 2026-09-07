package format

import (
	"math"
	"time"
)

// Kind discriminates the payload carried by a Value.
type Kind uint8

const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindFloat
	KindString
	KindTime
)

// Value is a single cell. It is a tagged union rather than an `any` so that
// rendering a screenful of cells does not allocate.
type Value struct {
	Kind Kind
	B    bool
	I    int64
	F    float64
	S    string
	T    time.Time
}

func Null() Value            { return Value{Kind: KindNull} }
func Bool(b bool) Value      { return Value{Kind: KindBool, B: b} }
func Int(i int64) Value      { return Value{Kind: KindInt, I: i} }
func Float(f float64) Value  { return Value{Kind: KindFloat, F: f} }
func String(s string) Value  { return Value{Kind: KindString, S: s} }
func Time(t time.Time) Value { return Value{Kind: KindTime, T: t} }

// IsNull reports whether the cell holds no value.
func (v Value) IsNull() bool { return v.Kind == KindNull }

// Truth coerces to a bool the way Python's truthiness does, which is what
// ngrid's BoolFormatter relies on: nonzero numbers and nonempty strings are
// true. A null is false.
func (v Value) Truth() bool {
	switch v.Kind {
	case KindBool:
		return v.B
	case KindInt:
		return v.I != 0
	case KindFloat:
		return v.F != 0 && !math.IsNaN(v.F)
	case KindString:
		return v.S != ""
	case KindTime:
		return !v.T.IsZero()
	default:
		return false
	}
}

// Number coerces to a float64. Nulls and non-numeric values become NaN.
func (v Value) Number() float64 {
	switch v.Kind {
	case KindBool:
		if v.B {
			return 1
		}
		return 0
	case KindInt:
		return float64(v.I)
	case KindFloat:
		return v.F
	default:
		return math.NaN()
	}
}
