package main

import (
	"testing"

	wm "ryoku-wm"
)

// Wayfire honours its HDR toggle only with the Vulkan renderer. A live session
// running the default GLES2 renderer must not claim outputHdr: the Hub would
// otherwise show a COLOUR row whose Apply silently leaves the display SDR.
// Offline probes keep the manifest intact so the capability list does not
// depend on which shell happened to launch the probe.
func TestOutputHdrCapFollowsRenderer(t *testing.T) {
	t.Run("offline probe keeps the manifest", func(t *testing.T) {
		// socketPath also scans XDG_RUNTIME_DIR for a live socket, so an
		// empty WAYFIRE_SOCKET alone is not offline on a compositor desktop.
		t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
		t.Setenv("WAYFIRE_SOCKET", "")
		t.Setenv("WLR_RENDERER", "")
		if !(wm.Caps{Supports: availableCaps()}).Has(wm.CapOutputHdr) {
			t.Error("an offline caps probe must keep HDR in the manifest")
		}
	})

	t.Run("vulkan renderer", func(t *testing.T) {
		stubAct(t, nil)
		t.Setenv("WAYFIRE_SOCKET", "/nonexistent/wayfire.socket")
		t.Setenv("WLR_RENDERER", "vulkan")
		if !(wm.Caps{Supports: availableCaps()}).Has(wm.CapOutputHdr) {
			t.Error("the Vulkan renderer must expose the HDR capability")
		}
	})

	t.Run("gles2 renderer", func(t *testing.T) {
		stubAct(t, nil)
		t.Setenv("WAYFIRE_SOCKET", "/nonexistent/wayfire.socket")
		t.Setenv("WLR_RENDERER", "")
		if (wm.Caps{Supports: availableCaps()}).Has(wm.CapOutputHdr) {
			t.Error("a live GLES2 session must not expose an HDR control it cannot honour")
		}
	})
}
