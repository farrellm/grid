package source

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/farrellm/grid/internal/format"
)

func open(t *testing.T, data string, mut ...func(*CSVOptions)) *CSV {
	t.Helper()
	opts := CSVOptions{
		Filename:  "test.csv",
		HasHeader: true,
		Config:    format.DefaultConfig(),
	}
	for _, m := range mut {
		m(&opts)
	}
	c, err := OpenCSV(strings.NewReader(data), opts)
	if err != nil {
		t.Fatalf("OpenCSV: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// waitAll loads every row and waits for the reader to finish.
func waitAll(t *testing.T, c *CSV) {
	t.Helper()
	c.RequestAll()
	select {
	case <-c.finished:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out loading")
	}
	if err := c.Err(); err != nil {
		t.Fatalf("load error: %v", err)
	}
}

func TestCSVBasics(t *testing.T) {
	c := open(t, "n,x,flag,word\n1,1.5,true,abc\n2,2.5,false,def\n")
	waitAll(t, c)

	if got := c.NumRows(); got != 2 {
		t.Errorf("NumRows() = %d, want 2", got)
	}
	if got := c.NumCols(); got != 4 {
		t.Errorf("NumCols() = %d, want 4", got)
	}
	if got := strings.Join(c.Names(), ","); got != "n,x,flag,word" {
		t.Errorf("Names() = %q", got)
	}
	if v := c.Value(0, 0); v.Kind != format.KindInt || v.I != 1 {
		t.Errorf("Value(0,0) = %+v, want int 1", v)
	}
	if v := c.Value(1, 3); v.Kind != format.KindString || v.S != "def" {
		t.Errorf("Value(1,3) = %+v, want string def", v)
	}
	if !c.Done() {
		t.Error("Done() = false after loading everything")
	}
}

func TestCSVNoHeader(t *testing.T) {
	c := open(t, "1,2\n3,4\n", func(o *CSVOptions) { o.HasHeader = false })
	waitAll(t, c)

	if got := strings.Join(c.Names(), ","); got != "col1,col2" {
		t.Errorf("Names() = %q, want col1,col2", got)
	}
	if got := c.NumRows(); got != 2 {
		t.Errorf("NumRows() = %d, want 2", got)
	}
}

// The heart of the Arrow workaround: a value past the sample that the sample
// did not predict must widen its column rather than fail the read.
func TestCSVPromotesLateFloat(t *testing.T) {
	var b strings.Builder
	b.WriteString("n\n")
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&b, "%d\n", i)
	}
	b.WriteString("1.5\n")

	c := open(t, b.String(), func(o *CSVOptions) { o.SampleSize = 100 })
	waitAll(t, c)

	if got := c.NumRows(); got != 301 {
		t.Fatalf("NumRows() = %d, want 301", got)
	}
	// Early rows keep the narrow type they were built with; the late one is a
	// float. Both must render, which is what matters to the viewer.
	if v := c.Value(0, 0); v.Kind != format.KindInt {
		t.Errorf("Value(0,0) kind = %v, want int", v.Kind)
	}
	if v := c.Value(300, 0); v.Number() != 1.5 {
		t.Errorf("Value(300,0) = %+v, want 1.5", v)
	}
}

func TestCSVPromotesLateString(t *testing.T) {
	var b strings.Builder
	b.WriteString("n\n")
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&b, "%d\n", i)
	}
	b.WriteString("oops\n")

	c := open(t, b.String(), func(o *CSVOptions) { o.SampleSize = 100 })
	waitAll(t, c)

	if got := c.NumRows(); got != 301 {
		t.Fatalf("NumRows() = %d, want 301", got)
	}
	if v := c.Value(300, 0); v.Kind != format.KindString || v.S != "oops" {
		t.Errorf("Value(300,0) = %+v, want string oops", v)
	}
}

// Blanks and sentinels become nulls instead of forcing the column wider, which
// ngrid could not do: it converted "" to NaN and made the column a float.
func TestCSVNullsKeepColumnNarrow(t *testing.T) {
	c := open(t, "n\n1\n\n3\nNA\n5\n")
	waitAll(t, c)

	if got := c.NumRows(); got != 4 {
		t.Fatalf("NumRows() = %d, want 4 (blank lines are skipped)", got)
	}
	for _, row := range []int{0, 1, 3} {
		if v := c.Value(row, 0); v.Kind != format.KindInt {
			t.Errorf("Value(%d,0) kind = %v, want int", row, v.Kind)
		}
	}
	if v := c.Value(2, 0); !v.IsNull() {
		t.Errorf("Value(2,0) = %+v, want null for \"NA\"", v)
	}
}

