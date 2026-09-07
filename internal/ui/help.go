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
// single column it inherited no longer did once search was added.
func (m *Model) helpView() string {
	var (
		titleStyle = lipgloss.NewStyle().Bold(true)
		keyStyle   = lipgloss.NewStyle().Bold(true)
	)

	sections := m.keys.sections()
	left := renderSections(sections[1:2], titleStyle, keyStyle)                      // MOVING
	right := renderSections(append(sections[2:], sections[0]), titleStyle, keyStyle) // the rest

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(42).Render(left),
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
