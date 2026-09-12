package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const paletteBridgeUnit = "ryoku-palette-bridge.service"

var paletteBridgeRun = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

type paletteBridgeIntegration struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Managed   bool   `json:"managed"`
}

type paletteBridgeStatus struct {
	Contract     int                        `json:"contract"`
	Available    bool                       `json:"available"`
	SourceReady  bool                       `json:"sourceReady"`
	Installed    bool                       `json:"installed"`
	Enabled      bool                       `json:"enabled"`
	Active       bool                       `json:"active"`
	Healthy      bool                       `json:"healthy"`
	Endpoint     string                     `json:"endpoint"`
	Version      string                     `json:"version"`
	Integrations []paletteBridgeIntegration `json:"integrations"`
}

func paletteBridgeHome() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func paletteBridgeConfigRoot() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	return filepath.Join(paletteBridgeHome(), ".config")
}

func paletteBridgeSource(source string, files ...string) (string, error) {
	if source == "" {
		source = "/usr/share/ryoku/palette-bridge"
	}
	abs, err := filepath.Abs(source)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("palette bridge source is not a directory: %s", abs)
	}
	for _, file := range files {
		info, err = os.Stat(filepath.Join(abs, file))
		if err != nil || info.IsDir() {
			return "", fmt.Errorf("palette bridge source is missing %s", file)
		}
	}
	return abs, nil
}

func paletteBridgeCommand(path string, args ...string) error {
	out, err := paletteBridgeRun(path, args...)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	if len(out) > 0 {
		fmt.Print(string(out))
	}
	return nil
}

func paletteBridgeUnitState(verb string) bool {
	_, err := paletteBridgeRun("systemctl", "--user", verb, "--quiet", paletteBridgeUnit)
	return err == nil
}

func paletteBridgeManagedFiles() map[string]bool {
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		state = filepath.Join(paletteBridgeHome(), ".local", "state")
	}
	b, err := os.ReadFile(filepath.Join(state, "ryoku", "palette-bridge", "owned-files.tsv"))
	if err != nil {
		return map[string]bool{}
	}
	managed := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		if fields := strings.SplitN(line, "\t", 2); len(fields) == 2 {
			managed[fields[0]+"\t"+filepath.Clean(fields[1])] = true
		}
	}
	return managed
}

func paletteBridgeStatusFor(source string) paletteBridgeStatus {
	config := paletteBridgeConfigRoot()
	managed := paletteBridgeManagedFiles()
	spotify := filepath.Join(config, "spicetify", "Extensions", "ryoku-wallpaper-colors.js")
	vesktop := filepath.Join(config, "vesktop", "themes", "midnight-ryoku.theme.css")
	zen := filepath.Join(config, "ryoku", "user_edits", "matugen", "templates", "zen.css")
	_, binErr := exec.LookPath("ryoku-palette-bridge")
	_, sourceErr := paletteBridgeSource(source, "install.sh", "install-integrations.sh")
	client := http.Client{Timeout: 350 * time.Millisecond}
	resp, healthErr := client.Get("http://127.0.0.1:47616/healthz")
	healthy := healthErr == nil && resp.StatusCode >= 200 && resp.StatusCode < 300
	if resp != nil {
		_ = resp.Body.Close()
	}
	return paletteBridgeStatus{
		Contract: 1, Available: binErr == nil || sourceErr == nil, SourceReady: sourceErr == nil,
		Installed: binErr == nil, Enabled: paletteBridgeUnitState("is-enabled"),
		Active: paletteBridgeUnitState("is-active"), Healthy: healthy,
		Endpoint: "http://127.0.0.1:47616", Version: "1",
		Integrations: []paletteBridgeIntegration{
			{ID: "spotify", Name: "Spotify", Installed: fileExists(spotify), Managed: managed["spotify\t"+spotify]},
			{ID: "vesktop", Name: "Vesktop", Installed: fileExists(vesktop), Managed: managed["vesktop\t"+vesktop]},
			{ID: "zen", Name: "Zen Browser", Installed: fileExists(zen), Managed: managed["zen\t"+zen]},
		},
	}
}

func paletteBridgeIntegrationID(id string) bool {
	return id == "spotify" || id == "vesktop" || id == "zen"
}

func runPaletteBridge(args []string) error {
	if len(args) == 0 || args[0] == "status" {
		source := ""
		if len(args) > 1 {
			source = args[1]
		}
		return printJSON(paletteBridgeStatusFor(source))
	}
	switch args[0] {
	case "install":
		if len(args) != 2 {
			return fmt.Errorf("palette-bridge install requires a source directory")
		}
		source, err := paletteBridgeSource(args[1], "install.sh")
		if err != nil {
			return err
		}
		return paletteBridgeCommand(filepath.Join(source, "install.sh"))
	case "service":
		if len(args) != 2 || (args[1] != "enable" && args[1] != "disable" && args[1] != "restart") {
			return fmt.Errorf("palette-bridge service requires enable, disable, or restart")
		}
		verb := args[1]
		if verb == "enable" || verb == "disable" {
			return paletteBridgeCommand("systemctl", "--user", verb, "--now", paletteBridgeUnit)
		}
		return paletteBridgeCommand("systemctl", "--user", "restart", paletteBridgeUnit)
	case "doctor":
		doctor, lookupErr := exec.LookPath("ryoku-palette-bridge-doctor")
		if lookupErr != nil {
			source := ""
			if len(args) > 1 {
				source = args[1]
			}
			resolved, err := paletteBridgeSource(source, "doctor.sh")
			if err != nil {
				return err
			}
			doctor = filepath.Join(resolved, "doctor.sh")
		}
		return paletteBridgeCommand(doctor)
	case "integration":
		if len(args) != 4 || (args[1] != "install" && args[1] != "remove") || !paletteBridgeIntegrationID(args[2]) {
			return fmt.Errorf("palette-bridge integration requires install|remove <spotify|vesktop|zen> <source>")
		}
		script := "install-integrations.sh"
		if args[1] == "remove" {
			script = "remove-integrations.sh"
		}
		source, err := paletteBridgeSource(args[3], script)
		if err != nil {
			return err
		}
		return paletteBridgeCommand(filepath.Join(source, script), "--"+args[2])
	default:
		return fmt.Errorf("unknown palette-bridge subcommand: %s", args[0])
	}
}
