package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderPinPanelRisk(t *testing.T) {
	cases := []struct {
		name       string
		laptop     bool
		panel, pin string
		want       bool
	}{
		{"iGPU panel under a NVIDIA-first pin", true, "i915", "nvidia", true},
		{"amdgpu panel under a NVIDIA-first pin", true, "amdgpu", "nvidia", true},
		{"NVIDIA-driven panel under its own pin", true, "nvidia", "nvidia", false},
		{"iGPU-first pin", true, "i915", "i915", false},
		{"no pin", true, "i915", "", false},
		{"desktop", false, "i915", "nvidia", false},
		{"no connected panel", true, "", "nvidia", false},
	}
	for _, c := range cases {
		if got := renderPinPanelRisk(c.laptop, c.panel, c.pin); got != c.want {
			t.Fatalf("%s: risk = %v, want %v", c.name, got, c.want)
		}
	}
}

// fixtureDRM lays a sysfs-shaped tree: card0 is an iGPU with a connected eDP
// panel, card1 an NVIDIA card with only an external connector. The driver
// symlinks dangle on purpose; Readlink reports the target either way.
func fixtureDRM(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, c := range []struct{ card, driver string }{{"card0", "i915"}, {"card1", "nvidia"}} {
		if err := os.MkdirAll(filepath.Join(root, c.card, "device"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join("/fake", c.driver), filepath.Join(root, c.card, "device", "driver")); err != nil {
			t.Fatal(err)
		}
	}
	write := func(conn, status string) {
		if err := os.MkdirAll(filepath.Join(root, conn), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, conn, "status"), []byte(status+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("card0-eDP-1", "connected")
	write("card1-HDMI-A-1", "connected")
	return root
}

func TestPanelAndCardDriverFromSysfs(t *testing.T) {
	old := drmRoot
	drmRoot = fixtureDRM(t)
	t.Cleanup(func() { drmRoot = old })
	if got := panelDriver(); got != "i915" {
		t.Fatalf("panelDriver = %q, want the iGPU driving the connected eDP panel", got)
	}
	if got := cardDriver("card1"); got != "nvidia" {
		t.Fatalf("cardDriver(card1) = %q, want nvidia", got)
	}
}

func TestDrmCardOf(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "card1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "card1"), filepath.Join(root, "pci-0000:01:00.0")); err != nil {
		t.Fatal(err)
	}
	if got := drmCardOf("/dev/dri/card0"); got != "card0" {
		t.Fatalf("drmCardOf(card device) = %q, want card0", got)
	}
	if got := drmCardOf(filepath.Join(root, "pci-0000:01:00.0")); got != "card1" {
		t.Fatalf("drmCardOf(by-path) = %q, want card1", got)
	}
	if got := drmCardOf("/dev/dri/renderD128"); got != "" {
		t.Fatalf("drmCardOf(render node) = %q, want empty", got)
	}
}
