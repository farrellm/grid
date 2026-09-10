// Package ui presents a Source as an interactive grid.
//
// It is a port of ngrid's GridView, which was view and controller in one. The
// state — the top row, the leftmost scrolling column, the cursor, the per-column
// formatters — carries over directly; the curses event loop is replaced by Bubble
// Tea, and the terminal-size ioctl by WindowSizeMsg.
package ui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/farrellm/grid/internal/format"
	"github.com/farrellm/grid/internal/source"
	"github.com/farrellm/grid/internal/textutil"
)

// cell names a position in the grid. It replaces a [2]int, where the two
// indices were only distinguished by convention.
type cell struct {
	row, col int
}

// mode is which input the keyboard is driving.
type mode int

const (
	modeNormal mode = iota
	modeHelp
	modeSearch
	modeFilter
)

// rowsMsg reports that more rows have been loaded.
type rowsMsg struct{}

// Model is the Bubble Tea model for the grid.
type Model struct {
	src  source.Source
	cfg  format.Config
	keys keyMap
	pal  palette

	// base is the source as opened. src is what the grid shows: base itself,
	// or, while any filter is set, filter, the rows of base that pass.
	base   source.Source
	filter *filtered

	width, height int

	// Visible window: rows [idx0, idx1) and columns from col0 rightwards,
	// after numFrozen pinned columns.
	idx0, idx1 int
	col0       int
	numFrozen  int
	numRows    int // data rows that fit on screen

	cursor     cell
	showCursor bool

	fmts       []format.Formatter
	overridden []bool // columns whose width or precision the user has set
	// display is what the grid renders with: fmts itself, or, while
	// expandNames is on, a copy with every column widened to fit its name.
	// fmts stays what the data and the user's adjustments make it, so turning
	// the mode off puts every column back as it was.
	display     []format.Formatter
	expandNames bool

	mode   mode
	input  textinput.Model
	search searchState
	flash  string

	loading bool
	// followEnd keeps the view pinned to the last row while the rest of the
	// input loads. ngrid's 'G' read the whole file synchronously and then
	// jumped; loading in the background means the target moves as rows arrive.
	followEnd bool
	// following is follow mode, which 'f' holds open over a live stream. It is
	// distinct from followEnd, the pin: 'G' pins the view to the end of a file
	// it is still loading, which any keystroke releases while the load carries
	// on. Follow instead keeps the source reading, so leaving it has to stop
	// that read as well -- and, as in less, any key leaves it.
	following bool
	quit      bool
}

// New creates a model over src. follow starts the view in follow mode, for
// --follow on a live stream.
func New(src source.Source, cfg format.Config, numFrozen int, follow bool) *Model {
	in := textinput.New()
	in.Prompt = "/"
	in.CharLimit = 256

	m := &Model{
		src:        src,
		base:       src,
		cfg:        cfg,
		keys:       defaultKeyMap(),
		pal:        newPalette(),
		numFrozen:  numFrozen,
		col0:       numFrozen,
		showCursor: cfg.ShowCursor,
		input:      in,
		// A sane default until the first WindowSizeMsg arrives.
		width:  80,
		height: 24,
	}
	m.refreshFormatters()
	m.setGeometry()
	if follow {
		m.setFollow(true)
	}
	return m
}

// setFollow enters or leaves follow mode. Following pins the view to the last
// row and keeps the source reading; leaving it stops that read, so a stream
// that never ends is not ingested for ever once the user has looked away.
func (m *Model) setFollow(on bool) {
	m.following = on
	m.followEnd = on
	m.loading = on
	if on {
		m.src.RequestAll()
	} else {
		m.src.StopAll()
	}
}

func (m *Model) Init() tea.Cmd {
	m.src.Request(m.idx1)
	return waitForRows(m.base)
}

