package main

// The Profile page's share backend. profile.json (user-owned, hand-editable, a
// preference like decor.json) stays the source of truth; a custom hero image
// lives beside it in profile/. "export" packs both into one self-contained,
// portable .ryoprofile envelope (the hero base64-embedded so there is no archive
// tooling), and "import" validates that envelope and unpacks it back, applying
// nothing when the file is not a valid .ryoprofile.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

func profileDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "ryoku")
}

func profileConfigPath() string { return filepath.Join(profileDir(), "profile.json") }

func profileHeroDir() string { return filepath.Join(profileDir(), "profile") }

// profileModel peeks at the card style and hero fields. Other fields stay byte-exact unless
// import must rewrite a custom hero's source to the canonical PNG filename or drop a cutout
// that didn't travel with it.
type profileModel struct {
	Style string `json:"style"`
	Hero  struct {
		Kind   string `json:"kind"`
		Source string `json:"source"`
		Cutout bool   `json:"cutout"`
	} `json:"hero"`
}

const maxProfileBytes = 64 << 10

// the hero and the cutout travel as base64, a third larger than their bytes
const maxProfileEnvelopeBytes = (maxPictureBytes+maxCutoutBytes)/3*4 + 1<<20

var errStyleNeedsBadge = errors.New("a card style comes with a team badge, and this machine holds none")

var errStyleNotYours = errors.New("this card style belongs to another badge holder; make your own look through a team design")

var profilePictureName = regexp.MustCompile(`^[a-z0-9_-]+\.(png|jpg|jpeg|gif)$`)

// the pictures that may move stay home: a .ryoprofile carries their still frame
var profileMovingName = regexp.MustCompile(`^hero\.(gif|webp|mp4|webm|mkv)$`)

// JSON needs its own envelope bound and UTF-8 rejection before Unmarshal,
// which otherwise silently replaces malformed bytes.
func readProfileFile(path string, limit int64) ([]byte, error) {
	raw, err := readBoundedFile(path, limit)
	if err != nil {
		return nil, fmt.Errorf("read profile file: %w", err)
	}
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("profile file is not valid UTF-8")
	}
	return raw, nil
}

func profileObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("expected a profile object")
	}
	return fields, nil
}

// profileWithHero sets one hero field, the other fields kept.
func profileWithHero(raw json.RawMessage, key string, value any) (json.RawMessage, error) {
	fields, err := profileObject(raw)
	if err != nil {
		return nil, err
	}
	hero, err := profileObject(fields["hero"])
	if err != nil {
		return nil, err
	}
	if hero[key], err = json.Marshal(value); err != nil {
		return nil, err
	}
	if fields["hero"], err = json.Marshal(hero); err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}

func profileChoice(raw json.RawMessage, choices ...string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("expected a string choice")
	}
	for _, choice := range choices {
		if value == choice {
			return nil
		}
	}
	return fmt.Errorf("unknown profile choice")
}

func profilePlainText(raw json.RawMessage, limit int) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("expected profile text")
	}
	if utf8.RuneCountInString(value) > limit {
		return fmt.Errorf("profile text exceeds %d characters", limit)
	}
	// Qt's automatic rich-text detection must not interpret a shared name or
	// motto as markup. Format controls can also disguise paths and UI text.
	for _, r := range value {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) || strings.ContainsRune("<>&", r) || r == utf8.RuneError {
			return fmt.Errorf("profile text contains markup or control characters")
		}
	}
	return nil
}

func profileStringList(raw json.RawMessage, limit int, validate func(json.RawMessage) error) error {
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("expected a profile list")
	}
	if len(values) > limit {
		return fmt.Errorf("profile list exceeds %d entries", limit)
	}
	for _, value := range values {
		if err := validate(value); err != nil {
			return err
		}
	}
	return nil
}

