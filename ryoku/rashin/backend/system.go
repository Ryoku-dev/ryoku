package main

// system.go is the read-only machine inventory behind GET /api/system: the
// services, timers, schedules, containers, listeners, and mounts a box runs.
// Nothing here mutates anything: every collector parses tool or /proc output,
// and a missing or failing source degrades to an absent section with an honest
// note, never an error and never a fabricated row. Each section carries its own
// stamp so the dashboard can mark stale data instead of hiding an old value.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// SystemInventory is everything /api/system shows.
type SystemInventory struct {
	CollectedAt time.Time        `json:"collectedAt"`
	Services    ServiceSection   `json:"services"`
	Timers      TimerSection     `json:"timers"`
	Schedules   ScheduleSection  `json:"schedules"`
	Containers  ContainerSection `json:"containers"`
	Listeners   ListenerSection  `json:"listeners"`
	Processes   ProcessSection   `json:"processes"`
	Mounts      MountSection     `json:"mounts"`
	Tips        []Tip            `json:"tips"`
}

// SectionMeta stamps when a section was gathered and why it may be empty.
type SectionMeta struct {
	CollectedAt time.Time `json:"collectedAt"`
	Note        string    `json:"note,omitempty"`
}

// UnitRow is one systemd service as list-units reports it.
type UnitRow struct {
	Name        string `json:"name"`
	LoadState   string `json:"loadState,omitempty"`
	ActiveState string `json:"activeState"`
	SubState    string `json:"subState,omitempty"`
	Description string `json:"description,omitempty"`
}

// ServiceSection splits machine services into running and not.
type ServiceSection struct {
	SectionMeta
	Running  []UnitRow `json:"running"`
	Stopped  []UnitRow `json:"stopped"`
	UserOnly []UnitRow `json:"userOnly"`
	RunningN int       `json:"runningN"`
	TotalN   int       `json:"totalN"`
}

// TimerRow is one systemd timer, active or merely loaded.
type TimerRow struct {
	Unit      string `json:"unit"`
	Activates string `json:"activates,omitempty"`
	NextRun   string `json:"nextRun"`
	Left      string `json:"left,omitempty"`
	Last      string `json:"last,omitempty"`
	Passive   bool   `json:"passive"`
}

// TimerSection keeps firing and dormant timers apart: a loaded-but-inactive
// timer is a common silent failure, so it gets its own group.
type TimerSection struct {
	SectionMeta
	Active  []TimerRow `json:"active"`
	Passive []TimerRow `json:"passive"`
}

// CronEntry is one scheduled command from any cron-family source. A file in
// an anacron script directory that the package manager owns is a packaged
// fallback (snapper's cron.hourly, logrotate's cron.daily): harmless on a
// systemd box whose equivalent timers already run, and never something the
// user scheduled, so it is marked and excluded from the "no cron daemon" act.
type CronEntry struct {
	Schedule     string `json:"schedule"`
	Command      string `json:"command"`
	Origin       string `json:"origin"`
	PackageOwned bool   `json:"packageOwned,omitempty"`
}

// ScheduleSection covers crontabs, anacron, and at; systemd timers live in
// their own section because they are the same job on a different daemon.
type ScheduleSection struct {
	SectionMeta
	Crontabs   []CronEntry `json:"crontabs"`
	Anacron    []CronEntry `json:"anacron"`
	AtJobs     []CronEntry `json:"atJobs"`
	CronActive *bool       `json:"cronActive,omitempty"`
}

// ContainerRow is one docker container, running or not.
type ContainerRow struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"`
	Created string `json:"created"`
}

// ContainerSection reports the whole `docker ps -a` set, not just live ones.
type ContainerSection struct {
	SectionMeta
	Rows      []ContainerRow `json:"rows"`
	RunningN  int            `json:"runningN"`
	TotalN    int            `json:"totalN"`
	Installed bool           `json:"installed"`
}

