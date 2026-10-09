package doctor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: boot partition headroom -------------------------------------
//
// Every installed kernel keeps its boot image on /boot, and limine-snapper-sync
// keeps one history copy per kernel so the snapshot entries have a kernel that
// matches them. Ryoku sizes /boot at 2 GiB, which fits that for two kernels
// with room to rebuild -- until it does not: a third kernel, a fallback image,
// or the nouveau firmware the GPU trim removes push it over. What makes a full
// /boot expensive is how quietly it fails: mkinitcpio builds the image in /tmp
// and the hook copies it in, so the copy fails, pacman still reports the
// transaction as a success, and the box keeps booting the previous kernel
// against a module tree the upgrade already deleted (issue #140). The stale
// image then cannot be rebuilt either, so the box stops taking kernel updates
// for good.
//
// Nothing here can free space safely on its own -- every candidate is either a
// running kernel's image or a snapshot's only matching kernel -- so this names
// the wall and what to remove, and the image reconcilers stay honest about why
// a rebuild failed.

// bootImageSpace is the state the plan needs: free bytes on /boot and the size
// of the largest image already there.
type bootImageSpace struct {
	free         uint64
	freeMeasured bool
	largest      uint64
	name         string
	inaccessible bool
}

// bootImageRoots are where a kernel image lands: the UKI directory, and /boot
// itself for the vmlinuz + initramfs layout.
var (
	bootImageRoots = []string{"/boot/EFI/Linux", "/boot"}
	bootFreeBytes  = sys.FreeBytes
)

// measureBootImages: free space on /boot plus the biggest kernel image on it.
// An inaccessible path marks the state so the plan warns instead of reporting
// false headroom or false success.
func measureBootImages() bootImageSpace {
	st := bootImageSpace{}
	if free, ok := bootFreeBytes("/boot"); ok {
		st.free = free
		st.freeMeasured = true
	}
	for _, dir := range bootImageRoots {
		ents, err := os.ReadDir(dir)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				st.inaccessible = true
			}
			continue
		}
		for _, e := range ents {
			if e.IsDir() || !bootImageName(e.Name()) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					st.inaccessible = true
				}
				continue
			}
			if size := uint64(info.Size()); size > st.largest {
				st.largest, st.name = size, filepath.Join(dir, e.Name())
			}
		}
	}
	return st
}

// bootImageName: is this a kernel image (a UKI, or an initramfs beside its
// vmlinuz)? The microcode image is deliberately not one: it is small and never
// rebuilt per kernel.
func bootImageName(name string) bool {
	switch filepath.Ext(name) {
	case ".efi":
		return true
	case ".img":
		return strings.HasPrefix(name, "initramfs-")
	}
	return false
}

// enoughBootHeadroom: a rebuild writes one new image and copies the outgoing
// one to the history directory, so room for two of the largest image is the
// floor. Below it, the next kernel update cannot land.
func enoughBootHeadroom(st bootImageSpace) bool {
	if st.largest == 0 {
		return true // no images measured: nothing to size against
	}
	return st.free >= 2*st.largest
}

func reconcileBootSpace(checkOnly bool) recResult {
	if !liminePkgInstalled("limine") {
		return okRes(i18n.T("not a limine-managed boot on this box"))
	}
	st := measureBootImages()
	if st.inaccessible {
		return warnRes(i18n.T("cannot inspect kernel images on /boot to verify headroom")).
			withFix("sudo ryoku doctor --check")
	}
	if !st.freeMeasured {
		return warnRes(i18n.T("cannot measure free space on /boot to verify headroom")).
			withFix("sudo ryoku doctor --check")
	}
	if st.largest == 0 {
		return okRes(i18n.T("no kernel images on /boot to size against"))
	}
	if enoughBootHeadroom(st) {
		return okRes(i18n.T("/boot has %s free, room for a %s image rebuild"),
			humanSize(st.free), humanSize(st.largest))
	}
	// Nothing safe to delete, in check mode or out of it: name the wall.
	return warnRes(i18n.T("/boot has only %s free but a kernel image is %s: the next kernel update builds its image and cannot copy it in, and pacman reports success anyway (the box then keeps booting the old kernel)"),
		humanSize(st.free), humanSize(st.largest)).
		withFix(i18n.T("run `ryoku doctor` to trim the initramfs, then remove a kernel you do not boot (`pacman -Rns <kernel>`) or lower MAX_SNAPSHOT_ENTRIES in /etc/default/limine"))
}

// humanSize: MiB up to a gibibyte, then GiB with one decimal. Sizes here are
// always image- or partition-scale, so bytes never need naming.
func humanSize(n uint64) string {
	if n >= 1<<30 {
		return fmt.Sprintf("%.1f GiB", float64(n)/(1<<30))
	}
	return fmt.Sprintf("%d MiB", n>>20)
}
