package doctor

import (
	"os"
	"strings"

	i18n "ryoku-i18n"
)

// ---- reconciler: boot menu entries that boot nothing -------------------------
//
// The boot menu must list what this box can actually boot, and nothing else.
// Two things write it: limine-entry-tool regenerates the kernel sub-entries
// from the installed kernels, and Ryoku's installer writes one flat entry so a
// fresh install boots before that tool ever runs. When the tool later adopts
// the flat entry as its tree root, any OTHER flat entry an earlier installer
// left behind is stranded: it names a kernel image by path, nothing regenerates
// it, and nothing removed it. limineDropFlat only clears those in the
// standalone "/+" layout, so on the adopted layout (limine-entry-tool 1.37+,
// which every current box runs) they survive forever.
//
// A box installed as plain Arch that was once touched by a CachyOS-variant
// installer therefore keeps offering "Ryoku Linux (CachyOS)", pointing at a
// vmlinuz that does not exist: a menu item that fails to boot, naming a kernel
// the user does not have. Selecting it drops to the Limine console.
//
// So the rule here is by evidence, never by name: a top-level entry is dead
// when it boots on its own (no generated sub-entries under it), it names its
// image on this ESP, and that image is not there. An entry whose image cannot
// be resolved (a guid()/fslabel() volume that is not this one, a chainload into
// another disk) is never touched, and neither is a directory that has children.

// limineEntry is one top-level menu entry: its title line, the line range it
// spans, whether it has generated children, and the image it boots.
type limineEntry struct {
	title    string
	from, to int // inclusive line indices
	children bool
	image    string // resolved path under the ESP, "" when not resolvable here
}

// limineTopEntries splits a config into its top-level entries. esp is where
// boot():/ resolves. Pure, so the parse is testable without an ESP.
func limineTopEntries(conf, esp string) []limineEntry {
	var out []limineEntry
	lines := strings.Split(conf, "\n")
	cur := -1
	for i, l := range lines {
		t := strings.TrimLeft(l, " \t")
		switch limineDepth(t) {
		case 0:
			if cur >= 0 && strings.TrimSpace(l) != "" {
				out[cur].to = i
				if img := limineEntryImagePath(strings.TrimSpace(l), esp); img != "" {
					out[cur].image = img
				}
				if img := limineEntryKernelPath(strings.TrimSpace(l), esp); img != "" {
					out[cur].image = img
				}
			}
		case 1:
			out = append(out, limineEntry{title: limineNodeName(t), from: i, to: i})
			cur = len(out) - 1
		default:
			if cur >= 0 {
				out[cur].children = true
				out[cur].to = i
			}
		}
	}
	return out
}

// limineEntryKernelPath: the image a "kernel_path:" body line names. The UKI
// and initramfs keys are limineEntryImagePath's; this is the third shape, the
// flat linux-protocol entry the installer writes.
func limineEntryKernelPath(bodyLine, esp string) string {
	if v := strings.TrimPrefix(bodyLine, "kernel_path: "); v != bodyLine {
		return resolveLimineBootPath(strings.TrimSpace(v), esp)
	}
	return ""
}

// planLimineDeadEntries evaluates top-level entries with an injected probe.
// probe returns (exists, err). Genuine ENOENT (err == nil && !exists) marks
// an entry dead. Any filesystem error marks it inaccessible so callers avoid
// false deletions when images cannot be inspected.
func planLimineDeadEntries(entries []limineEntry, probe func(string) (bool, error)) (dead, inaccessible []string) {
	for _, e := range entries {
		if e.children || e.image == "" || e.title == "" {
			continue
		}
		exists, err := probe(e.image)
		if err != nil {
			inaccessible = append(inaccessible, e.title)
		} else if !exists {
			dead = append(dead, e.title)
		}
	}
	return dead, inaccessible
}

var limineDeadEntriesProbe = probePresence

// limineDropEntries removes those entries, title line and body, and repoints
// default_entry when it named one of them (a default that points at a title
// which no longer exists loops the countdown, the same failure limineEnsureAutoboot
// exists to prevent). Pure; changed=false when there is nothing to do.
func limineDropEntries(conf, esp string, titles []string) (string, bool) {
	if len(titles) == 0 {
		return conf, false
	}
	drop := map[string]bool{}
	for _, t := range titles {
		drop[t] = true
	}
	entries := limineTopEntries(conf, esp)
	cut := map[int]bool{}
	for _, e := range entries {
		if !drop[e.title] {
			continue
		}
		for i := e.from; i <= e.to; i++ {
			cut[i] = true
		}
	}
	if len(cut) == 0 {
		return conf, false
	}
	lines := strings.Split(conf, "\n")
	out := make([]string, 0, len(lines))
	for i, l := range lines {
		if cut[i] {
			continue
		}
		out = append(out, l)
	}
	next := strings.Join(out, "\n")
	// default_entry: "<title>" or "<title>/<kernel>" both die with the entry.
	if cur := strings.TrimSpace(limineDefaultEntry(next)); cur != "" {
		if head := strings.SplitN(cur, "/", 2)[0]; drop[head] {
			if want := limineDefaultKernelPath(next); want != "" {
				next = strings.ReplaceAll(next, "default_entry: "+cur, "default_entry: "+want)
			}
		}
	}
	return next, true
}

func reconcileLimineDeadEntries(checkOnly bool) recResult {
	if !limineManagedBoot() {
		return unmanagedBootResult()
	}
	if !liminePkgInstalled("limine") {
		return okRes(i18n.T("not a limine-managed boot on this box"))
	}
	b, err := os.ReadFile(limineESPConf)
	if err != nil {
		return noteRes(i18n.T("no readable %s; the layout reconciler owns that"), limineESPConf)
	}
	conf := string(b)
	if !strings.Contains(conf, "\n/") && !strings.HasPrefix(conf, "/") {
		return okRes(i18n.T("no boot menu entries to check"))
	}
	dead, inaccessible := planLimineDeadEntries(limineTopEntries(conf, "/boot"), limineDeadEntriesProbe)
	if len(inaccessible) > 0 {
		return warnRes(i18n.T("cannot inspect boot images on /boot to verify boot menu entries: %s"), strings.Join(inaccessible, ", ")).
			withFix("sudo ryoku doctor --check")
	}
	if len(dead) == 0 {
		return okRes(i18n.T("every boot menu entry points at an image that is there"))
	}
	list := strings.Join(dead, ", ")
	if checkOnly {
		return wouldRes(i18n.T("boot menu entr(ies) that boot nothing: %s (the image they name is not on the boot partition, so picking one drops to the Limine console)"), list).
			withFix(i18n.T("ryoku doctor removes them and keeps the countdown on a kernel that exists"))
	}
	next, changed := limineDropEntries(conf, "/boot", dead)
	if !changed {
		return okRes(i18n.T("every boot menu entry points at an image that is there"))
	}
	// readFileSafe trims trailing newlines, so put the file's last one back:
	// this is a boot config, not a string.
	if err := writeRootFile(limineESPConf, strings.TrimRight(next, "\n")+"\n", "0644"); err != nil {
		return failRes(i18n.T("could not remove the dead boot menu entr(ies) %s: %v"), list, err).
			withFix(i18n.T("edit %s by hand and delete: %s"), limineESPConf, list)
	}
	return fixedRes(i18n.T("removed %d boot menu entr(ies) that boot nothing: %s"), len(dead), list)
}
