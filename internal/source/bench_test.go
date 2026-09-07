package source

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/farrellm/grid/internal/format"
	"github.com/farrellm/grid/internal/schema"
)

// genCSV builds a deterministic CSV covering every type the inferrer can pick:
// integer, float, boolean, text and timestamp. The mix matters, because the
// per-cell cost is dominated by the type switch that decodes it.
func genCSV(rows int) string {
	var b strings.Builder
	b.Grow(rows * 64)
	b.WriteString("n,x,flag,word,when\n")
	for i := range rows {
		fmt.Fprintf(&b, "%d,%d.%02d,%t,word%d,2024-01-%02dT12:00:00Z\n",
			i, i%1000, i%100, i%2 == 0, i%997, i%28+1)
	}
	return b.String()
}

// loadCSV opens a source and reads it to completion.
func loadCSV(tb testing.TB, data string) *CSV {
	tb.Helper()
	c, err := OpenCSV(context.Background(), strings.NewReader(data), CSVOptions{
		Filename:  "bench.csv",
		HasHeader: true,
		Config:    format.DefaultConfig(),
	})
	if err != nil {
		tb.Fatalf("OpenCSV: %v", err)
	}
	tb.Cleanup(func() { c.Close() })

	c.RequestAll()
	select {
	case <-c.finished:
	case <-time.After(30 * time.Second):
		tb.Fatal("timed out loading")
	}
	if err := c.Err(); err != nil {
		tb.Fatalf("load: %v", err)
	}
	return c
}

// BenchmarkOpenCSVFull covers the whole ingest path: sniffing the delimiter,
// inferring the schema, parsing every line and building the Arrow batches.
func BenchmarkOpenCSVFull(b *testing.B) {
	data := genCSV(20000)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))

	for b.Loop() {
		c, err := OpenCSV(context.Background(), strings.NewReader(data), CSVOptions{
			Filename:  "bench.csv",
			HasHeader: true,
			Config:    format.DefaultConfig(),
			Full:      true,
		})
		if err != nil {
			b.Fatalf("OpenCSV: %v", err)
		}
		if c.NumRows() != 20000 {
			b.Fatalf("NumRows() = %d, want 20000", c.NumRows())
		}
		c.Close()
	}
}

func BenchmarkParseRows(b *testing.B) {
	lines := strings.Split(strings.TrimSuffix(genCSV(1024), "\n"), "\n")[1:]
	b.ReportAllocs()

	for b.Loop() {
		if _, err := schema.ParseRows(lines, ','); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildRecord(b *testing.B) {
	lines := strings.Split(strings.TrimSuffix(genCSV(1024), "\n"), "\n")
	rows, err := schema.ParseRows(lines, ',')
	if err != nil {
		b.Fatal(err)
	}
	sch := schema.Infer(&schema.Sample{
		Delimiter: ',',
		Names:     rows[0],
		Rows:      rows[1:],
	}, schema.DefaultNullValues)

	mem := memory.NewGoAllocator()
	data := rows[1:]
	b.ReportAllocs()

	for b.Loop() {
		rec, err := buildRecord(mem, sch, data, schema.DefaultNullValues)
		if err != nil {
			b.Fatal(err)
		}
		rec.Release()
	}
}

// BenchmarkStoreValue reads cells in a scattered order, so the binary search in
// locate is exercised rather than one hot chunk.
func BenchmarkStoreValue(b *testing.B) {
	c := loadCSV(b, genCSV(20000))
	n, cols := c.NumRows(), c.NumCols()

	var sink format.Value
	b.ReportAllocs()

	i := 0
	for b.Loop() {
		// A stride coprime with n walks every row without repeating.
		row := (i * 7919) % n
		sink = c.Value(row, i%cols)
		i++
	}
	if sink.Kind == format.KindNull && n > 0 {
		b.Fatal("unexpected null")
	}
}

func BenchmarkGuessDelimiter(b *testing.B) {
	lines := strings.Split(strings.TrimSuffix(genCSV(100), "\n"), "\n")
	b.ReportAllocs()

	for b.Loop() {
		if d := schema.GuessDelimiter(lines, nil); d != ',' {
			b.Fatalf("GuessDelimiter() = %q", d)
		}
	}
}

func BenchmarkInfer(b *testing.B) {
	lines := strings.Split(strings.TrimSuffix(genCSV(100), "\n"), "\n")
	rows, err := schema.ParseRows(lines, ',')
	if err != nil {
		b.Fatal(err)
	}
	s := &schema.Sample{Delimiter: ',', Names: rows[0], Rows: rows[1:]}
	b.ReportAllocs()

	var sink *arrow.Schema
	for b.Loop() {
		sink = schema.Infer(s, schema.DefaultNullValues)
	}
	if sink == nil {
		b.Fatal("nil schema")
	}
}
