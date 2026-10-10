package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	wm "ryoku-wm"
)

// logs.go answers "app X broke, this happened" without the agent guessing. It
// resolves <target> to a Ryoku component or an arbitrary program and prints a
// bounded markdown bundle: units, the journal, coredumps, the app's own log
// files, and its package history, each section headed by the exact command or
// file it came from so the agent can dig further. Every probe is read-only,
// short-timeout, and never needs root.

const (
	logsDefaultSince = "6h"
	logsDefaultLines = 60
	// The whole bundle stays readable in one screen of an agent's context.
	logsTotalLineCap = 400
	// No probe is allowed to hang the bundle.
	logsProbeTimeout = 5
	// The path an app writes logs under is scanned this many directory levels
	// deep, and only the newest few files are tailed.
	logsFileDepth = 2
	logsFileCount = 4
	pacmanLogPath = "/var/log/pacman.log"
)

// logsConfig is the parsed window: how far back to look and how many lines to
// keep per source.
type logsConfig struct {
	since string
	lines int
}

// logSection is one probe's result. head is the exact command or file path it
// came from; body is already tail-bounded; note replaces an empty body.
type logSection struct {
	head string
	body string
	note string
}

// parseLogsArgs pulls the target and the --since/--lines flags out of argv.
// The target is every non-flag token joined by a space, so a stray sentence
// stays one (invalid) target rather than being silently truncated.
func parseLogsArgs(args []string) (target string, cfg logsConfig, err error) {
	cfg = logsConfig{since: logsDefaultSince, lines: logsDefaultLines}
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--since":
			if i+1 >= len(args) {
				return "", cfg, fmt.Errorf("--since needs a duration, e.g. 2h")
			}
			i++
			cfg.since = args[i]
		case "--lines":
			if i+1 >= len(args) {
				return "", cfg, fmt.Errorf("--lines needs a number")
			}
			i++
			n, e := strconv.Atoi(args[i])
			if e != nil || n <= 0 {
				return "", cfg, fmt.Errorf("--lines needs a positive number")
			}
			cfg.lines = n
		default:
			rest = append(rest, args[i])
		}
	}
	return strings.TrimSpace(strings.Join(rest, " ")), cfg, nil
}

// logsCommandTarget returns the single valid target when args are exactly one
// app or component name plus optional --since/--lines flags, and "" otherwise.
// The `rashin` passthrough uses it so `rashin logs firefox` runs the command
// while `rashin logs are huge, how do I clean them` falls through to an ask.
func logsCommandTarget(args []string) string {
	target, _, err := parseLogsArgs(args)
	if err != nil || !appName.MatchString(target) {
		return ""
	}
	return target
}

// cmdLogs is the `logs <target>` entry point. It always exits 0: a bad flag is
// the only error, and a target with nothing to show still prints what it checked.
func cmdLogs(args []string) error {
	target, cfg, err := parseLogsArgs(args)
	if err != nil {
		return err
	}
	if target == "" {
		return fmt.Errorf("usage: ryoku-rashin logs <target> [--since <duration>] [--lines <n>]")
	}
	if !appName.MatchString(target) {
		return fmt.Errorf("name one app, component, or unit (letters, digits, . _ + @ -), not a phrase or path")
	}
	fmt.Print(gatherLogs(target, cfg))
	return nil
}

// gatherLogs resolves the target and renders its bundle.
func gatherLogs(target string, cfg logsConfig) string {
	key := resolveComponent(target)
	var secs []logSection
	switch key {
	case "":
		secs = appLogSections(target, cfg)
	case "updates":
		secs = updatesSections(cfg)
	case "compositor":
		secs = compositorSections(cfg)
	default:
		secs = buildComponentSections(componentSpecFor(key), cfg)
	}
	return renderLogReport(target, key, cfg, secs)
}

