package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// What the run prints reaches the curated view only in the few shapes a person
// must see; the rest stays in the log.
func TestClassifyCapturedKeepsSignalOnly(t *testing.T) {
	type want struct {
		kind noteKind
		text string
		ok   bool
	}
	cases := []struct {
		step, line string
		want       want
	}{
		{"packages", "error: failed to commit transaction (conflicting files)", want{noteErr, "failed to commit transaction (conflicting files)", true}},
		{"packages", "warning: /etc/foo.conf installed as /etc/foo.conf.pacnew", want{noteWarn, "/etc/foo.conf installed as /etc/foo.conf.pacnew", true}},
		{"packages", "warning: zstd: local (1.5.7-3) is newer than cachyos-v3 (1.5.7-2)", want{}},
		{"snapshot", "note: snapper not installed; skipping pre-update snapshot", want{noteInfo, "snapper not installed; skipping pre-update snapshot", true}},
		{"deploy", "CMake Warning at /usr/lib/cmake/Qt6/QtPublicDependencyHelpers.cmake:339 (message):", want{}},
		{"deploy", "installed /home/u/.local/bin/ryoku-shell", want{}},
		{"packages", "core is up to date", want{}},
		// pacman's --noconfirm conflict, verbatim from a CachyOS run: the
		// question shares its line with the next error, then the pair repeats
		// after the summary. Both read as the same pair, so one note.
		{"packages", ":: limine-mkinitcpio-hook-1.37.1-1 and limine-entry-tool-1.40.0-1 are in conflict. Remove limine-entry-tool? [y/N] error: unresolvable package conflicts detected", want{noteErr, "limine-mkinitcpio-hook-1.37.1-1 and limine-entry-tool-1.40.0-1 are in conflict", true}},
		{"packages", ":: limine-mkinitcpio-hook-1.37.1-1 and limine-entry-tool-1.40.0-1 are in conflict", want{noteErr, "limine-mkinitcpio-hook-1.37.1-1 and limine-entry-tool-1.40.0-1 are in conflict", true}},
		// the question can name the virtual package behind the conflict while
		// the summary line does not; both are the same pair to a user.
		{"packages", ":: rust-1:1.99.0-1 and rustup-1.29.1-1 are in conflict (cargo). Remove rustup? [y/N] error: unresolvable package conflicts detected", want{noteErr, "rust-1:1.99.0-1 and rustup-1.29.1-1 are in conflict", true}},
		{"packages", ":: unable to satisfy dependency 'qt6-base=6.12.0' required by quickshell", want{noteErr, "unable to satisfy dependency 'qt6-base=6.12.0' required by quickshell", true}},
		{"packages", ":: Synchronizing package databases...", want{}},
		{"packages", ":: Proceed with installation? [Y/n]", want{}},
		{"doctor", "! orphaned packages", want{noteWarn, "orphaned packages", true}},
		{"doctor", "✓ stale pacman lock (fixed)", want{noteOK, "stale pacman lock (fixed)", true}},
		{"doctor", "✓ all checks passed", want{noteOK, "all checks passed", true}},
		{"doctor", "✓ swap is out of snapshots", want{}},
		{"deploy", "! not a doctor finding", want{}},
		// a doctor finding about .pacnew files is one warning, not two
		{"doctor", "! pending config (.pacnew)", want{noteWarn, "pending config (.pacnew)", true}},
		{"doctor", "2 pending config update(s) (.pacnew) need review", want{}},
	}
	for _, c := range cases {
		kind, text, ok := classifyCaptured(c.step, c.line)
		if ok != c.want.ok || (ok && (kind != c.want.kind || text != c.want.text)) {
			t.Errorf("classifyCaptured(%q, %q) = (%v, %q, %v), want (%v, %q, %v)",
				c.step, c.line, kind, text, ok, c.want.kind, c.want.text, c.want.ok)
		}
	}
}

func TestPieceLineLayoutKeepsStatusVisible(t *testing.T) {
	label := "Ryoku.Blobs, the frame renderer"
	right := "━━━━━━╸──────  62%  8s / ~12s"
	for _, width := range []int{100, 50} {
		line := layoutPieceLine("├─", "⠹", label, right, width)
		got := line.text()
		if utf8.RuneCountInString(got) > width {
			t.Errorf("width %d line is %d columns: %q", width, utf8.RuneCountInString(got), got)
		}
		if !strings.HasSuffix(got, right) {
			t.Errorf("width %d lost right side: %q", width, got)
		}
		if width == 50 && !strings.Contains(line.label, "…") {
			t.Errorf("width %d did not clip label: %q", width, got)
		}
		if width == 100 && line.label != label {
			t.Errorf("width %d clipped label: %q", width, got)
		}
	}
}