// waitForRows blocks off the UI goroutine until the source reports progress.
func waitForRows(s source.Source) tea.Cmd {
	return func() tea.Msg {
		<-s.Ready()
		return rowsMsg{}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.setGeometry()
		return m, nil

	case rowsMsg:
		m.refreshFormatters()
		if m.filter != nil {
			m.filter.scan(filterChunk)
		}
		m.afterRows()
		m.ensureLoaded()
		// Whether to wait for more is the base's to say: with a filter set,
		// the view is not done until the scan finishes too, but no more rows
		// will arrive to wake a wait once the base is.
		scan := m.scanFilter()
		if m.base.Done() {
			return m, scan
		}
		return m, tea.Batch(waitForRows(m.base), scan)

	case filterMsg:
		if msg.f != m.filter {
			return m, nil // queued for a filter since cleared
		}
		m.filter.pending = false
		m.filter.scan(filterChunk)
		m.afterRows()
		m.ensureLoaded()
		return m, m.scanFilter()

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeHelp:
		// Any key dismisses the help, as in ngrid.
		m.mode = modeNormal
		return m, nil
	case modeSearch:
		return m.updateSearch(msg)
	case modeFilter:
		return m.updateFilter(msg)
	}

	// Any keystroke clears the previous message, so it stays readable through
	// the repaints that background loading causes, and stops any seek to the
	// end that is still in progress. Any key also leaves follow mode, which
	// stops the read that follow started; a 'G' seek is only unpinned, so a
	// large file it is still loading carries on loading. Whether follow was on
	// is remembered so that its own key can toggle against what the user could
	// see rather than against the clear below.
	wasFollowing := m.following
	m.flash = ""
	m.followEnd = false
	if m.following {
		m.setFollow(false)
	}

	k := m.keys
	switch {
	case key.Matches(msg, k.Quit):
		m.quit = true
		return m, tea.Quit
	case key.Matches(msg, k.Help):
		m.mode = modeHelp

	case key.Matches(msg, k.Down):
		m.move(+1, 0)
	case key.Matches(msg, k.Up):
		m.move(-1, 0)
	case key.Matches(msg, k.Left):
		m.move(0, -1)
	case key.Matches(msg, k.Right):
		m.move(0, +1)
	case key.Matches(msg, k.PageDown):
		m.moveBy(m.numRows)
	case key.Matches(msg, k.PageUp):
		m.moveBy(-m.numRows)
	case key.Matches(msg, k.HalfDown):
		m.moveBy(m.numRows / 2)
	case key.Matches(msg, k.HalfUp):
		m.moveBy(-m.numRows / 2)

	case key.Matches(msg, k.Top):
		m.moveTo(0)
	case key.Matches(msg, k.Origin):
		m.moveTo(0)
		m.moveToCol(0)
	case key.Matches(msg, k.LastRead):
		m.moveTo(m.src.NumRows() - m.numRows)
	case key.Matches(msg, k.End):
		// Jumping to the true end means reading the rest of the input, which
		// happens in the background; follow it down as rows arrive. This is a
		// seek rather than follow mode: the next keystroke releases the pin but
		// leaves the input loading.
		m.loading = !m.src.Done()
		m.followEnd = !m.src.Done()
		m.src.RequestAll()
		m.moveTo(m.src.NumRows() - m.numRows)
	case key.Matches(msg, k.Follow):
		// Unlike 'G', this is a mode the user holds: on a stream that never
		// ends there is no last row to arrive at, so following is the state
		// rather than a means of getting somewhere.
		if wasFollowing {
			// The clear above has already left follow mode.
			m.flash = "follow off"
			break
		}
		if m.src.Done() {
			m.flash = "no more rows to follow"
			break
		}
		m.setFollow(true)
		m.moveTo(m.src.NumRows() - m.numRows)

	case key.Matches(msg, k.ToggleCursor):
		m.showCursor = !m.showCursor
	case key.Matches(msg, k.HideCursor):
		m.showCursor = false
	case key.Matches(msg, k.CycleSep):
		m.cycleSeparator()
	case key.Matches(msg, k.ToggleHeader):
		m.cfg.ShowHeader = !m.cfg.ShowHeader
		m.setGeometry()
	case key.Matches(msg, k.ToggleFooter):
		m.cfg.ShowFooter = !m.cfg.ShowFooter
		m.setGeometry()

	case key.Matches(msg, k.Narrower):
		m.changeSize(-1)
	case key.Matches(msg, k.Wider):
		m.changeSize(+1)
	case key.Matches(msg, k.ExpandNames):
		m.expandNames = !m.expandNames
		m.applyDisplay()
		// The columns have changed width, which may have pushed the cursor's
		// off the right edge.
		m.move(0, 0)
	case key.Matches(msg, k.LessPrec):
		m.changePrecision(-1)
	case key.Matches(msg, k.MorePrec):
		m.changePrecision(+1)

	case key.Matches(msg, k.SearchFwd):
		m.beginSearch(+1)
	case key.Matches(msg, k.SearchBack):
		m.beginSearch(-1)
	case key.Matches(msg, k.NextMatch):
		m.repeatSearch(+1, false)
	case key.Matches(msg, k.PrevMatch):
		m.repeatSearch(-1, false)
	case key.Matches(msg, k.NextCol):
		m.repeatSearch(+1, true)
	case key.Matches(msg, k.PrevCol):
		m.repeatSearch(-1, true)
	case key.Matches(msg, k.Filter):
		m.beginFilter()
	}

	return m, m.ensureLoaded()
}

