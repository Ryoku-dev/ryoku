package doctor

import (
	"strings"
	"testing"
)

// stubDgpu swaps the one seam the reconciler reads (the whole gathered state)
// for a fixture, restoring the real gatherer when the test ends. Every case is
// hermetic: no real /sys, DRM, PCI, or nvidia-smi is touched.
func stubDgpu(t *testing.T, s dgpuState) {
	t.Helper()
	orig := gatherDgpuState
	t.Cleanup(func() { gatherDgpuState = orig })
	gatherDgpuState = func() dgpuState { return s }
}

// The reconciler fires on either shape that leaves a discrete GPU pinned awake:
// the panel wired to it (MUX in Discrete), or the panel elsewhere but an open fd
// holding it up (MUX already Hybrid). Every other shape must report ok.
func TestDgpuPanelReconcile(t *testing.T) {
	cases := []struct {
		name      string
		state     dgpuState
		fires     bool
		remedyHas string
	}{
		{
			name:  "desktop stays silent",
			state: dgpuState{laptop: false},
		},
		{
			name:  "laptop with no discrete GPU stays silent",
			state: dgpuState{laptop: true, present: false},
		},
		{
			// Healthy hybrid: panel on the iGPU AND the dGPU actually sleeps.
			name:  "laptop with the panel on the iGPU stays silent",
			state: dgpuState{laptop: true, present: true, slot: "0000:01:00.0", panelOnDgpu: false, status: "suspended", suspended: 900},
		},
		{
			name:  "hybrid laptop whose dGPU has slept before stays silent",
			state: dgpuState{laptop: true, present: true, slot: "0000:01:00.0", panelOnDgpu: false, status: "active", suspended: 4200},
		},
		{
			name:  "laptop with the dGPU suspended stays silent",
			state: dgpuState{laptop: true, present: true, slot: "0000:01:00.0", panelOnDgpu: true, status: "suspended", suspended: 900},
		},
		{
			name:  "laptop dGPU awake but it has suspended before stays silent",
			state: dgpuState{laptop: true, present: true, slot: "0000:01:00.0", panelOnDgpu: true, status: "active", suspended: 120000},
		},
		{
			name:      "laptop with the panel on an awake, never-suspended dGPU fires",
			state:     dgpuState{laptop: true, present: true, slot: "0000:01:00.0", panelOnDgpu: true, status: "active", suspended: 0, watts: 10.4},
			fires:     true,
			remedyHas: "ryoku-gpu-mux set hybrid",
		},
		{
			// The regression this branch exists for: the MUX was switched, the
			// panel moved off the dGPU, and the card still never sleeps. Runtime
			// D3 is on, so the remedy must point at real usage, never at killing
			// the compositor -- fine-grained mode tolerates an idle open device.
			name:      "hybrid laptop whose dGPU still never sleeps fires",
			state:     dgpuState{laptop: true, present: true, slot: "0000:01:00.0", panelOnDgpu: false, status: "active", suspended: 0, watts: 9.9, rtd3: "Enabled (fine-grained)", holders: []string{"Hyprland", "qs"}},
			fires:     true,
			remedyHas: "CUDA",
		},
		{
			name:      "hybrid laptop never sleeping with no readable draw still fires",
			state:     dgpuState{laptop: true, present: true, slot: "0000:01:00.0", panelOnDgpu: false, status: "active", suspended: 0, watts: -1, rtd3: "Enabled (fine-grained)"},
			fires:     true,
			remedyHas: "CUDA",
		},
		{
			// Driver says the feature is off: closing applications cannot help,
			// so the remedy has to be the module parameter instead.
			name:      "runtime D3 switched off in the driver fires with the module fix",
			state:     dgpuState{laptop: true, present: true, slot: "0000:01:00.0", panelOnDgpu: false, status: "active", suspended: 0, watts: 9.4, rtd3: "Disabled"},
			fires:     true,
			remedyHas: "NVreg_DynamicPowerManagement=0x02",
		},
		{
			// An unreadable status must not be read as a fault we cannot see, so
			// this still lands on the usage remedy rather than the module one.
			name:      "hybrid laptop never sleeping with unreadable rtd3 uses the usage remedy",
			state:     dgpuState{laptop: true, present: true, slot: "0000:01:00.0", panelOnDgpu: false, status: "active", suspended: 0, watts: -1, rtd3: ""},
			fires:     true,
			remedyHas: "CUDA",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubDgpu(t, tc.state)
			got := reconcileDgpuPanel(true)
			if tc.fires {
				if got.status != recNote {
					t.Fatalf("expected a note, got %v (%s)", got.status, got.detail)
				}
				if !strings.Contains(got.remedy, tc.remedyHas) {
					t.Errorf("remedy must name %q, got %q", tc.remedyHas, got.remedy)
				}
				return
			}
			if got.status != recOK {
				t.Errorf("expected ok, got %v (%s)", got.status, got.detail)
			}
		})
	}
}

