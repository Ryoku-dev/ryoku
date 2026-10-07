// system.go holds everything that talks to the real machine: live lists for the
// pickers (keymaps, locales, time zones, disks, Wi-Fi), hardware detection, the
// small live actions (apply a keymap, hash a password, connect Wi-Fi), and the
// streamed handoff to the install backend. main.go stays pure UI; this file is the
// only place that shells out.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"ryoku-i18n"
	wm "ryoku-wm"
)

// run executes a command and returns its trimmed stdout, plus whether it worked.
// A missing tool or non-zero exit just yields ok=false so callers fall back.
func run(name string, args ...string) (string, bool) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// promote reorders items so the preferred keys lead (nice defaults at the top of a
// long system list), keeping everything else in its original order.
func promote(items []item, prefer []string) []item {
	idx := map[string]int{}
	for i, k := range prefer {
		idx[k] = i
	}
	lead := make([]item, len(prefer))
	var rest []item
	var have []bool = make([]bool, len(prefer))
	for _, it := range items {
		if p, ok := idx[it.key]; ok {
			lead[p] = it
			have[p] = true
		} else {
			rest = append(rest, it)
		}
	}
	var out []item
	for i, it := range lead {
		if have[i] {
			out = append(out, it)
		}
	}
	return append(out, rest...)
}

// sysKeymaps lists console keymaps from localectl. WIRE target.
func sysKeymaps() []item {
	out, ok := run("localectl", "list-keymaps")
	if !ok {
		return nil
	}
	labels := map[string]string{
		"us": i18n.T("US (QWERTY)"), "uk": i18n.T("United Kingdom"), "gb": i18n.T("United Kingdom"),
		"de": i18n.T("German"), "fr": i18n.T("French (AZERTY)"), "es": i18n.T("Spanish"), "it": i18n.T("Italian"),
		"dvorak": "Dvorak", "colemak": "Colemak",
	}
	var items []item
	for _, l := range strings.Split(out, "\n") {
		c := strings.TrimSpace(l)
		if c == "" {
			continue
		}
		label := c
		if v, ok := labels[c]; ok {
			label = v
		}
		items = append(items, item{c, label, ""})
	}
	return promote(items, []string{"us", "uk", "gb", "de", "fr", "es", "it", "dvorak", "colemak"})
}

// sysLocales reads the supported UTF-8 locales (the ones we can generate).
func sysLocales() []item {
	var lines []string
	if data, err := os.ReadFile("/usr/share/i18n/SUPPORTED"); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			if strings.Contains(l, "UTF-8") {
				if f := strings.Fields(l); len(f) > 0 {
					lines = append(lines, f[0])
				}
			}
		}
	}
	if len(lines) == 0 {
		if out, ok := run("locale", "-a"); ok {
			for _, l := range strings.Split(out, "\n") {
				if strings.Contains(strings.ToLower(l), "utf") {
					lines = append(lines, strings.TrimSpace(l))
				}
			}
		}
	}
	var items []item
	for _, c := range lines {
		if c == "" {
			continue
		}
		items = append(items, item{c, c, ""})
	}
	return promote(items, []string{"en_US.UTF-8", "en_GB.UTF-8", "de_DE.UTF-8", "fr_FR.UTF-8", "es_ES.UTF-8"})
}

func localeTerritory(locale string) string {
	v := locale
	if i := strings.IndexAny(v, ".@"); i >= 0 {
		v = v[:i]
	}
	parts := strings.SplitN(v, "_", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.ToUpper(parts[1])
}

// promoteKbLocales floats the keyboard's own country to the top: "be" is the
// Belgian layout and the Belarusian language, and be_BY sorted ahead of
// fr_BE, which handed Belgians a Cyrillic locale.
func promoteKbLocales(items []item, keymap string) []item {
	lay, _ := xkbFromKeymap(keymap)
	terr := strings.ToUpper(lay)
	if terr == "" || terr == "US" {
		return items // us is already the promoted default; nothing to disambiguate
	}
	var prefer []string
	for _, it := range items {
		if localeTerritory(it.key) == terr {
			prefer = append(prefer, it.key)
		}
	}
	return promote(items, prefer)
}

// sysTimezones lists time zones, with the auto-detect entry first.
func sysTimezones() []item {
	out, ok := run("timedatectl", "list-timezones")
	if !ok {
		return nil
	}
	items := []item{{"auto", i18n.T("Detect automatically"), i18n.T("via IP, also sets the clock")}}
	for _, l := range strings.Split(out, "\n") {
		c := strings.TrimSpace(l)
		if c != "" {
			items = append(items, item{c, c, ""})
		}
	}
	return items
}

// lsblkPairs parses `lsblk -P` key="value" lines into maps.
func lsblkPairs(fields string) []map[string]string {
	out, ok := run("lsblk", "-dpno", fields, "-P")
	if !ok {
		return nil
	}
	var rows []map[string]string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := map[string]string{}
		for _, tok := range splitPairs(line) {
			eq := strings.IndexByte(tok, '=')
			if eq < 0 {
				continue
			}
			k := tok[:eq]
			v := unescapeLsblk(strings.Trim(tok[eq+1:], "\""))
			m[k] = v
		}
		rows = append(rows, m)
	}
	return rows
}

// splitPairs splits a lsblk -P line into KEY="value" tokens (values may hold spaces).
func splitPairs(line string) []string {
	var toks []string
	var b strings.Builder
	inQ := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '"' {
			inQ = !inQ
			b.WriteByte(c)
		} else if c == ' ' && !inQ {
			if b.Len() > 0 {
				toks = append(toks, b.String())
				b.Reset()
			}
		} else {
			b.WriteByte(c)
		}
	}
	if b.Len() > 0 {
		toks = append(toks, b.String())
	}
	return toks
}

