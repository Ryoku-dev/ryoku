package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: limine per-kernel boot images -------------------------------
//
// Each installed kernel boots from its own image on the ESP (a UKI under
// /boot/EFI/Linux, or an initramfs beside vmlinuz). An image left behind by
// an interrupted update, a hook that never fired, or a kernel removed under
// its entry still boots, then drops to an emergency shell after switch-root
// because its module tree is gone: the linux-cachyos "emergency mode" of #140.
// mkinitcpio only warns about a missing module, so nothing upstream catches
// it. This rebuilds a stale or missing image for every installed kernel and
// prunes the entry of a kernel the box no longer has.

type installedKernel struct {
	version string
	vmlinuz time.Time
}

// limineBootKernel is one generated "//<name>" kernel entry and the state of
// the image it boots.
type limineBootKernel struct {
	name        string
	version     string
	image       string
	imageExists bool
	imageOlder  bool
}

// installedKernelVersions: pkgbase -> module-tree version and vmlinuz mtime.
// A module tree without a vmlinuz is not a kernel and is skipped.
func installedKernelVersions() map[string]installedKernel {
	if xbpsHost() {
		return installedVoidKernelVersions()
	}
	out := map[string]installedKernel{}
	pkgbases, _ := filepath.Glob("/usr/lib/modules/*/pkgbase")
	for _, pb := range pkgbases {
		name := strings.TrimSpace(readFileSafe(pb))
		if name == "" || strings.HasPrefix(name, "(") {
			continue
		}
		dir := strings.TrimSuffix(pb, "/pkgbase")
		fi, err := os.Stat(dir + "/vmlinuz")
		if err != nil {
			continue
		}
		out[name] = installedKernel{version: filepath.Base(dir), vmlinuz: fi.ModTime()}
	}
	return out
}

func installedVoidKernelVersions() map[string]installedKernel {
	out := map[string]installedKernel{}
	dirs, _ := filepath.Glob("/usr/lib/modules/*")
	for _, dir := range dirs {
		version := filepath.Base(dir)
		name := voidKernelEntryName(version)
		if name == "" {
			continue
		}
		fi, err := os.Stat("/boot/vmlinuz-" + version)
		if err != nil {
			continue
		}
		if previous, ok := out[name]; ok && previous.version > version {
			continue
		}
		out[name] = installedKernel{version: version, vmlinuz: fi.ModTime()}
	}
	return out
}

func voidKernelEntryName(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return ""
	}
	major := leadingDigits(parts[0])
	minor := leadingDigits(parts[1])
	if major == "" || minor == "" {
		return ""
	}
	return "linux" + major + "." + minor
}

func leadingDigits(value string) string {
	at := 0
	for at < len(value) && value[at] >= '0' && value[at] <= '9' {
		at++
	}
	return value[:at]
}

// gatherLimineBootKernels reads the kernel entries directly under the OS
// directory, skipping the Snapshots submenu. esp is where boot():/ resolves.
func gatherLimineBootKernels(conf, esp string, installed map[string]installedKernel) []limineBootKernel {
	var out []limineBootKernel
	var cur *limineBootKernel
	flush := func() {
		if cur == nil {
			return
		}
		e := *cur
		if e.image != "" {
			if fi, err := os.Stat(e.image); err == nil {
				e.imageExists = true
				if k, ok := installed[e.name]; ok && !k.vmlinuz.IsZero() && fi.ModTime().Before(k.vmlinuz) {
					e.imageOlder = true
				}
			}
		}
		out = append(out, e)
		cur = nil
	}
	inDir := false
	for _, l := range strings.Split(conf, "\n") {
		t := strings.TrimLeft(l, " \t")
		if strings.HasPrefix(t, "/") {
			switch limineDepth(t) {
			case 1:
				flush()
				inDir = true
			case 2:
				flush()
				if name := limineNodeName(t); inDir && name != "" && !strings.EqualFold(name, "Snapshots") {
					cur = &limineBootKernel{name: name}
				}
			default:
				flush()
			}
			continue
		}
		if cur == nil {
			continue
		}
		if v := strings.TrimPrefix(t, "comment: Kernel version: "); v != t {
			cur.version = strings.TrimSpace(v)
		} else if p := limineEntryImagePath(t, esp); p != "" {
			cur.image = p
		}
	}
	flush()
	return out
}

// limineEntryImagePath: the image a body line names (a UKI "path:" or an
// initramfs "module_path:"), or "".
func limineEntryImagePath(bodyLine, esp string) string {
	for _, key := range []string{"path: ", "module_path: "} {
		if v := strings.TrimPrefix(bodyLine, key); v != bodyLine {
			return resolveLimineBootPath(strings.TrimSpace(v), esp)
		}
	}
	return ""
}

// resolveLimineBootPath turns "boot():/EFI/Linux/x.efi#<hash>" into a path
// under esp, or "" when the value is not boot()-relative.
func resolveLimineBootPath(raw, esp string) string {
	rest := strings.TrimPrefix(raw, "boot():")
	if rest == raw || !strings.HasPrefix(rest, "/") {
		return ""
	}
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		rest = rest[:i]
	}
	return strings.TrimRight(esp, "/") + rest
}

