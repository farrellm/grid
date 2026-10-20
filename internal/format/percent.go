package format

import "github.com/farrellm/grid/internal/textutil"

// PercentFormatter renders fractions as percentages: 0.1234 as "12%".
type PercentFormatter struct {
	f *FloatFormatter
}

// NewPercent creates a formatter whose integral part holds size digits of the
// percentage.
func NewPercent(size, precision int, sign, nanStr, infStr string) *PercentFormatter {
	return &PercentFormatter{f: NewFloat(size, precision, ' ', sign, ".", nanStr, infStr)}
}

func (p *PercentFormatter) Width() int     { return p.f.Width() + 1 }
func (p *PercentFormatter) Size() int      { return p.f.Size() }
func (p *PercentFormatter) Precision() int { return p.f.Precision() }

func (p *PercentFormatter) WithSize(size int) Formatter {
	return &PercentFormatter{f: p.f.WithSize(size).(*FloatFormatter)}
}

func (p *PercentFormatter) WithPrecision(precision int) Formatter {
	return &PercentFormatter{f: p.f.WithPrecision(precision).(*FloatFormatter)}
}

func (p *PercentFormatter) Format(v Value) string {
	if v.IsNull() {
		return textutil.Pad("", p.Width(), ' ', true)
	}
	return p.f.Format(Float(v.Number()*100)) + "%"
}