func TestDgpuRtd3OffFixMatchesHost(t *testing.T) {
	t.Run("Arch keeps mkinitcpio advice", func(t *testing.T) {
		t.Setenv("RYOKU_HOST_PKGMGR", "pacman")

		want := "the NVIDIA driver reports runtime D3 power management off for this card, so it can never power down. Enable fine-grained control with `NVreg_DynamicPowerManagement=0x02` in a /etc/modprobe.d/nvidia.conf `options nvidia` line, rebuild the initramfs (`sudo mkinitcpio -P`), and reboot."
		if got := dgpuRtd3OffFix(); got != want {
			t.Errorf("Arch repair advice changed:\ngot:  %q\nwant: %q", got, want)
		}
	})

	t.Run("Void uses XBPS and dracut advice", func(t *testing.T) {
		t.Setenv("RYOKU_HOST_PKGMGR", "xbps")

		got := dgpuRtd3OffFix()
		for _, want := range []string{"XBPS", "xbps-reconfigure", "dracut"} {
			if !strings.Contains(got, want) {
				t.Errorf("Void repair advice must mention %q: %q", want, got)
			}
		}
		if strings.Contains(got, "mkinitcpio") {
			t.Errorf("Void repair advice must not mention mkinitcpio: %q", got)
		}
	})
}

// A measured draw is quoted; an unreadable one is never turned into a made-up
// wattage.
func TestDgpuPanelWattageOnlyWhenMeasured(t *testing.T) {
	measured := planDgpuPanel(dgpuState{laptop: true, present: true, slot: "0000:01:00.0", panelOnDgpu: true, status: "active", suspended: 0, watts: 10.4})
	if !strings.Contains(measured.detail, "10.4 W") {
		t.Errorf("a measured draw must appear in the detail, got %q", measured.detail)
	}
	unread := planDgpuPanel(dgpuState{laptop: true, present: true, slot: "0000:01:00.0", panelOnDgpu: true, status: "active", suspended: 0, watts: -1})
	if unread.status != recNote {
		t.Fatalf("still a note when the draw is unreadable, got %v", unread.status)
	}
	if strings.Contains(unread.detail, " W") {
		t.Errorf("an unreadable draw must not fabricate a wattage, got %q", unread.detail)
	}
}

func TestDgpuCanSuspend(t *testing.T) {
	cases := []struct {
		status    string
		suspended int64
		want      bool
	}{
		{"active", 0, false},     // pinned awake, never slept -> not suspendable
		{"suspended", 0, true},   // asleep now
		{"active", 120000, true}, // awake but has slept before
		{"active", -1, true},     // suspend time unreadable -> treat as suspendable
	}
	for _, c := range cases {
		if got := dgpuCanSuspend(c.status, c.suspended); got != c.want {
			t.Errorf("dgpuCanSuspend(%q, %d) = %v, want %v", c.status, c.suspended, got, c.want)
		}
	}
}

