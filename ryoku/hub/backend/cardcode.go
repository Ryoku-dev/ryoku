package main

// The card code carries a friend's card in pixels, so it survives a chat app
// re-encoding the picture. Three tick rulers each carry two bits per tick, with
// full-height ends to calibrate ink and scale. A length, CRC and Reed-Solomon
// parity protect the words, figure, hardware and device-derived card identity.
//
// Geometry is in the card's design units (630 x 880); `ryoku-hub profile code`
// hands it with the levels to components/about/TradingCard.qml.

import (
	"encoding/hex"
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
	codeVersion = 4
	// version 3 has the same body without the integrated-graphics bit; it is still read
	oldestReadVersion = 3
	codeParity        = 20 // parity bytes: up to 10 misread bytes are repaired
)

// the rulers' tops, in design units; each heads a band the card sets one move in, clear
// of the tallest tick
var rulerY = []float64{550, 608, 666}

// the card's window, inside its stroke, where the reader's own picture fills it
var windowRect = image.Rect(35, 112, 35+560, 112+390)

var errNoCardCode = errors.New("this picture's card code could not be read: share the whole card picture at full size, not a crop or part of a screenshot")

// a card that decodes cleanly but carries another version of the code, so one side has to update
var (
	errOldCardCode = errors.New("this card was made by an older Ryoku: ask its owner to update Ryoku and share the card again")
	errNewCardCode = errors.New("this card was made by a newer Ryoku: update Ryoku, then add the card again")
)

// below about 470 pixels across a 2-unit tick is narrower than a pixel and no reading recovers
// the code; a card-shaped picture that small is usually a chat app's preview, not the card
var errSmallCardCode = errors.New("this copy of the card is too small to read: open the picture at full size, or save it, and add it again")

type cardCode struct {
	CardID string    `json:"cardId"`
	X      float64   `json:"x"`
	Ys     []float64 `json:"ys"`
	Pitch  float64   `json:"pitch"`
	Tick   float64   `json:"tick"`
	Unit   float64   `json:"unit"`
	Levels [][]int   `json:"levels"`
}

