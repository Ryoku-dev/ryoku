// Package profile reads Oni character profiles: a name, artwork, animations,
// a personality and a voice. A profile is presentation only and grants
// nothing, so policy code never imports this package and a profile that names
// any authority is rejected. The shipped oni is built into the program; user
// characters live in their own folders and can never take its id.
package profile

import (
	"bytes"
	"encoding/json"
	"errors"
)

// Version is the only profile.json schema version this build reads.
const Version = 1

// ShippedID is the built-in character every reset returns to.
const ShippedID = "oni"

// SpecFile is the motion spec a parametric character is drawn from.
const SpecFile = "oni-spec.json"

// PoseFromSpec, as a state or gesture value, draws that pose from the motion
// spec instead of an image; only parametric artwork can.
const PoseFromSpec = "spec"

// Artwork kinds. Silhouette is never read from a profile: the loader sets it
// when the character's own artwork is missing.
const (
	ArtworkParametric = "parametric"
	ArtworkSVG        = "svg"
	ArtworkSilhouette = "silhouette"
)

// Voice engines.
const (
	VoiceSystem = "system"
	VoiceClips  = "clips"
	VoiceKokoro = "kokoro"
)

// Honorifics a voice may add when addressing the user.
const (
	HonorificNone = "none"
	HonorificSir  = "sir"
	HonorificMaam = "maam"
)

// Caps on what one character may hold.
const (
	MaxIDLen        = 32
	MaxNameRunes    = 40
	MaxProfileBytes = 64 << 10
	MaxPersonaBytes = 4 << 10
	MaxLinesBytes   = 32 << 10
	MaxLines        = 64
	MaxLineBytes    = 200
	MaxSVGBytes     = 256 << 10
	MaxAssetBytes   = 4 << 20
	MaxOutfits      = 16
	MaxMotifNotes   = 16
	MaxPathLen      = 128
	MaxPathDepth    = 4
	MaxClipEntries  = 4096
	MaxCharacters   = 64
	MaxSpeaker      = 511
	MinSpeed        = 0.5
	MaxSpeed        = 2.0
)

// Profile is one character after loading: profile.json with every fallback
// applied, plus the text read from its persona and lines files.
type Profile struct {
	Version     int         `json:"version"`
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Artwork     Artwork     `json:"artwork"`
	Animations  Animations  `json:"animations"`
	Personality Personality `json:"personality"`
	Voice       Voice       `json:"voice"`

	// Filled by the loader, never read from profile.json.
	Persona string   `json:"-"`
	Lines   []string `json:"-"`
	Shipped bool     `json:"-"`
}

// Artwork is how the character looks. Parametric draws it from SpecFile with
// Fallback as the image when the renderer cannot; svg shows Image. Outfits are
// alternative images by name. Anchor, when given, is where the strand leaves
// the drawing in place of the spec's strand.anchor; it moves only what is
// drawn.
type Artwork struct {
	Kind     string            `json:"kind"`
	Spec     string            `json:"spec,omitempty"`
	Image    string            `json:"image,omitempty"`
	Fallback string            `json:"fallback,omitempty"`
	Outfits  map[string]string `json:"outfits,omitempty"`
	Anchor   *Anchor           `json:"anchor,omitempty"`
}

// Anchor is a point on the artwork as fractions of its viewBox, from the
// top-left corner: both X and Y are given, each in [0, 1].
type Anchor struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// UnmarshalJSON refuses an anchor that leaves out a coordinate, which would
// otherwise read as the edge.
func (a *Anchor) UnmarshalJSON(b []byte) error {
	var raw struct {
		X *float64 `json:"x"`
		Y *float64 `json:"y"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	if raw.X == nil || raw.Y == nil {
		return errors.New("artwork.anchor needs both x and y")
	}
	a.X, a.Y = *raw.X, *raw.Y
	return nil
}

// Animations maps the spec's state and gesture ids to a pose (an SVG in the
// character's folder, or PoseFromSpec), and motif ids to MIDI notes that
// replace the spec's motif. A slot left out falls back.
type Animations struct {
	States   map[string]string `json:"states,omitempty"`
	Gestures map[string]string `json:"gestures,omitempty"`
	Motifs   map[string][]int  `json:"motifs,omitempty"`
}

// Personality names the character's voice lines and the persona appended as a
// style block after the safety rules.
type Personality struct {
	VoiceLines    string `json:"voiceLines,omitempty"`
	PromptPersona string `json:"promptPersona,omitempty"`
	IdleHumor     bool   `json:"idleHumor,omitempty"`
}

// Voice is how the character speaks. ClipPack, a folder of recorded clips,
// is only for the clips engine and must carry its license and provenance.
type Voice struct {
	Engine    string  `json:"engine,omitempty"`
	Speaker   int     `json:"speaker,omitempty"`
	Speed     float64 `json:"speed,omitempty"`
	ClipPack  string  `json:"clipPack,omitempty"`
	Honorific string  `json:"honorific,omitempty"`
}

// Lines is a lines file: the acknowledgements spoken for an ask.
type Lines struct {
	Acknowledge []string `json:"acknowledge"`
}

// Loaded is one character and what loading it found. A character that is not
// Valid carries only its id and the errors that rejected it.
type Loaded struct {
	Profile Profile `json:"profile"`
	Report  Report  `json:"report"`
	Valid   bool    `json:"valid"`

	// Files are a valid character's profile.json and every SVG it declares
	// that loaded, by its path in the character's folder as profile.json
	// names it; a slot that fell back has none. They passed the checks the
	// loader makes and together stay within MaxAssetBytes.
	Files map[string][]byte `json:"-"`
}

// ProfileFile is the key of a character's profile.json in Loaded.Files.
const ProfileFile = "profile.json"

// keys is each profile.json object's exact field set, by its path. Objects
// not listed are id-keyed maps, checked against the spec when loading.
var keys = map[string][]string{
	"":               {"version", "id", "name", "artwork", "animations", "personality", "voice"},
	"artwork":        {"kind", "spec", "image", "fallback", "outfits", "anchor"},
	"artwork.anchor": {"x", "y"},
	"animations":     {"states", "gestures", "motifs"},
	"personality":    {"voiceLines", "promptPersona", "idleHumor"},
	"voice":          {"engine", "speaker", "speed", "clipPack", "honorific"},
}

var linesKeys = map[string][]string{"": {"acknowledge"}}

// authorityKeys name grants a character can never carry, at any depth.
var authorityKeys = []string{"tools", "permissions", "capabilities", "consent", "approvals", "hands", "grants"}