// ListenerRow is one listening socket with the process behind it.
type ListenerRow struct {
	Proto    string `json:"proto"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Process  string `json:"process,omitempty"`
	Loopback bool   `json:"loopback"`
}

// ListenerSection is the network exposure surface.
type ListenerSection struct {
	SectionMeta
	Rows []ListenerRow `json:"rows"`
}

// ProcessRow is one entry of the top-CPU snapshot.
type ProcessRow struct {
	PID     int     `json:"pid"`
	Command string  `json:"command"`
	CPUPct  float64 `json:"cpuPct"`
	MemRSS  int64   `json:"memRss"`
}

// ProcessSection ranks live processes by CPU.
type ProcessSection struct {
	SectionMeta
	Rows []ProcessRow `json:"rows"`
}

// MountRow is one real mounted filesystem.
type MountRow struct {
	Device     string  `json:"device"`
	Mountpoint string  `json:"mountpoint"`
	FSType     string  `json:"fstype"`
	Size       int64   `json:"size"`
	Used       int64   `json:"used"`
	UsePct     float64 `json:"usePct"`
}

// MountSection reports block-backed mounts with usage from statfs.
type MountSection struct {
	SectionMeta
	Rows []MountRow `json:"rows"`
}

// Tip is a read-only recommendation: what was observed and how to act on it
// through your own terminal. Rashin never runs any of it.
type Tip struct {
	ID       string `json:"id"`
	Severity string `json:"severity"` // info | watch | act
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Command  string `json:"command,omitempty"`
}

// collectSystem gathers every section concurrently. Each collector owns its
// own struct field, so no lock is needed, and one hanging or missing tool only
// blanks its own section.
func collectSystem() SystemInventory {
	inv := SystemInventory{CollectedAt: time.Now()}
	var wg sync.WaitGroup
	for _, c := range []func(*SystemInventory){
		collectServices, collectTimers, collectSchedules, collectContainers,
		collectListeners, collectProcesses, collectMounts,
	} {
		wg.Add(1)
		go func(f func(*SystemInventory)) {
			defer wg.Done()
			f(&inv)
		}(c)
	}
	wg.Wait()
	inv.Tips = DeriveTips(&inv)
	return inv
}

func sysProbe(seconds int, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return string(out), err
}

// ---- services -----------------------------------------------------------------

// systemctlJSON asks systemctl for machine-readable output when the installed
// version speaks it, and reports false for older ones, whose callers fall back
// to parsing the column tables.
func systemctlJSON(args ...string) (string, bool) {
	full := append(append([]string{}, args...), "--output=json", "--no-pager")
	out, err := sysProbe(6, "systemctl", full...)
	if err != nil || strings.TrimSpace(out) == "" {
		return "", false
	}
	return out, true
}

func collectServices(inv *SystemInventory) {
	s := &inv.Services
	s.CollectedAt = time.Now()
	var all []UnitRow
	var failed []string
	if out, ok := systemctlJSON("list-units", "--type=service", "--all"); ok {
		all = parseUnitJSON(out)
	} else {
		out, err := sysProbe(6, "systemctl", "list-units", "--type=service", "--all",
			"--no-pager", "--no-legend", "--plain")
		if out == "" {
			s.Note = "systemctl unavailable; services not reported"
			_ = err
			return
		}
		all = parseUnitRows(out)
	}
	s.TotalN = len(all)
	for _, u := range all {
		switch {
		case u.ActiveState == "active" && u.SubState == "running":
			s.Running = append(s.Running, u)
		default:
			s.Stopped = append(s.Stopped, u)
			if u.ActiveState == "failed" {
				failed = append(failed, u.Name)
			}
		}
	}
	s.RunningN = len(s.Running)
	sortUnits(s.Running)
	sortUnits(s.Stopped)
	if uout, ok := systemctlJSON("--user", "list-units", "--type=service", "--all"); ok {
		s.UserOnly = parseUnitJSON(uout)
		sortUnits(s.UserOnly)
	}
	if len(failed) > 0 {
		s.Note = "failed: " + strings.Join(failed, " ")
	}
}

func sortUnits(rows []UnitRow) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
}

type unitJSON struct {
	Unit        string `json:"unit"`
	LoadState   string `json:"load"`
	ActiveState string `json:"active"`
	SubState    string `json:"sub"`
	Description string `json:"description"`
}

func parseUnitJSON(out string) []UnitRow {
	var units []unitJSON
	if json.Unmarshal([]byte(out), &units) != nil {
		return nil
	}
	rows := make([]UnitRow, 0, len(units))
	for _, u := range units {
		rows = append(rows, UnitRow{Name: u.Unit, LoadState: u.LoadState,
			ActiveState: u.ActiveState, SubState: u.SubState, Description: u.Description})
	}
	return rows
}

var unitLine = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.*)$`)

// parseUnitRows reads `systemctl list-units --plain` columns:
// UNIT LOAD ACTIVE SUB DESCRIPTION.
func parseUnitRows(out string) []UnitRow {
	var rows []UnitRow
	for _, line := range nonEmptyLines(out) {
		m := unitLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		rows = append(rows, UnitRow{
			Name:        m[1],
			LoadState:   m[2],
			ActiveState: m[3],
			SubState:    m[4],
			Description: strings.TrimSpace(m[5]),
		})
	}
	return rows
}

