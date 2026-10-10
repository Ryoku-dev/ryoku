package main

// gpuapply.go = the one-time, reversible "enable passthrough" + its undo.
// installs the stack, writes a small set of idempotent /etc files (kvmfr
// autoload + perms, the libvirt hook, a polkit rule), adds the user to
// libvirt/kvm, enables libvirtd, and -- only on an Intel host with IOMMU off --
// adds the kernel cmdline token. everything `enable` writes, `disable` removes.
// runs under pkexec (the lock.go pattern); a --dry-run prints the exact plan
// without touching anything, which is what the Hub shows before the user OKs.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func runGpuApply(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("gpu apply needs enable|disable [--dry-run]")
	}
	action := args[0]
	if action != "enable" && action != "disable" {
		return fmt.Errorf("gpu apply: action must be enable or disable")
	}
	dryRun := false
	for _, a := range args[1:] {
		if a == "--dry-run" {
			dryRun = true
		}
	}
	// hook = an internal entrypoint libvirt calls. same subcommand tree, but it
	// has to route before the privilege dance.
	if dryRun {
		return applyPlan(action, invokingUser(), selfExe(), true)
	}
	if action == "enable" && os.Geteuid() != 0 {
		// Looking Glass + the kvmfr module are AUR-only and build as the user
		// (makepkg refuses root), so install them before escalating. the
		// privileged half then finds kvmfr present and writes its module config.
		installPassthroughAUR()
	}
	if os.Geteuid() != 0 {
		return escalateApply(args)
	}
	return applyPlan(action, invokingUser(), selfExe(), false)
}

// escalateApply re-runs the gpu-apply subcommand as root via pkexec.
func escalateApply(args []string) error {
	return escalateSelf(append([]string{"gpu", "apply"}, args...)...)
}