// componentAliases maps the names a user types to a canonical Ryoku component.
// A miss falls through to arbitrary-app mode.
var componentAliases = map[string]string{
	"shell": "shell", "bar": "shell", "dock": "shell", "quickshell": "shell",
	"qs": "shell", "ryoku-shell": "shell", "widgets": "shell", "launcher": "shell",

	"wallpaper": "wallpaper", "ryogami": "wallpaper", "wall": "wallpaper", "livewall": "wallpaper",

	"rashin": "rashin", "ryoku-rashin": "rashin", "vault": "rashin", "dashboard": "rashin",

	"hub": "hub", "ryoku-hub": "hub", "settings": "hub",

	"idle": "idle", "lock": "idle", "lockscreen": "idle", "ryoku-idle": "idle", "qylock": "idle",

	"audio": "audio", "sound": "audio", "pipewire": "audio", "wireplumber": "audio", "pulse": "audio",

	"portal": "portals", "portals": "portals",

	"update": "updates", "updates": "updates", "doctor": "updates", "pacman": "updates", "dnf": "updates",

	"compositor": "compositor", "wm": "compositor", "window-manager": "compositor",
}

// resolveComponent is case-insensitive; "" means "not a known component".
func resolveComponent(target string) string {
	return componentAliases[strings.ToLower(strings.TrimSpace(target))]
}

// componentSpec declares where a Ryoku component's logs live, so the renderer
// stays generic. The special "updates" and "compositor" targets build their
// own sections instead.
type componentSpec struct {
	userUnits     []string
	userUnitGlobs []string
	comms         []string
	fileRoots     []string
	qsConfigs     []string // config-path substrings identifying a qs instance
	coredumps     []string
}

// componentSpecFor resolves a canonical component to its log sources. Paths are
// built from the live HOME at call time.
func componentSpecFor(key string) componentSpec {
	h := home()
	switch key {
	case "shell":
		return componentSpec{
			userUnits: []string{"ryoku-shell.service"},
			qsConfigs: []string{filepath.Join(h, ".config", "quickshell", "shell")},
			coredumps: []string{"quickshell", "ryoku-shell"},
		}
	case "wallpaper":
		return componentSpec{
			userUnits: []string{"ryogami.service"},
			fileRoots: []string{filepath.Join(h, ".cache", "ryogami")},
			comms:     []string{"ryogami"},
		}
	case "rashin":
		return componentSpec{
			userUnits: []string{"ryoku-rashin.service", "ryoku-prowl.service"},
			coredumps: []string{"ryoku-rashin"},
		}
	case "hub":
		return componentSpec{
			comms:     []string{"ryoku-hub"},
			coredumps: []string{"ryoku-hub", "quickshell"},
		}
	case "idle":
		return componentSpec{
			userUnits: []string{"ryoku-idle.service"},
			comms:     []string{"ryoku-idle"},
			coredumps: []string{"quickshell"},
		}
	case "audio":
		return componentSpec{
			userUnits: []string{"pipewire.service", "wireplumber.service", "pipewire-pulse.service"},
			comms:     []string{"pipewire", "wireplumber"},
		}
	case "portals":
		return componentSpec{
			userUnitGlobs: []string{"xdg-desktop-portal*"},
		}
	}
	return componentSpec{}
}

// buildComponentSections turns a spec into ordered sections.
func buildComponentSections(spec componentSpec, cfg logsConfig) []logSection {
	var secs []logSection
	for _, u := range spec.userUnits {
		secs = append(secs, journalUnit(true, u, cfg))
	}
	for _, g := range spec.userUnitGlobs {
		units := discoverUserUnits(g)
		if len(units) == 0 {
			secs = append(secs, logSection{head: "systemctl --user list-units " + g, note: "no matching units"})
			continue
		}
		for _, u := range units {
			secs = append(secs, journalUnit(true, u, cfg))
		}
	}
	for _, sub := range spec.qsConfigs {
		secs = append(secs, quickshellLogSection(sub, cfg))
	}
	for _, root := range spec.fileRoots {
		secs = append(secs, fileRootSections(root, cfg)...)
	}
	for _, c := range spec.comms {
		secs = append(secs, journalComm(true, c, cfg)...)
	}
	for _, name := range spec.coredumps {
		secs = append(secs, coredumpSections(name, cfg)...)
	}
	return secs
}

