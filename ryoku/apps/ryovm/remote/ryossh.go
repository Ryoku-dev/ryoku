// ryossh: the remote (SSH/VPS) data plane for the ryoport hub. It is a small,
// dependency-free CLI invoked as subcommands by the QML front end; it wraps the
// system OpenSSH tools (ssh, ssh -G, ssh-add, ssh-keygen, ssh-copy-id) plus an
// agentless /proc health probe and prints JSON to stdout, human errors to
// stderr, exiting nonzero on failure. Connection facts come from ~/.ssh/config
// (resolved via `ssh -G`); ryoport-owned metadata lives in a sidecar JSON, and
// GUI-added hosts go into an Include'd file so the CLI and GUI never diverge and
// the user's hand-written config is never rewritten.
//
// Usage: ryossh <verb> [args]
//
//	list                     JSON array of hosts (ssh_config + sidecar merge)
//	ping <alias>             TCP reachability + latency
//	pingall                  ping every host, one JSON array
//	probe <alias>            one-shot /proc health over ssh
//	probeall                 probe every host, one JSON array
//	connect <alias> [--cmd]  launch a detached terminal running ssh
//	keys                     agent + local pubkey fingerprints
//	copyid <alias> [pubkey]  ssh-copy-id (interactive)
//	keygen <name> [type]     generate a convenience key (no passphrase)
//	add <json>               add/replace a ryoport-managed host
//	remove <alias>           drop a ryoport-managed host
//	knownremove <alias>      ssh-keygen -R the resolved hostname
//	console list             JSON array of saved consoles (serial/telnet/RDP/VNC)
//	console add <json>       add/replace a saved console
//	console remove <id>      drop a console and its saved password
//	console open <id>        launch the console's client (keyring-fed when saved)
//	console probe <id>       reachability for one console
//	console probeall         reachability for every console, one JSON array
//	console setpass <id>     read a password from stdin into the keyring
//	console clearpass <id>   forget a console's saved password
//	tunnel <open|list|close|closeall>   tracked ssh forwards
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// poolSize bounds the concurrent `ssh -G`/ping/probe fan-out so a large config
// stays fast without spawning one process per host at once.
const poolSize = 8

// out prints a value as one JSON line on stdout (the app reads a line at a time).
func out(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		die("marshal: %v", err)
	}
	fmt.Println(string(b))
}

// die reports a human-readable error on stderr and exits nonzero; never a stack
// trace, so a missing file/tool degrades gracefully for the GUI.
func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ryossh: "+format+"\n", a...)
	os.Exit(1)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		die("usage: ryossh <list|ping|pingall|probe|probeall|appcheck|appcheckall|pveguests|pveaction|connect|keys|copyid|keygen|setpass|clearpass|add|remove|knownremove|tunnel|console> [args]")
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "list":
		cmdList()
	case "ping":
		out(pingOne(needAlias(rest, "ping")))
	case "pingall":
		cmdPingAll()
	case "probe":
		cmdProbe(needAlias(rest, "probe"))
	case "probeall":
		cmdProbeAll()
	case "appcheck":
		cmdAppCheck(needAlias(rest, "appcheck"))
	case "appcheckall":
		cmdAppCheckAll()
	case "pveguests":
		cmdPveGuests(needAlias(rest, "pveguests"))
	case "pveaction":
		cmdPveAction(rest)
	case "connect":
		cmdConnect(rest)
	case "keys":
		cmdKeys()
	case "copyid":
		cmdCopyID(rest)
	case "keygen":
		cmdKeygen(rest)
	case "add":
		cmdAdd(rest)
	case "remove":
		cmdRemove(needAlias(rest, "remove"))
	case "knownremove":
		cmdKnownRemove(needAlias(rest, "knownremove"))
	case "tunnel":
		cmdTunnel(rest)
	case "console":
		cmdConsole(rest)
	case "setpass":
		cmdSetPass(needAlias(rest, "setpass"))
	case "clearpass":
		cmdClearPass(needAlias(rest, "clearpass"))
	default:
		die("unknown verb %q", cmd)
	}
}

func needAlias(rest []string, verb string) string {
	if len(rest) < 1 {
		die("usage: ryossh %s <alias>", verb)
	}
	return rest[0]
}

// --- paths -----------------------------------------------------------------

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		die("cannot resolve home dir: %v", err)
	}
	return h
}

func sshDir() string          { return filepath.Join(home(), ".ssh") }
func sshConfigPath() string   { return filepath.Join(sshDir(), "config") }
func includeFilePath() string { return filepath.Join(sshDir(), "config.d", "ryoport") }
func sidecarPath() string {
	return filepath.Join(home(), ".config", "ryoku", "ryoport", "remotes.json")
}

// expandTilde turns a leading ~ into $HOME so Go can open the path itself.
func expandTilde(p string) string {
	if p == "~" {
		return home()
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home(), p[2:])
	}
	return p
}

// tildeShorten is the inverse: present home-relative paths as ~/… for the GUI.
func tildeShorten(p string) string {
	h := home()
	if p == h {
		return "~"
	}
	if strings.HasPrefix(p, h+"/") {
		return "~/" + p[len(h)+1:]
	}
	return p
}

// ensureSSHDir makes ~/.ssh exist (0700) so ControlPath sockets have a home.
func ensureSSHDir() {
	os.MkdirAll(sshDir(), 0700)
}

// nonInteractiveOpts are the canonical ssh options for every agentless call:
// batch (never prompt/hang), bounded connect, accept-new host keys, and a
// per-user ControlMaster so repeated probes reuse one TCP session.
func nonInteractiveOpts() []string {
	return []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=6",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=~/.ssh/ryoport-%r@%h:%p",
		"-o", "ControlPersist=60",
	}
}

// passwordOpts drops BatchMode so ssh can answer a saved-password host from the
// keyring via askpass: one prompt only, key first then password, same
// ControlMaster reuse as the agentless path.
func passwordOpts() []string {
	return []string{
		"-o", "ConnectTimeout=6",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=~/.ssh/ryoport-%r@%h:%p",
		"-o", "ControlPersist=60",
		"-o", "NumberOfPasswordPrompts=1",
		"-o", "PreferredAuthentications=publickey,password",
	}
}

// --- ssh_config resolution -------------------------------------------------

// Host is the merged record printed by `list`: ssh_config facts plus sidecar
// metadata. JSON field names are the contract the QML singleton parses against.
type Host struct {
	Alias        string   `json:"alias"`
	HostName     string   `json:"hostName"`
	User         string   `json:"user"`
	Port         int      `json:"port"`
	IdentityFile string   `json:"identityFile"`
	ProxyJump    string   `json:"proxyJump"`
	Group        string   `json:"group"`
	Tags         []string `json:"tags"`
	Notes        string   `json:"notes"`
	Pinned       bool     `json:"pinned"`
	Watch        []string `json:"watch"`
	Apps         []App    `json:"apps"`
	Pve          *PVE     `json:"pve,omitempty"`
	Auth         string   `json:"auth,omitempty"`
}

// Meta is the ryoport-owned sidecar entry, keyed by alias in remotes.json.
type Meta struct {
	Group  string   `json:"group"`
	Tags   []string `json:"tags"`
	Notes  string   `json:"notes"`
	Pinned bool     `json:"pinned"`
	Watch  []string `json:"watch"`
	Apps   []App    `json:"apps"`
	Pve    *PVE     `json:"pve,omitempty"`
	Auth   string   `json:"auth,omitempty"`
}

// App is a web service reachable on a host: a shortcut you open plus, when it
// answers HTTP, a live up/latency reading. Glance's monitor and bookmarks fused
// and scoped to one berth. Sidecar metadata, never an ssh_config fact.
type App struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// PVE is a host's Proxmox endpoint: the API base (https://host:8006) and an API
// token (USER@REALM!TOKENID=SECRET). Stored in the sidecar beside the ssh facts,
// so one berth is both an ssh host and a Proxmox control surface. Insecure
// accepts the self-signed cert a fresh cluster ships with.
type PVE struct {
	URL      string `json:"url"`
	Token    string `json:"token"`
	Insecure bool   `json:"insecure"`
}

// Guest is one Proxmox VM (qemu) or container (lxc) from /cluster/resources.
type Guest struct {
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	Node   string  `json:"node"`
	Type   string  `json:"type"`
	CPU    float64 `json:"cpu"`
	Mem    int64   `json:"mem"`
	MaxMem int64   `json:"maxmem"`
	Uptime int64   `json:"uptime"`
}

// resolveConfig runs `ssh -G alias` and returns the first value seen per keyword
// (ssh_config is first-match-wins, and -G lists IdentityFile defaults in order).
func resolveConfig(alias string) map[string]string {
	m := map[string]string{}
	cmd := exec.Command("ssh", "-G", alias)
	b, err := cmd.Output()
	if err != nil {
		return m
	}
	for _, ln := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(ln), " ")
		if !ok {
			continue
		}
		k = strings.ToLower(k)
		if _, seen := m[k]; !seen {
			m[k] = v
		}
	}
	return m
}

// resolveHostPort extracts the dial target from a resolved config.
func resolveHostPort(m map[string]string) (string, int) {
	host := m["hostname"]
	port := 22
	if p, err := strconv.Atoi(m["port"]); err == nil && p > 0 {
		port = p
	}
	return host, port
}

