package main

// The card code carries a friend's card in pixels, so it survives a chat app
// re-encoding the picture. Three tick rulers each carry two bits per tick, with
// full-height ends to calibrate ink and scale. A length, CRC and Reed-Solomon
// parity protect the words, figure, hardware and machine fingerprint.
//
// Geometry is in the card's design units (630 x 880); `ryoku-hub profile code`
// hands it with the levels to components/about/TradingCard.qml.

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"io"
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
	codeVersion = 2
	codeParity  = 20 // parity bytes: up to 10 misread bytes are repaired
)

// the rulers' tops, in design units; each heads a band the card sets one move in, clear
// of the tallest tick
var rulerY = []float64{550, 608, 666}

var machineIDPath = "/etc/machine-id"

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

type codedDesign struct {
	kind       int // 0 stock, 1 gallery, 2 own picture
	threads    int
	ramGb      int
	refresh    int
	width      int
	gpuClass   int
	distro     int
	wm         int
	owner      [3]byte
	name       string
	tagline    string
	epithet    string
	gallerySrc string
	gpu        string
}

func ownerFingerprint() ([3]byte, error) {
	raw, err := os.ReadFile(machineIDPath)
	if err != nil {
		return [3]byte{}, err
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(string(raw))))
	return [3]byte{sum[0], sum[1], sum[2]}, nil
}

func dataTicks() int { return len(rulerY) * (rulerCount - 2) }

