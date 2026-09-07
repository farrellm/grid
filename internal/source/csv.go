package source

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/farrellm/grid/internal/format"
	"github.com/farrellm/grid/internal/schema"
)

// DefaultChunkSize is how many rows are parsed into one Arrow record batch.
// It also bounds the raw lines held for re-parsing after a type promotion.
const DefaultChunkSize = 1024

// readAhead is how far past the requested row the reader keeps going, matching
// ngrid's SAMPLELINES lookahead.
const readAhead = 200

// sampleGrace bounds how long the sample waits for the rest of its lines once
// the first has arrived. A file or a fast pipe fills the sample well inside it,
// so nothing changes there; a live stream is cut short rather than holding the
// display hostage until it has produced SampleSize lines. Inferring from fewer
// rows is safe because ingest promotes a column the moment a value does not
// fit.
const sampleGrace = 250 * time.Millisecond

// chunkGrace bounds how long a chunk waits for more lines before settling for
// what has arrived, so rows already sitting in a pipe are displayed instead of
// waiting on a chunk that may never fill.
const chunkGrace = 50 * time.Millisecond

// chunkResult reports what readChunk managed to do. Distinguishing a stream
// that has merely paused from one that has ended is what keeps a live pipe from
// being declared complete at its first quiet moment.
type chunkResult int

const (
	chunkRows chunkResult = iota // rows were added to the store
	chunkIdle                    // nothing available yet, but the input is open
	chunkEOF                     // the input has ended
)

// CSVOptions configures a delimited-text source.
type CSVOptions struct {
	Filename      string
	HasHeader     bool
	SampleSize    int
	Delimiter     rune // 0 means sniff
	CommentPrefix string
	NullValues    []string
	ChunkSize     int
	Config        format.Config
	// Full reads the entire input before returning from OpenCSV, so column
	// widths and precision are chosen from every value rather than a sample.
	Full bool
}

// CSV is a delimited-text source that loads incrementally on its own goroutine.
type CSV struct {
	*store

	opts   CSVOptions
	mem    memory.Allocator
	lines  *lineReader
	titles []string
	names  []string

	// schema and formatters change when a column is promoted, so both are
	// guarded.
	smu    sync.RWMutex
	schema *arrow.Schema
	stats  []*format.ColumnStats
	fmts   []format.Formatter

	wake   chan struct{}
	tmu    sync.Mutex
	target int

	closeOnce sync.Once
	stop      chan struct{} // closed by Close to request shutdown
	finished  chan struct{} // closed when the reader goroutine exits

	// all requests every remaining row, avoiding an overflowing row target.
	all atomic.Bool
}

// OpenCSV samples the head of r to sniff the delimiter and infer column types,
// then begins loading the rest in the background.
func OpenCSV(r io.Reader, opts CSVOptions) (*CSV, error) {
	if opts.SampleSize <= 0 {
		opts.SampleSize = 100
	}
	if opts.ChunkSize <= 0 {
		opts.ChunkSize = DefaultChunkSize
	}
	if opts.NullValues == nil {
		opts.NullValues = schema.DefaultNullValues
	}

	c := &CSV{
		store:    newStore(),
		opts:     opts,
		mem:      memory.NewGoAllocator(),
		lines:    newLineReader(r, opts.CommentPrefix),
		wake:     make(chan struct{}, 1),
		stop:     make(chan struct{}),
		finished: make(chan struct{}),
	}

	sample, err := c.readSample()
	if err != nil {
		return nil, err
	}
	if err := c.inferFrom(sample); err != nil {
		return nil, err
	}

	// The sample rows are data too; seed the store with them.
	if err := c.ingest(sample.Rows); err != nil {
		return nil, err
	}

	go c.run()

	if opts.Full {
		// Reading everything up front lets column widths and precision be
		// chosen from every value, which is what ngrid's --dataframe gave.
		c.RequestAll()
		<-c.finished
	}
	return c, nil
}

