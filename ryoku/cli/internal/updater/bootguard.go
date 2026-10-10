package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

// The boot guard reverts a packaged update whose next boots never bring the
// desktop up. The pieces:
//
//   - stage2 arms it: pendingFile records the release the box ran before, the
//     one it moved to, the pre-update snapshot, and the boot the update ran in.
//   - the shell daemon records a good boot: once the shell has stayed up past
//     its crash window it writes bootOKDir/ok-<uid> with the boot id.
//   - ryoku-boot-guard.service runs `ryoku boot-guard` early in every boot,
//     as root. With a marker present and no ok file from a boot after the
//     update, it counts the boot; on the second such boot it tracks the
//     previous release back in one package transaction, re-materializes every
//     user's config from it, and leaves a notice the doctor surfaces. On a
//     third it points the boot menu at the pre-update snapshot, which is the
//     last resort when the packages were not the problem.
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
	limineLastBootedEntry = "/sys/firmware/efi/efivars/LimineLastBootedEntry-513ee0d0-6e43-cb05-b272-f146a2fcb88a"
)

var effectiveUID = os.Geteuid

const fsImmutableFlag = 0x10

type pendingUpdate struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Channel is what the box tracked before the update (stable, testing, or a
	// frozen version). The revert pins the previous version, but the user gets
	// back onto updates with `ryoku track <Channel>`.
	Channel   string `json:"channel,omitempty"`
	Snapshot  string `json:"snapshot,omitempty"`
	ArmedBoot string `json:"armedBoot"`
	Boots     int    `json:"boots"`
	At        string `json:"at"`
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
	p := pendingUpdate{From: from, To: to, Channel: packagedChannel(), Snapshot: snapshot, ArmedBoot: bootID(), At: time.Now().UTC().Format(time.RFC3339)}
	b, _ := json.MarshalIndent(p, "", "  ")
	// earlier ok files would read as proof of a boot after this update; clear
	// them so only a boot from here on counts.
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
	if provenAfter(p.ArmedBoot) {
		fmt.Printf(i18n.T("boot guard: %s came up after the update; disarmed\n"), p.To)
		return disarmBootGuard("")
	}
	p.Boots++
	b, _ := json.MarshalIndent(p, "", "  ")
	_ = os.WriteFile(pendingFile, append(b, '\n'), 0o644)
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

// provenAfter reports whether any session recorded a good boot other than
// the one the update ran in.
func provenAfter(armedBoot string) bool {
	matches, _ := filepath.Glob(filepath.Join(bootOKDir, "ok-*"))
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		if id := strings.TrimSpace(string(b)); id != "" && id != armedBoot {
			return true
		}
	}
	return false
}

func disarmBootGuard(why string) error {
	_ = os.Remove(pendingFile)
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
			b, _ := json.MarshalIndent(p, "", "  ")
			_ = os.WriteFile(pendingFile, append(b, '\n'), 0o644)
			return nil
		}
		return writeNotice(bootNotice{Action: "revert-failed", From: p.From, To: p.To, Channel: p.Channel, Snapshot: p.Snapshot, Detail: err.Error(), At: now()})
	}
	rematerializeUsers()
	_ = os.Remove(pendingFile)
	back := p.Channel
	if back == "" {
		back = sys.ChannelStable
	}
	detail := fmt.Sprintf(i18n.T("the desktop did not come up in two boots after the update; the Ryoku set is back on %s (Arch untouched). `ryoku track %s` moves forward again once the release is fixed."), p.From, sys.TrackName(back))
	if updatePackageManager() == host.XBPS {
		detail = fmt.Sprintf(i18n.T("the desktop did not come up in two boots after the update; the Ryoku set is back on %s (Void untouched). `ryoku track %s` moves forward again once the release is fixed."), p.From, sys.TrackName(back))
	}
	return writeNotice(bootNotice{Action: "reverted", From: p.From, To: p.To, Channel: p.Channel, Snapshot: p.Snapshot,
		Detail: detail,
		At:     now()})
}

// rematerializeUsers lays the (now previous) release's base config into every
// home that has a Ryoku config, as that user, so the session matches the
// packages it will start under.
func rematerializeUsers() {
	homes, _ := filepath.Glob("/home/*/.config/ryoku")
	for _, h := range homes {
		home := filepath.Dir(filepath.Dir(h))
		user := filepath.Base(home)
		_ = sys.Run("runuser", "-u", user, "--", "env", "HOME="+home, "USER="+user, "LOGNAME="+user, "ryoku", "materialize")
	}
}

// pointBootMenuAtSnapshot makes the pre-update snapshot the default boot
// entry, for the case where the packages were not what broke the boot.
func pointBootMenuAtSnapshot(p pendingUpdate) error {
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

func restoreBootMenuDefault() error {
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
	file, err := os.Open(limineLastBootedEntry)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	flags, ioctlErr := unix.IoctlGetInt(int(file.Fd()), unix.FS_IOC_GETFLAGS)
	if ioctlErr == nil && flags&fsImmutableFlag != 0 {
		ioctlErr = unix.IoctlSetPointerInt(int(file.Fd()), unix.FS_IOC_SETFLAGS, flags&^fsImmutableFlag)
	}
	closeErr := file.Close()
	if ioctlErr != nil {
		return fmt.Errorf(i18n.T("clear Limine remembered-entry attributes: %w"), ioctlErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Remove(limineLastBootedEntry); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(i18n.T("clear Limine remembered entry: %w"), err)
	}
	return nil
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