func TestClosingPieceLineText(t *testing.T) {
	ok := closingPieceLine(stepDone, 4, "", "12s", 100).text()
	if !strings.Contains(ok, "╰─ ✓  4 pieces built") || !strings.HasSuffix(ok, "12s") {
		t.Errorf("ok close = %q", ok)
	}
	failed := closingPieceLine(stepFailed, 3, "Ryoku.Blobs", "8s", 50).text()
	if !strings.Contains(failed, "╰─ ✗  stopped while building Ryoku.Blobs") || !strings.HasSuffix(failed, "8s") {
		t.Errorf("failed close = %q", failed)
	}
	if utf8.RuneCountInString(failed) > 50 {
		t.Errorf("failed close is %d columns: %q", utf8.RuneCountInString(failed), failed)
	}
}

func TestVerdictThresholds(t *testing.T) {
	cases := []struct {
		quiet time.Duration
		want  string
	}{
		{0, "working"},
		{quietAfter - time.Second, "working"},
		{quietAfter, "quiet"},
		{stallAfter - time.Second, "quiet"},
		{stallAfter, "stalled"},
	}
	for _, c := range cases {
		if got := verdict(c.quiet, "pacman", true); got.State != c.want || !got.Pkg || got.Current != "pacman" {
			t.Errorf("verdict(%v) = %+v, want state %s", c.quiet, got, c.want)
		}
	}
}

// The watchdog names the work a child is doing, not the wrapper around it, and
// flags a package manager in the tree. The fake package manager is a shell
// under the name pacman (comm follows the exec'd path's basename) blocked on a
// read from a pipe the test holds open, so it is alive with no child of its
// own. A renamed sleep would not do: on uutils coreutils (Ubuntu 26.04
// runners) sleep is a multicall binary that dispatches on argv[0].
func TestWatchdogNamesTheWorkInItsTree(t *testing.T) {
	dir := t.TempDir()
	pacman := filepath.Join(dir, "pacman")
	if err := os.Symlink("/bin/sh", pacman); err != nil {
		t.Skip(err)
	}
	cmd := exec.Command("sh", "-c", pacman+" -c 'read _' 5")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() })
	deadline := time.Now().Add(2 * time.Second)
	var ws watchState
	for time.Now().Before(deadline) {
		ws = newWatchdog(os.Getpid()).sample(nowMs())
		if ws.Pkg {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ws.Pkg || !strings.HasPrefix(ws.Current, "pacman ") {
		t.Errorf("sample = %+v, want the pacman child named and flagged", ws)
	}
	if ws.State != "working" {
		t.Errorf("fresh activity read as %q", ws.State)
	}
}

// Under capture a child's stdout and stderr land in the log and reach the tail
// line by line, and release gives the descriptors back.
func TestCaptureRoutesChildOutputToTheLog(t *testing.T) {
	logf, err := os.OpenFile(filepath.Join(t.TempDir(), "update-log.txt"), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer logf.Close()
	before, _ := os.Readlink("/proc/self/fd/1")

	var mu sync.Mutex
	var seen []string
	c := startCapture(logf, func(s string) { mu.Lock(); seen = append(seen, s); mu.Unlock() })
	if c == nil {
		t.Skip("descriptors cannot be redirected here")
	}
	run := exec.Command("sh", "-c", `printf 'out line\r\n'; printf 'err \033[1mline\033[0m\n' >&2`)
	run.Stdout, run.Stderr = os.Stdout, os.Stderr
	runErr := run.Run()
	c.release()
	after, _ := os.Readlink("/proc/self/fd/1")
	if runErr != nil {
		t.Fatal(runErr)
	}

	b, _ := os.ReadFile(logf.Name())
	if !strings.Contains(string(b), "out line") || !strings.Contains(string(b), "err") {
		t.Errorf("log = %q, want both streams", b)
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(seen, "|") != "out line|err line" {
		t.Errorf("tail = %q, want each line once, escapes stripped", seen)
	}
	if after != before {
		t.Errorf("fd 1 after release = %q, want %q back", after, before)
	}
}
