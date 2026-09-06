package source

import (
	"bufio"
	"io"
	"strings"
)

// maxLineSize bounds a single input line.
const maxLineSize = 16 << 20

// lineReader yields cleaned lines and hides comment lines from the caller.
type lineReader struct {
	sc      *bufio.Scanner
	comment string
	first   bool
	// pending holds comment lines seen before any data line.
	pending []string
}

func newLineReader(r io.Reader, comment string) *lineReader {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	return &lineReader{sc: sc, comment: comment, first: true}
}

// next returns the next data line. Comment lines are collected rather than
// returned; those appearing before any data become title lines.
func (l *lineReader) next() (string, bool) {
	for l.sc.Scan() {
		line := clean(l.sc.Text())
		if l.first {
			// A UTF-8 BOM would otherwise become part of the first column name.
			line = strings.TrimPrefix(line, "\ufeff")
			l.first = false
		}
		if l.comment != "" && strings.HasPrefix(line, l.comment) {
			l.pending = append(l.pending, line)
			continue
		}
		if line == "" {
			continue
		}
		return line, true
	}
	return "", false
}

// titles returns the comment lines gathered so far.
func (l *lineReader) titles() []string { return l.pending }

func (l *lineReader) err() error { return l.sc.Err() }

// clean strips NULs and trailing whitespace, as ngrid's clean_line does.
// Leading whitespace is kept, so a space-delimited file's empty leading fields
// survive.
func clean(s string) string {
	if strings.ContainsRune(s, 0) {
		s = strings.ReplaceAll(s, "\x00", "")
	}
	return strings.TrimRight(s, " \t\r\n")
}
