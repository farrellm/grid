package source

import (
	"sync"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/farrellm/grid/internal/format"
)

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
	return format.FromArrow(rec.Column(col), off)
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