func TestCSVSpacesAroundNumbers(t *testing.T) {
	c := open(t, "a, b\n1, 2\n3, 4\n")
	waitAll(t, c)

	if v := c.Value(0, 1); v.Kind != format.KindInt || v.I != 2 {
		t.Errorf("Value(0,1) = %+v, want int 2 (spaces must be trimmed)", v)
	}
}

func TestCSVComments(t *testing.T) {
	c := open(t, "# a title\n# more\nn,x\n1,2\n", func(o *CSVOptions) {
		o.CommentPrefix = "#"
	})
	waitAll(t, c)

	if got := len(c.TitleLines()); got != 2 {
		t.Errorf("TitleLines() has %d entries, want 2", got)
	}
	if got := strings.Join(c.Names(), ","); got != "n,x" {
		t.Errorf("Names() = %q, want n,x", got)
	}
}

func TestCSVDelimiterSniffing(t *testing.T) {
	for _, tc := range []struct{ name, data string }{
		{"tab", "a\tb\n1\t2\n"},
		{"pipe", "a|b\n1|2\n"},
		{"semicolonExplicit", "a;b\n1;2\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mut []func(*CSVOptions)
			if tc.name == "semicolonExplicit" {
				mut = append(mut, func(o *CSVOptions) { o.Delimiter = ';' })
			}
			c := open(t, tc.data, mut...)
			waitAll(t, c)
			if c.NumCols() != 2 {
				t.Errorf("NumCols() = %d, want 2", c.NumCols())
			}
			if v := c.Value(0, 1); v.I != 2 {
				t.Errorf("Value(0,1) = %+v, want 2", v)
			}
		})
	}
}

// Rows must be visible before the whole stream has been read, which is what
// lets a huge file open instantly.
func TestCSVLoadsIncrementally(t *testing.T) {
	var b strings.Builder
	b.WriteString("n\n")
	for i := 0; i < 100000; i++ {
		fmt.Fprintf(&b, "%d\n", i)
	}

	c := open(t, b.String(), func(o *CSVOptions) { o.SampleSize = 10 })
	if c.Done() {
		t.Error("Done() = true immediately; the stream should still be loading")
	}
	if c.NumRows() < 10 {
		t.Errorf("NumRows() = %d, want at least the sample", c.NumRows())
	}
	if c.NumRows() >= 100000 {
		t.Errorf("NumRows() = %d; the whole file should not be read up front", c.NumRows())
	}

	waitAll(t, c)
	if got := c.NumRows(); got != 100000 {
		t.Errorf("NumRows() = %d, want 100000", got)
	}
}

func TestCSVFullMode(t *testing.T) {
	var b strings.Builder
	b.WriteString("n\n")
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&b, "%d\n", i)
	}

	c := open(t, b.String(), func(o *CSVOptions) { o.Full = true })
	if !c.Done() {
		t.Error("Done() = false; --full must load everything before returning")
	}
	if got := c.NumRows(); got != 5000 {
		t.Errorf("NumRows() = %d, want 5000", got)
	}
	// Having seen every value, the width fits the largest, not the sample's.
	if w := c.Formatters()[0].Width(); w != 5 {
		t.Errorf("width = %d, want 5 (4 digits plus sign)", w)
	}
}

func TestCSVEmptyInput(t *testing.T) {
	if _, err := OpenCSV(strings.NewReader(""), CSVOptions{HasHeader: true}); err == nil {
		t.Error("OpenCSV on empty input: want an error")
	}
}

func TestCSVQuotedNewlines(t *testing.T) {
	c := open(t, "a,b\n1,\"line one\nline two\"\n2,plain\n")
	waitAll(t, c)

	if got := c.NumRows(); got != 2 {
		t.Fatalf("NumRows() = %d, want 2", got)
	}
	if v := c.Value(0, 1); !strings.Contains(v.S, "line two") {
		t.Errorf("Value(0,1) = %q, want the embedded newline preserved", v.S)
	}
}

