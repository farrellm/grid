package format

import "testing"

// Ported verbatim from ngrid's test/formatters.py, class IntFormatterTest.

func TestIntDefault(t *testing.T) {
	f := NewInt(4, ' ', "-")
	checkFmt(t, f, []fmtCase{
		{"    0", Int(0)},
		{"    1", Int(1)},
		{" 9999", Int(9999)},
		{"#####", Int(10000)},
		{"   -1", Int(-1)},
		{"  -10", Int(-10)},
		{"-9999", Int(-9999)},
		{"#####", Int(-10000)},

		{"    1", Bool(true)},
		{"    0", Bool(false)},

		{" 1000", Float(1.0e+3)},
		{"  999", Float(999.499)},
		{" 1000", Float(999.5)},
		{" -999", Float(-999.499)},
		{"-1000", Float(-999.5)},
	})
}

func TestIntSize(t *testing.T) {
	f := NewInt(1, ' ', "-")
	checkFmt(t, f, []fmtCase{
		{" 4", Int(4)},
		{"-6", Int(-6)},
		{"##", Int(-10)},
	})

	f = NewInt(20, ' ', "-")
	checkFmt(t, f, []fmtCase{
		{"                    0", Int(0)},
		{"                   -1", Int(-1)},
		{" 10000000000000000000", Float(1e+19)},
		{"-10000000000000000000", Float(-1e+19)},
	})
}

func TestIntPad(t *testing.T) {
	f := NewInt(4, ' ', "-")
	checkFmt(t, f, []fmtCase{
		{"    0", Int(0)},
		{"    1", Int(1)},
		{"   -1", Int(-1)},
		{"  999", Int(999)},
		{"-9999", Int(-9999)},
		{"#####", Int(10000)},
	})

	f = NewInt(4, '0', "-")
	checkFmt(t, f, []fmtCase{
		{" 0000", Int(0)},
		{" 0001", Int(1)},
		{"-0001", Int(-1)},
		{" 0999", Int(999)},
		{"-9999", Int(-9999)},
		{"#####", Int(10000)},
	})
}

func TestIntSign(t *testing.T) {
	f := NewInt(4, ' ', "-")
	if f.Width() != 5 {
		t.Errorf("Width() = %d, want 5", f.Width())
	}
	checkFmt(t, f, []fmtCase{
		{"    0", Int(0)},
		{"   -1", Int(-1)},
		{" 1000", Int(1000)},
		{"-1000", Int(-1000)},
		{"#####", Int(10000)},
	})

	f = NewInt(4, ' ', "+")
	if f.Width() != 5 {
		t.Errorf("Width() = %d, want 5", f.Width())
	}
	checkFmt(t, f, []fmtCase{
		{"   +0", Int(0)},
		{"   -1", Int(-1)},
		{"+1000", Int(1000)},
		{"-1000", Int(-1000)},
		{"#####", Int(10000)},
	})

	f = NewInt(4, ' ', "")
	if f.Width() != 4 {
		t.Errorf("Width() = %d, want 4", f.Width())
	}
	checkFmt(t, f, []fmtCase{
		{"   0", Int(0)},
		{"   1", Int(1)},
		{"####", Int(-1)},
		{"1000", Int(1000)},
		{"####", Int(-1000)},
		{"####", Int(10000)},
	})
}

type fmtCase struct {
	want string
	in   Value
}

func checkFmt(t *testing.T, f Formatter, cases []fmtCase) {
	t.Helper()
	for _, c := range cases {
		got := f.Format(c.in)
		if got != c.want {
			t.Errorf("Format(%+v) = %q, want %q", c.in, got, c.want)
		}
		if w := len([]rune(got)); w != f.Width() {
			t.Errorf("Format(%+v) = %q has width %d, want %d", c.in, got, w, f.Width())
		}
	}
}
