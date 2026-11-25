package daemon

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// agentUnset are the ways out of a sandbox an agent would otherwise inherit:
// the user's session bus and the system bus, display, SSH agent and keyring,
// and a daemon to start.
var agentUnset = []string{"DBUS_SESSION_BUS_ADDRESS", "DBUS_SYSTEM_BUS_ADDRESS", "WAYLAND_DISPLAY", "DISPLAY", "SSH_AUTH_SOCK", "GNOME_KEYRING_CONTROL", "HYAKKI_DAEMON"}

// keptEnv is what a team task's tools keep of the daemon's environment:
// where programs are, language and terminal, who they run as, and how the
// network and its certificates are reached. Everything else, API keys and
// tokens a login session exported included, stays outside the sandbox.
var keptEnv = []string{
	"PATH", "LANG", "LANGUAGE", "TERM", "COLORTERM", "TZ", "USER", "LOGNAME", "SHELL", "XDG_RUNTIME_DIR",
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "no_proxy", "all_proxy",
	"SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS", "REQUESTS_CA_BUNDLE",
}

// sandboxBase is what every team sandbox starts from. It never sees what
// Rashin always hides (alwaysHidden) or the paths Settings > Teams adds, has
// an empty runtime dir of its own, still sees Rashin's own programs wherever
// they run from, and runs its program through the rashin command's scope.
// Every path is resolved, as the sandbox compares them as written.
func (d *Daemon) sandboxBase() sandbox.Spec {
	spec := sandbox.Spec{RuntimeDir: paths.Resolved(filepath.Dir(paths.RuntimeDir())), ReadOnly: d.programDirs(), Hidden: alwaysHidden(), Scope: d.scopeCommand()}
	for _, path := range d.Settings().Teams.Hidden {
		spec.Hidden = append(spec.Hidden, paths.Resolved(paths.Expand(path)))
	}
	return spec
}

// never drop the Landlock scope when the sibling CLI is missing
func (d *Daemon) scopeCommand() string {
	if program := d.commandPath(); program != "" {
		return program
	}
	if d.deps.Program != "" {
		return filepath.Join(filepath.Dir(d.deps.Program), "hyakki")
	}
	return "/usr/bin/hyakki"
}

// privateDirs are Rashin's own: its state and cache, the teams' and dreams'
// folders, the login keyrings and the vaults, resolved. The tool homes lie
// in the state dir, so every team's stays hidden from every other.
func privateDirs() []string {
	dirs := []string{paths.StateDir(), paths.CacheDir(), paths.DataDir(), filepath.Dir(paths.ConfigFile()), paths.TeamsHome(), paths.DreamsHome(), paths.Keyrings(), paths.VaultDir(), paths.LegacyVaultDir()}
	for index, dir := range dirs {
		dirs[index] = paths.Resolved(dir)
	}
	return dirs
}

// alwaysHidden are hidden from every team task whatever the settings say:
// Rashin's private dirs and the places credentials live
// (protocol.TeamsAlwaysHidden), other coding agents' logins included,
// resolved.
func alwaysHidden() []string {
	dirs := privateDirs()
	for _, path := range protocol.TeamsAlwaysHiddenPaths() {
		dirs = append(dirs, paths.Resolved(paths.Expand(path)))
	}
	slices.Sort(dirs)
	return slices.Compact(dirs)
}

// withAccess adds what a team's own settings let its agents reach. A folder
// it may change that no longer exists is left out, as the sandbox binds only
// what exists, and so is one inside what Rashin always hides other than the
// team's own folder, which a deeper writable path would otherwise uncover.
func withAccess(spec sandbox.Spec, access protocol.TeamAccess) sandbox.Spec {
	for _, path := range access.Hidden {
		spec.Hidden = append(spec.Hidden, paths.Resolved(path))
	}
	hidden := alwaysHidden()
	for _, path := range access.Writable {
		path = paths.Resolved(path)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		inside := slices.ContainsFunc(hidden, func(dir string) bool { return paths.Within(dir, path) })
		if inside && (spec.Folder == "" || !paths.Within(spec.Folder, path)) {
			continue
		}
		spec.Writable = append(spec.Writable, path)
	}
	return spec
}

// hiddenFolder reports a team folder inside what Rashin always hides, other
// than the teams' and dreams' own homes: binding it would uncover what it
// holds.
func hiddenFolder(folder string) bool {
	teams, dreams := paths.Resolved(paths.TeamsHome()), paths.Resolved(paths.DreamsHome())
	return slices.ContainsFunc(alwaysHidden(), func(dir string) bool {
		return dir != teams && dir != dreams && paths.Within(dir, folder)
	})
}