// Keep the imported surface closed to future path, command or URL fields.
// Numeric limits mirror CardEditor.qml rather than Qt's coercion rules.
func validateProfile(raw json.RawMessage) (profileModel, error) {
	var model profileModel
	if len(raw) > maxProfileBytes || !utf8.Valid(raw) {
		return model, fmt.Errorf("profile settings are oversized or not valid UTF-8")
	}
	fields, err := profileObject(raw)
	if err != nil {
		return model, err
	}
	for key, value := range fields {
		err = nil
		switch key {
		case "style":
			err = profileChoice(value, "", "classic", "stand")
		case "preset":
			err = profileChoice(value, "", "full", "minimal")
		case "heroSide":
			err = profileChoice(value, "", "left", "right")
		case "homeView":
			err = profileChoice(value, "", "plate", "card")
		case "hero":
			err = validateProfileHero(value)
		case "blocks":
			var blocks map[string]json.RawMessage
			blocks, err = profileObject(value)
			for block, enabled := range blocks {
				switch block {
				case "epithets", "telemetry", "specs", "packages", "palette", "barcode", "signal", "watermark", "marginalia":
				default:
					err = fmt.Errorf("unknown profile block")
				}
				if err != nil {
					break
				}
				if string(bytes.TrimSpace(enabled)) != "true" && string(bytes.TrimSpace(enabled)) != "false" {
					err = fmt.Errorf("profile block must be boolean")
					break
				}
			}
		case "text":
			var text map[string]json.RawMessage
			text, err = profileObject(value)
			for field, content := range text {
				switch field {
				case "name", "watermarkGlyph":
					err = profilePlainText(content, 128)
				case "standName":
					err = profilePlainText(content, 64)
				case "tagline", "marginalia":
					err = profilePlainText(content, 512)
				case "epithets", "verse":
					err = profileStringList(content, 16, func(v json.RawMessage) error { return profilePlainText(v, 128) })
				default:
					err = fmt.Errorf("unknown profile text field")
				}
				if err != nil {
					break
				}
			}
		case "vitals":
			err = profileStringList(value, 5, func(v json.RawMessage) error { return profileChoice(v, "core", "gpu", "mem", "net", "frac") })
		case "specs":
			err = profileStringList(value, 3, func(v json.RawMessage) error { return profileChoice(v, "resolution", "compositor", "uptime") })
		default:
			err = fmt.Errorf("unknown profile field")
		}
		if err != nil {
			return model, fmt.Errorf("profile %s: %w", key, err)
		}
	}
	if err := json.Unmarshal(raw, &model); err != nil {
		return model, err
	}
	// a style is one badge holder's own look: only a badge whose login owns it may wear it
	if model.Style != "" && model.Style != "classic" {
		b, held, _, _ := ownBadge(false)
		if !held {
			return model, errStyleNeedsBadge
		}
		if !styleOwnedBy(model.Style, b.Login) {
			return model, errStyleNotYours
		}
	}
	return model, nil
}

func validateProfileHero(raw json.RawMessage) error {
	fields, err := profileObject(raw)
	if err != nil {
		return err
	}
	for key, value := range fields {
		switch key {
		case "kind":
			err = profileChoice(value, "", "default", "gallery", "custom")
		case "source":
			var source string
			err = json.Unmarshal(value, &source)
			if err == nil && (bytes.Equal(bytes.TrimSpace(value), []byte("null")) || len(source) > 128 || source != "" && !profilePictureName.MatchString(source)) {
				err = fmt.Errorf("hero source must be a bare image filename")
			}
		case "style":
			err = profileChoice(value, "", "photo", "mono", "dither")
		case "fit", "invert", "cutout":
			if string(bytes.TrimSpace(value)) != "true" && string(bytes.TrimSpace(value)) != "false" {
				err = fmt.Errorf("hero flag must be boolean")
			}
		case "zoom", "focalX", "focalY", "brightness", "contrast", "saturation", "dither":
			low, high := 0.0, 1.0
			switch key {
			case "zoom":
				low, high = 1, 4
			case "dither":
				low, high = 1, 6
			case "brightness", "contrast":
				low, high = -0.6, 0.6
			case "saturation":
				low, high = -1, 1
			}
			var n float64
			err = json.Unmarshal(value, &n)
			if err == nil && (bytes.Equal(bytes.TrimSpace(value), []byte("null")) || n < low || n > high) {
				err = fmt.Errorf("hero %s must be between %g and %g", key, low, high)
			}
		default:
			err = fmt.Errorf("unknown hero field")
		}
		if err != nil {
			return err
		}
	}
	var model profileModel
	if err := json.Unmarshal(append(append([]byte(`{"hero":`), raw...), '}'), &model); err != nil {
		return err
	}
	ext := filepath.Ext(model.Hero.Source)
	if model.Hero.Kind == "gallery" && (ext != ".png" && ext != ".gif" || !validGalleryName(strings.TrimSuffix(model.Hero.Source, ext))) {
		return fmt.Errorf("gallery hero needs a bare gallery filename")
	}
	if model.Hero.Kind == "custom" && model.Hero.Source == "" {
		return fmt.Errorf("custom hero needs a picture filename")
	}
	return nil
}