// updatesSections gathers the system-change trail: the doctor report, the last
// update run, and recent native package transactions.
func updatesSections(cfg logsConfig) []logSection {
	state := filepath.Join(home(), ".local", "state", "ryoku")
	secs := []logSection{
		fileSection(filepath.Join(state, "doctor-report.txt"), cfg),
		fileSection(filepath.Join(state, "update-log.txt"), cfg),
	}
	if logsPackageManager() == "dnf" {
		return append(secs, dnfHistorySection("", cfg))
	}
	if body := pacmanTail(cfg.lines); body != "" {
		secs = append(secs, logSection{head: pacmanLogPath + " (recent transactions)", body: body})
	} else {
		secs = append(secs, logSection{head: pacmanLogPath, note: "unreadable or empty"})
	}
	return secs
}

// compositorSections resolves the active window manager through the seam and
// searches by the detected name only, never a hardcoded compositor. When the
// seam offers no log path we fall back to the journal, coredumps, and the log
// files the compositor writes under its runtime dir, discovered at runtime.
func compositorSections(cfg logsConfig) []logSection {
	det := wm.Detect()
	if det.Name == "" {
		return []logSection{{head: "wm.Detect()", note: "no window-manager provider detected on this box"}}
	}
	var secs []logSection
	for _, comm := range commCandidates(det.Name) {
		secs = append(secs, journalComm(true, comm, cfg)...)
		secs = append(secs, journalComm(false, comm, cfg)...)
	}
	if sub := wm.ConfigDir(det.Name); sub != "" {
		if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
			secs = append(secs, fileRootSections(filepath.Join(rt, sub), cfg)...)
		}
	}
	for _, name := range commCandidates(det.Name) {
		secs = append(secs, coredumpSections(name, cfg)...)
	}
	return secs
}

// appLogSections is the arbitrary-program path: units, the journal by comm,
// coredumps, OOM kills, the app's own log files, and its package history.
func appLogSections(name string, cfg logsConfig) []logSection {
	comm := commName(name)
	var secs []logSection
	for _, u := range discoverUnits(name) {
		secs = append(secs, journalUnit(u.user, u.name, cfg))
	}
	secs = append(secs, journalComm(true, comm, cfg)...)
	secs = append(secs, journalComm(false, comm, cfg)...)
	secs = append(secs, coredumpSections(name, cfg)...)
	secs = append(secs, oomSection(name, cfg))
	secs = append(secs, fileRootSections2(appLogRoots(name), cfg)...)
	secs = append(secs, packageSections(name, cfg)...)
	secs = append(secs, flatpakSection(name))
	return secs
}

// journalSince turns the user's --since window into a form journalctl accepts.
// A bare Go duration ("6h", "90m") becomes a negative relative time ("-6h");
// an absolute or already-relative value ("yesterday", "-2h") passes through.
func journalSince(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-" + logsDefaultSince
	}
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		return s
	}
	if _, err := time.ParseDuration(s); err == nil {
		return "-" + s
	}
	return s
}

// journalUnit captures one systemd unit's recent journal, user or system scope.
func journalUnit(userScope bool, unit string, cfg logsConfig) logSection {
	var args []string
	if userScope {
		args = append(args, "--user")
	}
	args = append(args, "-u", unit, "--since", journalSince(cfg.since), "-n", strconv.Itoa(cfg.lines), "--no-pager", "-o", "short-iso")
	head := "journalctl " + strings.Join(args, " ")
	out, ok := probe(logsProbeTimeout, "journalctl", args...)
	if !ok {
		return logSection{head: head, note: "journalctl unavailable, or the system journal needs root"}
	}
	return logSection{head: head, body: tailLines(out, cfg.lines), note: "no entries in window"}
}

