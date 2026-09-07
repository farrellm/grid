package format

import (
	"math"
	"strconv"
	"strings"

	"github.com/farrellm/grid/internal/textutil"
)

// FloatFormatter renders floats in fixed-point notation with a fixed number of
// integral digits and fractional places.
type FloatFormatter struct {
	size      int
	precision int // NoPrecision suppresses the decimal point
	pad       rune
	sign      string
	point     string
	nanStr    string
	infStr    string
	width     int
}

// NewFloat creates a fixed-point formatter. Pass NoPrecision to suppress the
// decimal point entirely.
func NewFloat(size, precision int, pad rune, sign, point, nanStr, infStr string) *FloatFormatter {
	width := size
	if precision != NoPrecision {
		width += len(point) + precision
	}
	// The placeholders are cut to the pre-sign width, as Python does.
	nanStr = truncateBytes(nanStr, width)
	infStr = truncateBytes(infStr, width)
	if sign == "-" || sign == "+" {
		width++
	}
	return &FloatFormatter{
		size: size, precision: precision, pad: pad, sign: sign,
		point: point, nanStr: nanStr, infStr: infStr, width: width,
	}
}

func (f *FloatFormatter) Width() int     { return f.width }
func (f *FloatFormatter) Size() int      { return f.size }
func (f *FloatFormatter) Precision() int { return f.precision }

func (f *FloatFormatter) WithSize(size int) Formatter {
	return NewFloat(size, f.precision, f.pad, f.sign, f.point, f.nanStr, f.infStr)
}

func (f *FloatFormatter) WithPrecision(precision int) Formatter {
	return NewFloat(f.size, precision, f.pad, f.sign, f.point, f.nanStr, f.infStr)
}

func (f *FloatFormatter) Format(v Value) string {
	if v.IsNull() {
		return textutil.Pad(f.nanStr, f.width, ' ', true)
	}
	x := v.Number()
	// Python compares `value < 0`, so negative zero formats without a sign.
	neg := x < 0
	sign := signOf(neg, f.sign)

	switch {
	case math.IsNaN(x):
		return textutil.Pad(f.nanStr, f.width, ' ', true)
	case neg && f.sign == "":
		return hashes(f.width)
	case math.IsInf(x, 0):
		return textutil.Pad(sign+f.infStr, f.width, ' ', true)
	}

	// strconv rounds half-to-even on the exact binary value, which is what
	// Python's round() does; this avoids replaying ngrid's manual int/frac
	// arithmetic and its accumulated float error.
	prec := f.precision
	if prec == NoPrecision {
		prec = 0
	}
	digits := strconv.FormatFloat(math.Abs(x), 'f', prec, 64)

	intPart, fracPart, _ := strings.Cut(digits, ".")
	if len(intPart) > f.size {
		return hashes(f.width)
	}

	var b strings.Builder
	if f.pad == ' ' {
		// Space padding precedes the sign.
		b.WriteString(textutil.Pad(sign+intPart, f.size+len(sign), ' ', true))
	} else {
		// Zero padding follows the sign.
		b.WriteString(sign)
		b.WriteString(textutil.Pad(intPart, f.size, f.pad, true))
	}
	if f.precision != NoPrecision {
		b.WriteString(f.point)
		b.WriteString(fracPart)
	}
	return b.String()
}
