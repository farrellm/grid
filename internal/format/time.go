package format

import (
	"time"

	"github.com/farrellm/grid/internal/textutil"
)

// Named layouts carried over from ngrid's DATETIME_FORMATS, translated to Go
// reference-time layouts.
var timeLayouts = map[string]string{
	"simple":            "2006-01-02 15:04:05",
	"ISO 8601 extended": "2006-01-02T15:04:05Z",
	"ISO 8601":          "20060102T150405Z",
	"date":              "2006-01-02",
	"time":              "15:04:05",
}

// TimeFormatter renders timestamps with a fixed layout.
type TimeFormatter struct {
	layout  string
	loc     *time.Location // nil renders each time in its own zone
	nullStr string
	width   int
}

// NewTime creates a formatter. The spec may be a name from timeLayouts or a Go
// layout string.
func NewTime(spec, nullStr string) *TimeFormatter {
	return NewTimeIn(spec, nullStr, nil)
}

// NewTimeIn is NewTime rendering every time in loc.
func NewTimeIn(spec, nullStr string, loc *time.Location) *TimeFormatter {
	layout, ok := timeLayouts[spec]
	if !ok {
		layout = spec
	}
	// Measure with a reference instant whose fields are all two digits, so the
	// width is the true rendered width.
	ref := time.Date(2014, 9, 17, 10, 7, 53, 123456000, time.UTC)
	width := textutil.Width(ref.Format(layout))
	if w := textutil.Width(nullStr); w > width {
		nullStr = textutil.Elide(nullStr, width, "", 1.0)
	}
	return &TimeFormatter{layout: layout, loc: loc, nullStr: nullStr, width: width}
}

func (f *TimeFormatter) Width() int { return f.width }

func (f *TimeFormatter) Format(v Value) string {
	if v.Kind != KindTime {
		return textutil.Pad(f.nullStr, f.width, ' ', true)
	}
	t := v.T
	if f.loc != nil {
		t = t.In(f.loc)
	}
	return textutil.Palide(t.Format(f.layout), f.width, "", ' ', 1.0, false)
}
