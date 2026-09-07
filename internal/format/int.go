package format

import (
	"math"
	"strconv"
	"strings"

	"github.com/farrellm/grid/internal/textutil"
)

// IntFormatter renders integers right-aligned in a fixed number of digits.
type IntFormatter struct {
	size  int
	pad   rune   // ' ' or '0'
	sign  string // "-", "+", or "" for none
	width int
}

// NewInt creates a formatter for integers of at most size digits.
func NewInt(size int, pad rune, sign string) *IntFormatter {
	width := size
	if sign == "-" || sign == "+" {
		width++
	}
	return &IntFormatter{size: size, pad: pad, sign: sign, width: width}
}

func (f *IntFormatter) Width() int { return f.width }
func (f *IntFormatter) Size() int  { return f.size }

func (f *IntFormatter) WithSize(size int) Formatter {
	return NewInt(size, f.pad, f.sign)
}

func (f *IntFormatter) Format(v Value) string {
	if v.IsNull() {
		return textutil.Pad("", f.width, ' ', true)
	}

	neg, digits, ok := f.digits(v)
	if !ok {
		return hashes(f.width)
	}
	if len(digits) > f.size || (neg && f.sign == "") {
		return hashes(f.width)
	}

	sign := signOf(neg, f.sign)
	if f.pad == ' ' {
		// Space padding precedes the sign.
		return textutil.Pad(sign+digits, f.size+len(sign), ' ', true)
	}
	// Zero padding follows the sign.
	return sign + textutil.Pad(digits, f.size, f.pad, true)
}

// digits returns the sign and decimal magnitude of v. Values are rendered as a
// digit string rather than converted to int64, so magnitudes beyond int64 —
// which Python's arbitrary-precision ints handle natively — still format.
func (f *IntFormatter) digits(v Value) (neg bool, digits string, ok bool) {
	switch v.Kind {
	case KindInt:
		s := strconv.FormatInt(v.I, 10)
		return strings.HasPrefix(s, "-"), strings.TrimPrefix(s, "-"), true
	case KindBool:
		if v.B {
			return false, "1", true
		}
		return false, "0", true
	default:
		x := v.Number()
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false, "", false
		}
		// 'f' with zero precision rounds half-to-even, matching Python's round().
		return x < 0, strconv.FormatFloat(math.Abs(x), 'f', 0, 64), true
	}
}