// planLimineKernelImages: stale is an installed kernel whose entry is missing,
// names another version, or whose image is gone or older than the kernel;
// stray is an entry for a kernel that is not installed.
func planLimineKernelImages(installed map[string]installedKernel, entries []limineBootKernel) (stale, stray []string) {
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.name] = true
		inst, ok := installed[e.name]
		if !ok {
			stray = append(stray, e.name)
			continue
		}
		if (e.version != "" && e.version != inst.version) || (e.image != "" && !e.imageExists) || e.imageOlder {
			stale = append(stale, e.name)
		}
	}
	for name := range installed {
		if !seen[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	sort.Strings(stray)
	return stale, stray
}

// pruneLimineStrayImages removes the images of kernels the box no longer has
// and regenerates the menu so their entries drop out.
func pruneLimineStrayImages(names []string) error {
	if xbpsHost() {
		const hook = "/etc/kernel.d/post-install/50-ryoku-limine"
		if !sys.Exists(hook) {
			return fmt.Errorf(i18n.T("Void Limine kernel hook is missing"))
		}
		return sys.Sudo(hook)
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var victims []string
	efis, _ := filepath.Glob("/boot/EFI/Linux/*.efi")
	for _, path := range efis {
		base := strings.TrimSuffix(filepath.Base(path), ".efi")
		if i := strings.IndexByte(base, '_'); i >= 0 && want[base[i+1:]] {
			victims = append(victims, path)
		}
	}
	imgs, _ := filepath.Glob("/boot/initramfs-*.img")
	for _, path := range imgs {
		name := strings.TrimPrefix(strings.TrimSuffix(filepath.Base(path), ".img"), "initramfs-")
		if want[name] {
			victims = append(victims, path)
			if k := "/boot/vmlinuz-" + name; sys.Exists(k) {
				victims = append(victims, k)
			}
		}
	}
	if len(victims) > 0 {
		if err := removeRootFiles(victims...); err != nil {
			return err
		}
	}
	if sys.Has("limine-update") {
		return sys.Sudo("limine-update")
	}
	return nil
}

// reconcileLimineKernelImages: a no-op while every image matches its kernel.
func reconcileLimineKernelImages(checkOnly bool) recResult {
	if !limineManagedBoot() {
		return okRes(i18n.T("Ryoku's Limine tooling does not manage this boot"))
	}
	if !doctorPackageInstalled("limine") {
		return okRes(i18n.T("not a limine-managed boot on this box"))
	}
	installed := installedKernelVersions()
	if len(installed) == 0 {
		return okRes(i18n.T("no installed kernels to check"))
	}
	entries := gatherLimineBootKernels(readFileSafe(limineESPConf), "/boot", installed)
	if len(entries) == 0 {
		return okRes(i18n.T("no tool-generated kernel entries yet; the boot tree reconciler owns that"))
	}
	stale, stray := planLimineKernelImages(installed, entries)
	if len(stale) == 0 && len(stray) == 0 {
		return okRes(i18n.T("every installed kernel has a current boot image"))
	}
	if checkOnly {
		var parts []string
		if len(stale) > 0 {
			parts = append(parts, fmt.Sprintf(i18n.T("boot image missing, older than its kernel, or built for a version no longer installed: %s (boots into an emergency shell)"), strings.Join(stale, ", ")))
		}
		if len(stray) > 0 {
			parts = append(parts, fmt.Sprintf(i18n.T("boot entry for a kernel that is not installed: %s (a dead menu item)"), strings.Join(stray, ", ")))
		}
		return wouldRes("%s", strings.Join(parts, "; ")).
			withFix(i18n.T("ryoku doctor rebuilds the stale image(s) and prunes the stray entry"))
	}
	var done, problems []string
	if len(stale) > 0 {
		if err := rebuildInitramfs(); err != nil {
			return failRes(i18n.T("kernel boot image stale for %s but the rebuild failed: %v"), strings.Join(stale, ", "), err).
				withFix(nvidiaInitramfsAdvice())
		}
		done = append(done, fmt.Sprintf(i18n.T("rebuilt the boot image for %s to match the installed kernel"), strings.Join(stale, ", ")))
	}
	if len(stray) > 0 {
		if err := pruneLimineStrayImages(stray); err != nil {
			problems = append(problems, fmt.Sprintf(i18n.T("could not prune the stray entry for %s: %v"), strings.Join(stray, ", "), err))
		} else {
			done = append(done, fmt.Sprintf(i18n.T("pruned the stray boot entry for %s (kernel not installed)"), strings.Join(stray, ", ")))
		}
	}
	if len(problems) > 0 {
		return warnRes("%s", strings.Join(append(done, problems...), "; ")).
			withFix(i18n.T("check the ESP and rerun sudo ryoku doctor"))
	}
	return fixedRes("%s", strings.Join(done, "; "))
}
