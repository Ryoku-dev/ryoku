package doctor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: limine boot menu layout --------------------------------------

// limine-entry-tool (the stack behind limine-mkinitcpio-hook and
// limine-snapper-sync) manages exactly one config: /boot/limine.conf, the ESP
// root. Limine itself scans /boot/limine/limine.conf BEFORE /boot/limine.conf
// on the same partition, so a config in that older location shadows every
// generated entry -- UKIs, the Snapshots submenu, everything -- and the
// firmware keeps showing the frozen install-time menu. earlier installers
// wrote exactly that shadow file, and also hand-copied the Limine binary to
// EFI/limine/limine.efi, a path the tool never refreshes (it deploys
// EFI/limine/limine_x64.efi), so the booted bootloader silently ages while
// the package updates. this reconciler migrates both: merge the shadow's
// branding into the tool-managed config and remove it, then re-deploy the
// binary onto the tool's path and retire the stale NVRAM entry.
//
// limineBranding mirrors system/boot/limine/limine.conf (the globals): keep
// the two in sync so a doctored box matches a fresh install.
const limineBranding = `timeout: 3
default_entry: 1
remember_last_entry: yes
interface_branding: Ryoku Bootloader
interface_branding_color: C75D2B
interface_help_color: C75D2B
hash_mismatch_panic: no

term_background: 060607
backdrop: 060607
term_palette: 060607;EAE2D5;C75D2B;3A3630;88A57D;C75D2B;8C857A;EAE2D5
term_palette_bright: 141210;EAE2D5;C75D2B;3A3630;88A57D;C75D2B;8C857A;EAE2D5
term_foreground: EAE2D5
term_foreground_bright: EAE2D5
term_background_bright: 141210
`

var (
	limineESPConf      = "/boot/limine.conf"
	limineShadow       = "/boot/limine/limine.conf"
	limineLegacyEFI    = "/boot/EFI/limine/limine.efi"
	limineToolEFI      = "/boot/EFI/limine/limine_x64.efi"
	limineDefaultsPath = "/etc/default/limine"

	liminePkgInstalled = doctorPackageInstalled
)

var limineManagedBoot = func() bool {
	if dnfHost() {
		return false
	}
	if hasPacman() {
		return sys.Has("limine-entry-tool") || sys.Has("limine-mkinitcpio")
	}
	manager, err := doctorPackageManager()
	return err == nil && manager == host.XBPS && doctorPackageInstalled("limine")
}

func unmanagedBootResult() recResult {
	if dnfHost() {
		return okRes(i18n.T("Fedora uses GRUB; this boot-menu check does not apply"))
	}
	return okRes(i18n.T("Ryoku's Limine tooling does not manage this boot"))
}

func limineEFIPath() string {
	if xbpsHost() {
		return "/boot/EFI/ryoku/BOOTX64.EFI"
	}
	return limineToolEFI
}

func limineEFILoader() string {
	if xbpsHost() {
		return `\EFI\ryoku\BOOTX64.EFI`
	}
	return `\EFI\limine\limine_x64.efi`
}

type limineLayoutOutcome int

const (
	limineLayoutSkip limineLayoutOutcome = iota
	limineLayoutOK
	limineLayoutUnreadable
	limineLayoutMigrate
)

// limineLayoutState: the slice of /boot reconcileLimineLayout looks at,
// lifted to a value so planLimineLayout stays unit-testable.
type limineLayoutState struct {
	limineInstalled       bool
	espConfExists         bool
	espConf               string // "" when absent or unreadable
	espConfReadable       bool
	shadowExists          bool
	shadowConf            string
	shadowReadable        bool
	legacyEFIExists       bool
	legacyEFIInaccessible bool
	toolEFIExists         bool
	toolEFIInaccessible   bool
	installerTool         bool // limine-install on PATH
}

// planLimineLayout picks the branch from observed state. pure, no IO.
func planLimineLayout(s limineLayoutState) (limineLayoutOutcome, []string) {
	if !s.limineInstalled {
		return limineLayoutSkip, nil
	}
	if (s.espConfExists && !s.espConfReadable) || (s.shadowExists && !s.shadowReadable) || s.legacyEFIInaccessible || s.toolEFIInaccessible {
		return limineLayoutUnreadable, nil
	}
	if !s.espConfExists && !s.shadowExists {
		// not a limine-booted box (or the ESP isn't at /boot); nothing to own.
		return limineLayoutSkip, nil
	}
	var actions []string
	if s.shadowExists {
		actions = append(actions, fmt.Sprintf(i18n.T("merge %s into %s and remove it (it shadows the generated boot entries: kernels, snapshots)"), limineShadow, limineESPConf))
	} else if limineHasBootTree(s.espConf) {
		if limineDefaultEntry(s.espConf) == "1" {
			actions = append(actions, i18n.T("point default_entry at the newest kernel (entry 1 is the Ryoku directory, which cannot autoboot)"))
		}
		if limineDirtyRoot(s.espConf) {
			actions = append(actions, i18n.T("strip the leftover boot stanza from the Ryoku boot-menu directory (a directory that is also a boot entry cannot autoboot; the countdown loops)"))
		}
	}
	if s.legacyEFIExists {
		actions = append(actions, fmt.Sprintf(i18n.T("retire the stale hand-copied bootloader %s for the package-refreshed %s"), limineLegacyEFI, limineToolEFI))
	}
	if len(actions) == 0 {
		return limineLayoutOK, nil
	}
	return limineLayoutMigrate, actions
}

