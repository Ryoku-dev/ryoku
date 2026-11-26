package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"
)

const designNoFollow = syscall.O_NOFOLLOW | syscall.O_NONBLOCK
const designPixelLimit int64 = 64_000_000

type designSpec struct {
	Version int         `json:"version"`
	Name    string      `json:"name"`
	Badge   designBadge `json:"badge"`
	Card    designCard  `json:"card"`
}

// the badge part of a design: Ryoku draws the seal itself and only takes the centre mark, the code
// word round the diamond, the kanji and the ink from the pack
type designBadge struct {
	Emblem string `json:"emblem"`
	Title  string `json:"title"`
	Kanji  string `json:"kanji,omitempty"`
	Accent string `json:"accent"`
}
type designPalette struct {
	Ink    string `json:"ink"`
	Accent string `json:"accent"`
	Plate  string `json:"plate"`
	Rim    string `json:"rim"`
}
type designArt struct {
	Image string  `json:"image"`
	Depth float64 `json:"depth"`
	Blend string  `json:"blend"`
}
type designEffect struct {
	ID       string  `json:"id"`
	Strength float64 `json:"strength"`
	Speed    float64 `json:"speed"`
	Color    string  `json:"color"`
}
type designWords struct {
	Tagline string `json:"tagline"`
	Epithet string `json:"epithet"`
}
type designCard struct {
	Palette    designPalette  `json:"palette"`
	Background string         `json:"background,omitempty"`
	Art        []designArt    `json:"art"`
	Frame      string         `json:"frame,omitempty"`
	Glow       string         `json:"glow,omitempty"`
	Effects    []designEffect `json:"effects"`
	Words      designWords    `json:"words"`
}
type designCheckResult struct {
	OK     bool        `json:"ok"`
	Errors []string    `json:"errors"`
	Design *designSpec `json:"design"`
	Files  []string    `json:"files"`
	Bytes  int64       `json:"bytes"`
}
type designResult struct {
	Dir    string     `json:"dir"`
	Design designSpec `json:"design"`
}

var designColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func designText(s *string, min, max int) error {
	if !utf8.ValidString(*s) {
		return fmt.Errorf("invalid UTF-8 text")
	}
	*s = cleanCardText(*s)
	n := utf8.RuneCountInString(*s)
	if n < min || n > max {
		return fmt.Errorf("text length must be %d..%d", min, max)
	}
	return nil
}
func designColor(s *string) error {
	if !designColorPattern.MatchString(*s) {
		return fmt.Errorf("invalid color %q", *s)
	}
	*s = strings.ToLower(*s)
	return nil
}
func designClamp(v float64) float64 { return math.Max(0, math.Min(1, v)) }
func designLuminance(c string) float64 {
	n, _ := strconv.ParseUint(c[1:], 16, 24)
	rgb := []float64{float64(n>>16) / 255, float64(n>>8&255) / 255, float64(n&255) / 255}
	for i, v := range rgb {
		if v <= 0.04045 {
			rgb[i] = v / 12.92
		} else {
			rgb[i] = math.Pow((v+0.055)/1.055, 2.4)
		}
	}
	return rgb[0]*0.2126 + rgb[1]*0.7152 + rgb[2]*0.0722
}
func designImage(raw []byte, name string) (int, int, int64, error) {
	if filepath.Ext(name) == ".webp" {
		info, err := parseWebP(raw)
		// Partial animation updates still decode to full canvases for display.
		return info.Width, info.Height, int64(len(info.Frames)) * int64(info.Width) * int64(info.Height), err
	}
	cfg, kind, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return 0, 0, 0, err
	}
	expected := "png"
	if filepath.Ext(name) == ".jpg" || filepath.Ext(name) == ".jpeg" {
		expected = "jpeg"
	}
	if kind != expected {
		return 0, 0, 0, fmt.Errorf("image kind does not match extension")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 2048 || cfg.Height > 2048 {
		return 0, 0, 0, fmt.Errorf("image canvas exceeds 2048x2048")
	}
	return cfg.Width, cfg.Height, int64(cfg.Width) * int64(cfg.Height), nil
}