// escalateSelf re-runs this binary as root via pkexec, preserving the invoking
// user's id (PKEXEC_UID) so the privileged half acts on the right user -- set
// group membership, own the udev node (the lock.go greeter pattern).
func escalateSelf(args ...string) error {
	exe := selfExe()
	uid := strconv.Itoa(os.Getuid())
	full := append([]string{"env", "PKEXEC_UID=" + uid, exe}, args...)
	cmd := exec.Command("pkexec", full...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return cmd.Run()
}

// installPassthroughAUR installs Looking Glass + the kvmfr module as the
// invoking user. Best-effort: a build failure leaves a clear message and the
// rest of enable still runs, so caps honestly reports what is still missing.
// The package backend inherits this terminal so an Arch AUR helper can prompt.
func installPassthroughAUR() {
	missing := missingPkgs(extraPassthroughPkgs, pkgInstalled)
	if len(missing) == 0 {
		return
	}
	fmt.Println("Installing Looking Glass + the kvmfr module (this can take a few minutes)...")
	if err := aurInstall(missing); err != nil {
		if gpuHostUnavailable(err) {
			fmt.Println("Looking Glass and the kvmfr module are not available on this host; passthrough will stay off.")
			return
		}
		fmt.Printf("Could not install %s: %v\n", strings.Join(missing, " "), err)
		fmt.Println("Passthrough will stay off until they are available; try: ryoku-host pkg install --aur " + strings.Join(missing, " "))
	}
}

func aurInstall(pkgs []string) error {
	return ttyRun("ryoku-host", append([]string{"pkg", "install", "--aur"}, pkgs...)...)
}

func ttyRun(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// missingPkgs returns the packages not yet installed, order preserved.
func missingPkgs(pkgs []string, installed func(string) bool) []string {
	var m []string
	for _, p := range pkgs {
		if !installed(p) {
			m = append(m, p)
		}
	}
	return m
}

func selfExe() string {
	if e, err := os.Executable(); err == nil {
		return e
	}
	return "ryoku-hub"
}

// invokingUser = the human behind the action. PKEXEC_UID when escalated, else
// the current user's name.
func invokingUser() string {
	if u := os.Getenv("PKEXEC_UID"); u != "" {
		if name := userNameByID(u); name != "" {
			return name
		}
	}
	if u := os.Getenv("SUDO_USER"); u != "" {
		return u
	}
	return os.Getenv("USER")
}

func userNameByID(uid string) string {
	out, err := exec.Command("id", "-nu", uid).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

type managedFile struct {
	rel        string
	content    string
	mode       os.FileMode
	needsKvmfr bool // only after the kvmfr module is installed
}

func managedFiles(user, exe string) []managedFile {
	gpuBin := exe // ryoku-hub. the hook also needs ryoku-gpu, resolved below.
	ryokuGpu := "ryoku-gpu"
	if p, err := exec.LookPath("ryoku-gpu"); err == nil {
		ryokuGpu = p
	}
	hook := "#!/bin/bash\n" +
		"# Managed by Ryoku (ryoku-hub gpu apply). libvirt calls this for every domain;\n" +
		"# we forward the Ryoku VM's prepare/release to ryoku-hub, which binds the dGPU to\n" +
		"# vfio-pci on start and hands it back on stop.\n" +
		"export RYOKU_GPU_BIN=" + shellQuote(ryokuGpu) + "\n" +
		"guest=\"$1\"; op=\"$2\"\n" +
		"case \"$op\" in\n" +
		"  prepare) " + shellQuote(gpuBin) + " gpu hook prepare \"$guest\" ;;\n" +
		"  release|stopped) " + shellQuote(gpuBin) + " gpu hook release \"$guest\" ;;\n" +
		"esac\n" +
		"exit 0\n"
	udev := fmt.Sprintf("SUBSYSTEM==\"kvmfr\", OWNER=\"%s\", GROUP=\"kvm\", MODE=\"0660\"\n", user)
	return []managedFile{
		{"etc/modules-load.d/ryoku-kvmfr.conf", "kvmfr\n", 0o644, true},
		{"etc/modprobe.d/ryoku-kvmfr.conf", fmt.Sprintf("options kvmfr static_size_mb=%d\n", kvmfrStaticMB), 0o644, true},
		{"etc/udev/rules.d/99-ryoku-kvmfr.rules", udev, 0o644, true},
		{"etc/polkit-1/rules.d/50-ryoku-libvirt.rules", polkitRule, 0o644, false},
		{"etc/libvirt/hooks/qemu", hook, 0o755, false},
	}
}

// kvmfrModuleAvailable: is the kvmfr kernel module actually installed? a
// partial enable (no Looking Glass yet) must not write a modules-load entry
// that would fail at every boot.
func kvmfrModuleAvailable() bool {
	return exec.Command("modinfo", "kvmfr").Run() == nil
}

const polkitRule = `// Managed by Ryoku. Let the libvirt group manage libvirt without a password so the
// Ryoku VM launches straight from the app launcher.
polkit.addRule(function(action, subject) {
  if (action.id == "org.libvirt.unix.manage" && subject.isInGroup("libvirt")) {
    return polkit.Result.YES;
  }
});
`

// kvmfrStaticMB = Looking Glass shared-memory size in MiB, written to the kvmfr
// module's static_size_mb. 128 MiB covers SDR panels up to 2160p and most
// ultrawides; bumping it just locks down that RAM. Only the passthrough stack
// uses it (a passthrough VM is launched outside Ryoku, e.g. via libvirt).
const kvmfrStaticMB = 128

// The passthrough stack uses Arch package names as the stable interface.
// ryoku-host translates them for the host package manager.
var corePassthroughPkgs = []string{"qemu-desktop", "libvirt", "edk2-ovmf", "swtpm", "dnsmasq"}
var extraPassthroughPkgs = []string{"looking-glass", "looking-glass-module-dkms"}

func applyPlan(action, user, exe string, dryRun bool) error {
	files := managedFiles(user, exe)
	root := etcRoot()
	say := func(s string) { fmt.Println(planPrefix(dryRun) + s) }

	if action == "enable" {
		say("install packages: " + strings.Join(corePassthroughPkgs, " "))
		if !dryRun {
			snapshot("ryoku gpu passthrough enable")
			if err := packageInstall(corePassthroughPkgs); err != nil {
				if gpuHostUnavailable(err) {
					return fmt.Errorf("the passthrough stack is not available on this host")
				}
				return fmt.Errorf("installing the passthrough stack failed: %w (update the system with `ryoku update`, then retry)", err)
			}
		}
		// Install optional packages as the user before this privileged step, so
		// here we only report state.
		for _, p := range extraPassthroughPkgs {
			switch {
			case pkgInstalled(p):
				say(p + ": installed")
			case dryRun:
				say("install optional package: " + p)
			default:
				say(p + ": not installed -- optional package install skipped or failed; passthrough stays off until it is")
			}
		}
		kvmfrOK := dryRun || kvmfrModuleAvailable()
		for _, f := range files {
			if f.needsKvmfr && !kvmfrOK {
				say("skip /" + f.rel + " (kvmfr module not installed; re-run enable after adding Looking Glass)")
				continue
			}
			say("write /" + f.rel)
			if !dryRun {
				if err := writeManaged(root, f); err != nil {
					return err
				}
			}
		}
		say("add " + user + " to groups: libvirt, kvm")
		say("enable libvirtd.socket and the default network")
		if !dryRun {
			run("gpasswd", "-a", user, "libvirt")
			run("gpasswd", "-a", user, "kvm")
			run("ryoku-host", "svc", "--system", "enable", "--now", "libvirtd.socket")
			run("udevadm", "control", "--reload-rules")
			run("virsh", "net-autostart", "default")
		}
		if kvmfrOK {
			say("done. Log out and back in for group membership to take effect.")
		} else {
			say("core stack installed, but Looking Glass / kvmfr are missing, so passthrough stays off. Install them, then run enable again.")
		}
		return nil
	}

	// disable: remove exactly what enable wrote.
	for _, f := range files {
		say("remove /" + f.rel)
		if !dryRun {
			_ = os.Remove(filepath.Join(root, f.rel))
		}
	}
	say("remove " + user + " from groups: libvirt, kvm")
	if !dryRun {
		run("gpasswd", "-d", user, "libvirt")
		run("gpasswd", "-d", user, "kvm")
		run("udevadm", "control", "--reload-rules")
	}
	say("done. The discrete GPU returns to the host on the next boot.")
	return nil
}

func planPrefix(dryRun bool) string {
	if dryRun {
		return "[plan] "
	}
	return "[apply] "
}

func writeManaged(root string, f managedFile) error {
	p := filepath.Join(root, f.rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if b, err := os.ReadFile(p); err == nil && string(b) == f.content {
		return nil // idempotent: already correct
	}
	return os.WriteFile(p, []byte(f.content), f.mode)
}

func etcRoot() string {
	if r := os.Getenv("RYOKU_ETC_ROOT"); r != "" {
		return r
	}
	return "/"
}

func packageInstall(pkgs []string) error {
	cmd := exec.Command("ryoku-host", append([]string{"pkg", "install"}, pkgs...)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// pkgInstalled stays quiet because callers only need the package state.
func pkgInstalled(p string) bool {
	return exec.Command("ryoku-host", "pkg", "installed", p).Run() == nil
}

func gpuHostUnavailable(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	code := exitErr.ExitCode()
	return code == 4 || code == 5
}

func snapshotsUnavailable() bool {
	err := exec.Command("ryoku-host", "snapshots").Run()
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 5
}

func snapshot(desc string) {
	if snapshotsUnavailable() {
		return
	}
	if _, err := exec.LookPath("snapper"); err != nil {
		return
	}
	// -c number tags the snapshot for snapper's number cleanup so it counts
	// against NUMBER_LIMIT and prunes like update snapshots; without it these piled
	// up unbounded. Enforce the cap right after creating.
	run("snapper", "-c", "root", "create", "-c", "number", "--description", desc)
	run("snapper", "-c", "root", "cleanup", "number")
}

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	_ = cmd.Run() // best-effort; one missing tool never aborts the rest
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
