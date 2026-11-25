package app

import (
	"path/filepath"

	"ryoku-rashin/internal/hands/input"
)

// Display names the one Wayland display a Driver's input reaches: a runtime
// dir and the bare name of the display's socket in it. The keyboard tool's
// whole environment is input.Env for it, and the pointer dials its socket;
// nothing else of any session is passed on.
type Display struct {
	RuntimeDir string
	Name       string
}

// LiveDisplay is the person's own session as getenv (the daemon's
// environment) names it. A WAYLAND_DISPLAY that is a path names its own
// directory; an unset one is the protocol's default, wayland-0.
func LiveDisplay(getenv func(string) string) (Display, error) {
	d := Display{RuntimeDir: getenv("XDG_RUNTIME_DIR"), Name: getenv("WAYLAND_DISPLAY")}
	if d.Name == "" {
		d.Name = "wayland-0"
	}
	if filepath.IsAbs(d.Name) {
		d.RuntimeDir, d.Name = filepath.Dir(d.Name), filepath.Base(d.Name)
	}
	if _, err := d.env(); err != nil {
		return Display{}, err
	}
	return d, nil
}

// NestDisplay is Hyakki's separate nested desktop (a *nest.Workspace): its
// own runtime dir and display, never the live session's.
func NestDisplay(n interface {
	Dir() string
	Display() string
}) Display {
	return Display{RuntimeDir: n.Dir(), Name: n.Display()}
}

// Socket is the path of the display's Wayland socket.
func (d Display) Socket() string { return filepath.Join(d.RuntimeDir, d.Name) }

func (d Display) env() ([]string, error) { return input.Env(d.RuntimeDir, d.Name) }
