package updater

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

// The package manager firehose (database sync "up to date", "newer than"
// downgrade warnings, the resolving/conflict-check chatter, per-file progress
// bars, and the full wrapped package dump) buries the handful of lines a person
// running `ryoku update` actually cares about: what is upgrading, how far along
// it is, and any real warning or error. renderUpgrade runs an upgrade command
// with its output piped and rebuilds it as a curated, styled view -- a phase
// header, a one-line package summary, an in-place spinner, and untouched
// surfacing of warnings and errors. It only runs on a real terminal (the caller
// keeps raw passthrough for pipes, logs, and --verbose), so nothing that scrapes
// pacman output breaks, and the command's exit status is passed straight back.

// verboseLog forces raw package-manager output instead of the curated view; set
// from `ryoku update --verbose` / `-v` for debugging a transaction.
var verboseLog bool

var (
	pkgHeaderRe = regexp.MustCompile(`^Packages \((\d+)\)\s*(.*)$`)
	stepRe      = regexp.MustCompile(`^\((\d+)/(\d+)\)\s+(.*)$`)
	verStripRe  = regexp.MustCompile(`-[0-9][^-]*(-[^-\s]+)?$`)
	spinFrames  = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
)

// renderUpgrade runs argv, rendering a curated view of its output. phase is the
// short label for the header ("System", "AUR", "Flatpak").
func renderUpgrade(phase string, argv []string) error {
	// sudo is primed once up front by the update (primeSudo), so the piped
	// transaction here never blocks on an unseen password prompt.
	pr, pw, err := os.Pipe()
	if err != nil {
		return sys.Run(argv[0], argv[1:]...)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, pw, pw
	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return err
	}
	pw.Close()
	r := newUpgradeRenderer(os.Stdout, phase, sys.StdoutIsTTY())
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	sc.Split(scanLinesCR)
	for sc.Scan() {
		logRaw(sc.Text())
		r.feed(sc.Text())
	}
	pr.Close()
	werr := cmd.Wait()
	r.finish(werr == nil)
	return werr
}

// sleepInhibitOK probes once whether this session may take a sleep/idle
// inhibitor. Taking one is polkit-gated in sessions with no agent (SSH, a
// headless run), where systemd-inhibit exits "Access denied" BEFORE the wrapped
// command runs: wrapping there turned a best-effort guard into a failed
// transaction, with pacman never invoked. Probe once per process; on a denial
// the transaction runs unwrapped instead of failing the update.
var sleepInhibitOK = sync.OnceValue(func() bool {
	if !sys.Has("systemd-inhibit") {
		return false
	}
	return exec.Command("systemd-inhibit", "--what=sleep:idle",
		"--who=ryoku update", "--why=probe", "--mode=block", "true").Run() == nil
})

// runUpgradeCollecting runs a package transaction sleep-inhibited and rendered
// (or streamed raw for pipes/--verbose) exactly like renderUpgrade, and returns
// the "exists in filesystem" conflict paths pacman reported, so a failed upgrade
// can clear the strays no package owns and retry. Both views scan the same
// stream, so the collection is identical on a TTY and in a log.
func runUpgradeCollecting(phase, why string, argv []string) ([]string, error) {
	full := argv
	if sleepInhibitOK() {
		full = append([]string{"systemd-inhibit", "--what=sleep:idle",
			"--who=ryoku update", "--why=" + why, "--mode=block"}, argv...)
	}
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
	// Curated for every consumer, animated only for a terminal: the Hub's update
	// island and `ryoku update > log` read stdout through a pipe, where the raw
	// pacman firehose (package lists, progress redraws, conflict paths) reads as
	// a broken install. The full firehose still lands in update-log.txt, and
	// --verbose keeps the raw passthrough on stdout.
	rendered := !verboseLog
	var r *upgradeRenderer
	if rendered {
		r = newUpgradeRenderer(os.Stdout, phase, sys.StdoutIsTTY())
	}
	var conflicts []string
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	sc.Split(scanLinesCR)
	for sc.Scan() {
		line := sc.Text()
		logRaw(line)
		if p := conflictPath(line); p != "" {
			conflicts = append(conflicts, p)
		}
		if rendered {
			r.feed(line)
		} else {
			fmt.Fprintln(os.Stdout, line)
		}
	}
	pr.Close()
	werr := cmd.Wait()
	if rendered {
		r.finish(werr == nil)
	}
	return conflicts, werr
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

// rawLog, set for the duration of an update, receives every unstyled line the
// renderers consume, so the full firehose is preserved for review even though
// the terminal shows only the curated view.
var rawLog io.Writer

func logRaw(line string) {
	if rawLog != nil {
		fmt.Fprintln(rawLog, line)
	}
}

var updateLogFile *os.File

// startUpdateLog opens the per-update raw log (overwritten each run) and points
// rawLog at it, so a curated run still leaves the full firehose to read. Returns
// the path, or "" when it cannot be created (logging is best-effort).
func startUpdateLog() string {
	path := filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "update-log.txt")
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return ""
	}
	f, err := os.Create(path)
	if err != nil {
		return ""
	}
	updateLogFile = f
	rawLog = f
	return path
}

