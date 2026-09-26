package doctor

import (
	"errors"
	"strings"
	"testing"
)

func TestReadBootRWState(t *testing.T) {
	cases := []struct {
		out    string
		target string
		want   bootRWState
		ok     bool
	}{
		{"rw,relatime,fmask=0022,dmask=0022 /dev/nvme0n1p2 vfat", "/boot",
			bootRWState{target: "/boot", source: "/dev/nvme0n1p2", fstype: "vfat", ro: false}, true},
		{"ro,relatime /dev/nvme0n1p1 vfat", "/efi",
			bootRWState{target: "/efi", source: "/dev/nvme0n1p1", fstype: "vfat", ro: true}, true},
		{"", "/efi", bootRWState{}, false},
		{"rw,relatime only-two", "/boot", bootRWState{}, false},
	}
	for _, c := range cases {
		got, ok := readBootRWState(c.out, c.target)
		if ok != c.ok || got != c.want {
			t.Fatalf("readBootRWState(%q) = %+v, %v; want %+v, %v", c.out, got, ok, c.want, c.ok)
		}
	}
}

func TestBootRWOptionsRO(t *testing.T) {
	for opts, want := range map[string]bool{
		"ro,relatime":          true,
		"rw,relatime,noatime":  false,
		"rw":                   false,
		"ro":                   true,
		"":                     false,
		"defaults,ro=whatever": false,
	} {
		if got := bootRWOptionsRO(opts); got != want {
			t.Fatalf("bootRWOptionsRO(%q) = %v, want %v", opts, got, want)
		}
	}
}

// stubBootRW replaces the live-box seams and restores them when the test ends.
func stubBootRW(t *testing.T, findmnt func(string) (string, error), remount, umount, fsck, mount func(string) error) {
	t.Helper()
	oldF, oldR, oldU, oldK, oldM := bootRWFindmnt, bootRWRemount, bootRWUmount, bootRWFsck, bootRWMount
	bootRWFindmnt, bootRWRemount, bootRWUmount, bootRWFsck, bootRWMount = findmnt, remount, umount, fsck, mount
	t.Cleanup(func() {
		bootRWFindmnt, bootRWRemount, bootRWUmount, bootRWFsck, bootRWMount = oldF, oldR, oldU, oldK, oldM
	})
}

func TestRepairBootRWRemountSuffices(t *testing.T) {
	stubBootRW(t,
		func(target string) (string, error) { return "rw,relatime /dev/sda2 vfat", nil },
		func(string) error { return nil },
		func(string) error { t.Fatal("umount must not run when the remount works"); return nil },
		func(string) error { t.Fatal("fsck must not run when the remount works"); return nil },
		func(string) error { t.Fatal("mount must not run when the remount works"); return nil })
	fixed, detail := repairBootRW(bootRWState{target: "/boot", source: "/dev/sda2", fstype: "vfat", ro: true})
	if !fixed || !strings.Contains(detail, "remounted /boot read-write") {
		t.Fatalf("repairBootRW = %v, %q; want a plain remount", fixed, detail)
	}
}

func TestRepairBootRWFsckPath(t *testing.T) {
	state := "ro,relatime /dev/sda2 vfat"
	stubBootRW(t,
		func(string) (string, error) {
			out := state
			return out, nil
		},
		func(string) error { return errors.New("kernel refused") },
		func(string) error { state = ""; return nil },
		func(src string) error {
			if src != "/dev/sda2" {
				t.Fatalf("fsck ran on %q, want the mount source", src)
			}
			return nil
		},
		func(string) error { state = "rw,relatime /dev/sda2 vfat"; return nil })
	fixed, detail := repairBootRW(bootRWState{target: "/boot", source: "/dev/sda2", fstype: "vfat", ro: true})
	if !fixed || !strings.Contains(detail, "repaired and remounted /boot read-write") {
		t.Fatalf("repairBootRW = %v, %q; want the unmount-fsck-mount repair", fixed, detail)
	}
}

func TestRepairBootRWBusyMountIsLeftAlone(t *testing.T) {
	stubBootRW(t,
		func(string) (string, error) { return "ro,relatime /dev/sda2 vfat", nil },
		func(string) error { return errors.New("kernel refused") },
		func(string) error { return errors.New("target is busy") },
		func(string) error { t.Fatal("fsck must not run on a mounted volume"); return nil },
		func(string) error { t.Fatal("mount must not run after a failed umount"); return nil })
	fixed, detail := repairBootRW(bootRWState{target: "/boot", source: "/dev/sda2", fstype: "vfat", ro: true})
	if fixed {
		t.Fatalf("a busy read-only mount must not report fixed, got %q", detail)
	}
	if !strings.Contains(detail, "busy") || !strings.Contains(detail, "fsck.fat -a /dev/sda2") {
		t.Fatalf("the busy path must name the manual repair, got %q", detail)
	}
}

func TestReconcileBootRW(t *testing.T) {
	stubBootRW(t,
		func(target string) (string, error) {
			if target == "/efi" {
				return "", errors.New("not a mount point")
			}
			return "rw,relatime /dev/sda2 vfat", nil
		},
		func(string) error { return nil }, nil, nil, nil)
	if res := reconcileBootRW(false); res.status != recOK {
		t.Fatalf("a read-write boot volume must be ok, got %v (%s)", res.status, res.detail)
	}
	if res := reconcileBootRW(true); res.status != recOK {
		t.Fatalf("check-only on a healthy box must be ok, got %v", res.status)
	}

	ro := true
	stubBootRW(t,
		func(target string) (string, error) {
			if target == "/boot" {
				if ro {
					return "ro,relatime /dev/sda2 vfat", nil
				}
				return "rw,relatime /dev/sda2 vfat", nil
			}
			return "", errors.New("not a mount point")
		},
		func(string) error { ro = false; return nil }, nil, nil, nil)
	if res := reconcileBootRW(true); res.status != recWouldFix {
		t.Fatalf("check-only on a read-only /boot must say what it would do, got %v (%s)", res.status, res.detail)
	}
	if res := reconcileBootRW(false); res.status != recFixed {
		t.Fatalf("a remountable read-only /boot must be fixed, got %v (%s)", res.status, res.detail)
	}
}