// journalComm captures a binary's journal by its recorded process name: the
// warnings and worse, then the tail regardless of priority.
func journalComm(userScope bool, comm string, cfg logsConfig) []logSection {
	var base []string
	scope := "system"
	if userScope {
		base = append(base, "--user")
		scope = "user"
	}
	match := "_COMM=" + comm
	warnArgs := append(append([]string{}, base...), match, "--since", journalSince(cfg.since), "-p", "warning", "-n", strconv.Itoa(cfg.lines), "--no-pager", "-o", "short-iso")
	tailArgs := append(append([]string{}, base...), match, "--since", journalSince(cfg.since), "-n", strconv.Itoa(cfg.lines), "--no-pager", "-o", "short-iso")
	var secs []logSection
	if out, ok := probe(logsProbeTimeout, "journalctl", warnArgs...); ok {
		secs = append(secs, logSection{head: "journalctl " + strings.Join(warnArgs, " "), body: tailLines(out, cfg.lines), note: "no warnings in window"})
	} else {
		secs = append(secs, logSection{head: "journalctl " + strings.Join(warnArgs, " "), note: "journalctl unavailable, or the " + scope + " journal needs root"})
	}
	if out, ok := probe(logsProbeTimeout, "journalctl", tailArgs...); ok {
		secs = append(secs, logSection{head: "journalctl " + strings.Join(tailArgs, " "), body: tailLines(out, cfg.lines), note: "no entries in window"})
	}
	return secs
}

// oomSection pulls the OOM-killer lines from the kernel log that mention the
// target, so a silent kill shows up.
func oomSection(name string, cfg logsConfig) logSection {
	args := []string{"-k", "--since", journalSince(cfg.since), "-g", "Killed process|Out of memory|oom-kill", "--no-pager", "-o", "short-iso"}
	head := "journalctl " + strings.Join(args, " ") + " (kept: lines naming " + name + ")"
	out, ok := probe(logsProbeTimeout, "journalctl", args...)
	if !ok {
		return logSection{head: head, note: "kernel journal unavailable or needs root"}
	}
	lower := strings.ToLower(name)
	var kept []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(strings.ToLower(l), lower) {
			kept = append(kept, l)
		}
	}
	return logSection{head: head, body: tailLines(strings.Join(kept, "\n"), cfg.lines), note: "no OOM kills naming " + name}
}

// coredumpSections lists a binary's coredumps and heads the newest one's info.
func coredumpSections(name string, cfg logsConfig) []logSection {
	listArgs := []string{"list", "--no-pager", name}
	listHead := "coredumpctl " + strings.Join(listArgs, " ")
	out, ok := probe(logsProbeTimeout, "coredumpctl", listArgs...)
	if !ok || strings.TrimSpace(out) == "" {
		return []logSection{{head: listHead, note: "no coredumps for " + name}}
	}
	secs := []logSection{{head: listHead, body: tailLines(out, cfg.lines)}}
	if info, ok := probe(logsProbeTimeout, "coredumpctl", "info", name); ok && strings.TrimSpace(info) != "" {
		secs = append(secs, logSection{head: "coredumpctl info " + name + " (newest)", body: headLines(info, cfg.lines)})
	}
	return secs
}

// packageSections reports the native package that owns the binary and its
// recent transactions: an upgrade right before a breakage is the classic cause.
func packageSections(name string, cfg logsConfig) []logSection {
	binPath, err := exec.LookPath(name)
	if err != nil {
		return []logSection{{head: "PATH lookup for " + name, note: name + " is not on PATH (a script, an alias, or a Flatpak)"}}
	}
	if logsPackageManager() == "dnf" {
		return dnfPackageSections(binPath, cfg)
	}
	ownHead := "pacman -Qqo " + binPath
	out, ok := probe(logsProbeTimeout, "pacman", "-Qqo", binPath)
	pkg := strings.TrimSpace(out)
	if !ok || pkg == "" {
		return []logSection{{head: ownHead, note: binPath + " is not owned by a pacman package (an AUR build, a script, or a Flatpak)"}}
	}
	secs := []logSection{{head: ownHead, body: pkg}}
	if body := pacmanPackageLines(pkg, cfg.lines); body != "" {
		secs = append(secs, logSection{head: pacmanLogPath + " (transactions for " + pkg + ")", body: body})
	} else {
		secs = append(secs, logSection{head: pacmanLogPath + " (transactions for " + pkg + ")", note: "no recorded transactions"})
	}
	return secs
}