// ryoHero is the embedded custom hero image inside an envelope: the bare file
// name plus the base64 of its raw bytes.
type ryoHero struct {
	Name string `json:"name"`
	B64  string `json:"b64"`
}

// ryoEnvelope is the .ryoprofile file: a version tag, the profile.json object
// verbatim, (only for a custom hero) the packed image, else null, and the hero's
// cutout when the card stands one in front of it.
type ryoEnvelope struct {
	Ryoprofile int             `json:"ryoprofile"`
	Profile    json.RawMessage `json:"profile"`
	Hero       *ryoHero        `json:"hero"`
	Cutout     *ryoHero        `json:"cutout,omitempty"`
}

// exportProfile packs profile.json (and a custom hero image, if any) into a
// single .ryoprofile envelope at dstPath. A missing profile.json is treated as
// an empty object.
func exportProfile(dstPath string) error {
	raw, err := readProfileFile(profileConfigPath(), maxProfileBytes)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		raw = []byte("{}")
	}
	var m profileModel
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("profile.json is not valid JSON: %w", err)
	}
	env := ryoEnvelope{Ryoprofile: 1, Profile: json.RawMessage(raw)}
	if m.Hero.Kind == "custom" && m.Hero.Source != "" {
		name := m.Hero.Source
		if !profilePictureName.MatchString(name) && !profileMovingName.MatchString(name) || len(name) > 128 {
			return fmt.Errorf("custom hero needs a bare image filename")
		}
		var img []byte
		if profileMovingName.MatchString(name) {
			// a picture that may move travels as one still frame, so the design stays a picture
			if img, err = heroStill(name); err != nil {
				return fmt.Errorf("custom hero image: %w", err)
			}
			if env.Profile, err = profileWithHero(env.Profile, "source", "hero.png"); err != nil {
				return err
			}
			name = "hero.png"
		} else {
			if img, err = readPictureFile(filepath.Join(profileHeroDir(), name)); err != nil {
				return err
			}
			if _, err := decodePicture(img); err != nil {
				return fmt.Errorf("custom hero image: %w", err)
			}
		}
		env.Hero = &ryoHero{Name: name, B64: base64.StdEncoding.EncodeToString(img)}
		// a cutout that can't be read stays home, and the importer draws the card without one
		if m.Hero.Cutout {
			if cutout, err := readPictureFile(filepath.Join(profileHeroDir(), profileCutoutName)); err == nil {
				if clean, _, err := cutoutImage(cutout); err == nil {
					env.Cutout = &ryoHero{Name: profileCutoutName, B64: base64.StdEncoding.EncodeToString(clean)}
				}
			}
		}
	}
	out, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return atomicWrite(dstPath, out, 0o644)
}

