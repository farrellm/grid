package source

import (
	"bufio"
	"io"
	"strings"
	"sync"
	"time"
)

// maxLineSize bounds a single input line.
const maxLineSize = 16 << 20

// lineReader yields cleaned lines and hides comment lines from the caller.
//
// Scanning runs on its own goroutine feeding a bounded channel, so a caller can
// ask for whatever has arrived without waiting on a producer that is still
// thinking. That is what lets a pipe -- a live query, tail -f -- display as it
// streams instead of only once it has emitted a whole chunk. The bound gives
// backpressure: the scanner runs at most one chunk ahead, so a huge file is
// still read lazily rather than slurped.
//
// There is a single consumer: readSample runs before the reader goroutine
// starts, and everything after that is driven by CSV.run. So the pushback slot
// needs no lock; only the fields the scanner goroutine writes do.
type lineReader struct {
	lines chan string

	// unread holds a line handed back by wait, for the next read to return.
	unread    string
	hasUnread bool

	stop      chan struct{}
	closeOnce sync.Once

	mu sync.Mutex
	// pending holds comment lines seen before any data line.
	pending []string
	scanErr error
}

func newLineReader(r io.Reader, comment string) *lineReader {
	l := &lineReader{
		lines: make(chan string, DefaultChunkSize),
		stop:  make(chan struct{}),
	}
	go l.scan(r, comment)
	return l
}

// scan reads the input, cleaning lines and diverting comments, until the input
// ends or the reader is closed. Closing l.lines last means a caller that sees
// the channel closed can rely on scanErr already being set.
func (l *lineReader) scan(r io.Reader, comment string) {
	defer close(l.lines)

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	first := true
	for sc.Scan() {
		line := clean(sc.Text())
		if first {
			// A UTF-8 BOM would otherwise become part of the first column name.
			line = strings.TrimPrefix(line, "\ufeff")
			first = false
		}
		if comment != "" && strings.HasPrefix(line, comment) {
			l.mu.Lock()
			l.pending = append(l.pending, line)
			l.mu.Unlock()
			continue
		}
		if line == "" {
			continue
		}
		select {
		case l.lines <- line:
		case <-l.stop:
			return
		}
	}
	if err := sc.Err(); err != nil {
		l.mu.Lock()
		l.scanErr = err
		l.mu.Unlock()
	}
}

// next returns the next data line, blocking until one arrives. It reports false
// once the input has ended, or the reader has been closed.
func (l *lineReader) next() (string, bool) {
	if l.hasUnread {
		line := l.unread
		l.unread, l.hasUnread = "", false
		return line, true
	}
	select {
	case line, ok := <-l.lines:
		return line, ok
	case <-l.stop:
		return "", false
	}
}

// nextWithin returns the next data line, waiting at most d for one. idle
// reports that none arrived in time and the input has not ended, so the caller
// may use what it already has and come back for the rest. A false ok with idle
// false is the end of the input.
func (l *lineReader) nextWithin(d time.Duration) (line string, ok, idle bool) {
	if l.hasUnread {
		line, l.unread, l.hasUnread = l.unread, "", false
		return line, true, false
	}
	// Take an already-buffered line without paying for a timer.
	select {
	case line, ok = <-l.lines:
		return line, ok, false
	case <-l.stop:
		return "", false, false
	default:
	}
	if d <= 0 {
		return "", false, true
	}

	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case line, ok = <-l.lines:
		return line, ok, false
	case <-l.stop:
		return "", false, false
	case <-t.C:
		return "", false, true
	}
}

// wait blocks until a line is available, reporting false if the input ends
// first. The line is pushed back for the next read, so a caller can wait out a
// pause in the stream without consuming anything.
func (l *lineReader) wait() bool {
	if l.hasUnread {
		return true
	}
	line, ok := l.next()
	if !ok {
		return false
	}
	l.unread, l.hasUnread = line, true
	return true
}

// titles returns the comment lines gathered so far.
func (l *lineReader) titles() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.pending
}

func (l *lineReader) err() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.scanErr
}

// close releases callers blocked waiting for a line. The scanner goroutine may
// still be parked in a read on a stream that never ends -- that read cannot be
// interrupted without closing the file -- but it holds nothing but itself, and
// this is the shutdown path.
func (l *lineReader) close() {
	l.closeOnce.Do(func() { close(l.stop) })
}

// clean strips NULs and trailing whitespace, as ngrid's clean_line does.
// Leading whitespace is kept, so a space-delimited file's empty leading fields
// survive.
func clean(s string) string {
	if strings.ContainsRune(s, 0) {
		s = strings.ReplaceAll(s, "\x00", "")
	}
	return strings.TrimRight(s, " \t\r\n")
}
