package ui

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/farrellm/grid/internal/format"
	"github.com/farrellm/grid/internal/source"
)

// newModel builds a model over real CSV data at a fixed terminal size. It takes
// a testing.TB so the benchmarks can share it.
func newModel(tb testing.TB, data string, w, h int) *Model {
	tb.Helper()

	src, err := source.OpenCSV(context.Background(), strings.NewReader(data), source.CSVOptions{
		Filename:  "t.csv",
		HasHeader: true,
		Config:    format.DefaultConfig(),
	})
	if err != nil {
		tb.Fatalf("OpenCSV: %v", err)
	}
	tb.Cleanup(func() { src.Close() })

	src.RequestAll()
	for !src.Done() {
		select {
		case <-src.Ready():
		case <-time.After(30 * time.Second):
			tb.Fatal("timed out loading")
		}
	}

	m := New(src, format.DefaultConfig(), 1, false)
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

	if m.cursor.row != 1 {
		t.Errorf("cursor row = %d, want 1", m.cursor.row)
	}
	if m.cursor.col != 1 {
		t.Errorf("cursor column = %d, want 1", m.cursor.col)
	}
	// The footer echoes the selected cell, since the grid may have elided it.
	if footer := lines(m)[len(lines(m))-1]; !strings.Contains(footer, "beta") {
		t.Errorf("footer = %q, want the selected cell 'beta'", footer)
	}
}