// ---- timers ---------------------------------------------------------------------

func collectTimers(inv *SystemInventory) {
	t := &inv.Timers
	t.CollectedAt = time.Now()
	if out, ok := systemctlJSON("list-timers", "--all"); ok {
		for _, row := range parseTimerJSON(out) {
			if row.Passive {
				t.Passive = append(t.Passive, row)
			} else {
				t.Active = append(t.Active, row)
			}
		}
		return
	}
	// Fallback for systemctl without JSON: --all --plain lists
	// NEXT LEFT LAST PASSTHROUGH UNIT ACTIVATES. NEXT and LAST are
	// "<weekday> <date> <time>" or "-"; spans may hold spaces. A row whose
	// unit token ends in .timer is parsed by counting from the unit back.
	out, err := sysProbe(6, "systemctl", "list-timers", "--all", "--no-pager", "--no-legend", "--plain")
	if out == "" {
		t.Note = "systemctl unavailable; timers not reported"
		_ = err
		return
	}
	for _, line := range nonEmptyLines(out) {
		f := strings.Fields(line)
		ui := -1
		for i, tok := range f {
			if strings.HasSuffix(tok, ".timer") {
				ui = i
				break
			}
		}
		if ui < 3 {
			continue
		}
		row := TimerRow{Unit: f[ui]}
		if ui+1 < len(f) && f[ui+1] != "-" {
			row.Activates = f[ui+1]
		}
		if f[0] == "-" {
			row.Passive = true
			row.NextRun = "-"
		} else {
			row.NextRun = strings.Join(f[0:3], " ")
		}
		if row.Passive {
			t.Passive = append(t.Passive, row)
		} else {
			t.Active = append(t.Active, row)
		}
	}
}

// timerJSON mirrors systemctl list-timers --output=json. next/last are
// microseconds since the epoch; 0 means never. passed is how long the timer
// has sat between triggers.
type timerJSON struct {
	Next      int64  `json:"next"`
	Last      int64  `json:"last"`
	Passed    int64  `json:"passed"`
	Unit      string `json:"unit"`
	Activates string `json:"activates"`
}

func parseTimerJSON(out string) []TimerRow {
	var raw []timerJSON
	if json.Unmarshal([]byte(out), &raw) != nil {
		return nil
	}
	rows := make([]TimerRow, 0, len(raw))
	for _, r := range raw {
		row := TimerRow{Unit: r.Unit, Activates: r.Activates}
		if r.Next > 0 {
			row.NextRun = time.UnixMicro(r.Next).Format(time.RFC3339)
		} else {
			row.Passive = true
			row.NextRun = "-"
		}
		if r.Last > 0 {
			row.Last = time.UnixMicro(r.Last).Format(time.RFC3339)
		}
		if r.Next > 0 {
			row.Left = humanSpan(time.Duration(r.Next-time.Now().UnixMicro()) * time.Microsecond)
		}
		rows = append(rows, row)
	}
	return rows
}

func humanSpan(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	var parts []string
	push := func(n int64, unit string) {
		if n > 0 {
			parts = append(parts, strconv.FormatInt(n, 10)+unit)
		}
	}
	if d < time.Second {
		return "0s"
	}
	push(int64(d/(7*24*time.Hour)), "w")
	push(int64(d%(7*24*time.Hour)/(24*time.Hour)), "d")
	push(int64(d%(24*time.Hour)/time.Hour), "h")
	push(int64(d%(time.Hour)/time.Minute), "min")
	if len(parts) == 0 {
		push(int64(d/time.Second), "s")
	}
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, " ")
}

// ---- schedules (cron family) ------------------------------------------------------

