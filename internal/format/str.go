package format

import (
	"github.com/farrellm/grid/internal/textutil"
)

// StrFormatter renders strings elided and padded to a fixed width.
type StrFormatter struct {
	size     int
	ellipsis string
	pad      rune
	position float64
	padLeft  bool
}

// NewStr creates a formatter that fits strings into size display columns.
func NewStr(size int, ellipsis string, pad rune, position float64, padLeft bool) *StrFormatter {
	return &StrFormatter{
		size: size, ellipsis: ellipsis, pad: pad, position: position, padLeft: padLeft,
	}
}

func (f *StrFormatter) Width() int { return f.size }
func (f *StrFormatter) Size() int  { return f.size }

func (f *StrFormatter) WithSize(size int) Formatter {
	return NewStr(size, f.ellipsis, f.pad, f.position, f.padLeft)
}

func (f *StrFormatter) Format(v Value) string {
	return textutil.Palide(
		Text(v), f.size,
		textutil.Elide(f.ellipsis, f.size, "", 1.0),
		f.pad, f.position, f.padLeft)
}