// ensureLoaded asks the source for the rows the view is about to need.
func (m *Model) ensureLoaded() tea.Cmd {
	if m.idx1 >= m.src.NumRows() && !m.src.Done() {
		m.src.Request(m.idx1)
	}
	return nil
}

// afterRows brings the view up to date with rows that have arrived, or that a
// filter has let through.
func (m *Model) afterRows() {
	if m.followEnd {
		m.moveTo(m.src.NumRows() - m.numRows)
	}
	m.clampToLoaded()
	if m.src.Done() {
		m.loading = false
		m.following = false
		m.followEnd = false
	}
}

// setGeometry recomputes how many data rows fit, as ngrid's __set_geometry did
// from an ioctl; here the size arrives as a message.
func (m *Model) setGeometry() {
	extra := 0
	if m.cfg.ShowHeader {
		extra += 1 + len(m.src.TitleLines())
	}
	if m.cfg.ShowFooter {
		extra++
	}

	m.numRows = max(m.height-extra, 1)
	m.idx1 = m.idx0 + m.numRows
}

// lastCol returns the rightmost column that fits on screen.
func (m *Model) lastCol() int {
	sep := len(m.cfg.Separator)
	x := 0
	for c := range min(m.numFrozen, len(m.display)) {
		x += m.display[c].Width() + sep
	}
	for c := m.col0; c < m.src.NumCols(); c++ {
		if c >= len(m.display) {
			break
		}
		x += m.display[c].Width() + sep
		if x > m.width {
			return max(c-1, m.col0)
		}
	}
	return m.src.NumCols() - 1
}

// move shifts the cursor when it is shown, and the viewport otherwise.
func (m *Model) move(dr, dc int) {
	if !m.showCursor {
		if dr != 0 {
			m.moveBy(dr)
		}
		if dc != 0 {
			m.moveToCol(m.col0 + dc)
		}
		return
	}

	r := clamp(0, m.cursor.row+dr, m.maxRow())
	c := clamp(0, m.cursor.col+dc, m.src.NumCols()-1)

	if r < m.idx0 {
		m.moveBy(r - m.idx0)
	} else if r >= m.idx1 {
		m.moveBy(r - m.idx1 + 1)
	}
	if c < m.col0 {
		m.moveToCol(c)
	}
	for m.lastCol() < c && m.col0 < m.src.NumCols()-1 {
		m.moveToCol(m.col0 + 1)
	}
	m.cursor = cell{row: r, col: c}
}

func (m *Model) maxRow() int { return max(m.src.NumRows()-1, 0) }

