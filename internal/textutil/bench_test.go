package textutil

import "testing"

// The two alphabets are measured separately: ASCII takes the fast path through
// the width scan, while CJK text is where display width and rune count diverge.
const (
	asciiText = "a moderately long column value"
	cjkText   = "日本語のテキストはここにあります"
)

func BenchmarkWidth(b *testing.B) {
	b.Run("ascii", func(b *testing.B) { benchWidth(b, asciiText) })
	b.Run("cjk", func(b *testing.B) { benchWidth(b, cjkText) })
}

func benchWidth(b *testing.B, s string) {
	b.ReportAllocs()
	n := 0
	for b.Loop() {
		n = Width(s)
	}
	if n == 0 {
		b.Fatal("zero width")
	}
}

func BenchmarkPad(b *testing.B) {
	b.Run("ascii", func(b *testing.B) { benchPad(b, asciiText, 48) })
	b.Run("cjk", func(b *testing.B) { benchPad(b, cjkText, 48) })
	// Nothing to add: the common case for a full column.
	b.Run("exact", func(b *testing.B) { benchPad(b, asciiText, len(asciiText)) })
}

func benchPad(b *testing.B, s string, w int) {
	b.ReportAllocs()
	var sink string
	for b.Loop() {
		sink = Pad(s, w, ' ', true)
	}
	if sink == "" {
		b.Fatal("empty")
	}
}

func BenchmarkElide(b *testing.B) {
	b.Run("ascii", func(b *testing.B) { benchElide(b, asciiText, 16) })
	b.Run("cjk", func(b *testing.B) { benchElide(b, cjkText, 16) })
	// Fits already, so it returns unchanged: the common case.
	b.Run("fits", func(b *testing.B) { benchElide(b, asciiText, 64) })
}

func benchElide(b *testing.B, s string, w int) {
	b.ReportAllocs()
	var sink string
	for b.Loop() {
		sink = Elide(s, w, "…", 0.7)
	}
	if sink == "" {
		b.Fatal("empty")
	}
}

func BenchmarkPalide(b *testing.B) {
	b.Run("ascii", func(b *testing.B) { benchPalide(b, asciiText, 20) })
	b.Run("cjk", func(b *testing.B) { benchPalide(b, cjkText, 20) })
}

func benchPalide(b *testing.B, s string, w int) {
	b.ReportAllocs()
	var sink string
	for b.Loop() {
		sink = Palide(s, w, "…", ' ', 0.7, true)
	}
	if sink == "" {
		b.Fatal("empty")
	}
}
