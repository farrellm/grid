// Package schema sniffs the delimiter and column types of delimited text.
//
// Arrow's own csv.NewInferringReader infers each column's type from the first
// data row alone and then freezes the schema (arrow/csv/reader.go, the
// `r.bld == nil` guard in validate), so a column that is "1" on row 1 and "1.5"
// on row 500 fails mid-read. ngrid instead samples many rows before deciding.
// This package reproduces that sampling and hands Arrow an explicit schema.
package schema

import (
	"encoding/csv"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
)

// Delimiters are the candidates tried when sniffing, in ngrid's order.
var Delimiters = []rune{',', ' ', '|', '\t'}

// DefaultNullValues are the strings read as a missing value. ngrid had no such
// notion: it converted "" to NaN, which forced any column with a blank to
// become a float. Treating them as nulls keeps the column's real type.
var DefaultNullValues = []string{"", "NA", "N/A", "na", "n/a", "NULL", "null", "NaN", "nan"}

// DateLayouts and TimestampLayouts are the layouts a value may be written in to
// be inferred as a date or a timestamp. They are exported because inference and
// ingest must agree exactly: a value recognised here has to parse there, or a
// column would be typed as temporal and then fail to build.
var (
	DateLayouts = []string{"2006-01-02", "20060102", "2006/01/02"}

	TimestampLayouts = []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999",
	}
)

// ErrNotTime reports a value that matched none of the layouts offered.
var ErrNotTime = errors.New("not a date or timestamp")

// ParseTime returns the first of layouts that parses s. s must already be
// trimmed: both callers have a trimmed string in hand, and trimming again here
// would cost a pass over every temporal cell ingested.
func ParseTime(s string, layouts []string) (time.Time, error) {
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, ErrNotTime
}

// GuessDelimiter picks the delimiter that splits every sample line into the
// same, largest number of fields.
//
// Ties are broken by the highest delimiter rune, reproducing ngrid's
// `sorted(counts)[-1]` over (count, delimiter) pairs.
func GuessDelimiter(sample []string, delims []rune) rune {
	if len(delims) == 0 {
		delims = Delimiters
	}

	best, bestCount := delims[0], -1
	for _, d := range delims {
		n := uniformFieldCount(sample, d)
		if n > bestCount || (n == bestCount && d > best) {
			best, bestCount = d, n
		}
	}
	return best
}

// uniformFieldCount returns the field count if every line yields the same
// number of fields under delim, and 0 otherwise.
func uniformFieldCount(sample []string, delim rune) int {
	rows, err := ParseRows(sample, delim)
	if err != nil || len(rows) == 0 {
		return 0
	}
	count := len(rows[0])
	for _, r := range rows[1:] {
		if len(r) != count {
			return 0
		}
	}
	return count
}

// ParseRows splits lines into fields using delim.
func ParseRows(lines []string, delim rune) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(strings.Join(lines, "\n")))
	r.Comma = delim
	r.LazyQuotes = true
	r.FieldsPerRecord = -1 // ragged rows are checked by the caller
	r.ReuseRecord = false
	return r.ReadAll()
}

// GuessType returns the most specific Arrow type that represents every
// non-null value, trying candidates from narrowest to widest as ngrid's
// guess_type does.
func GuessType(values []string, nulls []string) arrow.DataType {
	var seen bool
	for _, v := range values {
		if !isNull(v, nulls) {
			seen = true
			break
		}
	}
	if !seen {
		// An entirely empty column carries no evidence; strings render blank,
		// where a numeric guess would fill the column with NaN placeholders.
		return arrow.BinaryTypes.String
	}

	for _, c := range candidates {
		if allMatch(values, nulls, c.match) {
			return c.typ
		}
	}
	return arrow.BinaryTypes.String
}

type candidate struct {
	typ   arrow.DataType
	match func(string) bool
}

// Ordered narrowest to widest. Booleans precede integers so a 0/1 column of
// "true"/"false" is not read as numeric; strings terminate the ladder.
var candidates = []candidate{
	{arrow.FixedWidthTypes.Boolean, isBool},
	{arrow.PrimitiveTypes.Int64, isInt},
	{arrow.PrimitiveTypes.Float64, isFloat},
	{arrow.FixedWidthTypes.Date32, isDate},
	{arrow.FixedWidthTypes.Timestamp_us, isTimestamp},
}

func allMatch(values, nulls []string, match func(string) bool) bool {
	for _, v := range values {
		if isNull(v, nulls) {
			continue
		}
		if !match(v) {
			return false
		}
	}
	return true
}

func isNull(v string, nulls []string) bool {
	return slices.Contains(nulls, v)
}

// isBool accepts only "true" and "false" in any case, as ngrid's as_bool does.
func isBool(s string) bool {
	switch strings.ToLower(s) {
	case "true", "false":
		return true
	}
	return false
}

func isInt(s string) bool {
	_, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return err == nil
}

func isFloat(s string) bool {
	_, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return err == nil
}

func isDate(s string) bool { return matchesAny(s, DateLayouts) }

func isTimestamp(s string) bool { return matchesAny(s, TimestampLayouts) }

func matchesAny(s string, layouts []string) bool {
	_, err := ParseTime(strings.TrimSpace(s), layouts)
	return err == nil
}
