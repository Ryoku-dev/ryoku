package bt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anacrolix/torrent"
)

// TestMain keeps the suite off the machine: private XDG dirs, no public
// tracker list, and clients that only listen on loopback with no DHT,
// trackers, UPnP or webtorrent.
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "ryoget-bt-test-")
	if err != nil {
		panic(err)
	}
	for _, kv := range [][2]string{
		{"HOME", "home"},
		{"XDG_CONFIG_HOME", "config"},
		{"XDG_CACHE_HOME", "cache"},
		{"XDG_DATA_HOME", "share"},
		{"XDG_STATE_HOME", "state"},
		{"XDG_RUNTIME_DIR", "runtime"},
	} {
		dir := filepath.Join(root, kv[1])
		if err := os.MkdirAll(dir, 0o700); err != nil {
			panic(err)
		}
		os.Setenv(kv[0], dir)
	}
	publicTrackersURL = ""
	configure = func(cc *torrent.ClientConfig) {
		cc.ListenHost = func(string) string { return "127.0.0.1" }
		cc.DisableIPv6 = true
		cc.NoDefaultPortForwarding = true
		cc.DisableTrackers = true
		cc.DisableWebtorrent = true
		cc.DisableWebseeds = true
		cc.NoDHT = true
	}
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}
