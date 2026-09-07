package format

import "github.com/farrellm/grid/internal/textutil"

// BoolFormatter renders booleans as one of two fixed labels.
type BoolFormatter struct {
	True, False string
	size        int
	PadLeft     bool
}

// NewBool creates a formatter. A size of 0 means "fit the longer label".
func NewBool(t, f string, size int, padLeft bool) *BoolFormatter {
	if size <= 0 {
		size = max(textutil.Width(t), textutil.Width(f))
	}
	return &BoolFormatter{True: t, False: f, size: size, PadLeft: padLeft}
}

func (b *BoolFormatter) Width() int { return b.size }
func (b *BoolFormatter) Size() int  { return b.size }

func (b *BoolFormatter) WithSize(size int) Formatter {
	return NewBool(b.True, b.False, size, b.PadLeft)
}

func (b *BoolFormatter) Format(v Value) string {
	s := b.False
	if v.Truth() {
		s = b.True
	}
	// The ellipsis is empty: labels are cut, not marked as elided.
	return textutil.Palide(s, b.size, "", ' ', 1.0, b.PadLeft)
}