func collectSchedules(inv *SystemInventory) {
	sc := &inv.Schedules
	sc.CollectedAt = time.Now()
	var sources int

	if out, err := sysProbe(4, "crontab", "-l"); err == nil {
		sources++
		sc.Crontabs = append(sc.Crontabs, parseCrontab(out, "user crontab")...)
	}
	for _, p := range []string{"/etc/crontab", "/etc/cron.d"} {
		if entries, ok := readCronPaths(p); ok {
			sources++
			sc.Crontabs = append(sc.Crontabs, entries...)
		}
	}
	if entries, ok := readCronScripts("/etc/cron.hourly", "hourly"); ok {
		sources++
		sc.Anacron = append(sc.Anacron, entries...)
	}
	if entries, ok := readCronScripts("/etc/cron.daily", "daily"); ok {
		sources++
		sc.Anacron = append(sc.Anacron, entries...)
	}
	if b, err := os.ReadFile("/etc/anacrontab"); err == nil {
		sources++
		sc.Anacron = append(sc.Anacron, parseAnacron(string(b))...)
	}
	if out, err := sysProbe(4, "atq"); err == nil {
		sources++
		sc.AtJobs = append(sc.AtJobs, parseAtq(out)...)
	}
	if haveSystemd() {
		for _, svc := range []string{"cronie.service", "cron.service", "chronyd.service", "anacron.service"} {
			if _, err := sysProbe(4, "systemctl", "is-active", "--quiet", svc); err == nil {
				on := true
				sc.CronActive = &on
				break
			}
		}
		if sc.CronActive == nil {
			off := false
			sc.CronActive = &off
		}
	}
	if sources == 0 {
		sc.Note = "no cron daemon, crontab tool, or cron files on this machine; scheduled work runs through systemd timers"
	}
}

// readCronPaths covers a crontab file or a cron.d directory. A directory of
// scripts (cron.hourly/daily) is NOT a crontab tree: its files are shell
// programs anacron runs, so each one becomes a single entry instead of being
// parsed line by line.
func readCronPaths(p string) ([]CronEntry, bool) {
	st, err := os.Stat(p)
	if err != nil {
		return nil, false
	}
	var entries []CronEntry
	if st.IsDir() {
		ents, _ := os.ReadDir(p)
		for _, e := range ents {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") || strings.HasSuffix(e.Name(), ".rpmsave") {
				continue
			}
			entries = append(entries, CronEntry{Schedule: "-", Command: filepath.Join(p, e.Name()), Origin: "script"})
		}
		return entries, true
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	return parseCrontab(string(b), p), true
}

// readCronScripts lists an anacron directory as scheduled work with a cadence.
func readCronScripts(p, cadence string) ([]CronEntry, bool) {
	entries, ok := readCronPaths(p)
	if !ok {
		return nil, false
	}
	for i := range entries {
		entries[i].Schedule = cadence
		entries[i].Origin = p
		entries[i].PackageOwned = packageOwnsFile(entries[i].Command)
	}
	return entries, true
}

// packageOwnsFile asks pacman whether a packaged file owns the path. On a
// non-Arch box or for a hand-dropped script the answer is simply "no".
func packageOwnsFile(path string) bool {
	if _, err := exec.LookPath("pacman"); err != nil {
		return false
	}
	out, err := sysProbe(4, "pacman", "-Qoq", path)
	return err == nil && strings.TrimSpace(out) != ""
}

var cronLineRe = regexp.MustCompile(`^(\S+\s+\S+\s+\S+\s+\S+\s+\S+)\s+(\S.*)$`)
var cronSpecialRe = regexp.MustCompile(`^(@\S+)\s+(\S.*)$`)

// parseCrontab keeps executable entries; comments and env assignments drop out.
// The six-field system form (crontab(5)) carries a user column between the
// schedule and the command; user crontabs use the five-field form.
func parseCrontab(text, origin string) []CronEntry {
	var out []CronEntry
	for _, line := range nonEmptyLines(text) {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if m := cronSpecialRe.FindStringSubmatch(line); m != nil {
			out = append(out, CronEntry{Schedule: m[1], Command: m[2], Origin: origin})
			continue
		}
		m := cronLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if !looksLikeCron(m[1]) {
			continue
		}
		fields := strings.Fields(m[2])
		if len(fields) == 0 {
			continue
		}
		cmd := m[2]
		if len(fields) >= 2 && len(fields[0]) <= 16 && !strings.ContainsAny(fields[0], "/.$@\"'") &&
			strings.HasPrefix(fields[1], "/") {
			// six-field form: fields[0] is the run-as user, not part of the command
			cmd = strings.Join(fields[1:], " ")
		}
		out = append(out, CronEntry{Schedule: m[1], Command: cmd, Origin: origin})
	}
	return out
}

// looksLikeCron rejects shell prose that happens to have five leading
// whitespace-separated tokens: every cron field is digits, *, ranges, steps,
// lists, or three-letter month/day names.
func looksLikeCron(spec string) bool {
	for _, f := range strings.Fields(spec) {
		for _, part := range strings.Split(f, ",") {
			for _, tok := range strings.Split(part, "/") {
				tok = strings.ReplaceAll(tok, "-", "")
				if tok == "*" || tok == "?" {
					continue
				}
				if !cronTokenRe.MatchString(tok) {
					return false
				}
			}
		}
	}
	return true
}