// unescapeLsblk decodes the \xNN hex escapes lsblk -P emits for spaces and other
// special bytes in values (e.g. PARTLABEL="Windows\x20Data"), so a partition
// label displays and matches on its real text instead of the literal escape.
func unescapeLsblk(s string) string {
	if !strings.Contains(s, `\x`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) && s[i+1] == 'x' {
			if n, err := strconv.ParseUint(s[i+2:i+4], 16, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// firstField returns the first whitespace- or newline-delimited token of s, so
// a multi-line lsblk/findmnt reply collapses to the one value we asked for.
func firstField(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return ""
}

// liveDisk resolves the whole disk backing the live ISO boot medium so sysDisks
// can hide it (never offer to erase the stick we booted from). It reads the
// archiso boot mount's source via findmnt, then resolves the physical disk: a
// partition's parent (lsblk PKNAME), or for a layered medium (Ventoy maps the
// ISO through a device-mapper node) the disk at the bottom of the inverse device
// tree. Off-ISO there is no /run/archiso, so it returns "" and nothing filters.
func liveDisk() string {
	if _, err := os.Stat("/run/archiso"); err != nil {
		return ""
	}
	src, ok := run("findmnt", "-nro", "SOURCE", "/run/archiso/bootmnt")
	if !ok {
		return ""
	}
	if src = firstField(src); src == "" {
		return ""
	}
	if pk, ok := run("lsblk", "-no", "PKNAME", src); ok {
		if pk = firstField(pk); pk != "" {
			return "/dev/" + pk // PKNAME is a bare kernel name (e.g. "sda")
		}
	}
	// src has no partition parent: a layered boot medium. Ventoy maps the ISO
	// through a device-mapper node (some tools use a loop), so the physical stick
	// sits a step lower. Walk the inverse device tree to the disk it is built on,
	// so the medium we booted from is still hidden from the picker.
	if tree, ok := run("lsblk", "-snpo", "NAME,TYPE", src); ok {
		if d := bottomDisk(tree); d != "" {
			return d
		}
	}
	return src // no resolvable parent: treat the source as the whole disk
}

// bottomDisk returns the deepest TYPE=disk device in `lsblk -s` (inverse
// dependency) output: the physical whole-disk a layered boot medium (a Ventoy
// device-mapper node, or a loop) is ultimately built on. "" when none is listed.
func bottomDisk(lsblkTree string) string {
	var disk string
	for _, line := range strings.Split(lsblkTree, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[1] == "disk" {
			disk = f[0]
		}
	}
	return disk
}

// excludeDisk reports whether a whole-disk device must be hidden from the
// installer picker. It drops zero-size and pseudo/removable devices, the eMMC
// boot0/boot1/rpmb hardware areas that lsblk surfaces as separate disks, and the
// live ISO medium (live, "" off-ISO). Pure so the filter can be unit-tested.
func excludeDisk(name, size, live string) bool {
	if name == "" || size == "" || size == "0B" {
		return true
	}
	for _, frag := range []string{"zram", "/dev/sr", "/dev/nbd", "loop"} {
		if strings.Contains(name, frag) {
			return true
		}
	}
	if strings.Contains(name, "mmcblk") &&
		(strings.HasSuffix(name, "boot0") || strings.HasSuffix(name, "boot1") || strings.HasSuffix(name, "rpmb")) {
		return true
	}
	return live != "" && name == live
}

// sysDisks lists installable whole disks, excluding pseudo devices, eMMC boot
// areas, and the live ISO boot medium. WIRE target.
func sysDisks() []item {
	rows := lsblkPairs("NAME,SIZE,MODEL,TRAN,ROTA,TYPE")
	live := liveDisk()
	var items []item
	for _, r := range rows {
		if r["TYPE"] != "disk" || excludeDisk(r["NAME"], r["SIZE"], live) {
			continue
		}
		kind := "SSD"
		if r["ROTA"] == "1" {
			kind = "HDD"
		}
		tran := strings.ToUpper(r["TRAN"])
		model := strings.TrimSpace(r["MODEL"])
		// Lead the row with what's actually on the disk (from the probe) so the
		// content summary survives truncation; the size/model/bus follow.
		sum := diskSummary(sysDiskLayout(r["NAME"]))
		hint := strings.TrimSpace(fmt.Sprintf("%s · %s · %s · %s %s", sum, humanSize(sysDiskBytes(r["NAME"])), model, tran, kind))
		items = append(items, item{r["NAME"], r["NAME"], hint})
	}
	return items
}

// diskHint explains an empty disk list on the target-disk step. Intel VMD (RST)
// hides NVMe behind a controller the live kernel can't see without the vmd
// module, so the fix is a firmware setting; anything else gets a generic hint.
func diskHint() string {
	if hasVMD() {
		return i18n.T("No disks found. This machine has Intel VMD (RST) enabled -- enable AHCI / disable VMD (Intel RST) in BIOS setup, then reboot the installer. dual-boot note: Windows installed under RST will not boot after switching; see docs/installation-hardware.md.")
	}
	return i18n.T("No disks found. Check that a drive is connected and detected in firmware, then reboot the installer.")
}

// hasVMD reports whether an Intel Volume Management Device controller is present
// (vendor 8086, controller name contains "Volume Management Device").
func hasVMD() bool {
	out, ok := run("sh", "-c", "lspci -nn")
	if !ok {
		return false
	}
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "Volume Management Device") &&
			(strings.Contains(l, "8086") || strings.Contains(strings.ToLower(l), "intel")) {
			return true
		}
	}
	return false
}

// sysDiskBytes returns a device's exact size in bytes via blockdev. WIRE target.
func sysDiskBytes(dev string) int64 {
	out, ok := run("blockdev", "--getsize64", dev)
	if !ok {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// sysDiskSize returns a device size in whole GiB (for the layout math).
func sysDiskSize(dev string) int { return int(sysDiskBytes(dev) / (1024 * 1024 * 1024)) }

// espTypeGUID is the GPT partition type for an EFI System Partition.
const espTypeGUID = "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"

// diskLayout is a disk's real partition layout: the existing partitions (kept for
// a dual-boot install) and the largest contiguous free region. Partitions come
// from lsblk; the free region comes from the backend's read-only alongside probe
// (the single source of truth for free space), so the TUI never guesses.
type diskLayout struct {
	parts        []part
	freeG        int
	regionStart  int64  // chosen free region, first sector (alongside; 0 when none)
	regionEnd    int64  // chosen free region, last sector
	probeVerdict string // ok|none|no-gpt|no-esp|error from the alongside probe
	probeMessage string // human cause for a non-ok verdict (rendered as the block reason)
	windows      bool   // an NTFS partition is present (a Windows install)
	gpt          bool   // GPT label (alongside requires it)
	bitlocker    bool   // a BitLocker-encrypted partition is present (recovery-key warning)
	espKind      string // windows|ryoku|linux for the disk's EF00 ESP ("" when none/older backend)
	existingBoot string // the existing OS's chainloadable EFI binary, or "none" ("" when absent)
	leftovers    []part // verified failed-install debris the backend reclaims (freed space)
	espCount     int    // EF00 ESPs on the disk (>1 surfaces a multi-ESP review note)
	espFreeKiB   int64  // free KiB on the existing ESP; below 8192 selects a dedicated Ryoku ESP
}

// sysDiskLayout reads the existing partitions and largest free region of a disk.
func sysDiskLayout(disk string) diskLayout {
	var dl diskLayout
	if pt, ok := run("blkid", "-o", "value", "-s", "PTTYPE", disk); ok {
		dl.gpt = strings.TrimSpace(pt) == "gpt"
	}
	if out, ok := run("lsblk", "-pnbo", "NAME,TYPE,SIZE,FSTYPE,PARTTYPE,PARTLABEL", "-P", disk); ok {
		dl.parts, dl.windows, dl.bitlocker = parseDiskParts(out)
	}
	pr := probeAlongside(disk)
	dl.freeG, dl.regionStart, dl.regionEnd = pr.freeG, pr.regionStart, pr.regionEnd
	dl.probeVerdict, dl.probeMessage = pr.verdict, pr.message
	dl.espKind, dl.existingBoot, dl.leftovers = pr.espKind, pr.existingBoot, pr.leftovers
	dl.espCount, dl.espFreeKiB = pr.espCount, pr.espFreeKiB
	return dl
}

// parseDiskParts parses `lsblk -pnbo NAME,TYPE,SIZE,FSTYPE,PARTTYPE,PARTLABEL -P`
// into a disk's partition list. Every `part` row is listed regardless of table
// type or filesystem, so an MBR disk or an unformatted partition never reads as
// blank. windows/bitlocker report a present NTFS / locked-NTFS volume.
func parseDiskParts(out string) (parts []part, windows, bitlocker bool) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		r := map[string]string{}
		for _, tok := range splitPairs(line) {
			if eq := strings.IndexByte(tok, '='); eq >= 0 {
				r[tok[:eq]] = unescapeLsblk(strings.Trim(tok[eq+1:], "\""))
			}
		}
		if r["TYPE"] != "part" {
			continue
		}
		sizeB, _ := strconv.ParseInt(r["SIZE"], 10, 64)
		gib := int((sizeB + (1 << 29)) / (1 << 30)) // round to nearest GiB
		fs := strings.ToLower(r["FSTYPE"])
		if fs == "bitlocker" {
			bitlocker = true // locked NTFS: booting Windows via Ryoku will demand the recovery key
		}
		p := part{size: gib, fs: fs, mount: "-", flags: "-", status: "keep"}
		switch {
		case strings.EqualFold(r["PARTTYPE"], espTypeGUID):
			p.dev, p.fs, p.mount, p.flags = "EFI System", "fat32", "-", "esp"
		case fs == "ntfs":
			p.dev, p.mount = winLabel(r["PARTLABEL"]), "Windows"
			windows = true
		default:
			p.dev = partLabel(r["PARTLABEL"], fs)
		}
		parts = append(parts, p)
	}
	return parts, windows, bitlocker
}

func winLabel(lbl string) string {
	if lbl = strings.TrimSpace(lbl); lbl != "" {
		return "Windows (" + lbl + ")"
	}
	return "Windows (NTFS)"
}

func partLabel(lbl, fs string) string {
	if lbl = strings.TrimSpace(lbl); lbl != "" {
		return lbl
	}
	if fs != "" {
		return strings.ToUpper(fs)
	}
	return i18n.T("partition")
}

// probeResult is the backend alongside probe's report: the largest usable free
// region (GiB + first/last sectors), plus the verdict and its human message so
// the exact cause (no ESP, no GPT, no region, probe failure) reaches the user
// instead of a generic "not enough space". The backend (sfdisk) is the single
// source of truth for free space; the TUI renders what it says and hands the
type probeResult struct {
	freeG                  int
	regionStart, regionEnd int64
	verdict, message       string
	espKind                string // esp_kind: windows|ryoku|linux ("" when no ESP or older backend)
	existingBoot           string // existing_boot: the existing OS's EFI binary, or "none" ("" when absent)
	espCount               int    // esp_count: number of EF00 ESPs on the disk (0 when older backend)
	espFreeKiB             int64  // esp_free_kib: free space on the existing ESP
	leftovers              []part // one per verified failed-install partition to reclaim (freed)
}

func probeAlongside(disk string) probeResult {
	bin := os.Getenv("RYOKU_BACKEND")
	if bin == "" {
		bin = "ryoku-install"
	}
	out, ok := run(bin, "probe", "alongside", disk)
	if !ok {
		return probeResult{verdict: "error", message: i18n.T("could not run the disk probe (ryoku-install probe alongside)."), espFreeKiB: -1}
	}
	r := probeResult{espFreeKiB: -1}
	var bestMiB int64
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch {
		case len(f) == 4 && f[0] == "region":
			if m := parseI64(f[3]); m > bestMiB {
				bestMiB, r.regionStart, r.regionEnd = m, parseI64(f[1]), parseI64(f[2])
			}
		case len(f) >= 2 && f[0] == "esp_kind":
			r.espKind = f[1]
		case len(f) >= 2 && f[0] == "existing_boot":
			r.existingBoot = f[1]
		case len(f) >= 2 && f[0] == "esp_count":
			r.espCount = int(parseI64(f[1]))
		case len(f) >= 2 && f[0] == "esp_free_kib":
			r.espFreeKiB = parseI64(f[1])
		case len(f) == 4 && f[0] == "leftover":
			// leftover <dev> <partlabel> <sizeMiB>: verified debris the backend frees.
			r.leftovers = append(r.leftovers, part{
				dev: i18n.T("previous Ryoku"), fs: f[2], size: gibRound(parseI64(f[3])),
				reclaim: true, status: "reclaim",
			})
		case len(f) >= 2 && f[0] == "verdict":
			r.verdict = f[1]
		case f[0] == "message":
			r.message = strings.TrimSpace(strings.TrimPrefix(line, "message"))
		}
	}
	r.freeG = int(bestMiB / 1024)
	return r
}

