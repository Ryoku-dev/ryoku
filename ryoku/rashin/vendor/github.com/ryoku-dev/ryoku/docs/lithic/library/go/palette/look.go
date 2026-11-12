package palette

// Contrast selects the tone ladder: standard, or high, which spreads the
// stones further from the ground and strengthens ink and hairlines.
type Contrast uint8

const (
	Standard Contrast = iota
	High
)

// Tint selects how much of the palette surface's chroma the stones take.
type Tint uint8

const (
	TintOff Tint = iota
	TintLow
	TintMedium
)

// Look is the reader's choice of contrast and tint. The zero look is
// standard contrast with no tint.
type Look struct {
	Contrast Contrast
	Tint     Tint
}

// ParseLook reads a contrast ("standard", "high") and a tint ("off", "low",
// "medium"). An unknown string gives the zero value of its field.
func ParseLook(contrast, tint string) Look {
	var look Look
	if contrast == "high" {
		look.Contrast = High
	}
	switch tint {
	case "low":
		look.Tint = TintLow
	case "medium":
		look.Tint = TintMedium
	}
	return look
}
