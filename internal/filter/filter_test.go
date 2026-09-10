package filter

import (
	"math"
	"testing"
	"time"

	"github.com/farrellm/grid/internal/format"
)

func day(s string) format.Value {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return format.Time(t)
}

func TestMatch(t *testing.T) {
	tests := []struct {
		expr string
		v    format.Value
		want bool
	}{
		// With no operator, a regular expression over the full text.
		{"^g", format.String("gamma"), true},
		{"^g", format.String("alpha"), false},
		{"mm", format.String("gamma"), true},
		{"^1", format.Int(12), true},
		{"^$", format.Null(), true},

		// A leading '!' negates, but "!=" is an operator.
		{"!^g", format.String("gamma"), false},
		{"!^g", format.String("alpha"), true},
		{"!> 3", format.Int(4), false},
		{"!=3", format.Int(4), true},
		{"!=3", format.Int(3), false},

		// Integers.
		{"= 3", format.Int(3), true},
		{"== 3", format.Int(3), true},
		{"!= 3", format.Int(3), false},
		{"< 3", format.Int(2), true},
		{"<= 3", format.Int(3), true},
		{"> 3", format.Int(3), false},
		{">= 3", format.Int(3), true},
		{">3", format.Int(4), true},
		{"> 2.5", format.Int(3), true},
		// Compared exactly, not through a float64 that cannot tell them apart.
		{"= 9007199254740993", format.Int(9007199254740993), true},
		{"= 9007199254740993", format.Int(9007199254740992), false},

		// Floats; NaN is unordered.
		{"> 2.5", format.Float(3.5), true},
		{"> 2.5", format.Float(2.5), false},
		{"= 3", format.Float(3), true},
		{"< 1", format.Float(math.NaN()), false},
		{"!= 1", format.Float(math.NaN()), true},

		// Dates and timestamps.
		{">= 2024-01-05", day("2024-01-05"), true},
		{"> 2024-01-05", day("2024-01-05"), false},
		{"< 2024-01-05T12:00:00", day("2024-01-05"), true},
		{"> 2024-01-04 23:59:59", day("2024-01-05"), true},

		// Booleans, false before true.
		{"= true", format.Bool(true), true},
		{"= TRUE", format.Bool(false), false},
		{"> false", format.Bool(true), true},

		// Strings, compared as written.
		{"= beta", format.String("beta"), true},
		{"= Beta", format.String("beta"), false},
		{"< b", format.String("alpha"), true},
		{"= 3", format.String("3"), true},

		// Quotes keep spaces and force text: as numbers 9 < 10, as text "9" > "10".
		{`= "a b"`, format.String("a b"), true},
		{`= " a"`, format.String(" a"), true},
		{`> "10"`, format.Int(9), true},
		{`= "NA"`, format.Null(), false},

		// A literal that is not the cell's kind compares as text.
		{"= abc", format.Int(3), false},
		{"< abc", format.Int(3), true},

		// Missing values.
		{"=", format.Null(), true},
		{"= NA", format.Null(), true},
		{"= NA", format.Int(1), false},
		{"= NA", format.Float(math.NaN()), true},
		{"!= NA", format.Int(1), true},
		{"!=", format.Null(), false},
		{"!!=", format.Null(), true},

		// A missing cell is unequal to everything and ordered against nothing.
		{"= 1", format.Null(), false},
		{"!= 1", format.Null(), true},
		{"> 1", format.Null(), false},
		{"< 1", format.Null(), false},
	}

	for _, tt := range tests {
		p, err := Parse(tt.expr)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.expr, err)
			continue
		}
		if got := p.Match(tt.v); got != tt.want {
			t.Errorf("%q on %+v = %v, want %v", tt.expr, tt.v, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, expr := range []string{
		"[",    // a bad regular expression
		"![",   // negated
		">",    // an ordering needs a value
		"< NA", // and a missing one will not do
	} {
		if _, err := Parse(expr); err == nil {
			t.Errorf("Parse(%q) succeeded, want an error", expr)
		}
	}
}

func TestString(t *testing.T) {
	for expr, want := range map[string]string{
		"^g":      "/^g/",
		"  ^g ":   "/^g/",
		"> 2.5":   ">2.5",
		"!= NA":   "!=NA",
		"!^a":     "!/^a/",
		`= "a b"`: `="a b"`,
	} {
		p, err := Parse(expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", expr, err)
		}
		if got := p.String(); got != want {
			t.Errorf("Parse(%q).String() = %q, want %q", expr, got, want)
		}
	}
}
