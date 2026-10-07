package doctor

import "testing"

// ryoku-gpu-trim only works between autodetect and kms: it edits autodetect's
// module allowlist, and kms is what filters against it. Placed anywhere else it
// silently does nothing and every kernel image keeps the ~107 MiB of nouveau
// GSP firmware that fills a 2 GiB /boot. Pin the placement, the idempotency
// (doctor must stay quiet on a healthy box) and the refusal to touch a line
// without autodetect, where the hook could not filter anything.
func TestWithGPUTrim(t *testing.T) {
	for _, c := range []struct {
		name, conf, want string
		changed          bool
	}{
		{
			name:    "inserted right after autodetect",
			conf:    "HOOKS=(base udev plymouth keyboard autodetect microcode modconf kms block filesystems fsck)\n",
			want:    "HOOKS=(base udev plymouth keyboard autodetect ryoku-gpu-trim microcode modconf kms block filesystems fsck)\n",
			changed: true,
		},
		{
			name: "already present is a no-op",
			conf: "HOOKS=(base udev autodetect ryoku-gpu-trim kms)\n",
			want: "HOOKS=(base udev autodetect ryoku-gpu-trim kms)\n",
		},
		{
			name: "no autodetect: nothing to filter, line untouched",
			conf: "HOOKS=(base udev kms block filesystems fsck)\n",
			want: "HOOKS=(base udev kms block filesystems fsck)\n",
		},
		{
			name:    "comments and other keys survive",
			conf:    "# why\nMODULES=(nvidia)\nHOOKS=(base autodetect kms)\n",
			want:    "# why\nMODULES=(nvidia)\nHOOKS=(base autodetect ryoku-gpu-trim kms)\n",
			changed: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, changed := withGPUTrim(c.conf)
			if got != c.want {
				t.Errorf("withGPUTrim =\n%q\nwant\n%q", got, c.want)
			}
			if changed != c.changed {
				t.Errorf("changed = %v, want %v", changed, c.changed)
			}
		})
	}
}

func TestWithConsoleKeys(t *testing.T) {
	for _, c := range []struct {
		name, conf, want string
		changed          bool
	}{
		{
			name:    "inserted right after udev",
			conf:    "HOOKS=(base udev plymouth keyboard autodetect microcode modconf kms block filesystems fsck)\n",
			want:    "HOOKS=(base udev ryoku-console-keys plymouth keyboard autodetect microcode modconf kms block filesystems fsck)\n",
			changed: true,
		},
		{
			name: "already present is a no-op",
			conf: "HOOKS=(base udev ryoku-console-keys plymouth keyboard autodetect)\n",
			want: "HOOKS=(base udev ryoku-console-keys plymouth keyboard autodetect)\n",
		},
		{
			name: "systemd hooks stay untouched",
			conf: "HOOKS=(base systemd autodetect modconf kms block filesystems fsck)\n",
			want: "HOOKS=(base systemd autodetect modconf kms block filesystems fsck)\n",
		},
		{
			name: "commented hooks stay untouched",
			conf: "# HOOKS=(base udev plymouth keyboard)\n",
			want: "# HOOKS=(base udev plymouth keyboard)\n",
		},
		{
			name:    "stays before GPU trim",
			conf:    "HOOKS=(base udev plymouth keyboard autodetect ryoku-gpu-trim microcode modconf kms)\n",
			want:    "HOOKS=(base udev ryoku-console-keys plymouth keyboard autodetect ryoku-gpu-trim microcode modconf kms)\n",
			changed: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, changed := withConsoleKeys(c.conf)
			if got != c.want {
				t.Errorf("withConsoleKeys =\n%q\nwant\n%q", got, c.want)
			}
			if changed != c.changed {
				t.Errorf("changed = %v, want %v", changed, c.changed)
			}
		})
	}
}

// A rebuild writes one image and copies the outgoing one to the history
// directory, so /boot needs room for two of the largest image before a kernel
// update can land. Under that floor the copy fails while pacman still reports
// success, so the floor is the whole value of the check.
func TestEnoughBootHeadroom(t *testing.T) {
	const img = 280 << 20
	for _, c := range []struct {
		name string
		st   bootImageSpace
		want bool
	}{
		{"room for two images", bootImageSpace{free: 2 * img, largest: img}, true},
		{"one byte short", bootImageSpace{free: 2*img - 1, largest: img}, false},
		{"room for one image only", bootImageSpace{free: img, largest: img}, false},
		{"nothing measured", bootImageSpace{free: 0, largest: 0}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := enoughBootHeadroom(c.st); got != c.want {
				t.Errorf("enoughBootHeadroom(%+v) = %v, want %v", c.st, got, c.want)
			}
		})
	}
}

// The floor is measured against kernel images only. Counting the microcode
// image would size a vmlinuz-layout box against a 300 KiB file and let a /boot
// with no room at all pass.
func TestBootImageName(t *testing.T) {
	for name, want := range map[string]bool{
		"ryoku_linux.efi":             true,
		"initramfs-linux.img":         true,
		"initramfs-linux-cachyos.img": true,
		"amd-ucode.img":               false,
		"vmlinuz-linux":               false,
		"limine.conf":                 false,
	} {
		if got := bootImageName(name); got != want {
			t.Errorf("bootImageName(%q) = %v, want %v", name, got, want)
		}
	}
}
