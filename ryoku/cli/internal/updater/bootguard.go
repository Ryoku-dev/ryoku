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
//     previous release back (the Ryoku set moves in one pacman transaction;
//     Arch stays as it is), re-materializes every user's config from it, and
//     leaves a notice the doctor surfaces. On a third it points the boot menu
//     at the pre-update snapshot, which is the last resort when the packages
//     were not the problem.
//
// Nothing here needs the user session, so it works when the session is what
// broke.
// The guard's on-disk state. Vars, not consts, so a test can point them at a
// temp dir instead of /var/lib and /boot.
var (
	pendingFile = "/var/lib/ryoku/update-pending.json"
	bootOKDir   = "/var/lib/ryoku/boot"
	noticeFile  = "/var/lib/ryoku/boot/notice.json"
	limineConf  = "/boot/limine.conf"
)

type pendingUpdate struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Channel the box tracked before the update (stable, testing, ...). The
	// revert pins the previous release tag, but the user gets back onto updates
	// with `ryoku track <Channel>`, so the guard has to remember it (#291).
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

// armBootGuard is called by stage2 on a packaged box once the packages are
// in. RYOKU_UPDATE_FROM carries the release the first stage read before pacman
// ran; the marker is written only when the release actually changed.
func armBootGuard(snapshot string) {
	if updatePackageManager() == host.XBPS || sys.ResolveRepo() != "" {
		return
	}
	from := strings.TrimSpace(os.Getenv("RYOKU_UPDATE_FROM"))
	to := sys.ReadRelease().Release
	if from == "" || to == "" || from == to || !sys.IsReleaseTag(from) {
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
	if os.Geteuid() != 0 {
		return fmt.Errorf(i18n.T("ryoku boot-guard runs as root (ryoku-boot-guard.service)"))
	}
	if updatePackageManager() == host.XBPS {
		_ = os.Remove(pendingFile)
		return nil
	}
	if len(args) > 0 && args[0] == "--disarm" {
		return disarmBootGuard(i18n.T("disarmed by hand"))
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
	return writeNotice(bootNotice{Action: "reverted", From: p.From, To: p.To, Channel: p.Channel, Snapshot: p.Snapshot,
		Detail: fmt.Sprintf(i18n.T("the desktop did not come up in two boots after the update; the Ryoku set is back on %s (Arch untouched). `ryoku track %s` moves forward again once the release is fixed."), p.From, sys.TrackName(back)),
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
// entry, for the case where the packages were not what broke the boot. The
// entry is the one limine-snapper-sync generated for that snapshot: the
// nested entry under //Snapshots whose cmdline names the snapshot subvolume.
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
	lines := strings.Split(string(raw), "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "default_entry:") {
			lines[i] = "default_entry: " + entry
		}
	}
	if err := os.WriteFile(limineConf, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}
	_ = os.Remove(pendingFile)
	return writeNotice(bootNotice{Action: "snapshot-default", From: p.From, To: p.To, Snapshot: p.Snapshot,
		Detail: "three boots failed after the update, so the boot menu now defaults to pre-update snapshot " + p.Snapshot + ". Boot it, then run `sudo limine-snapper-restore` to make it permanent, and `sudo ryoku boot-guard --disarm` clears this.",
		At:     now()})
}

// snapshotEntryPath finds `Snapshots/<title>/<kernel>` for snapshot id in a
// limine-snapper-sync generated config: the ///<title> under //Snapshots
// whose ////<kernel> cmdline carries subvol=/@snapshots/<id>/snapshot.
func snapshotEntryPath(conf, id string) string {
	want := "subvol=/@snapshots/" + id + "/snapshot"
	var top, title, kernel string
	for _, raw := range strings.Split(conf, "\n") {
		l := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(l, "////"):
			kernel = strings.TrimSpace(strings.TrimPrefix(l, "////"))
		case strings.HasPrefix(l, "///"):
			title = strings.TrimSpace(strings.TrimPrefix(l, "///"))
			kernel = ""
		case strings.HasPrefix(l, "//"):
			top = strings.TrimSpace(strings.TrimPrefix(l, "//"))
			title, kernel = "", ""
		case strings.HasPrefix(l, "cmdline:") && strings.Contains(l, want):
			if top == "Snapshots" && title != "" && kernel != "" {
				return top + "/" + title + "/" + kernel
			}
		}
	}
	return ""
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