func (m *Model) moveTo(row int) {
	m.idx0 = max(0, row)
	m.idx1 = m.idx0 + m.numRows
	m.clampToLoaded()
	m.cursor.row = clamp(m.idx0, m.cursor.row, max(m.idx1-1, m.idx0))
}

func (m *Model) moveBy(rows int) { m.moveTo(m.idx0 + rows) }

// clampToLoaded keeps the window inside the rows that exist, once the source
// knows how many there are.
func (m *Model) clampToLoaded() {
	if !m.src.Done() {
		return
	}
	n := m.src.NumRows()
	if m.idx1 > n {
		m.idx1 = n
	}
	if m.idx0 > m.idx1-1 {
		m.idx0 = max(m.idx1-1, 0)
	}
	m.idx1 = m.idx0 + m.numRows
	if m.idx1 > n {
		m.idx1 = n
	}
	m.cursor.row = clamp(0, m.cursor.row, m.maxRow())
}

func (m *Model) moveToCol(col int) {
	m.col0 = clamp(m.numFrozen, col, max(m.src.NumCols()-1, m.numFrozen))
	m.cursor.col = clamp(m.col0, m.cursor.col, m.lastCol())
}

func (m *Model) cycleSeparator() {
	i := -1
	for j, s := range format.Separators {
		if s == m.cfg.Separator {
			i = j
			break
		}
	}
	m.cfg.Separator = format.Separators[(i+1)%len(format.Separators)]
}

// changeSize widens or narrows the column under the cursor, where its formatter
// supports it.
func (m *Model) changeSize(d int) {
	if !m.showCursor {
		return
	}
	col := m.cursor.col
	if col >= len(m.fmts) {
		return
	}
	r, ok := m.fmts[col].(format.Resizable)
	if !ok {
		return
	}
	m.setFormatter(col, r.WithSize(max(r.Size()+d, 1)))
}

// changePrecision adds or removes decimal places in the column under the cursor.
func (m *Model) changePrecision(d int) {
	if !m.showCursor {
		return
	}
	col := m.cursor.col
	if col >= len(m.fmts) {
		return
	}
	p, ok := m.fmts[col].(format.Precise)
	if !ok {
		return
	}
	prec := p.Precision()
	if prec == format.NoPrecision {
		prec = 0
	}
	prec += d
	if prec < 0 {
		prec = format.NoPrecision
	}
	m.setFormatter(col, p.WithPrecision(prec))
}

func (m *Model) setFormatter(col int, f format.Formatter) {
	m.fmts[col] = f
	m.overridden[col] = true
	m.src.SetFormatter(col, f)
	m.applyDisplay()
}

// refreshFormatters picks up formatters resized by newly loaded data, leaving
// alone any column the user has adjusted by hand.
func (m *Model) refreshFormatters() {
	next := m.src.Formatters()
	if len(m.fmts) != len(next) {
		m.fmts = make([]format.Formatter, len(next))
		m.overridden = make([]bool, len(next))
		copy(m.fmts, next)
	} else {
		for i := range next {
			if !m.overridden[i] {
				m.fmts[i] = next[i]
			}
		}
	}
	m.applyDisplay()
}

// applyDisplay rebuilds the formatters the grid renders with from fmts. It is
// run when they change rather than on every frame, since widening allocates.
func (m *Model) applyDisplay() {
	if !m.expandNames {
		m.display = m.fmts
		return
	}
	// A slice of its own, so that widening never writes through into fmts.
	names := m.src.Names()
	m.display = make([]format.Formatter, len(m.fmts))
	for c, f := range m.fmts {
		if c < len(names) {
			f = format.Widen(f, textutil.Width(names[c]))
		}
		m.display[c] = f
	}
}

// clamp is not min(max(v, lo), hi): an empty range, where hi < lo, collapses to
// lo rather than to hi. moveToCol and move rely on that, since a table narrower
// than the frozen columns leaves them with no scrolling column to land on.
func clamp(lo, v, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
