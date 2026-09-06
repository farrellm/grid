package format

import "testing"

// Ported verbatim from ngrid's test/formatters.py, class BoolFormatterTest.

func TestBoolDefault(t *testing.T) {
	f := NewBool("True", "False", 0, false)
	checkBool(t, f, []boolCase{
		{"True ", Bool(true)},
		{"False", Bool(false)},
		{"True ", Int(1)},
		{"False", Int(0)},
		{"True ", String("yes")},
		{"False", String("")},
		{"False", Null()},
	})
}

func TestBoolNames(t *testing.T) {
	f := NewBool("yes", "no", 0, false)
	checkBool(t, f, []boolCase{
		{"yes", Bool(true)},
		{"no ", Bool(false)},
	})
}

func TestBoolSize(t *testing.T) {
	f := NewBool("True", "False", 8, false)
	checkBool(t, f, []boolCase{
		{"True    ", Bool(true)},
		{"False   ", Bool(false)},
	})

	f = NewBool("True", "False", 2, false)
	checkBool(t, f, []boolCase{
		{"Tr", Bool(true)},
		{"Fa", Bool(false)},
	})

	f = NewBool("Definitely so", "Absolutely not", 8, false)
	checkBool(t, f, []boolCase{
		{"Definite", Bool(true)},
		{"Absolute", Bool(false)},
	})
}

func TestBoolPadLeft(t *testing.T) {
	f := NewBool("True", "False", 0, true)
	checkBool(t, f, []boolCase{
		{" True", Bool(true)},
		{"False", Bool(false)},
	})

	f = NewBool("True", "False", 12, true)
	checkBool(t, f, []boolCase{
		{"        True", Bool(true)},
		{"       False", Bool(false)},
	})
}

type boolCase struct {
	want string
	in   Value
}

func checkBool(t *testing.T, f Formatter, cases []boolCase) {
	t.Helper()
	for _, c := range cases {
		if got := f.Format(c.in); got != c.want {
			t.Errorf("Format(%+v) = %q, want %q", c.in, got, c.want)
		}
	}
}