func designValidate(p designPack) (designSpec, error) {
	var s designSpec
	raw, ok := p.files["design.json"]
	if !ok {
		return s, fmt.Errorf("missing design.json")
	}
	if int64(len(raw)) > designJSONLimit {
		return s, fmt.Errorf("design.json exceeds 64 KB")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return s, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return s, fmt.Errorf("trailing JSON")
	}
	if s.Version != 1 {
		return s, fmt.Errorf("unsupported design version")
	}
	for _, t := range []struct {
		s        *string
		min, max int
	}{{&s.Name, 1, 24}, {&s.Badge.Title, 1, 12}, {&s.Badge.Kanji, 0, 4}, {&s.Card.Words.Tagline, 0, 36}, {&s.Card.Words.Epithet, 0, 12}} {
		if err := designText(t.s, t.min, t.max); err != nil {
			return s, err
		}
	}
	for _, c := range []*string{&s.Badge.Accent, &s.Card.Palette.Ink, &s.Card.Palette.Accent, &s.Card.Palette.Plate, &s.Card.Palette.Rim} {
		if err := designColor(c); err != nil {
			return s, err
		}
	}
	a, b := designLuminance(s.Card.Palette.Ink), designLuminance(s.Card.Palette.Plate)
	if a < b {
		a, b = b, a
	}
	if (a+0.05)/(b+0.05) < 4.5 {
		return s, fmt.Errorf("ink and plate contrast is below 4.5")
	}
	effectIDs := map[string]bool{"foil": true, "sweep": true, "diamond": true, "hexgrid": true, "circuit": true, "scan": true, "glitch": true, "embers": true, "aurora": true}
	seen := map[string]bool{}
	if len(s.Card.Effects) > 6 {
		return s, fmt.Errorf("too many card effects")
	}
	for i := range s.Card.Effects {
		e := &s.Card.Effects[i]
		if !effectIDs[e.ID] || seen[e.ID] {
			return s, fmt.Errorf("invalid or duplicate card effect")
		}
		seen[e.ID] = true
		if err := designColor(&e.Color); err != nil {
			return s, err
		}
		e.Strength = designClamp(e.Strength)
		e.Speed = designClamp(e.Speed)
	}
	if s.Card.Effects == nil {
		s.Card.Effects = []designEffect{}
	}
	sizes := map[string][2]int{}
	var pixels int64
	// Validate even unused images: an approved pack never hides an unchecked payload.
	for _, name := range p.names {
		if name == "design.json" {
			continue
		}
		w, h, decoded, err := designImage(p.files[name], name)
		if err != nil {
			return s, fmt.Errorf("%s: %w", name, err)
		}
		sizes[name] = [2]int{w, h}
		pixels += decoded
	}
	if pixels > designPixelLimit {
		return s, fmt.Errorf("pack decoded pixels total %d exceeds %d", pixels, designPixelLimit)
	}
	ref := func(v *string, kind string, optional bool) error {
		if *v == "" && optional {
			return nil
		}
		size, exists := sizes[*v]
		if !exists {
			return fmt.Errorf("missing image %q", *v)
		}
		w, h := size[0], size[1]
		switch kind {
		case "emblem":
			if w < 256 || h < 256 || w > 1024 || h > 1024 || math.Abs(float64(w)/float64(h)-1) > 0.1 {
				return fmt.Errorf("emblem must be near-square and 256..1024 px")
			}
		case "frame":
			if w < 630 || h < 880 || math.Abs(float64(w)/float64(h)/(630.0/880)-1) > 0.02 {
				return fmt.Errorf("frame aspect or dimensions invalid")
			}
		case "glow":
			if math.Abs(float64(w)/float64(h)/(560.0/390)-1) > 0.02 {
				return fmt.Errorf("glow aspect invalid")
			}
		}
		*v = filepath.Join(p.root, *v)
		return nil
	}
	if err := ref(&s.Badge.Emblem, "emblem", false); err != nil {
		return s, err
	}
	for _, r := range []struct {
		v    *string
		kind string
	}{{&s.Card.Background, "background"}, {&s.Card.Frame, "frame"}, {&s.Card.Glow, "glow"}} {
		if err := ref(r.v, r.kind, true); err != nil {
			return s, err
		}
	}
	if len(s.Card.Art) < 1 || len(s.Card.Art) > 4 {
		return s, fmt.Errorf("art requires 1..4 layers")
	}
	for i := range s.Card.Art {
		a := &s.Card.Art[i]
		if err := ref(&a.Image, "art", false); err != nil {
			return s, err
		}
		if a.Blend != "normal" && a.Blend != "screen" && a.Blend != "add" {
			return s, fmt.Errorf("invalid art blend")
		}
		a.Depth = designClamp(a.Depth)
	}
	return s, nil
}

func designCheck(path string) (designCheckResult, error) {
	result := designCheckResult{Errors: []string{}, Files: []string{}}
	p, err := designReadPack(path)
	if err == nil {
		p, err = canonicalDesignPack(p, true)
	}
	if err == nil {
		result.Files = p.names
		result.Bytes = p.bytes
		var s designSpec
		s, err = designValidate(p)
		if err == nil {
			result.OK = true
			result.Design = &s
		}
	}
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	return result, err
}
