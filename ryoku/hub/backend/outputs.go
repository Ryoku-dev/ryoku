package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

// outputs is the display page's backend. It enumerates the connected screens
// through the seam, applies a layout the page built, and keeps named layout
// profiles, so the Hub never spells a compositor or its display tool. Enumeration
// is the provider's full state read; apply and profiles hand the neutral layout
// to the provider, which persists it and applies live.
//
// Profiles are Hub-owned: a profile is just a stored OutputLayout, replayed
// through ApplyOutputs, so it works on every compositor and survives a switch
// (the provider reports whatever the new compositor cannot honour).
//
//	ryoku-hub outputs                list the connected outputs (JSON)
//	ryoku-hub outputs apply <json>   apply an output layout, print the report
//	ryoku-hub outputs profiles       list saved profiles and whether each matches
//	ryoku-hub outputs save <n> <json> save a profile and apply it
//	ryoku-hub outputs load <n>       apply a saved profile
//	ryoku-hub outputs rm <n>         delete a saved profile

// outputRow is one output as the display editor reads it: the seam's Output plus
// the physical width, height and refresh parsed from the current mode, since the
// editor sizes tiles in real pixels while Output.Width is the logical rectangle.
// Mirror and the colour fields are empty unless the provider supports those
// behaviours, so the page pre-fills its controls without clobbering them.
type outputRow struct {
	Name          string   `json:"name"`
	Focused       bool     `json:"focused"`
	Disabled      bool     `json:"disabled"`
	Make          string   `json:"make,omitempty"`
	Model         string   `json:"model,omitempty"`
	Width         int      `json:"width"`
	Height        int      `json:"height"`
	Refresh       int      `json:"refresh"`
	PhysicalWidth int      `json:"physicalWidth"`
	Scale         float64  `json:"scale"`
	X             int      `json:"x"`
	Y             int      `json:"y"`
	Transform     int      `json:"transform"`
	VRR           bool     `json:"vrr"`
	Mode          string   `json:"mode"`
	Modes         []string `json:"modes"`
	Mirror        string   `json:"mirror"`
	ColorMode     string   `json:"colorMode"`
	SdrBrightness float64  `json:"sdrBrightness"`
}

func runOutputs(args []string) error {
	switch {
	case len(args) == 0 || args[0] == "list":
		return listOutputs()
	case args[0] == "apply":
		if len(args) < 2 {
			return fmt.Errorf("outputs apply needs a layout JSON")
		}
		return applyOutputs(args[1])
	case args[0] == "profiles":
		return listProfiles()
	case args[0] == "save":
		if len(args) < 3 {
			return fmt.Errorf("outputs save needs a name and a layout JSON")
		}
		return saveProfile(args[1], args[2])
	case args[0] == "load":
		if len(args) < 2 {
			return fmt.Errorf("outputs load needs a name")
		}
		return loadProfile(args[1])
	case args[0] == "rm":
		if len(args) < 2 {
			return fmt.Errorf("outputs rm needs a name")
		}
		return rmProfile(args[1])
	}
	return fmt.Errorf("outputs: unknown subcommand %q", args[0])
}

func listOutputs() error {
	snap, err := desktopClient().State()
	if err != nil {
		return err
	}
	rows := make([]outputRow, 0, len(snap.Outputs))
	for _, o := range snap.Outputs {
		w, h, refresh := parseModeDims(o.Mode)
		rows = append(rows, outputRow{
			Name:          o.Name,
			Focused:       o.Focused,
			Disabled:      o.Disabled,
			Make:          o.Make,
			Model:         o.Model,
			Width:         w,
			Height:        h,
			Refresh:       refresh,
			PhysicalWidth: o.PhysicalWidth,
			Scale:         o.Scale,
			X:             o.X,
			Y:             o.Y,
			Transform:     o.Transform,
			VRR:           o.VRR,
			Mode:          o.Mode,
			Modes:         o.Modes,
			Mirror:        o.Mirror,
			ColorMode:     o.ColorMode,
			SdrBrightness: o.SdrBrightness,
		})
	}
	return printJSON(rows)
}

func applyOutputs(raw string) error {
	layout, err := parseLayout(raw)
	if err != nil {
		return err
	}
	return applyLayout(layout)
}

// applyLayout hands the layout to the provider by path, like the seam wants, and
// prints its report so the page can surface anything the compositor could not
// honour. Shared by a direct apply and a profile load.
func applyLayout(layout []wm.OutputLayout) error {
	body, err := json.Marshal(layout)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "ryoku-outputs-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	rep, err := desktopClient().ApplyOutputs(f.Name())
	if err != nil {
		return err
	}
	publishGreeterPrimary(layout)
	return printJSON(rep)
}

