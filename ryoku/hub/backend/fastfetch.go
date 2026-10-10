package main

// The friendly editor behind the Hub's Fastfetch section. config.jsonc stays the
// source of truth (user-owned, hand-editable, seed-once); this reads it into a
// small model the GUI edits and writes it back, so GUI edits and hand-edits share
// one file. Unknown modules pass through verbatim, so a hand-added line is never
// lost. Custom brand lines (tagline, section headers) are rebuilt from templates
// that reproduce the shipped styling, so a no-op round-trip is stable.

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// fixed brand palette used by the custom-line templates, matching the shipped
// config.jsonc. only the accent is user-facing (it also drives display.color).
const (
	ffTan              = "143;135;112"
	ffBright           = "243;237;225"
	ffDim              = "58;46;36"
	ffAccent           = "226;52;42"
	ffPaletteFixed     = "fixed"
	ffPaletteWallpaper = "wallpaper"
)

var (
	ffAnsiRe      = regexp.MustCompile("\x1b\\[[0-9;]*m")
	ffTanRe       = regexp.MustCompile("\x1b\\[38;2;" + regexp.QuoteMeta(ffTan) + "m(.*?)\x1b\\[0m")
	ffLabelRe     = regexp.MustCompile("\x1b\\[1;38;2;" + regexp.QuoteMeta(ffBright) + "m(.*?)\x1b\\[0m")
	ffKeysColor   = regexp.MustCompile(`(?:38;2;)?([0-9]+;[0-9]+;[0-9]+)`)
	ffRGBTriple   = regexp.MustCompile(`^([0-9]{1,3});([0-9]{1,3});([0-9]{1,3})$`)
	ffTrueColorRe = regexp.MustCompile("\x1b\\[((?:1;)?)38;2;([0-9]+;[0-9]+;[0-9]+)m")
)

func fastfetchDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "fastfetch")
}

func fastfetchConfigPath() string  { return filepath.Join(fastfetchDir(), "config.jsonc") }
func fastfetchPalettePath() string { return filepath.Join(fastfetchDir(), "ryoku-colors.json") }
func fastfetchPaletteStatePath() string {
	return filepath.Join(filepath.Dir(fastfetchDir()), "ryoku", "fastfetch.json")
}

// stripJSONC removes // line and /* */ block comments from a JSONC document,
// leaving comment-like text inside string literals (a $schema URL) intact so
// encoding/json can parse the rest.
func stripJSONC(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inStr, esc, inLine, inBlock, skip := false, false, false, false, false
	for i := range b {
		if skip {
			skip = false
			continue
		}
		c := b[i]
		var next byte
		if i+1 < len(b) {
			next = b[i+1]
		}
		switch {
		case inLine:
			if c == '\n' {
				inLine = false
				out = append(out, c)
			}
		case inBlock:
			if c == '*' && next == '/' {
				inBlock = false
				skip = true
			}
		case inStr:
			out = append(out, c)
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
		case c == '"':
			inStr = true
			out = append(out, c)
		case c == '/' && next == '/':
			inLine = true
			skip = true
		case c == '/' && next == '*':
			inBlock = true
			skip = true
		default:
			out = append(out, c)
		}
	}
	return out
}

// ---- model ------------------------------------------------------------------

