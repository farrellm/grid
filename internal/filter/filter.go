// Package filter parses the expressions that '&' keeps rows by.
//
// An expression that starts with a comparison operator compares each cell with
// a literal; anything else is a regular expression matched against the cell's
// full text. A leading '!' negates either, as less's "&!pattern" does.
package filter

import (
	"cmp"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/farrellm/grid/internal/format"
	"github.com/farrellm/grid/internal/schema"
)

// Pred decides whether one cell passes a filter.
type Pred interface {
	Match(v format.Value) bool
	// String is the expression, normalised, for the status bar.
	String() string
}

// Parse compiles an expression.
func Parse(expr string) (Pred, error) {
	s := strings.TrimSpace(expr)

	// "!=" is an operator; any other leading '!' negates.
	if rest, ok := strings.CutPrefix(s, "!"); ok && !strings.HasPrefix(rest, "=") {
		p, err := Parse(rest)
		if err != nil {
			return nil, err
		}
		return not{p}, nil
	}

	for _, o := range operators {
		if rest, ok := strings.CutPrefix(s, o.tok); ok {
			return newCompare(o.op, o.tok, rest)
		}
	}

	re, err := regexp.Compile(s)
	if err != nil {
		return nil, err
	}
	return match{re}, nil
}

type not struct{ p Pred }

func (n not) Match(v format.Value) bool { return !n.p.Match(v) }
func (n not) String() string            { return "!" + n.p.String() }

// match is a regular expression over the cell's full text. It deliberately
// ignores the formatter: a column drawn narrow, or with fewer decimal places,
// must not change which rows a filter keeps.
type match struct{ re *regexp.Regexp }

func (m match) Match(v format.Value) bool { return m.re.MatchString(format.Text(v)) }
func (m match) String() string            { return "/" + m.re.String() + "/" }

type op int

const (
	opEq op = iota
	opNe
	opLt
	opLe
	opGt
	opGe
)

// operators is ordered so that each two-character operator is tried before
// the one-character operator it starts with.
var operators = []struct {
	tok string
	op  op
}{
	{"==", opEq}, {"!=", opNe}, {"<=", opLe}, {">=", opGe},
	{"=", opEq}, {"<", opLt}, {">", opGt},
}

// holds reports whether the operator accepts a three-way comparison result.
func (o op) holds(c int) bool {
	switch o {
	case opEq:
		return c == 0
	case opNe:
		return c != 0
	case opLt:
		return c < 0
	case opLe:
		return c <= 0
	case opGt:
		return c > 0
	default:
		return c >= 0
	}
}

// compare tests a cell against a literal. The literal is parsed up front into
// every type it can be read as, and each cell is compared as its own kind:
// a column promoted part way through a stream holds values of more than one
// kind, since the batches read before the promotion keep the narrower type.
type compare struct {
	op   op
	tok  string
	lit  string // as written, quotes included, for String
	text string

	// missing makes the literal a null, which only = and != can test for.
	missing bool

	i       int64
	isInt   bool
	f       float64
	isFloat bool
	t       time.Time
	isTime  bool
	b       bool
	isBool  bool
}

func newCompare(o op, tok, lit string) (Pred, error) {
	lit = strings.TrimSpace(lit)
	c := compare{op: o, tok: tok, lit: lit, text: lit}

	// Quotes keep surrounding spaces, and force a comparison as text.
	if len(lit) >= 2 && lit[0] == '"' && lit[len(lit)-1] == '"' {
		c.text = lit[1 : len(lit)-1]
		return c, nil
	}

	if slices.Contains(schema.DefaultNullValues, lit) {
		if o != opEq && o != opNe {
			return nil, fmt.Errorf("%s needs a value to compare with, not a missing one", tok)
		}
		c.missing = true
		return c, nil
	}

	if i, err := strconv.ParseInt(lit, 10, 64); err == nil {
		c.i, c.isInt = i, true
	}
	if f, err := strconv.ParseFloat(lit, 64); err == nil {
		c.f, c.isFloat = f, true
	}
	layouts := slices.Concat(schema.DateLayouts, schema.TimestampLayouts)
	if t, err := schema.ParseTime(lit, layouts); err == nil {
		c.t, c.isTime = t, true
	}
	if strings.EqualFold(lit, "true") || strings.EqualFold(lit, "false") {
		c.b, c.isBool = strings.EqualFold(lit, "true"), true
	}
	return c, nil
}

func (c compare) String() string { return c.tok + c.lit }

func (c compare) Match(v format.Value) bool {
	if c.missing {
		missing := v.IsNull() || (v.Kind == format.KindFloat && math.IsNaN(v.F))
		return missing == (c.op == opEq)
	}
	// A missing cell is unequal to every value, and not ordered against any.
	if v.IsNull() {
		return c.op == opNe
	}

	var res int
	switch {
	case v.Kind == format.KindInt && c.isInt:
		// Exact, where a float64 would round integers past 2^53.
		res = cmp.Compare(v.I, c.i)
	case (v.Kind == format.KindInt || v.Kind == format.KindFloat) && c.isFloat:
		f := v.Number()
		if math.IsNaN(f) {
			return c.op == opNe
		}
		res = cmp.Compare(f, c.f)
	case v.Kind == format.KindTime && c.isTime:
		res = v.T.Compare(c.t)
	case v.Kind == format.KindBool && c.isBool:
		res = compareBool(v.B, c.b)
	default:
		// The literal cannot be read as the cell's kind, or the cell is text.
		res = strings.Compare(format.Text(v), c.text)
	}
	return c.op.holds(res)
}

// compareBool orders false before true.
func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case b:
		return -1
	default:
		return 1
	}
}
