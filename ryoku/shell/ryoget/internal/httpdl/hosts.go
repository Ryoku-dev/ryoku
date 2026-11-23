package httpdl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ryoget/internal/paths"
)

const hostLimitTTL = 24 * time.Hour

// the daemon exits when idle, so what a server tolerates lives on disk
type hostLimit struct {
	Conns int       `json:"connections"`
	At    time.Time `json:"at"`
}

var hostsMu sync.Mutex

func (d *Downloader) hostsFile() string {
	if d.HostsFile != "" {
		return d.HostsFile
	}
	return filepath.Join(paths.StateDir(), "hosts.json")
}

func readHosts(path string, now time.Time) map[string]hostLimit {
	m := map[string]hostLimit{}
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &m) != nil {
		return map[string]hostLimit{}
	}
	for h, l := range m {
		if l.Conns < 1 || now.Sub(l.At) >= hostLimitTTL {
			delete(m, h)
		}
	}
	return m
}

// ported from prism-download-manager crates/prism-engine/src/hostlimits.rs (MIT)
func learnedLimit(path, host string) int {
	hostsMu.Lock()
	defer hostsMu.Unlock()
	return readHosts(path, time.Now())[strings.ToLower(host)].Conns
}

func recordLimit(path, host string, conns int) {
	host = strings.ToLower(host)
	hostsMu.Lock()
	defer hostsMu.Unlock()
	now := time.Now()
	m := readHosts(path, now)
	if m[host].Conns == conns {
		return
	}
	m[host] = hostLimit{Conns: max(1, conns), At: now}
	b, err := json.Marshal(m)
	if err != nil || os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}