func TestDgpuConnectorParsing(t *testing.T) {
	if !isCardName("card0") || !isCardName("card1") {
		t.Error("cardN must be recognized as a card node")
	}
	for _, bad := range []string{"card0-eDP-1", "renderD128", "controlD64", "card", "cardX"} {
		if isCardName(bad) {
			t.Errorf("%q must not be a card node", bad)
		}
	}
	if got := connectorCard("card1-eDP-1"); got != "card1" {
		t.Errorf("connectorCard(card1-eDP-1) = %q, want card1", got)
	}
	if got := connectorCard("card0-DP-2"); got != "card0" {
		t.Errorf("connectorCard(card0-DP-2) = %q, want card0", got)
	}
	if got := connectorCard("card0"); got != "" {
		t.Errorf("a bare card node is not a connector, got %q", got)
	}
	if got := connectorCard("renderD128"); got != "" {
		t.Errorf("a render node is not a connector, got %q", got)
	}
}

func TestDgpuParseUevent(t *testing.T) {
	body := "DRIVER=nvidia\nPCI_SLOT_NAME=0000:01:00.0\nMODALIAS=pci:v000010DE\n"
	driver, slot := parseUevent(body)
	if driver != "nvidia" || slot != "0000:01:00.0" {
		t.Errorf("parseUevent = (%q, %q), want (nvidia, 0000:01:00.0)", driver, slot)
	}
	if d, s := parseUevent(""); d != "" || s != "" {
		t.Errorf("empty uevent must yield empties, got (%q, %q)", d, s)
	}
}

func TestDgpuAllConnectorsOnDgpu(t *testing.T) {
	if allConnectorsOnDgpu("card1", nil) {
		t.Error("no connected connector must not read as panel-on-dGPU")
	}
	if !allConnectorsOnDgpu("card1", []string{"card1"}) {
		t.Error("the sole connected connector on the dGPU is panel-on-dGPU")
	}
	if allConnectorsOnDgpu("card1", []string{"card1", "card0"}) {
		t.Error("a connector on the iGPU means the panel is not solely on the dGPU")
	}
}

func TestDgpuDiscreteCard(t *testing.T) {
	cards := map[string]cardInfo{
		"card0": {driver: "amdgpu", slot: "0000:65:00.0"},
		"card1": {driver: "nvidia", slot: "0000:01:00.0"},
	}
	name, info, ok := discreteCard(cards)
	if !ok || name != "card1" || info.slot != "0000:01:00.0" {
		t.Errorf("discreteCard picked (%q, %+v, %v), want card1 / nvidia slot", name, info, ok)
	}
	if _, _, ok := discreteCard(map[string]cardInfo{"card0": {driver: "amdgpu"}}); ok {
		t.Error("a single amdgpu iGPU must not be read as a discrete GPU")
	}
}

// The driver's own runtime-D3 verdict is what separates "the feature is off" from
// "something is using the card", so the parse has to survive the real procfs
// layout (indented sub-keys, a blank line, other sections).
func TestParseRuntimeD3(t *testing.T) {
	body := `Runtime D3 status:          Enabled (fine-grained)
Tegra iGPU Rail-Gating:     Disabled
Video Memory:               Active

GPU Hardware Support:
 Video Memory Self Refresh: Supported
 Video Memory Off:          Supported
`
	if got := parseRuntimeD3(body); got != "Enabled (fine-grained)" {
		t.Errorf("parseRuntimeD3 = %q, want %q", got, "Enabled (fine-grained)")
	}
	if got := parseRuntimeD3("Runtime D3 status: Disabled\n"); got != "Disabled" {
		t.Errorf("parseRuntimeD3 = %q, want Disabled", got)
	}
	if got := parseRuntimeD3("Video Memory: Active\n"); got != "" {
		t.Errorf("a body without the key must yield \"\", got %q", got)
	}
	if got := parseRuntimeD3(""); got != "" {
		t.Errorf("an empty body must yield \"\", got %q", got)
	}
}

// An unreadable status must never be reported as a fault: we do not invent a
// defect we cannot observe.
func TestRtd3Off(t *testing.T) {
	for _, v := range []string{"Enabled (fine-grained)", "Enabled (coarse-grained)", ""} {
		if rtd3Off(v) {
			t.Errorf("rtd3Off(%q) = true, want false", v)
		}
	}
	for _, v := range []string{"Disabled", "Not supported"} {
		if !rtd3Off(v) {
			t.Errorf("rtd3Off(%q) = false, want true", v)
		}
	}
}
