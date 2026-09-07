// Package format renders typed cell values as fixed-width strings.
//
// It is a port of ngrid's formatters.py. The rendering rules — how many digits,
// where the sign sits relative to the padding, when a value is replaced by '#'
// fill — are reproduced exactly; test/formatters.py from the Python project is
// ported alongside as the oracle.
package format

import "strings"

// Formatter renders a Value into exactly Width() display columns.
type Formatter interface {
	Width() int
	Format(Value) string
}

// Resizable is implemented by formatters whose integral width can be adjusted
// interactively (the ',' and '.' keys).
type Resizable interface {
	Formatter
	Size() int
	WithSize(int) Formatter
}

// Precise is implemented by formatters whose fractional precision can be
// adjusted interactively (the '<' and '>' keys). A precision of -1 means
// "suppressed", matching Python's None.
type Precise interface {
	Formatter
	Precision() int
	WithPrecision(int) Formatter
}

// NoPrecision is the sentinel for a suppressed decimal point, standing in for
// Python's None precision.
const NoPrecision = -1

// hashes returns the '#' fill used when a value does not fit its width.
func hashes(width int) string { return strings.Repeat("#", width) }

// signOf returns the sign prefix for a value under the given sign mode, which
// is "-", "+", or "" (meaning: render no sign, and reject negatives).
func signOf(neg bool, sign string) string {
	switch {
	case sign == "":
		return ""
	case neg:
		return "-"
	case sign == "+":
		return "+"
	default:
		return " "
	}
}

// truncateBytes cuts s to at most n bytes, as Python's slicing does for the NaN
// and infinity placeholders. It is bytes rather than display columns on purpose,
// which is why it is not one of textutil's helpers.
func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 0 {
		return ""
	}
	return s[:n]
}
