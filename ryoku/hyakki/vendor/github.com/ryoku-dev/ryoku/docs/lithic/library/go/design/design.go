// Package design names the design languages the Ryoku desktop draws in (shell.json
// theme.design) and carries each one's corner rule. It is the Go copy of
// Lithic.designs and Lithic.cornerRadius (docs/lithic/library/qml/Lithic/Lithic.qml:48-69).
//
// Lithic cuts its corners and keeps its own signature; Material, Adwaita and Fluent round
// their corners and wear the desktop's palette. The package imports nothing outside the
// standard library, so palette, theme and kit can all depend on it.
package design

// Style is one design language. The zero value is Lithic.
type Style uint8

const (
	Lithic Style = iota
	Material
	Adwaita
	Fluent
)

var names = [...]string{Lithic: "lithic", Material: "material", Adwaita: "adwaita", Fluent: "fluent"}

// Parse returns the style named as shell.json writes it ("lithic", "material", "adwaita",
// "fluent"). Any other name returns Lithic and false, as Lithic.qml falls back to lithic.
func Parse(name string) (Style, bool) {
	for index, known := range names {
		if name == known {
			return Style(index), true
		}
	}
	return Lithic, false
}

// String returns the name shell.json uses for s; an unknown value reads as "lithic".
func (s Style) String() string {
	if int(s) < len(names) {
		return names[s]
	}
	return names[Lithic]
}

// Profile mirrors one entry of Lithic.designs. Lengths are in dp.
type Profile struct {
	// Round is corner "round"; false is corner "cut".
	Round bool
	// Scale turns a cut into a radius for a plane larger than a pill.
	Scale float32
	// Pill: a shape whose shorter side is at most Pill is drawn as a pill.
	Pill float32
	// Max caps the radius of a larger plane.
	Max float32
	// Desktop is palette "desktop": the style wears the wallpaper's Material roles.
	Desktop bool
}

// profiles is Lithic.qml:49-52.
var profiles = [...]Profile{
	Lithic:   {Round: false, Scale: 1, Pill: 0, Max: 16, Desktop: false},
	Material: {Round: true, Scale: 2.6, Pill: 56, Max: 28, Desktop: true},
	Adwaita:  {Round: true, Scale: 1.6, Pill: 26, Max: 14, Desktop: true},
	Fluent:   {Round: true, Scale: 0.75, Pill: 22, Max: 8, Desktop: true},
}

// Profile returns the style's corner and palette profile; an unknown value is Lithic's.
func (s Style) Profile() Profile {
	if int(s) < len(profiles) {
		return profiles[s]
	}
	return profiles[Lithic]
}

// Corner is Lithic.cornerRadius (Lithic.qml:61-69), all lengths in dp: the corner a shape
// of this size and cut wears in s. Lithic, and any cut <= 0, return the cut itself (a cut
// length, or square). A round style draws a shape whose shorter side is at most its pill
// threshold as a pill (half that side); a larger one gets cut*Scale, never past Max or
// half the shorter side.
func (s Style) Corner(cut, width, height float32) float32 {
	profile := s.Profile()
	if !profile.Round || cut <= 0 {
		return cut
	}
	least := max(0, min(width, height))
	if least <= profile.Pill {
		return least / 2
	}
	return min(cut*profile.Scale, profile.Max, least/2)
}
