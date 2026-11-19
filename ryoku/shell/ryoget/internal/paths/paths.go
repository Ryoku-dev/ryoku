// Package paths is the only place ryoget and ryoseek spell where they keep
// things; tests and sandboxes move all of them with the XDG variables.
package paths

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func Home() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return "/"
}

// xdg ignores relative values, as the XDG spec requires.
func xdg(env, fallback string) string {
	if v := os.Getenv(env); filepath.IsAbs(v) {
		return v
	}
	return filepath.Join(Home(), fallback)
}

func StateDir() string {
	return filepath.Join(xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "ryoget")
}

func CacheDir() string { return filepath.Join(xdg("XDG_CACHE_HOME", ".cache"), "ryoku", "ryoget") }

func ConfigFile() string {
	return filepath.Join(xdg("XDG_CONFIG_HOME", ".config"), "ryoku", "downloads.json")
}

func TasksFile() string { return filepath.Join(StateDir(), "tasks.json") }

func Log() string { return filepath.Join(StateDir(), "ryoget.log") }

func Socket() string { return runtimeSocket("ryoget") }

func Lock() string { return strings.TrimSuffix(Socket(), ".sock") + ".lock" }

func runtimeSocket(name string) string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); filepath.IsAbs(d) {
		return filepath.Join(d, name+".sock")
	}
	return filepath.Join(os.TempDir(), name+"-"+strconv.Itoa(os.Getuid())+".sock")
}

// The search daemon keeps its own files so it never shares a lock or a log
// with the download engine.
func SeekStateDir() string {
	return filepath.Join(xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "ryoseek")
}

func SeekHealthFile() string { return filepath.Join(SeekStateDir(), "health.json") }

func SeekLog() string { return filepath.Join(SeekStateDir(), "ryoseek.log") }

func SeekCacheDir() string {
	return filepath.Join(xdg("XDG_CACHE_HOME", ".cache"), "ryoku", "ryoseek")
}

func SeekSocket() string { return runtimeSocket("ryoseek") }

func SeekLock() string { return strings.TrimSuffix(SeekSocket(), ".sock") + ".lock" }

// DownloadDir follows the session's XDG_DOWNLOAD_DIR, which already carries the
// localized folder name.
func DownloadDir() string {
	if d := os.Getenv("XDG_DOWNLOAD_DIR"); filepath.IsAbs(d) {
		return d
	}
	return filepath.Join(Home(), "Downloads")
}

// Expand turns a leading ~ into the home directory.
func Expand(p string) string {
	if p == "~" {
		return Home()
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(Home(), rest)
	}
	return p
}

func EnsureDir(dir string) error { return os.MkdirAll(dir, 0o700) }