// buildHost merges the resolved ssh_config facts with any sidecar metadata.
func buildHost(alias string, side map[string]Meta) Host {
	m := resolveConfig(alias)
	h := Host{
		Alias:        alias,
		HostName:     m["hostname"],
		User:         m["user"],
		IdentityFile: m["identityfile"],
		Tags:         []string{},
		Watch:        []string{},
		Apps:         []App{},
	}
	_, h.Port = resolveHostPort(m)
	if pj := m["proxyjump"]; pj != "" && pj != "none" {
		h.ProxyJump = pj
	}
	if meta, ok := side[alias]; ok {
		h.Group = meta.Group
		h.Notes = meta.Notes
		h.Pinned = meta.Pinned
		if meta.Tags != nil {
			h.Tags = meta.Tags
		}
		if meta.Watch != nil {
			h.Watch = meta.Watch
		}
		if meta.Apps != nil {
			h.Apps = meta.Apps
		}
		if meta.Pve != nil {
			h.Pve = meta.Pve
		}
		h.Auth = meta.Auth
	}
	return h
}

// listAliases scans ~/.ssh/config (following Include globs) for Host aliases,
// skipping wildcard/negated patterns which are not real hosts.
func listAliases() []string {
	var aliases []string
	seen := map[string]bool{}
	visited := map[string]bool{}
	var scan func(path string)
	scan = func(path string) {
		if visited[path] {
			return
		}
		visited[path] = true
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		for _, ln := range strings.Split(string(data), "\n") {
			kw, args := splitKV(ln)
			switch strings.ToLower(kw) {
			case "host":
				for _, pat := range strings.Fields(args) {
					if strings.ContainsAny(pat, "*?!") {
						continue // wildcard/negation pattern, not a host
					}
					if !seen[pat] {
						seen[pat] = true
						aliases = append(aliases, pat)
					}
				}
			case "include":
				for _, inc := range strings.Fields(args) {
					for _, f := range expandInclude(inc) {
						scan(f)
					}
				}
			}
		}
	}
	scan(sshConfigPath())
	return aliases
}

// expandInclude resolves an Include argument (relative paths are anchored at
// ~/.ssh, per ssh_config) into matching files via glob.
func expandInclude(inc string) []string {
	p := expandTilde(inc)
	if !filepath.IsAbs(p) {
		p = filepath.Join(sshDir(), p)
	}
	matches, err := filepath.Glob(p)
	if err != nil {
		return nil
	}
	// ssh's glob(3) skips leading-dot entries, so our .bak backups (written by
	// atomicWrite) are invisible to `Include config.d/*`; skip them here too so
	// our alias scan matches what ssh actually reads.
	kept := matches[:0]
	for _, m := range matches {
		if !strings.HasPrefix(filepath.Base(m), ".") {
			kept = append(kept, m)
		}
	}
	return kept
}

// splitKV separates a config line's keyword from its arguments; a keyword and
// its args may be split by whitespace or a single '='. Blank/comment lines
// return an empty keyword.
func splitKV(line string) (string, string) {
	t := strings.TrimSpace(line)
	if t == "" || t[0] == '#' {
		return "", ""
	}
	i := strings.IndexAny(t, " \t=")
	if i < 0 {
		return t, ""
	}
	return t[:i], strings.TrimLeft(t[i:], " \t=")
}

func cmdList() {
	aliases := listAliases()
	side := readSidecar()
	hosts := make([]Host, len(aliases))
	sem := make(chan struct{}, poolSize)
	var wg sync.WaitGroup
	for i, a := range aliases {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, a string) {
			defer wg.Done()
			defer func() { <-sem }()
			hosts[i] = buildHost(a, side)
		}(i, a)
	}
	wg.Wait()
	sort.Slice(hosts, func(i, j int) bool {
		if hosts[i].Group != hosts[j].Group {
			return hosts[i].Group < hosts[j].Group
		}
		return hosts[i].Alias < hosts[j].Alias
	})
	if hosts == nil {
		hosts = []Host{}
	}
	out(hosts)
}

// --- ping ------------------------------------------------------------------

// pingOne does a Go TCP dial to the resolved host:port and reports liveness and
// latency; it stays cheap (no ssh subprocess) so a fleet sweep is fast.
func pingOne(alias string) map[string]any {
	host, port := resolveHostPort(resolveConfig(alias))
	res := map[string]any{"alias": alias, "up": false, "rttMs": -1, "sshUp": false}
	if host == "" {
		return res
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return res
	}
	conn.Close()
	rtt := int(time.Since(start).Milliseconds())
	// sshUp mirrors up: a deeper auth check is skipped to keep ping cheap
	// (the contract allows this), so up==sshUp for the fast path.
	res["up"], res["rttMs"], res["sshUp"] = true, rtt, true
	return res
}

func cmdPingAll() {
	aliases := listAliases()
	results := make([]map[string]any, len(aliases))
	sem := make(chan struct{}, poolSize)
	var wg sync.WaitGroup
	for i, a := range aliases {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, a string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = pingOne(a)
		}(i, a)
	}
	wg.Wait()
	if results == nil {
		results = []map[string]any{}
	}
	out(results)
}

// --- probe -----------------------------------------------------------------

// Probe is the health snapshot for one host. Emitted only on success, so the
// numeric fields are shown even when zero (the GUI expects the full shape).
type Probe struct {
	Alias          string            `json:"alias"`
	OK             bool              `json:"ok"`
	Host           string            `json:"host"`
	Kernel         string            `json:"kernel"`
	Distro         string            `json:"distro"`
	UptimeS        int64             `json:"uptimeS"`
	CPUPercent     float64           `json:"cpuPercent"`
	Load1          float64           `json:"load1"`
	Load5          float64           `json:"load5"`
	Load15         float64           `json:"load15"`
	CPUs           int               `json:"cpus"`
	MemTotalKb     int64             `json:"memTotalKb"`
	MemAvailKb     int64             `json:"memAvailKb"`
	DiskTotalKb    int64             `json:"diskTotalKb"`
	DiskUsedKb     int64             `json:"diskUsedKb"`
	DiskPct        int               `json:"diskPct"`
	DiskReadBytes  int64             `json:"diskReadBytes"`
	DiskWriteBytes int64             `json:"diskWriteBytes"`
	NetRxBytes     int64             `json:"netRxBytes"`
	NetTxBytes     int64             `json:"netTxBytes"`
	FailedUnits    int               `json:"failedUnits"`
	Logins         int               `json:"logins"`
	TopProcs       []string          `json:"topProcs"`
	Services       map[string]string `json:"services"`
}

// baseProbeScript is the one-shot POSIX-sh health script (research §5). It reads
// /proc directly and emits key=value lines, which parse cleanly in Go and run on
// any POSIX box (busybox included). Fed to a remote `sh -s` over stdin (see
// probeOne), so a non-POSIX login shell can't touch it; the $ escaping here is
// for that remote sh.
const baseProbeScript = `LC_ALL=C
echo "ok=1"
echo "host=$(hostname 2>/dev/null)"
echo "kernel=$(uname -r)"
[ -r /etc/os-release ] && . /etc/os-release && echo "distro=$PRETTY_NAME"
read u _ < /proc/uptime; echo "uptime_s=${u%.*}"
read _ u1 n1 s1 i1 w1 q1 sq1 st1 _ < /proc/stat
t1=$((u1+n1+s1+i1+w1+q1+sq1+st1)); idle1=$((i1+w1))
sleep 1
read _ u2 n2 s2 i2 w2 q2 sq2 st2 _ < /proc/stat
t2=$((u2+n2+s2+i2+w2+q2+sq2+st2)); idle2=$((i2+w2))
dt=$((t2-t1)); di=$((idle2-idle1))
awk -v dt="$dt" -v di="$di" "BEGIN{if(dt>0) printf \"cpu_pct=%.1f\n\",100*(dt-di)/dt; else print \"cpu_pct=0\"}"
read l1 l5 l15 _ < /proc/loadavg; echo "load1=$l1"; echo "load5=$l5"; echo "load15=$l15"
echo "cpus=$(nproc 2>/dev/null || grep -c ^processor /proc/cpuinfo)"
awk "/^MemTotal:/{t=\$2}/^MemAvailable:/{a=\$2}END{print \"mem_total_kb=\"t; print \"mem_avail_kb=\"a}" /proc/meminfo
df -P -k / | awk "NR==2{print \"disk_total_kb=\"\$2; print \"disk_used_kb=\"\$3; print \"disk_avail_kb=\"\$4; print \"disk_pct=\"\$5}"
set -- $(awk '{r+=$3; w+=$7} END{print r*512, w*512}' /sys/block/*/stat 2>/dev/null)
echo "disk_read_bytes=${1:-0}"; echo "disk_write_bytes=${2:-0}"
set -- $(awk -F'[: ]+' 'NR>2 && $2!="lo"{rx+=$3; tx+=$11} END{print rx+0, tx+0}' /proc/net/dev)
echo "net_rx_bytes=${1:-0}"; echo "net_tx_bytes=${2:-0}"
command -v systemctl >/dev/null 2>&1 && echo "failed_units=$(systemctl --failed --no-legend 2>/dev/null | wc -l)"
echo "logins=$(who 2>/dev/null | wc -l)"
ps -eo pcpu,comm --sort=-pcpu 2>/dev/null | awk "NR>1&&NR<=4{printf \"top%d=%s:%s\n\",NR-1,\$2,\$1}"`

// buildProbeScript appends a `systemctl is-active` line per watched service so
// the whole probe stays one round trip. Unit names are validated to keep shell
// metacharacters out of the remote command.
func buildProbeScript(watch []string) string {
	var b strings.Builder
	b.WriteString(baseProbeScript)
	for _, svc := range watch {
		if !validUnit(svc) {
			continue
		}
		b.WriteString("\necho \"service." + svc + "=$(systemctl is-active " + svc + " 2>/dev/null || echo unknown)\"")
	}
	return b.String()
}

func validUnit(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_' || r == '@' || r == ':':
		default:
			return false
		}
	}
	return true
}

