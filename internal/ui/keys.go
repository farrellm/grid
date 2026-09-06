package ui

import "charm.land/bubbles/v2/key"

// keyMap collects every binding. ngrid kept these in a dict of keycodes to
// lambdas with the help text written out separately, which is how its help came
// to document ',' and '.' the wrong way round; here the help is generated from
// the bindings themselves so the two cannot disagree.
type keyMap struct {
	Help key.Binding
	Quit key.Binding

	Down     key.Binding
	Up       key.Binding
	PageDown key.Binding
	PageUp   key.Binding
	HalfDown key.Binding
	HalfUp   key.Binding
	Left     key.Binding
	Right    key.Binding

	Top      key.Binding
	Origin   key.Binding
	End      key.Binding
	LastRead key.Binding

	ToggleCursor key.Binding
	CycleSep     key.Binding
	ToggleHeader key.Binding
	ToggleFooter key.Binding

	Narrower key.Binding
	Wider    key.Binding
	LessPrec key.Binding
	MorePrec key.Binding

	SearchFwd  key.Binding
	SearchBack key.Binding
	NextMatch  key.Binding
	PrevMatch  key.Binding
	NextCol    key.Binding
	PrevCol    key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Help: key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "display this help")),
		Quit: key.NewBinding(key.WithKeys("q", "Q", "ctrl+c"), key.WithHelp("q Q", "exit")),

		Down:     key.NewBinding(key.WithKeys("down", "enter"), key.WithHelp("↓, RETURN", "forward one line")),
		Up:       key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "backward one line")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", " "), key.WithHelp("PGDN, SPACE", "forward one window")),
		PageUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("PGUP", "backward one window")),
		HalfDown: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "forward one half-window")),
		HalfUp:   key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "backward one half-window")),
		Left:     key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "left one column")),
		Right:    key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "right one column")),

		Top:      key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g, HOME", "jump to first row")),
		Origin:   key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "first row and column")),
		End:      key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "last row of file")),
		LastRead: key.NewBinding(key.WithKeys("end"), key.WithHelp("END", "last row read so far")),

		ToggleCursor: key.NewBinding(key.WithKeys("~", "insert"), key.WithHelp("~, INSERT", "toggle cursor")),
		CycleSep:     key.NewBinding(key.WithKeys("|"), key.WithHelp("|", "cycle column separator")),
		ToggleHeader: key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "toggle header")),
		ToggleFooter: key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "toggle footer")),

		// ngrid's help claimed ',' widened and '.' narrowed; its code did the
		// opposite. The code's reading is the natural one, so it wins.
		Narrower: key.NewBinding(key.WithKeys(","), key.WithHelp(",", "narrow column at cursor")),
		Wider:    key.NewBinding(key.WithKeys("."), key.WithHelp(".", "widen column at cursor")),
		LessPrec: key.NewBinding(key.WithKeys("<"), key.WithHelp("<", "less precision at cursor")),
		MorePrec: key.NewBinding(key.WithKeys(">"), key.WithHelp(">", "more precision at cursor")),

		SearchFwd:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/pattern", "search forward")),
		SearchBack: key.NewBinding(key.WithKeys("?"), key.WithHelp("?pattern", "search backward")),
		NextMatch:  key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "repeat search forward")),
		PrevMatch:  key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "repeat search backward")),
		NextCol:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "forward, scan to column")),
		PrevCol:    key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "backward, scan to column")),
	}
}

// helpSection groups bindings for the help overlay.
type helpSection struct {
	title    string
	bindings []key.Binding
}

func (k keyMap) sections() []helpSection {
	return []helpSection{
		{"", []key.Binding{k.Help, k.Quit}},
		{"MOVING", []key.Binding{
			k.Down, k.Up, k.PageDown, k.PageUp, k.HalfDown, k.HalfUp,
			k.Left, k.Right, k.Top, k.Origin, k.End, k.LastRead,
		}},
		{"CURSOR & DISPLAY", []key.Binding{
			k.CycleSep, k.ToggleHeader, k.ToggleFooter, k.ToggleCursor,
			k.Narrower, k.Wider, k.LessPrec, k.MorePrec,
		}},
		{"SEARCHING", []key.Binding{
			k.SearchFwd, k.SearchBack, k.NextMatch, k.PrevMatch, k.NextCol, k.PrevCol,
		}},
	}
}
