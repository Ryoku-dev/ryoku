// Package paths names every file and directory Rashin reads or writes. Nothing
// else in Rashin spells a path; tests and sandboxes move all of them with the
// XDG variables.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Home is the user's home directory.
func Home() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

// Short writes a path under the home directory with a leading tilde.
func Short(path string) string {
	if home := Home(); home != "" && strings.HasPrefix(path, home+"/") {
		return "~" + path[len(home):]
	}
	return path
}

func xdg(env, fallback string) string {
	if v := os.Getenv(env); v != "" && filepath.IsAbs(v) {
		return v
	}
	return filepath.Join(Home(), fallback)
}

// ConfigHome is $XDG_CONFIG_HOME.
func ConfigHome() string { return xdg("XDG_CONFIG_HOME", ".config") }

// StateHome is $XDG_STATE_HOME.
func StateHome() string { return xdg("XDG_STATE_HOME", ".local/state") }

// DataHome is $XDG_DATA_HOME.
func DataHome() string { return xdg("XDG_DATA_HOME", ".local/share") }

// CacheHome is $XDG_CACHE_HOME.
func CacheHome() string { return xdg("XDG_CACHE_HOME", ".cache") }

// ConfigFile is rashin.json.
func ConfigFile() string { return filepath.Join(ConfigHome(), "ryoku", "rashin.json") }

// StateDir holds sessions, the key store and the index registry.
func StateDir() string { return filepath.Join(StateHome(), "ryoku", "rashin") }

// SessionsDir holds one directory per session.
func SessionsDir() string { return filepath.Join(StateDir(), "sessions") }

// SessionDir is one session's directory.
func SessionDir(id string) string { return filepath.Join(SessionsDir(), id) }

// GatewayDB is the key, usage and cooldown store.
func GatewayDB() string { return filepath.Join(StateDir(), "gateway.db") }

// MasterKey seals provider keys at rest.
func MasterKey() string { return filepath.Join(StateDir(), "master.key") }

// IndexRegistry lists the registered indexes.
func IndexRegistry() string { return filepath.Join(StateDir(), "indexes.json") }

// HarnessCapabilities caches what each harness binary answered at initialize,
// keyed by the binary's path and mtime.
func HarnessCapabilities() string { return filepath.Join(StateDir(), "harness-capabilities.json") }

// DaemonLog keeps the daemon's warnings and errors.
func DaemonLog() string { return filepath.Join(StateDir(), "rashind.log") }

// MigrationMarker records that migration step name ran.
func MigrationMarker(name string) string {
	return filepath.Join(StateDir(), "migrated", name)
}

// CacheDir holds derived data that can be rebuilt.
func CacheDir() string { return filepath.Join(CacheHome(), "ryoku", "rashin") }

// IndexDir is one index's directory.
func IndexDir(id string) string { return filepath.Join(CacheDir(), "index", id) }

// RuntimeDir holds the socket and locks: $XDG_RUNTIME_DIR/rashin.
func RuntimeDir() string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" || !filepath.IsAbs(base) {
		base = filepath.Join(os.TempDir(), fmt.Sprintf("rashin-%d", os.Getuid()))
		return base
	}
	return filepath.Join(base, "rashin")
}

// socketName is the daemon socket's file name in its dir.
const socketName = "rashin.sock"

// Socket is the daemon socket; RASHIN_SOCKET overrides it for a client.
func Socket() string {
	if v := os.Getenv("RASHIN_SOCKET"); v != "" {
		return v
	}
	return filepath.Join(RuntimeDir(), socketName)
}

// AgentSocketsDir holds a dir for each running team session with the socket
// its agent reaches the daemon through.
func AgentSocketsDir() string { return filepath.Join(RuntimeDir(), "agents") }

// AgentSocket is a team session's own socket. Its dir takes RuntimeDir's
// place inside the session's sandbox, so there it sits where Socket is.
func AgentSocket(id string) string { return filepath.Join(AgentSocketsDir(), id, socketName) }