// publishGreeterPrimary records the layout's main output where the login
// greeter can read it. The greeter runs as the sddm user before any session
// exists, so it cannot see the per-user compositor config that otherwise holds
// the choice; without this hand-off it falls back to an internal-panel
// heuristic and a desktop whose main is an external gets its login form on the
// wrong screen. Best-effort: the file is created by tmpfiles on a packaged box,
// and a checkout without it simply keeps today's behaviour.
func publishGreeterPrimary(layout []wm.OutputLayout) {
	// The page's "Set as main" re-bases the layout so the chosen output sits at
	// the global origin; a layout that predates that convention still has a
	// top-left output. Take the origin first, else the top-left-most enabled
	// output -- the same order deriveMain uses.
	var main *wm.OutputLayout
	for i := range layout {
		o := &layout[i]
		if !o.Enabled {
			continue
		}
		if o.X == 0 && o.Y == 0 {
			main = o
			break
		}
		if main == nil || o.X < main.X || (o.X == main.X && o.Y < main.Y) {
			main = o
		}
	}
	if main == nil {
		return
	}
	_ = os.WriteFile(greeterPrimaryPath(), []byte(main.Name+"\n"), 0o666)
}

func greeterPrimaryPath() string {
	if p := os.Getenv("RYOKU_GREETER_PRIMARY_FILE"); p != "" {
		return p
	}
	return "/var/lib/ryoku/greeter-primary"
}
func parseLayout(raw string) ([]wm.OutputLayout, error) {
	var layout []wm.OutputLayout
	if err := json.Unmarshal([]byte(raw), &layout); err != nil {
		return nil, fmt.Errorf("outputs: %w", err)
	}
	return layout, nil
}

// --- profiles -------------------------------------------------------------

// outputProfile is a saved profile as the page lists it: the name, and whether
// every output it configures is connected now, so the page can mark the ones
// ready to apply.
type outputProfile struct {
	Name    string `json:"name"`
	Matches bool   `json:"matches"`
}

func profilesDir() string { return filepath.Join(ryokuConfigDir(), "output-profiles") }

// profileName rejects a name that would escape the profiles dir, since it comes
// from a text field.
func profileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("profile name is empty")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid profile name %q", name)
	}
	return name, nil
}

func saveProfile(name, raw string) error {
	n, err := profileName(name)
	if err != nil {
		return err
	}
	layout, err := parseLayout(raw)
	if err != nil {
		return err
	}
	body, err := json.Marshal(layout)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(profilesDir(), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(profilesDir(), n+".json"), body, 0o644); err != nil {
		return err
	}
	return applyLayout(layout)
}

func loadProfile(name string) error {
	layout, err := readProfile(name)
	if err != nil {
		return err
	}
	return applyLayout(layout)
}

func rmProfile(name string) error {
	n, err := profileName(name)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(profilesDir(), n+".json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return printJSON(map[string]bool{"removed": true})
}

func readProfile(name string) ([]wm.OutputLayout, error) {
	n, err := profileName(name)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(profilesDir(), n+".json"))
	if err != nil {
		return nil, err
	}
	var layout []wm.OutputLayout
	if err := json.Unmarshal(raw, &layout); err != nil {
		return nil, err
	}
	return layout, nil
}

func listProfiles() error {
	connected := connectedOutputs()
	ents, err := os.ReadDir(profilesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return printJSON([]outputProfile{})
		}
		return err
	}
	out := make([]outputProfile, 0, len(ents))
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		layout, err := readProfile(name)
		if err != nil {
			continue
		}
		out = append(out, outputProfile{Name: name, Matches: profileMatches(layout, connected)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return printJSON(out)
}

// connectedOutputs is the set of output names live now, for the profile match
// mark. nil off a session, which reads as "nothing matches".
func connectedOutputs() map[string]bool {
	snap, err := desktopClient().State()
	if err != nil {
		return nil
	}
	set := make(map[string]bool, len(snap.Outputs))
	for _, o := range snap.Outputs {
		set[o.Name] = true
	}
	return set
}

// profileMatches is true when every output the profile names is connected now,
// so applying it would govern the whole live set rather than a partial one.
func profileMatches(layout []wm.OutputLayout, connected map[string]bool) bool {
	if len(layout) == 0 || len(connected) == 0 {
		return false
	}
	for _, o := range layout {
		if !connected[o.Name] {
			return false
		}
	}
	return true
}

// parseModeDims pulls the physical width, height and rounded refresh from a
// "WxH@Hz" mode string. Zeroes for an empty mode, which is how a disabled output
// reads.
func parseModeDims(s string) (w, h, refresh int) {
	if s == "" {
		return 0, 0, 0
	}
	dims := s
	if at := strings.IndexByte(s, '@'); at >= 0 {
		dims = s[:at]
		if f, err := strconv.ParseFloat(s[at+1:], 64); err == nil {
			refresh = int(f + 0.5)
		}
	}
	if x := strings.IndexByte(dims, 'x'); x >= 0 {
		w, _ = strconv.Atoi(dims[:x])
		h, _ = strconv.Atoi(dims[x+1:])
	}
	return w, h, refresh
}