// Esc hides the cursor but never shows it, and in a prompt it cancels the
// prompt first, so each press backs out one level.
func TestEscHidesCursor(t *testing.T) {
	m := newModel(t, sample, 60, 8)
	esc := tea.KeyPressMsg{Code: tea.KeyEscape}

	press(m, "~")
	m.Update(esc)
	if m.showCursor {
		t.Fatal("esc did not hide the cursor")
	}
	m.Update(esc)
	if m.showCursor {
		t.Fatal("esc with the cursor off turned it on")
	}

	press(m, "&")
	m.Update(esc)
	if m.mode != modeNormal {
		t.Fatal("esc did not close the filter prompt")
	}
	if !m.showCursor {
		t.Fatal("esc in the prompt also hid the cursor")
	}
	m.Update(esc)
	if m.showCursor {
		t.Error("a second esc did not hide the cursor")
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
	if m.cursor.row != 2 {
		t.Errorf("cursor row = %d, want 2 (the gamma row)", m.cursor.row)
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

// The help is laid out to fit an 80x24 terminal. A binding added to the longer
// column pushes the last line off the bottom, and a description too long for
// its column loses its tail at the right edge; either is cut without warning.
func TestHelpFitsTerminal(t *testing.T) {
	m := newModel(t, sample, 80, 24)
	press(m, "h")

	out := ansi.Strip(m.View().Content)
	want := []string{"Press any key when done."}
	for _, sec := range m.keys.sections() {
		for _, b := range sec.bindings {
			want = append(want, b.Help().Desc)
		}
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("help at 80x24 is missing %q:\n%s", w, out)
		}
	}
}

func TestExpandNamesToggle(t *testing.T) {
	const name = "a_rather_long_name"
	m := newModel(t, "n,"+name+"\n1,2\n", 60, 8)
	before := m.fmts[1].Width()

	if header := lines(m)[0]; strings.Contains(header, name) {
		t.Fatalf("header = %q, want the long name elided before 'w'", header)
	}

	press(m, "w")
	got := lines(m)
	header, row := got[0], got[1]
	at := strings.Index(header, name)
	if at < 0 {
		t.Fatalf("after 'w' header = %q, want the full name", header)
	}
	// The number stays right-aligned, now under the end of its name.
	if end := strings.Index(row, "2") + 1; end != at+len(name) {
		t.Errorf("value ends at column %d, want %d under the end of its name:\n%q\n%q",
			end, at+len(name), header, row)
	}
	if got := m.fmts[1].Width(); got != before {
		t.Errorf("'w' changed the column's own width to %d, want %d", got, before)
	}

	// Arriving rows rebuild the formatters; the mode has to survive that.
	m.refreshFormatters()
	if header := lines(m)[0]; !strings.Contains(header, name) {
		t.Errorf("after a refresh header = %q, want the full name still", header)
	}

	press(m, "w")
	if header := lines(m)[0]; strings.Contains(header, name) {
		t.Errorf("after a second 'w' header = %q, want the name elided again", header)
	}
}

// Widening to the names can take the columns past the edge of the terminal;
// they must still clip rather than wrap.
func TestExpandedViewNeverExceedsWidth(t *testing.T) {
	long := "n," + strings.Repeat("a_column_with_a_very_long_name,", 5) +
		"the_last_column_with_a_very_long_name\n1,2,3,4,5,6,7\n"

	for _, w := range []int{20, 40, 80, 200} {
		m := newModel(t, long, w, 10)
		press(m, "w")
		for i, l := range lines(m) {
			if got := ansi.StringWidth(l); got > w {
				t.Errorf("width %d: line %d is %d columns: %q", w, i, got, l)
			}
		}
	}
}

func TestTitleLinesRender(t *testing.T) {
	src, err := source.OpenCSV(context.Background(), strings.NewReader("# a note\nn,x\n1,2\n"), source.CSVOptions{
		Filename:      "t.csv",
		HasHeader:     true,
		CommentPrefix: "#",
		Config:        format.DefaultConfig(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	m := New(src, format.DefaultConfig(), 1, false)
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

// Bubble Tea names the space bar "space", not " ", so a binding spelled with a
// literal space never matches.
func TestSpacePagesDown(t *testing.T) {
	m := newModel(t, "n\n"+strings.Repeat("1\n", 50), 40, 8)
	press(m, " ")
	if m.idx0 != m.numRows {
		t.Errorf("top row = %d, want %d (space should page down like PGDN)", m.idx0, m.numRows)
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

// applyFilter moves the cursor to col and filters it by expr with '&'.
func applyFilter(m *Model, col int, expr string) {
	m.showCursor = true
	m.move(0, col-m.cursor.col)
	press(m, "&")
	m.input.SetValue(expr)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// shown lists the values of one column across the rows the view holds.
func shown(m *Model, col int) []string {
	var got []string
	for r := range m.src.NumRows() {
		got = append(got, format.Text(m.src.Value(r, col)))
	}
	return got
}

func TestFilterPromptTurnsCursorOn(t *testing.T) {
	m := newModel(t, sample, 60, 8)

	press(m, "&")
	if !m.showCursor {
		t.Error("'&' did not turn the cursor on")
	}
	if m.mode != modeFilter {
		t.Fatal("'&' did not open the filter prompt")
	}
	// The prompt names the column it will filter.
	if footer := lines(m)[len(lines(m))-1]; !strings.HasPrefix(footer, "&n ") {
		t.Errorf("footer = %q, want the prompt naming column n", footer)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.mode != modeNormal || m.filter != nil {
		t.Error("esc did not cancel the filter")
	}
}

func TestFilterRegex(t *testing.T) {
	m := newModel(t, sample, 60, 8)
	applyFilter(m, 1, "^g")

	if got := strings.Join(shown(m, 1), ","); got != "gamma" {
		t.Fatalf("rows = %q, want only gamma", got)
	}
	got := lines(m)
	if !strings.Contains(got[1], "gamma") {
		t.Errorf("first row = %q, want gamma", got[1])
	}
	if !strings.HasPrefix(strings.TrimSpace(got[2]), "~") {
		t.Errorf("second row = %q, want the '~' past the end", got[2])
	}
	footer := got[len(got)-1]
	for _, want := range []string{"/1", "[word /^g/]", "%"} {
		if !strings.Contains(footer, want) {
			t.Errorf("footer = %q, want %q", footer, want)
		}
	}
}

func TestFilterComparison(t *testing.T) {
	m := newModel(t, sample, 60, 8)
	applyFilter(m, 2, "> 2.5")

	if got := strings.Join(shown(m, 2), ","); got != "3.5,4.5" {
		t.Errorf("rows = %q, want 3.5,4.5", got)
	}
}

func TestFilterNegates(t *testing.T) {
	m := newModel(t, sample, 60, 8)
	applyFilter(m, 1, "!^[ab]")

	if got := strings.Join(shown(m, 1), ","); got != "gamma,delta" {
		t.Errorf("rows = %q, want gamma,delta", got)
	}
}

func TestFiltersStack(t *testing.T) {
	m := newModel(t, sample, 80, 8)
	applyFilter(m, 1, "^[abg]")
	applyFilter(m, 0, ">= 2")

	if got := strings.Join(shown(m, 1), ","); got != "beta,gamma" {
		t.Errorf("rows = %q, want beta,gamma", got)
	}
	if footer := lines(m)[len(lines(m))-1]; !strings.Contains(footer, "[word /^[abg]/, n >=2]") {
		t.Errorf("footer = %q, want both filters listed", footer)
	}
}

func TestBadFilterIsReported(t *testing.T) {
	m := newModel(t, sample, 60, 8)
	applyFilter(m, 0, "< NA")

	if !strings.Contains(m.flash, "Bad filter") {
		t.Errorf("flash = %q, want a bad-filter message", m.flash)
	}
	if m.filter != nil || m.src.NumRows() != 4 {
		t.Error("a bad filter was applied")
	}
}

// Clearing the filters keeps the cursor on the row it was on, which has a
// different index among all the rows than among the filtered ones.
func TestClearFilterKeepsRow(t *testing.T) {
	m := newModel(t, sample, 60, 8)
	applyFilter(m, 1, "^[gd]")
	m.move(1, 0) // delta, row 1 filtered and row 3 in all

	applyFilter(m, 1, "")
	if m.filter != nil || m.src.NumRows() != 4 {
		t.Fatalf("an empty filter left %d rows, want all 4", m.src.NumRows())
	}
	if m.cursor.row != 3 {
		t.Errorf("cursor row = %d, want 3 (delta)", m.cursor.row)
	}
	if footer := lines(m)[len(lines(m))-1]; !strings.Contains(footer, "delta") {
		t.Errorf("footer = %q, want the cursor still on delta", footer)
	}
}

func TestSearchWithinFilter(t *testing.T) {
	m := newModel(t, sample, 60, 8)
	applyFilter(m, 1, "^[gd]")

	press(m, "/", "d", "e", "l")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.cursor.row != 1 {
		t.Errorf("cursor row = %d, want 1, delta's row among the filtered", m.cursor.row)
	}
}

// A filter over more rows than one chunk finishes the scan through filterMsg,
// between keystrokes, and counts every match.
func TestFilterScansInChunks(t *testing.T) {
	const n = 2*filterChunk + 10
	var b strings.Builder
	b.WriteString("n\n")
	for i := range n {
		b.WriteString(strconv.Itoa(i))
		b.WriteByte('\n')
	}
	m := newModel(t, b.String(), 60, 8)

	applyFilter(m, 0, "0$")
	if m.src.Done() {
		t.Fatal("one chunk scanned everything; the test needs more rows")
	}
	if footer := lines(m)[len(lines(m))-1]; !strings.Contains(footer, "filtering…") {
		t.Errorf("footer = %q, want it to say the scan is under way", footer)
	}

	for i := 0; !m.src.Done(); i++ {
		if i > n/filterChunk+1 {
			t.Fatal("scan did not finish")
		}
		m.Update(filterMsg{m.filter})
	}
	// The multiples of ten below n.
	if got, want := m.src.NumRows(), (n+9)/10; got != want {
		t.Errorf("rows = %d, want %d", got, want)
	}
}

// A filterMsg queued before the filters were cleared must not touch the next
// filter, which has a scan of its own.
func TestStaleFilterMsgIgnored(t *testing.T) {
	m := newModel(t, sample, 60, 8)
	applyFilter(m, 1, "^g")
	stale := filterMsg{m.filter}
	applyFilter(m, 1, "")
	applyFilter(m, 1, "^a")

	m.Update(stale)
	if got := strings.Join(shown(m, 1), ","); got != "alpha" {
		t.Errorf("rows = %q, want alpha", got)
	}
}

// sortSample has ties in word and blanks in x, which sort last either way.
const sortSample = "n,word,x\n1,b,2.5\n2,a,\n3,b,1.5\n4,a,3.5\n5,c,\n"

// sortBy moves the cursor to col and presses 's' there.
func sortBy(m *Model, col int, presses int) {
	m.showCursor = true
	m.move(0, col-m.cursor.col)
	for range presses {
		press(m, "s")
	}
}

func footer(m *Model) string {
	l := lines(m)
	return l[len(l)-1]
}

func TestSortCycles(t *testing.T) {
	m := newModel(t, sortSample, 80, 10)
	m.showCursor = false
	m.move(0, 2)
	press(m, "s")
	if !m.showCursor {
		t.Error("'s' did not turn the cursor on")
	}

	for i, step := range []struct{ want, footer string }{
		{"1.5,2.5,3.5,,", "sort[↑x]"},
		{"3.5,2.5,1.5,,", "sort[↓x]"},
		{"2.5,,1.5,3.5,", ""},
	} {
		if i > 0 {
			press(m, "s")
		}
		if got := strings.Join(shown(m, 2), ","); got != step.want {
			t.Errorf("x = %q, want %q", got, step.want)
		}
		if f := footer(m); step.footer != "" && !strings.Contains(f, step.footer) {
			t.Errorf("footer = %q, want %q", f, step.footer)
		}
	}
	if m.sort != nil {
		t.Error("the third 's' left the column sorted")
	}
}

// A column sorted afresh becomes the primary key, and the one sorted before it
// breaks its ties.
func TestSortsStack(t *testing.T) {
	m := newModel(t, sortSample, 80, 10)
	sortBy(m, 0, 2) // n, descending
	sortBy(m, 1, 1) // word, ascending

	if got := strings.Join(shown(m, 0), ","); got != "4,2,3,1,5" {
		t.Errorf("n = %q, want 4,2,3,1,5", got)
	}
	if f := footer(m); !strings.Contains(f, "sort[↑word, ↓n]") {
		t.Errorf("footer = %q, want both keys, word first", f)
	}

	// n was descending, so 's' on it again takes it out, and word's ties fall
	// back to the order they were read in.
	sortBy(m, 0, 1)
	if got := strings.Join(shown(m, 0), ","); got != "2,4,1,3,5" {
		t.Errorf("n = %q, want 2,4,1,3,5", got)
	}
	if f := footer(m); !strings.Contains(f, "sort[↑word]") {
		t.Errorf("footer = %q, want only word left", f)
	}
}

func TestClearSortKeepsRow(t *testing.T) {
	m := newModel(t, sortSample, 80, 10)
	sortBy(m, 2, 1)
	m.move(1, 0) // 2.5: row 1 sorted and row 0 as read

	press(m, "S")
	if m.sort != nil || m.sortKeys != nil {
		t.Fatal("'S' left a sort")
	}
	if m.cursor.row != 0 {
		t.Errorf("cursor row = %d, want 0 (2.5)", m.cursor.row)
	}
}

// A sort orders whatever the filters keep, and survives their changing.
func TestSortOverFilter(t *testing.T) {
	m := newModel(t, sortSample, 80, 10)
	applyFilter(m, 1, "^[ab]")
	sortBy(m, 2, 2)
	if got := strings.Join(shown(m, 2), ","); got != "3.5,2.5,1.5," {
		t.Errorf("x = %q, want 3.5,2.5,1.5,", got)
	}

	applyFilter(m, 0, "!= 4")
	if got := strings.Join(shown(m, 2), ","); got != "2.5,1.5," {
		t.Errorf("x = %q after another filter, want 2.5,1.5,", got)
	}
	if f := footer(m); !strings.Contains(f, "n !=4] sort[↓x]") {
		t.Errorf("footer = %q, want the filters then the sort", f)
	}

	m.move(1, 0) // 1.5, n=3
	applyFilter(m, 0, "")
	if got := strings.Join(shown(m, 2), ","); got != "3.5,2.5,1.5,," {
		t.Errorf("x = %q after clearing the filters, want every row sorted", got)
	}
	if got := format.Text(m.src.Value(m.cursor.row, 0)); got != "3" {
		t.Errorf("cursor on n=%s, want it kept on 3", got)
	}
}

// runSorts runs cmd and feeds any sortMsg it produces back to the model.
func runSorts(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			runSorts(m, c)
		}
	case sortMsg:
		_, next := m.Update(msg)
		runSorts(m, next)
	}
}

func bigColumn(n int) string {
	var b strings.Builder
	b.WriteString("n\n")
	for i := range n {
		b.WriteString(strconv.Itoa(i))
		b.WriteByte('\n')
	}
	return b.String()
}

// A sort of more than sortChunk rows runs off the UI goroutine, and the view
// says so until its result arrives.
func TestSortRunsInBackground(t *testing.T) {
	const n = sortChunk + 10
	m := newModel(t, bigColumn(n), 60, 8)
	m.showCursor = true
	press(m, "s")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	if m.src.Done() {
		t.Fatal("the sort finished on the UI goroutine")
	}
	if f := footer(m); !strings.Contains(f, "sorting…") {
		t.Errorf("footer = %q, want it to say the sort is under way", f)
	}

	runSorts(m, cmd)
	if !m.src.Done() || m.src.NumRows() != n {
		t.Fatalf("done = %v with %d rows, want all %d", m.src.Done(), m.src.NumRows(), n)
	}
	if got := format.Text(m.src.Value(0, 0)); got != strconv.Itoa(n-1) {
		t.Errorf("first row = %s, want %d", got, n-1)
	}
}

// A sort job finishing after its sort was replaced must not touch the new one.
func TestStaleSortMsgIgnored(t *testing.T) {
	const n = sortChunk + 10
	m := newModel(t, bigColumn(n), 60, 8)
	m.showCursor = true
	_, stale := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})

	runSorts(m, stale)
	if m.src.NumRows() != 0 {
		t.Fatal("a stale sort's rows were applied")
	}
	runSorts(m, cmd)
	if got := format.Text(m.src.Value(0, 0)); got != strconv.Itoa(n-1) {
		t.Errorf("first row = %s, want %d", got, n-1)
	}
}

// fakeSource is a source that never finishes, standing in for a live pipe so
// follow mode can be exercised without one.
type fakeSource struct {
	*source.CSV
	all bool
}

func (f *fakeSource) Done() bool   { return false }
func (f *fakeSource) RequestAll()  { f.all = true }
func (f *fakeSource) StopAll()     { f.all = false }
func (f *fakeSource) NumRows() int { return f.CSV.NumRows() }

func newStreamModel(t *testing.T) (*Model, *fakeSource) {
	t.Helper()
	src, err := source.OpenCSV(context.Background(), strings.NewReader("n,x\n1,1.5\n2,2.5\n3,3.5\n"),
		source.CSVOptions{Filename: "stream.csv", HasHeader: true, Config: format.DefaultConfig()})
	if err != nil {
		t.Fatalf("OpenCSV: %v", err)
	}
	t.Cleanup(func() { src.Close() })

	// Ready signals coalesce, so wait on the deadline rather than on a count
	// of wakeups.
	src.RequestAll()
	deadline := time.Now().Add(5 * time.Second)
	for !src.Done() {
		if time.Now().After(deadline) {
			t.Fatal("timed out loading")
		}
		time.Sleep(time.Millisecond)
	}

	f := &fakeSource{CSV: src}
	m := New(f, format.DefaultConfig(), 1, false)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	m.refreshFormatters()
	return m, f
}

// TestFollowToggle covers the mode 'f' holds open over a live stream: it names
// itself in the status bar and keeps the source reading, and turning it off
// stops that read rather than leaving a stream to be ingested for ever.
func TestFollowToggle(t *testing.T) {
	m, f := newStreamModel(t)

	if got := strings.Join(lines(m), "\n"); strings.Contains(got, "FOLLOW") {
		t.Error("FOLLOW shown before follow was asked for")
	}

	press(m, "f")
	if !m.following {
		t.Error("f did not enter follow mode")
	}
	if !f.all {
		t.Error("follow did not ask the source to keep reading")
	}
	if got := strings.Join(lines(m), "\n"); !strings.Contains(got, "FOLLOW") {
		t.Errorf("status bar does not show FOLLOW:\n%s", got)
	}

	press(m, "f")
	if m.following {
		t.Error("a second f did not leave follow mode")
	}
	if f.all {
		t.Error("leaving follow did not stop the source reading")
	}
	if got := strings.Join(lines(m), "\n"); strings.Contains(got, "FOLLOW") {
		t.Errorf("status bar still shows FOLLOW after leaving:\n%s", got)
	}
}

// TestFollowLeftByAnyKey follows less: a keystroke that is not f still leaves
// follow mode, and stops the read it started.
func TestFollowLeftByAnyKey(t *testing.T) {
	m, f := newStreamModel(t)

	press(m, "f")
	if !m.following || !f.all {
		t.Fatal("f did not enter follow mode")
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.following {
		t.Error("an arrow key left follow mode set")
	}
	if f.all {
		t.Error("leaving follow by another key did not stop the read")
	}
}

// TestSeekToEndKeepsLoading guards the distinction between follow and 'G': a
// seek to the end of a file that is still loading is only unpinned by the next
// keystroke, and the load carries on.
func TestSeekToEndKeepsLoading(t *testing.T) {
	m, f := newStreamModel(t)

	press(m, "G")
	if m.following {
		t.Error("G entered follow mode; it should only pin the view")
	}
	if !f.all {
		t.Fatal("G did not ask for the rest of the input")
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.followEnd {
		t.Error("a keystroke did not release the pin")
	}
	if !f.all {
		t.Error("a keystroke stopped a G seek from loading; it should carry on")
	}
}

// TestFollowOnFinishedInputSaysSo covers 'f' where there is nothing to follow.
func TestFollowOnFinishedInputSaysSo(t *testing.T) {
	m := newModel(t, "n,x\n1,1.5\n2,2.5\n", 60, 12)

	press(m, "f")
	if m.following {
		t.Error("entered follow mode on a fully loaded input")
	}
	if got := strings.Join(lines(m), "\n"); !strings.Contains(got, "no more rows to follow") {
		t.Errorf("no explanation shown:\n%s", got)
	}
}

const epochs = "id,word,created_utc\n1,alpha,1672531155\n2,beta,1672617555\n"

func TestAltFormatMenu(t *testing.T) {
	m := newModel(t, epochs, 60, 8)
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // created_utc

	press(m, "t")
	if !m.showCursor {
		t.Error("'t' did not turn the cursor on")
	}
	if m.mode != modeMenu {
		t.Fatal("'t' did not open the format menu")
	}
	if view := strings.Join(lines(m), "\n"); !strings.Contains(view, "UTC datetime") {
		t.Fatalf("menu is not drawn:\n%s", view)
	}

	m.Update(enter)
	if view := strings.Join(lines(m), "\n"); !strings.Contains(view, "2022-12-31T23:59:15Z") {
		t.Fatalf("column is not drawn as a timestamp:\n%s", view)
	}

	press(m, "t")
	if m.mode != modeNormal {
		t.Fatal("'t' on an alternate format opened the menu instead of restoring")
	}
	if view := strings.Join(lines(m), "\n"); !strings.Contains(view, "1672531155") {
		t.Fatalf("column is not restored:\n%s", view)
	}

	// The menu reopens on the choice made last time, and moves on from it.
	press(m, "t")
	if m.menu.alts[m.menu.sel].ID != "utc" {
		t.Errorf("menu reopened on %q, want utc", m.menu.alts[m.menu.sel].ID)
	}
	press(m, "j")
	m.Update(enter)
	if got := m.altChoice[2]; got != "local" {
		t.Errorf("choice = %q, want local", got)
	}

	// A digit picks directly.
	press(m, "t", "t", "5")
	if view := strings.Join(lines(m), "\n"); !strings.Contains(view, "1,672,531,155") {
		t.Errorf("'5' did not pick thousands separators:\n%s", view)
	}
}

func TestAltFormatMenuCancels(t *testing.T) {
	m := newModel(t, epochs, 60, 8)
	press(m, "~")
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	before := m.fmts[2]

	press(m, "t")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.mode != modeNormal || m.fmts[2] != before || m.orig[2] != nil {
		t.Error("esc did not leave the column as it was")
	}
}

func TestAltFormatOnText(t *testing.T) {
	m := newModel(t, epochs, 60, 8)
	press(m, "~")
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // word

	press(m, "t")
	if m.mode != modeNormal {
		t.Fatal("'t' opened a menu for a text column")
	}
	if footer := lines(m)[len(lines(m))-1]; !strings.Contains(footer, "no other formats") {
		t.Errorf("footer = %q, want the reason", footer)
	}
}

// The menu stays inside the terminal, however small, and wherever the column.
func TestAltFormatMenuFits(t *testing.T) {
	for _, size := range [][2]int{{60, 8}, {30, 4}, {80, 24}} {
		m := newModel(t, epochs, size[0], size[1])
		m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		press(m, "t")
		got := lines(m)
		if len(got) > size[1] {
			t.Errorf("%dx%d: %d lines", size[0], size[1], len(got))
		}
		for _, l := range got {
			if w := ansi.StringWidth(l); w > size[0] {
				t.Errorf("%dx%d: line %q is %d wide", size[0], size[1], l, w)
			}
		}
	}
}