// The fixture in testdata exercises the promotion path end to end: a comment
// header, an integer column that meets a float at row 400, another that meets
// text at row 450, and blanks that must stay null throughout.
func TestCSVWideningFixture(t *testing.T) {
	f, err := os.Open("../../testdata/widening.csv")
	if err != nil {
		t.Skipf("fixture not present: %v", err)
	}
	defer f.Close()

	c, err := OpenCSV(f, CSVOptions{
		Filename:      "widening.csv",
		HasHeader:     true,
		SampleSize:    100,
		CommentPrefix: "#",
		Config:        format.DefaultConfig(),
	})
	if err != nil {
		t.Fatalf("OpenCSV: %v", err)
	}
	defer c.Close()
	waitAll(t, c)

	if got := c.NumRows(); got != 500 {
		t.Fatalf("NumRows() = %d, want 500", got)
	}
	if got := len(c.TitleLines()); got != 2 {
		t.Errorf("TitleLines() = %d, want 2", got)
	}

	// The value that forced the "mixed" column to widen.
	if v := c.Value(400, 1); v.Number() != 1.5 {
		t.Errorf("Value(400,1) = %+v, want 1.5", v)
	}
	// And the one that forced "late" all the way to text.
	if v := c.Value(450, 2); v.Kind != format.KindString || v.S != "oops" {
		t.Errorf("Value(450,2) = %+v, want string oops", v)
	}
	// Blanks stay missing rather than widening anything.
	if v := c.Value(0, 2); !v.IsNull() {
		t.Errorf("Value(0,2) = %+v, want null", v)
	}
	// Promotion rebuilds the batch it happened in, so types are uniform within
	// a batch but may differ between them. Row 50 is in the sample batch, which
	// never saw a failing value and so kept the narrow type; row 300 shares a
	// batch with the "oops" that widened the column to text. Both render, which
	// is what the viewer needs.
	if v := c.Value(50, 1); v.Kind != format.KindInt || v.I != 50 {
		t.Errorf("Value(50,1) = %+v, want int 50", v)
	}
	if v := c.Value(300, 2); v.Kind != format.KindString || v.S != "300" {
		t.Errorf("Value(300,2) = %+v, want string \"300\"", v)
	}

	// The formatters follow the widened types, so the column renders sensibly
	// rather than showing nulls for the values that forced the widening.
	if _, ok := c.Formatters()[1].(*format.FloatFormatter); !ok {
		t.Errorf("mixed formatter = %T, want *format.FloatFormatter", c.Formatters()[1])
	}
	if _, ok := c.Formatters()[2].(*format.StrFormatter); !ok {
		t.Errorf("late formatter = %T, want *format.StrFormatter", c.Formatters()[2])
	}
}

// The view reads cells on the UI goroutine while the reader appends batches on
// another. This is the access pattern that matters for -race.
func TestCSVConcurrentReadWhileLoading(t *testing.T) {
	var b strings.Builder
	b.WriteString("n,word\n")
	for i := 0; i < 200000; i++ {
		fmt.Fprintf(&b, "%d,w%d\n", i, i)
	}

	c := open(t, b.String(), func(o *CSVOptions) { o.SampleSize = 10 })

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Poll the way a redraw does: ask for a window of rows, read them all.
		for i := 0; i < 3000; i++ {
			n := c.NumRows()
			c.Request(n + 50)
			for r := max(n-30, 0); r < n; r++ {
				for col := 0; col < c.NumCols(); col++ {
					_ = c.Value(r, col)
				}
			}
			for _, f := range c.Formatters() {
				_ = f.Width()
			}
			if c.Done() {
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("timed out")
	}
	if err := c.Err(); err != nil {
		t.Fatalf("load error: %v", err)
	}
}

// Closing while the reader is mid-stream must not hang or race.
func TestCSVCloseWhileLoading(t *testing.T) {
	var b strings.Builder
	b.WriteString("n\n")
	for i := 0; i < 200000; i++ {
		fmt.Fprintf(&b, "%d\n", i)
	}

	c, err := OpenCSV(strings.NewReader(b.String()), CSVOptions{
		Filename:   "big.csv",
		HasHeader:  true,
		SampleSize: 10,
		Config:     format.DefaultConfig(),
	})
	if err != nil {
		t.Fatal(err)
	}
	c.RequestAll()

	closed := make(chan error, 1)
	go func() { closed <- c.Close() }()

	select {
	case err := <-closed:
		if err != nil {
			t.Errorf("Close() = %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Close() hung while the reader was running")
	}
}