func stopUpdateLog() {
	rawLog = nil
	if updateLogFile != nil {
		updateLogFile.Close()
		updateLogFile = nil
	}
}

// liveSpinner animates on a timer, so a long silent step (a plugin compile that
// prints nothing for a minute) keeps spinning instead of looking hung. set()
// swaps the label, the ticker redraws, perm() prints a line above the spinner.
// Every write is serialized through the mutex.
type liveSpinner struct {
	mu     sync.Mutex
	w      io.Writer
	label  string
	frame  int
	active bool
	stop   chan struct{}
	done   chan struct{}
}

func newLiveSpinner(w io.Writer) *liveSpinner {
	s := &liveSpinner{w: w, stop: make(chan struct{}), done: make(chan struct{})}
	go s.loop()
	return s
}

func (s *liveSpinner) loop() {
	t := time.NewTicker(120 * time.Millisecond)
	defer t.Stop()
	defer close(s.done)
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.mu.Lock()
			s.draw()
			s.mu.Unlock()
		}
	}
}

// draw redraws the current label in place; the caller holds the lock.
func (s *liveSpinner) draw() {
	if s.label == "" {
		return
	}
	f := string(spinFrames[s.frame%len(spinFrames)])
	s.frame++
	label := s.label
	if w := sys.TermWidth() - 1; len(label)+4 > w && w > 4 {
		label = label[:w-4]
	}
	fmt.Fprint(s.w, "\r\033[K  "+sys.Brand(f)+" "+sys.Dim(label))
	s.active = true
}

func (s *liveSpinner) set(label string) {
	s.mu.Lock()
	s.label = label
	s.draw() // show the new step at once, without waiting for the next tick
	s.mu.Unlock()
}

func (s *liveSpinner) perm(line string) {
	s.mu.Lock()
	if s.active {
		fmt.Fprint(s.w, "\r\033[K")
		s.active = false
	}
	fmt.Fprintln(s.w, line)
	s.mu.Unlock()
}

func (s *liveSpinner) close() {
	close(s.stop)
	<-s.done
	s.mu.Lock()
	if s.active {
		fmt.Fprint(s.w, "\r\033[K")
		s.active = false
	}
	s.mu.Unlock()
}

// renderQuiet runs argv showing a single in-place spinner of its current line
// rather than its full output, surfacing only warnings and errors. It suits a
// noisy-but-uninteresting step like deploy.sh (dozens of "building/installed"
// lines) whose phase header the caller already narrated. Exit status is passed
// through; a pipe failure degrades to raw passthrough.
func renderQuiet(argv []string) error {
	// sudo is primed once up front by the update (primeSudo); nothing to do here.
	pr, pw, err := os.Pipe()
	if err != nil {
		return sys.Run(argv[0], argv[1:]...)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, pw, pw
	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return err
	}
	pw.Close()
	sp := newLiveSpinner(os.Stdout)
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	sc.Split(scanLinesCR)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		logRaw(line)
		switch {
		case quietError(line):
			sp.perm("  " + sys.Red("✗ ") + line)
		case quietWarn(line):
			sp.perm("  " + sys.Amber("! ") + line)
		default:
			sp.set(line)
		}
	}
	pr.Close()
	werr := cmd.Wait()
	sp.close()
	return werr
}

