package format

import (
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

// benchValues is a fixed spread of magnitudes and signs, so the benchmark is
// not dominated by one branch of a formatter's switch.
var (
	benchFloats = []Value{
		Float(0), Float(1.5), Float(-1.5), Float(1234.5678), Float(-9876.5432),
		Float(1e11), Float(-1e-9), Float(0.1), Null(),
	}
	benchInts = []Value{
		Int(0), Int(1), Int(-1), Int(1234), Int(-9876), Int(999999), Null(),
	}
	benchStrings = []Value{
		String("alpha"), String("a rather longer value than the column"),
		String("日本語のテキスト"), String(""), Null(),
	}
	benchTimes = []Value{
		Time(time.Unix(0, 0).UTC()),
		Time(time.Date(2024, 6, 15, 12, 34, 56, 0, time.UTC)),
		Null(),
	}
	benchBools = []Value{Bool(true), Bool(false), Null()}
)

// benchFormatter drives one formatter over a cycle of values, which is what
// rendering a column of the grid does.
func benchFormatter(b *testing.B, f Formatter, vals []Value) {
	b.Helper()
	b.ReportAllocs()

	var sink string
	i := 0
	for b.Loop() {
		sink = f.Format(vals[i%len(vals)])
		i++
	}
	if sink == "" && len(vals) > 0 {
		b.Fatal("empty output")
	}
}

func BenchmarkFloatFormat(b *testing.B) {
	benchFormatter(b, NewFloat(8, 3, ' ', "-", ".", "NaN", "∞"), benchFloats)
}

func BenchmarkEFloatFormat(b *testing.B) {
	benchFormatter(b, NewEFloat(2, 3, "-", ".", "E", "NaN", "∞"), benchFloats)
}

func BenchmarkIntFormat(b *testing.B) {
	benchFormatter(b, NewInt(8, ' ', "-"), benchInts)
}

func BenchmarkStrFormat(b *testing.B) {
	benchFormatter(b, NewStr(16, "…", ' ', 1.0, false), benchStrings)
}

func BenchmarkBoolFormat(b *testing.B) {
	benchFormatter(b, NewBool("TRUE", "FALSE", 1, true), benchBools)
}

func BenchmarkTimeFormat(b *testing.B) {
	benchFormatter(b, NewTime("ISO 8601 extended", "NaN"), benchTimes)
}

// arrowFloats builds a batch-sized Float64 array with a scattering of nulls.
func arrowFloats(n int) arrow.Array {
	bld := array.NewFloat64Builder(memory.NewGoAllocator())
	defer bld.Release()
	for i := range n {
		if i%37 == 0 {
			bld.AppendNull()
			continue
		}
		bld.Append(float64(i) * 1.25)
	}
	return bld.NewArray()
}

func arrowStrings(n int) arrow.Array {
	bld := array.NewStringBuilder(memory.NewGoAllocator())
	defer bld.Release()
	for i := range n {
		if i%37 == 0 {
			bld.AppendNull()
			continue
		}
		bld.Append("value number " + string(rune('a'+i%26)))
	}
	return bld.NewArray()
}

// BenchmarkAccumulate covers the per-batch statistics fold, which runs on every
// arriving record batch and decides the column widths.
func BenchmarkAccumulateFloat(b *testing.B) {
	arr := arrowFloats(1024)
	defer arr.Release()
	cfg := DefaultConfig()
	b.ReportAllocs()

	for b.Loop() {
		var s ColumnStats
		s.Accumulate(arr, cfg)
	}
}

func BenchmarkAccumulateString(b *testing.B) {
	arr := arrowStrings(1024)
	defer arr.Release()
	cfg := DefaultConfig()
	b.ReportAllocs()

	for b.Loop() {
		var s ColumnStats
		s.Accumulate(arr, cfg)
	}
}

// BenchmarkDefaultFormatter covers picking and sizing a formatter, which the
// view redoes whenever a batch lands.
func BenchmarkDefaultFormatter(b *testing.B) {
	arr := arrowFloats(1024)
	defer arr.Release()
	cfg := DefaultConfig()
	var s ColumnStats
	s.Accumulate(arr, cfg)
	b.ReportAllocs()

	var sink Formatter
	for b.Loop() {
		sink = DefaultFormatter(arrow.PrimitiveTypes.Float64, &s, cfg)
	}
	if sink == nil {
		b.Fatal("nil formatter")
	}
}
