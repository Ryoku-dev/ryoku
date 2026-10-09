package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The real shape from a box installed by an older CachyOS-variant installer and
// since adopted by limine-entry-tool 1.37+: the tree root carries the generated
// kernel children, and a second flat entry names a kernel image that is not on
// the ESP. That entry must go, and only that entry: the tree root has children,
// the EFI fallback resolves to a binary that is there, and a chainload into
// another volume is not ours to judge.
const limineAdoptedConf = `timeout: 3
default_entry: Ryoku Linux/linux
remember_last_entry: yes

/Ryoku Linux
  //linux
    protocol: efi
    path: boot():/EFI/Linux/ryoku_linux.efi#abc
     //Snapshots
     ///2026-09-06
     ////linux
     protocol: efi
     path: boot():/machine/limine_history/ryoku_linux.efi_sha256_x#abc

/Ryoku Linux (CachyOS)
    protocol: linux
    kernel_path: boot():/vmlinuz-linux-cachyos
    module_path: boot():/initramfs-linux-cachyos.img

/EFI fallback
    protocol: efi
    path: boot():/EFI/BOOT/BOOTX64.EFI

/Windows
    protocol: efi_chainload
    image_path: guid(1234):/EFI/Microsoft/Boot/bootmgfw.efi
`

func TestLimineDeadEntries(t *testing.T) {
	probe := func(p string) (bool, error) {
		switch p {
		case "/boot/EFI/Linux/ryoku_linux.efi",
			"/boot/machine/limine_history/ryoku_linux.efi_sha256_x",
			"/boot/EFI/BOOT/BOOTX64.EFI":
			return true, nil
		default:
			return false, nil
		}
	}
	dead, inaccessible := planLimineDeadEntries(limineTopEntries(limineAdoptedConf, "/boot"), probe)
	if len(inaccessible) != 0 {
		t.Fatalf("unexpected inaccessible entries: %v", inaccessible)
	}
	if len(dead) != 1 || dead[0] != "Ryoku Linux (CachyOS)" {
		t.Fatalf("dead = %v, want exactly the flat entry whose kernel image is missing", dead)
	}
}

func TestLimineDropEntriesKeepsTheRest(t *testing.T) {
	out, changed := limineDropEntries(limineAdoptedConf, "/boot", []string{"Ryoku Linux (CachyOS)"})
	if !changed {
		t.Fatal("dropping a dead entry must report a change")
	}
	if strings.Contains(out, "(CachyOS)") || strings.Contains(out, "vmlinuz-linux-cachyos") {
		t.Errorf("the dead entry or its body survived:\n%s", out)
	}
	for _, keep := range []string{
		"timeout: 3",
		"/Ryoku Linux\n",
		"//linux",
		"////linux",
		"/EFI fallback",
		"/Windows",
		"guid(1234):/EFI/Microsoft/Boot/bootmgfw.efi",
	} {
		if !strings.Contains(out, keep) {
			t.Errorf("%q was lost:\n%s", keep, out)
		}
	}
	if again, changed := limineDropEntries(out, "/boot", nil); changed || again != out {
		t.Error("a config with nothing dead must be a fixed point")
	}
}

// A default_entry naming the entry being removed would loop the countdown, so
// it has to move onto a kernel that still exists.
func TestLimineDropEntriesRepointsTheDefault(t *testing.T) {
	conf := strings.Replace(limineAdoptedConf,
		"default_entry: Ryoku Linux/linux",
		"default_entry: Ryoku Linux (CachyOS)", 1)
	out, changed := limineDropEntries(conf, "/boot", []string{"Ryoku Linux (CachyOS)"})
	if !changed {
		t.Fatal("expected the removal to change the config")
	}
	if strings.Contains(out, "default_entry: Ryoku Linux (CachyOS)") {
		t.Errorf("default_entry still names the removed entry:\n%s", out)
	}
	if !strings.Contains(out, "default_entry: Ryoku Linux/linux") {
		t.Errorf("default_entry was not repointed at a kernel that exists:\n%s", out)
	}
}