type codedDesign struct {
	kind     int // 0 stock, 1 gallery, 2 own picture
	threads  int
	ramGb    int
	refresh  int
	width    int
	gpuClass int
	// built-in graphics beside a discrete card: one more rarity point
	igpu       bool
	distro     int
	wm         int
	cardID     [8]byte
	name       string
	tagline    string
	epithet    string
	gallerySrc string
	gpu        string
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
	d := codedDesign{name: clip(p.Text.Name, 14), tagline: clip(p.Text.Tagline, 20)}
	if d.name == "" {
		d.name = clip(os.Getenv("USER"), 14)
	}
	if len(p.Text.Epithets) > 0 {
		d.epithet = clip(p.Text.Epithets[0], 7)
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
	return clip(strings.TrimSpace(s), 8)
}

func (d codedDesign) bytes() []byte {
	out := []byte{
		codeVersion<<4 | byte(d.kind)<<2 | igpuBit(d.igpu),
		byte(min(255, max(0, d.threads))),
		byte(min(255, max(0, d.ramGb))),
		byte(min(255, max(0, d.refresh/2))),
		byte(min(255, max(0, d.width/32))),
		byte(min(5, max(0, d.gpuClass)))<<5 | byte(min(5, max(0, d.distro)))<<2 | byte(min(3, max(0, d.wm))),
	}
	out = append(out, d.cardID[:]...)
	// Fourteen fixed bytes, five length bytes and at most 59 string bytes fit
	// within the 79-byte body budget without changing the ruler geometry.
	for _, s := range []string{clip(d.name, 14), clip(d.tagline, 20), clip(d.epithet, 7), clip(d.gallerySrc, 10), shortGPU(d.gpu)} {
		out = append(out, byte(len(s)))
		out = append(out, s...)
	}
	return out
}

func igpuBit(on bool) byte {
	if on {
		return 1
	}
	return 0
}

func parseDesign(b []byte) (codedDesign, error) {
	if len(b) > 0 && (b[0]>>4 < oldestReadVersion || b[0]>>4 > codeVersion) {
		switch v := b[0] >> 4; {
		case v > 0 && v < oldestReadVersion:
			return codedDesign{}, errOldCardCode
		case v > codeVersion:
			return codedDesign{}, errNewCardCode
		}
		return codedDesign{}, errNoCardCode
	}
	if len(b) < 19 || len(b) > 78 {
		return codedDesign{}, errNoCardCode
	}
	// the low bit is the built-in graphics flag from version 4; the bit above it stays clear
	spare := byte(2)
	if b[0]>>4 < 4 {
		spare = 3
	}
	if b[0]&spare != 0 || b[0]>>2&3 > 2 || b[5]>>5 > 5 || b[5]>>2&7 > 5 {
		return codedDesign{}, errNoCardCode
	}
	d := codedDesign{
		kind: int(b[0]>>2) & 3, threads: int(b[1]), ramGb: int(b[2]),
		refresh: int(b[3]) * 2, width: int(b[4]) * 32,
		gpuClass: int(b[5] >> 5), igpu: b[0]&1 == 1, distro: int(b[5]>>2) & 7, wm: int(b[5]) & 3,
		cardID: [8]byte(b[6:14]),
	}
	rest := b[14:]
	var strs [5]string
	for i, limit := range []int{14, 20, 7, 10, 8} {
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
	return encodeFrame(d.bytes())
}

// encodeFrame frames a body with its length, CRC and parity and lays it out as tick levels
func encodeFrame(body []byte) ([][]int, error) {
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

// rulerInk is one ruler as the reader measured it: each tick's ink over one pitch around it,
// and its ink width along the first unit, which every tick covers whatever its level
type rulerInk struct {
	area, width []float64
}

// measureRulers measures the rulers in a picture of the whole card, at any size
func measureRulers(img image.Image) ([]rulerInk, error) {
	if img == nil {
		return nil, errNoCardCode
	}
	bounds := img.Bounds()
	if !validPictureSize(bounds.Dx(), bounds.Dy()) {
		return nil, errNoCardCode
	}
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	s := w / cardW
	if math.Abs(h/cardH-s) > s*0.03 {
		return nil, errNoCardCode
	}
	if s < 0.74 {
		return nil, errSmallCardCode
	}
	at := func(x, y float64) float64 {
		return luma(img.At(bounds.Min.X+int(x*s), bounds.Min.Y+int(y*s)))
	}
	rulers := make([]rulerInk, len(rulerY))
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
		// the ink inside a box of the picture, in pixels, edge pixels counted in part: a resize
		// spreads a tick across pixels but keeps its ink, whatever the tick's sub-pixel place
		covered := func(left, right, top, bottom float64) float64 {
			var sum float64
			for x := int(math.Floor(left)); float64(x) < right; x++ {
				wx := math.Min(float64(x+1), right) - math.Max(float64(x), left)
				if wx <= 0 {
					continue
				}
				for y := int(math.Floor(top)); float64(y) < bottom; y++ {
					wy := math.Min(float64(y+1), bottom) - math.Max(float64(y), top)
					if wy <= 0 {
						continue
					}
					sum += wx * wy * (luma(img.At(bounds.Min.X+x, bounds.Min.Y+y)) - paper) / (ink - paper)
				}
			}
			return sum
		}
		ruler := rulerInk{area: make([]float64, rulerCount), width: make([]float64, rulerCount)}
		band := rulerUnit / 2 * s
		for k := range rulerCount {
			left := (rulerX + float64(k)*rulerPitch - (rulerPitch-rulerTick)/2) * s
			right := left + rulerPitch*s
			ruler.area[k] = covered(left, right, (y0-1)*s, (y0+4*rulerUnit+1)*s)
			ruler.width[k] = covered(left, right, (y0+rulerUnit/4)*s, (y0+rulerUnit/4)*s+band) / band
		}
		rulers[r] = ruler
	}
	return rulers, nil
}

// levelsByArea reads each tick's level from its ink alone, which a resize or a blur keeps.
// The full-height ticks at the ends set the ink of one level
func (r rulerInk) levelsByArea() []int {
	return levelsFrom(r.area)
}

// levelsByHeight first divides each tick's ink by its own width. A renderer that snaps edges
// to whole pixels draws a 2-unit tick 5 or 6 pixels wide at a display scale like 133%, and a
// wide tick then lays down the ink of the level above it
func (r rulerInk) levelsByHeight() []int {
	heights := make([]float64, rulerCount)
	for k := range rulerCount {
		heights[k] = r.area[k] / r.width[k]
	}
	return levelsFrom(heights)
}

func levelsFrom(measure []float64) []int {
	per := (measure[0] + measure[rulerCount-1]) / 2 / 4
	row := make([]int, rulerCount)
	for k, m := range measure {
		if v := m / per; per > 0 && !math.IsNaN(v) {
			row[k] = int(math.Max(0, math.Min(3, math.Round(v)-1)))
		}
	}
	return row
}

// readCard reads the code off a picture of a card and decodes it. Ink alone reads a card
// however it was scaled; ink over each tick's own width reads one whose ticks were snapped to
// whole pixels. Reed-Solomon and the CRC accept only a clean read, so trying both is safe
func readCard(img image.Image) ([][]int, codedDesign, error) {
	rulers, err := measureRulers(img)
	if err != nil {
		return nil, codedDesign{}, err
	}
	for _, read := range []func(rulerInk) []int{rulerInk.levelsByArea, rulerInk.levelsByHeight} {
		levels := make([][]int, len(rulers))
		for i, r := range rulers {
			levels[i] = read(r)
		}
		// decoded, or decoded cleanly into another version's card: either way the read is right
		if d, err := decodeLevels(levels); !errors.Is(err, errNoCardCode) {
			return levels, d, err
		}
	}
	return nil, codedDesign{}, errNoCardCode
}

// printCardCode prints the rulers' geometry and levels for the current profile.
func printCardCode(args []string) error {
	d, err := cardDesignArgs(args)
	if err != nil {
		return err
	}
	levels, err := encodeLevels(d)
	if err != nil {
		return err
	}
	return printJSON(cardCode{CardID: hex.EncodeToString(d.cardID[:]), X: rulerX, Ys: rulerY, Pitch: rulerPitch, Tick: rulerTick, Unit: rulerUnit, Levels: levels})
}

// cardDesignArgs builds the card's coded design from the hardware flags Settings passes and the saved
// profile; the rulers and a badge holder's published record both carry exactly this.
func cardDesignArgs(args []string) (codedDesign, error) {
	flags := flag.NewFlagSet("profile code", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var d codedDesign
	var distro, wm string
	flags.IntVar(&d.threads, "threads", 0, "")
	flags.IntVar(&d.ramGb, "ram", 0, "")
	flags.IntVar(&d.refresh, "refresh", 0, "")
	flags.IntVar(&d.width, "width", 0, "")
	flags.IntVar(&d.gpuClass, "gpu-class", 0, "")
	flags.BoolVar(&d.igpu, "igpu", false, "")
	flags.StringVar(&d.gpu, "gpu", "", "")
	flags.StringVar(&distro, "distro", "", "")
	flags.StringVar(&wm, "wm", "", "")
	if err := flags.Parse(args); err != nil {
		return d, err
	}
	if flags.NArg() != 0 {
		return d, fmt.Errorf("profile code accepts hardware flags only")
	}
	d.distro = map[string]int{"arch": 1, "cachyos": 2, "fedora": 3, "void": 4, "nixos": 5}[distro]
	d.wm = map[string]int{"hyprland": 1, "niri": 2, "wayfire": 3}[wm]
	var err error
	d.cardID, err = ownCardID()
	if err != nil {
		return d, err
	}
	raw, err := os.ReadFile(profileConfigPath())
	if err != nil {
		if !os.IsNotExist(err) {
			return d, err
		}
		raw = []byte("{}")
	}
	profile, err := designFromProfile(raw)
	if err != nil {
		return d, fmt.Errorf("profile.json is not valid JSON: %w", err)
	}
	d.kind, d.name, d.tagline, d.epithet, d.gallerySrc = profile.kind, profile.name, profile.tagline, profile.epithet, profile.gallerySrc
	return d, nil
}