func parseI64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// resizePart is one `part` line of `ryoku-install probe resize <disk>`: a
// partition and whether the carve flow may shrink it, with the exact size/used/
// min figures the bounds math needs and, when it can't be carved, the reason to
// show the user. Field order is frozen in .superpowers/sdd/resize-probe-format.txt.
type resizePart struct {
	dev        string // /dev/sdaN — handed back verbatim as RYOKU_RESIZE_PART
	index      int    // partition number (the backend's sfdisk -N target)
	fs         string
	label      string // "" when the probe reported "-"; spaces come as underscores
	sizeMiB    int64
	usedMiB    int64
	minMiB     int64 // smallest the fs can shrink to (used + safety margin)
	shrinkable bool
	reason     string // why not, when !shrinkable ("BitLocker: decrypt in Windows first")
}

// name is the short human label for a partition in the carve UI: its label, or
// "Windows" for a plain NTFS volume, else the filesystem in caps.
func (p resizePart) name() string {
	if p.label != "" {
		return p.label
	}
	switch p.fs {
	case "ntfs":
		return "Windows"
	case "":
		return i18n.T("partition")
	default:
		return strings.ToUpper(p.fs)
	}
}

// parseResizeParts pulls the `part` lines out of a resize-probe report:
//
//	part <dev> <index> <fstype> <label> <sizeMiB> <usedMiB> <minMiB> <yes|no> <reason...>
//
// The first nine fields are positional (label is a single whitespace-free token,
// "-" = none); the reason is the free-text remainder. A malformed line — too few
// fields or a non-numeric size — is skipped rather than trusted, so a garbled
// probe fails closed to "nothing shrinkable" instead of offering a bad carve.
func parseResizeParts(out string) []resizePart {
	var parts []resizePart
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 9 || f[0] != "part" {
			continue
		}
		idx, e1 := strconv.Atoi(f[2])
		size, e2 := strconv.ParseInt(f[5], 10, 64)
		used, e3 := strconv.ParseInt(f[6], 10, 64)
		min, e4 := strconv.ParseInt(f[7], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
			continue
		}
		lbl := f[4]
		if lbl == "-" {
			lbl = ""
		}
		parts = append(parts, resizePart{
			dev: f[1], index: idx, fs: strings.ToLower(f[3]), label: lbl,
			sizeMiB: size, usedMiB: used, minMiB: min,
			shrinkable: f[8] == "yes", reason: strings.Join(f[9:], " "),
		})
	}
	return parts
}