// probePresence checks whether path exists, distinguishing genuine absence (ENOENT)
// from access or I/O errors.
func probePresence(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

var liminePathProbe = probePresence

// probeFile reads a config path, distinguishing genuine absence (ENOENT) from
// all other filesystem failures (permission errors, I/O errors, etc.). Only
// genuine ENOENT reports absence; any other failure marks the file as existing
// but unreadable so callers avoid false missing-file conclusions.
func probeFile(path string) (content string, exists, readable bool) {
	b, err := os.ReadFile(path)
	if err == nil {
		return string(b), true, true
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", false, false
	}
	return "", true, false
}

func gatherLimineLayoutState() limineLayoutState {
	s := limineLayoutState{
		limineInstalled: liminePkgInstalled("limine"),
		legacyEFIExists: sys.Exists(limineLegacyEFI),
		toolEFIExists:   sys.Exists(limineEFIPath()),
		installerTool:   sys.Has("limine-install"),
	}
	s.espConf, s.espConfExists, s.espConfReadable = probeFile(limineESPConf)
	s.shadowConf, s.shadowExists, s.shadowReadable = probeFile(limineShadow)
	if exists, err := liminePathProbe(limineLegacyEFI); err != nil {
		s.legacyEFIInaccessible = true
	} else {
		s.legacyEFIExists = exists
	}
	if exists, err := liminePathProbe(limineToolEFI); err != nil {
		s.toolEFIInaccessible = true
	} else {
		s.toolEFIExists = exists
	}
	return s
}

func reconcileLimineLayout(checkOnly bool) recResult {
	if !limineManagedBoot() {
		return unmanagedBootResult()
	}
	st := gatherLimineLayoutState()
	outcome, actions := planLimineLayout(st)
	switch outcome {
	case limineLayoutSkip:
		return okRes(i18n.T("not a limine-managed boot on this box"))
	case limineLayoutUnreadable:
		var detail string
		confUnreadable := (st.espConfExists && !st.espConfReadable) || (st.shadowExists && !st.shadowReadable)
		efiInaccessible := st.legacyEFIInaccessible || st.toolEFIInaccessible
		switch {
		case confUnreadable && efiInaccessible:
			detail = i18n.T("cannot read limine config or inspect bootloader binaries under /boot to verify the boot menu layout")
		case efiInaccessible:
			detail = i18n.T("cannot inspect limine bootloader binaries under /boot to verify the boot menu layout")
		default:
			detail = i18n.T("cannot read the limine config under /boot to verify the boot menu layout")
		}
		return warnRes(detail).withFix("sudo ryoku doctor --check")
	case limineLayoutOK:
		return okRes(i18n.T("boot menu lives in /boot/limine.conf; nothing shadows it"))
	}
	if checkOnly {
		return wouldRes(i18n.T("limine boot layout needs migration: %s"), strings.Join(actions, "; ")).
			withFix(i18n.T("ryoku doctor (applies the migration)"))
	}
	return migrateLimineLayout(st)
}

// migrateLimineLayout applies the plan: config first (that alone puts the
// generated kernel + snapshot entries back on screen at next boot), then the
// binary. every step is separately recoverable; the box stays bootable at
// any interruption point because the merged config is written before the
// shadow is removed, and the tool EFI is deployed before the legacy one is
// retired.
func migrateLimineLayout(st limineLayoutState) recResult {
	var done []string

	if st.shadowExists || (limineHasBootTree(st.espConf) && (limineDefaultEntry(st.espConf) == "1" || limineDirtyRoot(st.espConf))) {
		merged := mergeLimineConf(st.espConf, st.shadowConf)
		if st.espConfExists {
			_ = sys.Run("sudo", "cp", limineESPConf, limineESPConf+".ryoku-bak")
		}
		if err := writeBootFile(limineESPConf, merged); err != nil {
			return failRes(i18n.T("writing %s: %v"), limineESPConf, err).
				withFix(i18n.T("re-run with sudo available; the old configs were left untouched"))
		}
		done = append(done, i18n.Tf("merged boot menu into %s", limineESPConf))
		if st.shadowExists {
			if err := sys.Run("sudo", "rm", "-f", limineShadow); err != nil {
				return failRes(i18n.T("removing the shadowing %s: %v (the merged config is written, but limine still reads the shadow)"), limineShadow, err).
					withFix("sudo rm %s", limineShadow)
			}
			_ = exec.Command("sudo", "rmdir", "/boot/limine").Run() // only if empty
			done = append(done, i18n.Tf("removed the shadowing %s", limineShadow))
		}
	}

	if st.legacyEFIExists {
		// Retire the legacy hand-copied bootloader for the package-refreshed
		// path, but NEVER before a boot entry points at the new binary. Deleting
		// the only NVRAM entry with nothing to replace it drops the machine off
		// the firmware boot menu entirely: the "boot option gone after an
		// update, not even in the BIOS" failure.
		if st.installerTool {
			// deploys EFI/limine/limine_x64.efi + the EFI/BOOT fallback and
			// registers the NVRAM entry (deduped by partition uuid + path).
			if err := sys.Run("sudo", "limine-install"); err != nil {
				return warnRes(i18n.T("boot menu migrated (%s), but limine-install failed: %v"), strings.Join(done, "; "), err).
					withFix(i18n.T("sudo limine-install, then sudo rm %s"), limineLegacyEFI)
			}
		} else if sys.Exists("/usr/share/limine/BOOTX64.EFI") {
			// no tool: deploy the fresh binary at the package path, then register
			// the entry the way the installer does. if the entry cannot be
			// written, leave the working legacy boot path alone rather than
			// strand the machine.
			if err := sys.Run("sudo", "cp", "/usr/share/limine/BOOTX64.EFI", limineToolEFI); err != nil {
				return warnRes(i18n.T("boot menu migrated (%s), but could not deploy %s: %v"), strings.Join(done, "; "), limineToolEFI, err).
					withFix("sudo cp /usr/share/limine/BOOTX64.EFI %s", limineToolEFI)
			}
			if !hasRyokuBootEntry(efibootmgrOutput()) {
				if err := registerRyokuBootEntry(); err != nil {
					return warnRes(i18n.T("boot menu migrated (%s), but could not register a boot entry for %s (%v); left the current entry in place so the machine still boots"), strings.Join(done, "; "), limineToolEFI, err).
						withFix(i18n.T("install limine-mkinitcpio-hook, then sudo ryoku doctor"))
				}
			}
		}
		// only retire the stale entry + binary once a live NVRAM entry loads the
		// package-refreshed bootloader.
		if sys.Exists(limineToolEFI) && hasRyokuBootEntry(efibootmgrOutput()) {
			for _, boot := range staleLimineBootNums(efibootmgrOutput()) {
				_ = sys.Run("sudo", "efibootmgr", "-q", "-b", boot, "-B")
			}
			if err := sys.Run("sudo", "rm", "-f", limineLegacyEFI); err != nil {
				return warnRes(i18n.T("boot menu migrated (%s), but could not remove the stale %s: %v"), strings.Join(done, "; "), limineLegacyEFI, err).
					withFix("sudo rm %s", limineLegacyEFI)
			}
			done = append(done, i18n.T("bootloader binary now on the package-refreshed path"))
		}
	}

	// the shadow (or the rewrite) may have carried a Windows chainload block;
	// re-assert it against the merged config. best-effort, needs root mounts.
	if sys.Has("ryoku-windows-entry") {
		_ = exec.Command("sudo", "ryoku-windows-entry", "sync").Run()
	}

	return fixedRes(i18n.T("%s (snapshots and new kernels appear in the boot menu from the next boot)"), strings.Join(done, "; "))
}

// limineHasBootTree: has limine-mkinitcpio-hook taken over the file? two
// shapes qualify: the older tool writes a standalone expanded directory
// ("/+Ryoku"); 1.37+ adopts the flat "/Ryoku Linux" placeholder as the menu
// directory and nests "//<kernel>" sub-entries under it. either means a
// directory sits at entry 1, which cannot autoboot. the flat installer
// placeholder and foreign leaf entries carry neither a "/+" nor a "//".
func limineHasBootTree(conf string) bool {
	for _, line := range strings.Split(conf, "\n") {
		if strings.HasPrefix(line, "/+") {
			return true
		}
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			return true
		}
	}
	return false
}

