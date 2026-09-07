package source

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"

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
	var raw []string
	for len(raw) < c.opts.SampleSize+1 {
		line, ok := c.lines.next()
		if !ok {
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
			more, err := c.readChunk()
			if err != nil {
				c.finish(err)
				return
			}
			if !more {
				c.finish(nil)
				return
			}
		}
	}
}

// readChunk reads up to ChunkSize rows and adds them to the store.
func (c *CSV) readChunk() (bool, error) {
	// Track quote parity incrementally: a chunk may only end where we are
	// outside a quoted field, so a value containing newlines is never split.
	var raw []string
	quotes := 0
	for len(raw) < c.opts.ChunkSize || quotes%2 != 0 {
		line, ok := c.lines.next()
		if !ok {
			break
		}
		quotes += strings.Count(line, `"`)
		raw = append(raw, line)
	}
	if err := c.lines.err(); err != nil {
		return false, err
	}
	if len(raw) == 0 {
		return false, nil
	}

	rows, err := schema.ParseRows(raw, c.opts.Delimiter)
	if err != nil {
		return false, err
	}
	if err := c.ingest(rows); err != nil {
		return false, err
	}
	return true, nil
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
		<-c.finished // let the reader drop its reference to the batches
	})
	c.release()
	return nil
}