// DaemonLock keeps a second daemon from starting.
func DaemonLock() string { return filepath.Join(RuntimeDir(), "rashind.lock") }

// AppLock keeps a second app from starting.
func AppLock() string { return filepath.Join(RuntimeDir(), "app.lock") }

// ConfigLock serialises writes of rashin.json between the daemon and the CLI
// (settings.set against rashin on and off).
func ConfigLock() string { return filepath.Join(RuntimeDir(), "config.lock") }

// AppState holds the app's window size and view state.
func AppState() string { return filepath.Join(StateHome(), "ryoku", "rashin-app.json") }

// PastedDir holds images pasted into a composer until the session ends.
func PastedDir() string { return filepath.Join(RuntimeDir(), "pasted") }

// ScreensDir holds the agent screens: each one's nested session and its
// latest frame.
func ScreensDir() string { return filepath.Join(RuntimeDir(), "screens") }

// ScreenHome is the HOME of the apps agents open on their screens, kept apart
// from the user's and kept between screens so a login made there lasts.
func ScreenHome() string { return filepath.Join(DataHome(), "ryoku", "rashin", "screen-home") }

// Keyrings holds the login keyrings, which keep the credential vault's key.
func Keyrings() string { return filepath.Join(DataHome(), "keyrings") }

// SystemConfig is Ryoku's system-wide configuration directory.
func SystemConfig() string { return "/etc/ryoku" }

// WorktreesDir is the default home of worktrees Rashin creates.
func WorktreesDir() string { return filepath.Join(DataHome(), "ryoku", "worktrees") }

// Projects holds projects, their settings, Rashin-made worktrees, tasks and
// review comments.
func Projects() string { return filepath.Join(StateDir(), "projects.json") }

// ActivityLog is one project's action ledger.
func ActivityLog(project string) string {
	return filepath.Join(StateDir(), "activity", project+".jsonl")
}

// TeamsDir holds the teams, their tasks, routines, approvals, ledger and
// spend.
func TeamsDir() string { return filepath.Join(StateDir(), "teams") }

// TeamsHome is where a new team's folder goes unless the user picks another.
func TeamsHome() string { return filepath.Join(Home(), "Teams") }

// CredentialVault holds the sealed credentials teams use and their record of
// use. Its key lives in the login keyring, never beside it.
func CredentialVault() string { return filepath.Join(StateDir(), "vault.db") }

// VaultDir is the Ryoku vault, owned by Ryoku core.
func VaultDir() string { return filepath.Join(DataHome(), "ryoku", "vault") }

// SystemShare is the shipped Ryoku data root.
func SystemShare() string { return "/usr/share/ryoku" }

// LegacyVaultDir is where Rashin 1 kept the vault.
func LegacyVaultDir() string { return filepath.Join(DataHome(), "ryoku", "rashin") }

// LegacyAskHistory is Rashin 1's ask log.
func LegacyAskHistory() string { return filepath.Join(StateHome(), "ryoku", "rashin-asks.jsonl") }

// LegacyStateDirs are Rashin 1's lane directories, removed by migration.
func LegacyStateDirs() []string {
	return []string{
		filepath.Join(StateHome(), "ryoku", "rashin-chat"),
		filepath.Join(StateHome(), "ryoku", "rashin-ask"),
	}
}

// ProwlGatewayDir is Prowl's gateway state, read once by migration.
func ProwlGatewayDir() string { return filepath.Join(DataHome(), "prowl-agent", "gateway") }

// EnsureDir creates dir and its parents with mode 0700.
func EnsureDir(dir string) error { return os.MkdirAll(dir, 0o700) }

// Within reports whether path is root or below it.
func Within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// Resolved is path with the symlinks of its longest existing part resolved
// and the rest kept as written, so two paths compare by where they lead.
func Resolved(path string) string {
	path = filepath.Clean(path)
	rest := ""
	for {
		if real, err := filepath.EvalSymlinks(path); err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return filepath.Join(path, rest)
		}
		rest = filepath.Join(filepath.Base(path), rest)
		path = parent
	}
}