// limineDefaultEntry: the value of the global default_entry option, "" when
// absent.
func limineDefaultEntry(conf string) string {
	for _, line := range strings.Split(conf, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "default_entry:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// liminePlaceholderBodyKeys: the local boot options ryoku's flat "/Ryoku
// Linux" placeholder carries (installer bootloader.sh with_entry). when
// limine-entry-tool 1.37+ adopts that placeholder as the menu directory and
// nests the "//<kernel>" UKIs under it, this stanza is left wedged between the
// directory title and its first sub-entry -- where Limine's grammar allows
// only a `comment`. a directory that is also a boot entry cannot autoboot: the
// timeout resolves nothing bootable and the countdown restarts forever.
var liminePlaceholderBodyKeys = []string{
	"protocol:", "kernel_path:", "module_path:", "path:", "cmdline:",
}

func liminePlaceholderBodyKey(trimmed string) bool {
	for _, k := range liminePlaceholderBodyKeys {
		if strings.HasPrefix(trimmed, k) {
			return true
		}
	}
	return false
}

// limineDirtyRoot reports whether the "/Ryoku Linux" menu directory still
// carries the leftover placeholder boot stanza before its first sub-entry.
func limineDirtyRoot(conf string) bool {
	return stripLiminePlaceholderBody(conf) != conf
}

// stripLiminePlaceholderBody removes the flat-placeholder boot stanza (and the
// blank lines around it) wedged under the "/Ryoku Linux" directory title,
// keeping the title, any `comment:` lines, and every sub-entry. it is a no-op
// unless "/Ryoku Linux" is a directory (a "//" sub-entry follows its head), so
// the legitimate flat placeholder of an offline install is never touched.
func stripLiminePlaceholderBody(conf string) string {
	lines := strings.Split(conf, "\n")
	start := -1
	for i, ln := range lines {
		if ln == "/Ryoku Linux" {
			start = i
			break
		}
	}
	if start < 0 {
		return conf
	}
	var drop []int
	isDir := false
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "//") { // a sub-entry: it is a directory
			isDir = true
			break
		}
		if strings.HasPrefix(lines[i], "/") { // next top-level entry: no sub-entry
			break
		}
		if trimmed == "" || liminePlaceholderBodyKey(trimmed) {
			drop = append(drop, i)
		}
	}
	if !isDir || len(drop) == 0 {
		return conf
	}
	dropSet := make(map[int]bool, len(drop))
	for _, i := range drop {
		dropSet[i] = true
	}
	var out []string
	for i, ln := range lines {
		if dropSet[i] {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// limineBrandedKeys: global options the canonical branding header owns. when
// merging, lines carrying these are dropped from the existing prelude so the
// header's values win without duplicates. everything else a user (or the
// tool) put in the prelude -- quiet, remember_last_entry, interface_resolution,
// macros -- survives.
var limineBrandedKeys = []string{
	"timeout:", "default_entry:", "remember_last_entry:", "interface_branding:",
	"interface_branding_color:", "interface_branding_colour:",
	"interface_help_color:", "interface_help_colour:",
	"interface_help_color_bright:", "interface_help_colour_bright:",
	"hash_mismatch_panic:", "term_background:", "backdrop:", "wallpaper:",
	"term_palette:", "term_palette_bright:", "term_foreground:",
	"term_foreground_bright:", "term_background_bright:",
}

// limineForcedKeys are the only globals set over a user's value: the boot
// identity, and the flag a snapshot entry needs to boot at all. Everything
// else is seeded when absent and never reset.
var limineForcedKeys = []string{"interface_branding:", "hash_mismatch_panic:"}

func limineLineKey(trimmed string) string {
	if i := strings.IndexByte(trimmed, ':'); i >= 0 {
		return strings.ToLower(trimmed[:i+1])
	}
	return ""
}

func limineForcedKey(trimmed string) bool {
	k := limineLineKey(trimmed)
	for _, f := range limineForcedKeys {
		if k == f {
			return true
		}
	}
	return false
}

// mergeLimineConf builds the migrated /boot/limine.conf: the canonical Ryoku
// branding header, then whatever non-branding globals the base prelude
// carried, then the base's entries verbatim. the base is the ESP-root config
// when the tool's boot tree lives there (never throw generated entries
// away), else the shadow (the menu the firmware was actually showing).
// default_entry is then pointed at the kernel's entry path (or 1 on a flat
// menu) and remember_last_entry ensured by limineEnsureAutoboot, so the merged
// menu autoboots the kernel instead of looping on the /EFI fallback.
func mergeLimineConf(espConf, shadowConf string) string {
	base := espConf
	if !limineHasBootTree(espConf) && shadowConf != "" {
		base = shadowConf
	}
	prelude, body := splitLimineConf(base)
	body = stripLiminePlaceholderBody(body)

	baseBranded := map[string]string{}
	var extras []string
	for _, line := range strings.Split(prelude, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue // blank lines and header comments are restated below
		}
		if limineBrandedKey(t) {
			if k := limineLineKey(t); baseBranded[k] == "" {
				baseBranded[k] = line
			}
			continue
		}
		extras = append(extras, line)
	}

	var b strings.Builder
	b.WriteString("# Ryoku limine config -- branding globals + generated entries. managed by\n")
	b.WriteString("# limine-mkinitcpio-hook / limine-snapper-sync (entries) and ryoku (globals).\n")
	emitted := map[string]bool{}
	for _, line := range strings.Split(strings.TrimRight(limineBranding, "\n"), "\n") {
		t := strings.TrimSpace(line)
		if limineBrandedKey(t) {
			k := limineLineKey(t)
			emitted[k] = true
			if !limineForcedKey(t) {
				if bl, ok := baseBranded[k]; ok {
					b.WriteString(bl + "\n")
					continue
				}
			}
		}
		b.WriteString(line + "\n")
	}
	for _, k := range limineBrandedKeys {
		kk := strings.ToLower(k)
		if !emitted[kk] {
			if bl, ok := baseBranded[kk]; ok {
				b.WriteString(bl + "\n")
			}
		}
	}
	if len(extras) > 0 {
		b.WriteString(strings.Join(extras, "\n"))
		b.WriteString("\n")
	}
	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
	}
	out := b.String()
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out, _ = limineEnsureAutoboot(out)
	return out
}
func limineBrandedKey(trimmedLine string) bool {
	l := strings.ToLower(trimmedLine)
	for _, k := range limineBrandedKeys {
		if strings.HasPrefix(l, k) {
			return true
		}
	}
	return false
}

// splitLimineConf: prelude (global options before the first menu entry) and
// body (the first "/" entry line to EOF, verbatim -- entries, sub-entries,
// their comments and fences).
func splitLimineConf(conf string) (prelude, body string) {
	lines := strings.Split(conf, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "/") {
			return strings.Join(lines[:i], "\n"), strings.Join(lines[i:], "\n")
		}
	}
	return conf, ""
}

