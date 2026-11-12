package main

// gpuapply.go = the one-time, reversible "enable passthrough" + its undo.
// installs the stack, writes a small set of idempotent /etc files (kvmfr
// autoload + perms, the libvirt hook, a polkit rule), adds the user to
// libvirt/kvm, enables libvirtd, and -- only on an Intel host with IOMMU off --
// adds the kernel cmdline token. everything `enable` writes, `disable` removes.
// runs under pkexec (the lock.go pattern); a --dry-run prints the exact plan
// without touching anything, which is what the Hub shows before the user OKs.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	osuser "os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

type gpuPassthroughAvailability struct {
	Available bool   `json:"available"`
	Package   string `json:"package,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type gpuHostCapabilities struct {
	AUR struct {
		Supported bool   `json:"supported"`
		Reason    string `json:"reason"`
	} `json:"aur"`
}

func runGpuApply(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("gpu apply needs enable|disable [--dry-run]")
	}
	if args[0] == "availability" {
		return printJSON(passthroughAvailability())
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
	if action == "enable" {
		availability := passthroughAvailability()
		if !availability.Available {
			if availability.Reason != "" {
				return errors.New(availability.Reason)
			}
			return fmt.Errorf("%s is not available on this system", availability.Package)
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
// group membership, own the udev node (the lock.go greeter pattern). A stop
// signal (Settings' Cancel) goes on to pkexec, so an open password prompt
// closes with it instead of acting after the caller gave up.
func escalateSelf(args ...string) error {
	exe := selfExe()
	uid := strconv.Itoa(os.Getuid())
	full := append([]string{"env", "PKEXEC_UID=" + uid, exe}, args...)
	cmd := exec.Command("pkexec", full...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return runForwardingStop(cmd)
}

// runForwardingStop runs cmd and passes a stop signal (Settings' Cancel) on to
// it instead of dying with it. A child that escalates (pkexec, or a helper that
// execs pkexec) would otherwise outlive this process with its password prompt
// open and could still make the change after the user gave up.
func runForwardingStop(cmd *exec.Cmd) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer signal.Stop(stop)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case s := <-stop:
				_ = cmd.Process.Signal(s)
			case <-done:
				return
			}
		}
	}()
	return cmd.Wait()
}

// installPassthroughAUR installs Looking Glass + the kvmfr module as the
// invoking user. The host capability check keeps this path on Arch, where
// makepkg must run as the user and the helper inherits this terminal.
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
	return runForwardingStop(cmd)
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

func passthroughAvailability() gpuPassthroughAvailability {
	if unavailable, found := firstUnavailablePackage(corePassthroughPkgs, false); found {
		return unavailable
	}
	var caps gpuHostCapabilities
	out, err := exec.Command("ryoku-host", "capabilities").Output()
	if err != nil || json.Unmarshal(out, &caps) != nil {
		return gpuPassthroughAvailability{}
	}
	if unavailable, found := firstUnavailablePackage(extraPassthroughPkgs, caps.AUR.Supported); found {
		return unavailable
	}
	return gpuPassthroughAvailability{Available: true}
}

func firstUnavailablePackage(packages []string, aur bool) (gpuPassthroughAvailability, bool) {
	for _, pkg := range packages {
		cmd := exec.Command("ryoku-host", "pkg", "available", pkg)
		if aur {
			cmd = exec.Command("ryoku-host", "pkg", "available", "--aur", pkg)
		}
		if cmd.Run() == nil {
			continue
		}
		out, err := exec.Command("ryoku-host", "pkg", "why", pkg).Output()
		reason := ""
		if err == nil {
			reason, _, _ = strings.Cut(strings.TrimSpace(string(out)), "\n")
			reason = strings.TrimSpace(reason)
		}
		return gpuPassthroughAvailability{Package: pkg, Reason: reason}, true
	}
	return gpuPassthroughAvailability{}, false
}

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
		var before fileSnapshot
		var created []string
		if !dryRun {
			var paths []string
			for _, f := range files {
				if !f.needsKvmfr || kvmfrOK {
					paths = append(paths, filepath.Join(root, f.rel))
				}
			}
			var err error
			if before, err = snapshotFiles(paths...); err != nil {
				return err
			}
			created = missingDirs(paths)
		}
		for _, f := range files {
			if f.needsKvmfr && !kvmfrOK {
				say("skip /" + f.rel + " (kvmfr module not installed; re-run enable after adding Looking Glass)")
				continue
			}
			say("write /" + f.rel)
			if !dryRun {
				// nothing has reloaded these yet, so putting the bytes back is the whole undo
				if err := writeManaged(root, f); err != nil {
					if rerr := errors.Join(before.restore(), removeEmptyDirs(created)); rerr != nil {
						return errors.Join(err, fmt.Errorf("put back the files written before it: %w", rerr))
					}
					return err
				}
			}
		}
		say("add " + user + " to groups: libvirt, kvm")
		say("enable libvirtd.socket and the default network")
		if !dryRun {
			// every step is tried; the files stay, since these steps may already
			// have acted on them
			var failed []error
			step := func(what string, err error) {
				if err != nil {
					failed = append(failed, fmt.Errorf("%s: %w", what, err))
				}
			}
			step("add "+user+" to libvirt", joinGroup(user, "libvirt"))
			step("add "+user+" to kvm", joinGroup(user, "kvm"))
			step("enable libvirtd.socket", runErr("ryoku-host", "svc", "--system", "enable", "--now", "libvirtd.socket"))
			step("reload the udev rules", runErr("udevadm", "control", "--reload-rules"))
			step("autostart the default network", runErr("virsh", "net-autostart", "default"))
			if len(failed) > 0 {
				return fmt.Errorf("the files written above are in place, but these steps did not take:\n%w", errors.Join(failed...))
			}
		}
		if kvmfrOK {
			say("done. Log out and back in for group membership to take effect.")
		} else {
			say("core stack installed, but Looking Glass / kvmfr are missing, so passthrough stays off. Install them, then run enable again.")
		}
		return nil
	}

	// disable: remove exactly what enable wrote, trying every step past a failure
	var errs []error
	for _, f := range files {
		say("remove /" + f.rel)
		if !dryRun {
			errs = append(errs, removeIfPresent(filepath.Join(root, f.rel)))
		}
	}
	say("remove " + user + " from groups: libvirt, kvm")
	if !dryRun {
		errs = append(errs,
			leaveGroup(user, "libvirt"),
			leaveGroup(user, "kvm"),
			runErr("udevadm", "control", "--reload-rules"))
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	say("done. The discrete GPU returns to the host on the next boot.")
	return nil
}

// inGroup: is name in group (no such group counts as not)? ok is false when the
// lookup fails, which leaves gpasswd to decide
func inGroup(name, group string) (member, ok bool) {
	g, err := osuser.LookupGroup(group)
	if errors.As(err, new(osuser.UnknownGroupError)) {
		return false, true
	}
	if err != nil {
		return false, false
	}
	u, err := osuser.Lookup(name)
	if err != nil {
		return false, false
	}
	ids, err := u.GroupIds()
	if err != nil {
		return false, false
	}
	return slices.Contains(ids, g.Gid), true
}

// leaveGroup drops name from group; not in it is already done (gpasswd -d fails
// on a non-member)
func leaveGroup(name, group string) error {
	if member, ok := inGroup(name, group); ok && !member {
		return nil
	}
	return runErr("gpasswd", "-d", name, group)
}

// joinGroup adds name to group; already in it is already done
func joinGroup(name, group string) error {
	if member, ok := inGroup(name, group); ok && member {
		return nil
	}
	return runErr("gpasswd", "-a", name, group)
}

// missingDirs lists the directories above paths that do not exist yet, deepest
// first, so a rollback can remove what a write is about to create
func missingDirs(paths []string) []string {
	seen := map[string]bool{}
	var dirs []string
	for _, p := range paths {
		for d := filepath.Dir(p); !seen[d] && !fileExists(d); d = filepath.Dir(d) {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	slices.SortFunc(dirs, func(a, b string) int {
		return strings.Count(b, string(filepath.Separator)) - strings.Count(a, string(filepath.Separator))
	})
	return dirs
}

// removeEmptyDirs removes each dir that is still empty; one something else has
// since put a file in stays
func removeEmptyDirs(dirs []string) error {
	var errs []error
	for _, d := range dirs {
		if entries, err := os.ReadDir(d); err == nil && len(entries) == 0 {
			errs = append(errs, removeIfPresent(d))
		}
	}
	return errors.Join(errs...)
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

func snapshot(desc string) {
	run("ryoku-host", "snapshots", "create", desc)
}

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	_ = cmd.Run() // best-effort; one missing tool never aborts the rest
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