// TestPlanLimineDeadEntriesDistinguishesMissingFromInaccessible verifies that
// planLimineDeadEntries distinguishes genuine ENOENT from EACCES, EPERM, and EIO.
// Inaccessible entries must never be reported as dead.
func TestPlanLimineDeadEntriesDistinguishesMissingFromInaccessible(t *testing.T) {
	const multiEntryConf = `timeout: 3
/Valid Entry
    protocol: efi
    path: boot():/EFI/Linux/valid.efi

/Missing Entry
    protocol: linux
    kernel_path: boot():/vmlinuz-missing

/Permission Denied Entry
    protocol: linux
    kernel_path: boot():/vmlinuz-permission-denied

/IO Error Entry
    protocol: linux
    kernel_path: boot():/vmlinuz-io-error
`
	probe := func(p string) (bool, error) {
		switch p {
		case "/boot/EFI/Linux/valid.efi":
			return true, nil
		case "/boot/vmlinuz-missing":
			return false, nil // genuine ENOENT
		case "/boot/vmlinuz-permission-denied":
			return false, os.ErrPermission // EACCES / EPERM
		case "/boot/vmlinuz-io-error":
			return false, syscall.EIO // EIO
		default:
			return false, nil
		}
	}

	entries := limineTopEntries(multiEntryConf, "/boot")
	dead, inaccessible := planLimineDeadEntries(entries, probe)

	if len(dead) != 1 || dead[0] != "Missing Entry" {
		t.Fatalf("dead = %v, want exactly [\"Missing Entry\"]", dead)
	}
	if len(inaccessible) != 2 || inaccessible[0] != "Permission Denied Entry" || inaccessible[1] != "IO Error Entry" {
		t.Fatalf("inaccessible = %v, want [\"Permission Denied Entry\", \"IO Error Entry\"]", inaccessible)
	}
}

// TestReconcileLimineDeadEntriesInaccessiblePreventsRepair verifies that when any
// entry has an inaccessible image, reconcileLimineDeadEntries does NOT delete dead
// entries (even in apply mode), and instead warns with a safe inspection remedy.
func TestReconcileLimineDeadEntriesInaccessiblePreventsRepair(t *testing.T) {
	dir := t.TempDir()

	origESP, origPkg, origProbe := limineESPConf, liminePkgInstalled, limineDeadEntriesProbe
	t.Cleanup(func() {
		limineESPConf, liminePkgInstalled, limineDeadEntriesProbe = origESP, origPkg, origProbe
	})
	liminePkgInstalled = func(string) bool { return true }

	confPath := filepath.Join(dir, "limine.conf")
	initialConf := `timeout: 3
/Missing Entry
    protocol: linux
    kernel_path: boot():/vmlinuz-missing

/Protected Entry
    protocol: linux
    kernel_path: boot():/vmlinuz-protected
`
	if err := os.WriteFile(confPath, []byte(initialConf), 0o644); err != nil {
		t.Fatal(err)
	}
	limineESPConf = confPath

	limineDeadEntriesProbe = func(p string) (bool, error) {
		switch p {
		case "/boot/vmlinuz-missing":
			return false, nil // genuinely missing
		case "/boot/vmlinuz-protected":
			return false, os.ErrPermission // inaccessible
		default:
			return false, nil
		}
	}

	// Run in apply mode (checkOnly = false). Must NOT mutate because filesystem state is uncertain!
	res := reconcileLimineDeadEntries(false)
	if res.status != recWarn {
		t.Fatalf("got status %d, want recWarn (%d)", res.status, recWarn)
	}
	if !strings.Contains(res.detail, "cannot inspect boot images on /boot") || !strings.Contains(res.detail, "Protected Entry") {
		t.Errorf("unexpected detail: %s", res.detail)
	}
	if res.remedy != "sudo ryoku doctor --check" {
		t.Errorf("expected remedy 'sudo ryoku doctor --check', got %q", res.remedy)
	}

	// Verify the configuration file was left untouched
	content, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != initialConf {
		t.Errorf("config file was mutated despite uncertain state:\n%s", string(content))
	}
}