func designFromProfile(raw []byte) (codedDesign, error) {
	var p struct {
		Hero struct {
			Kind   string `json:"kind"`
			Source string `json:"source"`
		} `json:"hero"`
		Text struct {
			Name     string   `json:"name"`
			Tagline  string   `json:"tagline"`
			Epithets []string `json:"epithets"`
		} `json:"text"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return codedDesign{}, err
	}
	d := codedDesign{name: clip(p.Text.Name, 14), tagline: clip(p.Text.Tagline, 22)}
	if d.name == "" {
		d.name = clip(os.Getenv("USER"), 14)
	}
	if len(p.Text.Epithets) > 0 {
		d.epithet = clip(p.Text.Epithets[0], 8)
	}
	switch p.Hero.Kind {
	case "gallery":
		if !validGalleryName(strings.TrimSuffix(p.Hero.Source, ".png")) {
			return codedDesign{}, fmt.Errorf("invalid gallery figure")
		}
		d.kind, d.gallerySrc = 1, clip(strings.TrimSuffix(p.Hero.Source, ".png"), 10)
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

func shortGPU(s string) string {
	for _, part := range []string{"NVIDIA ", "GeForce ", "AMD ", "Radeon ", "Intel ", " Laptop GPU", " Max-Q"} {
		s = strings.ReplaceAll(s, part, "")
	}
	return clip(strings.TrimSpace(s), 10)
}

func (d codedDesign) bytes() []byte {
	out := []byte{
		codeVersion<<4 | byte(d.kind)<<2,
		byte(min(255, max(0, d.threads))),
		byte(min(255, max(0, d.ramGb))),
		byte(min(255, max(0, d.refresh/2))),
		byte(min(255, max(0, d.width/32))),
		byte(min(5, max(0, d.gpuClass)))<<5 | byte(min(5, max(0, d.distro)))<<2 | byte(min(3, max(0, d.wm))),
		d.owner[0], d.owner[1], d.owner[2],
	}
	// Nine fixed bytes, five length bytes and at most 64 string bytes: 78
	// bytes, leaving one byte spare in the 79-byte body budget.
	for _, s := range []string{clip(d.name, 14), clip(d.tagline, 22), clip(d.epithet, 8), clip(d.gallerySrc, 10), shortGPU(d.gpu)} {
		out = append(out, byte(len(s)))
		out = append(out, s...)
	}
	return out
}

func parseDesign(b []byte) (codedDesign, error) {
	if len(b) < 14 || len(b) > codeBytes()-codeParity-3 || b[0]>>4 != codeVersion || b[0]&3 != 0 || b[0]>>2&3 > 2 || b[5]>>5 > 5 || b[5]>>2&7 > 5 {
		return codedDesign{}, errNoCardCode
	}
	d := codedDesign{
		kind: int(b[0]>>2) & 3, threads: int(b[1]), ramGb: int(b[2]),
		refresh: int(b[3]) * 2, width: int(b[4]) * 32,
		gpuClass: int(b[5] >> 5), distro: int(b[5]>>2) & 7, wm: int(b[5]) & 3,
		owner: [3]byte{b[6], b[7], b[8]},
	}
	rest := b[9:]
	var strs [5]string
	for i, limit := range []int{14, 22, 8, 10, 10} {
		if len(rest) < 1 || int(rest[0]) > len(rest)-1 || int(rest[0]) > limit {
			return codedDesign{}, errNoCardCode
		}
		n := int(rest[0])
		strs[i] = string(rest[1 : 1+n])
		if !utf8.ValidString(strs[i]) {
			return codedDesign{}, errNoCardCode
		}
		if i != 3 {
			strs[i] = cleanCardText(strs[i])
		}
		rest = rest[1+n:]
	}
	if len(rest) != 0 {
		return codedDesign{}, errNoCardCode
	}
	d.name, d.tagline, d.epithet, d.gallerySrc, d.gpu = strs[0], strs[1], strs[2], strs[3], strs[4]
	if d.kind == 1 && !validGalleryName(d.gallerySrc) {
		return codedDesign{}, errNoCardCode
	}
	if d.gallerySrc != "" && !validGalleryName(d.gallerySrc) {
		return codedDesign{}, errNoCardCode
	}
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
	if len(levels) != len(rulerY) {
		return codedDesign{}, errNoCardCode
	}
	ticks := make([]int, 0, dataTicks())
	for _, row := range levels {
		if len(row) != rulerCount {
			return codedDesign{}, errNoCardCode
		}
		for _, level := range row {
			if level < 0 || level > 3 {
				return codedDesign{}, errNoCardCode
			}
		}
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
	if img == nil {
		return nil, errNoCardCode
	}
	bounds := img.Bounds()
	if !validPictureSize(bounds.Dx(), bounds.Dy()) {
		return nil, errNoCardCode
	}
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

// printCardCode prints the rulers' geometry and levels for the current profile.
func printCardCode(args []string) error {
	flags := flag.NewFlagSet("profile code", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var d codedDesign
	var distro, wm string
	flags.IntVar(&d.threads, "threads", 0, "")
	flags.IntVar(&d.ramGb, "ram", 0, "")
	flags.IntVar(&d.refresh, "refresh", 0, "")
	flags.IntVar(&d.width, "width", 0, "")
	flags.IntVar(&d.gpuClass, "gpu-class", 0, "")
	flags.StringVar(&d.gpu, "gpu", "", "")
	flags.StringVar(&distro, "distro", "", "")
	flags.StringVar(&wm, "wm", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("profile code accepts hardware flags only")
	}
	d.distro = map[string]int{"arch": 1, "cachyos": 2, "fedora": 3, "void": 4, "nixos": 5}[distro]
	d.wm = map[string]int{"hyprland": 1, "niri": 2, "wayfire": 3}[wm]
	var err error
	d.owner, err = ownerFingerprint()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(profileConfigPath())
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		raw = []byte("{}")
	}
	profile, err := designFromProfile(raw)
	if err != nil {
		return fmt.Errorf("profile.json is not valid JSON: %w", err)
	}
	d.kind, d.name, d.tagline, d.epithet, d.gallerySrc = profile.kind, profile.name, profile.tagline, profile.epithet, profile.gallerySrc
	levels, err := encodeLevels(d)
	if err != nil {
		return err
	}
	return printJSON(cardCode{X: rulerX, Ys: rulerY, Pitch: rulerPitch, Tick: rulerTick, Unit: rulerUnit, Levels: levels})
}