var cronTokenRe = regexp.MustCompile(`^([0-9]+|[A-Za-z]{3})$`)

var anacronRe = regexp.MustCompile(`^(\d+)\s+(\d+)\s+(\S+)\s+(\S.*)$`)

func parseAnacron(text string) []CronEntry {
	var out []CronEntry
	for _, line := range nonEmptyLines(text) {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if m := anacronRe.FindStringSubmatch(line); m != nil {
			out = append(out, CronEntry{
				Schedule: "every " + m[1] + " day(s)",
				Command:  m[4],
				Origin:   "/etc/anacrontab job " + m[3],
			})
		}
	}
	return out
}

func parseAtq(out string) []CronEntry {
	var entries []CronEntry
	for _, line := range nonEmptyLines(out) {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		entries = append(entries, CronEntry{
			Schedule: strings.Join(f[1:3], " "),
			Command:  "at job " + f[0],
			Origin:   "atq",
		})
	}
	return entries
}

// ---- containers ---------------------------------------------------------------------

func collectContainers(inv *SystemInventory) {
	c := &inv.Containers
	c.CollectedAt = time.Now()
	if _, err := exec.LookPath("docker"); err != nil {
		c.Note = "docker not installed; no containers"
		return
	}
	c.Installed = true
	out, err := sysProbe(6, "docker", "ps", "-a", "--format", `{{json .}}`)
	if out == "" {
		c.Note = "docker installed but the daemon is unreachable (not running, or this user is not in the docker group)"
		_ = err
		return
	}
	for _, line := range nonEmptyLines(out) {
		var d struct {
			ID      string `json:"Id"`
			Names   string `json:"Names"`
			Image   string `json:"Image"`
			Status  string `json:"Status"`
			State   string `json:"State"`
			Created string `json:"CreatedAt"`
		}
		if json.Unmarshal([]byte(line), &d) != nil {
			continue
		}
		name := d.Names
		if n := strings.SplitN(name, ",", 2); len(n) > 0 {
			name = n[0]
		}
		c.Rows = append(c.Rows, ContainerRow{
			ID:      shortID(d.ID),
			Name:    name,
			Image:   d.Image,
			Status:  d.Status,
			State:   d.State,
			Created: d.Created,
		})
	}
	sort.Slice(c.Rows, func(i, j int) bool {
		if (c.Rows[i].State == "running") != (c.Rows[j].State == "running") {
			return c.Rows[i].State == "running"
		}
		return c.Rows[i].Name < c.Rows[j].Name
	})
	for _, r := range c.Rows {
		if r.State == "running" {
			c.RunningN++
		}
	}
	c.TotalN = len(c.Rows)
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// ---- listeners ------------------------------------------------------------------------

func collectListeners(inv *SystemInventory) {
	l := &inv.Listeners
	l.CollectedAt = time.Now()
	out, err := sysProbe(5, "ss", "-H", "-ltnup")
	if out == "" {
		l.Note = "ss unavailable; listeners not reported"
		_ = err
		return
	}
	seen := map[string]bool{}
	for _, line := range nonEmptyLines(out) {
		row, ok := parseSSListener(line)
		if !ok {
			continue
		}
		key := row.Proto + "|" + row.Address + ":" + strconv.Itoa(row.Port)
		if seen[key] {
			continue
		}
		seen[key] = true
		l.Rows = append(l.Rows, row)
	}
	sort.Slice(l.Rows, func(i, j int) bool {
		if l.Rows[i].Loopback != l.Rows[j].Loopback {
			return !l.Rows[i].Loopback
		}
		if l.Rows[i].Proto != l.Rows[j].Proto {
			return l.Rows[i].Proto < l.Rows[j].Proto
		}
		return l.Rows[i].Port < l.Rows[j].Port
	})
}

// parseSSListener reads one `ss -ltnup -H` row. With -u every row is prefixed
// by its netid: "tcp LISTEN 0 4096 127.0.0.1:3600 0.0.0.0:* users:(...)".
func parseSSListener(line string) (ListenerRow, bool) {
	f := strings.Fields(line)
	if len(f) < 5 {
		return ListenerRow{}, false
	}
	var proto string
	switch f[0] {
	case "udp":
		proto = "udp"
	case "tcp":
		proto = "tcp"
	default:
		return ListenerRow{}, false
	}
	switch f[1] {
	case "LISTEN", "UNCONN":
	default:
		return ListenerRow{}, false
	}
	host, port, ok := splitHostPort(f[4])
	if !ok {
		return ListenerRow{}, false
	}
	loopback := strings.HasPrefix(host, "127.") || host == "::1"
	return ListenerRow{Proto: proto, Address: host, Port: port, Process: ssProcess(line), Loopback: loopback}, true
}

// splitHostPort cuts "[::1]:80" or "0.0.0.0%eth0:53" or "*:53" at the last colon.
func splitHostPort(s string) (string, int, bool) {
	i := strings.LastIndex(s, ":")
	if i <= 0 {
		return "", 0, false
	}
	port, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return "", 0, false
	}
	host := strings.TrimSuffix(strings.TrimPrefix(s[:i], "["), "]")
	if j := strings.Index(host, "%"); j >= 0 {
		host = host[:j]
	}
	return host, port, true
}

