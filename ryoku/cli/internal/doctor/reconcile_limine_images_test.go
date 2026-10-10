package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Which kernel the countdown autoboots is decided from the box, never from the
// entry names: the install records the kernel it was built around, and the
// kernel this session booted is the next-best statement. No brand is preferred,
// so a plain install that added a second kernel keeps its menu order.
func TestPickKernelPath(t *testing.T) {
	both := []string{"Ryoku Linux/linux", "Ryoku Linux/linux-cachyos"}
	for _, c := range []struct {
		name   string
		paths  []string
		prefer []string
		want   string
	}{
		{"recorded kernel wins", both, []string{"linux-cachyos", "linux"}, "Ryoku Linux/linux-cachyos"},
		{"recorded stock kernel is honoured even beside a cachyos entry", both, []string{"linux"}, "Ryoku Linux/linux"},
		{"running kernel when nothing is recorded", both, []string{"", "linux-cachyos"}, "Ryoku Linux/linux-cachyos"},
		{"box says nothing: menu order, no brand preference", both, nil, "Ryoku Linux/linux"},
		{"a preferred kernel with no entry falls through", both, []string{"linux-lts"}, "Ryoku Linux/linux"},
		{"no kernel entries at all", nil, []string{"linux"}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := pickKernelPath(c.paths, c.prefer); got != c.want {
				t.Errorf("pickKernelPath = %q, want %q", got, c.want)
			}
		})
	}

	conf := "/Ryoku Linux\n  //linux\n  //linux-cachyos\n     //Snapshots\n"
	if got := limineFirstKernelPath(conf); got != "Ryoku Linux/linux" {
		t.Errorf("first kernel path = %q, want the stock linux kernel", got)
	}
	if got := limineDefaultKernelPath("/Ryoku Linux\n    protocol: linux\n"); got != "" {
		t.Errorf("flat menu should have no kernel path, got %q", got)
	}
}

// The autoboot reconciler must only fix a default_entry that cannot boot (a bare
// numeric index that loops on the collapsed layout, or none at all) and never
// reset a deliberate one; remember_last_entry is seeded only when missing.
func TestLimineEnsureAutobootPreservesUserChoices(t *testing.T) {
	conf := "timeout: 3\ndefault_entry: Ryoku Linux/linux\nremember_last_entry: no\n\n/Ryoku Linux\n  //linux\n  //linux-cachyos\n"
	got, changed := limineEnsureAutoboot(conf)
	if changed {
		t.Errorf("a valid user default_entry + remember must be left untouched:\n%s", got)
	}
	if !strings.Contains(got, "default_entry: Ryoku Linux/linux") {
		t.Errorf("user default_entry was reset:\n%s", got)
	}
	if !strings.Contains(got, "remember_last_entry: no") {
		t.Errorf("user remember_last_entry was reset:\n%s", got)
	}

	num := "timeout: 3\ndefault_entry: 2\n\n/Ryoku Linux\n  //linux\n  //linux-cachyos\n"
	fixed, ch := limineEnsureAutoboot(num)
	if !ch {
		t.Fatal("a numeric default on the nested layout loops and must be repointed")
	}
	// Which kernel it lands on comes from the box (TestPickKernelPath covers
	// that); here it only has to stop being an index that loops.
	if !strings.Contains(fixed, "default_entry: Ryoku Linux/linux") {
		t.Errorf("numeric default not repointed at a kernel entry:\n%s", fixed)
	}
	if !strings.Contains(fixed, "remember_last_entry: yes") {
		t.Errorf("remember_last_entry not seeded:\n%s", fixed)
	}
	if again, ch2 := limineEnsureAutoboot(fixed); ch2 || again != fixed {
		t.Error("limineEnsureAutoboot is not idempotent after the fix")
	}
}

