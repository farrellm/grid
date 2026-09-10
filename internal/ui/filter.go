package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/farrellm/grid/internal/filter"
	"github.com/farrellm/grid/internal/format"
	"github.com/farrellm/grid/internal/source"
)

// filterChunk is how many rows a filter examines between keystrokes. Scanning
// runs on the UI goroutine, where the formatters and the rest of the model live,
// so a large input is taken a chunk at a time rather than freezing the display.
const filterChunk = 50_000

// filterMsg asks for the next chunk of a filter's scan. It names the filter it
// was queued for, so a message outliving that filter is ignored.
type filterMsg struct{ f *filtered }

// colPred is one '&' filter: a predicate on one column.
type colPred struct {
	col  int
	name string
	pred filter.Pred
}

// filtered is the rows of a source that pass every filter. It is a Source
// itself, mapping its rows onto the base's, so scrolling, the cursor, search,
// 'G' and follow all work on the filtered rows unchanged. Filters stack: a row
// is kept only if it passes all of them.
//
// Rows are examined as the base loads them and never revisited, which relies on
// the base being append-only: widening a column re-reads only the batch being
// built, so a row index, once assigned, always names the same row.
type filtered struct {
	source.Source
	preds   []colPred
	rows    []int // base indices of the rows that pass
	scanned int   // base rows examined so far
	pending bool  // a filterMsg is queued
}

func (f *filtered) NumRows() int { return len(f.rows) }

func (f *filtered) Value(row, col int) format.Value {
	if row < 0 || row >= len(f.rows) {
		return format.Null()
	}
	return f.Source.Value(f.rows[row], col)
}

// Done reports whether every row has been both loaded and examined.
func (f *filtered) Done() bool { return f.Source.Done() && !f.unscanned() }

// Request asks the base for more rows once those it has are exhausted. How many
// more it takes to supply n filtered rows cannot be known, so it asks in steps,
// each arrival being scanned before the next is requested; a sparse filter reads
// on until the screen fills, as less's '&' does.
func (f *filtered) Request(n int) {
	if len(f.rows) < n && !f.unscanned() {
		f.Source.Request(f.scanned + searchAhead)
	}
}

// Close leaves the base open: whoever opened it closes it.
func (f *filtered) Close() error { return nil }

// unscanned reports whether loaded rows remain to be examined.
func (f *filtered) unscanned() bool { return f.scanned < f.Source.NumRows() }

// reset forgets every row examined, for when the filters have changed.
func (f *filtered) reset() {
	f.rows = f.rows[:0]
	f.scanned = 0
}

// scan examines up to limit more of the loaded rows, keeping those that pass.
func (f *filtered) scan(limit int) {
	end := min(f.Source.NumRows(), f.scanned+limit)
	for r := f.scanned; r < end; r++ {
		if f.keeps(r) {
			f.rows = append(f.rows, r)
		}
	}
	f.scanned = end
}

func (f *filtered) keeps(row int) bool {
	for _, p := range f.preds {
		if !p.pred.Match(f.Source.Value(row, p.col)) {
			return false
		}
	}
	return true
}

// String lists the filters for the status bar.
func (f *filtered) String() string {
	parts := make([]string, len(f.preds))
	for i, p := range f.preds {
		parts[i] = p.name + " " + p.pred.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// beginFilter opens the prompt for a filter on the cursor's column. The cursor
// is turned on if it was off, so the column being filtered is highlighted.
func (m *Model) beginFilter() {
	m.showCursor = true
	m.mode = modeFilter
	m.input.Reset()
	m.input.Prompt = "&" + m.colName(m.cursor.col) + " "
	m.input.Focus()
}

// updateFilter drives the prompt until it is submitted or cancelled. An empty
// filter clears them all, as an empty '&' does in less.
func (m *Model) updateFilter(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		expr := strings.TrimSpace(m.input.Value())
		m.mode = modeNormal
		m.input.Blur()
		if expr == "" {
			m.clearFilter()
			m.ensureLoaded()
			return m, nil
		}
		p, err := filter.Parse(expr)
		if err != nil {
			m.flash = fmt.Sprintf("Bad filter: %v", err)
			return m, nil
		}
		col := m.cursor.col
		m.addFilter(colPred{col: col, name: m.colName(col), pred: p})
		m.ensureLoaded()
		return m, m.scanFilter()

	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// addFilter narrows the view by one more filter and returns to the top, since
// the rows on screen may be gone. The first chunk is scanned at once, so a
// small input shows its result without waiting on the event loop.
func (m *Model) addFilter(p colPred) {
	if m.filter == nil {
		m.filter = &filtered{Source: m.base}
		m.src = m.filter
	}
	m.filter.preds = append(m.filter.preds, p)
	m.filter.reset()
	m.filter.scan(filterChunk)
	m.search.exhausted = false // its resume point indexed the old rows
	m.moveTo(0)
	m.cursor.row = 0
}

// clearFilter removes every filter, keeping the cursor on the row it was on and
// at the same height on the screen.
func (m *Model) clearFilter() {
	if m.filter == nil {
		return
	}
	offset := m.cursor.row - m.idx0
	row := 0
	if r := m.cursor.row; r < len(m.filter.rows) {
		row = m.filter.rows[r]
	}

	m.filter = nil
	m.src = m.base
	m.search.exhausted = false
	m.moveTo(row - offset)
	m.cursor.row = clamp(0, row, m.maxRow())
}

// scanFilter queues the next chunk of the filter's scan, if loaded rows remain
// to be examined and a chunk is not already queued. One chunk per message lets
// keystrokes through in between.
func (m *Model) scanFilter() tea.Cmd {
	f := m.filter
	if f == nil || f.pending || !f.unscanned() {
		return nil
	}
	f.pending = true
	return func() tea.Msg { return filterMsg{f} }
}

// colName names a column for the prompt and the status bar.
func (m *Model) colName(col int) string {
	if names := m.src.Names(); col < len(names) {
		return names[col]
	}
	return strconv.Itoa(col)
}