// probeOne runs the health script over ssh with a hard deadline and returns
// either a filled Probe (success) or a {alias,ok:false,error,needAuth} map
// (unreachable or blocked). needAuth tells the GUI the box is answering but
// ryoport cannot prove it is healthy yet, so it can offer a save-password fix
// instead of showing dead graphs over a quietly failing probe.
func probeOne(alias string, watch []string, timeout time.Duration, usePassword bool) any {
	ensureSSHDir()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	opts, env := nonInteractiveOpts(), os.Environ()
	if usePassword {
		// a keyless box with a saved password: drop BatchMode and let ssh pull
		// the secret from the keyring via askpass, capped by the same deadline.
		opts = passwordOpts()
		env = append(env, "SSH_ASKPASS="+ensureAskpass(), "SSH_ASKPASS_REQUIRE=force", "RYOPORT_ALIAS="+alias)
	}
	// feed the script to an explicit POSIX `sh -s` over stdin, so a fish or csh
	// login shell on the remote can't mangle it.
	cmd := exec.CommandContext(ctx, "ssh", append(opts, alias, "sh", "-s")...)
	cmd.Env = env
	cmd.Stdin = strings.NewReader(buildProbeScript(watch))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	kv := parseKV(string(b))
	if err != nil || kv["ok"] != "1" {
		msg, needAuth := probeErr(ctx, err, stderr.String(), usePassword)
		return map[string]any{"alias": alias, "ok": false, "error": msg, "needAuth": needAuth}
	}
	return fillProbe(alias, kv)
}

// probeErr turns an ssh failure into one actionable line. "authentication
// required" (needAuth true) means the box answered and only wants a credential
// ryoport does not have: BatchMode refused to prompt, or a saved keyring
// password was rejected. Anything else keeps ssh's own last line.
func probeErr(ctx context.Context, err error, stderr string, usePassword bool) (string, bool) {
	s := strings.TrimSpace(stderr)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		s = s[i+1:] // ssh prints the actionable reason last
	}
	low := strings.ToLower(s)
	authBlocked := strings.Contains(low, "permission denied") ||
		strings.Contains(low, "too many authentication failures") ||
		strings.Contains(low, "authentications that can continue") ||
		strings.Contains(low, "no supported authentication methods")
	if authBlocked {
		if usePassword {
			return "saved password rejected", false
		}
		return "authentication required", true
	}
	if strings.Contains(low, "host key verification failed") {
		return "host key changed (known_hosts)", false
	}
	if strings.Contains(low, "unable to negotiate") {
		return "ssh handshake failed (very old server?)", false
	}
	if s != "" {
		return s, false
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "timed out", false
	}
	if err != nil {
		return err.Error(), false
	}
	return "unreachable", false
}

// parseKV turns the probe's key=value output into a map.
func parseKV(s string) map[string]string {
	m := map[string]string{}
	for _, ln := range strings.Split(s, "\n") {
		if k, v, ok := strings.Cut(ln, "="); ok {
			m[k] = v
		}
	}
	return m
}

// fillProbe type-converts the parsed key=value map into a Probe.
func fillProbe(alias string, kv map[string]string) Probe {
	p := Probe{
		Alias:          alias,
		OK:             true,
		Host:           kv["host"],
		Kernel:         kv["kernel"],
		Distro:         kv["distro"],
		UptimeS:        atoi64(kv["uptime_s"]),
		CPUPercent:     atof(kv["cpu_pct"]),
		Load1:          atof(kv["load1"]),
		Load5:          atof(kv["load5"]),
		Load15:         atof(kv["load15"]),
		CPUs:           atoi(kv["cpus"]),
		MemTotalKb:     atoi64(kv["mem_total_kb"]),
		MemAvailKb:     atoi64(kv["mem_avail_kb"]),
		DiskTotalKb:    atoi64(kv["disk_total_kb"]),
		DiskUsedKb:     atoi64(kv["disk_used_kb"]),
		DiskPct:        atoi(strings.TrimSuffix(kv["disk_pct"], "%")),
		DiskReadBytes:  atoi64(kv["disk_read_bytes"]),
		DiskWriteBytes: atoi64(kv["disk_write_bytes"]),
		NetRxBytes:     atoi64(kv["net_rx_bytes"]),
		NetTxBytes:     atoi64(kv["net_tx_bytes"]),
		FailedUnits:    atoi(kv["failed_units"]),
		Logins:         atoi(kv["logins"]),
		TopProcs:       []string{},
		Services:       map[string]string{},
	}
	for i := 1; ; i++ {
		v, ok := kv["top"+strconv.Itoa(i)]
		if !ok {
			break
		}
		p.TopProcs = append(p.TopProcs, v)
	}
	for k, v := range kv {
		if svc, ok := strings.CutPrefix(k, "service."); ok {
			p.Services[svc] = v
		}
	}
	return p
}

func cmdProbe(alias string) {
	m := readSidecar()[alias]
	res := probeOne(alias, m.Watch, 15*time.Second, m.Auth == "password")
	out(res)
	if m, ok := res.(map[string]any); ok && m["ok"] == false {
		os.Exit(1) // single-host probe: signal unreachable to the caller
	}
}

func cmdProbeAll() {
	aliases := listAliases()
	side := readSidecar()
	results := make([]any, len(aliases))
	sem := make(chan struct{}, poolSize)
	var wg sync.WaitGroup
	for i, a := range aliases {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, a string) {
			defer wg.Done()
			defer func() { <-sem }()
			// per-host deadline so one dead host can't stall the batch.
			results[i] = probeOne(a, side[a].Watch, 8*time.Second, side[a].Auth == "password")
		}(i, a)
	}
	wg.Wait()
	if results == nil {
		results = []any{}
	}
	out(results) // exit 0: individual failures live in the array
}

// --- apps (web service shortcuts + http monitor) ---------------------------

// AppStatus is one app's HTTP reachability: state is up|warn|down, ms the round
// trip, code the HTTP status (0 when the service never answered).
type AppStatus struct {
	Alias string `json:"alias"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	State string `json:"state"`
	Ms    int64  `json:"ms"`
	Code  int    `json:"code"`
}

// checkApp does one GET with a short deadline, tolerating self-signed TLS since
// homelab services routinely use it; the body is never read. up = a <400 answer,
// warn = the service answered but errored (>=400), down = it never answered.
// Redirects are not followed: a 3xx already proves the service is there.
func checkApp(alias string, app App, timeout time.Duration) AppStatus {
	s := AppStatus{Alias: alias, Name: app.Name, URL: app.URL, State: "down"}
	req, err := http.NewRequest(http.MethodGet, app.URL, nil)
	if err != nil {
		return s
	}
	client := &http.Client{
		Timeout:       timeout,
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	start := time.Now()
	resp, err := client.Do(req)
	s.Ms = time.Since(start).Milliseconds()
	if err != nil {
		return s
	}
	resp.Body.Close()
	s.Code = resp.StatusCode
	if resp.StatusCode < 400 {
		s.State = "up"
	} else {
		s.State = "warn"
	}
	return s
}

// checkApps probes one host's apps concurrently, skipping entries with no URL.
func checkApps(alias string, apps []App) []AppStatus {
	res := []AppStatus{}
	sem := make(chan struct{}, poolSize)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, app := range apps {
		if strings.TrimSpace(app.URL) == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(app App) {
			defer wg.Done()
			defer func() { <-sem }()
			st := checkApp(alias, app, 4*time.Second)
			mu.Lock()
			res = append(res, st)
			mu.Unlock()
		}(app)
	}
	wg.Wait()
	return res
}

// cmdAppCheck prints one host's app statuses.
func cmdAppCheck(alias string) {
	out(checkApps(alias, readSidecar()[alias].Apps))
}

// cmdAppCheckAll prints app statuses across every host as one flat array.
func cmdAppCheckAll() {
	side := readSidecar()
	all := []AppStatus{}
	for alias, meta := range side {
		all = append(all, checkApps(alias, meta.Apps)...)
	}
	out(all)
}

// --- proxmox (dedicated PVE API) -------------------------------------------

// pveClient is an HTTP client for one endpoint; it skips TLS verification only
// when the host is flagged insecure (a fresh cluster's self-signed cert).
func pveClient(p PVE) *http.Client {
	return &http.Client{
		Timeout:   8 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: p.Insecure}},
	}
}

// pveDo runs one authenticated request and returns the body on a 2xx, or an
// error carrying the status. The API token rides in the header, never the URL.
func pveDo(p PVE, method, path string) ([]byte, error) {
	req, err := http.NewRequest(method, strings.TrimRight(p.URL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "PVEAPIToken="+p.Token)
	resp, err := pveClient(p).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s", strings.TrimSpace(resp.Status))
	}
	return b, nil
}

// pveGuests lists every VM and container across the cluster in one call, sorted
// by vmid; node/storage rows from /cluster/resources are dropped.
func pveGuests(p PVE) ([]Guest, error) {
	b, err := pveDo(p, http.MethodGet, "/api2/json/cluster/resources")
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Data []Guest `json:"data"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil {
		return nil, err
	}
	guests := []Guest{}
	for _, g := range wrap.Data {
		if g.Type == "qemu" || g.Type == "lxc" {
			guests = append(guests, g)
		}
	}
	sort.Slice(guests, func(i, j int) bool { return guests[i].VMID < guests[j].VMID })
	return guests, nil
}

// pveValidNode guards the node segment that goes into a request path: only the
// conservative host charset, so a crafted value can't reshape the URL.
func pveValidNode(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.' || r == '_') {
			return false
		}
	}
	return true
}

