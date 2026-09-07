package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/farrellm/grid/internal/format"
	"github.com/farrellm/grid/internal/source"
)

// newModel builds a model over real CSV data at a fixed terminal size.
func newModel(t *testing.T, data string, w, h int) *Model {
	t.Helper()

	src, err := source.OpenCSV(strings.NewReader(data), source.CSVOptions{
		Filename:  "t.csv",
		HasHeader: true,
		Config:    format.DefaultConfig(),
	})
	if err != nil {
		t.Fatalf("OpenCSV: %v", err)
	}
	t.Cleanup(func() { src.Close() })

	src.RequestAll()
	select {
	case <-src.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("timed out loading")
	}
	for !src.Done() {
		select {
		case <-src.Ready():
		case <-time.After(5 * time.Second):
			t.Fatal("timed out loading")
		}
	}

	m := New(src, format.DefaultConfig(), 1)
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.refreshFormatters()
	return m
}

// lines renders the frame with styling stripped, so assertions read as the
// user sees the screen.
func lines(m *Model) []string {
	return strings.Split(ansi.Strip(m.View().Content), "\n")
}

func press(m *Model, keys ...string) {
	for _, k := range keys {
		m.Update(tea.KeyPressMsg{Code: keyCode(k), Text: k})
	}
}

// keyCode maps a single-character key to the rune Bubble Tea reports.
func keyCode(k string) rune {
	r := []rune(k)
	return r[0]
}

const sample = "n,word,x\n1,alpha,1.5\n2,beta,2.5\n3,gamma,3.5\n4,delta,4.5\n"

func TestViewRendersHeaderAndRows(t *testing.T) {
	m := newModel(t, sample, 40, 8)
	got := lines(m)

	if len(got) == 0 {
		t.Fatal("no output")
	}
	if !strings.Contains(got[0], "n") || !strings.Contains(got[0], "word") {
		t.Errorf("header = %q, want the column names", got[0])
	}
	if !strings.Contains(got[1], "alpha") {
		t.Errorf("first row = %q, want alpha", got[1])
	}
	if !strings.Contains(got[1], "1.5") {
		t.Errorf("first row = %q, want the float rendered", got[1])
	}
}

// Every line must fit the terminal, or the display wraps and the grid breaks up.
func TestViewNeverExceedsWidth(t *testing.T) {
	wide := "a,b,c,d,e,f,g,h\n" +
		strings.Repeat("averyverylongvalueindeed,", 7) + "averyverylongvalueindeed\n"

	for _, w := range []int{20, 40, 80, 200} {
		m := newModel(t, wide, w, 10)
		for i, l := range lines(m) {
			if got := ansi.StringWidth(l); got > w {
				t.Errorf("width %d: line %d is %d columns: %q", w, i, got, l)
			}
		}
	}
}

func TestFooterShowsPositionAndProgress(t *testing.T) {
	m := newModel(t, sample, 60, 8)
	got := lines(m)
	footer := got[len(got)-1]

	if !strings.Contains(footer, "t.csv") {
		t.Errorf("footer = %q, want the filename", footer)
	}
	if !strings.Contains(footer, "/4") {
		t.Errorf("footer = %q, want the row count", footer)
	}
	// A fully loaded source shows a percentage, not the '+' of a partial read.
	if !strings.Contains(footer, "%") || strings.Contains(footer, "+") {
		t.Errorf("footer = %q, want a percentage and no '+'", footer)
	}
}

func TestToggleHeaderAndFooter(t *testing.T) {
	m := newModel(t, sample, 40, 8)

	press(m, "H")
	if first := lines(m)[0]; strings.Contains(first, "word") {
		t.Errorf("after H the header is still shown: %q", first)
	}

	press(m, "H", "F")
	last := lines(m)[len(lines(m))-1]
	if strings.Contains(last, "t.csv") {
		t.Errorf("after F the footer is still shown: %q", last)
	}
}

func TestCursorMovementAndCellReadout(t *testing.T) {
	m := newModel(t, sample, 60, 8)

	press(m, "~") // show cursor
	if !m.showCursor {
		t.Fatal("~ did not enable the cursor")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})

	if m.cursor[0] != 1 {
		t.Errorf("cursor row = %d, want 1", m.cursor[0])
	}
	if m.cursor[1] != 1 {
		t.Errorf("cursor column = %d, want 1", m.cursor[1])
	}
	// The footer echoes the selected cell, since the grid may have elided it.
	if footer := lines(m)[len(lines(m))-1]; !strings.Contains(footer, "beta") {
		t.Errorf("footer = %q, want the selected cell 'beta'", footer)
	}
}

