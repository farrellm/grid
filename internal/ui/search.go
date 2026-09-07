package ui

import (
	"fmt"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// searchState holds a compiled pattern and the direction it was entered with.
//
// ngrid shipped this feature commented out and broken — _nextSearchOccurrence
// referenced sampleMatrix, columns and num_rows, none of which exist on the
// view — and "Fix search" heads its todo list. This is a working replacement.
type searchState struct {
	re      *regexp.Regexp
	dir     int  // +1 forward, -1 backward
	toCol   bool // also move the cursor to the matching column
	pattern string

	// resume is where a scan stopped because it ran out of loaded rows, so
	// repeating the search picks up there once more of the stream has arrived
	// rather than starting over.
	resume    int
	exhausted bool
}

// beginSearch opens the prompt in the footer.
func (m *Model) beginSearch(dir int) {
	m.mode = modeSearch
	m.search.dir = dir
	m.input.Reset()
	m.input.Prompt = "/"
	if dir < 0 {
		m.input.Prompt = "?"
	}
	m.input.Focus()
}

// updateSearch drives the prompt until it is submitted or cancelled.
func (m *Model) updateSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		pattern := strings.TrimSpace(m.input.Value())
		m.mode = modeNormal
		m.input.Blur()
		if pattern == "" {
			return m, nil
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			m.flash = fmt.Sprintf("Bad pattern: %v", err)
			return m, nil
		}
		m.search.re = re
		m.search.pattern = pattern
		m.search.exhausted = false
		m.findFrom(m.idx0, m.search.dir, false)
		return m, m.ensureLoaded()

	case "esc", "ctrl+c":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// repeatSearch runs the last pattern again.
func (m *Model) repeatSearch(dir int, toCol bool) {
	if m.search.re == nil {
		m.flash = "No previous search"
		return
	}
	m.search.toCol = toCol

	// Continue where the previous scan ran out of data, if it did.
	start := m.idx0 + dir
	if m.search.exhausted && dir > 0 && m.search.resume > start {
		start = m.search.resume
	}
	m.findFrom(start, dir, toCol)
}

// searchAhead is how many extra rows a search asks for when it reaches the end
// of what has been loaded.
const searchAhead = 10000

// findFrom scans rows from start in direction dir for a cell matching the
// pattern. Matching is against the formatted text, which is what the reader
// actually sees on screen.
//
// Only loaded rows are scanned: the source loads on another goroutine, so
// blocking here would freeze the display on a slow pipe. When the scan runs out
// of rows it asks for more and says so, and 'n' resumes from that point.
func (m *Model) findFrom(start, dir int, toCol bool) {
	if m.search.re == nil {
		return
	}
	m.search.exhausted = false

	row := max(start, 0)
	if dir < 0 {
		row = start
	}

	for {
		if row < 0 {
			m.flash = "Pattern not found"
			return
		}
		if row >= m.src.NumRows() {
			if m.src.Done() {
				m.flash = "Pattern not found"
				return
			}
			// More of the stream may match; fetch it and let 'n' resume.
			m.src.Request(row + searchAhead)
			m.search.resume = row
			m.search.exhausted = true
			m.flash = fmt.Sprintf(
				"Not found in the first %d rows; still reading — press n to continue", row)
			return
		}

		if col, ok := m.matchRow(row, dir); ok {
			m.moveTo(row)
			m.cursor.row = row
			if toCol {
				m.moveToCol(col)
				m.cursor.col = col
			}
			m.showCursor = true // so the match is visible
			m.flash = ""
			return
		}
		row += dir
	}
}

// matchRow reports the first matching column in a row, scanning in the search
// direction so a backward search finds the rightmost match first.
func (m *Model) matchRow(row, dir int) (int, bool) {
	ncols := m.src.NumCols()
	scan := func(c int) bool {
		if c >= len(m.fmts) {
			return false
		}
		return m.search.re.MatchString(m.fmts[c].Format(m.src.Value(row, c)))
	}

	if dir >= 0 {
		for c := range ncols {
			if scan(c) {
				return c, true
			}
		}
	} else {
		for c := ncols - 1; c >= 0; c-- {
			if scan(c) {
				return c, true
			}
		}
	}
	return 0, false
}
