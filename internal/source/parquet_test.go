package source

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/farrellm/grid/internal/format"
)

// Both sources must satisfy the same interface, so the view needs no special
// case for either.
var (
	_ Source = (*CSV)(nil)
	_ Source = (*Parquet)(nil)
)

// writeParquet builds a small file covering the types the viewer must render.
func writeParquet(t *testing.T, rows int) string {
	t.Helper()

	mem := memory.NewGoAllocator()
	sch := arrow.NewSchema([]arrow.Field{
		{Name: "n", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "x", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "flag", Type: arrow.FixedWidthTypes.Boolean, Nullable: true},
		{Name: "word", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "when", Type: arrow.FixedWidthTypes.Timestamp_us, Nullable: true},
	}, nil)

	rb := array.NewRecordBuilder(mem, sch)
	defer rb.Release()

	base := time.Date(2014, 9, 17, 10, 7, 53, 0, time.UTC)
	for i := 0; i < rows; i++ {
		rb.Field(0).(*array.Int64Builder).Append(int64(i))
		rb.Field(1).(*array.Float64Builder).Append(float64(i) + 0.5)
		rb.Field(2).(*array.BooleanBuilder).Append(i%2 == 0)
		rb.Field(3).(*array.StringBuilder).Append("word")
		ts, _ := arrow.TimestampFromTime(base.Add(time.Duration(i)*time.Hour), arrow.Microsecond)
		rb.Field(4).(*array.TimestampBuilder).Append(ts)
	}
	rec := rb.NewRecordBatch()
	defer rec.Release()

	path := filepath.Join(t.TempDir(), "sample.parquet")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w, err := pqarrow.NewFileWriter(sch, f,
		parquet.NewWriterProperties(parquet.WithAllocator(mem)),
		pqarrow.DefaultWriterProps())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(rec); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	// pqarrow's writer closes f as part of Close.
	return path
}

func openParquet(t *testing.T, path string, full bool) *Parquet {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := OpenParquet(f, ParquetOptions{
		Filename: path,
		Config:   format.DefaultConfig(),
		Full:     full,
	})
	if err != nil {
		t.Fatalf("OpenParquet: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

func TestParquetReadsTypedColumns(t *testing.T) {
	p := openParquet(t, writeParquet(t, 50), true)

	if got := p.NumRows(); got != 50 {
		t.Fatalf("NumRows() = %d, want 50", got)
	}
	if got := p.NumCols(); got != 5 {
		t.Fatalf("NumCols() = %d, want 5", got)
	}

	if v := p.Value(3, 0); v.Kind != format.KindInt || v.I != 3 {
		t.Errorf("Value(3,0) = %+v, want int 3", v)
	}
	if v := p.Value(3, 1); v.Kind != format.KindFloat || v.F != 3.5 {
		t.Errorf("Value(3,1) = %+v, want float 3.5", v)
	}
	if v := p.Value(2, 2); v.Kind != format.KindBool || !v.B {
		t.Errorf("Value(2,2) = %+v, want bool true", v)
	}
	if v := p.Value(0, 3); v.Kind != format.KindString || v.S != "word" {
		t.Errorf("Value(0,3) = %+v, want string word", v)
	}
	// Timestamps arrive already typed; the CSV path has to infer them.
	if v := p.Value(0, 4); v.Kind != format.KindTime || v.T.Year() != 2014 {
		t.Errorf("Value(0,4) = %+v, want a 2014 timestamp", v)
	}
}

// Parquet and CSV must produce the same kind of formatter for the same data,
// since the view treats them identically.
func TestParquetFormattersMatchTypes(t *testing.T) {
	p := openParquet(t, writeParquet(t, 50), true)
	fmts := p.Formatters()

	if _, ok := fmts[0].(*format.IntFormatter); !ok {
		t.Errorf("column n: got %T, want *format.IntFormatter", fmts[0])
	}
	if _, ok := fmts[1].(*format.FloatFormatter); !ok {
		t.Errorf("column x: got %T, want *format.FloatFormatter", fmts[1])
	}
	if _, ok := fmts[2].(*format.BoolFormatter); !ok {
		t.Errorf("column flag: got %T, want *format.BoolFormatter", fmts[2])
	}
	if _, ok := fmts[3].(*format.StrFormatter); !ok {
		t.Errorf("column word: got %T, want *format.StrFormatter", fmts[3])
	}
	if _, ok := fmts[4].(*format.TimeFormatter); !ok {
		t.Errorf("column when: got %T, want *format.TimeFormatter", fmts[4])
	}
}

func TestParquetRejectsNonParquet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not.parquet")
	if err := os.WriteFile(path, []byte("a,b\n1,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err := OpenParquet(f, ParquetOptions{Filename: path}); err == nil {
		t.Error("OpenParquet on a CSV file: want an error")
	}
}
