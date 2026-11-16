package main

// The card code: a system card's design carried in its pixels, so it survives a chat
// app that re-encodes the picture and drops everything but the pixels. The card draws
// three tick rulers (the measuring rule ryoku.dev and Home use); each tick's height is
// one of four, two bits a tick. The first and last tick of every ruler stand full
// height, so a reader can tell ink from paper on any theme and measure heights at any
// scale. The bits spell a short design (the words, the figure, the layout) with a
// length and a CRC, padded with filler so every card's rulers look alike in kind, and
// Reed-Solomon parity (reedsolomon.go) repairs ticks a resize or recompression misread. A card
// with the reader's own picture carries the picture as it shows in the card's window,
// lifted from the pixels on the way back.
//
// Geometry is in the card's design units (630 x 880); `ryoku-hub profile code` hands
// it with the levels to the card that draws them (components/about/TradingCard.qml).

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"strings"
	"unicode/utf8"
)

const (
	cardW       = 630.0
	cardH       = 880.0
	rulerX      = 40.0
	rulerPitch  = 4.0
	rulerTick   = 2.0
	rulerCount  = 138
	rulerUnit   = 2.5 // a level's height step; level n stands (n+1) units tall
	codeVersion = 1
	codeParity  = 20 // parity bytes: up to 10 misread bytes are repaired
)

// the rulers' tops, in design units; the card keeps these lines free for them
var rulerY = []float64{552, 622, 678}

// the card's window, inside its stroke, where the reader's own picture fills it
var windowRect = image.Rect(35, 112, 35+560, 112+390)

var errNoCardCode = errors.New("this picture's card code could not be read: share the whole card picture at full size, not a crop or part of a screenshot")

type cardCode struct {
	X      float64   `json:"x"`
	Ys     []float64 `json:"ys"`
	Pitch  float64   `json:"pitch"`
	Tick   float64   `json:"tick"`
	Unit   float64   `json:"unit"`
	Levels [][]int   `json:"levels"`
}

// the design a card code carries, read from and written to profile.json's shape
type codedDesign struct {
	kind       int // 0 stock, 1 gallery, 2 own picture
	left       bool
	cardHome   bool
	minimal    bool
	epithets   bool
	telemetry  bool
	specs      bool
	name       string
	tagline    string
	epithet    string
	gallerySrc string
}

func dataTicks() int { return len(rulerY) * (rulerCount - 2) }