// pveAction starts/stops/shuts down one guest; Proxmox routes to the owning node.
func pveAction(p PVE, node, typ, vmid, action string) error {
	_, err := pveDo(p, http.MethodPost, fmt.Sprintf("/api2/json/nodes/%s/%s/%s/status/%s", node, typ, vmid, action))
	return err
}

// pveOf returns a host's Proxmox endpoint or dies if none is configured.
func pveOf(alias string) PVE {
	meta, ok := readSidecar()[alias]
	if !ok || meta.Pve == nil || meta.Pve.URL == "" || meta.Pve.Token == "" {
		die("no proxmox endpoint configured for %q", alias)
	}
	return *meta.Pve
}

func cmdPveGuests(alias string) {
	guests, err := pveGuests(pveOf(alias))
	if err != nil {
		die("proxmox: %v", err)
	}
	out(guests)
}

func cmdPveAction(rest []string) {
	if len(rest) < 5 {
		die("usage: ryossh pveaction <alias> <node> <qemu|lxc> <vmid> <start|stop|shutdown|reboot>")
	}
	alias, node, typ, vmid, action := rest[0], rest[1], rest[2], rest[3], rest[4]
	if typ != "qemu" && typ != "lxc" {
		die("type must be qemu or lxc")
	}
	switch action {
	case "start", "stop", "shutdown", "reboot":
	default:
		die("action must be start, stop, shutdown or reboot")
	}
	if !pveValidNode(node) {
		die("bad node name")
	}
	if _, err := strconv.Atoi(vmid); err != nil {
		die("vmid must be a number")
	}
	if err := pveAction(pveOf(alias), node, typ, vmid, action); err != nil {
		die("proxmox: %v", err)
	}
	out(map[string]any{"ok": true})
}

func atoi(s string) int     { n, _ := strconv.Atoi(strings.TrimSpace(s)); return n }
func atoi64(s string) int64 { n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64); return n }
func atof(s string) float64 { f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64); return f }

// --- connect ---------------------------------------------------------------

// sshArgv is the interactive ssh command for a host: keepalive so idle sessions
// through NAT/firewalls survive. The alias alone lets ~/.ssh/config apply.
func sshArgv(alias string) []string {
	return []string{"ssh", "-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=3", alias}
}

func cmdConnect(rest []string) {
	if len(rest) < 1 {
		die("usage: ryossh connect <alias> [--cmd]")
	}
	alias := rest[0]
	if len(rest) > 1 && rest[1] == "--cmd" {
		out(sshArgv(alias)) // GUI spawns it itself
		return
	}
	argv := terminalArgv(alias)
	cmd := exec.Command(argv[0], argv[1:]...)
	// Setsid detaches the child into its own session so the terminal survives
	// ryoport exiting; nil stdio wires it to /dev/null.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Env = connectEnv(alias) // adds SSH_ASKPASS for a saved-password host
	if err := cmd.Start(); err != nil {
		die("launch %s: %v", argv[0], err)
	}
	cmd.Process.Release()
	out(map[string]any{"launched": true, "alias": alias})
}

// connectArgv is the ssh invocation for a host. In kitty a keyed host runs
// through the ssh kitten (terminfo, shell integration, OSC-52 clipboard); a
// saved-password host uses plain ssh instead, since the kitten ignores
// SSH_ASKPASS and would prompt. Keepalive keeps an idle session alive through NAT.
func connectArgv(alias, term string) []string {
	opts := []string{"-o", "ServerAliveInterval=30", "-o", "ServerAliveCountMax=3", alias}
	if filepath.Base(term) == "kitty" && readSidecar()[alias].Auth != "password" {
		return append([]string{"kitten", "ssh"}, opts...)
	}
	return append([]string{"ssh"}, opts...)
}

// ryoportAppID is the window class every ryoport terminal launch asks for.
// Ghostty validates --class as a GTK application id, which needs at least one
// dot, so a bare "ryoport-ssh" is rejected and silently ignored: the window
// keeps com.mitchellh.ghostty and the float rule can never match it. A
// reverse-DNS id is valid for every terminal on the switch below.
const ryoportAppID = "dev.ryoku.ryoport_ssh"

// ryoportConsoleAppID is the window class for the direct GUI console clients
// (RDP, VNC) launched outside a terminal, so the compositors can float them.
const ryoportConsoleAppID = "dev.ryoku.ryoport_console"

// terminalArgv wraps the connect command in a real terminal window, respecting
// $TERMINAL and its invocation quirks (kitty/foot take trailing args; alacritty/
// xterm need -e; wezterm needs `start --`; ghostty wants `--class=` with an
// equals and a GTK-valid id). A failed connect holds on the error via a local
// read, since a bare terminal drops into a login shell that buries it.
func terminalArgv(alias string) []string {
	term := os.Getenv("TERMINAL")
	if term == "" {
		term = "kitty"
	}
	return terminalWrap(term, "ssh: "+alias, connectArgv(alias, term))
}

// terminalWrap is the shared terminal invocation behind terminalArgv: the
// float-class window, a title, and a hold-on-failure so a bare terminal never
// buries its own error under a login shell.
func terminalWrap(term, title string, conn []string) []string {
	hold := `"$@" || { printf "\n── press enter to close ──\n"; read _; }`
	prog := append([]string{"sh", "-c", hold, "_"}, conn...)
	switch filepath.Base(term) {
	case "kitty":
		return append([]string{term, "--title", title, "--class=" + ryoportAppID}, prog...)
	case "ghostty":
		// ghostty has no --title; the class is what identifies the window.
		return append([]string{term, "--class=" + ryoportAppID, "-e"}, prog...)
	case "foot":
		return append([]string{term, "--title", title, "--app-id=" + ryoportAppID}, prog...)
	case "alacritty":
		return append([]string{term, "--class=" + ryoportAppID, "-e"}, prog...)
	case "wezterm":
		return append([]string{term, "start", "--"}, prog...)
	default: // xterm and most others accept -e <program...>
		return append([]string{term, "-e"}, prog...)
	}
}

// --- saved passwords (Secret Service keyring) ------------------------------

// secretAttr keys ryoport's entries in the login keyring; the alias is the value
// so one lookup finds a host's password. The secret lives only in the Secret
// Service -- never in the sidecar JSON, ssh_config, argv, or a file.
const secretAttr = "ryoport-alias"

func storeSecret(alias, pw string) error {
	cmd := exec.Command("secret-tool", "store", "--label", "ryoport: "+alias, secretAttr, alias)
	cmd.Stdin = strings.NewReader(pw)
	return cmd.Run()
}

func clearSecret(alias string) {
	exec.Command("secret-tool", "clear", secretAttr, alias).Run()
}

// askpassPath is the helper ssh runs to answer a password prompt; it reads the
// secret for $RYOPORT_ALIAS straight from the keyring, so the password is fetched
// on demand and never lands in argv, the environment, or a file.
func askpassPath() string { return filepath.Join(stateDir(), "askpass") }

func ensureAskpass() string {
	p := askpassPath()
	script := "#!/bin/sh\nexec secret-tool lookup " + secretAttr + " \"$RYOPORT_ALIAS\"\n"
	if b, err := os.ReadFile(p); err != nil || string(b) != script {
		os.MkdirAll(filepath.Dir(p), 0700)
		os.WriteFile(p, []byte(script), 0700)
	}
	return p
}

// connectEnv adds the askpass wiring when a host has a saved password, so ssh
// pulls it from the keyring at the password prompt. SSH_ASKPASS_REQUIRE=force
// makes ssh use the helper even with a tty attached (OpenSSH 8.4+).
func connectEnv(alias string) []string {
	env := os.Environ()
	if readSidecar()[alias].Auth == "password" {
		env = append(env,
			"SSH_ASKPASS="+ensureAskpass(),
			"SSH_ASKPASS_REQUIRE=force",
			"RYOPORT_ALIAS="+alias,
		)
	}
	return env
}

// cmdSetPass reads a password from stdin and stores it in the login keyring keyed
// by alias, marking the sidecar auth=password. The secret never touches argv.
func cmdSetPass(alias string) {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		die("secret-tool not found: install libsecret to save passwords")
	}
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	pw := strings.TrimRight(line, "\r\n")
	if pw == "" {
		die("no password on stdin")
	}
	if err := storeSecret(alias, pw); err != nil {
		die("keyring store failed: %v", err)
	}
	side := readSidecar()
	m := side[alias]
	m.Auth = "password"
	side[alias] = m
	writeSidecar(side)
	out(map[string]any{"ok": true, "alias": alias})
}

// cmdClearPass forgets a host's saved password: drop the keyring entry and the
// sidecar auth flag, so the next connect prompts normally.
func cmdClearPass(alias string) {
	clearSecret(alias)
	side := readSidecar()
	if m, ok := side[alias]; ok {
		m.Auth = ""
		side[alias] = m
		writeSidecar(side)
	}
	out(map[string]any{"ok": true, "alias": alias})
}

// --- keys ------------------------------------------------------------------

// AgentKey is an ssh-agent-loaded key from `ssh-add -l`.
type AgentKey struct {
	Bits        int    `json:"bits"`
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment"`
	Type        string `json:"type"`
}

// FileKey is a local ~/.ssh/*.pub key from `ssh-keygen -lf`.
type FileKey struct {
	Path        string `json:"path"`
	Fingerprint string `json:"fingerprint"`
	Type        string `json:"type"`
	Comment     string `json:"comment"`
}

