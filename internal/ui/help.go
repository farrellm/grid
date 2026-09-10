package ui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/farrellm/grid/internal/textutil"
)

// helpView renders the key reference.
//
// ngrid kept this as a hardcoded block of text that had drifted out of step
// with its keymap — it documented ',' and '<' as increasing what the code
// decreased. Here it is generated from the bindings, so the two cannot
// disagree. It is laid out in two columns to fit a 24-line terminal, which the
// single column it inherited no longer did once search was added. Help and quit
// sit under MOVING, which is the shorter column.
func (m *Model) helpView() string {
	var (
		titleStyle = lipgloss.NewStyle().Bold(true)
		keyStyle   = lipgloss.NewStyle().Bold(true)
	)

	sections := m.keys.sections()
	left := renderSections([]helpSection{sections[1], sections[0]}, titleStyle, keyStyle) // MOVING, help and quit
	right := renderSections(sections[2:], titleStyle, keyStyle)                           // the rest

	// The left column is exactly as wide as its longest line; the right one
	// brings its own indent as a gutter. Any wider and the longest lines on the
	// right lose their last letter on an 80-column terminal.
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(41).Render(left),
		right)

	out := titleStyle.Render("  SUMMARY OF GRID COMMANDS") + "\n\n" +
		body + "\n\n  Press any key when done."

	lines := strings.Split(out, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for i, l := range lines {
		lines[i] = clip(l, m.width)
	}
	return strings.Join(lines, "\n")
}

func renderSections(sections []helpSection, titleStyle, keyStyle lipgloss.Style) string {
	var b strings.Builder
	for i, sec := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		if sec.title != "" {
			b.WriteString("  " + titleStyle.Render(sec.title) + "\n")
		}
		for _, bind := range sec.bindings {
			h := bind.Help()
			b.WriteString("  " + keyStyle.Render(textutil.Pad(h.Key, 12, ' ', false)) +
				" " + h.Desc + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
