package ui

import "strings"

// A cell style is an SGR parameter string, applied by wrapping text in
// "\x1b[<params>m" ... "\x1b[m".
//
// Lip Gloss is used for the help overlay, but not for grid cells: rendering an
// underlined style through it emits a separate escape pair around every rune
// (\x1b[1;4;4mM\x1b[m\x1b[1;4;4mI\x1b[m...), which on a 200x50 grid is far more
// output and allocation than a viewer redrawing on every keystroke can afford.
type cellStyle struct {
	prefix string
	suffix string
}

func newCellStyle(params string) cellStyle {
	if params == "" {
		return cellStyle{}
	}
	return cellStyle{prefix: "\x1b[" + params + "m", suffix: "\x1b[m"}
}

// write appends s to b wrapped in this style.
func (c cellStyle) write(b *strings.Builder, s string) {
	if c.prefix == "" {
		b.WriteString(s)
		return
	}
	b.WriteString(c.prefix)
	b.WriteString(s)
	b.WriteString(c.suffix)
}

// palette mirrors the seven curses colour pairs ngrid initialises, plus the
// bold-underline emphasis it adds to header cells.
type palette struct {
	normal    cellStyle // 1: default
	frozen    cellStyle // 2: blue on default
	separator cellStyle // 3: default
	footer    cellStyle // 4: reverse video
	cursor    cellStyle // 5: black on white
	selection cellStyle // 6: white on blue
	frozenSel cellStyle // 7: blue on white

	headerNormal    cellStyle
	headerFrozen    cellStyle
	headerCursor    cellStyle
	headerFrozenSel cellStyle
}

func newPalette() palette {
	const emphasis = "1;4" // bold + underline, for the header row
	return palette{
		normal:    newCellStyle(""),
		frozen:    newCellStyle("34"),
		separator: newCellStyle(""),
		footer:    newCellStyle("7"),
		cursor:    newCellStyle("30;47"),
		selection: newCellStyle("37;44"),
		frozenSel: newCellStyle("34;47"),

		headerNormal:    newCellStyle(emphasis),
		headerFrozen:    newCellStyle("34;" + emphasis),
		headerCursor:    newCellStyle("30;47;" + emphasis),
		headerFrozenSel: newCellStyle("34;47;" + emphasis),
	}
}

// cell picks the style for a data cell, following ngrid's precedence: the
// selected cell wins, then a frozen column under the cursor, then any cell
// under the cursor, then any frozen column.
func (p palette) cell(frozen, atCursor, atSelect bool) cellStyle {
	switch {
	case atSelect:
		return p.selection
	case frozen && atCursor:
		return p.frozenSel
	case atCursor:
		return p.cursor
	case frozen:
		return p.frozen
	default:
		return p.normal
	}
}

// header picks the style for a header cell.
func (p palette) header(frozen, atCursor bool) cellStyle {
	switch {
	case frozen && atCursor:
		return p.headerFrozenSel
	case atCursor:
		return p.headerCursor
	case frozen:
		return p.headerFrozen
	default:
		return p.headerNormal
	}
}
