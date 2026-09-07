package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/farrellm/grid/internal/textutil"
)

// View renders the whole frame. It is a port of ngrid's __print, which wrote
// cell by cell through curses; here the frame is assembled into one string.
func (m *Model) View() tea.View {
	var v tea.View
	v.AltScreen = true

	if m.mode == modeHelp {
		v.Content = m.helpView()
		return v
	}

	var b strings.Builder
	b.Grow(m.width * m.height)

	rows := 0
	// Comment lines found above the data. ngrid crashed here: it called
	// write(line) with one argument for a function requiring two.
	for _, line := range m.src.TitleLines() {
		if rows >= m.height {
			break
		}
		b.WriteString(clip(line, m.width))
		b.WriteByte('\n')
		rows++
	}

	if m.cfg.ShowHeader && rows < m.height {
		m.writeHeader(&b)
		b.WriteByte('\n')
		rows++
	}

	for i := 0; i < m.numRows && rows < m.height; i++ {
		m.writeRow(&b, m.idx0+i)
		b.WriteByte('\n')
		rows++
	}

	if m.cfg.ShowFooter {
		m.writeFooter(&b)
	}

	v.Content = strings.TrimRight(b.String(), "\n")
	return v
}

// visibleColumns lists the frozen columns followed by the scrolled ones.
func (m *Model) visibleColumns() []int {
	n := m.src.NumCols()
	cols := make([]int, 0, n)
	for c := 0; c < m.numFrozen && c < n; c++ {
		cols = append(cols, c)
	}
	for c := max(m.col0, m.numFrozen); c < n; c++ {
		cols = append(cols, c)
	}
	return cols
}

func (m *Model) writeHeader(b *strings.Builder) {
	names := m.src.Names()
	m.writeCells(b,
		func(c int) (string, cellStyle) {
			name := ""
			if c < len(names) {
				name = names[c]
			}
			w := m.fmts[c].Width()
			// Header names elide near the right but keep their tail, so
			// similar prefixes stay distinguishable.
			text := textutil.Palide(name, w, m.ellipsisFor(w), ' ', 0.7, true)
			return text, m.pal.header(c < m.numFrozen, m.showCursor && c == m.cursor[1])
		},
		func(int) cellStyle { return m.pal.separator })
}

func (m *Model) writeRow(b *strings.Builder, idx int) {
	loaded := idx < m.src.NumRows()

	m.writeCells(b,
		func(c int) (string, cellStyle) {
			var text string
			switch {
			case loaded:
				text = m.fmts[c].Format(m.src.Value(idx, c))
			case c == 0:
				// Past the end of the data, as less marks empty lines.
				text = textutil.Pad("~", m.fmts[c].Width(), ' ', false)
			default:
				text = strings.Repeat(" ", m.fmts[c].Width())
			}
			atCursor := m.showCursor && (idx == m.cursor[0] || c == m.cursor[1])
			atSelect := m.showCursor && idx == m.cursor[0] && c == m.cursor[1]
			return text, m.pal.cell(c < m.numFrozen, atCursor, atSelect)
		},
		func(int) cellStyle {
			if m.showCursor && idx == m.cursor[0] {
				return m.pal.cursor
			}
			return m.pal.separator
		})
}

// writeCells lays out one line of the grid, clipping each cell to the space
// left so that no line can run past the edge of the terminal and wrap.
func (m *Model) writeCells(b *strings.Builder, render func(int) (string, cellStyle), sep func(int) cellStyle) {
	x := 0
	for _, c := range m.visibleColumns() {
		if c >= len(m.fmts) || x >= m.width {
			return
		}

		text, style := render(c)
		text = clip(text, m.width-x)
		style.write(b, text)
		x += textutil.Width(text)
		if x >= m.width {
			return
		}

		s := clip(m.cfg.Separator, m.width-x)
		sep(c).write(b, s)
		x += textutil.Width(s)
	}
}

func (m *Model) writeFooter(b *strings.Builder) {
	if m.mode == modeSearch {
		b.WriteString(clip(m.input.View(), m.width))
		return
	}

	status := m.statusText()

	// The cell under the cursor is shown in full at the right, since the grid
	// may have elided it.
	value := ""
	if m.showCursor && m.cursor[0] < m.src.NumRows() && m.cursor[1] < m.src.NumCols() {
		raw := m.fmts[m.cursor[1]].Format(m.src.Value(m.cursor[0], m.cursor[1]))
		room := m.width - textutil.Width(status) - 4
		if room > 0 {
			value = textutil.Elide(strings.TrimSpace(raw), room, m.cfg.Ellipsis, 1.0)
		}
	}

	gap := m.width - textutil.Width(status) - textutil.Width(value)
	if gap < 1 {
		gap = 1
	}
	line := status + strings.Repeat(" ", gap) + value
	m.pal.footer.write(b, clip(line, m.width))
}

func (m *Model) statusText() string {
	if m.flash != "" {
		return m.flash
	}

	name := m.src.Filename()
	if maxLen := m.width - 40; maxLen > 3 && len(name) > maxLen {
		name = "..." + name[len(name)-maxLen+3:]
	}

	total := m.src.NumRows()
	sep := ""
	if name != "" {
		sep = " "
	}
	status := fmt.Sprintf("%s%slines %d-%d/%d", name, sep, m.idx0, m.idx1, total)

	if m.src.Done() {
		frac := 0.0
		if total > 0 {
			frac = float64(min(m.idx1, total)) / float64(total)
		}
		status += fmt.Sprintf(" %.0f%%", 100*frac)
	} else {
		// A '+' means more rows exist but have not been read, as in ngrid.
		status += "+"
	}
	if m.following && !m.src.Done() {
		// Following is a mode the user is in, so it is named rather than
		// described as activity; it subsumes the loading note.
		status += " FOLLOW"
	} else if m.loading && !m.src.Done() {
		status += " loading…"
	}
	if err := m.src.Err(); err != nil {
		status += " [" + err.Error() + "]"
	}
	return status
}

// ellipsisFor keeps the ellipsis from being wider than the column it marks.
func (m *Model) ellipsisFor(width int) string {
	return textutil.Elide(m.cfg.Ellipsis, width, "", 1.0)
}

// clip hard-truncates s to width display columns, with no ellipsis: it bounds
// a line to the screen rather than signalling elision, which the formatters
// have already done at the cell level.
func clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if textutil.Width(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "")
}