// The reset-on-update bug: the reconcilers rewrote /boot/limine.conf and clobbered
// the user's edits. A merge must keep every value the box already carries -- the
// timeout, colours, wallpaper, default_entry, remember flag -- and only Ryoku's
// boot identity and the snapshot-safety flag stay forced.
func TestMergeLimineConfPreservesUserEdits(t *testing.T) {
	base := "timeout: 20\n" +
		"default_entry: Ryoku/linux-cachyos\n" +
		"remember_last_entry: no\n" +
		"interface_branding_color: 00FF00\n" +
		"term_background: 123456\n" +
		"wallpaper: boot():/my-wall.png\n" +
		"\n" +
		"/+Ryoku\n  //linux\n  //linux-cachyos\n"
	got := mergeLimineConf(base, "")
	for _, want := range []string{
		"timeout: 20",
		"default_entry: Ryoku/linux-cachyos",
		"remember_last_entry: no",
		"interface_branding_color: 00FF00",
		"term_background: 123456",
		"wallpaper: boot():/my-wall.png",
		"interface_branding: Ryoku Bootloader",
		"hash_mismatch_panic: no",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("merge dropped or reset %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "interface_branding_color: C75D2B") {
		t.Errorf("merge reset the user's accent colour to the shipped default:\n%s", got)
	}
	for _, k := range []string{"timeout:", "default_entry:", "remember_last_entry:", "term_background:", "wallpaper:", "interface_branding_color:", "interface_branding:"} {
		if n := strings.Count(got, "\n"+k) + boolToInt(strings.HasPrefix(got, k)); n != 1 {
			t.Errorf("global %q appears %d times, want 1:\n%s", k, n, got)
		}
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestPlanLimineKernelImages(t *testing.T) {
	installed := map[string]installedKernel{
		"linux":         {version: "7.2-arch"},
		"linux-cachyos": {version: "7.2-cachy"},
	}
	entries := []limineBootKernel{
		{name: "linux", version: "7.2-arch", image: "/boot/EFI/Linux/ryoku_linux.efi", imageExists: true},
		{name: "linux-cachyos", version: "7.1-cachy", image: "/boot/EFI/Linux/ryoku_linux-cachyos.efi", imageExists: true}, // version drifted
		{name: "linux-lts", version: "6.6", image: "/boot/EFI/Linux/ryoku_linux-lts.efi", imageExists: true},               // not installed
	}
	stale, stray := planLimineKernelImages(installed, entries)
	if len(stale) != 1 || stale[0] != "linux-cachyos" {
		t.Errorf("stale = %v, want [linux-cachyos] (its entry names a version no longer installed)", stale)
	}
	if len(stray) != 1 || stray[0] != "linux-lts" {
		t.Errorf("stray = %v, want [linux-lts] (no such kernel installed)", stray)
	}

	// missing image and missing entry are both stale.
	entries2 := []limineBootKernel{
		{name: "linux", version: "7.2-arch", image: "/boot/EFI/Linux/ryoku_linux.efi", imageExists: false},
	}
	stale2, _ := planLimineKernelImages(installed, entries2)
	if len(stale2) != 2 || stale2[0] != "linux" || stale2[1] != "linux-cachyos" {
		t.Errorf("stale = %v, want [linux linux-cachyos] (missing image + kernel with no entry)", stale2)
	}
}

func TestGatherLimineBootKernels(t *testing.T) {
	esp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(esp, "EFI", "Linux"), 0o755); err != nil {
		t.Fatal(err)
	}
	mkuki := func(name string, mod time.Time) {
		p := filepath.Join(esp, "EFI", "Linux", name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	mkuki("ryoku_linux.efi", now)                           // newer than its kernel
	mkuki("ryoku_linux-cachyos.efi", now.Add(-2*time.Hour)) // older than its kernel

	installed := map[string]installedKernel{
		"linux":         {version: "7.2-arch", vmlinuz: now.Add(-time.Hour)},
		"linux-cachyos": {version: "7.2-cachy", vmlinuz: now},
	}
	conf := "/Ryoku Linux\n" +
		"  //linux\n" +
		"  comment: Kernel version: 7.2-arch\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux.efi#abc\n" +
		"  //linux-cachyos\n" +
		"  comment: Kernel version: 7.2-cachy\n" +
		"  protocol: efi\n" +
		"  path: boot():/EFI/Linux/ryoku_linux-cachyos.efi#def\n" +
		"     //Snapshots\n" +
		"     ///2026-01-01\n" +
		"     ////linux\n" +
		"     path: boot():/history/old.efi\n" +
		"/EFI fallback\n" +
		"  path: boot():/EFI/BOOT/BOOTX64.EFI\n"

	got := gatherLimineBootKernels(conf, esp, installed)
	if len(got) != 2 {
		t.Fatalf("gathered %d kernel entries, want 2 (linux, linux-cachyos), skipping Snapshots and the EFI fallback: %+v", len(got), got)
	}
	byName := map[string]limineBootKernel{}
	for _, e := range got {
		byName[e.name] = e
	}
	if e := byName["linux"]; e.version != "7.2-arch" || !e.imageExists || e.imageOlder {
		t.Errorf("linux entry = %+v, want version 7.2-arch, image present, not older", e)
	}
	if e := byName["linux-cachyos"]; e.version != "7.2-cachy" || !e.imageExists || !e.imageOlder {
		t.Errorf("cachyos entry = %+v, want version 7.2-cachy, image present, older than kernel", e)
	}

	stale, stray := planLimineKernelImages(installed, got)
	if len(stale) != 1 || stale[0] != "linux-cachyos" || len(stray) != 0 {
		t.Errorf("plan = stale %v stray %v, want stale [linux-cachyos] (stale image), no strays", stale, stray)
	}
}

func TestLimineToolingRequiresRyokuManagerBinary(t *testing.T) {
	stubPacmanHost(t, true)
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if limineManagedBoot() {
		t.Fatal("Limine package alone was treated as a Ryoku-managed boot")
	}
	tool := filepath.Join(bin, "limine-entry-tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !limineManagedBoot() {
		t.Fatal("Ryoku Limine manager binary was not recognized")
	}
}

func TestLimineToolingIsNotManagedOnNonPacmanHost(t *testing.T) {
	stubPacmanHost(t, false)
	if limineManagedBoot() {
		t.Fatal("non-pacman host was treated as Ryoku-managed Limine")
	}
}

func TestLimineReconcilersAreNeutralWithoutRyokuTooling(t *testing.T) {
	old := limineManagedBoot
	limineManagedBoot = func() bool { return false }
	t.Cleanup(func() { limineManagedBoot = old })

	checks := []struct {
		name string
		run  func() recResult
	}{
		{"layout", func() recResult { return reconcileLimineLayout(false) }},
		{"boot entry", func() recResult { return reconcileLimineBootEntry(false) }},
		{"UKI tree", func() recResult { return reconcileLimineUKITree(false) }},
		{"OS name", func() recResult { return reconcileLimineOSName(false) }},
		{"autoboot", func() recResult { return reconcileLimineAutoboot(false) }},
		{"kernel images", func() recResult { return reconcileLimineKernelImages(false) }},
		{"dead entries", func() recResult { return reconcileLimineDeadEntries(false) }},
		{"alongside entry", func() recResult { return reconcileAlongsideBootEntry(false) }},
		{"boot space", func() recResult { return reconcileBootSpace(false) }},
		{"boot read-write", func() recResult { return reconcileBootRW(false) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if got := check.run(); got.status != recOK {
				t.Fatalf("result = %#v, want neutral ok", got)
			}
		})
	}
}
