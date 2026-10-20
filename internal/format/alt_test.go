package format

import (
	"testing"
	"time"
)

// alt finds the alternate with the given ID among those offered for orig.
func alt(t *testing.T, orig Formatter, id string) Formatter {
	t.Helper()
	for _, a := range AltsFor(orig) {
		if a.ID == id {
			return a.Make(orig, DefaultConfig())
		}
	}
	t.Fatalf("no %q alternate for %T", id, orig)
	return nil
}

func TestEpochUTC(t *testing.T) {
	f := alt(t, NewInt(10, ' ', "-"), "utc")
	checkFmt(t, f, []fmtCase{
		{"2022-12-31T23:59:15Z", Int(1672531155)},
		{"1970-01-01T00:00:00Z", Int(0)},
		{"1969-12-31T23:59:59Z", Int(-1)},
		{"                 NaN", Null()},
	})
}

// The unit is read from how many digits the column holds.
func TestEpochUnits(t *testing.T) {
	cases := []struct {
		size int
		in   int64
	}{
		{13, 1672531155123},
		{16, 1672531155123456},
		{19, 1672531155123456789},
	}
	for _, c := range cases {
		f := alt(t, NewInt(c.size, ' ', "-"), "utc")
		if got := f.Format(Int(c.in)); got != "2022-12-31T23:59:15Z" {
			t.Errorf("%d digits: Format(%d) = %q", c.size, c.in, got)
		}
	}
}

func TestEpochFloat(t *testing.T) {
	f := alt(t, NewFloat(10, 1, ' ', "", ".", "NaN", "∞"), "utc")
	checkFmt(t, f, []fmtCase{
		{"2022-12-31T23:59:15Z", Float(1672531155.5)},
		{"                 NaN", nan},
	})
	// An EFloat does not say how many digits it holds, so each value decides.
	f = alt(t, NewEFloat(2, 3, "", ".", "E", "NaN", "∞"), "utc")
	if got := f.Format(Float(1672531155123)); got != "2022-12-31T23:59:15Z" {
		t.Errorf("Format(ms) = %q", got)
	}
}

func TestEpochLocal(t *testing.T) {
	saved := time.Local
	time.Local = time.FixedZone("EST", -5*3600)
	t.Cleanup(func() { time.Local = saved })

	f := alt(t, NewInt(10, ' ', "-"), "local")
	checkFmt(t, f, []fmtCase{{"2022-12-31 18:59:15", Int(1672531155)}})
}

func TestPercent(t *testing.T) {
	f := alt(t, NewFloat(1, 4, ' ', "", ".", "NaN", "∞"), "percent")
	checkFmt(t, f, []fmtCase{
		{" 12%", Float(0.1234)},
		{"100%", Float(1)},
		{"  0%", Float(0)},
		{"    ", Null()},
	})
	p := f.(Precise).WithPrecision(1)
	checkFmt(t, p, []fmtCase{{" 12.3%", Float(0.1234)}})
}

func TestScientific(t *testing.T) {
	f := alt(t, NewInt(10, ' ', "-"), "sci")
	checkFmt(t, f, []fmtCase{
		{" 1.673e+09", Int(1672531155)},
		{"-1.000e-02", Float(-0.01)},
	})
}

func TestThousands(t *testing.T) {
	f := alt(t, NewInt(7, ' ', "-"), "thousands")
	checkFmt(t, f, []fmtCase{
		{" 1,234,567", Int(1234567)},
		{"-1,234,567", Int(-1234567)},
		{"        12", Int(12)},
		{"   123,456", Int(123456)},
		{"##########", Int(12345678)},
		{"          ", Null()},
	})
	f = alt(t, NewFloat(4, 2, ' ', "-", ".", "NaN", "∞"), "thousands")
	checkFmt(t, f, []fmtCase{{" 1,234.50", Float(1234.5)}})
}

func TestTimeAlts(t *testing.T) {
	orig := NewTime("ISO 8601 extended", "NaN")
	in := Time(time.Date(2023, 1, 1, 9, 30, 0, 0, time.FixedZone("JST", 9*3600)))
	checkFmt(t, alt(t, orig, "utc"), []fmtCase{{"2023-01-01T00:30:00Z", in}})
	checkFmt(t, alt(t, orig, "date"), []fmtCase{{"2023-01-01", in}})
}

func TestNoAltsForText(t *testing.T) {
	if alts := AltsFor(NewStr(8, "…", ' ', 1.0, false)); alts != nil {
		t.Errorf("AltsFor(str) = %v, want none", alts)
	}
}

// A timestamp widens like any other time: on the right.
func TestWidenEpoch(t *testing.T) {
	f := Widen(alt(t, NewInt(10, ' ', "-"), "utc"), 24)
	if got := f.Format(Int(0)); got != "1970-01-01T00:00:00Z    " {
		t.Errorf("widened = %q", got)
	}
}
