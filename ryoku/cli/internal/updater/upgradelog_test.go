package updater

import (
	"os"
	"strings"
	"testing"
)

func TestConflictPath(t *testing.T) {
	cases := map[string]string{
		"noto-fonts: /usr/share/fontconfig/conf.avail/46-noto-sans.conf exists in filesystem":          "/usr/share/fontconfig/conf.avail/46-noto-sans.conf",
		"ryoku-desktop: /usr/lib/systemd/system/ryoku-network-kill-guard.service exists in filesystem": "/usr/lib/systemd/system/ryoku-network-kill-guard.service",
		"foo: /a/b exists in filesystem (owned by bar)":                                                "/a/b",
		" downloading...": "",
		"error: failed to commit transaction (conflicting files)": "",
		":: Proceed with installation? [Y/n]":                     "",
	}
	for line, want := range cases {
		if got := conflictPath(line); got != want {
			t.Errorf("conflictPath(%q) = %q, want %q", line, got, want)
		}
	}
}

// A failed transaction hands back what the healing retry needs: the conflict
// paths pacman named, and its own error lines in the returned error instead of
// a bare "exit status 1".
func TestTransactionCollectsConflictsAndReasons(t *testing.T) {
	oldInhibit := sleepInhibitOK
	sleepInhibitOK = func() bool { return false }
	t.Cleanup(func() { sleepInhibitOK = oldInhibit })

	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Skip(err)
	}
	keep := os.Stdout
	os.Stdout = devnull
	script := `printf '%s\n' '(1/2) upgrading ryoku' 'ryoku-desktop: /usr/share/plymouth/themes/ryoku/bullet.png exists in filesystem' 'error: failed to commit transaction (conflicting files)' 'error: failed to commit transaction (conflicting files)'; exit 1`
	conflicts, err := runUpgradeCollecting("test", []string{"bash", "-c", script})
	os.Stdout = keep
	devnull.Close()

	if len(conflicts) != 1 || conflicts[0] != "/usr/share/plymouth/themes/ryoku/bullet.png" {
		t.Errorf("conflicts = %q, want the one pacman conflict path", conflicts)
	}
	if err == nil || strings.Count(err.Error(), "failed to commit transaction") != 1 {
		t.Errorf("error = %v, want pacman's reason exactly once", err)
	}
}

func TestVoidInhibitorUsesHostSeam(t *testing.T) {
	oldInhibit := sleepInhibitOK
	sleepInhibitOK = func() bool { return true }
	t.Cleanup(func() { sleepInhibitOK = oldInhibit })
	t.Setenv("RYOKU_HOST_INIT", "runit")

	got := strings.Join(inhibited("package update", []string{"xbps-install", "-Syu"}), " ")
	want := "ryoku-host inhibit --what=sleep:idle --who=ryoku update --why=package update --mode=block xbps-install -Syu"
	if got != want || strings.Contains(got, "systemd") {
		t.Fatalf("inhibited command = %q, want %q", got, want)
	}
}