// probeResize runs the backend's read-only resize probe and returns its
// per-partition shrinkability report. A probe that will not run (missing verb on
// an older backend, error) yields no candidates, so carve simply stays unoffered.
func probeResize(disk string) []resizePart {
	bin := os.Getenv("RYOKU_BACKEND")
	if bin == "" {
		bin = "ryoku-install"
	}
	out, ok := run(bin, "probe", "resize", disk)
	if !ok {
		return nil
	}
	return parseResizeParts(out)
}

// diskSummary is the one-line "what's on this disk" the target picker shows: the
// headline occupant, how many other partitions there are, and the free headroom,
// so a blank spare disk reads differently from the full 1 TB Windows drive
// without opening it. e.g. "Windows + 3 more · 190 GiB free", "ryoku · full".
func diskSummary(dl diskLayout) string {
	if len(dl.parts) == 0 {
		return i18n.T("empty")
	}
	head := diskPrimary(dl)
	if more := len(dl.parts) - 1; more > 0 {
		head += i18n.Tf(" + %d more", more)
	}
	free := i18n.T("full")
	if dl.freeG > 0 {
		free = i18n.Tf("%d GiB free", dl.freeG)
	}
	return head + " · " + free
}

// diskPrimary names the headline occupant: Windows when NTFS is present, a
// previous Ryoku when the probe flagged failed-install debris, otherwise the
// biggest partition's label.
func diskPrimary(dl diskLayout) string {
	if dl.windows {
		return "Windows"
	}
	if len(dl.leftovers) > 0 {
		return "ryoku"
	}
	big := dl.parts[0]
	for _, p := range dl.parts[1:] {
		if p.size > big.size {
			big = p
		}
	}
	return big.dev
}

// sysSSIDs lists the cached nearby Wi-Fi networks via nmcli. It uses --rescan no
// so it never blocks the UI on a scan; NetworkManager refreshes the cache on its
// own, and the picker's r key relists it. WIRE target.
func sysSSIDs() []item {
	out, ok := run("nmcli", "-t", "-f", "SSID,SIGNAL,SECURITY", "dev", "wifi", "list", "--rescan", "no")
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var items []item
	for _, line := range strings.Split(out, "\n") {
		f := splitNMTerse(line) // nmcli terse escapes ':' and '\' inside SSIDs
		if len(f) < 3 || f[0] == "" || seen[f[0]] {
			continue
		}
		seen[f[0]] = true
		sec := f[2]
		if sec == "" {
			sec = "open"
		}
		items = append(items, item{f[0], f[0], bars(f[1]) + " · " + sec})
	}
	return items
}

// splitNMTerse splits one nmcli -t line on unescaped ':' field separators and
// unescapes the values in a single pass. nmcli's terse output backslash-escapes a
// literal ':' as "\:" and a literal '\' as "\\" inside a value, so a naive split
// on ':' mangles any SSID that contains either. Pure so it can be tested.
func splitNMTerse(line string) []string {
	var fields []string
	var b strings.Builder
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case c == '\\' && i+1 < len(line): // keep the escaped byte literally (\: -> :, \\ -> \)
			b.WriteByte(line[i+1])
			i++
		case c == ':':
			fields = append(fields, b.String())
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}
	return append(fields, b.String())
}

// bars renders a 0-100 signal value as a four-cell bar graph.
func bars(sig string) string {
	n, _ := strconv.Atoi(strings.TrimSpace(sig))
	full := n / 25
	if full > 4 {
		full = 4
	}
	return strings.Repeat("▆", full) + strings.Repeat("_", 4-full)
}

// hwInfo is the detected-hardware summary shown on the hardware card.
type hwInfo struct {
	cpu, gpu, mem, fw, disk, profile string
	hybrid, ok, bios, secureBoot     bool
}

var (
	hwCache hwInfo
	hwOnce  sync.Once
)

func ensureHW() hwInfo { hwOnce.Do(func() { hwCache = detectHardware() }); return hwCache }

