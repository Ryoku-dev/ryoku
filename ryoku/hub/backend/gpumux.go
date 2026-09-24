package main

// gpumux.go: `ryoku-hub gpu mux ...`, the Machine page's display-routing
// surface. The hardware MUX (a firmware knob on some laptops, called GPU Mode,
// MUX, Optimus or Ultimate depending on the vendor) decides which GPU the
// built-in panel is physically wired to, and with it whether the discrete GPU
// can ever sleep. `ryoku-gpu-mux` owns the knob; this layer only reshapes its
// JSON for the page and forwards an explicit switch. The write escalates
// inside the helper (passwordless polkit grant, scoped to the one program),
// because the page must work without a terminal.

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// muxState mirrors `ryoku-gpu-mux status --json`.
type muxState struct {
	Capable     bool    `json:"capable"`
	Mode        string  `json:"mode"`
	PanelOnDgpu bool    `json:"panel_on_dgpu"`
	DgpuWatts   float64 `json:"dgpu_watts"`
	RebootPend  bool    `json:"reboot_pending"`
}

func runGpuMux(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("gpu mux needs get|set")
	}
	switch args[0] {
	case "get":
		st, err := readMuxState()
		if err != nil {
			return err
		}
		return printJSON(st)
	case "set":
		if len(args) < 2 || (args[1] != "hybrid" && args[1] != "discrete") {
			return fmt.Errorf("gpu mux set needs hybrid|discrete")
		}
		st, err := readMuxState()
		if err != nil {
			return err
		}
		if !st.Capable {
			return fmt.Errorf("this machine has no display-routing switch; its screen is wired to one GPU by design")
		}
		if st.Mode == args[1] && !st.RebootPend {
			return printJSON(st)
		}
		out, err := exec.Command("ryoku-gpu-mux", "set", args[1]).CombinedOutput()
		if err != nil {
			return fmt.Errorf("ryoku-gpu-mux set %s: %v: %s", args[1], err, strings.TrimSpace(string(out)))
		}
		st.RebootPend = true
		st.Mode = args[1]
		return printJSON(st)
	default:
		return fmt.Errorf("gpu mux needs get|set")
	}
}

// readMuxState asks the owning tool. `status --json` answers the whole picture
// in one call (its watt probe has a hard timeout inside the tool, so it always
// returns, possibly with watts -1). If it cannot run at all, the cheap `get`
// still says whether a knob exists; no knob is a normal state, not a failure.
func readMuxState() (muxState, error) {
	if _, err := exec.LookPath("ryoku-gpu-mux"); err != nil {
		return muxState{Capable: false}, nil
	}
	if full, err := exec.Command("ryoku-gpu-mux", "status", "--json").Output(); err == nil {
		var s muxState
		if json.Unmarshal(full, &s) == nil {
			return s, nil
		}
	}
	out, err := exec.Command("ryoku-gpu-mux", "get").Output()
	mode := strings.TrimSpace(string(out))
	if (err != nil && mode == "") || mode == "unknown" {
		return muxState{Capable: false}, nil
	}
	return muxState{Capable: true, Mode: mode}, nil
}
