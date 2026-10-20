package format

import (
	"math"
	"strconv"
	"strings"

	"github.com/farrellm/grid/internal/textutil"
)

// GroupedFormatter renders numbers with their integral digits in groups of
// three: 1234567.5 as "1,234,567.5".
type GroupedFormatter struct {
	size      int // integral digits, not counting separators
	precision int // NoPrecision suppresses the decimal point
	sign      string
	width     int
}

// NewGrouped creates a formatter for numbers of at most size integral digits.
func NewGrouped(size, precision int, sign string) *GroupedFormatter {
	width := size + (size-1)/3
	if precision != NoPrecision {
		width += 1 + precision
	}
	if sign == "-" || sign == "+" {
		width++
	}
	return &GroupedFormatter{size: size, precision: precision, sign: sign, width: width}
}

func (f *GroupedFormatter) Width() int     { return f.width }
func (f *GroupedFormatter) Size() int      { return f.size }
func (f *GroupedFormatter) Precision() int { return f.precision }

func (f *GroupedFormatter) WithSize(size int) Formatter {
	return NewGrouped(size, f.precision, f.sign)
}

func (f *GroupedFormatter) WithPrecision(precision int) Formatter {
	return NewGrouped(f.size, precision, f.sign)
}

func (f *GroupedFormatter) Format(v Value) string {
	if v.IsNull() {
		return textutil.Pad("", f.width, ' ', true)
	}

	var neg bool
	var digits string
	prec := max(f.precision, 0)
	if v.Kind == KindInt {
		// Exact, where a float64 would lose the last digits of a large int.
		neg = v.I < 0
		digits = strings.TrimPrefix(strconv.FormatInt(v.I, 10), "-")
		if prec > 0 {
			digits += "." + strings.Repeat("0", prec)
		}
	} else {
		x := v.Number()
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return hashes(f.width)
		}
		neg = x < 0
		digits = strconv.FormatFloat(math.Abs(x), 'f', prec, 64)
	}
	if neg && f.sign == "" {
		return hashes(f.width)
	}

	intPart, fracPart, _ := strings.Cut(digits, ".")
	if len(intPart) > f.size {
		return hashes(f.width)
	}

	var b strings.Builder
	b.WriteString(signOf(neg, f.sign))
	for i, d := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	if f.precision != NoPrecision {
		b.WriteByte('.')
		b.WriteString(fracPart)
	}
	return textutil.Pad(b.String(), f.width, ' ', true)
}
