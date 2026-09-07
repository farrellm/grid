package source

import (
	"context"
	"errors"
	"fmt"
	"io"

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
	*store   // the loaded batches
	*loader  // the demand-driven read loop
	*columns // the schema, statistics and formatters

	opts   ParquetOptions
	reader pqarrow.RecordReader
	pq     *file.Reader
	names  []string
}

// OpenParquet opens a Parquet file for reading. Cancelling ctx stops the
// background reading. Closing the source closes r if it is an io.Closer, which
// file.Reader.Close does for us.
func OpenParquet(ctx context.Context, r parquet.ReaderAtSeeker, opts ParquetOptions) (*Parquet, error) {
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

	rr, err := fr.GetRecordReader(ctx, nil, nil)
	if err != nil {
		pf.Close()
		return nil, fmt.Errorf("reading parquet: %w", err)
	}

	names := make([]string, len(sch.Fields()))
	for i, f := range sch.Fields() {
		names[i] = f.Name
	}

	p := &Parquet{
		store:   newStore(),
		loader:  newLoader(ctx),
		columns: newColumns(sch, opts.Config),
		opts:    opts,
		reader:  rr,
		pq:      pf,
		names:   names,
	}

	go p.pump(p.NumRows, p.finish, p.step)

	// Load one batch up front so the view has something to size itself from.
	p.Request(int(opts.BatchSize))
	if opts.Full {
		p.RequestAll()
		<-p.finished
	}
	return p, nil
}

// step reads one record batch for the loader, reporting whether to carry on.
// Unlike the CSV source there is no idle case: a file always has a next batch
// or an end.
func (p *Parquet) step() bool {
	if !p.reader.Next() {
		if err := p.reader.Err(); err != nil && !errors.Is(err, io.EOF) {
			p.finish(err)
		} else {
			p.finish(nil)
		}
		return false
	}
	rec := p.reader.RecordBatch()
	p.accumulate(rec)
	p.append(rec)
	return true
}

func (p *Parquet) Names() []string      { return p.names }
func (p *Parquet) NumCols() int         { return len(p.names) }
func (p *Parquet) Filename() string     { return p.opts.Filename }
func (p *Parquet) TitleLines() []string { return nil }

func (p *Parquet) Close() error {
	p.shutdown(nil)
	p.release()
	p.reader.Release()
	return p.pq.Close()
}
