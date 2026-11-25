package window

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrNoDesktop means env names no desktop Hyakki can read windows from.
var ErrNoDesktop = errors.New("window: no supported desktop in this session")

// detectTimeout bounds the one seam probe Detect makes.
const detectTimeout = 2 * time.Second

// Detect picks the provider for the session env describes: the Ryoku seam if
// `ryoku wm caps` answers, else niri, else Hyprland, else the degraded
// wlr-foreign-toplevel provider. run serves one-shot reads; Watch uses the
// real allowlisted event streams for env, and Hyprland is read over its own
// sockets.
func Detect(env []string, run Runner) (Provider, error) {
	return DetectWith(env, run, ExecStream(env), HyprlandSockets(env))
}

// DetectWith is Detect with the event sources and Hyprland's sockets
// supplied, so tests and the lab can feed recorded answers.
func DetectWith(env []string, run Runner, stream Stream, hypr HyprlandIPC) (Provider, error) {
	run = guard(run)
	ctx, cancel := context.WithTimeout(context.Background(), detectTimeout)
	defer cancel()
	if out, err := run(ctx, "ryoku", "wm", "caps"); err == nil {
		var caps seamCaps
		if json.Unmarshal(out, &caps) == nil && caps.Name != "" {
			return newSeam(caps, run, stream), nil
		}
	}
	switch {
	case lookup(env, EnvNiriSocket) != "":
		return newNiri(run, stream), nil
	case lookup(env, EnvHyprlandInstance) != "":
		return newHyprland(hypr.Requests, hypr.Events), nil
	case lookup(env, "WAYLAND_DISPLAY") != "":
		return NewWlr(), nil
	}
	return nil, ErrNoDesktop
}