// ssProcess pulls the first process name out of users:(("ryoku-rashin",...)).
// ssProcess pulls the first process name out of users:(("ryoku-rashin",...)).
func ssProcess(line string) string {
	const open = "users:((\""
	i := strings.Index(line, open)
	if i < 0 {
		return ""
	}
	rest := line[i+len(open):]
	if j := strings.Index(rest, "\""); j >= 0 {
		return rest[:j]
	}
	return ""
}

// ---- processes --------------------------------------------------------------------------

func collectProcesses(inv *SystemInventory) {
	p := &inv.Processes
	p.CollectedAt = time.Now()
	out, err := sysProbe(5, "ps", "-eo", "pid,pcpu,rss,comm", "--sort=-pcpu", "--no-headers")
	if out == "" {
		p.Note = "ps unavailable; processes not reported"
		_ = err
		return
	}
	for _, line := range nonEmptyLines(out) {
		if len(p.Rows) >= 12 {
			break
		}
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		pid, err1 := strconv.Atoi(f[0])
		cpu, err2 := strconv.ParseFloat(f[1], 64)
		rss, err3 := strconv.ParseInt(f[2], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		p.Rows = append(p.Rows, ProcessRow{PID: pid, Command: f[3], CPUPct: cpu, MemRSS: rss * 1024})
	}
}

// ---- mounts -------------------------------------------------------------------------------

func collectMounts(inv *SystemInventory) {
	m := &inv.Mounts
	m.CollectedAt = time.Now()
	seen := map[string]bool{}
	for _, line := range nonEmptyLines(readProcFileOrEmpty("/proc/mounts")) {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		dev, point, fstype := unescapeMount(f[0]), f[1], f[2]
		if skipMount(fstype, point) || seen[dev] {
			continue
		}
		var st syscall.Statfs_t
		if syscall.Statfs(point, &st) != nil || st.Blocks == 0 {
			continue
		}
		seen[dev] = true
		total := int64(st.Blocks) * st.Bsize
		free := int64(st.Bfree) * st.Bsize
		m.Rows = append(m.Rows, MountRow{
			Device: filepath.Base(dev), Mountpoint: point, FSType: fstype,
			Size: total, Used: total - free, UsePct: 100 * float64(total-free) / float64(total),
		})
	}
	sort.Slice(m.Rows, func(i, j int) bool { return m.Rows[i].Mountpoint < m.Rows[j].Mountpoint })
}

func readProcFileOrEmpty(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func unescapeMount(s string) string {
	return strings.ReplaceAll(s, "\\040", " ")
}

func skipMount(fstype, mountpoint string) bool {
	switch fstype {
	case "swap", "squashfs", "tmpfs", "devtmpfs", "overlay", "efivarfs", "autofs", "ramfs",
		"binfmt_misc", "proc", "sysfs", "devpts", "securityfs", "pstore", "cgroup", "cgroup2",
		"debugfs", "tracefs", "configfs", "fusectl", "mqueue", "hugetlbfs", "bpf", "none",
		"rpc_pipefs", "nsfs", "fuse.gvfsd-fuse", "fuse.portal":
		return true
	}
	for _, p := range []string{"/proc", "/sys", "/dev", "/run", "/var/lib/docker", "/snap"} {
		if mountpoint == p || strings.HasPrefix(mountpoint, p+"/") {
			return true
		}
	}
	return mountpoint == "/boot/efi"
}

// ---- tips ------------------------------------------------------------------------------------

// DeriveTips turns observations into recommendations. Every tip names the
// command you would run in a terminal; rashin never runs it and never claims
// to have. The rules are deterministic, so the advice is always grounded in a
// row the user can see above it, and it costs no model tokens.
func DeriveTips(inv *SystemInventory) []Tip {
	var tips []Tip
	add := func(t Tip) { tips = append(tips, t) }

	// Failed and dead units group into one tip each: four clones of the same
	// advice is noise, and the unit names are already visible in the table.
	var failedUnits, deadUnits []string
	var failedCmd, deadCmd string
	for _, u := range inv.Services.Stopped {
		if strings.Contains(u.LoadState, "not-found") {
			deadUnits = append(deadUnits, u.Name)
			if deadCmd == "" {
				deadCmd = "systemctl list-dependencies " + u.Name
			}
			continue
		}
		if u.ActiveState == "failed" {
			failedUnits = append(failedUnits, u.Name)
			if failedCmd == "" {
				failedCmd = "journalctl -u " + strings.TrimSuffix(u.Name, ".service") + " -n 50 --no-pager"
			}
		}
	}
	if len(failedUnits) > 0 {
		title := failedUnits[0] + " is in failed state"
		if len(failedUnits) > 1 {
			title = strconv.Itoa(len(failedUnits)) + " services are in failed state: " + listTitle(failedUnits)
		}
		add(Tip{ID: "failed-services", Severity: "act",
			Title:   title,
			Detail:  "They exited with an error. Read the journal before restarting anything; a service that keeps failing usually says why in its last lines.",
			Command: failedCmd})
	}
	if len(deadUnits) > 0 {
		verb := " has no unit file"
		if len(deadUnits) > 1 {
			verb = " have no unit file"
		}
		add(Tip{ID: "dead-units", Severity: "watch",
			Title:   listTitle(deadUnits) + verb,
			Detail:  "Something still asks systemd for them (a dependency or a leftover enable). Read where the request comes from, then drop them from the boot order.",
			Command: deadCmd})
	}
	if inv.Services.TotalN > 0 && inv.Services.RunningN > 60 {
		add(Tip{ID: "boot-load", Severity: "info",
			Title:   strconv.Itoa(inv.Services.RunningN) + " services are running",
			Detail:  "A quieter boot costs less RAM and fewer surprises. Blame the slowest units and disable what you do not use.",
			Command: "systemd-analyze blame | head -n 15"})
	}
	for _, t := range inv.Timers.Passive {
		add(Tip{ID: "timer-off-" + t.Unit, Severity: "watch",
			Title:   t.Unit + " is loaded but never fires",
			Detail:  "It is installed and enabled-looking, but sits passive, so whatever it schedules has not run. Start it, or disable the unit to keep the timer list honest.",
			Command: "systemctl status " + t.Unit})
	}
	for _, c := range inv.Containers.Rows {
		if c.State == "running" && strings.Contains(c.Status, "Restarting") {
			add(Tip{ID: "container-restart-" + c.Name, Severity: "act",
				Title:   "container " + c.Name + " is crash-looping",
				Detail:  "The restart counter keeps climbing: it dies and docker relaunches it. Read its log before touching the image or config.",
				Command: "docker logs --tail 50 " + c.Name})
		}
	}
	if stopped := exitedContainers(inv); stopped > 3 {
		add(Tip{ID: "container-cleanup", Severity: "info",
			Title:   strconv.Itoa(stopped) + " stopped containers still hold disk",
			Detail:  "Each keeps its writable layer until pruned. List them, confirm nothing matters, then prune.",
			Command: "docker ps -a --filter status=exited && docker container prune"})
	}
	for _, l := range inv.Listeners.Rows {
		if l.Loopback || isWildcardAddr(l.Address) {
			continue
		}
		add(Tip{ID: "exposed-" + l.Proto + "-" + strconv.Itoa(l.Port), Severity: "watch",
			Title:   orUnknown(l.Process) + " serves " + l.Address + ":" + strconv.Itoa(l.Port) + " on a named interface",
			Detail:  "A home box should expose only what you mean to. Check who is dialing in, then firewall it or bind it to 127.0.0.1.",
			Command: "ss -tnp dport = :" + strconv.Itoa(l.Port)})
	}
	var wildcard []string
	for _, l := range inv.Listeners.Rows {
		if isWildcardAddr(l.Address) {
			wildcard = append(wildcard, l.Proto+"/"+strconv.Itoa(l.Port))
		}
	}
	if len(wildcard) > 0 {
		add(Tip{ID: "wildcard-bind", Severity: "watch",
			Title:   strconv.Itoa(len(wildcard)) + " listeners are bound to all interfaces",
			Detail:  "These answer on every network the machine joins: " + strings.Join(wildcard, " ") + ". Walk each one and decide whether it should face outward.",
			Command: "ss -ltnup"})
	}
	if inv.Schedules.CronActive != nil && !*inv.Schedules.CronActive &&
		scheduledWorkWaiting(inv.Schedules) > 0 {
		add(Tip{ID: "cron-off", Severity: "act",
			Title:   "cron entries exist but no cron daemon is running",
			Detail:  "Nothing you scheduled through those files is firing. Start the daemon or move the entries to systemd timers.",
			Command: "systemctl status cronie anacron"})
	}
	for _, m := range inv.Mounts.Rows {
		if m.UsePct >= 90 {
			add(Tip{ID: "disk-" + m.Mountpoint, Severity: "act",
				Title:   m.Mountpoint + " is " + strconv.Itoa(int(m.UsePct)) + "% full",
				Detail:  "Past 90% updates and snapshot tooling start failing. Find the fat directories before the last gigabytes go.",
				Command: "du -xh --max-depth=2 " + m.Mountpoint + " 2>/dev/null | sort -rh | head"})
		}
	}
	if inv.Containers.Installed && inv.Containers.TotalN == 0 && inv.Containers.Note == "" {
		add(Tip{ID: "no-containers", Severity: "info",
			Title:   "docker reports zero containers",
			Detail:  "The daemon answers but nothing has ever run, or everything was pruned. This is the panel where they would appear.",
			Command: "docker ps -a"})
	}
	sort.SliceStable(tips, func(i, j int) bool { return sevRank(tips[i].Severity) < sevRank(tips[j].Severity) })
	if len(tips) > 12 {
		tips = tips[:12]
	}
	return tips
}

// scheduledWorkWaiting counts cron entries that would fire if a daemon ran:
// a crontab line or a hand-dropped script. Files a package owns (snapper's
// cron.hourly fallback and friends) are not counted: their work already runs
// through systemd timers, and installing a cron daemon on top would schedule
// it twice.
func scheduledWorkWaiting(sc ScheduleSection) int {
	n := len(sc.Crontabs)
	for _, e := range sc.Anacron {
		if !e.PackageOwned {
			n++
		}
	}
	return n
}

func isWildcardAddr(a string) bool {
	return a == "*" || a == "::" || a == "0.0.0.0"
}

// listTitle keeps a tip title readable: the first few names, the rest counted.
func listTitle(names []string) string {
	const head = 3
	if len(names) <= head {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:head], ", ") + fmt.Sprintf(" and %d more", len(names)-head)
}

func sevRank(s string) int {
	switch s {
	case "act":
		return 0
	case "watch":
		return 1
	default:
		return 2
	}
}

func exitedContainers(inv *SystemInventory) int {
	n := 0
	for _, c := range inv.Containers.Rows {
		if c.State != "running" {
			n++
		}
	}
	return n
}

// ---- caching --------------------------------------------------------------------------------

// systemTTL keeps the overview cheap under dashboard polling: systemctl,
// docker, and ss together would otherwise exec every couple of seconds forever.
// A refresh runs in the background; callers always get a snapshot, never a block.
const systemTTL = 30 * time.Second

var systemCache struct {
	mu       sync.Mutex
	at       time.Time
	data     *SystemInventory
	inflight bool
}

// SystemNow returns the freshest inventory, kicking off a refresh in the
// background when the cached one ages out. The very first call blocks once so
// the dashboard never renders an empty shell.
func SystemNow() SystemInventory {
	systemCache.mu.Lock()
	inv := systemCache.data
	fresh := inv != nil && time.Since(systemCache.at) < systemTTL
	start := inv != nil && !fresh && !systemCache.inflight
	if start {
		systemCache.inflight = true
	}
	systemCache.mu.Unlock()

	switch {
	case fresh:
		return *inv
	case start:
		go func() {
			next := collectSystem()
			systemCache.mu.Lock()
			systemCache.at, systemCache.data, systemCache.inflight = time.Now(), &next, false
			systemCache.mu.Unlock()
		}()
		return *inv
	default:
		next := collectSystem()
		systemCache.mu.Lock()
		systemCache.at, systemCache.data = time.Now(), &next
		systemCache.mu.Unlock()
		return next
	}
}