func cmdKeys() {
	agent := []AgentKey{}
	if b, err := exec.Command("ssh-add", "-l").Output(); err == nil {
		for _, ln := range strings.Split(string(b), "\n") {
			if bits, fp, typ, comment, ok := parseKeyLine(ln); ok {
				agent = append(agent, AgentKey{Bits: bits, Fingerprint: fp, Comment: comment, Type: typ})
			}
		}
	}
	files := []FileKey{}
	pubs, _ := filepath.Glob(filepath.Join(sshDir(), "*.pub"))
	sort.Strings(pubs)
	for _, pub := range pubs {
		b, err := exec.Command("ssh-keygen", "-lf", pub).Output()
		if err != nil {
			continue
		}
		if _, fp, typ, comment, ok := parseKeyLine(strings.TrimRight(string(b), "\n")); ok {
			files = append(files, FileKey{Path: tildeShorten(pub), Fingerprint: fp, Type: typ, Comment: comment})
		}
	}
	out(map[string]any{"agent": agent, "files": files})
}

// parseKeyLine parses the shared `<bits> <fingerprint> <comment...> (<TYPE>)`
// format emitted by both ssh-add -l and ssh-keygen -lf.
func parseKeyLine(ln string) (bits int, fingerprint, typ, comment string, ok bool) {
	f := strings.Fields(ln)
	if len(f) < 3 || !strings.Contains(f[1], ":") {
		return 0, "", "", "", false // "no identities" / agent-not-running / blank
	}
	bits, _ = strconv.Atoi(f[0])
	fingerprint = f[1]
	last := f[len(f)-1]
	if strings.HasPrefix(last, "(") && strings.HasSuffix(last, ")") {
		typ = last[1 : len(last)-1]
		comment = strings.Join(f[2:len(f)-1], " ")
	} else {
		comment = strings.Join(f[2:], " ")
	}
	return bits, fingerprint, typ, comment, true
}

// --- copyid / keygen / knownremove ----------------------------------------

func cmdCopyID(rest []string) {
	if len(rest) < 1 {
		die("usage: ryossh copyid <alias> [pubkey]")
	}
	alias := rest[0]
	pub := ""
	if len(rest) > 1 && rest[1] != "" {
		pub = expandTilde(rest[1])
	} else if id := resolveConfig(alias)["identityfile"]; id != "" {
		// default to the key this host connects with, so the copied key matches
		// what ssh offers, not a stray ~/.ssh/id_* or an empty agent.
		p := expandTilde(id) + ".pub"
		if _, err := os.Stat(p); err == nil {
			pub = p
		}
	}
	args := []string{}
	if pub != "" {
		args = append(args, "-i", pub)
	}
	args = append(args, alias)
	cmd := exec.Command("ssh-copy-id", args...)
	// interactive: inherit stdio so a passphrase/host prompt is visible; a saved
	// password is answered from the keyring via askpass (connectEnv).
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stderr, os.Stderr
	cmd.Env = connectEnv(alias)
	if err := cmd.Run(); err != nil {
		die("ssh-copy-id: %v", err)
	}
	out(map[string]any{"ok": true})
}

func cmdKeygen(rest []string) {
	if len(rest) < 1 {
		die("usage: ryossh keygen <name> [type]")
	}
	name := rest[0]
	typ := "ed25519"
	if len(rest) > 1 && rest[1] != "" {
		typ = rest[1]
	}
	ensureSSHDir()
	key := filepath.Join(sshDir(), name)
	// refuse to clobber an existing key.
	if _, err := os.Stat(key); err == nil {
		die("key already exists: %s", tildeShorten(key))
	}
	// -N "" means no passphrase: these are GUI-generated convenience keys; the
	// user should still ssh-copy-id and can add a passphrase later.
	cmd := exec.Command("ssh-keygen", "-t", typ, "-f", key, "-C", name+"@ryoku", "-N", "")
	if b, err := cmd.CombinedOutput(); err != nil {
		die("ssh-keygen: %v: %s", err, strings.TrimSpace(string(b)))
	}
	pub := key + ".pub"
	fp := ""
	if b, err := exec.Command("ssh-keygen", "-lf", pub).Output(); err == nil {
		if _, f, _, _, ok := parseKeyLine(strings.TrimRight(string(b), "\n")); ok {
			fp = f
		}
	}
	out(map[string]any{"ok": true, "path": tildeShorten(pub), "fingerprint": fp})
}

func cmdKnownRemove(alias string) {
	host, port := resolveHostPort(resolveConfig(alias))
	if host == "" {
		host = alias
	}
	target := host
	if port != 22 {
		target = fmt.Sprintf("[%s]:%d", host, port) // non-standard port form
	}
	if b, err := exec.Command("ssh-keygen", "-R", target).CombinedOutput(); err != nil {
		die("ssh-keygen -R: %v: %s", err, strings.TrimSpace(string(b)))
	}
	out(map[string]any{"ok": true})
}

// --- add / remove (ryoport-managed include + sidecar) ----------------------

func cmdAdd(rest []string) {
	if len(rest) < 1 {
		die("usage: ryossh add <json>")
	}
	var h Host
	if err := json.Unmarshal([]byte(rest[0]), &h); err != nil {
		die("invalid host JSON: %v", err)
	}
	if h.Alias == "" || h.HostName == "" {
		die("host JSON needs at least alias and hostName")
	}
	// the include is pulled in globally by `Include config.d/*`, so a stray newline
	// or leading '-' in any field could inject a directive or corrupt resolution for
	// every host; whitespace in alias/hostName would split them into extra patterns.
	for _, f := range []string{h.Alias, h.HostName, h.User, h.IdentityFile, h.ProxyJump} {
		if strings.ContainsAny(f, "\n\r") || strings.HasPrefix(f, "-") {
			die("host fields must not contain newlines or start with '-'")
		}
	}
	if strings.ContainsAny(h.Alias, " \t") || strings.ContainsAny(h.HostName, " \t") {
		die("alias and hostName must not contain whitespace")
	}
	ensureSSHDir()
	ensureIncludeLine()

	header, blocks := loadInclude(includeFilePath())
	stanza := incBlock{alias: h.Alias, lines: stanzaLines(h)}
	replaced := false
	for i := range blocks {
		if blocks[i].alias == h.Alias {
			blocks[i] = stanza // replace a prior ryoport stanza for this alias
			replaced = true
			break
		}
	}
	if !replaced {
		blocks = append(blocks, stanza)
	}
	writeInclude(includeFilePath(), header, blocks)

	side := readSidecar()
	prev := side[h.Alias]
	side[h.Alias] = Meta{
		Group:  h.Group,
		Tags:   nonNil(h.Tags),
		Notes:  h.Notes,
		Pinned: h.Pinned,
		Watch:  nonNil(h.Watch),
		Apps:   h.Apps,
		Pve:    h.Pve,
		Auth:   prev.Auth, // a saved password survives an edit-save
	}
	writeSidecar(side)
	out(map[string]any{"ok": true})
}

func cmdRemove(alias string) {
	header, blocks := loadInclude(includeFilePath())
	kept := blocks[:0]
	for _, b := range blocks {
		if b.alias != alias {
			kept = append(kept, b)
		}
	}
	if len(kept) != len(blocks) {
		writeInclude(includeFilePath(), header, kept)
	}
	clearSecret(alias) // drop the keyring password too, if any
	side := readSidecar()
	if _, ok := side[alias]; ok {
		delete(side, alias)
		writeSidecar(side)
	}
	out(map[string]any{"ok": true}) // idempotent: ok even if nothing matched
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// stanzaLines renders a host as an ssh_config stanza for the include file.
func stanzaLines(h Host) []string {
	lines := []string{"Host " + h.Alias, "    HostName " + h.HostName}
	if h.User != "" {
		lines = append(lines, "    User "+h.User)
	}
	port := h.Port
	if port == 0 {
		port = 22
	}
	lines = append(lines, "    Port "+strconv.Itoa(port))
	if h.IdentityFile != "" {
		lines = append(lines, "    IdentityFile "+h.IdentityFile)
	}
	if h.ProxyJump != "" {
		lines = append(lines, "    ProxyJump "+h.ProxyJump)
	}
	return lines
}

// incBlock is one Host stanza in the ryoport include file.
type incBlock struct {
	alias string
	lines []string
}

// loadInclude parses the ryoport include file into a header (lines before the
// first Host) and one block per stanza. A missing file yields the managed header.
func loadInclude(path string) ([]string, []incBlock) {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{
			"# Managed by ryoport. Do not edit; use the ryoport app or `ryossh add`/`ryossh remove`.",
			"",
		}, nil
	}
	var header []string
	var blocks []incBlock
	ci := -1
	for _, ln := range strings.Split(string(data), "\n") {
		kw, args := splitKV(ln)
		if strings.EqualFold(kw, "host") {
			blocks = append(blocks, incBlock{alias: firstField(args), lines: []string{ln}})
			ci = len(blocks) - 1
		} else if ci >= 0 {
			blocks[ci].lines = append(blocks[ci].lines, ln)
		} else {
			header = append(header, ln)
		}
	}
	return header, blocks
}

