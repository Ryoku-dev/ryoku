package doctor

import (
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: boot volumes mounted read-only ------------------------------
//
// /boot, and /efi on an alongside install, are FAT volumes, and a FAT the
// firmware or a neighbouring OS left dirty can come up read-only, or flip to
// read-only after the first write error. Every Ryoku boot-path write then fails
// in a way that looks unrelated to a filesystem: the mkinitcpio hook copies
// nothing, limine-update writes nothing, the boot guard reports a missing
// image, and `ryoku update` ends in "rollback now" over a volume the user never
// chose to protect. The btrfs root's read-only flip is reconcileBtrfsHealth's
// business; this owns the FAT boot volumes, where the repair is mechanical:
// remount read-write, and if the kernel refuses, unmount, let fsck.fat -a
// repair, and mount again.

// bootRWMounts are the mounts this reconciler owns, in report order. /efi only
// exists on an alongside install; a target that is not its own mount point is
// skipped, not reported.
var bootRWMounts = []string{"/boot", "/efi"}

// bootRWState is one mount's findmnt answer.
type bootRWState struct {
	target string
	source string
	fstype string
	ro     bool
}

// bootRWOptionsRO reads the OPTIONS column: the mount flags, comma-joined,
// where the first is rw or ro.
func bootRWOptionsRO(opts string) bool {
	first := strings.SplitN(strings.TrimSpace(opts), ",", 2)
	return len(first) > 0 && first[0] == "ro"
}

// readBootRWState parses one `findmnt -n -o OPTIONS,SOURCE,FSTYPE <target>`
// line. None of the three columns carries a space, so the line is three fields;
// anything else is a findmnt we do not understand and reads as absent.
func readBootRWState(out, target string) (bootRWState, bool) {
	f := strings.Fields(out)
	if len(f) != 3 {
		return bootRWState{}, false
	}
	return bootRWState{target: target, source: f[1], fstype: f[2], ro: bootRWOptionsRO(f[0])}, true
}

// Seams over the live box, replaced in tests.
var (
	bootRWFindmnt = func(target string) (string, error) {
		return sys.RunOut("findmnt", "-n", "-o", "OPTIONS,SOURCE,FSTYPE", target)
	}
	bootRWRemount = func(target string) error {
		return sys.Run("sudo", "mount", "-o", "remount,rw", target)
	}
	bootRWUmount = func(target string) error { return sys.Run("sudo", "umount", target) }
	bootRWFsck   = func(source string) error { return sys.Run("sudo", "fsck.fat", "-a", source) }
	bootRWMount  = func(target string) error { return sys.Run("sudo", "mount", target) }
)

// repairBootRW brings one read-only FAT mount back read-write. The remount is
// tried first because it is free when the read-only state was transient; the
// unmount-and-fsck path only runs for a volume the kernel still refuses, and a
// busy mount is left alone with the exact manual step instead of being forced.
func repairBootRW(st bootRWState) (fixed bool, detail string) {
	if bootRWRemount(st.target) == nil {
		if out, err := bootRWFindmnt(st.target); err == nil {
			if now, ok := readBootRWState(out, st.target); ok && !now.ro {
				return true, i18n.Tf("remounted %s read-write", st.target)
			}
		}
	}
	if st.fstype != "vfat" {
		return false, i18n.Tf("%s is read-only and not a FAT volume; check it by hand", st.target)
	}
	if err := bootRWUmount(st.target); err != nil {
		return false, i18n.Tf("%s is read-only and busy, so it could not be unmounted for a repair; close what holds it (or reboot) and run `sudo fsck.fat -a %s`, then `sudo mount %s`", st.target, st.source, st.target)
	}
	_ = bootRWFsck(st.source)
	if err := bootRWMount(st.target); err != nil {
		return false, i18n.Tf("%s was unmounted for a repair and would not mount again; run `sudo fsck.fat -a %s` and `sudo mount %s`", st.target, st.source, st.target)
	}
	out, err := bootRWFindmnt(st.target)
	if err != nil {
		return false, i18n.Tf("%s mounted but its state could not be re-read", st.target)
	}
	now, ok := readBootRWState(out, st.target)
	if !ok || now.ro {
		return false, i18n.Tf("%s came back read-only after fsck.fat: the volume has errors the repair could not clear, back it up and recreate it", st.target)
	}
	return true, i18n.Tf("repaired and remounted %s read-write", st.target)
}

func reconcileBootRW(checkOnly bool) recResult {
	var ro []bootRWState
	for _, target := range bootRWMounts {
		out, err := bootRWFindmnt(target)
		if err != nil {
			continue // not a mount point on this box
		}
		if st, ok := readBootRWState(out, target); ok && st.ro {
			ro = append(ro, st)
		}
	}
	if len(ro) == 0 {
		return okRes(i18n.T("boot volumes are mounted read-write"))
	}
	if checkOnly {
		targets := make([]string, 0, len(ro))
		for _, st := range ro {
			targets = append(targets, st.target)
		}
		return wouldRes(i18n.T("%s mounted read-only; a repair would remount it read-write"), strings.Join(targets, ", ")).
			withFix(i18n.T("run `ryoku doctor` to repair the boot volume"))
	}
	var fixed, stuck []string
	for _, st := range ro {
		if ok, detail := repairBootRW(st); ok {
			fixed = append(fixed, detail)
		} else {
			stuck = append(stuck, detail)
		}
	}
	switch {
	case len(stuck) > 0 && len(fixed) > 0:
		return warnRes("%s", strings.Join(stuck, "; ")).withFix(i18n.T("the repaired volume(s): %s"), strings.Join(fixed, "; "))
	case len(stuck) > 0:
		return warnRes("%s", strings.Join(stuck, "; ")).
			withFix(i18n.T("repair the volume from a live session if it stays read-only"))
	default:
		return fixedRes("%s", strings.Join(fixed, "; "))
	}
}
