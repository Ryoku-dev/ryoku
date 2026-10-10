package doctor

import (
	"strings"
	"testing"
)

// The real shape from a box installed by an older CachyOS-variant installer and
// since adopted by limine-entry-tool 1.37+: the tree root carries the generated
// kernel children, and a second flat entry names a kernel image that is not on
// the ESP. That entry must go, and only that entry: the tree root has children,
// the EFI fallback resolves to a binary that is there, and a chainload into
// another volume is not ours to judge.
const limineAdoptedConf = `timeout: 3
default_entry: Ryoku Linux/linux
remember_last_entry: yes

/Ryoku Linux
  //linux
    protocol: efi
    path: boot():/EFI/Linux/ryoku_linux.efi#abc
     //Snapshots
     ///2026-09-06
     ////linux
     protocol: efi
     path: boot():/machine/limine_history/ryoku_linux.efi_sha256_x#abc

/Ryoku Linux (CachyOS)
    protocol: linux
    kernel_path: boot():/vmlinuz-linux-cachyos
    module_path: boot():/initramfs-linux-cachyos.img

/EFI fallback
    protocol: efi
    path: boot():/EFI/BOOT/BOOTX64.EFI

/Windows
    protocol: efi_chainload
    image_path: guid(1234):/EFI/Microsoft/Boot/bootmgfw.efi
`

func onESP(present ...string) func(string) bool {
	have := map[string]bool{}
	for _, p := range present {
		have[p] = true
	}
	return func(p string) bool { return have[p] }
}

func TestVoidRyokuUEFIBootEntry(t *testing.T) {
	out := "Boot0007* Ryoku\tHD(1,GPT,...)/File(\\EFI\\ryoku\\BOOTX64.EFI)\n"
	if !hasRyokuBootEntry(out) {
		t.Fatal("Void's active Ryoku loader entry was not recognized")
	}
	if hasRyokuBootEntry(strings.Replace(out, "Boot0007*", "Boot0007 ", 1)) {
		t.Fatal("an inactive Void loader entry was treated as active")
	}
}

func TestLimineDeadEntries(t *testing.T) {
	exists := onESP(
		"/boot/EFI/Linux/ryoku_linux.efi",
		"/boot/machine/limine_history/ryoku_linux.efi_sha256_x",
		"/boot/EFI/BOOT/BOOTX64.EFI",
	)
	dead := limineDeadEntries(limineTopEntries(limineAdoptedConf, "/boot"), exists)
	if len(dead) != 1 || dead[0] != "Ryoku Linux (CachyOS)" {
		t.Fatalf("dead = %v, want exactly the flat entry whose kernel image is missing", dead)
	}
}

func TestLimineDropEntriesKeepsTheRest(t *testing.T) {
	out, changed := limineDropEntries(limineAdoptedConf, "/boot", []string{"Ryoku Linux (CachyOS)"})
	if !changed {
		t.Fatal("dropping a dead entry must report a change")
	}
	if strings.Contains(out, "(CachyOS)") || strings.Contains(out, "vmlinuz-linux-cachyos") {
		t.Errorf("the dead entry or its body survived:\n%s", out)
	}
	for _, keep := range []string{
		"timeout: 3",
		"/Ryoku Linux\n",
		"//linux",
		"////linux",
		"/EFI fallback",
		"/Windows",
		"guid(1234):/EFI/Microsoft/Boot/bootmgfw.efi",
	} {
		if !strings.Contains(out, keep) {
			t.Errorf("%q was lost:\n%s", keep, out)
		}
	}
	if again, changed := limineDropEntries(out, "/boot", nil); changed || again != out {
		t.Error("a config with nothing dead must be a fixed point")
	}
}

// A default_entry naming the entry being removed would loop the countdown, so
// it has to move onto a kernel that still exists.
func TestLimineDropEntriesRepointsTheDefault(t *testing.T) {
	conf := strings.Replace(limineAdoptedConf,
		"default_entry: Ryoku Linux/linux",
		"default_entry: Ryoku Linux (CachyOS)", 1)
	out, changed := limineDropEntries(conf, "/boot", []string{"Ryoku Linux (CachyOS)"})
	if !changed {
		t.Fatal("expected the removal to change the config")
	}
	if strings.Contains(out, "default_entry: Ryoku Linux (CachyOS)") {
		t.Errorf("default_entry still names the removed entry:\n%s", out)
	}
	if !strings.Contains(out, "default_entry: Ryoku Linux/linux") {
		t.Errorf("default_entry was not repointed at a kernel that exists:\n%s", out)
	}
}
