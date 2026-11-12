package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/harness"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// agentUnset are the ways out of a sandbox an agent would otherwise inherit:
// the user's session bus and the system bus, display, SSH agent and keyring,
// and a daemon to start.
var agentUnset = []string{"DBUS_SESSION_BUS_ADDRESS", "DBUS_SYSTEM_BUS_ADDRESS", "WAYLAND_DISPLAY", "DISPLAY", "SSH_AUTH_SOCK", "GNOME_KEYRING_CONTROL", "RASHIN_DAEMON"}

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

// scopeKnown remembers, per rashin command and its modification time,
// whether it knows `rashin scope`.
var scopeKnown sync.Map

// scopeCommand is the rashin command sandboxes run their program through, ""
// when it is older than `rashin scope`: an older one would take "scope" for
// a question to Ask, and the program would never start. It says so in the
// log once per command.
func (d *Daemon) scopeCommand() string {
	rashin := d.commandPath()
	info, err := os.Stat(rashin)
	if rashin == "" || err != nil {
		return ""
	}
	key := rashin + "\x00" + info.ModTime().String()
	if known, ok := scopeKnown.Load(key); ok {
		if known.(bool) {
			return rashin
		}
		return ""
	}
	check := exec.Command(rashin, "__is-command", "scope")
	check.WaitDelay = time.Second
	known := check.Run() == nil
	scopeKnown.Store(key, known)
	if !known {
		d.warn("sandbox: %s has no scope command; sandboxed programs run unscoped until it is updated", rashin)
		return ""
	}
	return rashin
}

// privateDirs are Rashin's own: its state and cache, the teams' and dreams'
// folders, the login keyrings and the vaults, resolved.
func privateDirs() []string {
	dirs := []string{paths.StateDir(), paths.CacheDir(), paths.TeamsHome(), paths.DreamsHome(), paths.Keyrings(), paths.VaultDir(), paths.LegacyVaultDir()}
	for index, dir := range dirs {
		dirs[index] = paths.Resolved(dir)
	}
	return dirs
}

// alwaysHidden are hidden from every team agent and every harness Check
// whatever the settings say: Rashin's private dirs, the places credentials
// live (protocol.TeamsAlwaysHidden) and every harness's own state under the
// user's home, resolved. A team's harness runs in a team home of its own.
func alwaysHidden() []string {
	dirs := privateDirs()
	for _, path := range protocol.TeamsAlwaysHiddenPaths() {
		dirs = append(dirs, paths.Resolved(paths.Expand(path)))
	}
	for _, path := range harness.StatePaths() {
		dirs = append(dirs, paths.Resolved(path))
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

// programDirs are the dirs of the running daemon and of the rashin command
// agents are handed, so a build run from /tmp or /dev/shm still starts its
// MCP server inside a sandbox.
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

// agentSandbox is the sandbox a team session's agent runs in: its team's
// folder, created when missing, and its own socket where the daemon's would
// be, so the tools it is handed reach the daemon only as that session. Its
// team's built-in skills show read-only where they lie, inside the data dir
// it otherwise sees empty, so the paths its brief names work. A folder
// inside what Rashin always hides is refused.
func (d *Daemon) agentSandbox(info protocol.SessionInfo) (*sandbox.Spec, error) {
	if hiddenFolder(paths.Resolved(info.Cwd)) {
		return nil, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.Tf("The team's folder %s is inside a place Rashin hides from every team. Choose another folder.", paths.Short(info.Cwd)))
	}
	if err := os.MkdirAll(info.Cwd, 0o700); err != nil {
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
	spec.Env = map[string]string{"RASHIN_SOCKET": filepath.Join(runtime, filepath.Base(paths.AgentSocket(info.ID)))}
	spec.Unset = agentUnset
	if d.deps.Teams != nil {
		spec = withAccess(spec, d.deps.Teams.Access(info))
	}
	for _, dir := range d.skillDirs(info) {
		spec.ReadOnly = append(spec.ReadOnly, paths.Resolved(dir))
	}
	return &spec, nil
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

// probeSandbox is the sandbox harnesses.probe starts a harness in: a team
// agent's, without a team's own access, socket or tools.
func (d *Daemon) probeSandbox() sandbox.Spec {
	spec := d.sandboxBase()
	spec.Unset = agentUnset
	return spec
}

// askSandbox is the sandbox a harness that answers an Ask runs in while the
// Ask is read-only. It hides nothing, as it is the user's own harness
// answering the user, but the whole host is read-only, its folder and
// Rashin's own programs show again where the private /tmp or /dev/shm would
// cover them, and its runtime dir is an empty one of its own: no session bus
// to start something outside it, no display, and the Ask's own socket where
// the daemon's would be, so its tools reach the daemon only as that Ask.
func (d *Daemon) askSandbox(info protocol.SessionInfo) (*sandbox.Spec, error) {
	dir, err := d.openAgentSocket(info.ID)
	if err != nil {
		return nil, err
	}
	folder := info.Cwd
	if folder == "" {
		folder = paths.Home()
	}
	folder = paths.Resolved(folder)
	runtime := paths.Resolved(paths.RuntimeDir())
	return &sandbox.Spec{
		Dir:        folder,
		ReadOnly:   append(d.programDirs(), folder),
		RuntimeDir: filepath.Dir(runtime),
		Binds:      []sandbox.Bind{{Source: paths.Resolved(dir), Target: runtime}},
		Env:        map[string]string{"RASHIN_SOCKET": filepath.Join(runtime, filepath.Base(paths.AgentSocket(info.ID)))},
		Unset:      agentUnset,
		Scope:      d.scopeCommand(),
	}, nil
}