// writeInclude renders header + blocks back to the include file atomically.
func writeInclude(path string, header []string, blocks []incBlock) {
	var b strings.Builder
	for _, ln := range header {
		b.WriteString(strings.TrimRight(ln, " \t"))
		b.WriteByte('\n')
	}
	for _, blk := range blocks {
		for _, ln := range blk.lines {
			if strings.TrimSpace(ln) == "" {
				continue // drop blank filler; one separator is added below
			}
			b.WriteString(strings.TrimRight(ln, " \t"))
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	atomicWrite(path, []byte(b.String()), 0600)
}

func firstField(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// ensureIncludeLine guarantees ~/.ssh/config starts with `Include config.d/*`,
// prepending it once if absent. The user's existing config is never rewritten
// beyond this one line (and a .bak is kept).
func ensureIncludeLine() {
	cfg := sshConfigPath()
	data, err := os.ReadFile(cfg)
	if err != nil {
		atomicWrite(cfg, []byte("Include config.d/*\n"), 0600)
		return
	}
	for _, ln := range strings.Split(string(data), "\n") {
		kw, args := splitKV(ln)
		if strings.EqualFold(kw, "include") {
			for _, f := range strings.Fields(args) {
				if f == "config.d/*" {
					return // already present
				}
			}
		}
	}
	atomicWrite(cfg, append([]byte("Include config.d/*\n"), data...), 0600)
}

// --- sidecar + atomic IO ---------------------------------------------------

// readSidecar loads remotes.json; a missing/corrupt file yields an empty map so
// no verb ever crashes on it.
func readSidecar() map[string]Meta {
	m := map[string]Meta{}
	data, err := os.ReadFile(sidecarPath())
	if err != nil {
		return m
	}
	json.Unmarshal(data, &m)
	if m == nil {
		m = map[string]Meta{}
	}
	return m
}

func writeSidecar(m map[string]Meta) {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		die("marshal sidecar: %v", err)
	}
	atomicWrite(sidecarPath(), append(b, '\n'), 0600)
}

// atomicWrite writes via a temp file + rename so a crash never leaves a
// half-written config, keeping a .bak of the previous content first.
func atomicWrite(path string, data []byte, perm os.FileMode) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		die("mkdir %s: %v", dir, err)
	}
	if old, err := os.ReadFile(path); err == nil {
		// dotfile-prefixed backup: a `ryoport.bak` next to the ryoport include
		// would be pulled in by `Include config.d/*`; `.ryoport.bak` is skipped
		// by ssh's glob(3), so backups never pollute the resolved config.
		bak := filepath.Join(dir, "."+filepath.Base(path)+".bak")
		os.WriteFile(bak, old, perm) // best-effort backup
	}
	tmp, err := os.CreateTemp(dir, ".ryossh-*")
	if err != nil {
		die("temp file: %v", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		die("write %s: %v", path, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		die("chmod %s: %v", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		die("close %s: %v", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		die("rename %s: %v", path, err)
	}
}

// --- tunnels ---------------------------------------------------------------

// Tunnel is one tracked `ssh -N` forward. JSON field names are the contract the
// QML tunnels panel parses against; kind is local|remote|dynamic, spec is the
// human forward string (e.g. L:15432:127.0.0.1:22), pid is the ssh process, and
// opened is the unix epoch it started.
type Tunnel struct {
	ID     string `json:"id"`
	Alias  string `json:"alias"`
	Kind   string `json:"kind"`
	Spec   string `json:"spec"`
	Pid    int    `json:"pid"`
	Opened int64  `json:"opened"`
}

// stateDir is the XDG state home for ryoport (defaults to ~/.local/state); the
// tunnels file lives here rather than in ~/.config since it is runtime state,
// not user configuration.
func stateDir() string {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "ryoku", "ryoport")
	}
	return filepath.Join(home(), ".local", "state", "ryoku", "ryoport")
}

func tunnelsPath() string { return filepath.Join(stateDir(), "tunnels.json") }

// readTunnels loads the state file; a missing or corrupt file yields an empty
// list so no verb ever crashes on it (same tolerance as readSidecar).
func readTunnels() []Tunnel {
	data, err := os.ReadFile(tunnelsPath())
	if err != nil {
		return nil
	}
	var ts []Tunnel
	if err := json.Unmarshal(data, &ts); err != nil {
		return nil
	}
	return ts
}

func writeTunnels(ts []Tunnel) {
	if ts == nil {
		ts = []Tunnel{}
	}
	b, err := json.MarshalIndent(ts, "", "  ")
	if err != nil {
		die("marshal tunnels: %v", err)
	}
	atomicWrite(tunnelsPath(), append(b, '\n'), 0600)
}

// alive reports whether pid is a running process via signal 0 (kill -0): it
// checks existence without delivering a signal.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

// pruneTunnels drops entries whose pid is gone, so the state file self-heals
// after a tunnel dies or the machine reboots (pids are not reused across boots
// fast enough to matter here, and a stale-but-reused pid at worst shows a ghost
// until its next close).
func pruneTunnels(ts []Tunnel) []Tunnel {
	live := make([]Tunnel, 0, len(ts))
	for _, t := range ts {
		if alive(t.Pid) {
			live = append(live, t)
		}
	}
	return live
}

// killGroup SIGTERMs the whole process group (negative pid) so ssh and any
// children die together; the tunnel was spawned Setpgid so its pgid == pid.
// Falls back to the bare pid if the group is already gone.
func killGroup(pid int) {
	if pid <= 0 {
		return
	}
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		syscall.Kill(pid, syscall.SIGTERM)
	}
}

// newID returns a short random hex id for a tunnel, unique enough to address one
// entry in the state file from the GUI.
func newID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		die("rand: %v", err)
	}
	return hex.EncodeToString(b[:])
}

// validHost is the conservative charset for a bind/dest host in a forward spec:
// alnum, dot, dash, underscore only. It rejects spaces, newlines, and shell
// metacharacters (;, $, backticks, …) before any spawn. IPv6 literals (colons)
// are intentionally unsupported; use a hostname or IPv4.
func validHost(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

// validPort dies unless s is an integer in 1-65535.
func validPort(s, spec string) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		die("bad spec %q: port %q must be 1-65535", spec, s)
	}
}

// checkHost dies unless s passes validHost.
func checkHost(s, spec string) {
	if !validHost(s) {
		die("bad spec %q: host %q has invalid characters", spec, s)
	}
}

// parseSpec validates a forward spec strictly and returns its kind, the ssh flag
// (-L/-R/-D), and the forward argument (the spec minus its L:/R:/D: prefix). It
// dies on anything malformed BEFORE the caller spawns ssh.
func parseSpec(spec string) (kind, flag, arg string) {
	if len(spec) < 3 || spec[1] != ':' {
		die("bad spec %q: must start with L:, R:, or D:", spec)
	}
	arg = spec[2:]
	parts := strings.Split(arg, ":")
	switch spec[0] {
	case 'L', 'R':
		var bind, port1, dhost, dport string
		switch len(parts) {
		case 3:
			port1, dhost, dport = parts[0], parts[1], parts[2]
		case 4:
			bind, port1, dhost, dport = parts[0], parts[1], parts[2], parts[3]
		default:
			die("bad spec %q: expected [bind:]port:host:port", spec)
		}
		if bind != "" {
			checkHost(bind, spec)
		}
		validPort(port1, spec)
		checkHost(dhost, spec)
		validPort(dport, spec)
		if spec[0] == 'L' {
			return "local", "-L", arg
		}
		return "remote", "-R", arg
	case 'D':
		var bind, port string
		switch len(parts) {
		case 1:
			port = parts[0]
		case 2:
			bind, port = parts[0], parts[1]
		default:
			die("bad spec %q: expected [bind:]port", spec)
		}
		if bind != "" {
			checkHost(bind, spec)
		}
		validPort(port, spec)
		return "dynamic", "-D", arg
	default:
		die("bad spec %q: must start with L:, R:, or D:", spec)
	}
	return "", "", "" // unreachable: every case above returns or dies
}

func cmdTunnel(rest []string) {
	if len(rest) < 1 {
		die("usage: ryossh tunnel <open|list|close|closeall> [args]")
	}
	sub, args := rest[0], rest[1:]
	switch sub {
	case "open":
		cmdTunnelOpen(args)
	case "list":
		cmdTunnelList(args)
	case "close":
		cmdTunnelClose(args)
	case "closeall":
		cmdTunnelCloseAll(args)
	default:
		die("unknown tunnel verb %q", sub)
	}
}

// cmdTunnelOpen validates the spec, spawns a detached `ssh -N` forward, and
// records it. It spawns WITHOUT -f: ssh stays in the foreground of its own
// session (Setsid, which also makes it its own process-group leader) so
// cmd.Process.Pid IS the tunnel pid and the group survives ryoport exiting. A
// forward that can't bind (ExitOnForwardFailure)
// or fails auth (BatchMode) exits within ~1s, so we wait ~800ms and, if it has
// already died, surface the captured stderr instead of recording a dead tunnel.
func cmdTunnelOpen(rest []string) {
	if len(rest) < 2 {
		die("usage: ryossh tunnel open <alias> <spec>")
	}
	alias, spec := rest[0], rest[1]
	if !validHost(alias) || strings.HasPrefix(alias, "-") {
		die("bad alias %q", alias)
	}
	kind, flag, arg := parseSpec(spec) // dies before we spawn on anything malformed

	// Long-lived tunnel: keepalive (not ConnectTimeout) so an idle forward
	// through NAT survives; ExitOnForwardFailure so a bind clash fails fast and
	// we detect it; BatchMode so it never blocks on a prompt.
	argv := []string{
		"-N",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		"-o", "BatchMode=yes",
		flag, arg, alias,
	}
	cmd := exec.Command("ssh", argv...)
	// Setsid detaches ssh into its own new session AND new process group (pgid ==
	// pid): it survives ryoport exiting, and close can SIGTERM the whole group by
	// -pid. (Adding Setpgid too is redundant and in fact EPERMs on Linux, since
	// setpgid on a fresh session leader is not permitted.)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		die("open %s: %v", os.DevNull, err)
	}
	defer devnull.Close()
	cmd.Stdin = devnull
	cmd.Stdout = devnull
	var errbuf bytes.Buffer // captured only to report a fast startup failure
	cmd.Stderr = &errbuf
	if err := cmd.Start(); err != nil {
		die("spawn ssh: %v", err)
	}
	pid := cmd.Process.Pid

	// Reap in the background; if it dies inside the window, Wait returning also
	// guarantees errbuf is fully copied before we read it.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case werr := <-done:
		msg := strings.TrimSpace(errbuf.String())
		if msg == "" {
			msg = fmt.Sprintf("ssh exited early: %v", werr)
		}
		die("tunnel failed: %s", msg)
	case <-time.After(800 * time.Millisecond):
		// Still alive: the reaper goroutine stays blocked in Wait for the life
		// of this short-lived CLI, then dies with us while ssh (a new session)
		// keeps running.
	}

	t := Tunnel{ID: newID(), Alias: alias, Kind: kind, Spec: spec, Pid: pid, Opened: time.Now().Unix()}
	writeTunnels(append(pruneTunnels(readTunnels()), t))
	out(map[string]any{"ok": true, "id": t.ID, "pid": t.Pid, "kind": t.Kind, "spec": t.Spec})
}