func TestWidthAndPrecisionKeys(t *testing.T) {
	m := newModel(t, sample, 60, 8)
	press(m, "~")
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // column x, a float

	f, ok := m.fmts[2].(format.Precise)
	if !ok {
		t.Fatalf("column x formatter is %T, want one supporting precision", m.fmts[2])
	}
	before := f.Precision()

	press(m, ">")
	if got := m.fmts[2].(format.Precise).Precision(); got != before+1 {
		t.Errorf("after '>' precision = %d, want %d", got, before+1)
	}
	press(m, "<", "<")
	if got := m.fmts[2].(format.Precise).Precision(); got != before-1 {
		t.Errorf("after '<<' precision = %d, want %d", got, before-1)
	}

}

// ',' narrows and '.' widens, which is what the help now says; ngrid's help
// claimed the reverse of what its own code did.
func TestWidthKeys(t *testing.T) {
	// Values wide enough that narrowing is not already at the floor.
	m := newModel(t, "n,big\n1,123456\n2,234567\n", 60, 8)
	press(m, "~")
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})

	before := m.fmts[1].Width()
	press(m, ",")
	narrowed := m.fmts[1].Width()
	if narrowed >= before {
		t.Errorf("',' gave width %d, want less than %d", narrowed, before)
	}

	press(m, ".", ".")
	if got := m.fmts[1].Width(); got != before+1 {
		t.Errorf("'..' gave width %d, want %d", got, before+1)
	}
}

// Narrowing stops at one digit rather than collapsing the column.
func TestNarrowingFloorsAtOne(t *testing.T) {
	m := newModel(t, "n,x\n1,1.5\n", 60, 8)
	press(m, "~")
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})

	for i := 0; i < 5; i++ {
		press(m, ",")
	}
	if r, ok := m.fmts[1].(format.Resizable); !ok || r.Size() != 1 {
		t.Errorf("size = %v, want 1", m.fmts[1])
	}
}

func TestSeparatorCycles(t *testing.T) {
	m := newModel(t, sample, 40, 8)
	seen := map[string]bool{m.cfg.Separator: true}
	for range format.Separators {
		press(m, "|")
		seen[m.cfg.Separator] = true
	}
	if len(seen) != len(format.Separators) {
		t.Errorf("saw %d separators, want all %d", len(seen), len(format.Separators))
	}
}

func TestSearchMovesToMatch(t *testing.T) {
	m := newModel(t, sample, 60, 8)

	press(m, "/")
	if m.mode != modeSearch {
		t.Fatal("'/' did not open the search prompt")
	}
	press(m, "g", "a", "m", "m", "a")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.mode != modeNormal {
		t.Error("enter did not close the search prompt")
	}
	if m.cursor[0] != 2 {
		t.Errorf("cursor row = %d, want 2 (the gamma row)", m.cursor[0])
	}
}

func TestSearchReportsNoMatch(t *testing.T) {
	m := newModel(t, sample, 60, 8)

	press(m, "/", "z", "z", "z")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if !strings.Contains(m.flash, "not found") && !strings.Contains(m.flash, "Not found") {
		t.Errorf("flash = %q, want a not-found message", m.flash)
	}
}

func TestBadSearchPatternIsReported(t *testing.T) {
	m := newModel(t, sample, 60, 8)

	press(m, "/", "[")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if !strings.Contains(m.flash, "Bad pattern") {
		t.Errorf("flash = %q, want a bad-pattern message", m.flash)
	}
}

func TestHelpOverlay(t *testing.T) {
	m := newModel(t, sample, 80, 30)
	press(m, "h")

	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"SUMMARY OF GRID COMMANDS", "MOVING", "SEARCHING", "exit"} {
		if !strings.Contains(out, want) {
			t.Errorf("help is missing %q", want)
		}
	}
	// The help is generated from the bindings, so it agrees with the code.
	if !strings.Contains(out, "narrow column at cursor") {
		t.Error("help does not describe ',' as narrowing")
	}

	press(m, "x") // any key dismisses
	if m.mode != modeNormal {
		t.Error("help did not close")
	}
}

func TestTitleLinesRender(t *testing.T) {
	src, err := source.OpenCSV(strings.NewReader("# a note\nn,x\n1,2\n"), source.CSVOptions{
		Filename:      "t.csv",
		HasHeader:     true,
		CommentPrefix: "#",
		Config:        format.DefaultConfig(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	m := New(src, format.DefaultConfig(), 1)
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 8})

	// ngrid crashes on this path: __print calls write(line) with one argument
	// for a function that takes two.
	if first := lines(m)[0]; !strings.Contains(first, "a note") {
		t.Errorf("first line = %q, want the comment header", first)
	}
}

func TestPastEndOfDataShowsTilde(t *testing.T) {
	m := newModel(t, sample, 40, 20) // more rows on screen than in the file
	got := lines(m)

	found := false
	for _, l := range got {
		if strings.HasPrefix(strings.TrimSpace(l), "~") {
			found = true
			break
		}
	}
	if !found {
		t.Error("no '~' marking the rows past the end of the data")
	}
}

func TestQuitKey(t *testing.T) {
	m := newModel(t, sample, 40, 8)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("q returned no command, want tea.Quit")
	}
	if !m.quit {
		t.Error("q did not mark the model as quitting")
	}
}
