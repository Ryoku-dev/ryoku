package updater

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"
)

// Package transactions and the per-update raw log. A transaction streams its
// output untouched to stdout: inside an update the console has pointed stdout
// at the raw log and shows its own curated view (console.go), and with
// `ryoku update -v` (or `ryoku track`) the raw stream is the point. The scan
// in between only collects what a failed transaction needs to heal or explain
// itself.

// verboseLog forces raw output instead of the curated console; set from
// `ryoku update --verbose` / `-v`.
var verboseLog bool

// sleepInhibitOK probes once whether this session may take a sleep/idle
// inhibitor. A denied inhibitor must never prevent the wrapped package
// transaction from running.
var sleepInhibitOK = sync.OnceValue(func() bool {
	if updateInitSystem() == host.Runit {
		if !sys.Has("ryoku-host") {
			return false
		}
		return exec.Command("ryoku-host", "inhibit", "--what=sleep:idle",
			"--who=ryoku update", "--why=probe", "--mode=block", "true").Run() == nil
	}
	if !sys.Has("systemd-inhibit") {
		return false
	}
	return exec.Command("systemd-inhibit", "--what=sleep:idle",
		"--who=ryoku update", "--why=probe", "--mode=block", "true").Run() == nil
})

// runUpgradeCollecting runs a package transaction sleep-inhibited, streaming
// its output, and returns the "exists in filesystem" conflict paths pacman
// reported, so a failed upgrade can clear the strays no package owns and
// retry.
func runUpgradeCollecting(why string, argv []string) ([]string, error) {
	full := inhibited(why, argv)
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, sys.Run(full[0], full[1:]...)
	}
	cmd := exec.Command(full[0], full[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, pw, pw
	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return nil, err
	}
	pw.Close()
	var conflicts []string
	var reasons []string
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	sc.Split(scanLinesCR)
	for sc.Scan() {
		line := sc.Text()
		if p := conflictPath(line); p != "" {
			conflicts = append(conflicts, p)
		}
		if msg := pacmanError(line); msg != "" {
			reasons = append(reasons, msg)
		}
		fmt.Fprintln(os.Stdout, line)
	}
	pr.Close()
	werr := cmd.Wait()
	// pacman exits 1 for every failure, so werr alone reads "exit status 1" and
	// tells a user nothing: the reason lives in the `error:` lines it printed.
	// Carrying them into the returned error is what makes the Hub's failure
	// card, the console's closing card, and the db-signature healing in
	// healPackageUpgrade able to say what actually went wrong.
	if werr != nil && len(reasons) > 0 {
		werr = fmt.Errorf("%s: %w", strings.Join(dedupe(reasons), "; "), werr)
	}
	return conflicts, werr
}

// inhibited wraps argv in a logind sleep+idle block, so a suspend mid-upgrade
// cannot interrupt a package transaction; argv runs bare when the session may
// not take the block.
func inhibited(why string, argv []string) []string {
	if !sleepInhibitOK() {
		return argv
	}
	prefix := []string{"systemd-inhibit"}
	if updateInitSystem() == host.Runit {
		prefix = []string{"ryoku-host", "inhibit"}
	}
	prefix = append(prefix, "--what=sleep:idle",
		"--who=ryoku update", "--why="+why, "--mode=block")
	return append(prefix, argv...)
}

// runInhibited runs a transaction whose conflicts nobody heals (the system
// lane, the AUR, Flatpak, a channel move). With -v it keeps the terminal, so
// pacman draws its own progress bars.
func runInhibited(why string, argv []string) error {
	if verboseLog {
		full := inhibited(why, argv)
		return sys.Run(full[0], full[1:]...)
	}
	_, err := runUpgradeCollecting(why, argv)
	return err
}

// pacmanError pulls the message out of a pacman `error:` line. Empty for any
// other line. pure.
func pacmanError(line string) string {
	trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	for _, prefix := range []string{"error:", "erreur :"} {
		if rest, ok := strings.CutPrefix(trimmed, prefix); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// dedupe keeps first occurrences, so a transaction that repeats one error per
// package reports it once. pure.
func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// conflictPath pulls the path from a pacman file-conflict line, e.g.
// "noto-fonts: /usr/share/fontconfig/conf.avail/46-noto-sans.conf exists in
// filesystem" -> the path. Empty for any other line.
func conflictPath(line string) string {
	const marker = " exists in filesystem"
	i := strings.Index(line, marker)
	if i < 0 {
		return ""
	}
	head := strings.TrimSpace(line[:i])
	c := strings.LastIndex(head, ": ")
	if c < 0 {
		return ""
	}
	if p := strings.TrimSpace(head[c+2:]); strings.HasPrefix(p, "/") {
		return p
	}
	return ""
}

// scanLinesCR splits on either newline or carriage return, so each redraw of a
// progress bar arrives as its own token instead of one buffered mega-line.
func scanLinesCR(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// updateLogFile is the open per-update raw log while a run lasts.
var updateLogFile *os.File

// updateLogPath is where the per-update raw log lives.
func updateLogPath() string {
	return filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "update-log.txt")
}

// startUpdateLog opens the per-update raw log. The process that starts the
// update truncates; the stage2 hand-off appends, so the package transaction's
// output survives into the deploy and doctor steps. Returns the path, or ""
// when it cannot be created (logging is best-effort).
func startUpdateLog(appendMode bool) string {
	path := updateLogPath()
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return ""
	}
	stopUpdateLog()
	flags := os.O_CREATE | os.O_RDWR | os.O_APPEND
	if !appendMode {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return ""
	}
	updateLogFile = f
	return path
}

func stopUpdateLog() {
	if updateLogFile != nil {
		updateLogFile.Close()
		updateLogFile = nil
	}
}

// logNarrative mirrors a narrative line into the raw log, marked so the
// console's tail knows it is ours, so the log reads as the run did.
func logNarrative(line string) {
	if updateLogFile != nil {
		fmt.Fprintln(updateLogFile, "▸ "+line)
	}
}