// flatpakSection notes when the target is a Flatpak and where its logs live.
func flatpakSection(name string) logSection {
	head := "flatpak list --app"
	out, ok := probe(logsProbeTimeout, "flatpak", "list", "--app")
	if !ok {
		return logSection{head: head, note: "flatpak not installed"}
	}
	lower := strings.ToLower(name)
	for _, l := range strings.Split(out, "\n") {
		if !strings.Contains(strings.ToLower(l), lower) {
			continue
		}
		id := flatpakAppID(l)
		return logSection{head: head, body: id + " is a Flatpak app. Its logs are in the journal (journalctl --user) and its private HOME is ~/.var/app/" + id}
	}
	return logSection{head: head, note: name + " is not a Flatpak app"}
}

// flatpakAppID picks the reverse-DNS application id out of a `flatpak list`
// row, falling back to the whole line when the columns are unusual.
func flatpakAppID(row string) string {
	for _, f := range strings.FieldsFunc(row, func(r rune) bool { return r == '\t' }) {
		f = strings.TrimSpace(f)
		if strings.Contains(f, ".") && !strings.ContainsAny(f, " /") {
			return f
		}
	}
	return strings.TrimSpace(row)
}

// quickshellLogSection finds the running Quickshell instance whose config path
// is under configSub and tails its log.
func quickshellLogSection(configSub string, cfg logsConfig) logSection {
	list, ok := probe(logsProbeTimeout, "qs", "list", "--all")
	if !ok {
		return logSection{head: "qs list --all", note: "quickshell (qs) not available"}
	}
	id := quickshellInstanceFor(list, configSub)
	if id == "" {
		return logSection{head: "qs list --all", note: "no running quickshell instance for " + configSub}
	}
	head := "qs log -i " + id + " -t " + strconv.Itoa(cfg.lines)
	out, ok := probe(logsProbeTimeout, "qs", "log", "-i", id, "-t", strconv.Itoa(cfg.lines))
	if !ok {
		return logSection{head: head, note: "qs log returned nothing"}
	}
	return logSection{head: head, body: tailLines(out, cfg.lines)}
}

// quickshellInstanceFor parses `qs list --all` and returns the instance id
// whose "Config path:" contains configSub, or "".
func quickshellInstanceFor(list, configSub string) string {
	var cur string
	for _, l := range strings.Split(list, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "Instance ") && strings.HasSuffix(t, ":") {
			cur = strings.TrimSuffix(strings.TrimPrefix(t, "Instance "), ":")
			continue
		}
		if strings.HasPrefix(t, "Config path:") {
			if strings.Contains(strings.TrimSpace(strings.TrimPrefix(t, "Config path:")), configSub) {
				return cur
			}
		}
	}
	return ""
}

// unitRef names a discovered unit and its scope.
type unitRef struct {
	name string
	user bool
}

// discoverUnits finds user and system units whose name contains the target.
func discoverUnits(name string) []unitRef {
	var refs []unitRef
	seen := map[string]bool{}
	for _, user := range []bool{true, false} {
		args := []string{}
		if user {
			args = append(args, "--user")
		}
		args = append(args, "list-units", "--all", "--no-legend", "--plain")
		out, ok := probe(logsProbeTimeout, "systemctl", args...)
		if !ok {
			continue
		}
		lower := strings.ToLower(name)
		for _, l := range strings.Split(out, "\n") {
			fields := strings.Fields(l)
			if len(fields) == 0 {
				continue
			}
			u := fields[0]
			if !strings.HasSuffix(u, ".service") || !strings.Contains(strings.ToLower(u), lower) {
				continue
			}
			k := boolPrefix(user) + u
			if seen[k] {
				continue
			}
			seen[k] = true
			refs = append(refs, unitRef{name: u, user: user})
		}
	}
	return refs
}

func boolPrefix(user bool) string {
	if user {
		return "u:"
	}
	return "s:"
}