func designFromProfile(raw []byte) (codedDesign, error) {
	var p struct {
		Hero struct {
			Kind   string `json:"kind"`
			Source string `json:"source"`
		} `json:"hero"`
		HeroSide string          `json:"heroSide"`
		HomeView string          `json:"homeView"`
		Preset   string          `json:"preset"`
		Blocks   map[string]bool `json:"blocks"`
		Text     struct {
			Name     string   `json:"name"`
			Tagline  string   `json:"tagline"`
			Epithets []string `json:"epithets"`
		} `json:"text"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return codedDesign{}, err
	}
	on := func(k string) bool {
		v, ok := p.Blocks[k]
		return !ok || v
	}
	d := codedDesign{
		left: p.HeroSide == "left", cardHome: p.HomeView == "card", minimal: p.Preset == "minimal",
		epithets: on("epithets"), telemetry: on("telemetry"), specs: on("specs"),
		name: clip(p.Text.Name, 16), tagline: clip(p.Text.Tagline, 36),
	}
	if len(p.Text.Epithets) > 0 {
		d.epithet = clip(p.Text.Epithets[0], 9)
	}
	switch p.Hero.Kind {
	case "gallery":
		d.kind, d.gallerySrc = 1, clip(strings.TrimSuffix(p.Hero.Source, ".png"), 12)
	case "custom":
		d.kind = 2
	}
	return d, nil
}

// clip cuts s to at most n bytes without splitting a character
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

func b2i(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func (d codedDesign) bytes() []byte {
	out := []byte{
		codeVersion<<4 | byte(d.kind)<<2 | b2i(d.left)<<1 | b2i(d.cardHome),
		b2i(d.minimal)<<3 | b2i(d.epithets)<<2 | b2i(d.telemetry)<<1 | b2i(d.specs),
	}
	for _, s := range []string{d.name, d.tagline, d.epithet, d.gallerySrc} {
		out = append(out, byte(len(s)))
		out = append(out, s...)
	}
	return out
}

func parseDesign(b []byte) (codedDesign, error) {
	if len(b) < 2 || b[0]>>4 != codeVersion {
		return codedDesign{}, errNoCardCode
	}
	d := codedDesign{
		kind: int(b[0]>>2) & 3, left: b[0]&2 != 0, cardHome: b[0]&1 != 0,
		minimal: b[1]&8 != 0, epithets: b[1]&4 != 0, telemetry: b[1]&2 != 0, specs: b[1]&1 != 0,
	}
	rest := b[2:]
	var strs [4]string
	for i := range strs {
		if len(rest) < 1 || int(rest[0]) > len(rest)-1 {
			return codedDesign{}, errNoCardCode
		}
		n := int(rest[0])
		strs[i] = string(rest[1 : 1+n])
		rest = rest[1+n:]
	}
	d.name, d.tagline, d.epithet, d.gallerySrc = strs[0], strs[1], strs[2], strs[3]
	return d, nil
}

func crc16(b []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, c := range b {
		crc ^= uint16(c) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func codeBytes() int { return dataTicks() / 4 }

// encodeLevels lays the design out as tick levels, ruler by ruler
func encodeLevels(d codedDesign) ([][]int, error) {
	body := d.bytes()
	frame := append([]byte{byte(len(body))}, body...)
	c := crc16(frame)
	frame = append(frame, byte(c>>8), byte(c))
	room := codeBytes() - codeParity
	if len(frame) > room {
		return nil, fmt.Errorf("the card's words are too long for its code")
	}
	// filler from the CRC, so the rulers past the design look like the rest
	seed := uint32(c) | 1
	for len(frame) < room {
		seed = seed*1664525 + 1013904223
		frame = append(frame, byte(seed>>24))
	}
	frame = rsEncode(frame, codeParity)
	ticks := make([]int, 0, dataTicks())
	for _, b := range frame {
		for shift := 6; shift >= 0; shift -= 2 {
			ticks = append(ticks, int(b>>shift)&3)
		}
	}
	levels := make([][]int, len(rulerY))
	per := rulerCount - 2
	for r := range rulerY {
		row := make([]int, rulerCount)
		row[0], row[rulerCount-1] = 3, 3
		copy(row[1:rulerCount-1], ticks[r*per:(r+1)*per])
		levels[r] = row
	}
	return levels, nil
}

func decodeLevels(levels [][]int) (codedDesign, error) {
	var ticks []int
	for _, row := range levels {
		ticks = append(ticks, row[1:len(row)-1]...)
	}
	var code []byte
	for i := 0; i+3 < len(ticks); i += 4 {
		code = append(code, byte(ticks[i]<<6|ticks[i+1]<<4|ticks[i+2]<<2|ticks[i+3]))
	}
	if len(code) != codeBytes() {
		return codedDesign{}, errNoCardCode
	}
	frame, err := rsDecode(code, codeParity)
	if err != nil {
		return codedDesign{}, errNoCardCode
	}
	n := int(frame[0])
	if n+3 > len(frame) {
		return codedDesign{}, errNoCardCode
	}
	if crc16(frame[:1+n]) != uint16(frame[1+n])<<8|uint16(frame[2+n]) {
		return codedDesign{}, errNoCardCode
	}
	return parseDesign(frame[1 : 1+n])
}

func luma(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 65535
}

// readLevels measures the rulers in a picture of the whole card, at any size
func readLevels(img image.Image) ([][]int, error) {
	bounds := img.Bounds()
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	s := w / cardW
	if s < 0.74 || math.Abs(h/cardH-s) > s*0.03 {
		return nil, errNoCardCode
	}
	at := func(x, y float64) float64 {
		return luma(img.At(bounds.Min.X+int(x*s), bounds.Min.Y+int(y*s)))
	}
	levels := make([][]int, len(rulerY))
	for r, y0 := range rulerY {
		// paper from the gaps between ticks, ink from the full-height ticks at the ends
		var paper, ink float64
		for k := range rulerCount - 1 {
			paper += at(rulerX+float64(k)*rulerPitch+3, y0+5)
		}
		paper /= rulerCount - 1
		for _, k := range []int{0, rulerCount - 1} {
			ink += at(rulerX+float64(k)*rulerPitch+1, y0+5)
		}
		ink /= 2
		if math.Abs(ink-paper) < 0.15 {
			return nil, errNoCardCode
		}
		// each tick's height is the ink it lays down over one period around it: a resize
		// spreads a tick across pixels but keeps its ink, whatever the tick's sub-pixel
		// place. The full-height ticks at the ends set the ink of one level
		px := func(v float64) int { return bounds.Min.X + int(math.Floor(v)) }
		area := make([]float64, rulerCount)
		top, bottom := (y0-1)*s, (y0+4*rulerUnit+1)*s
		for k := range rulerCount {
			left := (rulerX + float64(k)*rulerPitch - (rulerPitch-rulerTick)/2) * s
			right := left + rulerPitch*s
			for x := px(left); float64(x-bounds.Min.X) < right; x++ {
				fx := float64(x - bounds.Min.X)
				wx := math.Min(fx+1, right) - math.Max(fx, left)
				if wx <= 0 {
					continue
				}
				for y := int(math.Floor(top)); float64(y) < bottom; y++ {
					fy := float64(y)
					wy := math.Min(fy+1, bottom) - math.Max(fy, top)
					if wy <= 0 {
						continue
					}
					v := (luma(img.At(x, bounds.Min.Y+y)) - paper) / (ink - paper)
					area[k] += wx * wy * v
				}
			}
		}
		perLevel := (area[0] + area[rulerCount-1]) / 2 / 4
		if perLevel <= 0 {
			return nil, errNoCardCode
		}
		row := make([]int, rulerCount)
		for k := range rulerCount {
			row[k] = int(math.Max(0, math.Min(3, math.Round(area[k]/perLevel)-1)))
		}
		levels[r] = row
	}
	return levels, nil
}

// envelopeFromPixels reads a card picture's code and builds the .ryoprofile it stands
// for; an own picture is lifted from the card's window
func envelopeFromPixels(raw []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("that picture could not be read: %w", err)
	}
	levels, err := readLevels(img)
	if err != nil {
		return nil, err
	}
	d, err := decodeLevels(levels)
	if err != nil {
		return nil, err
	}
	prof := map[string]any{}
	hero := map[string]any{"kind": "default", "source": ""}
	env := ryoEnvelope{Ryoprofile: 1}
	switch d.kind {
	case 1:
		hero = map[string]any{"kind": "gallery", "source": d.gallerySrc + ".png"}
	case 2:
		s := float64(img.Bounds().Dx()) / cardW
		r := image.Rect(int(float64(windowRect.Min.X)*s), int(float64(windowRect.Min.Y)*s),
			int(float64(windowRect.Max.X)*s), int(float64(windowRect.Max.Y)*s)).Add(img.Bounds().Min)
		sub, ok := img.(interface {
			SubImage(image.Rectangle) image.Image
		})
		if !ok {
			return nil, errNoCardCode
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, sub.SubImage(r)); err != nil {
			return nil, err
		}
		env.Hero = &ryoHero{Name: "hero.png", B64: base64.StdEncoding.EncodeToString(buf.Bytes())}
		hero = map[string]any{"kind": "custom", "source": "hero.png", "style": "photo", "fit": false,
			"zoom": 1, "focalX": 0.5, "focalY": 0.5}
	}
	prof["hero"] = hero
	if d.left {
		prof["heroSide"] = "left"
	}
	if d.cardHome {
		prof["homeView"] = "card"
	}
	if d.minimal {
		prof["preset"] = "minimal"
	}
	prof["blocks"] = map[string]bool{"epithets": d.epithets, "telemetry": d.telemetry, "specs": d.specs}
	text := map[string]any{}
	if d.name != "" {
		text["name"] = d.name
	}
	if d.tagline != "" {
		text["tagline"] = d.tagline
	}
	if d.epithet != "" {
		text["epithets"] = []string{d.epithet}
	}
	if len(text) > 0 {
		prof["text"] = text
	}
	env.Profile, err = json.Marshal(prof)
	if err != nil {
		return nil, err
	}
	return json.Marshal(env)
}

// printCardCode prints the rulers' geometry and levels for the current profile
func printCardCode() error {
	raw, err := os.ReadFile(profileConfigPath())
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		raw = []byte("{}")
	}
	d, err := designFromProfile(raw)
	if err != nil {
		return fmt.Errorf("profile.json is not valid JSON: %w", err)
	}
	levels, err := encodeLevels(d)
	if err != nil {
		return err
	}
	return printJSON(cardCode{X: rulerX, Ys: rulerY, Pitch: rulerPitch, Tick: rulerTick, Unit: rulerUnit, Levels: levels})
}