func cmdTunnelList(rest []string) {
	orig := readTunnels()
	live := pruneTunnels(orig)
	if len(live) != len(orig) {
		writeTunnels(live) // self-heal the state file
	}
	filter := ""
	if len(rest) > 0 {
		filter = rest[0]
	}
	res := make([]Tunnel, 0, len(live))
	for _, t := range live {
		if filter == "" || t.Alias == filter {
			res = append(res, t)
		}
	}
	out(res)
}

func cmdTunnelClose(rest []string) {
	if len(rest) < 1 {
		die("usage: ryossh tunnel close <id>")
	}
	id := rest[0]
	ts := readTunnels()
	idx := -1
	for i, t := range ts {
		if t.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		out(map[string]any{"ok": false, "error": "no such tunnel"})
		os.Exit(1)
	}
	killGroup(ts[idx].Pid)
	keep := make([]Tunnel, 0, len(ts)-1)
	for i, t := range ts {
		if i != idx {
			keep = append(keep, t)
		}
	}
	writeTunnels(pruneTunnels(keep))
	out(map[string]any{"ok": true})
}

func cmdTunnelCloseAll(rest []string) {
	filter := ""
	if len(rest) > 0 {
		filter = rest[0]
	}
	ts := readTunnels()
	keep := make([]Tunnel, 0, len(ts))
	closed := 0
	for _, t := range ts {
		if filter == "" || t.Alias == filter {
			killGroup(t.Pid)
			closed++
			continue
		}
		keep = append(keep, t)
	}
	writeTunnels(pruneTunnels(keep))
	out(map[string]any{"ok": true, "closed": closed})
}

// --- consoles (serial / telnet / RDP / VNC) --------------------------------
//
// Ryoport speaks SSH itself; every other protocol it delegates to a mature
// open-source client that already owns negotiation, keyboard handling, and
// rendering: picocom (GPL-2.0) for serial consoles, GNU inetutils telnet
// (GPL-3.0) for console-cable-equivalent network logins, FreeRDP (Apache-2.0)
// for RDP, and TigerVNC (GPL-2.0) for VNC. Ryoport's job is the harbour part:
// validate the profile, fetch a saved password from the keyring on demand,
// and hand the child its secret through a private stdin fd or the child's own
// environment -- never argv, never a file that outlives the launch.

// Console is one saved non-SSH berth. JSON field names are the contract the
// QML consoles page parses against. Kind selects which fields matter:
// serial uses Device/Baud; telnet/rdp/vnc use Address/Port; rdp also uses
// User/Domain; Auth is "password" when the keyring holds the secret.
type Console struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Device     string `json:"device,omitempty"`
	Baud       int    `json:"baud,omitempty"`
	Address    string `json:"address,omitempty"`
	Port       int    `json:"port,omitempty"`
	User       string `json:"user,omitempty"`
	Domain     string `json:"domain,omitempty"`
	Fullscreen bool   `json:"fullscreen,omitempty"`
	Auth       string `json:"auth,omitempty"`
}

func consolesPath() string {
	return filepath.Join(home(), ".config", "ryoku", "ryoport", "consoles.json")
}

// readConsoles loads the book; a missing or corrupt file yields an empty list
// so no verb ever crashes on it (same tolerance as readSidecar).
func readConsoles() []Console {
	data, err := os.ReadFile(consolesPath())
	if err != nil {
		return []Console{}
	}
	var cs []Console
	if err := json.Unmarshal(data, &cs); err != nil {
		return []Console{}
	}
	if cs == nil {
		cs = []Console{}
	}
	return cs
}

func writeConsoles(cs []Console) {
	b, err := json.MarshalIndent(cs, "", "  ")
	if err != nil {
		die("marshal consoles: %v", err)
	}
	atomicWrite(consolesPath(), append(b, '\n'), 0600)
}

func findConsole(id string) (Console, int) {
	cs := readConsoles()
	for i, c := range cs {
		if c.ID == id {
			return c, i
		}
	}
	return Console{}, -1
}

// upsertConsole stores one console, replacing any entry with the same id. A
// new console gets a fresh id; an edit-save keeps the stored Auth flag so the
// keyring link survives an edit.
func upsertConsole(c Console) Console {
	cs := readConsoles()
	if c.ID == "" {
		c.ID = newID()
	} else {
		for _, old := range cs {
			if old.ID == c.ID {
				c.Auth = old.Auth
				break
			}
		}
	}
	kept := make([]Console, 0, len(cs)+1)
	for _, old := range cs {
		if old.ID != c.ID {
			kept = append(kept, old)
		}
	}
	writeConsoles(append(kept, c))
	return c
}

// dropConsole removes one console and forgets its keyring password.
func dropConsole(id string) {
	cs := readConsoles()
	kept := make([]Console, 0, len(cs))
	for _, c := range cs {
		if c.ID == id {
			clearConsoleSecret(c.ID)
			continue
		}
		kept = append(kept, c)
	}
	writeConsoles(kept)
}

// consoleSecretAttr keys console passwords in the login keyring, separate from
// the ssh-host attribute so an alias and an id can never collide.
const consoleSecretAttr = "ryoport-console"

func storeConsoleSecret(c Console, pw string) error {
	cmd := exec.Command("secret-tool", "store", "--label", "ryoport console: "+c.Name, consoleSecretAttr, c.ID)
	cmd.Stdin = strings.NewReader(pw)
	return cmd.Run()
}

func clearConsoleSecret(id string) {
	exec.Command("secret-tool", "clear", consoleSecretAttr, id).Run()
}

// lookupConsoleSecret reads a console password back from the keyring. A miss
// (no entry, no daemon) is not an error: ok=false lets the caller fall back to
// the client's own interactive prompt instead of failing the launch.
func lookupConsoleSecret(id string) (string, bool) {
	b, err := exec.Command("secret-tool", "lookup", consoleSecretAttr, id).Output()
	if err != nil {
		return "", false
	}
	pw := strings.TrimRight(string(b), "\r\n")
	return pw, pw != ""
}

// findBin returns the first client binary present on PATH, or "" when none is;
// the GUI turns a miss into an install hint rather than a dead button.
func findBin(names ...string) string {
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	return ""
}

func consoleBin(kind string) (string, error) {
	switch kind {
	case "serial":
		if b := findBin("picocom"); b != "" {
			return b, nil
		}
		return "", fmt.Errorf("no serial client found: install picocom (pacman -S picocom)")
	case "telnet":
		if b := findBin("telnet"); b != "" {
			return b, nil
		}
		return "", fmt.Errorf("no telnet client found: install inetutils (pacman -S inetutils)")
	case "rdp":
		if b := findBin("xfreerdp3", "xfreerdp"); b != "" {
			return b, nil
		}
		return "", fmt.Errorf("no RDP client found: install freerdp (pacman -S freerdp)")
	case "vnc":
		if b := findBin("vncviewer"); b != "" {
			return b, nil
		}
		return "", fmt.Errorf("no VNC client found: install tigervnc (pacman -S tigervnc)")
	}
	return "", fmt.Errorf("unknown console kind %q", kind)
}

func cmdConsoleCaps() {
	out(map[string]any{
		"serial": findBin("picocom") != "",
		"telnet": findBin("telnet") != "",
		"rdp":    findBin("xfreerdp3", "xfreerdp") != "",
		"vnc":    findBin("vncviewer") != "",
	})
}

// validConsole rejects anything a client could misparse as its own flag or
// that cannot round-trip through JSON and argv (direct exec never sees a
// shell, so the risks are argument injection and garbage profiles, not shell
// quoting). Control characters and leading '-' are the whole attack surface.
func validConsole(c *Console) error {
	switch c.Kind {
	case "serial", "telnet", "rdp", "vnc":
	default:
		return fmt.Errorf("kind must be serial, telnet, rdp, or vnc")
	}
	if strings.ContainsAny(c.Name, "\r\n") || strings.ContainsRune(c.Name, 0) {
		return fmt.Errorf("name must not contain control characters")
	}
	for _, s := range []string{c.Device, c.Address, c.User, c.Domain} {
		if strings.ContainsAny(s, "\r\n") || strings.ContainsRune(s, 0) {
			return fmt.Errorf("fields must not contain control characters")
		}
		if strings.HasPrefix(s, "-") {
			return fmt.Errorf("fields must not start with '-'")
		}
	}
	switch c.Kind {
	case "serial":
		if !strings.HasPrefix(c.Device, "/dev/") || strings.ContainsAny(c.Device, " \t") {
			return fmt.Errorf("serial device must be a /dev/ path without spaces")
		}
		if c.Baud < 30 || c.Baud > 4000000 {
			return fmt.Errorf("baud must be between 30 and 4000000")
		}
	case "telnet":
		if !validHost(c.Address) {
			return fmt.Errorf("telnet address must be a bare host (letters, digits, . _ -)")
		}
		if c.Port < 1 || c.Port > 65535 {
			c.Port = 23
		}
	case "rdp":
		if !validHost(c.Address) {
			return fmt.Errorf("RDP address must be a bare host (letters, digits, . _ -)")
		}
		if c.Port < 1 || c.Port > 65535 {
			c.Port = 3389
		}
	case "vnc":
		if !validHost(c.Address) {
			return fmt.Errorf("VNC address must be a bare host (letters, digits, . _ -)")
		}
		if c.Port < 1 || c.Port > 65535 {
			c.Port = 5900
		}
	}
	return nil
}