// detectHardware probes CPU, GPU(s), memory, firmware, and disk, then suggests a
// profile and whether the machine is a hybrid iGPU+dGPU laptop. WIRE target.
func detectHardware() hwInfo {
	h := hwInfo{ok: true, profile: "vm"}

	if out, ok := run("lscpu"); ok {
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "Model name:") {
				h.cpu = strings.TrimSpace(strings.TrimPrefix(l, "Model name:"))
				break
			}
		}
	}

	virt, _ := run("systemd-detect-virt")
	isVM := virt != "" && virt != "none"

	gpuLines := []string{}
	hasNvidia, hasAMD, hasIntel := false, false, false
	if out, ok := run("sh", "-c", "lspci | grep -E 'VGA compatible controller|3D controller|Display controller'"); ok {
		for _, l := range strings.Split(out, "\n") {
			if strings.TrimSpace(l) == "" {
				continue
			}
			gpuLines = append(gpuLines, l)
			ll := strings.ToLower(l)
			switch {
			case strings.Contains(ll, "nvidia"):
				hasNvidia = true
			case strings.Contains(ll, "amd") || strings.Contains(ll, "ati") || strings.Contains(ll, "radeon"):
				hasAMD = true
			case strings.Contains(ll, "intel"):
				hasIntel = true
			}
		}
	}
	h.gpu = summarizeGPU(gpuLines)
	h.hybrid = hasNvidia && (hasAMD || hasIntel)

	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(l, "MemTotal:") {
				if f := strings.Fields(l); len(f) >= 2 {
					if kb, err := strconv.Atoi(f[1]); err == nil {
						h.mem = fmt.Sprintf("%d GiB", (kb+512*1024)/(1024*1024))
					}
				}
				break
			}
		}
	}

	if _, err := os.Stat("/sys/firmware/efi"); err == nil {
		h.fw = "UEFI"
	} else {
		h.fw, h.bios = "BIOS", true // backend is UEFI-only; the TUI hard-blocks BIOS boot
	}
	if isVM {
		h.fw += i18n.T(" · virtual machine")
	} else {
		h.fw += i18n.T(" · bare metal")
	}
	h.secureBoot = secureBootEnabled() // Limine is unsigned; blocks Review when on

	if rows := lsblkPairs("NAME,SIZE,MODEL,ROTA,TYPE"); len(rows) > 0 {
		live := liveDisk()
		for _, r := range rows {
			if r["TYPE"] != "disk" || excludeDisk(r["NAME"], r["SIZE"], live) {
				continue
			}
			kind := "SSD"
			if r["ROTA"] == "1" {
				kind = "HDD"
			}
			h.disk = strings.TrimSpace(fmt.Sprintf("%s · %s · %s (%s)", r["NAME"], r["SIZE"], r["MODEL"], kind))
			break
		}
	}

	h.profile = suggestProfile(isVM, hasNvidia, hasAMD, hasIntel)
	// Unclassifiable: not a VM, no vendor GPU matched, and no GPU line at all. The
	// picker still lets the user choose; ok=false shows the fallback card copy.
	if !isVM && !hasNvidia && !hasAMD && !hasIntel && len(gpuLines) == 0 {
		h.ok = false
	}
	return h
}

// suggestProfile maps detected traits to a hardware profile. A VM always maps to
// vm; otherwise NVIDIA wins over the CPU vendor (the dGPU needs the proprietary
// stack), then AMD, then Intel; with nothing classifiable we fall back to vm.
func suggestProfile(isVM, hasNvidia, hasAMD, hasIntel bool) string {
	switch {
	case isVM:
		return "vm"
	case hasNvidia:
		return "amd-nvidia"
	case hasAMD:
		return "amd"
	case hasIntel:
		return "intel"
	default:
		return "vm"
	}
}

// secureBootEnabled reports whether UEFI Secure Boot is active. The SecureBoot
// efivar is <attrs:4 bytes><value:1 byte>; the value byte is 1 when enabled. A
// missing variable (BIOS boot, or efivars not mounted) reads as off -- nothing to
// block. Limine is unsigned, so active Secure Boot must stop Review.
func secureBootEnabled() bool {
	data, err := os.ReadFile("/sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c")
	if err != nil || len(data) == 0 {
		return false
	}
	return data[len(data)-1] == 1
}

func summarizeGPU(lines []string) string {
	var names []string
	for _, l := range lines {
		// Prefer the bracketed marketing name, e.g. "[GeForce RTX 4060]".
		if a := strings.LastIndex(l, "["); a >= 0 {
			if b := strings.Index(l[a:], "]"); b > 0 {
				names = append(names, strings.TrimSpace(l[a+1:a+b]))
				continue
			}
		}
		if i := strings.Index(l, ": "); i >= 0 {
			rest := l[i+2:]
			if j := strings.Index(rest, ": "); j >= 0 {
				rest = rest[j+2:]
			}
			names = append(names, strings.TrimSpace(rest))
		}
	}
	if len(names) == 0 {
		return i18n.T("unclassified")
	}
	return strings.Join(names, " + ")
}

// applyKeymap loads the chosen console keymap so the rest of the wizard is typed
// in that layout. Best effort. WIRE target.
func applyKeymap(code string) { _ = exec.Command("loadkeys", code).Run() }

// validXkbLayout reports whether l is a real X11/XKB layout, from
// `localectl list-x11-keymap-layouts` (probed once, cached). A bogus layout would
// make the graphical session fail to start, so xkbFromKeymap falls back on this.
// When localectl is unavailable the check trusts the input rather than forcing
// "us".
var (
	xkbLayouts map[string]bool
	xkbProbed  bool

	xkbVariants       = map[string]map[string]bool{}
	xkbVariantsProbed = map[string]bool{}
)

func validXkbLayout(l string) bool {
	if !xkbProbed {
		xkbProbed = true
		if out, ok := run("localectl", "list-x11-keymap-layouts"); ok {
			xkbLayouts = map[string]bool{}
			for _, line := range strings.Split(out, "\n") {
				if s := strings.TrimSpace(line); s != "" {
					xkbLayouts[s] = true
				}
			}
		}
	}
	if xkbLayouts == nil {
		return true
	}
	return xkbLayouts[l]
}

// validXkbVariant is the variant counterpart to validXkbLayout. Results are
// cached per layout because localectl needs a separate query for each one. A
// failed probe does not accept the suffix: many console suffixes, such as
// "latin1", are not XKB variants.
func validXkbVariant(layout, variant string) bool {
	if variant == "" {
		return false
	}
	if !xkbVariantsProbed[layout] {
		xkbVariantsProbed[layout] = true
		if out, ok := run("localectl", "list-x11-keymap-variants", layout); ok {
			variants := map[string]bool{}
			for _, line := range strings.Split(out, "\n") {
				if s := strings.TrimSpace(line); s != "" {
					variants[s] = true
				}
			}
			xkbVariants[layout] = variants
		}
	}
	variants, ok := xkbVariants[layout]
	if !ok {
		return false
	}
	return variants[variant]
}