// staleLimineBootNums: NVRAM boot numbers whose loader is the legacy
// hand-copied \EFI\limine\limine.efi (never \EFI\limine\limine_x64.efi).
// parsed from `efibootmgr` output lines like
//
//	Boot0003* Ryoku HD(1,GPT,...)/\EFI\limine\limine.efi
func staleLimineBootNums(efibootmgr string) []string {
	var nums []string
	for _, line := range strings.Split(efibootmgr, "\n") {
		if !strings.HasPrefix(line, "Boot") || len(line) < 8 {
			continue
		}
		num := line[4:8]
		if !isHex4(num) {
			continue
		}
		if strings.Contains(line, `\limine.efi`) {
			nums = append(nums, num)
		}
	}
	return nums
}

func isHex4(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func efibootmgrOutput() string {
	out, err := exec.Command("efibootmgr").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// hasRyokuBootEntry: does any active NVRAM entry boot the package-refreshed
// limine? limine-install registers its entry labeled "Limine" with a VenHw
// device path (no file path), while the installer writes a "Ryoku" entry
// loading EFI/limine/limine_x64.efi. Match either, but NOT the legacy
// \limine.efi entry (staleLimineBootNums owns that). pure, so the "do not
// retire the old entry until a replacement exists" guard and the missing-entry
// reconciler are testable without efibootmgr.
func hasRyokuBootEntry(efibootmgr string) bool {
	for _, line := range strings.Split(efibootmgr, "\n") {
		if len(line) < 9 || !strings.HasPrefix(line, "Boot") || !isHex4(line[4:8]) || line[8] != '*' {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, `\limine_x64.efi`) ||
			strings.Contains(lower, `\efi\ryoku\bootx64.efi`) ||
			limineBootLabel(line) == "Limine" {
			return true
		}
	}
	return false
}

// limineBootLabel: the label field of an efibootmgr entry line
// ("Boot0004* Limine\tVenHw(...)" -> "Limine"). efibootmgr separates the label
// from the device path with a tab; fall back to the first field for
// space-separated output. "" when the line is not a boot entry.
func limineBootLabel(line string) string {
	if len(line) < 8 || !isHex4(line[4:8]) {
		return ""
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line[8:], "*"))
	if i := strings.IndexByte(rest, '\t'); i >= 0 {
		return strings.TrimSpace(rest[:i])
	}
	if fields := strings.Fields(rest); len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// parseEspDiskPart derives the efibootmgr --disk (whole device) and --part
// (number) from the ESP mount source, its parent-disk name (lsblk PKNAME), and
// the partition's sysfs number. pure: the wrangling that decides what NVRAM
// entry gets written, tested without real block devices.
func parseEspDiskPart(source, pkname, partition string) (disk, part string, ok bool) {
	source = strings.TrimSpace(source)
	pkname = strings.TrimSpace(pkname)
	part = strings.TrimSpace(partition)
	if !strings.HasPrefix(source, "/dev/") || pkname == "" || part == "" {
		return "", "", false
	}
	return "/dev/" + pkname, part, true
}

// espDiskPart resolves the ESP block device backing /boot into an efibootmgr
// --disk / --part pair.
func espDiskPart() (disk, part string, ok bool) {
	out, err := exec.Command("findmnt", "-n", "-o", "SOURCE", "--target", "/boot").Output()
	if err != nil {
		return "", "", false
	}
	src := strings.TrimSpace(string(out))
	pk, err := exec.Command("lsblk", "-no", "PKNAME", src).Output()
	if err != nil {
		return "", "", false
	}
	partition := readFileSafe("/sys/class/block/" + filepath.Base(src) + "/partition")
	return parseEspDiskPart(src, string(pk), partition)
}

// registerRyokuBootEntry writes the UEFI boot entry the installer writes.
func registerRyokuBootEntry() error {
	disk, part, ok := espDiskPart()
	if !ok {
		return fmt.Errorf(i18n.T("could not determine the ESP disk and partition"))
	}
	return sys.Run("sudo", "efibootmgr", "--create", "--disk", disk, "--part", part,
		"--label", "Ryoku", "--loader", limineEFILoader(), "--unicode")
}

// ---- reconciler: limine UEFI boot entry --------------------------------------

// reconcileLimineBootEntry restores a vanished Ryoku UEFI boot entry. When the
// firmware has no NVRAM entry loading the package-refreshed bootloader but the
// binary is on the ESP, the machine has dropped off the boot menu (an earlier
// migrate bug retired the old entry without writing a replacement) and boots
// only via the removable EFI/BOOT fallback, if at all. Re-register the entry
// exactly as the installer does. Idempotent: no-op once an entry loads the
// bootloader, and it stands aside for the layout migration while a legacy entry
// is still there to convert.
func reconcileLimineBootEntry(checkOnly bool) recResult {
	if !limineManagedBoot() {
		return unmanagedBootResult()
	}
	if !liminePkgInstalled("limine") {
		return okRes(i18n.T("not a limine-managed boot on this box"))
	}
	if exists, err := liminePathProbe(limineEFIPath()); err != nil {
		return warnRes(i18n.T("cannot inspect %s to verify the UEFI boot entry"), limineEFIPath()).
			withFix("sudo ryoku doctor --check")
	} else if !exists {
		return okRes(i18n.T("not a limine-managed boot on this box"))
	}
	if !sys.Has("efibootmgr") {
		return okRes(i18n.T("no efibootmgr to inspect the UEFI boot menu"))
	}
	out := efibootmgrOutput()
	if out == "" {
		return okRes(i18n.T("no UEFI boot entries to check"))
	}
	if hasRyokuBootEntry(out) {
		return okRes(i18n.T("Ryoku UEFI boot entry present"))
	}
	if len(staleLimineBootNums(out)) > 0 {
		return okRes(i18n.T("legacy limine boot entry present; the layout migration owns it"))
	}
	fix := fmt.Sprintf(`sudo efibootmgr --create --disk <ESP disk> --part <ESP part> --label Ryoku --loader '%s' --unicode`, limineEFILoader())
	if checkOnly {
		return wouldRes(i18n.T("no UEFI boot entry loads the Ryoku bootloader; the boot option is missing from firmware")).
			withFix(fix)
	}
	if err := registerRyokuBootEntry(); err != nil {
		return failRes(i18n.T("the Ryoku UEFI boot entry is missing and could not be re-registered: %v"), err).
			withFix(fix)
	}
	return fixedRes(i18n.T("re-registered the missing Ryoku UEFI boot entry"))
}

// writeBootFile: stage + `sudo cp` (not install -o/-g: the ESP is vfat, which
// has no owners to set and rejects chown).
func writeBootFile(path, contents string) error {
	tmp, err := os.CreateTemp("", "ryoku-limine-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(contents); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return sys.Run("sudo", "cp", tmp.Name(), path)
}

// ---- reconciler: limine snapshot-sync OS name --------------------------------

// reconcileLimineOSName aligns TARGET_OS_NAME in /etc/default/limine with the
// name of the actual Ryoku boot entry in /boot/limine.conf, so limine-snapper-sync
// can find it to hang the Snapshots submenu under. the healthy menu is the
// "/+Ryoku" UKI tree (name "Ryoku") that limine-mkinitcpio-hook generates, and the
// shipped default already matches it -- a no-op. but a box still on the flat
// "/Ryoku Linux" fallback entry needs that name instead, and the mismatch fails
// limine-snapper-sync on every snapper-cleanup (its ExecStopPost), so the boot
// menu never lists a rollback snapshot. read the real entry name and converge to
// it, then clear the stale failed state. never invents a name: no Ryoku entry
// found -> leave it alone.
// ---- reconciler: limine UKI boot tree ------------------------------------------

// reconcileLimineUKITree converges a limine box onto the hook-owned boot menu
// the design always intended: /etc/default/limine ships with ENABLE_UKI=yes
// and aur.packages carries limine-mkinitcpio-hook, but boxes installed before
// (or without) the AUR step run on the flat "/Ryoku Linux" placeholder
// forever. limine-snapper-sync refuses to hang the Snapshots submenu under an
// entry with no "//<kernel>" sub-entries, so those boxes never see a rollback
// in the boot menu at all; omarchy works because its installer hard-requires
// the hook and fails the install when no "/+" tree appears. Install the hook
// (its deploy hook builds the UKIs and regenerates the entries), drop the
// flat placeholder the way the installer's finalize does, and run one sync so
// the snapshots show up now, not at the next snapper event.
func reconcileLimineUKITree(checkOnly bool) recResult {
	if xbpsHost() {
		return okRes(i18n.T("Void boots a kernel plus a dracut initramfs image; the UKI boot-tree check does not apply"))
	}
	if !limineManagedBoot() {
		return unmanagedBootResult()
	}
	if !liminePkgInstalled("limine") {
		return okRes(i18n.T("not a limine-managed boot on this box"))
	}
	defaults, defaultsExist, defaultsReadable := probeFile(limineDefaultsPath)
	if !defaultsExist {
		return okRes(i18n.T("limine box without the UKI design (no ENABLE_UKI); nothing to converge"))
	}
	if !defaultsReadable {
		return warnRes(i18n.T("cannot read %s to verify the UKI boot tree configuration"), limineDefaultsPath).
			withFix("sudo ryoku doctor --check")
	}
	if !strings.Contains(defaults, "ENABLE_UKI=yes") {
		return okRes(i18n.T("limine box without the UKI design (no ENABLE_UKI); nothing to converge"))
	}
	b, err := os.ReadFile(limineESPConf)
	if err != nil {
		return noteRes(i18n.T("no readable %s; the layout reconciler owns that"), limineESPConf)
	}
	conf := string(b)
	hookMissing := !liminePkgInstalled("limine-mkinitcpio-hook")
	_, hasFlat := limineDropFlat(conf)
	if !hookMissing && limineHasUKITree(conf) && !hasFlat {
		return okRes(i18n.T("limine-mkinitcpio-hook owns the boot menu (UKI tree with kernel sub-entries)"))
	}
	if checkOnly {
		return wouldRes(i18n.T("the boot menu is still the flat install placeholder, so limine-snapper-sync cannot add the Snapshots submenu (rollbacks never appear at boot)")).
			withFix(i18n.T("ryoku doctor installs limine-mkinitcpio-hook and promotes the menu to the /+Ryoku UKI tree"))
	}
	var done []string
	if hookMissing {
		if err := sys.Run("ryoku-pkg-aur-add", "limine-mkinitcpio-hook"); err != nil {
			return failRes(i18n.T("could not install limine-mkinitcpio-hook: %v"), err).
				withFix(i18n.T("ryoku-pkg-aur-add limine-mkinitcpio-hook, then sudo ryoku doctor"))
		}
		done = append(done, i18n.T("installed limine-mkinitcpio-hook"))
	}
	// the install's deploy hook normally regenerates the menu; if the tree is
	// still absent (hook was present but never ran), ask for it explicitly.
	if !limineHasUKITree(readFileSafe(limineESPConf)) && sys.Has("limine-update") {
		if err := sys.Sudo("limine-update"); err != nil {
			return failRes(i18n.T("limine-update could not build the UKI boot tree: %v"), err).
				withFix(i18n.T("sudo limine-update, then sudo ryoku doctor"))
		}
		done = append(done, i18n.T("rebuilt the boot menu with limine-update"))
	}
	conf = readFileSafe(limineESPConf)
	if !limineHasUKITree(conf) {
		return failRes(i18n.T("no UKI kernel entries in %s even after limine-update; snapshots cannot attach"), limineESPConf)
	}
	// mirror the installer's finalize: with a standalone tree the flat
	// placeholder is clutter; either way a directory can't autoboot, so
	// default_entry moves to the newest UKI inside it.
	if promoted, changed := limineDropFlat(conf); changed {
		// readFileSafe trimmed the file's trailing newline; put it back.
		if err := writeRootFile(limineESPConf, strings.TrimRight(promoted, "\n")+"\n", "0644"); err != nil {
			return failRes(i18n.T("could not promote the boot menu in %s: %v"), limineESPConf, err)
		}
		done = append(done, i18n.T("promoted the menu default onto the UKI tree"))
	}
	if sys.Has("limine-snapper-sync") {
		if err := sys.Sudo("limine-snapper-sync"); err == nil {
			done = append(done, i18n.T("synced the Snapshots submenu"))
		}
	}
	return fixedRes(i18n.T("boot menu converged onto the UKI tree: %s; rollback snapshots now appear at boot"), strings.Join(done, "; "))
}

// limineHasUKITree: does the config carry tool-generated kernel sub-entries.
// older limine-entry-tool writes a standalone "/+Name" expanded tree; 1.37+
// adopts the installer's flat entry as the tree root and nests indented
// "//<kernel>" children under it. either shape satisfies limine-snapper-sync.
func limineHasUKITree(conf string) bool {
	for _, l := range strings.Split(conf, "\n") {
		if strings.HasPrefix(l, "/+") {
			return true
		}
		if t := strings.TrimLeft(l, " \t"); strings.HasPrefix(t, "//") {
			return true
		}
	}
	return false
}

// limineDropFlat promotes the menu past the install placeholder, mirroring the
// installer's finalize. with a standalone "/+" tree the flat "/Ryoku Linux..."
// entries (entry line plus indented options) are clutter and go; in the
// adopted layout the placeholder IS the tree root, so entries stay untouched.
// either way default_entry: 1 points at a directory that can't autoboot and
// moves to 2 (the first UKI inside). pure, so it is unit-testable;
// changed=false when there is nothing to do.
func limineDropFlat(conf string) (string, bool) {
	dropFlat := false
	for _, l := range strings.Split(conf, "\n") {
		if strings.HasPrefix(l, "/+") {
			dropFlat = true
			break
		}
	}
	var out []string
	skip, changed := false, false
	for _, l := range strings.Split(conf, "\n") {
		if dropFlat && strings.HasPrefix(l, "/Ryoku Linux") {
			skip, changed = true, true
			continue
		}
		if skip && strings.TrimSpace(l) != "" && (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) {
			continue
		}
		skip = false
		if l == "default_entry: 1" {
			out = append(out, "default_entry: 2")
			changed = true
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n"), changed
}

func reconcileLimineOSName(checkOnly bool) recResult {
	if !limineManagedBoot() {
		return unmanagedBootResult()
	}
	path := limineDefaultsPath
	if !doctorPackageInstalled("limine-snapper-sync") {
		return okRes(i18n.T("limine snapshot sync is not installed"))
	}
	cur, exists, readable := probeFile(path)
	if exists && !readable {
		return warnRes(i18n.T("cannot read %s to verify snapshot sync OS name"), path).
			withFix("sudo ryoku doctor --check")
	}
	espConf, espExists, espReadable := probeFile(limineESPConf)
	if espExists && !espReadable {
		return noteRes(i18n.T("no readable %s; the layout reconciler owns that"), limineESPConf)
	}
	want := limineEntryName(espConf)
	if want == "" {
		return okRes(i18n.T("no Ryoku boot entry found to match TARGET_OS_NAME against"))
	}
	if !exists {
		if !xbpsHost() {
			return okRes(i18n.T("no /etc/default/limine (limine snapshot sync not in use)"))
		}
		if checkOnly {
			return wouldRes(i18n.T("%s is missing, so limine-snapper-sync cannot attach snapshots under %q"), path, want).
				withFix(i18n.T("ryoku doctor creates the limine snapshot-sync defaults"))
		}
		sourceRaw, sourceErr := os.ReadFile("/usr/share/ryoku/boot/default.conf")
		if sourceErr != nil {
			return failRes(i18n.T("could not read the shipped limine snapshot-sync defaults: %v"), sourceErr).
				withFix(doctorInstallAdvice("ryoku-desktop"))
		}
		defaults := limineSnapshotDefaults(string(sourceRaw), want)
		if defaults == "" {
			return failRes(i18n.T("the shipped limine snapshot-sync defaults are incomplete")).
				withFix(doctorInstallAdvice("ryoku-desktop"))
		}
		if err := writeRootFile(path, defaults, "0644"); err != nil {
			return failRes(i18n.T("could not create %s: %v"), path, err)
		}
		_ = runSystemService("reset-failed", "snapper-cleanup.service")
		return fixedRes(i18n.T("created %s with TARGET_OS_NAME %q so snapshots sync"), path, want)
	}
	got, ok := limineOSNameValue(cur)
	if !ok {
		if !xbpsHost() {
			return okRes(i18n.T("limine config sets no TARGET_OS_NAME"))
		}
		got = ""
	}
	if got == want {
		return okRes(i18n.T("limine snapshot entries sync under %q"), want)
	}
	if checkOnly {
		return wouldRes(i18n.T("TARGET_OS_NAME %q does not match the boot entry %q, so limine-snapper-sync fails snapper-cleanup"), got, want).
			withFix(i18n.T("ryoku doctor sets TARGET_OS_NAME to %q"), want)
	}
	if err := writeRootFile(path, setLimineOSName(cur, want), "0644"); err != nil {
		return failRes(i18n.T("could not update %s: %v"), path, err)
	}
	_ = runSystemService("reset-failed", "snapper-cleanup.service")
	return fixedRes(i18n.T("set TARGET_OS_NAME to %q to match the boot entry so snapshots sync"), want)
}

// limineEntryName: the name of the primary Ryoku OS entry in a /boot/limine.conf.
// the expanded UKI tree ("/+Ryoku" -> "Ryoku") wins over the flat fallback
// ("/Ryoku Linux" -> "Ryoku Linux") when both are present, so a healthy box that
// still carries a stray flat placeholder is never re-pointed off its real entry.
// top-level entries only ("/name" or "/+name", not a "//" sub-entry); "" when no
// Ryoku entry is found.
func limineEntryName(conf string) string {
	var flat string
	for _, l := range strings.Split(conf, "\n") {
		t := strings.TrimRight(l, " \t\r")
		if !strings.HasPrefix(t, "/") || strings.HasPrefix(t, "//") {
			continue
		}
		expanded := strings.HasPrefix(t, "/+")
		name := strings.TrimPrefix(strings.TrimPrefix(t, "/"), "+")
		if !strings.HasPrefix(name, "Ryoku") {
			continue
		}
		if expanded {
			return name
		}
		if flat == "" {
			flat = name
		}
	}
	return flat
}

// limineOSNameValue pulls the TARGET_OS_NAME value out of an /etc/default/limine,
// quotes stripped. ok=false when there is no such assignment.
func limineOSNameValue(conf string) (string, bool) {
	for _, l := range strings.Split(conf, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "#") || !strings.HasPrefix(t, "TARGET_OS_NAME") {
			continue
		}
		if eq := strings.IndexByte(t, '='); eq >= 0 {
			return strings.Trim(strings.TrimSpace(t[eq+1:]), "\"'"), true
		}
	}
	return "", false
}

var limineSnapshotKeys = map[string]bool{
	"TARGET_OS_NAME":         true,
	"ESP_PATH":               true,
	"MAX_SNAPSHOT_ENTRIES":   true,
	"SNAPSHOT_FORMAT_CHOICE": true,
}

func limineSnapshotDefaults(source, name string) string {
	var lines []string
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		key, _, ok := strings.Cut(trimmed, "=")
		if !ok || !limineSnapshotKeys[key] {
			continue
		}
		if key == "TARGET_OS_NAME" {
			line = fmt.Sprintf("TARGET_OS_NAME=%q", name)
		}
		lines = append(lines, line)
	}
	if len(lines) != len(limineSnapshotKeys) {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// setLimineOSName rewrites the TARGET_OS_NAME assignment to name, every other
// line preserved verbatim.
func setLimineOSName(conf, name string) string {
	lines := strings.Split(conf, "\n")
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "#") || !strings.HasPrefix(t, "TARGET_OS_NAME") {
			continue
		}
		if strings.IndexByte(t, '=') >= 0 {
			lines[i] = fmt.Sprintf("TARGET_OS_NAME=%q", name)
			return strings.Join(lines, "\n")
		}
	}
	if conf != "" && !strings.HasSuffix(conf, "\n") {
		conf += "\n"
	}
	return conf + fmt.Sprintf("TARGET_OS_NAME=%q\n", name)
}

// ---- reconciler: limine autoboot (the countdown must boot, not loop) ----------

// limineDepth counts the leading '/' of a whitespace-trimmed menu line: 1 for a
// top-level entry ("/Ryoku Linux"), 2 for a sub-entry ("//linux"), and so on.
func limineDepth(trimmed string) int {
	n := 0
	for n < len(trimmed) && trimmed[n] == '/' {
		n++
	}
	return n
}

// limineNodeName is the title of a menu line: leading slashes and the optional
// '+' expanded-directory marker stripped.
func limineNodeName(trimmed string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimLeft(trimmed, "/"), "+"))
}

// limineKernelPaths: the entry paths of the kernels under the OS directory,
// in menu order. A numeric default_entry counts top-level entries only, so on
// the collapsed layout a bare index lands on the EFI fallback and loops the
// countdown; a path addresses the kernel and autoboots.
func limineKernelPaths(conf string) []string {
	var out []string
	dir, inDir := "", false
	for _, l := range strings.Split(conf, "\n") {
		t := strings.TrimLeft(l, " \t")
		switch limineDepth(t) {
		case 1:
			dir, inDir = limineNodeName(t), true
		case 2:
			if inDir {
				if child := limineNodeName(t); child != "" && !strings.EqualFold(child, "Snapshots") {
					out = append(out, dir+"/"+child)
				}
			}
		}
	}
	return out
}

func limineFirstKernelPath(conf string) string {
	if paths := limineKernelPaths(conf); len(paths) > 0 {
		return paths[0]
	}
	return ""
}

// defaultKernelFile records the kernel the install chose to boot, written by
// the installer's bootloader step from the variant it built (`linux` or
// `linux-cachyos`). It is the only statement of intent a live box has: a kernel
// package the user added later is indistinguishable from the one the install
// was built around.
const defaultKernelFile = "/etc/ryoku/default-kernel"

// pickKernelPath: which kernel entry the countdown should autoboot. prefer is
// exact kernel names in priority order: the kernel the install recorded, then
// the one this session booted. No kernel name is spelled out here and none is
// preferred by brand: Ryoku ships two variants and a box may carry any set of
// kernels, so the menu order stands unless the box itself says otherwise.
func pickKernelPath(paths, prefer []string) string {
	if len(paths) == 0 {
		return ""
	}
	for _, want := range prefer {
		if want == "" {
			continue
		}
		for _, p := range paths {
			if strings.EqualFold(p[strings.LastIndex(p, "/")+1:], want) {
				return p
			}
		}
	}
	return paths[0]
}

// limineDefaultKernelPath reads the box, then picks.
func limineDefaultKernelPath(conf string) string {
	return pickKernelPath(limineKernelPaths(conf),
		[]string{readKernelName(defaultKernelFile), runningKernelPkgbase()})
}

// readKernelName: a one-line kernel name from path, or "" when it is unreadable
// or holds anything else. readFileSafe returns its error as text, which must
// never reach the name comparison.
func readKernelName(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(b))
	if name == "" || strings.ContainsAny(name, " \t\n/") {
		return ""
	}
	return name
}

