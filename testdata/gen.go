//go:build ignore

// Command gen regenerates the fixtures in testdata.
//
// Run it with `make fixtures`, or `go run testdata/gen.go` from the repo root.
// The columns mirror ngrid's test/sample0.py — flags, normals, exponentials,
// counts, words, signed decimals and timestamps — so the two can be compared
// side by side. The seed is fixed, so the output is reproducible.
package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

const letters = "abcdefghijklmnopqrstuvwxyz"

func word(r *rand.Rand) string {
	n := 3 + r.Intn(10)
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[r.Intn(len(letters))]
	}
	return string(b)
}

func main() {
	const n = 2000
	r := rand.New(rand.NewSource(20140917))
	base := time.Date(2014, 9, 17, 10, 7, 53, 0, time.UTC)

	names := []string{"flag0", "flag1", "normal0", "normal1", "exp0", "expexp0",
		"count0", "count1", "word0", "word1", "sig2", "negsig3", "when"}

	var csv strings.Builder
	csv.WriteString(strings.Join(names, ",") + "\n")

	mem := memory.NewGoAllocator()
	sch := arrow.NewSchema([]arrow.Field{
		{Name: "flag0", Type: arrow.FixedWidthTypes.Boolean, Nullable: true},
		{Name: "flag1", Type: arrow.FixedWidthTypes.Boolean, Nullable: true},
		{Name: "normal0", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "normal1", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "exp0", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "expexp0", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "count0", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "count1", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "word0", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "word1", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "sig2", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "negsig3", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "when", Type: arrow.FixedWidthTypes.Timestamp_us, Nullable: true},
	}, nil)
	rb := array.NewRecordBuilder(mem, sch)
	defer rb.Release()

	for i := range n {
		f0, f1 := r.Intn(2) == 0, r.Intn(2) == 0
		nm0, nm1 := r.NormFloat64(), r.NormFloat64()
		e0 := math.Exp(r.Float64() * 20)
		ee0 := math.Exp(math.Exp(r.Float64() * 5))
		c0 := int64(math.Exp(r.Float64() * 12))
		c1 := int64(r.Intn(1000))
		w0, w1 := word(r), word(r)
		s2 := float64(r.Intn(100)) / 10
		ns3 := float64(r.Intn(2000)-1000) / 100
		when := base.Add(time.Duration(i) * time.Minute)

		rb.Field(0).(*array.BooleanBuilder).Append(f0)
		rb.Field(1).(*array.BooleanBuilder).Append(f1)
		rb.Field(2).(*array.Float64Builder).Append(nm0)
		rb.Field(3).(*array.Float64Builder).Append(nm1)
		rb.Field(4).(*array.Float64Builder).Append(e0)
		rb.Field(5).(*array.Float64Builder).Append(ee0)
		rb.Field(6).(*array.Int64Builder).Append(c0)
		rb.Field(7).(*array.Int64Builder).Append(c1)
		rb.Field(8).(*array.StringBuilder).Append(w0)
		rb.Field(9).(*array.StringBuilder).Append(w1)
		rb.Field(10).(*array.Float64Builder).Append(s2)
		rb.Field(11).(*array.Float64Builder).Append(ns3)
		ts, _ := arrow.TimestampFromTime(when, arrow.Microsecond)
		rb.Field(12).(*array.TimestampBuilder).Append(ts)

		fmt.Fprintf(&csv, "%t,%t,%.6f,%.6f,%.6f,%.6f,%d,%d,%s,%s,%.1f,%.2f,%s\n",
			f0, f1, nm0, nm1, e0, ee0, c0, c1, w0, w1, s2, ns3,
			when.Format(time.RFC3339))
	}

	must(os.WriteFile("testdata/sample.csv", []byte(csv.String()), 0o644))

	rec := rb.NewRecordBatch()
	defer rec.Release()
	pf, err := os.Create("testdata/sample.parquet")
	must(err)
	w, err := pqarrow.NewFileWriter(sch, pf,
		parquet.NewWriterProperties(parquet.WithAllocator(mem)), pqarrow.DefaultWriterProps())
	must(err)
	must(w.Write(rec))
	must(w.Close())

	// A file whose comment header crashes ngrid, and whose types only settle
	// after the sample window.
	var wide strings.Builder
	wide.WriteString("# grid test fixture\n# second comment line\n")
	wide.WriteString("n,mixed,late,text\n")
	for i := range 500 {
		mixed, late := fmt.Sprint(i), fmt.Sprint(i)
		if i == 400 {
			mixed = "1.5" // int column widens to float, past the sample
		}
		if i == 450 {
			late = "oops" // and this one all the way to text
		}
		if i%7 == 0 {
			late = "" // blanks must stay null, not widen the column
		}
		fmt.Fprintf(&wide, "%d,%s,%s, spaced %d \n", i, mixed, late, i)
	}
	must(os.WriteFile("testdata/widening.csv", []byte(wide.String()), 0o644))

	fmt.Println("wrote testdata/sample.csv, sample.parquet, widening.csv")
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