// ffLogo carries the emblem's whole cell box. Padding is three values because
// fastfetch treats them differently: left and right shift the emblem and the
// readout sideways, while top pushes the emblem down and leaves the text where
// it is. The Hub preview draws from these same numbers, so a config and its
// preview cannot drift apart.
type ffLogo struct {
	Kind         string `json:"kind"` // image | ascii | builtin | none
	Source       string `json:"source"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Padding      int    `json:"padding"` // left, the one the UI exposes
	PaddingRight int    `json:"paddingRight"`
	PaddingTop   int    `json:"paddingTop"`
	// Dither is on when Source names a baked 1-bit sibling; it rides the source
	// name, never a key in config.jsonc, so fastfetch never sees an unknown field.
	Dither bool `json:"dither"`
}

// ffRow is one readout line. break/colors/title/module keep their original JSON in
// Raw so extra fields (a gpu detectionMethod, a command's echo) survive; tagline
// and header carry editable Text and are rebuilt from a template.
type ffRow struct {
	Kind    string          `json:"kind"` // title|tagline|header|module|break|colors|raw
	Enabled bool            `json:"enabled"`
	Label   string          `json:"label,omitempty"`
	Module  string          `json:"module,omitempty"`
	Key     string          `json:"key,omitempty"`
	Text    string          `json:"text,omitempty"`
	Raw     json.RawMessage `json:"raw,omitempty"`
}

type ffModel struct {
	Logo    ffLogo          `json:"logo"`
	Accent  string          `json:"accent"`
	Palette string          `json:"palette"`
	Rows    []ffRow         `json:"rows"`
	Display json.RawMessage `json:"display,omitempty"`
	Schema  string          `json:"schema,omitempty"`
}

type ffPalette struct {
	Keys    string `json:"keys"`
	Title   string `json:"title"`
	Muted   string `json:"muted"`
	Section string `json:"section"`
	Rule    string `json:"rule"`
	Percent struct {
		Green  string `json:"green"`
		Yellow string `json:"yellow"`
		Red    string `json:"red"`
	} `json:"percent"`
}

type ffPaletteState struct {
	Palette string `json:"palette"`
}

func ffPaletteMode(mode string) string {
	if mode == ffPaletteWallpaper {
		return ffPaletteWallpaper
	}
	return ffPaletteFixed
}

func readFastfetchPaletteMode() string {
	b, err := os.ReadFile(fastfetchPaletteStatePath())
	if err != nil {
		return ffPaletteFixed
	}
	var state ffPaletteState
	if json.Unmarshal(b, &state) != nil {
		return ffPaletteFixed
	}
	return ffPaletteMode(state.Palette)
}

func setFastfetchPaletteMode(mode string) error {
	mode = ffPaletteMode(mode)
	b, err := json.MarshalIndent(ffPaletteState{Palette: mode}, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(fastfetchPaletteStatePath(), append(b, '\n'), 0o644)
}

func validFFRGB(value string) bool {
	match := ffRGBTriple.FindStringSubmatch(value)
	if match == nil {
		return false
	}
	for _, part := range match[1:] {
		n, err := strconv.Atoi(part)
		if err != nil || n > 255 {
			return false
		}
	}
	return true
}

func loadFastfetchPalette() (ffPalette, error) {
	var palette ffPalette
	b, err := os.ReadFile(fastfetchPalettePath())
	if err != nil {
		return palette, fmt.Errorf("read Fastfetch wallpaper palette: %w", err)
	}
	if err := json.Unmarshal(b, &palette); err != nil {
		return palette, fmt.Errorf("parse Fastfetch wallpaper palette: %w", err)
	}
	values := map[string]string{
		"keys": palette.Keys, "title": palette.Title, "muted": palette.Muted,
		"section": palette.Section, "rule": palette.Rule,
		"percent.green": palette.Percent.Green, "percent.yellow": palette.Percent.Yellow,
		"percent.red": palette.Percent.Red,
	}
	for name, value := range values {
		if !validFFRGB(value) {
			return ffPalette{}, fmt.Errorf("Fastfetch wallpaper palette has invalid %s colour", name)
		}
	}
	return palette, nil
}

func runFastfetch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("fastfetch needs get|save|reset|preview|effective|palette|import-logo|remove-logo|dither-logo")
	}
	switch args[0] {
	case "get":
		m, err := loadFastfetch()
		if err != nil {
			return err
		}
		return printJSON(m)
	case "save":
		if len(args) < 2 {
			return fmt.Errorf("fastfetch save needs a JSON argument")
		}
		var m ffModel
		if err := json.Unmarshal([]byte(args[1]), &m); err != nil {
			return err
		}
		m.Palette = ffPaletteMode(m.Palette)
		if m.Palette == ffPaletteWallpaper {
			if _, err := loadFastfetchPalette(); err != nil {
				return err
			}
		}
		b, err := buildFastfetch(m)
		if err != nil {
			return err
		}
		if err := atomicWrite(fastfetchConfigPath(), b, 0o644); err != nil {
			return err
		}
		return setFastfetchPaletteMode(m.Palette)
	case "preview":
		if len(args) < 2 {
			return fmt.Errorf("fastfetch preview needs a JSON argument")
		}
		var m ffModel
		if err := json.Unmarshal([]byte(args[1]), &m); err != nil {
			return err
		}
		return previewFastfetch(m)
	case "effective":
		if len(args) != 2 {
			return fmt.Errorf("fastfetch effective needs an output path")
		}
		return writeEffectiveFastfetch(args[1])
	case "palette":
		if len(args) != 2 || (args[1] != ffPaletteFixed && args[1] != ffPaletteWallpaper) {
			return fmt.Errorf("fastfetch palette needs fixed or wallpaper")
		}
		if args[1] == ffPaletteWallpaper {
			if _, err := loadFastfetchPalette(); err != nil {
				return err
			}
		}
		return setFastfetchPaletteMode(args[1])
	case "import-logo":
		if len(args) < 2 {
			return fmt.Errorf("fastfetch import-logo needs a path")
		}
		p, err := importFastfetchLogo(args[1])
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	case "remove-logo":
		removed, err := removeFastfetchLogo()
		if err != nil {
			return err
		}
		if removed != "" {
			fmt.Println(removed)
		}
		return nil
	case "dither-logo":
		if len(args) < 3 {
			return fmt.Errorf("fastfetch dither-logo needs <on|off> and a source path")
		}
		p, err := ditherFastfetchLogo(args[1], args[2])
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	case "reset":
		m := defaultFastfetchModel()
		m.Palette = readFastfetchPaletteMode()
		b, err := buildFastfetch(m)
		if err != nil {
			return err
		}
		return atomicWrite(fastfetchConfigPath(), b, 0o644)
	default:
		return fmt.Errorf("unknown fastfetch subcommand: %s", args[0])
	}
}

// ---- read: config.jsonc -> model --------------------------------------------

func loadFastfetch() (ffModel, error) {
	raw, err := os.ReadFile(fastfetchConfigPath())
	if err != nil {
		m := defaultFastfetchModel()
		m.Palette = readFastfetchPaletteMode()
		return m, nil
	}
	var doc struct {
		Schema  string            `json:"$schema"`
		Logo    map[string]any    `json:"logo"`
		Display json.RawMessage   `json:"display"`
		Modules []json.RawMessage `json:"modules"`
	}
	if err := json.Unmarshal(stripJSONC(raw), &doc); err != nil {
		return ffModel{}, fmt.Errorf("parse config.jsonc: %w", err)
	}
	m := ffModel{Schema: doc.Schema, Display: doc.Display, Accent: ffDisplayAccent(doc.Display), Palette: readFastfetchPaletteMode(), Logo: ffNormalizeLogo(doc.Logo)}
	for _, rm := range doc.Modules {
		m.Rows = append(m.Rows, ffNormalizeModule(rm))
	}
	return m, nil
}

func ffNormalizeLogo(l map[string]any) ffLogo {
	out := ffLogo{Kind: "none", Width: 28, Height: 14, Padding: 3, PaddingRight: 5, PaddingTop: 5}
	if l == nil {
		return out
	}
	t, _ := l["type"].(string)
	switch t {
	case "kitty", "kitty-direct", "kitty-icat", "chafa", "sixel", "iterm", "raw":
		out.Kind = "image"
	case "file", "file-raw", "data", "data-raw":
		out.Kind = "ascii"
	case "builtin", "small":
		out.Kind = "builtin"
	default:
		out.Kind = "none"
	}
	if s, ok := l["source"].(string); ok {
		out.Source = s
	}
	if w, ok := l["width"].(float64); ok {
		out.Width = int(w)
	}
	if h, ok := l["height"].(float64); ok {
		out.Height = int(h)
	}
	switch p := l["padding"].(type) {
	case float64:
		out.Padding = int(p)
	case map[string]any:
		if v, ok := p["left"].(float64); ok {
			out.Padding = int(v)
		}
		if v, ok := p["right"].(float64); ok {
			out.PaddingRight = int(v)
		}
		if v, ok := p["top"].(float64); ok {
			out.PaddingTop = int(v)
		}
	}
	out.Dither = ffIsDithered(out.Source)
	return out
}

func ffNormalizeModule(rm json.RawMessage) ffRow {
	s := strings.TrimSpace(string(rm))
	if strings.HasPrefix(s, "\"") { // a bare string module ("break")
		var str string
		_ = json.Unmarshal(rm, &str)
		return ffRow{Kind: "break", Enabled: true, Label: "Spacer", Raw: rm}
	}
	var obj struct {
		Type   string `json:"type"`
		Key    string `json:"key"`
		Format string `json:"format"`
	}
	_ = json.Unmarshal(rm, &obj)
	switch obj.Type {
	case "title":
		return ffRow{Kind: "title", Enabled: true, Label: "Host / user", Raw: rm}
	case "custom":
		if strings.Contains(ffStripAnsi(obj.Format), "─") {
			return ffRow{Kind: "header", Enabled: true, Text: ffHeaderLabel(obj.Format), Label: "Section header", Raw: rm}
		}
		return ffRow{Kind: "tagline", Enabled: true, Text: ffTaglineText(obj.Format), Label: "Tagline", Raw: rm}
	case "colors":
		return ffRow{Kind: "colors", Enabled: true, Label: "Colour swatches", Raw: rm}
	default:
		label := obj.Key
		if label == "" {
			label = ffTitleCase(obj.Type)
		}
		return ffRow{Kind: "module", Enabled: true, Module: obj.Type, Key: obj.Key, Label: label, Raw: rm}
	}
}

func ffStripAnsi(s string) string { return ffAnsiRe.ReplaceAllString(s, "") }

func ffTaglineText(format string) string {
	if mm := ffTanRe.FindStringSubmatch(format); mm != nil {
		return mm[1]
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ffStripAnsi(format)), "■"))
}

func ffHeaderLabel(format string) string {
	if mm := ffLabelRe.FindStringSubmatch(format); mm != nil {
		return mm[1]
	}
	return strings.TrimSpace(strings.Trim(ffStripAnsi(format), "─ "))
}

// ffDisplayAccent lifts the "R;G;B" of the readout keys out of the display block,
// so the templates and a colour picker share one accent. Defaults to the brand
// vermilion when the block is missing or unparseable.
func ffDisplayAccent(display json.RawMessage) string {
	if len(display) == 0 {
		return ffAccent
	}
	var d struct {
		Color json.RawMessage `json:"color"`
	}
	if json.Unmarshal(display, &d) != nil {
		return ffAccent
	}
	var keys string
	var cobj struct {
		Keys string `json:"keys"`
	}
	if json.Unmarshal(d.Color, &cobj) == nil && cobj.Keys != "" {
		keys = cobj.Keys
	} else {
		_ = json.Unmarshal(d.Color, &keys)
	}
	if mm := ffKeysColor.FindStringSubmatch(keys); mm != nil {
		return mm[1]
	}
	return ffAccent
}

// ---- write: model -> config.jsonc -------------------------------------------

type ffOutLogo struct {
	Type    string         `json:"type"`
	Source  string         `json:"source,omitempty"`
	Width   int            `json:"width,omitempty"`
	Height  int            `json:"height,omitempty"`
	Padding map[string]int `json:"padding,omitempty"`
}

type ffOutConfig struct {
	Schema  string            `json:"$schema,omitempty"`
	Logo    ffOutLogo         `json:"logo"`
	Display json.RawMessage   `json:"display,omitempty"`
	Modules []json.RawMessage `json:"modules"`
}

func buildFastfetch(m ffModel) ([]byte, error) {
	accent := m.Accent
	if ffKeysColor.FindStringSubmatch(accent) == nil {
		accent = ffAccent
	}
	var mods []json.RawMessage
	for _, r := range m.Rows {
		if !r.Enabled {
			continue
		}
		rm, err := ffBuildRow(r, accent)
		if err != nil {
			return nil, err
		}
		if rm != nil {
			mods = append(mods, rm)
		}
	}
	out := ffOutConfig{Schema: m.Schema, Display: ffApplyAccentToDisplay(m.Display, accent), Modules: mods, Logo: ffBuildLogo(m.Logo)}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func ffApplyPaletteToRow(raw json.RawMessage, palette ffPalette) (json.RawMessage, error) {
	if len(raw) == 0 || raw[0] == '"' {
		return raw, nil
	}
	var row map[string]any
	if err := json.Unmarshal(raw, &row); err != nil {
		return raw, nil
	}
	format, ok := row["format"].(string)
	if !ok || format == "" {
		return raw, nil
	}
	row["format"] = ffTrueColorRe.ReplaceAllStringFunc(format, func(code string) string {
		match := ffTrueColorRe.FindStringSubmatch(code)
		if match == nil {
			return code
		}
		color := match[2]
		switch color {
		case ffAccent:
			color = palette.Section
		case ffBright:
			color = palette.Title
		case ffTan:
			color = palette.Muted
		case ffDim:
			color = palette.Rule
		default:
			return code
		}
		return "\x1b[" + match[1] + "38;2;" + color + "m"
	})
	return json.Marshal(row)
}

func ffApplyPaletteToDisplay(display json.RawMessage, palette ffPalette) json.RawMessage {
	display = ffApplyAccentToDisplay(display, palette.Keys)
	var doc map[string]any
	if json.Unmarshal(display, &doc) != nil {
		doc = map[string]any{}
	}
	percent, _ := doc["percent"].(map[string]any)
	if percent == nil {
		percent = map[string]any{}
	}
	colors, _ := percent["color"].(map[string]any)
	if colors == nil {
		colors = map[string]any{}
	}
	colors["green"] = "38;2;" + palette.Percent.Green
	colors["yellow"] = "38;2;" + palette.Percent.Yellow
	colors["red"] = "38;2;" + palette.Percent.Red
	percent["color"] = colors
	doc["percent"] = percent
	b, err := json.Marshal(doc)
	if err != nil {
		return display
	}
	return b
}

func buildFastfetchEffective(m ffModel) ([]byte, error) {
	if ffPaletteMode(m.Palette) != ffPaletteWallpaper {
		return buildFastfetch(m)
	}
	palette, err := loadFastfetchPalette()
	if err != nil {
		return nil, err
	}
	accent := m.Accent
	if !validFFRGB(accent) {
		accent = ffAccent
	}
	var mods []json.RawMessage
	for _, row := range m.Rows {
		if !row.Enabled {
			continue
		}
		var raw json.RawMessage
		switch row.Kind {
		case "tagline":
			raw = ffCustomModule(ffTaglineStyled(palette.Section, palette.Muted, row.Text))
		case "header":
			raw = ffCustomModule(ffHeaderStyled(palette.Section, palette.Title, palette.Rule, row.Text))
		default:
			raw, err = ffBuildRow(row, accent)
			if err != nil {
				return nil, err
			}
			if raw != nil {
				raw, err = ffApplyPaletteToRow(raw, palette)
				if err != nil {
					return nil, err
				}
			}
		}
		if raw == nil {
			continue
		}
		mods = append(mods, raw)
	}
	out := ffOutConfig{
		Schema: m.Schema, Logo: ffBuildLogo(m.Logo), Modules: mods,
		Display: ffApplyPaletteToDisplay(m.Display, palette),
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func fastfetchEffectiveFresh(path string, mode string) bool {
	out, err := os.Stat(path)
	if err != nil || out.IsDir() {
		return false
	}
	inputs := []string{fastfetchConfigPath(), fastfetchPaletteStatePath()}
	if mode == ffPaletteWallpaper {
		inputs = append(inputs, fastfetchPalettePath())
	}
	if executable, err := os.Executable(); err == nil {
		inputs = append(inputs, executable)
	}
	for _, input := range inputs {
		info, err := os.Stat(input)
		if err != nil || out.ModTime().Before(info.ModTime()) {
			return false
		}
	}
	return true
}

func writeEffectiveFastfetch(path string) error {
	mode := readFastfetchPaletteMode()
	if fastfetchEffectiveFresh(path, mode) {
		return nil
	}
	model, err := loadFastfetch()
	if err != nil {
		return err
	}
	model.Palette = mode
	b, err := buildFastfetchEffective(model)
	if err != nil {
		return err
	}
	return atomicWrite(path, b, 0o600)
}

// ffApplyAccentToDisplay writes the accent into display.color.keys so the key
// labels match the accent the custom-line templates use, and so a saved accent
// reloads (get reads it back from here). Other display fields are preserved.
func ffApplyAccentToDisplay(display json.RawMessage, accent string) json.RawMessage {
	var d map[string]any
	if len(display) == 0 || json.Unmarshal(display, &d) != nil {
		d = map[string]any{}
	}
	color, _ := d["color"].(map[string]any)
	if color == nil {
		color = map[string]any{}
	}
	color["keys"] = "38;2;" + accent
	d["color"] = color
	b, err := json.Marshal(d)
	if err != nil {
		return display
	}
	return b
}

func ffBuildLogo(l ffLogo) ffOutLogo {
	out := ffOutLogo{Width: l.Width, Height: l.Height}
	switch l.Kind {
	case "image":
		out.Type = "kitty-direct"
		out.Source = l.Source
	case "ascii":
		out.Type = "file"
		out.Source = l.Source
	case "builtin":
		out.Type = "builtin"
		out.Width, out.Height = 0, 0
	default:
		out.Type = "none"
		out.Width, out.Height = 0, 0
	}
	if out.Type != "none" && out.Type != "builtin" {
		out.Padding = map[string]int{"top": l.PaddingTop, "right": l.PaddingRight, "left": l.Padding}
	}
	return out
}

func ffBuildRow(r ffRow, accent string) (json.RawMessage, error) {
	switch r.Kind {
	case "break":
		if len(r.Raw) > 0 {
			return r.Raw, nil
		}
		return json.RawMessage(`"break"`), nil
	case "tagline":
		return ffCustomModule(ffTaglineFormat(accent, r.Text)), nil
	case "header":
		return ffCustomModule(ffHeaderFormat(accent, r.Text)), nil
	case "module":
		// start from the original object so extras (detectionMethod, echo text)
		// survive, then apply the edited key.
		obj := map[string]any{}
		if len(r.Raw) > 0 {
			_ = json.Unmarshal(r.Raw, &obj)
		}
		if obj["type"] == nil {
			obj["type"] = r.Module
		}
		if r.Key != "" {
			obj["key"] = r.Key
		}
		return json.Marshal(obj)
	default: // title, colors, raw: verbatim
		if len(r.Raw) > 0 {
			return r.Raw, nil
		}
		return nil, nil
	}
}

func ffCustomModule(format string) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"type": "custom", "format": format})
	return b
}

func ffTaglineFormat(accent, text string) string {
	return ffTaglineStyled(accent, ffTan, text)
}

func ffTaglineStyled(section, muted, text string) string {
	return "\x1b[38;2;" + section + "m■\x1b[0m \x1b[38;2;" + muted + "m" + text + "\x1b[0m"
}

func ffHeaderFormat(accent, label string) string {
	return ffHeaderStyled(accent, ffBright, ffDim, label)
}

func ffHeaderStyled(section, title, ruleColor, label string) string {
	n := 32 - utf8.RuneCountInString(label)
	if n < 4 {
		n = 4
	}
	rule := strings.Repeat("─", n)
	return "\x1b[38;2;" + section + "m──\x1b[0m \x1b[1;38;2;" + title + "m" + label + "\x1b[0m \x1b[38;2;" + ruleColor + "m" + rule + "\x1b[0m"
}

// ---- preview + logo import --------------------------------------------------

func previewFastfetch(m ffModel) error {
	b, err := buildFastfetchEffective(m)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp("", "ryoku-ff-*.jsonc")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		return err
	}
	tmp.Close()
	// --logo none: the emblem is previewed in the Hub, this is just the readout.
	// --pipe false keeps the truecolor SGR the Hub parses into rich text.
	out, _ := exec.Command("fastfetch", "--config", tmp.Name(), "--logo", "none", "--pipe", "false").Output()
	os.Stdout.Write(out)
	return nil
}

// importFastfetchLogo copies a chosen logo into the fastfetch config dir so it is
// self-contained (survives moving the original), rasterizing an SVG to PNG since
// fastfetch has no SVG type. Prints the resulting path for the logo source.
func importFastfetchLogo(src string) (string, error) {
	src = ffExpandTilde(strings.TrimPrefix(src, "file://"))
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("logo not found: %s", src)
	}
	dir := fastfetchDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.Ext(src), ".svg") {
		out := filepath.Join(dir, "ryoku-logo.png")
		if err := ffRasterizeSVG(src, out); err != nil {
			return "", err
		}
		return out, nil
	}
	dst := filepath.Join(dir, "ryoku-logo"+strings.ToLower(filepath.Ext(src)))
	if err := ffCopyFile(src, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// ---- emblem: 1-bit dither + imported-emblem removal -------------------------

// The bake appends this to the whole source name, so trimming it always recovers
// the exact original path whatever the extension, and a source that already ends
// in it is known to be baked (so a re-bake is a no-op, not a double bake).
const ffDitherSuffix = ".1bit.png"

// ffBone and ffBayer4 reproduce bin/art/ryodither: the decor set's single ink,
// bone, carried on a transparent ground by an ordered Bayer 4x4 dither.
var (
	ffBone   = color.NRGBA{R: 0xe8, G: 0xd8, B: 0xc9, A: 0xff}
	ffBayer4 = [4][4]int{
		{0, 8, 2, 10},
		{12, 4, 14, 6},
		{3, 11, 1, 9},
		{15, 7, 13, 5},
	}
)

func ffIsDithered(source string) bool { return strings.HasSuffix(source, ffDitherSuffix) }

// ditherFastfetchLogo toggles the 1-bit bake, returning the source to store. The
// stored form is kept (a leading ~ stays portable); only the filesystem side is
// expanded. On is a no-op when the source is already the baked sibling; off points
// back at the original, which the bake never modified.
func ditherFastfetchLogo(mode, source string) (string, error) {
	switch mode {
	case "on":
		if ffIsDithered(source) {
			return source, nil
		}
		expanded := ffExpandTilde(source)
		if err := ffBakeDither(expanded, expanded+ffDitherSuffix); err != nil {
			return "", err
		}
		return source + ffDitherSuffix, nil
	case "off":
		return strings.TrimSuffix(source, ffDitherSuffix), nil
	default:
		return "", fmt.Errorf("dither-logo mode must be on or off, got %q", mode)
	}
}

// ffBakeDither writes src as a 1-bit two-colour PNG: bone where the tone clears
// the tiled Bayer threshold, transparent elsewhere. A two-entry palette makes it a
// genuine 1-bit indexed PNG. A fully transparent source pixel stays ground, so an
// emblem's cut-out survives; the original is only read, never written.
func ffBakeDither(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("emblem not found: %s", src)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("cannot read emblem image %s: %w", src, err)
	}
	b := img.Bounds()
	out := image.NewPaletted(b, color.Palette{color.NRGBA{}, ffBone})
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A == 0 {
				continue
			}
			// straight (un-premultiplied) ITU-R 601 luma, matching PIL's L.
			luma := (299*uint32(c.R) + 587*uint32(c.G) + 114*uint32(c.B)) / 1000
			thr := uint32((ffBayer4[(y-b.Min.Y)&3][(x-b.Min.X)&3]*2 + 1) * 255 / 32)
			if luma > thr {
				out.SetColorIndex(x, y, 1)
			}
		}
	}
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer w.Close()
	return png.Encode(w, out)
}

// removeFastfetchLogo deletes the imported emblem (ryoku-logo.*, including any
// bake) from the fastfetch dir and reports what it removed. It only touches
// ryoku-logo.* inside that dir, so the shipped fastfetch-emblem.png is never in
// range. If config.jsonc still points at a deleted file, it repoints the source at
// the shipped emblem so the live readout does not reference a file that is gone.
func removeFastfetchLogo() (string, error) {
	dir := fastfetchDir()
	matches, err := filepath.Glob(filepath.Join(dir, "ryoku-logo.*"))
	if err != nil {
		return "", err
	}
	var removed []string
	for _, m := range matches {
		// the glob is already rooted in the fastfetch dir; this refuses a match
		// that a symlink or odd name resolved to somewhere else.
		if filepath.Dir(m) != dir {
			continue
		}
		if err := os.Remove(m); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		removed = append(removed, m)
	}
	if len(removed) == 0 {
		return "", nil
	}
	ffRepointAfterRemoval(dir, removed)
	return strings.Join(removed, "\n"), nil
}

// applyFastfetchEmblem installs src as the readout's emblem the way the Hub's
// import does: copied to ryoku-logo.<ext> in the fastfetch dir, a user-owned
// name no package ships, and config.jsonc repointed at it. It returns the
// stored source. A rice used to copy its emblem over the SHIPPED
// fastfetch-emblem.png instead, which `ryoku materialize` re-lays on every
// update, so the rice's logo reset to the brand mark each time (docs/updates.md:
// never write into a shipped path).
func applyFastfetchEmblem(src string) (string, error) {
	p, err := importFastfetchLogo(src)
	if err != nil {
		return "", err
	}
	m, err := loadFastfetch()
	if err != nil {
		m = defaultFastfetchModel()
	}
	m.Logo.Kind, m.Logo.Source, m.Logo.Dither = "image", p, false
	b, err := buildFastfetch(m)
	if err != nil {
		return "", err
	}
	return p, atomicWrite(fastfetchConfigPath(), b, 0o644)
}

// fastfetchOnShippedEmblem reports whether the readout draws the package's own
// fastfetch-emblem.png (or nothing Ryoku wrote): the state a rice emblem lands in
// after materialize clobbered it, and the only state `rice emblem` repairs.
func fastfetchOnShippedEmblem() bool {
	m, err := loadFastfetch()
	if err != nil {
		return true
	}
	if m.Logo.Kind != "image" || m.Logo.Source == "" {
		return true
	}
	return filepath.Base(ffExpandTilde(m.Logo.Source)) == "fastfetch-emblem.png"
}

func ffRepointAfterRemoval(dir string, removed []string) {
	m, err := loadFastfetch()
	if err != nil {
		return
	}
	src := ffExpandTilde(m.Logo.Source)
	dangling := false
	for _, r := range removed {
		if src == r {
			dangling = true
			break
		}
	}
	if !dangling {
		return
	}
	if shipped := filepath.Join(dir, "fastfetch-emblem.png"); fileExists(shipped) {
		m.Logo.Kind, m.Logo.Source = "image", shipped
	} else {
		m.Logo.Kind, m.Logo.Source = "builtin", ""
	}
	m.Logo.Dither = false
	if b, err := buildFastfetch(m); err == nil {
		_ = atomicWrite(fastfetchConfigPath(), b, 0o644)
	}
}

func ffRasterizeSVG(src, out string) error {
	if _, err := exec.LookPath("rsvg-convert"); err == nil {
		return exec.Command("rsvg-convert", "-w", "512", "-h", "512", "-o", out, src).Run()
	}
	if _, err := exec.LookPath("magick"); err == nil {
		return exec.Command("magick", "-background", "none", "-density", "384", src, "-resize", "512x512", out).Run()
	}
	if _, err := exec.LookPath("convert"); err == nil {
		return exec.Command("convert", "-background", "none", "-density", "384", src, "-resize", "512x512", out).Run()
	}
	return fmt.Errorf("no SVG rasterizer found (install librsvg or imagemagick)")
}

func ffCopyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return atomicWrite(dst, b, 0o644)
}

func ffExpandTilde(p string) string {
	if p == "~" {
		return os.Getenv("HOME")
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(os.Getenv("HOME"), p[2:])
	}
	return p
}

func ffTitleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// defaultFastfetchModel: a minimal readout for a box with no config.jsonc yet, so
// the section is never empty. materialize seeds the real one on install.
func defaultFastfetchModel() ffModel {
	mk := func(t, k string) ffRow {
		raw, _ := json.Marshal(map[string]string{"type": t, "key": k})
		return ffRow{Kind: "module", Enabled: true, Module: t, Key: k, Label: k, Raw: raw}
	}
	return ffModel{
		Accent:  ffAccent,
		Palette: ffPaletteFixed,
		Logo:    ffLogo{Kind: "builtin", Width: 28, Height: 14, Padding: 3},
		Rows: []ffRow{
			mk("os", "OS"), mk("kernel", "KERNEL"), mk("cpu", "CPU"),
			mk("memory", "MEMORY"), mk("uptime", "UPTIME"),
		},
	}
}
