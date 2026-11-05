package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/limineefi"
	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

// The boot guard reverts a packaged update whose next boots never bring the
// desktop up. The pieces:
//
//   - stage2 arms it: pendingFile records the releases, snapshot, and current
//     boot.
//   - the shell daemon records a good boot after its crash window.
//   - ryoku-boot-guard.service judges the boot that just ended, never the one
//     currently starting. On the second completed boot without a good marker it
//     tracks the previous release back, re-materializes every user's config,
//     and leaves a notice the doctor surfaces. On a third it points the boot
//     menu at the pre-update snapshot, which is the last resort when the
//     packages were not the problem.
//
// Nothing here needs the user session, so it works when the session is what
// broke.
// The guard's on-disk state. Vars, not consts, so a test can point them at a
// temp dir instead of /var/lib, /boot, and efivarfs.
var (
	pendingFile           = "/var/lib/ryoku/update-pending.json"
	bootOKDir             = "/var/lib/ryoku/boot"
	noticeFile            = "/var/lib/ryoku/boot/notice.json"
	limineConf            = "/boot/limine.conf"
	restoreMarker         = "/run/lock/limine-snapper-restore.lock"
	limineLastBootedEntry = limineefi.LastBootedEntryPath
)

var effectiveUID = os.Geteuid

var (
	readBootID          = bootID
	removePendingFile   = os.Remove
	materializeHomeGlob = "/home/*/.config/ryoku"
	runUserMaterialize  = materializeUser
)

type pendingUpdate struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Channel is what the box tracked before the update (stable, testing, or a
	// frozen version). The revert pins the previous version, but the user gets
	// back onto updates with `ryoku track <Channel>`.
	Channel     string `json:"channel,omitempty"`
	Snapshot    string `json:"snapshot,omitempty"`
	ArmedBoot   string `json:"armedBoot"`
	CheckedBoot string `json:"checkedBoot,omitempty"`
	Boots       int    `json:"boots"`
	At          string `json:"at"`
}

// bootNotice is what the doctor shows after the guard acted.
type bootNotice struct {
	Action   string `json:"action"` // reverted, snapshot-default, revert-failed
	From     string `json:"from"`
	To       string `json:"to"`
	Channel  string `json:"channel,omitempty"` // the channel to `ryoku track` back to
	Snapshot string `json:"snapshot,omitempty"`
	Detail   string `json:"detail,omitempty"`
	At       string `json:"at"`
}

func bootID() string {
	b, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func bootProven(id string) bool {
	matches, _ := filepath.Glob(filepath.Join(bootOKDir, "ok-*"))
	for _, match := range matches {
		body, err := os.ReadFile(match)
		if err == nil && strings.TrimSpace(string(body)) == id {
			return true
		}
	}
	return false
}

func writePendingUpdate(p pendingUpdate) error {
	body, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pendingFile, append(body, '\n'), 0o644)
}