// discoverUserUnits expands a systemctl glob to the matching user unit names.
func discoverUserUnits(glob string) []string {
	out, ok := probe(logsProbeTimeout, "systemctl", "--user", "list-units", "--all", "--no-legend", "--plain", glob)
	if !ok {
		return nil
	}
	var units []string
	for _, l := range strings.Split(out, "\n") {
		fields := strings.Fields(l)
		if len(fields) == 0 {
			continue
		}
		u := fields[0]
		if strings.HasSuffix(u, ".service") || strings.HasSuffix(u, ".socket") || strings.HasSuffix(u, ".target") {
			units = append(units, u)
		}
	}
	return units
}

// appLogRoots are the per-app trees a program tends to write logs under.
func appLogRoots(name string) []string {
	h := home()
	return []string{
		filepath.Join(h, ".local", "state", name),
		filepath.Join(h, ".cache", name),
		filepath.Join(h, ".config", name),
		filepath.Join(h, ".local", "share", name),
	}
}

// fileRootSections tails the newest *.log files found under one root.
func fileRootSections(root string, cfg logsConfig) []logSection {
	return fileRootSections2([]string{root}, cfg)
}

// fileRootSections2 tails the newest *.log files found across several roots. A
// single "nothing found" section stands in when no roots hold a log.
func fileRootSections2(roots []string, cfg logsConfig) []logSection {
	files := discoverLogFiles(roots, logsFileDepth, logsFileCount)
	if len(files) == 0 {
		return []logSection{{head: "*.log under " + strings.Join(shortRoots(roots), ", "), note: "no log files found"}}
	}
	secs := make([]logSection, 0, len(files))
	for _, f := range files {
		secs = append(secs, logSection{head: f, body: tailFile(f, cfg.lines), note: "empty file"})
	}
	return secs
}

func shortRoots(roots []string) []string {
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		out = append(out, tildeAbbrev(r))
	}
	return out
}

// fileSection tails a single named file.
func fileSection(path string, cfg logsConfig) logSection {
	if !fileExists(path) {
		return logSection{head: path, note: "not present"}
	}
	return logSection{head: path, body: tailFile(path, cfg.lines), note: "empty file"}
}

// discoverLogFiles walks roots up to maxDepth directory levels and returns the
// newest maxCount files ending in .log; non-log files are ignored. Read-only.
func discoverLogFiles(roots []string, maxDepth, maxCount int) []string {
	type ent struct {
		path string
		mod  int64
	}
	var found []ent
	for _, root := range roots {
		base := strings.Count(filepath.Clean(root), string(os.PathSeparator))
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if strings.Count(filepath.Clean(p), string(os.PathSeparator))-base >= maxDepth {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".log") {
				return nil
			}
			info, e := d.Info()
			if e != nil {
				return nil
			}
			found = append(found, ent{p, info.ModTime().UnixNano()})
			return nil
		})
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].mod > found[j].mod })
	out := make([]string, 0, maxCount)
	for _, e := range found {
		out = append(out, e.path)
		if len(out) >= maxCount {
			break
		}
	}
	return out
}

// commName is the process name journald records for a binary: the base of
// argv[0] truncated to 15 bytes, since the kernel's TASK_COMM_LEN is 16 and
// one byte is the terminating NUL.
func commName(binary string) string {
	base := filepath.Base(binary)
	if len(base) > 15 {
		return base[:15]
	}
	return base
}

// commCandidates derives the process-name spellings to search for from a
// runtime-detected name, so a lowercase provider name still matches a
// capitalised binary without either being written into the source.
func commCandidates(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	out := []string{commName(name)}
	if title := strings.ToUpper(name[:1]) + name[1:]; title != name {
		out = append(out, commName(title))
	}
	return out
}

func logsPackageManager() string {
	out, ok := probe(logsProbeTimeout, "ryoku-host", "pkgmgr")
	if !ok {
		return ""
	}
	return strings.TrimSpace(out)
}

