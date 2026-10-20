package format

import "time"

// Alt is an alternate rendering the 't' menu offers for a column.
type Alt struct {
	ID   string // stable key a column's choice is remembered by
	Name string // menu text
	Make func(orig Formatter, cfg Config) Formatter
}

var numberAlts = []Alt{
	{"utc", "UTC datetime", func(orig Formatter, cfg Config) Formatter {
		return NewEpoch(orig, NewTimeIn(cfg.TimeFormat, cfg.NaNString, time.UTC))
	}},
	{"local", "Local datetime", func(orig Formatter, cfg Config) Formatter {
		return NewEpoch(orig, NewTimeIn("simple", cfg.NaNString, time.Local))
	}},
	{"percent", "Percentage, rounded", func(orig Formatter, cfg Config) Formatter {
		// Two more digits than the fraction had, for the factor of 100.
		return NewPercent(integralDigits(orig)+2, NoPrecision, numberSign(orig),
			cfg.NaNString, cfg.InfString)
	}},
	{"sci", "Scientific notation", func(orig Formatter, cfg Config) Formatter {
		return NewEFloat(2, 3, numberSign(orig), ".", "e", cfg.NaNString, cfg.InfString)
	}},
	{"thousands", "Thousands separators", func(orig Formatter, _ Config) Formatter {
		prec := NoPrecision
		if p, ok := orig.(*FloatFormatter); ok {
			prec = p.precision
		}
		return NewGrouped(integralDigits(orig), prec, numberSign(orig))
	}},
}

var timeAlts = []Alt{
	{"utc", "UTC datetime", func(_ Formatter, cfg Config) Formatter {
		return NewTimeIn(cfg.TimeFormat, cfg.NaNString, time.UTC)
	}},
	{"local", "Local datetime", func(_ Formatter, cfg Config) Formatter {
		return NewTimeIn("simple", cfg.NaNString, time.Local)
	}},
	{"date", "Date only", func(_ Formatter, cfg Config) Formatter {
		return NewTime("date", cfg.NaNString)
	}},
}

// AltsFor lists the alternates that suit a column drawn with f, or nil if
// there are none.
func AltsFor(f Formatter) []Alt {
	switch f.(type) {
	case *IntFormatter, *FloatFormatter, *EFloatFormatter:
		return numberAlts
	case *TimeFormatter:
		return timeAlts
	}
	return nil
}

// integralDigits is how many integral digits a number formatter holds. An
// EFloat's size counts its exponent's digits instead; it is chosen for values
// past FixedRangeMax, so it gets room for those.
func integralDigits(f Formatter) int {
	switch f := f.(type) {
	case *IntFormatter:
		return f.size
	case *FloatFormatter:
		return f.size
	case *EFloatFormatter:
		return 15
	}
	return 1
}

// numberSign is the sign mode a number formatter renders with.
func numberSign(f Formatter) string {
	switch f := f.(type) {
	case *IntFormatter:
		return f.sign
	case *FloatFormatter:
		return f.sign
	case *EFloatFormatter:
		return f.sign
	}
	return "-"
}
