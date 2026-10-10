package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	defaultStarshipLayout  = "pill"
	defaultStarshipPalette = "fixed"
)

var starshipLayoutID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type starshipState struct {
	Layout  string `json:"layout"`
	Palette string `json:"palette"`
}

type starshipLayout struct {
	ID string `json:"id"`
}

func starshipConfigHome() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return base
}

func starshipLayoutsDir() string {
	if dir := os.Getenv("RYOKU_STARSHIP_LAYOUTS"); dir != "" {
		return dir
	}
	return filepath.Join(starshipConfigHome(), "starship", "layouts")
}

func starshipConfigPath() string { return filepath.Join(starshipConfigHome(), "starship.toml") }
func starshipPalettePath() string {
	return filepath.Join(starshipConfigHome(), "starship", "ryoku-colors.toml")
}
func starshipStatePath() string { return filepath.Join(starshipConfigHome(), "ryoku", "starship.json") }

func readStarshipState() starshipState {
	state := starshipState{Layout: defaultStarshipLayout, Palette: defaultStarshipPalette}
	b, err := os.ReadFile(starshipStatePath())
	if err == nil {
		_ = json.Unmarshal(b, &state)
	}
	if !starshipLayoutID.MatchString(state.Layout) {
		state.Layout = defaultStarshipLayout
	}
	if state.Palette != "fixed" && state.Palette != "wallpaper" {
		state.Palette = defaultStarshipPalette
	}
	if _, err := os.Stat(starshipLayoutPath(state.Layout)); err != nil {
		state.Layout = defaultStarshipLayout
	}
	return state
}

func starshipLayoutPath(id string) string {
	return filepath.Join(starshipLayoutsDir(), id+".toml")
}

func listStarshipLayouts() ([]starshipLayout, error) {
	entries, err := os.ReadDir(starshipLayoutsDir())
	if err != nil {
		return nil, fmt.Errorf("read Starship layouts: %w", err)
	}
	layouts := make([]starshipLayout, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".toml")
		if starshipLayoutID.MatchString(id) {
			layouts = append(layouts, starshipLayout{ID: id})
		}
	}
	sort.Slice(layouts, func(i, j int) bool { return layouts[i].ID < layouts[j].ID })
	return layouts, nil
}

func validateStarshipTOML(data string) error {
	var doc map[string]any
	_, err := toml.Decode(data, &doc)
	return err
}

func readStarshipLayout(id string) ([]byte, error) {
	if !starshipLayoutID.MatchString(id) {
		return nil, fmt.Errorf("invalid Starship layout %q", id)
	}
	b, err := os.ReadFile(starshipLayoutPath(id))
	if err != nil {
		return nil, fmt.Errorf("read Starship layout %q: %w", id, err)
	}
	if err := validateStarshipTOML(string(b)); err != nil {
		return nil, fmt.Errorf("parse Starship layout %q: %w", id, err)
	}
	return b, nil
}

func composeStarshipWallpaper(layout, palette []byte) ([]byte, error) {
	if err := validateStarshipTOML(string(palette)); err != nil {
		return nil, fmt.Errorf("parse Starship wallpaper palette: %w", err)
	}
	var out strings.Builder
	inPalette := false
	replacedPalette := false
	for _, line := range strings.SplitAfter(string(layout), "\n") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\n"))
		if strings.HasPrefix(trimmed, "[palettes.") && strings.HasSuffix(trimmed, "]") {
			inPalette = true
			continue
		}
		if inPalette {
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
				inPalette = false
			} else {
				continue
			}
		}
		if !replacedPalette && strings.HasPrefix(trimmed, "palette") && strings.Contains(trimmed, "=") {
			out.WriteString("palette = \"wallpaper\"\n")
			replacedPalette = true
			continue
		}
		out.WriteString(line)
	}
	if !replacedPalette {
		return nil, fmt.Errorf("Starship layout has no named palette")
	}
	body := strings.TrimRight(out.String(), "\n") + "\n\n" + strings.TrimSpace(string(palette)) + "\n"
	if err := validateStarshipTOML(body); err != nil {
		return nil, fmt.Errorf("compose Starship config: %w", err)
	}
	return []byte(body), nil
}

func renderStarship(state starshipState) ([]byte, error) {
	layout, err := readStarshipLayout(state.Layout)
	if err != nil {
		return nil, err
	}
	if state.Palette == "fixed" {
		return layout, nil
	}
	palette, err := os.ReadFile(starshipPalettePath())
	if err != nil {
		return nil, fmt.Errorf("read Starship wallpaper palette: %w", err)
	}
	return composeStarshipWallpaper(layout, palette)
}

func applyStarship(state starshipState, persist bool) error {
	b, err := renderStarship(state)
	if err != nil {
		return err
	}
	if err := atomicWrite(starshipConfigPath(), b, 0o644); err != nil {
		return fmt.Errorf("write Starship config: %w", err)
	}
	if !persist {
		return nil
	}
	stateBytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	stateBytes = append(stateBytes, '\n')
	return atomicWrite(starshipStatePath(), stateBytes, 0o644)
}

func runStarship(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("starship needs list|get|set|palette")
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("starship list takes no arguments")
		}
		layouts, err := listStarshipLayouts()
		if err != nil {
			return err
		}
		return printJSON(layouts)
	case "get":
		if len(args) != 1 {
			return fmt.Errorf("starship get takes no arguments")
		}
		return printJSON(readStarshipState())
	case "set":
		if len(args) != 2 {
			return fmt.Errorf("starship set needs a layout id")
		}
		state := readStarshipState()
		state.Layout = args[1]
		return applyStarship(state, true)
	case "palette":
		if len(args) != 2 || (args[1] != "fixed" && args[1] != "wallpaper") {
			return fmt.Errorf("starship palette needs fixed or wallpaper")
		}
		state := readStarshipState()
		state.Palette = args[1]
		return applyStarship(state, true)
	case "refresh":
		if len(args) != 1 {
			return fmt.Errorf("starship refresh takes no arguments")
		}
		state := readStarshipState()
		if state.Palette == defaultStarshipPalette {
			return nil
		}
		return applyStarship(state, false)
	default:
		return fmt.Errorf("unknown starship subcommand: %s", args[0])
	}
}