func dnfBinary() string {
	for _, name := range []string{"dnf5", "dnf"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func dnfHistorySection(pkg string, cfg logsConfig) logSection {
	command := dnfBinary()
	args := []string{"history", "list"}
	if pkg != "" {
		if filepath.Base(command) == "dnf5" {
			args = append(args, "--contains-pkgs="+pkg)
		} else {
			args = append(args, pkg)
		}
	}
	head := "dnf history list"
	if pkg != "" {
		if filepath.Base(command) == "dnf5" {
			head += " --contains-pkgs=" + pkg
		} else {
			head += " " + pkg
		}
	}
	if command == "" {
		return logSection{head: head, note: "dnf unavailable"}
	}
	out, ok := probe(logsProbeTimeout, command, args...)
	if !ok {
		return logSection{head: head, note: "dnf history unavailable"}
	}
	return logSection{head: head, body: tailLines(out, cfg.lines), note: "no recorded transactions"}
}

func dnfPackageSections(binPath string, cfg logsConfig) []logSection {
	head := "ryoku-host pkg owner " + binPath
	out, ok := probe(logsProbeTimeout, "ryoku-host", "pkg", "owner", binPath)
	pkg := strings.TrimSpace(out)
	if !ok || pkg == "" {
		return []logSection{{head: head, note: binPath + " is not owned by an RPM package (a script or a Flatpak)"}}
	}
	return []logSection{
		{head: head, body: pkg},
		dnfHistorySection(pkg, cfg),
	}
}

// pacmanTail returns the last n lines of pacman.log.
func pacmanTail(n int) string {
	return tailFile(pacmanLogPath, n)
}

// pacmanPackageLines keeps the pacman.log transactions for one package.
func pacmanPackageLines(pkg string, n int) string {
	data, err := os.ReadFile(pacmanLogPath)
	if err != nil {
		return ""
	}
	actions := []string{"installed", "upgraded", "downgraded", "reinstalled", "removed"}
	var kept []string
	for _, l := range strings.Split(string(data), "\n") {
		if !strings.Contains(l, "[ALPM]") {
			continue
		}
		for _, a := range actions {
			if strings.Contains(l, a+" "+pkg+" ") {
				kept = append(kept, l)
				break
			}
		}
	}
	return tailLines(strings.Join(kept, "\n"), n)
}

// renderLogReport prints the bundle: a header, the window, then each section
// fenced under its exact command or file. The total body is capped so the
// bundle always fits an agent's context.
func renderLogReport(target, key string, cfg logsConfig, secs []logSection) string {
	var b strings.Builder
	kind := "an app or unit"
	if key != "" {
		kind = "the Ryoku " + key
	}
	fmt.Fprintf(&b, "# Logs: %s\n\n", target)
	fmt.Fprintf(&b, "Resolved as %s. Window: since %s, up to %d lines per source. Read-only, no root.\n\n", kind, cfg.since, cfg.lines)
	remaining := logsTotalLineCap
	for _, s := range secs {
		fmt.Fprintf(&b, "## %s\n\n", s.head)
		body := strings.TrimRight(s.body, "\n")
		if body == "" {
			note := s.note
			if note == "" {
				note = "(nothing found)"
			}
			b.WriteString(note + "\n\n")
			continue
		}
		if remaining <= 0 {
			b.WriteString("(omitted: output budget reached; run the command above)\n\n")
			continue
		}
		lines := strings.Split(body, "\n")
		if len(lines) > remaining {
			lines = lines[len(lines)-remaining:]
			body = "(trimmed to the output budget)\n" + strings.Join(lines, "\n")
		} else {
			body = strings.Join(lines, "\n")
		}
		remaining -= len(lines)
		b.WriteString("```\n" + body + "\n```\n\n")
	}
	return b.String()
}

// tailFile returns the last n lines of a file, reading only the tail of a large
// file rather than loading the whole thing.
func tailFile(path string, n int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return ""
	}
	const block = 8192
	size := stat.Size()
	pos := size
	var buf []byte
	for pos > 0 && bytes.Count(buf, []byte{'\n'}) <= n {
		chunk := int64(block)
		if pos < chunk {
			chunk = pos
		}
		pos -= chunk
		tmp := make([]byte, chunk)
		if _, err := f.ReadAt(tmp, pos); err != nil {
			break
		}
		buf = append(tmp, buf...)
	}
	return tailLines(string(buf), n)
}

// tailLines returns the last n lines of s, trimmed of a trailing newline.
func tailLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// headLines returns the first n lines of s.
func headLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
