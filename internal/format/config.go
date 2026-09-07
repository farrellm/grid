package format

// Config holds display settings. It replaces ngrid's DEFAULT_CFG, which stored
// every value as a string and parsed it back on each use.
type Config struct {
	Ellipsis  string
	InfString string
	NaNString string
	Separator string

	PrecisionMin int
	PrecisionMax int

	// Values outside [FixedRangeMin, FixedRangeMax] switch to scientific
	// notation. ngrid spelled these scientific_max and scientific_min, whose
	// names read backwards from their meaning.
	FixedRangeMin float64
	FixedRangeMax float64

	StrWidthMin int
	StrWidthMax int

	ShowCursor bool
	ShowHeader bool
	ShowFooter bool

	TimeFormat string
}

// DefaultConfig returns ngrid's DEFAULT_CFG values.
func DefaultConfig() Config {
	return Config{
		Ellipsis:      "…",
		InfString:     "∞",
		NaNString:     "NaN",
		Separator:     " ",
		PrecisionMin:  1,
		PrecisionMax:  6,
		FixedRangeMin: 1e-8,
		FixedRangeMax: 1e+12,
		StrWidthMin:   4,
		StrWidthMax:   32,
		ShowCursor:    false,
		ShowHeader:    true,
		ShowFooter:    true,
		TimeFormat:    "ISO 8601 extended",
	}
}

// Separators are cycled by the '|' key.
var Separators = []string{" ", "┊", "  ", "   ", " ┊ "}