// runningKernelPkgbase: the pkgbase of the kernel this session booted, from the
// module tree's pkgbase file ("linux", "linux-cachyos", ...); "" when the tree
// is gone, which is exactly the stale-image case the images reconciler owns.
func runningKernelPkgbase() string {
	release := readKernelName("/proc/sys/kernel/osrelease")
	if release == "" {
		return ""
	}
	return readKernelName(filepath.Join("/usr/lib/modules", release, "pkgbase"))
}

// limineEnsureAutoboot rewrites default_entry only when it is absent or a bare
// index (the countdown loop); a path or title is the user's and stays.
func limineEnsureAutoboot(conf string) (string, bool) {
	prelude, body := splitLimineConf(conf)
	current := strings.TrimSpace(limineDefaultEntry(conf))
	prefer := limineDefaultKernelPath(conf)

	wantDefault := current
	if current == "" || limineIsNumeric(current) {
		if prefer != "" {
			wantDefault = prefer
		} else {
			wantDefault = "1"
		}
	}

	var out []string
	changed := false
	haveDefault, haveRemember := false, false
	for _, l := range strings.Split(prelude, "\n") {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "default_entry:"):
			haveDefault = true
			nl := "default_entry: " + wantDefault
			out = append(out, nl)
			changed = changed || l != nl
		case strings.HasPrefix(t, "remember_last_entry:"):
			haveRemember = true
			out = append(out, l) // an existing value is the user's, keep it
		default:
			out = append(out, l)
		}
	}
	if !haveDefault {
		out = append(out, "default_entry: "+wantDefault)
		changed = true
	}
	if !haveRemember {
		out = append(out, "remember_last_entry: yes")
		changed = true
	}
	newPrelude := strings.Join(out, "\n")
	if body == "" {
		return newPrelude, changed
	}
	return newPrelude + "\n" + body, changed
}

func limineIsNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// reconcileLimineAutoboot makes the countdown autoboot the kernel instead of
// looping. The installer and older reconcilers set a numeric default_entry
// believing it lands on the kernel "inside" the boot-menu directory, but Limine
// counts top-level entries, so on the hook's collapsed-directory layout the
// index lands on the "/EFI fallback" -- which chainloads Limine again, so the
// timeout re-shows the menu forever and the user must pick the kernel by hand.
// Point default_entry at the kernel's entry path and remember the last booted
// kernel. Runs on every `ryoku update`, so existing boxes heal without a
// reinstall; idempotent once the prelude is right.
func reconcileLimineAutoboot(checkOnly bool) recResult {
	if !limineManagedBoot() {
		return unmanagedBootResult()
	}
	if !doctorPackageInstalled("limine") {
		return okRes(i18n.T("not a limine-managed boot on this box"))
	}
	b, err := os.ReadFile(limineESPConf)
	if err != nil {
		return noteRes(i18n.T("no readable %s; the layout reconciler owns that"), limineESPConf)
	}
	fixed, changed := limineEnsureAutoboot(string(b))
	if !changed {
		return okRes(i18n.T("limine autoboots the kernel and remembers the last one"))
	}
	if checkOnly {
		return wouldRes(i18n.T("limine default_entry lands on the EFI fallback, not the kernel, so the boot countdown loops forever")).
			withFix(i18n.T("ryoku doctor points default_entry at the kernel entry path and enables remember_last_entry"))
	}
	if err := writeRootFile(limineESPConf, fixed, "0644"); err != nil {
		return failRes(i18n.T("could not update %s: %v"), limineESPConf, err)
	}
	return fixedRes(i18n.T("pointed default_entry at the kernel entry path and enabled remember_last_entry, so the countdown autoboots the last kernel used"))
}
