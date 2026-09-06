package format

import (
	"math"
	"strconv"
	"strings"

	"github.com/farrellm/grid/internal/textutil"
)

// EFloatFormatter renders floats in scientific notation with a fixed-width
// exponent, e.g. " 1.23E+11".
type EFloatFormatter struct {
	size      int // digits in the exponent
	precision int // digits after the point; NoPrecision suppresses it
	Sign      string
	Point     string
	Exp       string
	NaNStr    string
	InfStr    string
	width     int
}

// NewEFloat creates a scientific-notation formatter with size exponent digits.
func NewEFloat(size, precision int, sign, point, exp, nanStr, infStr string) *EFloatFormatter {
	width := 1 + len(exp) + 1 + size
	if precision != NoPrecision {
		width += len(point) + precision
	}
	nanStr = truncate(nanStr, width)
	infStr = truncate(infStr, width)
	if sign == "-" || sign == "+" {
		width++
	}
	return &EFloatFormatter{
		size: size, precision: precision, Sign: sign, Point: point,
		Exp: exp, NaNStr: nanStr, InfStr: infStr, width: width,
	}
}

func (f *EFloatFormatter) Width() int     { return f.width }
func (f *EFloatFormatter) Size() int      { return f.size }
func (f *EFloatFormatter) Precision() int { return f.precision }

func (f *EFloatFormatter) WithSize(size int) Formatter {
	return NewEFloat(size, f.precision, f.Sign, f.Point, f.Exp, f.NaNStr, f.InfStr)
}

func (f *EFloatFormatter) WithPrecision(precision int) Formatter {
	return NewEFloat(f.size, precision, f.Sign, f.Point, f.Exp, f.NaNStr, f.InfStr)
}

func (f *EFloatFormatter) Format(v Value) string {
	if v.IsNull() {
		return textutil.Pad(f.NaNStr, f.width, ' ', true)
	}
	x := v.Number()
	neg := x < 0
	sign := signOf(neg, f.Sign)

	switch {
	case math.IsNaN(x):
		return textutil.Pad(f.NaNStr, f.width, ' ', true)
	case neg && f.Sign == "":
		return hashes(f.width)
	case math.IsInf(x, 0):
		return textutil.Pad(sign+f.InfStr, f.width, ' ', true)
	}

	// 'e' formatting rounds the mantissa and carries into the exponent for us
	// (9.996e99 -> 1.00e+100), which is what ngrid's log10 arithmetic was
	// reconstructing by hand.
	prec := f.precision
	if prec == NoPrecision {
		prec = 0
	}
	s := strconv.FormatFloat(math.Abs(x), 'e', prec, 64)

	mantissa, expStr, ok := strings.Cut(s, "e")
	if !ok {
		return hashes(f.width)
	}
	exp, err := strconv.Atoi(expStr)
	if err != nil {
		return hashes(f.width)
	}

	absExp := strconv.Itoa(exp)
	if exp < 0 {
		absExp = absExp[1:]
	}
	if len(absExp) > f.size {
		return hashes(f.width)
	}
	expSign := "+"
	if exp < 0 {
		expSign = "-"
	}

	intPart, fracPart, _ := strings.Cut(mantissa, ".")

	var b strings.Builder
	b.WriteString(sign)
	b.WriteString(intPart)
	if f.precision != NoPrecision {
		b.WriteString(f.Point)
		b.WriteString(fracPart)
	}
	b.WriteString(f.Exp)
	b.WriteString(expSign)
	b.WriteString(textutil.Pad(absExp, f.size, '0', true))
	return b.String()
}
