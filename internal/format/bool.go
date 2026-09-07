package format

import "github.com/farrellm/grid/internal/textutil"

// BoolFormatter renders booleans as one of two fixed labels.
type BoolFormatter struct {
	trueStr, falseStr string
	size              int
	padLeft           bool
}

// NewBool creates a formatter. A size of 0 means "fit the longer label".
func NewBool(t, f string, size int, padLeft bool) *BoolFormatter {
	if size <= 0 {
		size = max(textutil.Width(t), textutil.Width(f))
	}
	return &BoolFormatter{trueStr: t, falseStr: f, size: size, padLeft: padLeft}
}

func (b *BoolFormatter) Width() int { return b.size }
func (b *BoolFormatter) Size() int  { return b.size }

func (b *BoolFormatter) WithSize(size int) Formatter {
	return NewBool(b.trueStr, b.falseStr, size, b.padLeft)
}

func (b *BoolFormatter) Format(v Value) string {
	s := b.falseStr
	if v.Truth() {
		s = b.trueStr
	}
	// The ellipsis is empty: labels are cut, not marked as elided.
	return textutil.Palide(s, b.size, "", ' ', 1.0, b.padLeft)
}