// armBootGuard is called by stage2 on a packaged box once the packages are in.
// RYOKU_UPDATE_FROM carries the version the first stage read before the package
// move; the marker is written only when a frozen version actually changed.
func armBootGuard(snapshot string) {
	if sys.ResolveRepo() != "" {
		return
	}
	from := strings.TrimSpace(os.Getenv("RYOKU_UPDATE_FROM"))
	to := sys.ReadRelease().Release
	if from == "" || to == "" || from == to || !sys.IsFrozenVersion(from) {
		return
	}
	armedBoot := readBootID()
	p := pendingUpdate{
		From:        from,
		To:          to,
		Channel:     packagedChannel(),
		Snapshot:    snapshot,
		ArmedBoot:   armedBoot,
		CheckedBoot: armedBoot,
		At:          time.Now().UTC().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(p, "", "  ")
	if matches, _ := filepath.Glob(filepath.Join(bootOKDir, "ok-*")); len(matches) > 0 {
		_ = sys.Sudo(append([]string{"rm", "-f"}, matches...)...)
	}
	if err := sys.WriteRootFile(pendingFile, string(b)+"\n", "0644"); err != nil {
		fmt.Fprintf(os.Stderr, i18n.T("note: boot guard not armed: %v\n"), err)
		return
	}
	progress.logf(i18n.T("Boot guard armed: %s -> %s"), from, to)
}

// BootGuard is `ryoku boot-guard`, run as root by ryoku-boot-guard.service.
func BootGuard(args []string) error {
	if effectiveUID() != 0 {
		return fmt.Errorf(i18n.T("ryoku boot-guard runs as root (ryoku-boot-guard.service)"))
	}
	if len(args) > 0 {
		switch args[0] {
		case "--disarm":
			return disarmBootGuard(i18n.T("disarmed by hand"))
		case "--restored":
			return restoreBootMenuDefault()
		}
	}
	raw, err := os.ReadFile(pendingFile)
	if err != nil {
		return nil // nothing pending
	}
	var p pendingUpdate
	if err := json.Unmarshal(raw, &p); err != nil {
		_ = os.Remove(pendingFile)
		return nil
	}
	current := readBootID()
	previous := p.CheckedBoot
	if previous == "" {
		previous = p.ArmedBoot
	}
	if current == "" || current == previous {
		return nil
	}
	p.CheckedBoot = current
	if previous == "" || previous == p.ArmedBoot {
		return writePendingUpdate(p)
	}
	if bootProven(previous) {
		fmt.Printf(i18n.T("boot guard: %s came up after the update; disarmed\n"), p.To)
		return disarmBootGuard("")
	}
	p.Boots++
	if err := writePendingUpdate(p); err != nil {
		return err
	}
	fmt.Printf(i18n.T("boot guard: boot %d after %s -> %s without the desktop coming up\n"), p.Boots, p.From, p.To)
	switch {
	case p.Boots < 2:
		return nil
	case p.Boots == 2:
		return revertRelease(p)
	default:
		return pointBootMenuAtSnapshot(p)
	}
}

func disarmBootGuard(why string) error {
	if err := removePendingFile(pendingFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(i18n.T("boot guard: could not disarm: %w"), err)
	}
	if why != "" {
		fmt.Println("boot guard:", why)
	}
	return nil
}

// revertRelease puts the Ryoku set back on the release the box ran before and
// re-materializes every user's config from it. It goes through the same
// transactional set-aware move an interactive `ryoku track` uses
// (channelmove.go): the pin is written, the whole set (split metas dropped)
// moves in one transaction, and on ANY failure the previous pin is restored and
// the databases re-synced -- so a downgrade that cannot satisfy the new split
// packages never leaves the pin, sync db, and installed set disagreeing (#291,
// the bug this file caused). Package moves need the network: the guard waits for
// it here (only on this boot, so a healthy boot never pays for it), and when the
// channel is still unreachable it restores the pin and hands the boot back so
// the next one retries the revert instead of escalating to the snapshot.
func revertRelease(p pendingUpdate) error {
	fmt.Printf(i18n.T("boot guard: reverting to %s\n"), p.From)
	if sys.Has("nm-online") {
		_ = sys.Run("nm-online", "-q", "--timeout=90")
	}
	err := retargetChannel(p.From, func() error {
		_, err := moveRyokuSetToChannel()
		return err
	})
	if err != nil {
		if errors.Is(err, errChannelUnreachable) {
			// retargetChannel already put the pin back; retry the whole revert
			// next boot rather than escalating to the snapshot.
			fmt.Println(i18n.T("boot guard: package channel unreachable; retrying the revert next boot"))
			p.Boots--
			if writeErr := writePendingUpdate(p); writeErr != nil {
				return writeErr
			}
			return nil
		}
		return writeNotice(bootNotice{Action: "revert-failed", From: p.From, To: p.To, Channel: p.Channel, Snapshot: p.Snapshot, Detail: err.Error(), At: now()})
	}
	if err := rematerializeUsers(); err != nil {
		p.Boots--
		if writeErr := writePendingUpdate(p); writeErr != nil {
			return errors.Join(err, writeErr)
		}
		return fmt.Errorf(i18n.T("boot guard: packages reverted but user configuration was not repaired: %w"), err)
	}
	if err := disarmBootGuard(""); err != nil {
		return err
	}
	back := p.Channel
	if back == "" {
		back = sys.ChannelStable
	}
	detail := fmt.Sprintf(i18n.T("the desktop did not come up in two boots after the update; the Ryoku set is back on %s (Arch untouched). `ryoku track %s` moves forward again once the release is fixed."), p.From, sys.TrackName(back))
	switch updatePackageManager() {
	case host.XBPS:
		detail = fmt.Sprintf(i18n.T("the desktop did not come up in two boots after the update; the Ryoku set is back on %s (Void untouched). `ryoku track %s` moves forward again once the release is fixed."), p.From, sys.TrackName(back))
	case host.DNF:
		detail = fmt.Sprintf(i18n.T("the desktop did not come up in two boots after the update; the Ryoku set is back on %s (Fedora untouched). `ryoku track %s` moves forward again once the release is fixed."), p.From, sys.TrackName(back))
	}
	return writeNotice(bootNotice{Action: "reverted", From: p.From, To: p.To, Channel: p.Channel, Snapshot: p.Snapshot,
		Detail: detail,
		At:     now()})
}

// rematerializeUsers lays the (now previous) release's base config into every
// home that has a Ryoku config, as that user, so the session matches the
// packages it will start under.
func rematerializeUsers() error {
	homes, err := filepath.Glob(materializeHomeGlob)
	if err != nil {
		return err
	}
	var failures []error
	for _, config := range homes {
		home := filepath.Dir(filepath.Dir(config))
		user := filepath.Base(home)
		if err := runUserMaterialize(user, home); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", user, err))
		}
	}
	return errors.Join(failures...)
}

