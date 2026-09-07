package format

import (
	"strconv"
	"time"

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
		f.text(v), f.size,
		textutil.Elide(f.ellipsis, f.size, "", 1.0),
		f.pad, f.position, f.padLeft)
}

// text renders any value kind as a string, so a string column can still show
// values that failed to parse as something narrower.
func (f *StrFormatter) text(v Value) string {
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