// programDirs are the dirs of the running daemon and of the rashin command,
// so a build run from /tmp or /dev/shm still starts its tool host inside a
// sandbox.
func (d *Daemon) programDirs() []string {
	var dirs []string
	running, _ := os.Executable()
	for _, program := range []string{running, d.commandPath()} {
		if program == "" {
			continue
		}
		dir := filepath.Dir(paths.Resolved(program))
		if !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// agentSandbox is the sandbox a team session's tool host runs its file and
// shell tools in: its team's folder, created when missing, and its own
// socket where the daemon's would be, so the rashin command typed into its
// shell reaches the daemon only as that session. It starts from an empty
// environment that keeps only keptEnv and the locale, so no key a login
// exported enters, and HOME and the XDG dirs point at the task's scratch
// home, with the XDG cache at its team's cache, kept across tasks. Its
// team's skills show read-only where they lie, inside the data dir it
// otherwise sees empty, so the paths its brief names work. A folder inside
// what Rashin always hides is refused.
func (d *Daemon) agentSandbox(info protocol.SessionInfo) (*sandbox.Spec, error) {
	if hiddenFolder(paths.Resolved(info.Cwd)) {
		return nil, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.Tf("The team's folder %s is inside a place Hyakki hides from every team. Choose another folder.", paths.Short(info.Cwd)))
	}
	if err := os.MkdirAll(info.Cwd, 0o700); err != nil {
		return nil, err
	}
	task := info.TeamTask
	if task == "" {
		task = info.ID
	}
	home, cache, err := prepareToolHome(info.Team, task)
	if err != nil {
		return nil, err
	}
	dir, err := d.openAgentSocket(info.ID)
	if err != nil {
		return nil, err
	}
	runtime := paths.Resolved(paths.RuntimeDir())
	spec := d.sandboxBase()
	spec.Folder = paths.Resolved(info.Cwd)
	spec.Binds = []sandbox.Bind{{Source: paths.Resolved(dir), Target: runtime}}
	spec.Writable = append(spec.Writable, home, cache)
	spec.ClearEnv = true
	spec.Env = toolEnv(home, cache)
	spec.Env["HYAKKI_SOCKET"] = filepath.Join(runtime, filepath.Base(paths.AgentSocket(info.ID)))
	spec.Unset = agentUnset
	if d.deps.Teams != nil {
		spec = withAccess(spec, d.deps.Teams.Access(info))
	}
	for _, dir := range d.skillDirs(info) {
		spec.ReadOnly = append(spec.ReadOnly, paths.Resolved(dir))
	}
	return &spec, nil
}

// toolEnv is the whole environment of a team task's tools: keptEnv and the
// locale's LC_* of the daemon's, then HOME and the XDG base dirs at home,
// the cache at cache.
func toolEnv(home, cache string) map[string]string {
	env := make(map[string]string)
	for _, name := range keptEnv {
		if value, ok := os.LookupEnv(name); ok {
			env[name] = value
		}
	}
	for _, entry := range os.Environ() {
		if name, value, ok := strings.Cut(entry, "="); ok && strings.HasPrefix(name, "LC_") {
			env[name] = value
		}
	}
	env["HOME"] = home
	env["XDG_CONFIG_HOME"] = filepath.Join(home, ".config")
	env["XDG_DATA_HOME"] = filepath.Join(home, ".local", "share")
	env["XDG_STATE_HOME"] = filepath.Join(home, ".local", "state")
	env["XDG_CACHE_HOME"] = cache
	return env
}

// toolHomeDirs are made in every scratch home, so programs that expect the
// XDG dirs find them.
var toolHomeDirs = []string{".config", filepath.Join(".local", "share"), filepath.Join(".local", "state")}

// prepareToolHome creates a team task's scratch home and its team's cache,
// both 0700 under paths.ToolHomes(), and gives the home a git config that
// includes the user's, so commits made there carry the user's name; git
// skips an include that is missing. It returns both resolved.
func prepareToolHome(team, task string) (home, cache string, err error) {
	if !toolHomeName(team) || !toolHomeName(task) {
		return "", "", errors.New(i18n.T("A team task's tools need a team and a task."))
	}
	home, cache = paths.ToolHome(team, task), paths.ToolCache(team)
	if err := paths.EnsureDir(paths.StateDir()); err != nil {
		return "", "", err
	}
	for _, dir := range []string{paths.ToolHomes(), filepath.Dir(home), home, cache} {
		if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return "", "", err
		}
		info, err := os.Lstat(dir)
		if err != nil {
			return "", "", err
		}
		if !info.IsDir() {
			return "", "", fmt.Errorf("tool home %q is not a directory", dir)
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return "", "", err
		}
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		return "", "", err
	}
	defer root.Close()
	for _, dir := range toolHomeDirs {
		if err := root.MkdirAll(dir, 0o700); err != nil {
			return "", "", err
		}
	}
	if info, err := root.Lstat(".gitconfig"); errors.Is(err, fs.ErrNotExist) {
		include := fmt.Sprintf("# Hyakki: your git identity, read from your own config.\n[include]\n\tpath = %s\n\tpath = %s\n",
			filepath.Join(paths.Home(), ".gitconfig"), filepath.Join(paths.ConfigHome(), "git", "config"))
		if err := root.WriteFile(".gitconfig", []byte(include), 0o600); err != nil {
			return "", "", err
		}
	} else if err != nil {
		return "", "", err
	} else if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("tool home %q has an unsafe git config", home)
	}
	return paths.Resolved(home), paths.Resolved(cache), nil
}

// toolHomeName reports a team or task id that names one folder: not empty,
// no separator, not "." or "..", and not the per-team "cache".
func toolHomeName(name string) bool {
	return name != "" && name != "." && name != ".." && name != "cache" && !strings.ContainsRune(name, filepath.Separator)
}

// screenSandbox is the sandbox the apps on a team session's screen run in;
// nil for any other session's screen, which runs its apps as the user. A
// session the daemon cannot find gets a sandbox all the same. The apps do
// not see what the team hides, and change nothing the team may change.
func (d *Daemon) screenSandbox(id string) *sandbox.Spec {
	info, ok := d.manager.Info(id)
	if ok && info.Team == "" {
		return nil
	}
	spec := d.sandboxBase()
	if ok && d.deps.Teams != nil {
		spec = withAccess(spec, protocol.TeamAccess{Hidden: d.deps.Teams.Access(info).Hidden})
	}
	return &spec
}
