package doctor

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"ryoku-cli/internal/host"
)

// idempotency lock on the NVIDIA reconciler. canonical config we write must
// read back ok (else doctor rebuilds the initramfs every run); pre-fix or
// missing config = "needs fixing".
func TestNvidiaConfigOK(t *testing.T) {
	cases := []struct {
		name             string
		modprobe, mkinit string
		want             bool
	}{
		{"canonical config the reconciler writes", nvidiaModprobeConf, nvidiaMkinitcpioConf, true},
		{"old install: modeset only, no nouveau blacklist", "options nvidia_drm modeset=1 fbdev=1\n", nvidiaMkinitcpioConf, false},
		{"old install: modeset without fbdev heals", "options nvidia_drm modeset=1\nblacklist nouveau\n", nvidiaMkinitcpioConf, false},
		{"blacklisted but nvidia modules not in the initramfs", nvidiaModprobeConf, "", false},
		{"both drop-ins missing (readFileSafe error strings)", "(open /etc/modprobe.d/nvidia.conf: no such file or directory)", "(open /etc/mkinitcpio.conf.d/nvidia.conf: no such file or directory)", false},
	}
	for _, c := range cases {
		if got := nvidiaConfigOK(c.modprobe, c.mkinit); got != c.want {
			t.Errorf("%s: nvidiaConfigOK(...) = %v, want %v", c.name, got, c.want)
		}
	}
}

