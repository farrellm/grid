package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/farrellm/grid/internal/format"
)

// ParquetOptions configures a Parquet source.
type ParquetOptions struct {
	Filename  string
	BatchSize int64
	Config    format.Config
	Full      bool
}

// Parquet reads a Parquet file into the same Arrow batches the CSV source
// produces, so the view and formatters need no special case for it.
//
// Parquet is columnar and self-describing, so there is no type guessing here:
// the file states its schema. It also needs random access, so unlike CSV it
// cannot be read from a pipe.
type Parquet struct {
	*store

	opts   ParquetOptions
	mem    memory.Allocator
	reader pqarrow.RecordReader
	pq     *file.Reader

	schema *arrow.Schema
	names  []string

	smu   sync.Mutex
	stats []*format.ColumnStats
	fmts  []format.Formatter

	wake   chan struct{}
	tmu    sync.Mutex
	target int
	all    atomic.Bool

	closeOnce sync.Once
	stop      chan struct{}
	finished  chan struct{}
}

// OpenParquet opens a Parquet file for reading. Closing the source closes r if
// it is an io.Closer, which file.Reader.Close does for us.
func OpenParquet(r parquet.ReaderAtSeeker, opts ParquetOptions) (*Parquet, error) {
	if opts.BatchSize <= 0 {
		opts.BatchSize = DefaultChunkSize
	}

	pf, err := file.NewParquetReader(r)
	if err != nil {
		return nil, fmt.Errorf("reading parquet: %w", err)
	}

	mem := memory.NewGoAllocator()
	fr, err := pqarrow.NewFileReader(pf, pqarrow.ArrowReadProperties{
		BatchSize: opts.BatchSize,
	}, mem)
	if err != nil {
		pf.Close()
		return nil, fmt.Errorf("reading parquet: %w", err)
	}

	sch, err := fr.Schema()
	if err != nil {
		pf.Close()
		return nil, fmt.Errorf("reading parquet schema: %w", err)
	}

	rr, err := fr.GetRecordReader(context.Background(), nil, nil)
	if err != nil {
		pf.Close()
		return nil, fmt.Errorf("reading parquet: %w", err)
	}

	names := make([]string, len(sch.Fields()))
	for i, f := range sch.Fields() {
		names[i] = f.Name
	}

	p := &Parquet{
		store:    newStore(),
		opts:     opts,
		mem:      mem,
		reader:   rr,
		pq:       pf,
		schema:   sch,
		names:    names,
		stats:    make([]*format.ColumnStats, len(names)),
		wake:     make(chan struct{}, 1),
		stop:     make(chan struct{}),
		finished: make(chan struct{}),
	}
	for i := range p.stats {
		p.stats[i] = &format.ColumnStats{}
	}

	go p.run()

	// Load one batch up front so the view has something to size itself from.
	p.Request(int(opts.BatchSize))
	if opts.Full {
		p.RequestAll()
		<-p.finished
	}
	return p, nil
}

func (p *Parquet) run() {
	defer close(p.finished)

	for {
		select {
		case <-p.wake:
		case <-p.stop:
			return
		}

		for {
			select {
			case <-p.stop:
				return
			default:
			}

			p.tmu.Lock()
			target := p.target
			p.tmu.Unlock()

			if !p.all.Load() && p.NumRows() >= target+readAhead {
				break
			}
			if !p.reader.Next() {
				if err := p.reader.Err(); err != nil && !errors.Is(err, io.EOF) {
					p.finish(err)
				} else {
					p.finish(nil)
				}
				return
			}
			rec := p.reader.RecordBatch()
			p.accumulate(rec)
			p.append(rec)
		}
	}
}

func (p *Parquet) accumulate(rec arrow.RecordBatch) {
	p.smu.Lock()
	defer p.smu.Unlock()
	for i := 0; i < int(rec.NumCols()) && i < len(p.stats); i++ {
		p.stats[i].Accumulate(rec.Column(i), p.opts.Config)
	}
	p.fmts = nil
}

// Request asks that at least n rows be loaded and returns immediately.
func (p *Parquet) Request(n int) {
	p.tmu.Lock()
	if n > p.target {
		p.target = n
	}
	p.tmu.Unlock()
	p.poke()
}

// RequestAll asks for every remaining row.
func (p *Parquet) RequestAll() {
	p.all.Store(true)
	p.poke()
}

func (p *Parquet) poke() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

func (p *Parquet) Names() []string      { return p.names }
func (p *Parquet) NumCols() int         { return len(p.names) }
func (p *Parquet) Filename() string     { return p.opts.Filename }
func (p *Parquet) TitleLines() []string { return nil }

func (p *Parquet) Formatters() []format.Formatter {
	p.smu.Lock()
	defer p.smu.Unlock()

	if p.fmts != nil {
		return p.fmts
	}
	fields := p.schema.Fields()
	p.fmts = make([]format.Formatter, len(fields))
	for i, f := range fields {
		p.fmts[i] = format.DefaultFormatter(f.Type, p.stats[i], p.opts.Config)
	}
	return p.fmts
}

func (p *Parquet) SetFormatter(col int, f format.Formatter) {
	p.smu.Lock()
	defer p.smu.Unlock()
	if p.fmts == nil || col < 0 || col >= len(p.fmts) {
		return
	}
	p.fmts[col] = f
}

func (p *Parquet) Close() error {
	p.closeOnce.Do(func() {
		close(p.stop)
		<-p.finished
	})
	p.release()
	p.reader.Release()
	return p.pq.Close()
}
