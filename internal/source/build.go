package source

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

// parseError reports a value that did not fit its column's type, naming the
// column so the caller can widen exactly that one.
//
// Arrow's own CSV reader is not used for this: it cannot trim spaces around
// numeric fields (so "1, 2" fails to parse as integers) and its error does not
// say which column failed, leaving promotion no target. Building the arrays
// directly keeps Arrow as the columnar store while giving both.
type parseError struct {
	Col   int
	Value string
	Err   error
}

func (e *parseError) Error() string {
	return fmt.Sprintf("column %d: cannot parse %q: %v", e.Col, e.Value, e.Err)
}

// buildRecord converts parsed text rows into an Arrow record batch under the
// given schema. On the first value that does not fit, it returns a *parseError
// naming the column.
func buildRecord(mem memory.Allocator, schema *arrow.Schema, rows [][]string, nulls []string) (arrow.RecordBatch, error) {
	rb := array.NewRecordBuilder(mem, schema)
	defer rb.Release()

	ncols := len(schema.Fields())
	for _, row := range rows {
		for c := 0; c < ncols; c++ {
			var cell string
			if c < len(row) {
				cell = row[c]
			}
			if err := appendValue(rb.Field(c), cell, nulls); err != nil {
				return nil, &parseError{Col: c, Value: cell, Err: err}
			}
		}
	}
	return rb.NewRecordBatch(), nil
}

// appendValue parses one cell into a builder, appending a null for missing
// values so a blank does not force the column to a wider type.
func appendValue(b array.Builder, cell string, nulls []string) error {
	if slices.Contains(nulls, cell) {
		b.AppendNull()
		return nil
	}
	s := strings.TrimSpace(cell)

	switch bld := b.(type) {
	case *array.BooleanBuilder:
		switch strings.ToLower(s) {
		case "true":
			bld.Append(true)
		case "false":
			bld.Append(false)
		default:
			return fmt.Errorf("not a boolean")
		}

	case *array.Int64Builder:
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		bld.Append(v)

	case *array.Float64Builder:
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		bld.Append(v)

	case *array.Date32Builder:
		t, err := parseTime(s, dateLayouts)
		if err != nil {
			return err
		}
		bld.Append(arrow.Date32FromTime(t))

	case *array.TimestampBuilder:
		t, err := parseTime(s, timestampLayouts)
		if err != nil {
			return err
		}
		ts, err := arrow.TimestampFromTime(t, arrow.Microsecond)
		if err != nil {
			return err
		}
		bld.Append(ts)

	case *array.StringBuilder:
		// Strings never fail, which is why promotion terminates here.
		bld.Append(cell)

	default:
		bld.AppendValueFromString(cell) //nolint:errcheck // best effort
	}
	return nil
}

// Layouts accepted when parsing, mirroring those the schema package infers.
var (
	dateLayouts      = []string{"2006-01-02", "20060102", "2006/01/02"}
	timestampLayouts = []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999",
	}
)

func parseTime(s string, layouts []string) (time.Time, error) {
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("not a timestamp")
}
