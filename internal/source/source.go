// Package source loads tabular data into Arrow record batches and serves cells
// to the view.
//
// Sources load incrementally: NumRows reports what is available now, and Done
// reports whether more may still arrive. This is what lets a huge or endless
// stream open instantly, as ngrid's DelimitedFileModel did, but here the
// reading happens on its own goroutine so a slow pipe never blocks the UI.
package source

import "github.com/farrellm/grid/internal/format"

// Source is a table of cells that may still be loading.
type Source interface {
	// Names returns the column headings.
	Names() []string
	// NumCols returns the column count.
	NumCols() int
	// NumRows returns the number of rows loaded so far.
	NumRows() int
	// Value returns one cell, or a null Value if it is out of range.
	Value(row, col int) format.Value
	// Done reports whether every row has been loaded.
	Done() bool
	// Err returns the first fatal error, if any.
	Err() error
	// Filename names the input, for the status bar.
	Filename() string
	// TitleLines returns comment lines found above the data.
	TitleLines() []string
	// Formatters returns one formatter per column, sized from what has been
	// seen so far.
	Formatters() []format.Formatter
	// Request asks that at least n rows be loaded, returning immediately.
	Request(n int)
	// RequestAll asks for every remaining row.
	RequestAll()
	// StopAll cancels a RequestAll, returning to demand-driven reading.
	StopAll()
	// SetFormatter overrides one column's formatter.
	SetFormatter(col int, f format.Formatter)
	// Ready returns a channel that receives whenever new rows land.
	Ready() <-chan struct{}
	// Close releases held memory and stops loading.
	Close() error
}
