package updater

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// a transcript shaped like a real `pacman -Syu`, including the mixed-repo
// "newer than" chatter and a file-conflict failure like the plymouth one.
const pacmanTranscript = `:: Synchronizing package databases...
 cachyos-v3 is up to date
 core is up to date
 ryoku is up to date
:: Starting full system upgrade...
warning: binutils: local (2.47-4) is newer than cachyos-v3 (2.47-2)
warning: zstd: local (1.5.7-3) is newer than cachyos-v3 (1.5.7-2)
resolving dependencies...
looking for conflicting packages...

Packages (37) blesh-0.4.0_devel3-1  colord-1.4.8-1  ryoku-desktop-0.49.9.r2972.gdc7b811-2
              ryoku-shell-0.49.9.r2972.gdc7b811-1  wine-11.16-2.1  zsh-5.9.2-1

Total Installed Size:  1903.15 MiB
Net Upgrade Size:      64.09 MiB

(37/37) checking keys in keyring
(37/37) checking package integrity
(1/37) upgrading blesh
(2/37) upgrading colord
(37/37) upgrading zsh
error: failed to commit transaction (conflicting files)
ryoku-desktop: /usr/share/plymouth/themes/ryoku/bullet.png exists in filesystem
Errors occurred, no packages were upgraded.`

func renderTranscript(t *testing.T, transcript string, ok bool) string {
	t.Helper()
	var buf bytes.Buffer
	r := newUpgradeRenderer(&buf, "System", false) // animate=false: deterministic
	for _, ln := range strings.Split(transcript, "\n") {
		r.feed(ln)
	}
	r.finish(ok)
	return buf.String()
}

func TestRendererHidesNoise(t *testing.T) {
	out := renderTranscript(t, pacmanTranscript, false)
	for _, noise := range []string{
		"Synchronizing", "is up to date", "is newer than",
		"resolving dependencies", "looking for conflicting",
		"checking keys", "checking package integrity", "upgrading blesh",
	} {
		if strings.Contains(out, noise) {
			t.Errorf("curated output leaked noise %q:\n%s", noise, out)
		}
	}
}

func TestRendererKeepsSignal(t *testing.T) {
	out := renderTranscript(t, pacmanTranscript, false)
	for _, want := range []string{
		"System packages",       // header
		"37 packages",           // collapsed summary
		"64.09 MiB",             // net size in the summary
		"ryoku-desktop",         // ryoku highlight (version stripped)
		"ryoku-shell",           // ryoku highlight
		"exists in filesystem",  // conflict detail is surfaced
		"failed to commit",      // real error is surfaced
		"System upgrade failed", // failure cap
	} {
		if !strings.Contains(out, want) {
			t.Errorf("curated output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "ryoku-desktop-0.49") {
		t.Errorf("version not stripped from ryoku highlight:\n%s", out)
	}
}

func TestRendererSuccessCap(t *testing.T) {
	out := renderTranscript(t, pacmanTranscript, true)
	if !strings.Contains(out, "✓") || !strings.Contains(out, "37 upgraded") {
		t.Errorf("expected success cap with count, got:\n%s", out)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		line string
		want lineClass
	}{
		{" core is up to date", clDrop},
		{"warning: zstd: local (1.5.7-3) is newer than cachyos-v3 (1.5.7-2)", clDrop},
		{"warning: /etc/foo.conf installed as /etc/foo.conf.pacnew", clWarn},
		{"error: failed to commit transaction (conflicting files)", clError},
		{"ryoku-desktop: /usr/share/plymouth/themes/ryoku/logo.png exists in filesystem", clConflict},
		{"(12/37) upgrading wine", clProgress},
		{":: Running post-transaction hooks...", clProgress},
	}
	for _, c := range cases {
		if got := classify(c.line, strings.TrimSpace(c.line)); got != c.want {
			t.Errorf("classify(%q) = %d, want %d", c.line, got, c.want)
		}
	}
}

func TestSizeValueAndCount(t *testing.T) {
	if got := sizeValue("Net Upgrade Size:      64.09 MiB"); got != "64.09 MiB" {
		t.Errorf("sizeValue = %q, want 64.09 MiB", got)
	}
	if got := countUpgrade("(9/37) upgrading wine", 3); got != 9 {
		t.Errorf("countUpgrade = %d, want 9", got)
	}
	if got := countUpgrade("(2/37) checking integrity", 9); got != 9 {
		t.Errorf("countUpgrade should ignore non-upgrade step, got %d", got)
	}
}

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

// TestPipedRunIsCurated pins the consumer the Hub's update island and
// `ryoku update > log` represent: stdout is a pipe, so the curated view (never
// the raw pacman firehose) must reach it, while the conflict paths are still
// collected for the stray-clearing retry.
func TestPipedRunIsCurated(t *testing.T) {
	oldInhibit := sleepInhibitOK
	sleepInhibitOK = func() bool { return false }
	t.Cleanup(func() { sleepInhibitOK = oldInhibit })

	capR, capW, err := os.Pipe()
	if err != nil {
		t.Skipf("no pipe: %v", err)
	}
	keep := os.Stdout
	os.Stdout = capW
	script := `printf '%s\n' ':: Synchronizing package databases...' ' core is up to date' 'Packages (2) ryoku-1-1 ryoku-desktop-2-2' '' 'Net Upgrade Size: 1.00 MiB' '(1/2) upgrading ryoku' 'warning: /etc/foo.conf installed as /etc/foo.conf.pacnew' 'ryoku-desktop: /usr/share/plymouth/themes/ryoku/bullet.png exists in filesystem'`
	conflicts, _ := runUpgradeCollecting("System", "test", []string{"bash", "-c", script})
	capW.Close()
	os.Stdout = keep
	out, _ := io.ReadAll(capR)
	capR.Close()
	got := string(out)

	for _, noise := range []string{"Synchronizing", "is up to date", "upgrading ryoku"} {
		if strings.Contains(got, noise) {
			t.Errorf("piped output leaked firehose line %q:\n%s", noise, got)
		}
	}
	for _, want := range []string{"System packages", ".pacnew", "exists in filesystem"} {
		if !strings.Contains(got, want) {
			t.Errorf("piped output missing signal %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\r") {
		t.Errorf("piped output carries spinner carriage returns:\n%q", got)
	}
	if len(conflicts) != 1 || conflicts[0] != "/usr/share/plymouth/themes/ryoku/bullet.png" {
		t.Errorf("conflicts = %q, want the one pacman conflict path", conflicts)
	}
}
