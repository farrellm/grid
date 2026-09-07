package schema

import (
	"fmt"

	"github.com/apache/arrow-go/v18/arrow"
)

// Sample holds the head of a delimited stream: the raw lines, the delimiter
// sniffed from them, the column names, and the parsed rows used for inference.
type Sample struct {
	Delimiter rune
	Names     []string
	Rows      [][]string // data rows only
	Comments  []string   // comment lines seen before the first data line
}

// Infer builds an Arrow schema from sampled rows. Each column's type is the
// narrowest that fits every sampled value, not just the first — the property
// Arrow's own inferring reader lacks.
func Infer(s *Sample, nulls []string) *arrow.Schema {
	fields := make([]arrow.Field, len(s.Names))
	for i, name := range s.Names {
		fields[i] = arrow.Field{
			Name:     name,
			Type:     GuessType(column(s.Rows, i), nulls),
			Nullable: true,
		}
	}
	return arrow.NewSchema(fields, nil)
}

// column extracts the i'th field of every row, skipping rows too short to have
// one, so a ragged file still yields a usable guess.
func column(rows [][]string, i int) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if i < len(r) {
			out = append(out, r[i])
		}
	}
	return out
}

// DefaultNames generates placeholder column names for headerless input, using
// ngrid's "col1", "col2", ... convention.
func DefaultNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("col%d", i+1)
	}
	return names
}

// Promote widens a type one step along Boolean -> Int64 -> Float64 -> String,
// with temporal types collapsing straight to String. It reports false when the
// type is already String and cannot widen further.
//
// This is the recovery path for a value that appears past the sample and does
// not fit the inferred type — the case ngrid left as a FIXME in
// DelimitedFileModel.ensure_rows.
func Promote(dt arrow.DataType) (arrow.DataType, bool) {
	switch dt.ID() {
	case arrow.BOOL:
		return arrow.PrimitiveTypes.Int64, true
	case arrow.INT64:
		return arrow.PrimitiveTypes.Float64, true
	case arrow.STRING:
		return dt, false
	default:
		// Float64, Date32, Timestamp and anything else fall back to text,
		// which always renders.
		return arrow.BinaryTypes.String, true
	}
}

// PromoteSchema returns a copy of schema with the named column widened one step.
func PromoteSchema(s *arrow.Schema, col int) (*arrow.Schema, bool) {
	if col < 0 || col >= len(s.Fields()) {
		return s, false
	}
	fields := make([]arrow.Field, len(s.Fields()))
	copy(fields, s.Fields())

	next, ok := Promote(fields[col].Type)
	if !ok {
		return s, false
	}
	fields[col].Type = next
	return arrow.NewSchema(fields, nil), true
}