// idempotency lock on the guard-hook reconciler: the canonical hook must read
// back ok (else doctor rewrites it every run), a stale/absent one must not.
func TestNvidiaGuardHookOK(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want bool
	}{
		{"canonical hook the reconciler writes", nvidiaGuardHook, true},
		{"canonical hook with a trailing newline", nvidiaGuardHook + "\n", true},
		{"pre-fix hook: blind rebuild, no kernel trigger", "[Trigger]\nType=Package\nTarget=nvidia-utils\n[Action]\nExec=/bin/sh -c 'mkinitcpio -P'\n", false},
		{"missing hook (readFileSafe error string)", "(open /etc/pacman.d/hooks/ryoku-nvidia.hook: no such file or directory)", false},
	}
	for _, c := range cases {
		if got := nvidiaGuardHookOK(c.got); got != c.want {
			t.Errorf("%s: nvidiaGuardHookOK(...) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestReconcileKeplerNvidia(t *testing.T) {
	oldPCI := nvidiaPCIOutput
	oldInstalled := nvidia580Installed
	oldRemoveDriver := removeKepler580
	oldRestoreConfig := restoreKeplerNouveau
	oldRebuild := rebuildKeplerNouveau
	t.Cleanup(func() {
		nvidiaPCIOutput = oldPCI
		nvidia580Installed = oldInstalled
		removeKepler580 = oldRemoveDriver
		restoreKeplerNouveau = oldRestoreConfig
		rebuildKeplerNouveau = oldRebuild
	})

	nvidiaPCIOutput = func() string {
		return "01:00.0 VGA compatible controller: NVIDIA Corporation GK208B [GeForce GT 710]"
	}
	nvidia580Installed = func() bool { return true }

	removed, restored, rebuilt := false, false, false
	removeKepler580 = func() error { removed = true; return nil }
	restoreKeplerNouveau = func() error { restored = true; return nil }
	rebuildKeplerNouveau = func() error { rebuilt = true; return nil }

	got := reconcileKeplerNvidia(false)
	if got.status != recFixed {
		t.Fatalf("Kepler recovery status = %v, want fixed", got.status)
	}
	if !removed || !restored || !rebuilt {
		t.Fatalf("Kepler recovery removed=%t restored=%t rebuilt=%t, want all true", removed, restored, rebuilt)
	}

	removed, restored, rebuilt = false, false, false
	nvidiaPCIOutput = func() string {
		return "01:00.0 VGA compatible controller: NVIDIA Corporation GA107M [GeForce RTX 3050]"
	}
	got = reconcileKeplerNvidia(false)
	if got.status != recOK {
		t.Fatalf("non-Kepler recovery status = %v, want ok", got.status)
	}
	if removed || restored || rebuilt {
		t.Fatalf("non-Kepler recovery changed driver=%t config=%t initramfs=%t", removed, restored, rebuilt)
	}
}

func TestNvidiaDriverActiveForKeplerRecovery(t *testing.T) {
	if nvidiaDriverActiveFor(true, false, []string{"nvidia"}) {
		t.Fatal("a removed 580xx package must not reapply the NVIDIA blacklist on Kepler")
	}
	if !nvidiaDriverActiveFor(true, true, []string{"nvidia"}) {
		t.Fatal("an installed Kepler-compatible NVIDIA package must remain active")
	}
}

func TestKepler580RemovalArgs(t *testing.T) {
	got := kepler580RemovalArgs()
	want := []string{"pacman", "-R", "--noconfirm", "nvidia-580xx-dkms"}
	if len(got) != len(want) {
		t.Fatalf("removal args = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("removal args = %q, want %q", got, want)
		}
	}
}

func TestKeplerRemovalFailureUsesScopedRemedy(t *testing.T) {
	oldPCI := nvidiaPCIOutput
	oldInstalled := nvidia580Installed
	oldRemoveDriver := removeKepler580
	t.Cleanup(func() {
		nvidiaPCIOutput = oldPCI
		nvidia580Installed = oldInstalled
		removeKepler580 = oldRemoveDriver
	})
	nvidiaPCIOutput = func() string { return "NVIDIA GK208B" }
	nvidia580Installed = func() bool { return true }
	removeKepler580 = func() error { return errors.New("blocked") }

	got := reconcileKeplerNvidia(false)
	want := "sudo pacman -R --noconfirm nvidia-580xx-dkms"
	if got.remedy != want {
		t.Fatalf("remedy = %q, want %q", got.remedy, want)
	}
}

// The sleep-units reconciler repairs installer drift (nvidia.sh enables the
// three units), so a healthy box must read ok (idempotency: doctor never
// re-enables what is already enabled), and only the disabled units are
// named for the fix.
func TestPlanNvidiaSleepUnits(t *testing.T) {
	all := map[string]bool{"nvidia-suspend.service": true, "nvidia-hibernate.service": true, "nvidia-resume.service": true}
	enabled := func(states ...string) map[string]bool {
		m := map[string]bool{}
		for _, s := range states {
			m[s] = true
		}
		return m
	}
	cases := []struct {
		name        string
		nvidia      bool
		exists      map[string]bool
		enabled     map[string]bool
		wantMissing []string
		wantVerdict string
	}{
		{"mesa-only box stays silent", false, nil, nil, nil, "no proprietary NVIDIA driver in use"},
		{"nvidia-utils absent: nothing to enable", true, map[string]bool{}, nil, nil, "the NVIDIA sleep units are not installed on this machine"},
		{"healthy box: all three enabled", true, all, enabled("nvidia-suspend.service", "nvidia-hibernate.service", "nvidia-resume.service"), nil, "the NVIDIA sleep units are enabled"},
		{"converted box: all three disabled", true, all, nil, []string{"nvidia-suspend.service", "nvidia-hibernate.service", "nvidia-resume.service"}, ""},
		{"partial drift: only resume disabled", true, all, enabled("nvidia-suspend.service", "nvidia-hibernate.service"), []string{"nvidia-resume.service"}, ""},
	}
	for _, c := range cases {
		missing, verdict := planNvidiaSleepUnits(c.nvidia, c.exists, c.enabled)
		if verdict != c.wantVerdict {
			t.Errorf("%s: verdict = %q, want %q", c.name, verdict, c.wantVerdict)
		}
		if len(missing) != len(c.wantMissing) {
			t.Errorf("%s: missing = %v, want %v", c.name, missing, c.wantMissing)
			continue
		}
		for i := range missing {
			if missing[i] != c.wantMissing[i] {
				t.Errorf("%s: missing[%d] = %s, want %s", c.name, i, missing[i], c.wantMissing[i])
			}
		}
	}
}

func TestNvidiaVoidConfig(t *testing.T) {
	modprobe := nvidiaVoidModprobeConf(true)
	if !nvidiaConfigOKFor(host.XBPS, modprobe, nvidiaDracutConf) {
		t.Fatal("canonical Void modprobe and dracut policy must be healthy")
	}
	if nvidiaConfigOKFor(host.XBPS, modprobe, nvidiaMkinitcpioConf) {
		t.Fatal("mkinitcpio policy must not satisfy the Void dracut check")
	}
	if nvidiaConfigOKFor(host.XBPS, nvidiaVoidSafeModprobeConf, nvidiaVoidSafeDracutConf) {
		t.Fatal("nouveau-safe fallback must not claim the proprietary driver is configured")
	}
	legacy := nvidiaVoidModprobeConf(false)
	if strings.Contains(legacy, "NVreg_PreserveVideoMemoryAllocations") {
		t.Fatal("nvidia470 must not receive the preserve-VRAM option without its elogind hook")
	}
}

func TestNvidiaVoidHostBranches(t *testing.T) {
	oldManager := nvidiaPackageManager
	oldInit := nvidiaInitSystem
	t.Cleanup(func() {
		nvidiaPackageManager = oldManager
		nvidiaInitSystem = oldInit
	})
	nvidiaPackageManager = func() host.PackageManager { return host.XBPS }
	nvidiaInitSystem = func() host.InitSystem { return host.Runit }

	modprobe, initramfs := nvidiaConfigPaths()
	if modprobe != "/etc/modprobe.d/nvidia.conf" || initramfs != "/etc/dracut.conf.d/nvidia.conf" {
		t.Fatalf("Void NVIDIA paths = %q, %q", modprobe, initramfs)
	}
	path, content, mode, executable := nvidiaGuardSpec()
	if path != voidNvidiaGuardHookPath || content != voidNvidiaGuardHook || mode != "0755" || !executable {
		t.Fatalf("Void guard spec = %q mode %q executable=%t", path, mode, executable)
	}
	if !nvidiaGuardHookOKFor(host.XBPS, voidNvidiaGuardHook, true) {
		t.Fatal("canonical executable Void guard must pass")
	}
	if nvidiaGuardHookOKFor(host.XBPS, voidNvidiaGuardHook, false) {
		t.Fatal("non-executable Void kernel hook must be repaired")
	}
}

func TestVoidKernelSeriesFromXBPS(t *testing.T) {
	got := voidKernelSeriesFrom("ii linux6.12-6.12.58_1 kernel\nii linux-base-4.4_1 base\nii linux6.6-6.6.91_1 kernel\nii linux6.12-6.12.58_1 duplicate\n")
	want := []string{"linux6.12", "linux6.6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Void kernel series = %v, want %v", got, want)
	}
}

func TestPlanNvidiaElogindHook(t *testing.T) {
	cases := []struct {
		name        string
		active      bool
		ships       bool
		exists      bool
		wantMissing bool
		wantVerdict string
	}{
		{"nouveau", false, false, false, false, "no proprietary NVIDIA driver in use"},
		{"470 uses kernel suspend", true, false, false, false, "the installed NVIDIA branch uses the kernel suspend path and does not ship an elogind sleep hook"},
		{"current hook installed", true, true, true, false, "the NVIDIA elogind sleep hook is installed"},
		{"current hook missing", true, true, false, true, ""},
	}
	for _, tc := range cases {
		missing, verdict := planNvidiaElogindHook(tc.active, tc.ships, tc.exists)
		if missing != tc.wantMissing || verdict != tc.wantVerdict {
			t.Errorf("%s: missing=%t verdict=%q", tc.name, missing, verdict)
		}
	}
}

func TestNvidiaBacklightKernelLog(t *testing.T) {
	if !nvidiaBacklightDeadFrom("nvidia_wmi_ec_backlight: no NVIDIA native backlight") {
		t.Fatal("dmesg fallback must recognize the NVIDIA backlight failure")
	}
	if nvidiaBacklightDeadFrom("NVIDIA DRM initialized") {
		t.Fatal("unrelated NVIDIA kernel messages must not trigger the backlight warning")
	}
}