func materializeUser(user, home string) error {
	return sys.Run("runuser", "-u", user, "--", "env",
		"HOME="+home,
		"USER="+user,
		"LOGNAME="+user,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"ryoku", "materialize")
}

// pointBootMenuAtSnapshot makes the pre-update snapshot the default boot
// entry, for the case where the packages were not what broke the boot.
func pointBootMenuAtSnapshot(p pendingUpdate) error {
	if updatePackageManager() == host.DNF {
		return grubBootSnapshot(p)
	}
	if p.Snapshot == "" {
		return writeNotice(bootNotice{Action: "revert-failed", From: p.From, To: p.To, Detail: "no pre-update snapshot to boot; restore from the Limine Snapshots menu by hand", At: now()})
	}
	raw, err := os.ReadFile(limineConf)
	if err != nil {
		return err
	}
	entry := snapshotEntryPath(string(raw), p.Snapshot)
	if entry == "" {
		return writeNotice(bootNotice{Action: "revert-failed", From: p.From, To: p.To, Snapshot: p.Snapshot, Detail: "snapshot " + p.Snapshot + " has no boot entry; restore from the Limine Snapshots menu by hand", At: now()})
	}
	if err := setLimineDefault(raw, entry); err != nil {
		return err
	}
	if err := clearLimineLastBootedEntry(); err != nil {
		return err
	}
	_ = os.Remove(pendingFile)
	return writeNotice(bootNotice{Action: "snapshot-default", From: p.From, To: p.To, Snapshot: p.Snapshot,
		Detail: "three boots failed after the update, so the boot menu now defaults to pre-update snapshot " + p.Snapshot + ". Boot it, then run `sudo limine-snapper-restore` to make it permanent, and `sudo ryoku boot-guard --disarm` clears this.",
		At:     now()})
}

// grubBootSnapshot sends the next boot, once, to the pre-update snapshot's look
// entry. It never restores unattended: the user sees the system from before
// the update and keeps it with `sudo ryoku boot-menu restore`, and a boot after
// that one without a decision is back on the updated system.
func grubBootSnapshot(p pendingUpdate) error {
	_ = os.Remove(pendingFile)
	fail := func(detail string) error {
		return writeNotice(bootNotice{Action: "revert-failed", From: p.From, To: p.To, Channel: p.Channel,
			Snapshot: p.Snapshot, Detail: detail, At: now()})
	}
	if p.Snapshot == "" {
		return fail(i18n.T("no pre-update snapshot to boot; `ryoku rollback` shows how to recover from another one"))
	}
	if !strings.Contains(readText(snapshotMenuCfg()), " --id ryoku-snapshot-"+p.Snapshot+" ") {
		return fail(fmt.Sprintf(i18n.T("pre-update snapshot %s is not in the boot menu; `ryoku rollback %s` shows how to recover"), p.Snapshot, p.Snapshot))
	}
	if err := sys.Run("grub2-reboot", snapshotEntryID(p.Snapshot)); err != nil {
		return fail(fmt.Sprintf(i18n.T("could not send the next boot to snapshot %s: %v; `ryoku rollback %s` shows how to recover"), p.Snapshot, err, p.Snapshot))
	}
	return writeNotice(bootNotice{Action: "snapshot-default", From: p.From, To: p.To, Channel: p.Channel, Snapshot: p.Snapshot,
		Detail: fmt.Sprintf(i18n.T("three boots failed after the update, so the next boot opens pre-update snapshot %s read-only. If it works, `sudo ryoku boot-menu restore` there keeps it."), p.Snapshot),
		At:     now()})
}