// importProfile validates a .ryoprofile and unpacks its hero and settings.
// Everything is decoded before any write, so a rejected file leaves the
// existing profile untouched. Shared pictures belong in the binder instead.
func importProfile(srcPath string) error {
	raw, err := readProfileFile(srcPath, maxProfileEnvelopeBytes)
	if err != nil {
		return err
	}
	var env ryoEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("not a valid .ryoprofile file: %w", err)
	}
	if env.Ryoprofile != 1 {
		return fmt.Errorf("not a .ryoprofile file (ryoprofile != 1)")
	}
	profile := env.Profile
	if len(profile) == 0 {
		profile = []byte("{}")
	}
	model, err := validateProfile(profile)
	if err != nil {
		return fmt.Errorf("invalid .ryoprofile settings: %w", err)
	}
	var heroBytes []byte
	heroPath := ""
	if env.Hero != nil {
		if model.Hero.Kind != "custom" || env.Hero.Name != model.Hero.Source || !profilePictureName.MatchString(env.Hero.Name) {
			return fmt.Errorf(".ryoprofile hero must match its custom source filename")
		}
		if len(env.Hero.B64) > base64.StdEncoding.EncodedLen(maxPictureBytes) {
			return fmt.Errorf(".ryoprofile hero image is too large")
		}
		heroBytes, err = base64.StdEncoding.DecodeString(env.Hero.B64)
		if err != nil {
			return fmt.Errorf(".ryoprofile hero image is not valid base64: %w", err)
		}
		img, err := decodePicture(heroBytes)
		if err != nil {
			return fmt.Errorf(".ryoprofile hero image: %w", err)
		}
		// Store only pixels, not a second decoder's animation or metadata input.
		var clean bytes.Buffer
		if err := png.Encode(&clean, img); err != nil {
			return fmt.Errorf(".ryoprofile hero PNG: %w", err)
		}
		heroBytes = clean.Bytes()
		if len(heroBytes) > maxPictureBytes {
			return fmt.Errorf(".ryoprofile hero PNG is too large")
		}
		if model.Hero.Source != "hero.png" {
			if profile, err = profileWithHero(profile, "source", "hero.png"); err != nil {
				return err
			}
		}
		heroPath = filepath.Join(profileHeroDir(), "hero.png")
	} else if model.Hero.Kind == "custom" {
		return fmt.Errorf(".ryoprofile custom hero image is missing")
	}
	cutoutBytes, cutoutPath := []byte(nil), ""
	if model.Hero.Cutout {
		if model.Hero.Kind == "custom" {
			cutoutBytes = packedCutout(env.Cutout)
		}
		if cutoutBytes == nil {
			if profile, err = profileWithHero(profile, "cutout", false); err != nil {
				return err
			}
		} else {
			cutoutPath = filepath.Join(profileHeroDir(), profileCutoutName)
		}
	}
	// the pictures land before profile.json, which is what points the card at them
	var paths []string
	var files [][]byte
	if heroPath != "" {
		paths, files = append(paths, heroPath), append(files, heroBytes)
	}
	if cutoutPath != "" {
		paths, files = append(paths, cutoutPath), append(files, cutoutBytes)
	}
	paths, files = append(paths, profileConfigPath()), append(files, profile)
	before, err := snapshotFiles(paths...)
	if err != nil {
		return err
	}
	for i, path := range paths {
		if err := atomicWrite(path, files[i], 0o644); err != nil {
			if rerr := before.restore(); rerr != nil {
				return errors.Join(err, fmt.Errorf("restore profile: %w", rerr))
			}
			return err
		}
	}
	return nil
}

func runProfile(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("profile needs export|import|card|code|picture|cutout|wallpaper")
	}
	switch args[0] {
	case "export":
		if len(args) < 2 {
			return fmt.Errorf("profile export needs a path")
		}
		return exportProfile(args[1])
	case "import":
		if len(args) < 2 {
			return fmt.Errorf("profile import needs a .ryoprofile path")
		}
		return importProfile(args[1])
	case "code":
		return printCardCode(args[1:])
	case "card":
		if len(args) < 3 {
			return fmt.Errorf("profile card needs the rendered picture and a path or --copy")
		}
		return writeCard(args[1], args[2])
	case "picture":
		if len(args) != 2 {
			return fmt.Errorf("profile picture needs the chosen file's path")
		}
		saved, err := savePicture(args[1])
		if err != nil {
			return err
		}
		return printJSON(saved)
	case "cutout":
		return runCutout(args[1:])
	case "wallpaper":
		return printJSON(liveWallpaper())
	default:
		return fmt.Errorf("unknown profile subcommand: %s", args[0])
	}
}