// xkbFromKeymap maps a console keymap (what the picker offers, from
// `localectl list-keymaps`) to the XKB layout and variant the graphical stack
// needs. Console and XKB names coincide for most layouts; aliases and console
// naming conventions need translating. Anything that is not a real XKB layout
// falls back to "us" so a pick can never break the graphical session.
func xkbFromKeymap(code string) (layout, variant string) {
	if code == "" {
		return "us", ""
	}
	switch code {
	case "dvorak", "ANSI-dvorak":
		return "us", "dvorak"
	case "colemak":
		return "us", "colemak"
	case "dvorak-programmer":
		return "us", "dvp"
	case "dvorak-l":
		return "us", "dvorak-l"
	case "dvorak-r":
		return "us", "dvorak-r"
	case "dvorak-uk":
		return "gb", "dvorak"
	case "fr-bepo":
		return "fr", "bepo"
	case "fr-bepo-latin9":
		return "fr", "bepo_latin9"
	}

	base, suffix := code, ""
	if i := strings.IndexByte(code, '-'); i > 0 {
		base, suffix = code[:i], code[i+1:]
	}
	switch base {
	case "uk":
		base = "gb"
	case "trq", "trf":
		base = "tr"
	}
	if validXkbLayout(base) {
		if validXkbVariant(base, suffix) {
			return base, suffix
		}
		return base, ""
	}
	if validXkbLayout(code) {
		return code, ""
	}
	return "us", ""
}

// keymapRelaunch handles the one thing loadkeys cannot: the graphical wizard runs
// in cage (Wayland), whose keyboard layout is fixed at launch, so a password typed
// after the keyboard pick would still be captured in cage's launch layout (us) and
// then fail at a login prompt on the user's real layout. When the picked layout
// or variant differs from cage's active pair, write it for the installer session
// and return true so the caller quits; the session relaunches cage under the pair
// and the wizard resumes past the keyboard step (RYOKU_KB_PRESET). Console path
// (no cage): loadkeys already applies to the VT, so this is a no-op.
func keymapRelaunch(code string) bool {
	if os.Getenv("RYOKU_SESSION") != "graphical" {
		return false
	}
	lay, varnt := xkbFromKeymap(code)
	activeLayout := os.Getenv("RYOKU_XKB")
	if activeLayout == "" {
		activeLayout = "us" // cage's default when the session set nothing
	}
	if lay == activeLayout && varnt == os.Getenv("XKB_DEFAULT_VARIANT") {
		return false
	}
	_ = os.WriteFile("/tmp/ryoku-xkb", []byte(code+"\n"+lay+"\n"+varnt+"\n"), 0o644)
	return true
}

// applyExit runs the post-install action chosen on the done screen. reboot and
// poweroff hand off to systemd; anything else just returns so the live session
// ends and drops to a shell.
func applyExit(action string) {
	switch action {
	case "reboot":
		_ = exec.Command("systemctl", "reboot").Run()
	case "poweroff":
		_ = exec.Command("systemctl", "poweroff").Run()
	}
}

// wifiBackendConf is NetworkManager's Wi-Fi backend drop-in in the live session.
// var (not const) so a test can point it at a temp file. The installer carries
// whatever this ends up set to into the target (backend/lib/network.sh), so the
// installed machine's first boot uses the backend that actually associated here.
var wifiBackendConf = "/etc/NetworkManager/conf.d/wifi-backend.conf"

var wpaBackendRe = regexp.MustCompile(`(?mi)^\s*wifi\.backend\s*=\s*wpa_supplicant`)

// activeWifiBackend reports the live session's NM Wi-Fi backend. Anything that is
// not an explicit wpa_supplicant pin reads as iwd, the Ryoku/ISO default.
func activeWifiBackend() string {
	if b, err := os.ReadFile(wifiBackendConf); err == nil && wpaBackendRe.Match(b) {
		return "wpa_supplicant"
	}
	return "iwd"
}

// otherWifiBackend is the backend we have not tried yet.
func otherWifiBackend(cur string) string {
	if cur == "wpa_supplicant" {
		return "iwd"
	}
	return "wpa_supplicant"
}

