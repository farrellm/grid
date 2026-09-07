package source

import (
	"sync"
	"sync/atomic"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/farrellm/grid/internal/format"
	"github.com/farrellm/grid/internal/schema"
)

// loader carries the demand-driven read loop that both sources run: how many
// rows have been asked for, how the reading goroutine is woken, and how it is
// shut down. Embedding it supplies Request, RequestAll and StopAll, three of the
// Source methods.
//
// Reading is demand-driven so that a huge file or an endless stream opens
// instantly: the loop sleeps until the view asks for rows it does not have.
type loader struct {
	wake chan struct{}

	mu     sync.Mutex
	target int

	// all requests every remaining row, avoiding an overflowing row target.
	all atomic.Bool

	closeOnce sync.Once
	stop      chan struct{} // closed by Close to request shutdown
	finished  chan struct{} // closed when the reading goroutine exits
}

func newLoader() *loader {
	return &loader{
		wake:     make(chan struct{}, 1),
		stop:     make(chan struct{}),
		finished: make(chan struct{}),
	}
}

// Request asks that at least n rows be loaded and returns immediately.
func (l *loader) Request(n int) {
	l.mu.Lock()
	if n > l.target {
		l.target = n
	}
	l.mu.Unlock()
	l.poke()
}

// RequestAll asks for every remaining row, as the 'G' key does.
func (l *loader) RequestAll() {
	l.all.Store(true)
	l.poke()
}

// StopAll cancels a RequestAll, returning to demand-driven reading. Leaving
// follow mode uses it so a live stream is not ingested for ever.
func (l *loader) StopAll() { l.all.Store(false) }

func (l *loader) poke() {
	select {
	case l.wake <- struct{}{}:
	default: // a wakeup is already pending
	}
}

// satisfied reports whether the rows already loaded meet the demand, keeping
// readAhead rows past what was asked for so scrolling does not stall.
func (l *loader) satisfied(loaded int) bool {
	if l.all.Load() {
		return false
	}
	l.mu.Lock()
	target := l.target
	l.mu.Unlock()
	return loaded >= target+readAhead
}

// pump runs the read loop on its own goroutine: sleep until woken, then call
// step until the demand is met. step reports whether reading should carry on,
// and is responsible for calling finish when it says no.
func (l *loader) pump(loaded func() int, step func() bool) {
	defer close(l.finished)

	for {
		select {
		case <-l.wake:
		case <-l.stop:
			return
		}

		for {
			select {
			case <-l.stop:
				return
			default:
			}

			if l.satisfied(loaded()) {
				break
			}
			if !step() {
				return
			}
		}
	}
}

// shutdown asks the reading goroutine to stop and waits for it, so that it has
// dropped its references to the record batches before they are released. release
// runs in between, to free a reader parked on a stream that has gone quiet;
// without it, waiting here could block for ever.
func (l *loader) shutdown(release func()) {
	l.closeOnce.Do(func() {
		close(l.stop)
		if release != nil {
			release()
		}
		<-l.finished
	})
}

// columns owns the per-column schema, statistics and cached formatters.
//
// The three live under one lock because they change together: promoting a
// column rewrites the schema and invalidates the statistics gathered under the
// narrower type. Splitting them would let Formatters observe a promoted schema
// paired with stale statistics, and size a column from values it no longer
// holds.
//
// Embedding it supplies Formatters and SetFormatter, two more Source methods.
type columns struct {
	mu     sync.Mutex
	cfg    format.Config
	schema *arrow.Schema
	stats  []*format.ColumnStats
	fmts   []format.Formatter // nil when it must be rebuilt
}

func newColumns(sch *arrow.Schema, cfg format.Config) *columns {
	stats := make([]*format.ColumnStats, len(sch.Fields()))
	for i := range stats {
		stats[i] = &format.ColumnStats{}
	}
	return &columns{cfg: cfg, schema: sch, stats: stats}
}

// currentSchema returns the schema to build the next batch under.
func (c *columns) currentSchema() *arrow.Schema {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.schema
}

// promote widens one column and discards the formatters and statistics derived
// from the old type. It reports false when no further widening is possible.
func (c *columns) promote(col int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	next, ok := schema.PromoteSchema(c.schema, col)
	if !ok {
		return false
	}
	c.schema = next
	// Statistics gathered under the narrower type no longer describe the
	// column; start it over.
	c.stats[col] = &format.ColumnStats{}
	c.fmts = nil
	return true
}

// accumulate folds a batch into the per-column statistics that size formatters.
func (c *columns) accumulate(rec arrow.RecordBatch) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := 0; i < int(rec.NumCols()) && i < len(c.stats); i++ {
		c.stats[i].Accumulate(rec.Column(i), c.cfg)
	}
	c.fmts = nil // sizes may have grown
}

// Formatters returns one formatter per column, rebuilt whenever the statistics
// or column types have changed.
func (c *columns) Formatters() []format.Formatter {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.fmts != nil {
		return c.fmts
	}
	fields := c.schema.Fields()
	c.fmts = make([]format.Formatter, len(fields))
	for i, f := range fields {
		c.fmts[i] = format.DefaultFormatter(f.Type, c.stats[i], c.cfg)
	}
	return c.fmts
}

// SetFormatter replaces one column's formatter, for interactive width and
// precision changes.
func (c *columns) SetFormatter(col int, f format.Formatter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fmts == nil || col < 0 || col >= len(c.fmts) {
		return
	}
	c.fmts[col] = f
}
