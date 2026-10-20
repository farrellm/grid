package format

import (
	"cmp"
	"math"
	"strconv"
	"strings"
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

// IsMissing reports whether the cell holds no usable value: a null, or a float
// NaN, which a numeric column reads a missing value as.
func (v Value) IsMissing() bool {
	return v.IsNull() || (v.Kind == KindFloat && math.IsNaN(v.F))
}

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

// Text renders any value as plain text, in full: no width, padding or elision.
// A string column shows values that failed to parse as something narrower this
// way, and a filter matches against it, so that what a filter keeps does not
// depend on how wide its column is drawn.
func Text(v Value) string {
	switch v.Kind {
	case KindNull:
		return ""
	case KindBool:
		if v.B {
			return "True"
		}
		return "False"
	case KindInt:
		return strconv.FormatInt(v.I, 10)
	case KindFloat:
		return strconv.FormatFloat(v.F, 'g', -1, 64)
	case KindTime:
		return v.T.Format(time.RFC3339)
	default:
		return v.S
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

// Compare orders two cells, returning -1, 0 or +1. Each pair is compared as its
// own kinds rather than its column's, since a column widened part way through a
// stream holds values of more than one kind: integers exactly, where a float64
// would round past 2^53; an integer and a float numerically; times in time
// order; false before true; and text by its bytes. Cells of kinds that cannot be
// compared with each other are ordered by kind, numbers before times before
// text, which keeps the order transitive where comparing their text would not
// (9 < 10, "10" < "5x" and "5x" < "9" would make a cycle). Missing values sort
// after everything else.
func Compare(a, b Value) int {
	am, bm := a.IsMissing(), b.IsMissing()
	switch {
	case am || bm:
		return CompareBool(am, bm)
	case a.Kind == KindInt && b.Kind == KindInt:
		return cmp.Compare(a.I, b.I)
	}
	if ra, rb := rank(a.Kind), rank(b.Kind); ra != rb {
		return cmp.Compare(ra, rb)
	}
	switch a.Kind {
	case KindBool:
		return CompareBool(a.B, b.B)
	case KindInt, KindFloat:
		return cmp.Compare(a.Number(), b.Number())
	case KindTime:
		return a.T.Compare(b.T)
	default:
		return strings.Compare(a.S, b.S)
	}
}

// rank groups the kinds that Compare can order against each other.
func rank(k Kind) int {
	switch k {
	case KindBool:
		return 0
	case KindInt, KindFloat:
		return 1
	case KindTime:
		return 2
	default:
		return 3
	}
}

// CompareBool orders false before true.
func CompareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case b:
		return -1
	default:
		return 1
	}
}