// switchWifiBackend repoints NetworkManager at backend and restarts it (root, in
// the live session), then waits, bounded, for the Wi-Fi device to come back.
// iwd cannot associate with WPA3-SAE networks (many Android phone hotspots), so
// the connect path falls back through here. It only runs after the current
// backend already failed, so it can only help.
func switchWifiBackend(backend string) bool {
	body := "# Set by the Ryoku installer: the Wi-Fi backend that associated in the\n" +
		"# live session, carried into the target so first boot stays online.\n" +
		"[device]\nwifi.backend=" + backend + "\n"
	if err := os.WriteFile(wifiBackendConf, []byte(body), 0o644); err != nil {
		return false
	}
	leaving := "wpa_supplicant.service"
	if backend == "wpa_supplicant" {
		leaving = "iwd.service"
	}
	_ = exec.Command("systemctl", "stop", leaving).Run()
	if err := exec.Command("systemctl", "restart", "NetworkManager.service").Run(); err != nil {
		return false
	}
	for range 20 {
		if out, ok := run("nmcli", "-t", "-f", "TYPE,STATE", "dev"); ok && strings.Contains(out, "wifi:") {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	return true
}

// wifiConnect joins a network with nmcli. If the live backend (iwd by default)
// cannot associate -- WPA3-SAE networks and many phone hotspots, which iwd does
// not yet support -- it switches to the other backend and retries once, so more
// networks work during install. Whichever backend succeeds is left in place for
// the installer to carry into the target.
func wifiConnect(ssid, pass string) bool {
	if nmWifiConnect(ssid, pass) {
		return true
	}
	return switchWifiBackend(otherWifiBackend(activeWifiBackend())) && nmWifiConnect(ssid, pass)
}

func nmWifiConnect(ssid, pass string) bool {
	args := []string{"dev", "wifi", "connect", ssid}
	if pass != "" {
		args = append(args, "password", pass)
	}
	return exec.Command("nmcli", args...).Run() == nil
}

// ───────────────────────── install handoff ─────────────────────────

// installEnv builds the RYOKU_* environment the backend reads (install-contract.md).
func (m model) installEnv() []string {
	b := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	xkbLay, xkbVar := xkbFromKeymap(m.picks["keyboard"])
	env := []string{
		"RYOKU_DISK=" + m.diskDev,
		// No default for the disk strategy: an empty value MUST reach the backend so
		// it can fail closed. Defaulting to "whole" here was a silent wipe path
		// (a missing/cleared pick would auto-erase the disk).
		"RYOKU_DISK_STRATEGY=" + m.picks["disk"],
		"RYOKU_HOSTNAME=" + def(m.picks["hostname"], "ryoku"),
		"RYOKU_USERNAME=" + def(m.picks["username"], "ryoku"),
		"RYOKU_PASSWORD_HASH=" + m.pwHash,
		"RYOKU_KEYMAP=" + def(m.picks["keyboard"], "us"),
		"RYOKU_XKB_LAYOUT=" + xkbLay,
		"RYOKU_XKB_VARIANT=" + xkbVar,
		"RYOKU_LOCALE=" + def(m.picks["locale"], "en_US.UTF-8"),
		// The backend's shell reads RYOKU_LANG (lib/i18n.sh) so its progress log
		// speaks the language the user picked in the TUI.
		"RYOKU_LANG=" + i18n.Lang(),
		"RYOKU_TIMEZONE=" + def(m.picks["timezone"], "UTC"),
		"RYOKU_PROFILE=" + def(m.picks["profile"], "vm"),
		"RYOKU_ESP_GIB=" + strconv.Itoa(m.espG),
		"RYOKU_SWAP_GIB=" + strconv.Itoa(m.swapG),
		"RYOKU_ESP_MODE=" + m.espMode(),
		"RYOKU_SUBVOL_SNAPSHOTS=" + b(m.snapshots),
		"RYOKU_SUBVOL_HOME=" + b(m.sepHome),
		"RYOKU_SUBVOL_BACKUPS=" + b(m.backups),
		// package source (RYOKU_ONLINE / RYOKU_OFFLINE_REPO) is appended after the
		// literal, since it depends on whether this ISO baked an offline repo.
	}
	if offlineRepo() {
		// offline ISO: the whole closure is baked into a file:// repo and the
		// backend pacstraps from it with no network (see lib/offline.sh).
		env = append(env, "RYOKU_ONLINE=0", "RYOKU_OFFLINE_REPO="+offlineRepoPath)
	} else {
		env = append(env, "RYOKU_ONLINE=1")
	}
	// backend picks the ryoku-desktop-<name> variant and seeds ConfigDir(name);
	// an unknown name yields an empty dir the backend refuses on. The GPU mode
	// write goes to the provider's render-pin file, asked of the seam: a
	// provider with no ryoku-gpu writer (niri picks its own device) exports an
	// empty path and the backend skips the step.
	comp := m.picks["compositor"]
	env = append(env, "RYOKU_COMPOSITOR="+comp, "RYOKU_COMPOSITOR_CONFIG_DIR="+wm.ConfigDir(comp),
		"RYOKU_COMPOSITOR_GPU_PIN="+wm.GpuPinFile(comp))
	// Product choices reach the backend as one drop list: every package removed
	// in Apps & tools, the two browsers that lost, and the two unchosen shell
	// stacks. The backend filters this list from every package source and records
	// it in the provisioning ledger.
	br := def(m.picks["browser"], "firefox")
	brPkg := br
	if br == "zen" {
		brPkg = "zen-browser-bin"
	}
	sh := def(m.picks["login-shell"], "fish")
	var drop []string
	drop = append(drop, deselectedPkgs(m.selectedApps())...)
	for _, pkg := range browserPackages() {
		if pkg != brPkg {
			drop = append(drop, pkg)
		}
	}
	drop = append(drop, loginShellDropPackages(sh)...)
	env = append(env, "RYOKU_BROWSER="+br, "RYOKU_LOGIN_SHELL="+sh)
	if len(drop) > 0 {
		env = append(env, "RYOKU_DROP_PACKAGES="+strings.Join(drop, ","))
	}
	if m.picks["gpu"] != "" {
		env = append(env, "RYOKU_GPU_MODE="+m.picks["gpu"])
	}
	if m.picks["encryption"] == "LUKS" {
		env = append(env, "RYOKU_ENCRYPT=1", "RYOKU_LUKS_PASSPHRASE="+m.luksPass)
	}
	// Alongside hands the backend either a pre-existing gap or a carve request,
	// never both. Carve exports only the partition + take; the backend shrinks it,
	// then re-probes the freed gap and drives the same region math from there — so
	// the region sectors are computed downstream, not here. Absent RESIZE vars =
	// no carve.
	if m.picks["disk"] == "alongside" {
		switch {
		case m.carving():
			p := m.resizeParts[m.carvePart]
			env = append(env,
				"RYOKU_RESIZE_PART="+p.dev,
				"RYOKU_RESIZE_TAKE_MIB="+strconv.FormatInt(m.carveTakeMiB, 10))
		case m.regionEnd > 0:
			env = append(env,
				"RYOKU_REGION_START="+strconv.FormatInt(m.regionStart, 10),
				"RYOKU_REGION_END="+strconv.FormatInt(m.regionEnd, 10))
		}
	}
	// The typed "ERASE" acknowledgement (wipeStage == 2) authorizes a destructive
	// step, but which one depends on strategy: a whole-disk wipe needs
	// RYOKU_WIPE_CONFIRMED; an alongside install that must free leftover ryoku/
	// ryokuboot partitions needs RYOKU_RECLAIM_LEFTOVERS (the backend otherwise
	// dies listing them). Never emit both -- each backend path demands only its own.
	if m.wipeStage == 2 {
		switch m.picks["disk"] {
		case "whole":
			env = append(env, "RYOKU_WIPE_CONFIRMED=1")
		case "alongside":
			// Also on a retry: a failed attempt leaves its own half-made ryoku/
			// ryokuboot partitions behind, and the pre-failure probe knew nothing of
			// them, so the retry used to die at the partition step listing debris it
			// had just created and telling the user to restart the installer. The
			// ERASE ack for this install already covers this disk, and the backend
			// still refuses to touch a LIVING Ryoku install.
			if len(m.reclaim) > 0 || m.retried {
				env = append(env, "RYOKU_RECLAIM_LEFTOVERS=1")
			}
		}
	}
	return env
}

func def(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// install messages flow from the backend goroutine into the Bubble Tea loop.
type installLineMsg string
type installStepMsg int
type installDoneMsg struct{ err error }

type installStream struct {
	ch       chan tea.Msg
	cmd      *exec.Cmd     // the running backend, so a ctrl+c abort can signal it
	procDone chan struct{} // closed once cmd.Wait returns, so kill can block on reap
}

func (s *installStream) wait() tea.Cmd { return func() tea.Msg { return <-s.ch } }

// kill SIGKILLs the backend group and blocks (bounded) until it is reaped, so no
// sgdisk/pacstrap child keeps writing the disk after the TUI exits. Used by the
// install-state ctrl+c abort.
func (s *installStream) kill() {
	if s == nil {
		return
	}
	killBackend(s.cmd)
	if s.procDone != nil {
		select {
		case <-s.procDone:
		case <-time.After(3 * time.Second): // wedged reap must not hang the quit
		}
	}
}

// killBackend SIGKILLs the backend's whole process group (we set Setpgid), so a
// stuck sgdisk/pacstrap child dies with it; falls back to the bare process. A
// no-op before Start (Process nil).
func killBackend(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill()
	}
}

// stepIndex maps a backend @@RYOKU_STEP id to an install row.
func stepIndex(id string) (int, bool) {
	order := []string{"partition", "filesystems", "mount", "pacstrap", "configure", "bootloader"}
	for i, s := range order {
		if s == id {
			return i, true
		}
	}
	return 0, false
}

// ansiCSI matches an ANSI CSI escape (colours, cursor moves, line erase) -- what
// pacman/curl progress bars spray alongside carriage returns.
var ansiCSI = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

// sanitizeLine flattens a child's terminal output into a plain string the
// bubbletea viewport can render without shredding: keep only the final segment a
// carriage-return progress bar left, strip ANSI escapes, and drop stray control
// bytes. Display-only -- the @@RYOKU sentinels are matched on the raw line.
func sanitizeLine(s string) string {
	if i := strings.LastIndexByte(s, '\r'); i >= 0 {
		s = s[i+1:]
	}
	s = ansiCSI.ReplaceAllString(s, "")
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// startInstall launches the backend with the built environment and streams its
// output as messages. The backend path comes from RYOKU_BACKEND or PATH.
func (m *model) startInstall() tea.Cmd {
	st := &installStream{ch: make(chan tea.Msg, 128), procDone: make(chan struct{})}
	m.istream = st
	env := append(os.Environ(), m.installEnv()...)
	bin := os.Getenv("RYOKU_BACKEND")
	if bin == "" {
		bin = "ryoku-install"
	}
	cmd := exec.Command(bin)
	cmd.Env = env
	// Own process group so an abort (scanner overflow or ctrl+c) can signal the
	// whole tree -- backend plus its sgdisk/pacstrap child -- and reap it. The
	// model holds the cmd (via st) to do that.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	st.cmd = cmd
	// Start synchronously: st.cmd.Process is then set before any ctrl+c abort can
	// read it (no data race with kill), and a spawn failure surfaces at once.
	if err := cmd.Start(); err != nil {
		close(st.procDone)
		st.ch <- installDoneMsg{err} // buffered channel: never blocks
		return st.wait()
	}
	go func() {
		done := make(chan error, 1)
		go func() { done <- cmd.Wait(); pw.Close(); close(st.procDone) }()
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			if id, ok := strings.CutPrefix(line, "@@RYOKU_STEP "); ok {
				if idx, ok := stepIndex(strings.TrimSpace(id)); ok {
					st.ch <- installStepMsg(idx)
					continue
				}
			}
			if line == "@@RYOKU_DONE" {
				continue
			}
			st.ch <- installLineMsg(sanitizeLine(line))
		}
		// A backend line longer than the 1 MiB scanner cap stops Scan with
		// ErrTooLong. Left alone, this goroutine would exit while the backend
		// blocks writing into the now-unread pipe, and the UI would spin on <-done
		// forever. Kill the group, drain the pipe so cmd.Wait can return, and
		// deliver a wrapping error so the failure screen shows instead.
		if err := sc.Err(); err != nil {
			killBackend(cmd)
			io.Copy(io.Discard, pr)
			st.ch <- installDoneMsg{fmt.Errorf("install output overflowed the reader buffer: %w", err)}
			return
		}
		st.ch <- installDoneMsg{<-done}
	}()
	return st.wait()
}

// netOnline reports whether the live system already has internet, and nothing
// else. It used to answer true on a bundled image "because the install can
// proceed", which hid a real question behind a false answer: every gate that
// asked about connectivity silently got "yes" on a machine with no network, so
// whether an offline install worked depended on a short-circuit buried in here
// rather than on the gates. The gates now ask offlineRepo() themselves (see
// reviewBlockReason, the kNet step, and installEnv). WIRE target.
func netOnline() bool {
	if out, ok := run("ip", "-4", "route"); ok && strings.Contains(out, "default") {
		return true
	}
	// no default route: probe a real endpoint over HTTPS. ICMP (ping) is dropped
	// by many corporate/hotel/ISP firewalls even where package mirrors are fully
	// reachable, so a ping check false-negatives and blocks the install; an HTTPS
	// fetch of a canonical Arch endpoint does not.
	return exec.Command("curl", "-fsS", "--max-time", "4", "-o", "/dev/null",
		"https://geo.mirror.pkgbuild.com/").Run() == nil
}

// offlineRepoPath is where an offline ISO bakes the full package closure
// (installation/iso/build.sh + offline-repo.sh).
const offlineRepoPath = "/usr/share/ryoku/offline/repo"

// offlineRepo reports whether this live ISO carries a USABLE baked offline
// package repo. The db glob is the same test lib/offline.sh:ryoku_offline_active
// uses, and it has to be: this decides whether installEnv sends RYOKU_ONLINE=0,
// and a bare directory (a bake that died before repo-add, say) used to satisfy
// the installer while failing the backend's check -- so the TUI promised an
// offline install and the backend quietly took the online path into a pacstrap
// that cannot reach a mirror. Same question, same answer, both sides.
func offlineRepo() bool {
	fi, err := os.Stat(offlineRepoPath)
	if err != nil || !fi.IsDir() {
		return false
	}
	db, _ := filepath.Glob(filepath.Join(offlineRepoPath, "offline.db*"))
	return len(db) > 0
}

// netInterface returns the active default-route interface name, for the
// connected screen (for example "eth0" or "enp1s0"). WIRE target.
func netInterface() string {
	if out, ok := run("sh", "-c", "ip -4 route show default | awk '{print $5; exit}'"); ok && out != "" {
		return out
	}
	return i18n.T("online")
}
