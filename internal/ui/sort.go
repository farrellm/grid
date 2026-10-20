package ui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/farrellm/grid/internal/format"
	"github.com/farrellm/grid/internal/source"
)

// sortChunk is the most rows a sort places on the UI goroutine. A sort can't be
// taken a piece at a time between keystrokes as a filter's scan is, so a larger
// one runs on a goroutine of its own and the view waits for it.
const sortChunk = 100_000

// sortMsg carries a finished sort job back to the UI goroutine. It names the
// sort it was run for, so a result outliving that sort is ignored.
type sortMsg struct {
	s   *sorted
	res sortResult
}

// sortKey is one 's' sort: a column and a direction.
type sortKey struct {
	col  int
	name string
	desc bool
}

// sorted is the rows of a source reordered by one or more columns. Like
// filtered, it is a Source itself, mapping its rows onto inner's, so scrolling,
// the cursor, search and the cell readout work on the sorted rows unchanged.
//
// Rows that inner gains are sorted on their own and merged in, which relies on
// inner growing only at the end: the base is append-only, and so is a filter
// between resets, and a reset builds a new sorted.
type sorted struct {
	source.Source // inner: the base, or the filtered view

	base   source.Source
	filter *filtered // inner, when it is a filter; nil otherwise
	keys   []sortKey // primary first

	rows []int // inner indices, in sorted order
	// vals holds each key's value for every placed row, indexed by key and
	// then inner row, so that comparing two rows reads no batches.
	vals    [][]format.Value
	merged  int  // inner rows placed so far
	pending bool // a sort job is running
}

func newSorted(inner, base source.Source, f *filtered, keys []sortKey) *sorted {
	return &sorted{Source: inner, base: base, filter: f, keys: keys, vals: make([][]format.Value, len(keys))}
}

func (s *sorted) NumRows() int { return len(s.rows) }

func (s *sorted) Value(row, col int) format.Value {
	if row < 0 || row >= len(s.rows) {
		return format.Null()
	}
	return s.Source.Value(s.rows[row], col)
}

// Done reports whether every row has been loaded and placed.
func (s *sorted) Done() bool { return s.Source.Done() && !s.unsorted() }

// Request asks for every row whatever the screen needs: until the last row has
// been read, any of them might sort first. Leaving follow mode stops a read
// with StopAll, and the next request takes it up again.
func (s *sorted) Request(int) { s.RequestAll() }

// Close leaves inner open: whoever opened it closes it.
func (s *sorted) Close() error { return nil }

// unsorted reports whether rows remain to be placed, or are being placed.
func (s *sorted) unsorted() bool { return s.pending || s.merged < s.Source.NumRows() }

// String lists the keys for the status bar, primary first.
func (s *sorted) String() string {
	parts := make([]string, len(s.keys))
	for i, k := range s.keys {
		arrow := "↑"
		if k.desc {
			arrow = "↓"
		}
		parts[i] = arrow + k.name
	}
	return "sort[" + strings.Join(parts, ", ") + "]"
}

// sortResult is a sort's state with a run of new rows merged in.
type sortResult struct {
	rows   []int
	vals   [][]format.Value
	merged int
}

// place sorts inner rows [from, to) and merges them into the rows already
// placed. ids names their base rows when inner is a filter, and is nil when
// inner is the base. It reads only the base, which is safe from any goroutine,
// and rows and vals, which nothing changes while a job is pending; vals may be
// extended in place, but past the length anything else reads.
func (s *sorted) place(ids []int, from, to int) sortResult {
	vals := make([][]format.Value, len(s.keys))
	for k, key := range s.keys {
		vs := slices.Grow(s.vals[k], to-from)[:to]
		for r := from; r < to; r++ {
			id := r
			if ids != nil {
				id = ids[r-from]
			}
			vs[r] = s.base.Value(id, key.col)
		}
		vals[k] = vs
	}

	order := func(a, b int) int {
		for k, key := range s.keys {
			if c := compareKey(vals[k][a], vals[k][b], key.desc); c != 0 {
				return c
			}
		}
		return 0
	}
	run := make([]int, to-from)
	for i := range run {
		run[i] = from + i
	}
	slices.SortStableFunc(run, order)
	return sortResult{rows: merge(s.rows, run, order), vals: vals, merged: to}
}

// compareKey orders two cells in one key's direction. Missing values stay last
// either way, as they do in a spreadsheet: they are not the largest values, and
// a descending sort is read for the largest ones.
func compareKey(a, b format.Value, desc bool) int {
	c := format.Compare(a, b)
	if desc && !a.IsMissing() && !b.IsMissing() {
		return -c
	}
	return c
}

// merge interleaves two sorted runs. On a tie the row from a goes first: a
// holds the rows read earlier, so the sort stays stable.
func merge(a, b []int, order func(a, b int) int) []int {
	out := make([]int, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if order(a[i], b[j]) <= 0 {
			out = append(out, a[i])
			i++
		} else {
			out = append(out, b[j])
			j++
		}
	}
	out = append(out, a[i:]...)
	return append(out, b[j:]...)
}

func (s *sorted) apply(res sortResult) {
	s.rows, s.vals, s.merged = res.rows, res.vals, res.merged
	s.pending = false
}

// toggleSort cycles the cursor's column through ascending, descending and
// unsorted. A column sorted afresh becomes the primary key, and those sorted
// before it break its ties. The cursor is turned on if it was off, so the
// column being sorted is highlighted.
func (m *Model) toggleSort() tea.Cmd {
	m.showCursor = true
	col := m.cursor.col
	i := slices.IndexFunc(m.sortKeys, func(k sortKey) bool { return k.col == col })

	var cmd tea.Cmd
	if i >= 0 && m.sortKeys[i].desc {
		// The last step of the cycle takes the column out, keeping the cursor
		// on its row in whatever order the other keys leave.
		m.sortKeys = slices.Delete(slices.Clone(m.sortKeys), i, i+1)
		m.keepRow(func() {
			m.rebuildView()
			cmd = m.scanSort()
		})
		return cmd
	}

	key := sortKey{col: col, name: m.colName(col), desc: i >= 0}
	keys := slices.Clone(m.sortKeys)
	if i >= 0 {
		keys = slices.Delete(keys, i, i+1)
	}
	m.sortKeys = slices.Insert(keys, 0, key)
	m.rebuildView()
	cmd = m.scanSort()
	m.moveTo(0)
	m.cursor.row = 0
	return cmd
}

// clearSort puts the rows back in the order they were read, keeping the cursor
// on the row it was on.
func (m *Model) clearSort() {
	if m.sort == nil {
		return
	}
	m.sortKeys = nil
	m.keepRow(m.rebuildView)
}

// scanSort places the rows inner has gained since the sort last looked, if a
// job is not already doing so: at once if the whole sort is small, and
// otherwise on a goroutine whose result arrives as a sortMsg.
func (m *Model) scanSort() tea.Cmd {
	s := m.sort
	if s == nil || s.pending || s.merged >= s.Source.NumRows() {
		return nil
	}
	from, to := s.merged, s.Source.NumRows()
	var ids []int
	if s.filter != nil {
		// The filter appends to its rows on this goroutine, so the job gets a
		// copy of the ones it is to place.
		ids = slices.Clone(s.filter.rows[from:to])
	}
	if to <= sortChunk {
		s.apply(s.place(ids, from, to))
		return nil
	}
	s.pending = true
	return func() tea.Msg { return sortMsg{s, s.place(ids, from, to)} }
}