// readSample pulls the head of the stream: enough lines to sniff a delimiter
// and infer types from many rows rather than just the first.
func (c *CSV) readSample() (*schema.Sample, error) {
	// The first line is waited for without a deadline. A producer that thinks
	// before it speaks -- a database query, say -- must still get a full sample
	// once it starts, so the clock only begins when data does.
	var raw []string
	if line, ok := c.lines.next(); ok {
		raw = append(raw, line)
	}
	// From there the deadline is absolute rather than reset per line, so a
	// steady trickle cannot extend the sample indefinitely.
	deadline := time.Now().Add(sampleGrace)
	for len(raw) > 0 && len(raw) < c.opts.SampleSize+1 {
		line, ok, idle := c.lines.nextWithin(time.Until(deadline))
		if idle || !ok {
			break
		}
		raw = append(raw, line)
	}
	if err := c.lines.err(); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.New("no data")
	}

	delim := c.opts.Delimiter
	if delim == 0 {
		delim = schema.GuessDelimiter(raw, nil)
	}

	rows, err := schema.ParseRows(raw, delim)
	if err != nil {
		return nil, fmt.Errorf("parsing sample: %w", err)
	}
	if len(rows) == 0 {
		return nil, errors.New("no data")
	}

	s := &schema.Sample{
		Delimiter: delim,
		Rows:      rows,
		Comments:  c.lines.titles(),
	}
	if c.opts.HasHeader {
		s.Names = rows[0]
		s.Rows = rows[1:]
	} else {
		s.Names = schema.DefaultNames(len(rows[0]))
	}
	return s, nil
}

func (c *CSV) inferFrom(s *schema.Sample) error {
	c.opts.Delimiter = s.Delimiter
	c.names = s.Names
	c.titles = s.Comments

	c.smu.Lock()
	defer c.smu.Unlock()
	c.schema = schema.Infer(s, c.opts.NullValues)
	c.stats = make([]*format.ColumnStats, len(c.names))
	for i := range c.stats {
		c.stats[i] = &format.ColumnStats{}
	}
	return nil
}

// ingest converts rows into a record batch, widening column types and retrying
// if a value does not fit. This is the recovery ngrid left as a FIXME: a value
// past the sample that the sample did not predict.
func (c *CSV) ingest(rows [][]string) error {
	if len(rows) == 0 {
		return nil
	}
	for {
		c.smu.RLock()
		sch := c.schema
		c.smu.RUnlock()

		rec, err := buildRecord(c.mem, sch, rows, c.opts.NullValues)
		if err == nil {
			c.accumulate(rec)
			c.append(rec)
			rec.Release()
			return nil
		}

		var pe *parseError
		if !errors.As(err, &pe) {
			return err
		}
		if !c.promote(pe.Col) {
			// The column is already text and still failed; nothing more to try.
			return err
		}
	}
}