func quietError(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "error") || strings.Contains(l, "failed") ||
		strings.Contains(l, "cannot ") || strings.HasPrefix(l, "fatal")
}

func quietWarn(s string) bool {
	l := strings.ToLower(s)
	return strings.HasPrefix(l, "warning") || strings.Contains(l, "not restarted") ||
		strings.Contains(l, "skipped") || strings.Contains(l, ".pacnew")
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

type upgradeRenderer struct {
	w        io.Writer
	phase    string
	animate  bool
	frame    int
	status   bool // an in-place status line is currently on screen
	inPkgs   bool // collecting the wrapped "Packages (N) ..." block
	pending  bool // package summary parsed, not yet printed (waiting for sizes)
	count    int
	size     string
	ryoku    []string
	upgraded int
}

func newUpgradeRenderer(w io.Writer, phase string, animate bool) *upgradeRenderer {
	r := &upgradeRenderer{w: w, phase: phase, animate: animate}
	r.perm(sys.Brand("▸ ") + sys.Bold(phase+i18n.T(" packages")))
	return r
}

func (r *upgradeRenderer) feed(raw string) {
	line := strings.TrimRight(raw, " \t")
	trimmed := strings.TrimSpace(line)

	// The wrapped package list runs from "Packages (N) ..." to the next blank
	// line; collect names for the count highlight, print nothing inline.
	if r.inPkgs {
		if trimmed == "" {
			// The size lines follow this blank line; defer the summary until the
			// first real activity so it can include the net size.
			r.inPkgs = false
			return
		}
		r.collectPkgs(trimmed)
		return
	}
	if m := pkgHeaderRe.FindStringSubmatch(line); m != nil {
		fmt.Sscanf(m[1], "%d", &r.count)
		r.inPkgs = true
		r.pending = true
		r.collectPkgs(m[2])
		return
	}
	if v := sizeValue(line); v != "" {
		r.size = v
		return
	}

	c := classify(line, trimmed)
	if c == clDrop {
		return
	}
	r.flush() // print the deferred package summary before the first real activity
	switch c {
	case clDrop:
		return
	case clProgress:
		if m := stepRe.FindStringSubmatch(trimmed); m != nil {
			r.upgraded = countUpgrade(trimmed, r.upgraded)
			r.spin(stepLabel(m[1], m[2], m[3]))
		} else {
			r.spin(i18n.T("working…"))
		}
	case clWarn:
		r.perm("  " + sys.Amber("! ") + sys.Dim(stripPrefix(trimmed, "warning:")))
	case clError:
		r.perm("  " + sys.Red("✗ ") + stripPrefix(trimmed, "error:"))
	case clConflict:
		r.perm("  " + sys.Red("  · ") + trimmed)
	}
}

func (r *upgradeRenderer) finish(ok bool) {
	r.clear()
	r.flush()
	if ok {
		tail := ""
		if r.upgraded > 0 {
			tail = fmt.Sprintf(i18n.T(" · %d upgraded"), r.upgraded)
		} else if r.count > 0 {
			tail = fmt.Sprintf(i18n.T(" · %d upgraded"), r.count)
		}
		r.perm(sys.Green("✓ ") + r.phase + tail)
		return
	}
	r.perm(sys.Red("✗ ") + r.phase + i18n.T(" upgrade failed"))
}

// --- rendering primitives ---------------------------------------------------

func (r *upgradeRenderer) perm(line string) {
	if r.status {
		fmt.Fprint(r.w, "\r\033[K")
		r.status = false
	}
	fmt.Fprintln(r.w, line)
}

func (r *upgradeRenderer) spin(label string) {
	if !r.animate {
		return
	}
	frame := string(spinFrames[r.frame%len(spinFrames)])
	r.frame++
	line := "  " + sys.Brand(frame) + " " + label
	if w := sys.TermWidth() - 1; len(label)+4 > w && w > 4 {
		line = "  " + sys.Brand(frame) + " " + label[:w-4]
	}
	fmt.Fprint(r.w, "\r\033[K"+line)
	r.status = true
}

func (r *upgradeRenderer) clear() {
	if r.status {
		fmt.Fprint(r.w, "\r\033[K")
		r.status = false
	}
}

func (r *upgradeRenderer) collectPkgs(s string) {
	for _, f := range strings.Fields(s) {
		if strings.HasPrefix(f, "ryoku") {
			if name := verStripRe.ReplaceAllString(f, ""); name != "" {
				r.ryoku = append(r.ryoku, name)
			}
		}
	}
}

// flush prints the deferred one-line package summary once, after the sizes have
// been parsed and before the first activity. A no-op until a package block was
// seen, and only once.
func (r *upgradeRenderer) flush() {
	if !r.pending {
		return
	}
	r.pending = false
	if r.count == 0 {
		return
	}
	line := fmt.Sprintf(i18n.T("  %s package%s"), sys.Bold(fmt.Sprint(r.count)), plural(r.count))
	if r.size != "" {
		line += sys.Dim(" · " + r.size)
	}
	r.perm(line)
	if len(r.ryoku) > 0 {
		r.perm("  " + sys.Dim(i18n.T("includes ")) + sys.Brand(strings.Join(r.ryoku, ", ")))
	}
}

// --- classification ---------------------------------------------------------

type lineClass int

const (
	clDrop lineClass = iota
	clProgress
	clWarn
	clError
	clConflict
)

func classify(line, trimmed string) lineClass {
	switch {
	case trimmed == "":
		return clDrop
	case strings.HasPrefix(trimmed, "error:"),
		strings.Contains(trimmed, "failed to commit"),
		strings.Contains(trimmed, "Errors occurred"):
		return clError
	case strings.Contains(trimmed, "exists in filesystem"),
		strings.Contains(trimmed, "exists in filesystem)"):
		return clConflict
	// .pacnew/.pacsave notices are the one warning class a user must act on.
	case strings.Contains(trimmed, ".pacnew"), strings.Contains(trimmed, ".pacsave"):
		return clWarn
	case strings.HasPrefix(trimmed, "warning:"):
		if strings.Contains(trimmed, " is newer than ") {
			return clDrop // local-newer downgrade chatter from mixed repos
		}
		return clWarn
	case stepRe.MatchString(trimmed),
		strings.HasPrefix(trimmed, ":: "),
		hasBar(trimmed),
		strings.HasPrefix(trimmed, "resolving dependencies"),
		strings.HasPrefix(trimmed, "looking for conflicting"):
		return clProgress
	case isUpToDate(trimmed):
		return clDrop
	}
	// Unknown, non-empty, non-noise: fold into the spinner rather than let an
	// unrecognised line break the curated view or leak the firehose.
	return clProgress
}

func isUpToDate(s string) bool { return strings.HasSuffix(s, " is up to date") }

func hasBar(s string) bool {
	return strings.Contains(s, "[") && (strings.Contains(s, "#") || strings.Contains(s, "-")) &&
		strings.Contains(s, "]") || strings.HasSuffix(s, "%")
}

func sizeValue(line string) string {
	for _, k := range []string{"Net Upgrade Size:", "Total Download Size:", "Total Installed Size:"} {
		if i := strings.Index(line, k); i >= 0 {
			return strings.TrimSpace(line[i+len(k):])
		}
	}
	return ""
}

func stepLabel(x, y, rest string) string {
	verb := strings.Fields(rest)
	if len(verb) == 0 {
		return fmt.Sprintf("%s/%s", x, y)
	}
	return fmt.Sprintf("%s (%s/%s)", strings.TrimSuffix(strings.Join(verb, " "), "..."), x, y)
}

// countUpgrade tracks the highest "(x/y) upgrading/installing" index seen, the
// closest cheap proxy for how many packages were applied.
func countUpgrade(step string, cur int) int {
	if !strings.Contains(step, "upgrading") && !strings.Contains(step, "installing") &&
		!strings.Contains(step, "reinstalling") {
		return cur
	}
	m := stepRe.FindStringSubmatch(step)
	if m == nil {
		return cur
	}
	n := 0
	fmt.Sscanf(m[1], "%d", &n)
	if n > cur {
		return n
	}
	return cur
}

func stripPrefix(s, prefix string) string {
	return strings.TrimSpace(strings.TrimPrefix(s, prefix))
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
