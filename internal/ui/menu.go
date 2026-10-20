package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/farrellm/grid/internal/format"
	"github.com/farrellm/grid/internal/textutil"
)

// menuState is the 't' menu: the alternate formats offered for one column.
type menuState struct {
	col  int
	alts []format.Alt
	sel  int
}

// toggleAlt restores a column showing an alternate format, or otherwise opens
// the menu of alternates for it. Like '&', it turns the cursor on, since the
// menu is about the column under it.
func (m *Model) toggleAlt() {
	m.showCursor = true
	col := m.cursor.col
	if col >= len(m.fmts) {
		return
	}
	if m.orig[col] != nil {
		m.setFormatter(col, m.orig[col])
		m.orig[col] = nil
		m.move(0, 0) // the column has changed width
		return
	}

	alts := format.AltsFor(m.fmts[col])
	if len(alts) == 0 {
		m.flash = "no other formats for this column"
		return
	}
	// Reopen on the format chosen last time, which toggling off forgets.
	sel := 0
	for i, a := range alts {
		if a.ID == m.altChoice[col] {
			sel = i
		}
	}
	m.menu = menuState{col: col, alts: alts, sel: sel}
	m.mode = modeMenu
}

// updateMenu drives the menu until a format is chosen or it is cancelled.
func (m *Model) updateMenu(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k := msg.String(); k {
	case "up", "k":
		m.menu.sel = max(m.menu.sel-1, 0)
	case "down", "j":
		m.menu.sel = min(m.menu.sel+1, len(m.menu.alts)-1)
	case "enter":
		m.applyAlt(m.menu.sel)
	case "esc", "q", "t", "ctrl+c":
		m.mode = modeNormal
	default:
		if n, err := strconv.Atoi(k); err == nil && n >= 1 && n <= len(m.menu.alts) {
			m.applyAlt(n - 1)
		}
	}
	return m, nil
}

// applyAlt draws the menu's column with its i-th alternate.
func (m *Model) applyAlt(i int) {
	m.mode = modeNormal
	col, alt := m.menu.col, m.menu.alts[i]
	m.orig[col] = m.fmts[col]
	m.altChoice[col] = alt.ID
	m.setFormatter(col, alt.Make(m.orig[col], m.cfg))
	m.move(0, 0) // the column has changed width
}

// colX returns the screen column at which column c starts, or -1 if it is
// scrolled out of view.
func (m *Model) colX(c int) int {
	sep := textutil.Width(m.cfg.Separator)
	x := 0
	for _, vc := range m.visibleColumns() {
		if vc == c {
			return x
		}
		if vc >= len(m.display) {
			break
		}
		x += m.display[vc].Width() + sep
	}
	return -1
}

// withMenu draws the menu over a rendered frame, below the header of the
// column it is for.
func (m *Model) withMenu(frame string) string {
	var b strings.Builder
	for i, a := range m.menu.alts {
		if i > 0 {
			b.WriteByte('\n')
		}
		line := " " + strconv.Itoa(i+1) + " " + a.Name + " "
		if i == m.menu.sel {
			m.pal.cursor.write(&b, line)
		} else {
			b.WriteString(line)
		}
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Render(b.String())

	// Keep the box on screen: pull it left if it would run off the right edge,
	// and up over the header if the terminal is too short for it to sit
	// between the header and the footer, which says how to use it.
	bw, bh := lipgloss.Width(box), lipgloss.Height(box)
	x := min(max(m.colX(m.menu.col), 0), max(m.width-bw, 0))
	y := len(m.src.TitleLines())
	if m.cfg.ShowHeader {
		y++
	}
	bottom := m.height
	if m.cfg.ShowFooter {
		bottom--
	}
	y = min(y, max(bottom-bh, 0))

	return lipgloss.NewCanvas(m.width, m.height).
		Compose(lipgloss.NewCompositor(
			lipgloss.NewLayer(frame),
			lipgloss.NewLayer(box).X(x).Y(y).Z(1),
		)).
		Render()
}
