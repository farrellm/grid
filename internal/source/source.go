// Package source loads tabular data into Arrow record batches and serves cells
// to the view.
//
// Sources load incrementally: NumRows reports what is available now, and Done
// reports whether more may still arrive. This is what lets a huge or endless
// stream open instantly, as ngrid's DelimitedFileModel did, but here the
// reading happens on its own goroutine so a slow pipe never blocks the UI.
package source

import (
	"sync"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/farrellm/grid/internal/format"
)

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
	// SetFormatter overrides one column's formatter.
	SetFormatter(col int, f format.Formatter)
	// Ready returns a channel that receives whenever new rows land.
	Ready() <-chan struct{}
	// Close releases held memory and stops loading.
	Close() error
}

// store holds loaded record batches and maps global row indices onto them.
// Batches may have differing schemas: when a column turns out to be wider than
// the sample suggested, later batches carry the promoted type while earlier
// ones keep the narrower one. Value type-switches per batch, so both render.
type store struct {
	mu     sync.RWMutex
	chunks []arrow.RecordBatch
	starts []int // starts[i] is the global index of chunk i's first row
	nrows  int
	done   bool
	err    error
	ready  chan struct{}
}

func newStore() *store {
	return &store{ready: make(chan struct{}, 1)}
}

func (s *store) append(rec arrow.RecordBatch) {
	rec.Retain()
	s.mu.Lock()
	s.chunks = append(s.chunks, rec)
	s.starts = append(s.starts, s.nrows)
	s.nrows += int(rec.NumRows())
	s.mu.Unlock()
	s.signal()
}

func (s *store) signal() {
	select {
	case s.ready <- struct{}{}:
	default: // a wakeup is already pending; coalesce
	}
}

func (s *store) Ready() <-chan struct{} { return s.ready }

func (s *store) NumRows() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nrows
}

func (s *store) Done() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.done
}

func (s *store) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

func (s *store) finish(err error) {
	s.mu.Lock()
	s.done = true
	if err != nil && s.err == nil {
		s.err = err
	}
	s.mu.Unlock()
	s.signal()
}

// locate finds the batch holding a global row index.
func (s *store) locate(row int) (arrow.RecordBatch, int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if row < 0 || row >= s.nrows {
		return nil, 0, false
	}
	// Binary search over chunk start offsets.
	lo, hi := 0, len(s.starts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if s.starts[mid] <= row {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return s.chunks[lo], row - s.starts[lo], true
}

func (s *store) Value(row, col int) format.Value {
	rec, off, ok := s.locate(row)
	if !ok || col < 0 || col >= int(rec.NumCols()) {
		return format.Null()
	}
	return valueAt(rec.Column(col), off)
}

func (s *store) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.chunks {
		c.Release()
	}
	s.chunks = nil
	s.starts = nil
	s.nrows = 0
}

// valueAt converts one element of an Arrow array into a format.Value.
func valueAt(arr arrow.Array, i int) format.Value {
	if arr == nil || i < 0 || i >= arr.Len() || arr.IsNull(i) {
		return format.Null()
	}
	switch a := arr.(type) {
	case *array.Boolean:
		return format.Bool(a.Value(i))
	case *array.Int8:
		return format.Int(int64(a.Value(i)))
	case *array.Int16:
		return format.Int(int64(a.Value(i)))
	case *array.Int32:
		return format.Int(int64(a.Value(i)))
	case *array.Int64:
		return format.Int(a.Value(i))
	case *array.Uint8:
		return format.Int(int64(a.Value(i)))
	case *array.Uint16:
		return format.Int(int64(a.Value(i)))
	case *array.Uint32:
		return format.Int(int64(a.Value(i)))
	case *array.Uint64:
		return format.Int(int64(a.Value(i)))
	case *array.Float16:
		return format.Float(float64(a.Value(i).Float32()))
	case *array.Float32:
		return format.Float(float64(a.Value(i)))
	case *array.Float64:
		return format.Float(a.Value(i))
	case *array.String:
		return format.String(a.Value(i))
	case *array.LargeString:
		return format.String(a.Value(i))
	case *array.Binary:
		return format.String(string(a.Value(i)))
	case *array.Date32:
		return format.Time(a.Value(i).ToTime())
	case *array.Date64:
		return format.Time(a.Value(i).ToTime())
	case *array.Timestamp:
		unit := arrow.Microsecond
		if tt, ok := a.DataType().(*arrow.TimestampType); ok {
			unit = tt.Unit
		}
		return format.Time(a.Value(i).ToTime(unit))
	case *array.Time32:
		unit := arrow.Second
		if tt, ok := a.DataType().(*arrow.Time32Type); ok {
			unit = tt.Unit
		}
		return format.Time(a.Value(i).ToTime(unit))
	case *array.Time64:
		unit := arrow.Microsecond
		if tt, ok := a.DataType().(*arrow.Time64Type); ok {
			unit = tt.Unit
		}
		return format.Time(a.Value(i).ToTime(unit))
	default:
		return format.String(arr.ValueStr(i))
	}
}
