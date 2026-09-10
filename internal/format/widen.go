package format

import "github.com/farrellm/grid/internal/textutil"

// Widen returns f rendering into at least width display columns, the extra
// space going where that formatter's own padding would put it. It backs the
// 'w' key, which widens every column to show its full name.
//
// WithSize is not the general answer: it sizes a different part of the value
// for each type -- the exponent of an EFloat, the integral digits of a Float,
// which may be zero-padded -- so only the formatters whose size is their width
// are widened through it.
func Widen(f Formatter, width int) Formatter {
	if f.Width() >= width {
		return f
	}
	switch f := f.(type) {
	case *StrFormatter:
		return f.WithSize(width)
	case *BoolFormatter:
		return f.WithSize(width)
	case *TimeFormatter:
		return padded{Formatter: f, width: width}
	}
	// Numbers are right-aligned.
	return padded{Formatter: f, width: width, left: true}
}

// padded pads another formatter's output out to a wider column.
type padded struct {
	Formatter
	width int
	left  bool
}

func (p padded) Width() int { return p.width }

func (p padded) Format(v Value) string {
	return textutil.Pad(p.Formatter.Format(v), p.width, ' ', p.left)
}
