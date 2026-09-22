package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Screen sharing lives or dies on one observable fact: whether the portal
// frontend currently publishes org.freedesktop.portal.ScreenCast. When it does
// not, an app asking "which screen should I share?" gets no dialog, no stream
// and no error the user can see -- so the Recording page needs a way to ask.
// The repair itself belongs to ryoku doctor, which owns the provider seam and
// the package knowledge; this only reports.

const shareScreenCastInterface = "org.freedesktop.portal.ScreenCast"

type shareStatus struct {
	Contract  int    `json:"contract"`
	Available bool   `json:"available"`
	Probeable bool   `json:"probeable"`
	Backend   string `json:"backend"`
	Detail    string `json:"detail"`
}

// shareScreenCastPublished introspects the live frontend. probeable is false
// when the session bus cannot be reached at all, which is not a broken desktop:
// a headless or dev shell has no screen to share and must not be shown a fault.
func shareScreenCastPublished() (exposed, probeable bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "busctl", "--user", "introspect",
		"org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop").Output()
	if err != nil {
		return false, false
	}
	for _, line := range strings.Split(string(out), "\n") {
		for _, field := range strings.Fields(line) {
			if field == shareScreenCastInterface {
				return true, true
			}
		}
	}
	return false, true
}

func runShare(args []string) error {
	if len(args) > 0 && args[0] != "status" {
		return fmt.Errorf("unknown share subcommand: %s", args[0])
	}
	exposed, probeable := shareScreenCastPublished()
	st := shareStatus{
		Contract:  1,
		Available: exposed,
		Probeable: probeable,
	}
	if caps, err := freshCaps(); err == nil {
		st.Backend = caps.PortalBackend
	}
	switch {
	case !probeable:
		st.Detail = "not running inside a desktop session, so there is nothing to share"
	case exposed:
		st.Detail = "apps can offer a screen picker"
	default:
		st.Detail = "no app can pick a screen to share; run ryoku doctor to repair the portal"
	}
	return printJSON(st)
}
