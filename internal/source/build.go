package source

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/farrellm/grid/internal/schema"
)

// parseError reports a value that did not fit its column's type, naming the
// column so the caller can widen exactly that one.
//
// Arrow's own CSV reader is not used for this: it cannot trim spaces around
// numeric fields (so "1, 2" fails to parse as integers) and its error does not
// say which column failed, leaving promotion no target. Building the arrays
// directly keeps Arrow as the columnar store while giving both.
// errNotBool is the one rejection appendValue raises itself; the rest come from
// strconv and time. It is a package-level value because a failing cell is the
// common case on the promotion path, where a whole column is rejected one value
// at a time.
var errNotBool = errors.New("not a boolean")

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
		for c := range ncols {
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
			return errNotBool
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
		t, err := schema.ParseTime(s, schema.DateLayouts)
		if err != nil {
			return err
		}
		bld.Append(arrow.Date32FromTime(t))

	case *array.TimestampBuilder:
		t, err := schema.ParseTime(s, schema.TimestampLayouts)
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
