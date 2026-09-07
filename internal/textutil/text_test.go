package textutil

import "testing"

// The Elide cases are the doctest examples from ngrid's text.py:47-70.
func TestElideDoctests(t *testing.T) {
	const s = "Hello, world.  This is a test."

	tests := []struct {
		name     string
		maxWidth int
		ellipsis string
		position float64
		want     string
	}{
		{"default", 24, "...", 1.0, "Hello, world.  This i..."},
		{"long ellipsis", 24, " and more", 1.0, "Hello, world.   and more"},
		{"position 0", 24, "...", 0, "...rld.  This is a test."},
		{"position 0.7", 24, "...", 0.7, "Hello, world.  ... test."},
		{"fits", 40, "...", 1.0, s},
		{"exact", 30, "...", 1.0, s},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Elide(s, tt.maxWidth, tt.ellipsis, tt.position)
			if got != tt.want {
				t.Errorf("Elide(%d, %q, %v) = %q, want %q",
					tt.maxWidth, tt.ellipsis, tt.position, got, tt.want)
			}
			if w := Width(got); tt.maxWidth < Width(s) && w != tt.maxWidth {
				t.Errorf("Elide width = %d, want %d", w, tt.maxWidth)
			}
		})
	}
}

func TestPad(t *testing.T) {
	tests := []struct {
		s     string
		width int
		pad   rune
		left  bool
		want  string
	}{
		{"ab", 5, ' ', false, "ab   "},
		{"ab", 5, ' ', true, "   ab"},
		{"ab", 5, '0', true, "000ab"},
		{"abcde", 5, ' ', true, "abcde"},
		{"abcdef", 5, ' ', true, "abcdef"}, // never truncates
		{"", 3, '-', false, "---"},
	}
	for _, tt := range tests {
		if got := Pad(tt.s, tt.width, tt.pad, tt.left); got != tt.want {
			t.Errorf("Pad(%q, %d, %q, %v) = %q, want %q",
				tt.s, tt.width, tt.pad, tt.left, got, tt.want)
		}
	}
}

func TestPalideAlwaysExactWidth(t *testing.T) {
	for _, s := range []string{"", "a", "hello", "a much longer string than fits"} {
		for width := 1; width <= 12; width++ {
			got := Palide(s, width, "…", ' ', 1.0, false)
			if w := Width(got); w != width {
				t.Errorf("Palide(%q, %d) = %q, width %d, want %d", s, width, got, w, width)
			}
		}
	}
}

// Wide characters must be measured in display columns, not runes: this is the
// alignment bug the Python version has.
func TestWideCharacters(t *testing.T) {
	const wide = "你好世界" // 4 runes, 8 columns
	if w := Width(wide); w != 8 {
		t.Fatalf("Width(%q) = %d, want 8", wide, w)
	}
	for width := 2; width <= 10; width++ {
		got := Palide(wide, width, "…", ' ', 1.0, false)
		if w := Width(got); w != width {
			t.Errorf("Palide(wide, %d) = %q, width %d, want %d", width, got, w, width)
		}
	}
}