func restoreBootMenuDefault() error {
	if updatePackageManager() == host.DNF {
		return nil
	}
	rawMarker, err := os.ReadFile(restoreMarker)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(rawMarker)) != "restored" {
		return nil
	}
	raw, err := os.ReadFile(limineConf)
	if err != nil {
		return err
	}
	entry := firstKernelEntryPath(string(raw))
	if entry == "" {
		return errors.New(i18n.T("the Ryoku Linux menu has no kernel entry"))
	}
	if err := setLimineDefault(raw, entry); err != nil {
		return err
	}
	if err := clearLimineLastBootedEntry(); err != nil {
		return err
	}
	fmt.Printf(i18n.T("boot guard: snapshot restore complete; Limine now defaults to %s\n"), entry)
	return nil
}

func setLimineDefault(raw []byte, entry string) error {
	lines := strings.Split(string(raw), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "default_entry:") {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		lines[i] = indent + "default_entry: " + entry
		found = true
	}
	if !found {
		return errors.New(i18n.T("Limine config has no default_entry setting"))
	}
	return os.WriteFile(limineConf, []byte(strings.Join(lines, "\n")), 0o644)
}

func clearLimineLastBootedEntry() error {
	return limineefi.ClearLastBootedEntry(limineLastBootedEntry)
}

// snapshotEntryPath follows the generated menu hierarchy so nested snapshot
// entries retain their Ryoku Linux parent.
func snapshotEntryPath(conf, id string) string {
	want := "subvol=/@snapshots/" + id + "/snapshot"
	var path []string
	for _, raw := range strings.Split(conf, "\n") {
		depth, name, ok := limineEntry(raw)
		if ok {
			if depth <= len(path) {
				path = path[:depth-1]
			}
			if depth != len(path)+1 {
				path = nil
				continue
			}
			path = append(path, name)
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(raw), "cmdline:") || !strings.Contains(raw, want) {
			continue
		}
		snapshots := -1
		for i, component := range path {
			if limineUnescape(component) == "Snapshots" {
				snapshots = i
				break
			}
		}
		if snapshots < 0 || len(path) != snapshots+3 {
			continue
		}
		if snapshots == 0 || snapshots == 1 && limineUnescape(path[0]) == "Ryoku Linux" {
			return limineEntryPath(path)
		}
	}
	return ""
}

func firstKernelEntryPath(conf string) string {
	inRyoku := false
	var root string
	for _, raw := range strings.Split(conf, "\n") {
		depth, name, ok := limineEntry(raw)
		if !ok {
			continue
		}
		switch depth {
		case 1:
			inRyoku = limineUnescape(name) == "Ryoku Linux"
			root = name
		case 2:
			if inRyoku && limineUnescape(name) != "Snapshots" {
				return limineEntryPath([]string{root, name})
			}
		}
	}
	return ""
}

func limineEntry(raw string) (int, string, bool) {
	line := strings.TrimSpace(raw)
	depth := 0
	for depth < len(line) && line[depth] == '/' {
		depth++
	}
	if depth == 0 {
		return 0, "", false
	}
	name := strings.TrimSpace(line[depth:])
	return depth, name, name != ""
}

func limineEntryPath(components []string) string {
	escaped := make([]string, len(components))
	for i, component := range components {
		escaped[i] = limineEscape(limineUnescape(component))
	}
	return strings.Join(escaped, "/")
}

func limineUnescape(name string) string {
	var out strings.Builder
	out.Grow(len(name))
	for i := 0; i < len(name); i++ {
		if name[i] == '\\' && i+1 < len(name) && (name[i+1] == '\\' || name[i+1] == '/' || name[i+1] == '#') {
			i++
		}
		out.WriteByte(name[i])
	}
	return out.String()
}

func limineEscape(name string) string {
	var out strings.Builder
	out.Grow(len(name))
	for i := range len(name) {
		if name[i] == '\\' || name[i] == '/' || name[i] == '#' {
			out.WriteByte('\\')
		}
		out.WriteByte(name[i])
	}
	return out.String()
}

func writeNotice(n bootNotice) error {
	b, _ := json.MarshalIndent(n, "", "  ")
	_ = os.MkdirAll(bootOKDir, 0o1777)
	fmt.Println("boot guard:", n.Action, n.Detail)
	return os.WriteFile(noticeFile, append(b, '\n'), 0o644)
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// BootNotice reads the guard's last notice, or nil.
func BootNotice() *bootNotice {
	b, err := os.ReadFile(noticeFile)
	if err != nil {
		return nil
	}
	var n bootNotice
	if json.Unmarshal(b, &n) != nil {
		return nil
	}
	return &n
}