// serialArgv is a plain picocom invocation: fixed baud, no flow control (the
// console-cable default). No credential surface by design -- the device's own
// login stays interactive in the terminal window.
func serialArgv(bin string, c Console) []string {
	return []string{bin, "--baud", strconv.Itoa(c.Baud), "--flow", "n", c.Device}
}

func telnetArgv(bin string, c Console) []string {
	return []string{bin, c.Address, strconv.Itoa(c.Port)}
}

// rdpArgv is the FreeRDP windowed launch without a saved password: the client
// shows its own credential dialog when the server demands one. /cert:tofu
// trusts a fresh server certificate once (it lands in FreeRDP's own store) so
// a home lab box with a self-signed cert does not eat a modal every connect;
// never /cert:ignore.
func rdpArgs(c Console) []string {
	// the dedicated class lets the compositor float and centre the desktop at
	// its requested size; the ssh terminal class carries a fixed 900x560.
	argv := []string{"/v:" + net.JoinHostPort(c.Address, strconv.Itoa(c.Port)), "/cert:tofu",
		"/wm-class:" + ryoportConsoleAppID, "/t:ryoport: " + c.Name}
	if c.User != "" {
		argv = append(argv, "/u:"+c.User)
	}
	if c.Domain != "" {
		argv = append(argv, "/d:"+c.Domain)
	}
	if c.Fullscreen {
		argv = append(argv, "/f")
	} else {
		argv = append(argv, "/size:1280x800")
	}
	return argv
}

func rdpArgv(bin string, c Console) []string {
	return append([]string{bin}, rdpArgs(c)...)
}

// rdpArgsFile is the saved-password form: every argument (including /p:) on
// one line of a private pipe handed to the child as fd 3 via /args-from. The
// password never reaches argv, the environment, or a file on disk -- only the
// inheriting child can read it, and only until exec.
func rdpArgsFile(c Console, pw string) string {
	args := append(rdpArgs(c), "/p:"+pw)
	return strings.Join(args, "\n") + "\n"
}

// vncTarget is TigerVNC's explicit-port endpoint form: host::5901.
func vncTarget(c Console) string {
	return c.Address + "::" + strconv.Itoa(c.Port)
}

// vncArgv connects TigerVNC to a host display; the saved password (when any)
// rides in the child's environment, the mechanism vncviewer documents.
func vncArgv(bin string, c Console) []string {
	argv := []string{bin, vncTarget(c)}
	if c.Fullscreen {
		argv = append(argv, "-FullScreen")
	}
	return argv
}

// consoleProbeOne answers "can ryoport reach this berth right now": serial
// checks the node exists and the user may read+write it (group membership is
// the usual miss), network kinds dial TCP and report latency.
func consoleProbeOne(c Console) map[string]any {
	res := map[string]any{"id": c.ID, "kind": c.Kind, "up": false, "rttMs": -1}
	switch c.Kind {
	case "serial":
		st, err := os.Stat(c.Device)
		switch {
		case err != nil:
			res["error"] = "device not present: " + c.Device
		case st.Mode()&os.ModeDevice == 0:
			res["error"] = "not a device node: " + c.Device
		case syscall.Access(c.Device, 06) != nil:
			res["error"] = "no permission on " + c.Device + " (add yourself to the operator group)"
		default:
			res["up"] = true
			res["rttMs"] = 0
		}
		return res
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(c.Address, strconv.Itoa(c.Port)), 3*time.Second)
	if err != nil {
		res["error"] = err.Error()
		return res
	}
	conn.Close()
	res["up"] = true
	res["rttMs"] = int(time.Since(start).Milliseconds())
	return res
}

func cmdConsoleProbe(rest []string) {
	if len(rest) < 1 {
		die("usage: ryossh console probe <id>")
	}
	c, idx := findConsole(rest[0])
	if idx < 0 {
		die("no such console")
	}
	res := consoleProbeOne(c)
	out(res)
	if res["up"] == false {
		os.Exit(1)
	}
}

func cmdConsoleProbeAll() {
	cs := readConsoles()
	results := make([]map[string]any, len(cs))
	sem := make(chan struct{}, poolSize)
	var wg sync.WaitGroup
	for i, c := range cs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, c Console) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = consoleProbeOne(c)
		}(i, c)
	}
	wg.Wait()
	if results == nil {
		results = []map[string]any{}
	}
	out(results)
}

// consoleTermArgv wraps a console command in a real terminal window (same
// grammar as ssh connects: float class, title, hold-on-failure).
func consoleTermArgv(c Console, prog []string) []string {
	term := os.Getenv("TERMINAL")
	if term == "" {
		term = "kitty"
	}
	return terminalWrap(term, c.Kind+": "+c.Name, prog)
}

func cmdConsoleOpen(rest []string) {
	if len(rest) < 1 {
		die("usage: ryossh console open <id>")
	}
	c, idx := findConsole(rest[0])
	if idx < 0 {
		die("no such console")
	}
	bin, err := consoleBin(c.Kind)
	if err != nil {
		die("%v", err)
	}

	var cmd *exec.Cmd
	switch c.Kind {
	case "serial":
		argv := consoleTermArgv(c, serialArgv(bin, c))
		cmd = exec.Command(argv[0], argv[1:]...)
	case "telnet":
		argv := consoleTermArgv(c, telnetArgv(bin, c))
		cmd = exec.Command(argv[0], argv[1:]...)
	case "rdp":
		pw, saved := "", false
		if c.Auth == "password" {
			pw, saved = lookupConsoleSecret(c.ID)
		}
		if saved {
			pipeR, pipeW, err := os.Pipe()
			if err != nil {
				die("pipe: %v", err)
			}
			defer pipeR.Close() // parent never reads it; only the child's fd does
			go func() {
				pipeW.Write([]byte(rdpArgsFile(c, pw)))
				// wipe the buffer's only copy by letting it go; pw stays in
				// Go's heap, acceptable for a short-lived CLI.
				pipeW.Close()
			}()
			cmd = exec.Command(bin, "/args-from:fd:3")
			cmd.ExtraFiles = []*os.File{pipeR} // fd 3 in the child
		} else {
			argv := rdpArgv(bin, c)
			cmd = exec.Command(argv[0], argv[1:]...)
		}
	case "vnc":
		argv := vncArgv(bin, c)
		cmd = exec.Command(argv[0], argv[1:]...)
		if c.Auth == "password" {
			if pw, ok := lookupConsoleSecret(c.ID); ok {
				cmd.Env = append(os.Environ(), "VNC_PASSWORD="+pw)
				if c.User != "" {
					cmd.Env = append(cmd.Env, "VNC_USERNAME="+c.User)
				}
			}
		}
	}
	// Setsid so the session window survives ryoport exiting, like ssh connects.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if cmd.Stdin == nil {
		devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			die("open %s: %v", os.DevNull, err)
		}
		defer devnull.Close()
		cmd.Stdin = devnull
	}
	if err := cmd.Start(); err != nil {
		die("launch %s: %v", bin, err)
	}
	go cmd.Wait() // reap so a fast-exiting client can't leave a zombie
	out(map[string]any{"launched": true, "id": c.ID, "kind": c.Kind})
}

func cmdConsole(rest []string) {
	if len(rest) < 1 {
		die("usage: ryossh console <list|add|remove|open|probe|probeall|caps|setpass|clearpass> [args]")
	}
	sub, args := rest[0], rest[1:]
	switch sub {
	case "list":
		out(readConsoles())
	case "caps":
		cmdConsoleCaps()
	case "add":
		if len(args) < 1 {
			die("usage: ryossh console add <json>")
		}
		var c Console
		if err := json.Unmarshal([]byte(args[0]), &c); err != nil {
			die("invalid console JSON: %v", err)
		}
		if err := validConsole(&c); err != nil {
			die("%v", err)
		}
		c = upsertConsole(c)
		out(map[string]any{"ok": true, "id": c.ID})
	case "remove":
		if len(args) < 1 {
			die("usage: ryossh console remove <id>")
		}
		dropConsole(args[0])
		out(map[string]any{"ok": true})
	case "open":
		cmdConsoleOpen(args)
	case "probe":
		cmdConsoleProbe(args)
	case "probeall":
		cmdConsoleProbeAll()
	case "setpass":
		if len(args) < 1 {
			die("usage: ryossh console setpass <id>")
		}
		c, idx := findConsole(args[0])
		if idx < 0 {
			die("no such console")
		}
		if _, err := exec.LookPath("secret-tool"); err != nil {
			die("secret-tool not found: install libsecret to save passwords")
		}
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		pw := strings.TrimRight(line, "\r\n")
		if pw == "" {
			die("no password on stdin")
		}
		if err := storeConsoleSecret(c, pw); err != nil {
			die("keyring store failed: %v", err)
		}
		cs := readConsoles()
		cs[idx].Auth = "password"
		writeConsoles(cs)
		out(map[string]any{"ok": true, "id": c.ID})
	case "clearpass":
		if len(args) < 1 {
			die("usage: ryossh console clearpass <id>")
		}
		clearConsoleSecret(args[0])
		cs := readConsoles()
		if _, idx := findConsole(args[0]); idx >= 0 {
			cs[idx].Auth = ""
			writeConsoles(cs)
		}
		out(map[string]any{"ok": true})
	default:
		die("unknown console verb %q", sub)
	}
}
