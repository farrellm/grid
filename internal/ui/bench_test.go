package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/farrellm/grid/internal/filter"
)

// benchCSV builds a wide, deterministic table: enough columns that a 200-column
// terminal has to choose which fit, and a mix of types so the render walks every
// formatter rather than one.
func benchCSV(rows, cols int) string {
	var b strings.Builder
	b.Grow(rows * cols * 12)

	for c := range cols {
		if c > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "column%d", c)
	}
	b.WriteByte('\n')

	for r := range rows {
		for c := range cols {
			if c > 0 {
				b.WriteByte(',')
			}
			switch c % 5 {
			case 0:
				fmt.Fprintf(&b, "%d", r)
			case 1:
				fmt.Fprintf(&b, "%d.%03d", r%1000, r%1000)
			case 2:
				fmt.Fprintf(&b, "%t", r%2 == 0)
			case 3:
				fmt.Fprintf(&b, "word%d", r%997)
			default:
				fmt.Fprintf(&b, "2024-01-%02dT12:00:00Z", r%28+1)
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// benchView renders whole frames, which is what every keystroke and every
// arriving batch costs: View -> visibleColumns -> writeCells -> Source.Value ->
// Formatter.Format -> cellStyle.write.
func benchView(b *testing.B, m *Model) {
	b.Helper()
	b.ReportAllocs()

	var sink string
	for b.Loop() {
		sink = m.View().Content
	}
	if sink == "" {
		b.Fatal("empty frame")
	}
}

func BenchmarkView(b *testing.B) {
	m := newModel(b, benchCSV(2000, 20), 200, 50)
	benchView(b, m)
}

// With the cursor on, every cell also consults the palette for a highlight.
func BenchmarkViewCursor(b *testing.B) {
	m := newModel(b, benchCSV(2000, 20), 200, 50)
	m.showCursor = true
	m.cursor = cell{row: 10, col: 5}
	benchView(b, m)
}

// Widened to their names, the numeric and time columns render through a
// padding wrapper.
func BenchmarkViewExpanded(b *testing.B) {
	m := newModel(b, benchCSV(2000, 20), 200, 50)
	press(m, "w")
	benchView(b, m)
}

// A narrow terminal makes every cell clip, which is the expensive path through
// writeCells.
func BenchmarkViewNarrow(b *testing.B) {
	m := newModel(b, benchCSV(2000, 20), 60, 24)
	benchView(b, m)
}

// Wide characters force the display-width scan rather than the ASCII fast path.
func BenchmarkViewCJK(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("名前,説明,値\n")
	for i := range 2000 {
		fmt.Fprintf(&sb, "項目%d,日本語のテキストはここにあります,%d.5\n", i, i)
	}
	m := newModel(b, sb.String(), 200, 50)
	benchView(b, m)
}

// BenchmarkScroll covers a keystroke end to end: the key match, the viewport
// move, and the frame it produces.
func BenchmarkScroll(b *testing.B) {
	m := newModel(b, benchCSV(2000, 20), 200, 50)
	b.ReportAllocs()

	down := tea.KeyPressMsg{Code: 'd', Text: "d"}
	up := tea.KeyPressMsg{Code: 'u', Text: "u"}
	i := 0
	for b.Loop() {
		if i%2 == 0 {
			m.Update(down)
		} else {
			m.Update(up)
		}
		_ = m.View().Content
		i++
	}
}

// BenchmarkSearch scans the loaded rows for a pattern, formatting every cell it
// tests, which is the worst case for a miss near the end.
func BenchmarkSearch(b *testing.B) {
	m := newModel(b, benchCSV(2000, 20), 200, 50)
	b.ReportAllocs()

	for b.Loop() {
		m.moveTo(0)
		m.search.re = nil
		m.input.SetValue("word996")
		m.mode = modeSearch
		m.search.dir = 1
		m.updateSearch(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	}
	if m.flash != "" {
		b.Fatalf("search failed: %s", m.flash)
	}
}

// BenchmarkFilter scans every loaded row through two stacked filters, a numeric
// comparison and a regular expression, which is what applying a filter costs.
func BenchmarkFilter(b *testing.B) {
	m := newModel(b, benchCSV(2000, 20), 200, 50)
	num, err := filter.Parse("> 500")
	if err != nil {
		b.Fatal(err)
	}
	re, err := filter.Parse("^word9")
	if err != nil {
		b.Fatal(err)
	}
	f := &filtered{Source: m.base, preds: []colPred{{col: 0, pred: num}, {col: 3, pred: re}}}
	b.ReportAllocs()

	for b.Loop() {
		f.reset()
		f.scan(filterChunk)
	}
	if f.NumRows() == 0 {
		b.Fatal("nothing matched")
	}
}

func BenchmarkHelpView(b *testing.B) {
	m := newModel(b, benchCSV(10, 5), 200, 50)
	b.ReportAllocs()

	var sink string
	for b.Loop() {
		sink = m.helpView()
	}
	if sink == "" {
		b.Fatal("empty help")
	}
}
