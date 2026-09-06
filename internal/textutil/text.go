// Package textutil provides padding and eliding helpers for fixed-width display.
//
// It is a port of ngrid's text.py, with one deliberate change: lengths are
// measured in terminal display columns rather than runes, so wide (CJK) and
// zero-width characters line up in the grid.
package textutil

import (
	"math"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Width returns the display width of s in terminal columns.
func Width(s string) int { return ansi.StringWidth(s) }

// Pad pads s to a minimum display width using the pad rune. If left is true the
// padding goes on the left, otherwise on the right.
func Pad(s string, width int, pad rune, left bool) string {
	n := width - Width(s)
	if n <= 0 {
		return s
	}
	fill := strings.Repeat(string(pad), n)
	if left {
		return fill + s
	}
	return s + fill
}

// Elide truncates s to at most maxWidth display columns, replacing the removed
// characters with ellipsis. Position gives the location of the ellipsis as a
// fraction of the retained text: 0 keeps only the tail, 1 keeps only the head.
func Elide(s string, maxWidth int, ellipsis string, position float64) string {
	if maxWidth <= 0 {
		return ""
	}
	if Width(s) <= maxWidth {
		return s
	}
	if position < 0 {
		position = 0
	} else if position > 1 {
		position = 1
	}

	ew := Width(ellipsis)
	if ew >= maxWidth {
		return ansi.Truncate(ellipsis, maxWidth, "")
	}

	keep := maxWidth - ew
	// math.RoundToEven matches Python's round(), which the expected
	// outputs in text.py's docstrings depend on for exact .5 cases.
	left := int(math.RoundToEven(position * float64(keep)))
	if left > keep {
		left = keep
	}
	right := keep - left

	var b strings.Builder
	if left > 0 {
		b.WriteString(ansi.Truncate(s, left, ""))
	}
	b.WriteString(ellipsis)
	if right > 0 {
		b.WriteString(ansi.TruncateLeft(s, Width(s)-right, ""))
	}
	return b.String()
}

// Palide elides s to width and then pads it back out to exactly that width.
func Palide(s string, width int, ellipsis string, pad rune, position float64, left bool) string {
	return Pad(Elide(s, width, ellipsis, position), width, pad, left)
}