// promote widens one column and discards the formatters derived from the old
// type. It reports false when no further widening is possible.
func (c *CSV) promote(col int) bool {
	c.smu.Lock()
	defer c.smu.Unlock()

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
func (c *CSV) accumulate(rec arrow.RecordBatch) {
	c.smu.Lock()
	defer c.smu.Unlock()
	for i := 0; i < int(rec.NumCols()) && i < len(c.stats); i++ {
		c.stats[i].Accumulate(rec.Column(i), c.opts.Config)
	}
	c.fmts = nil // sizes may have grown
}

// run loads batches until the requested row count is met, then sleeps.
func (c *CSV) run() {
	defer close(c.finished)

	for {
		select {
		case <-c.wake:
		case <-c.stop:
			return
		}

		for {
			select {
			case <-c.stop:
				return
			default:
			}

			c.tmu.Lock()
			target := c.target
			c.tmu.Unlock()

			if !c.all.Load() && c.NumRows() >= target+readAhead {
				break
			}
			res, err := c.readChunk()
			if err != nil {
				c.finish(err)
				return
			}
			switch res {
			case chunkEOF:
				c.finish(nil)
				return
			case chunkIdle:
				// The producer has paused with the pipe still open. Block for
				// the next line rather than spinning on it or mistaking the
				// pause for the end of the input.
				c.lines.wait()
			}
		}
	}
}

// readChunk reads up to ChunkSize rows and adds them to the store, settling for
// fewer if the input goes quiet with rows in hand.
func (c *CSV) readChunk() (chunkResult, error) {
	// Track quote parity incrementally: a chunk may only end where we are
	// outside a quoted field, so a value containing newlines is never split.
	var raw []string
	quotes := 0
	eof := false
	for len(raw) < c.opts.ChunkSize || quotes%2 != 0 {
		var (
			line string
			ok   bool
			idle bool
		)
		if quotes%2 != 0 {
			// Mid-way through a quoted value: the rest of it is still coming,
			// and stopping here would tear the row in half. Wait it out.
			line, ok = c.lines.next()
		} else {
			line, ok, idle = c.lines.nextWithin(chunkGrace)
		}
		if idle {
			break
		}
		if !ok {
			eof = true
			break
		}
		quotes += strings.Count(line, `"`)
		raw = append(raw, line)
	}
	if err := c.lines.err(); err != nil {
		return chunkEOF, err
	}
	if len(raw) == 0 {
		if eof {
			return chunkEOF, nil
		}
		return chunkIdle, nil
	}

	rows, err := schema.ParseRows(raw, c.opts.Delimiter)
	if err != nil {
		return chunkEOF, err
	}
	if err := c.ingest(rows); err != nil {
		return chunkEOF, err
	}
	// Rows and the end of the input can arrive together; the next call sees the
	// closed input and reports it, which costs one cheap extra pass.
	return chunkRows, nil
}

// Request asks that at least n rows be loaded and returns immediately.
func (c *CSV) Request(n int) {
	c.tmu.Lock()
	if n > c.target {
		c.target = n
	}
	c.tmu.Unlock()
	c.poke()
}

// RequestAll asks for every remaining row, as the 'G' key does.
func (c *CSV) RequestAll() {
	c.all.Store(true)
	c.poke()
}

// StopAll cancels a RequestAll, returning to demand-driven reading. Leaving
// follow mode uses it so a live stream is not ingested for ever.
func (c *CSV) StopAll() { c.all.Store(false) }

func (c *CSV) poke() {
	select {
	case c.wake <- struct{}{}:
	default: // a wakeup is already pending
	}
}

func (c *CSV) Names() []string      { return c.names }
func (c *CSV) NumCols() int         { return len(c.names) }
func (c *CSV) Filename() string     { return c.opts.Filename }
func (c *CSV) TitleLines() []string { return c.titles }

// Formatters returns one formatter per column, rebuilt whenever the statistics
// or column types have changed.
func (c *CSV) Formatters() []format.Formatter {
	c.smu.Lock()
	defer c.smu.Unlock()

	if c.fmts != nil {
		return c.fmts
	}
	fields := c.schema.Fields()
	c.fmts = make([]format.Formatter, len(fields))
	for i, f := range fields {
		c.fmts[i] = format.DefaultFormatter(f.Type, c.stats[i], c.opts.Config)
	}
	return c.fmts
}

// SetFormatter replaces one column's formatter, for interactive width and
// precision changes.
func (c *CSV) SetFormatter(col int, f format.Formatter) {
	c.smu.Lock()
	defer c.smu.Unlock()
	if c.fmts == nil || col < 0 || col >= len(c.fmts) {
		return
	}
	c.fmts[col] = f
}

func (c *CSV) Close() error {
	c.closeOnce.Do(func() {
		close(c.stop)
		// Release the reader if it is parked waiting on a stream that has gone
		// quiet, so waiting for it below cannot deadlock.
		c.lines.close()
		<-c.finished // let the reader drop its reference to the batches
	})
	c.release()
	return nil
}
