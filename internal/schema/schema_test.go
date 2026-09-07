package schema

import (
	"strings"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
)

func TestGuessDelimiter(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  rune
	}{
		{"comma", []string{"a,b,c", "1,2,3", "4,5,6"}, ','},
		{"tab", []string{"a\tb\tc", "1\t2\t3"}, '\t'},
		{"pipe", []string{"a|b|c", "1|2|3"}, '|'},
		{"space", []string{"a b c", "1 2 3"}, ' '},
		// A comma column count of 3 beats the single field the other
		// delimiters see, even though the values contain spaces.
		{"comma with spaces in values", []string{"a,b c,d", "1,2 3,4"}, ','},
		// Ragged under commas, uniform under pipes.
		{"ragged comma", []string{"a|b", "1,2|3"}, '|'},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GuessDelimiter(tt.lines, nil); got != tt.want {
				t.Errorf("GuessDelimiter() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGuessType(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   arrow.DataType
	}{
		{"bools", []string{"true", "false", "TRUE"}, arrow.FixedWidthTypes.Boolean},
		{"ints", []string{"1", "2", "-3"}, arrow.PrimitiveTypes.Int64},
		{"floats", []string{"1", "2.5", "-3"}, arrow.PrimitiveTypes.Float64},
		{"scientific", []string{"1e10", "2.5e-3"}, arrow.PrimitiveTypes.Float64},
		{"strings", []string{"a", "b", "1"}, arrow.BinaryTypes.String},
		{"dates", []string{"2014-09-17", "2020-01-01"}, arrow.FixedWidthTypes.Date32},
		{"timestamps", []string{"2014-09-17T10:07:53Z"}, arrow.FixedWidthTypes.Timestamp_us},
		// 0/1 are integers, not booleans: only "true"/"false" are boolean.
		{"zero one", []string{"0", "1"}, arrow.PrimitiveTypes.Int64},
		// An all-empty column has no evidence; strings render blank.
		{"all null", []string{"", "", ""}, arrow.BinaryTypes.String},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GuessType(tt.values, DefaultNullValues)
			if !arrow.TypeEqual(got, tt.want) {
				t.Errorf("GuessType(%v) = %v, want %v", tt.values, got, tt.want)
			}
		})
	}
}

// ngrid turned a blank into NaN, which forced any column containing one to
// become a float. Nulls let the column keep its narrower type.
func TestGuessTypeNullsDoNotWidenColumn(t *testing.T) {
	for _, null := range []string{"", "NA", "N/A", "null"} {
		values := []string{"1", "2", null, "4"}
		got := GuessType(values, DefaultNullValues)
		if !arrow.TypeEqual(got, arrow.PrimitiveTypes.Int64) {
			t.Errorf("GuessType(%v) = %v, want int64", values, got)
		}
	}
}

// The whole point of sampling: a value far past the first row must still
// influence the inferred type.
func TestGuessTypeUsesEveryRowNotJustTheFirst(t *testing.T) {
	values := make([]string, 200)
	for i := range values {
		values[i] = "1"
	}
	values[199] = "1.5"

	if got := GuessType(values, DefaultNullValues); !arrow.TypeEqual(got, arrow.PrimitiveTypes.Float64) {
		t.Errorf("GuessType() = %v, want float64 (a late float must widen the column)", got)
	}
}

func TestPromote(t *testing.T) {
	tests := []struct {
		from arrow.DataType
		want arrow.DataType
		ok   bool
	}{
		{arrow.FixedWidthTypes.Boolean, arrow.PrimitiveTypes.Int64, true},
		{arrow.PrimitiveTypes.Int64, arrow.PrimitiveTypes.Float64, true},
		{arrow.PrimitiveTypes.Float64, arrow.BinaryTypes.String, true},
		{arrow.FixedWidthTypes.Date32, arrow.BinaryTypes.String, true},
		{arrow.BinaryTypes.String, arrow.BinaryTypes.String, false},
	}
	for _, tt := range tests {
		got, ok := Promote(tt.from)
		if ok != tt.ok || !arrow.TypeEqual(got, tt.want) {
			t.Errorf("Promote(%v) = %v, %v; want %v, %v", tt.from, got, ok, tt.want, tt.ok)
		}
	}
}

// Promotion must terminate: repeated widening always reaches String.
func TestPromoteTerminates(t *testing.T) {
	for _, start := range []arrow.DataType{
		arrow.FixedWidthTypes.Boolean,
		arrow.PrimitiveTypes.Int64,
		arrow.FixedWidthTypes.Timestamp_us,
	} {
		dt := start
		for i := 0; ; i++ {
			next, ok := Promote(dt)
			if !ok {
				break
			}
			dt = next
			if i > 8 {
				t.Fatalf("Promote from %v did not terminate", start)
			}
		}
		if !arrow.TypeEqual(dt, arrow.BinaryTypes.String) {
			t.Errorf("Promote from %v settled on %v, want string", start, dt)
		}
	}
}

func TestInfer(t *testing.T) {
	lines := []string{"n,x,flag,word", "1,1.5,true,abc", "2,2.5,false,def"}
	rows, err := ParseRows(lines, ',')
	if err != nil {
		t.Fatal(err)
	}
	s := &Sample{Names: rows[0], Rows: rows[1:]}

	got := Infer(s, DefaultNullValues)
	want := []arrow.DataType{
		arrow.PrimitiveTypes.Int64,
		arrow.PrimitiveTypes.Float64,
		arrow.FixedWidthTypes.Boolean,
		arrow.BinaryTypes.String,
	}
	if len(got.Fields()) != len(want) {
		t.Fatalf("got %d fields, want %d", len(got.Fields()), len(want))
	}
	for i, f := range got.Fields() {
		if !arrow.TypeEqual(f.Type, want[i]) {
			t.Errorf("field %q: got %v, want %v", f.Name, f.Type, want[i])
		}
		if f.Name != strings.Split("n,x,flag,word", ",")[i] {
			t.Errorf("field %d name = %q", i, f.Name)
		}
	}
}

func TestDefaultNames(t *testing.T) {
	got := DefaultNames(3)
	want := []string{"col1", "col2", "col3"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DefaultNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
