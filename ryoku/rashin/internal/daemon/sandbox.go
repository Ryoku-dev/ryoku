package daemon

import (
	"os"
	"path/filepath"
	"slices"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// agentUnset are the ways out of a sandbox an agent would otherwise inherit:
// the user's session bus, display, SSH agent and keyring, and a daemon to
// start.
var agentUnset = []string{"DBUS_SESSION_BUS_ADDRESS", "WAYLAND_DISPLAY", "DISPLAY", "SSH_AUTH_SOCK", "GNOME_KEYRING_CONTROL", "RASHIN_DAEMON"}

// sandboxBase is what every team sandbox starts from. It never sees Rashin's
// state and cache, the teams' folders, the login keyrings, the vaults or the
// paths Settings > Teams hides, has an empty runtime dir of its own, and
// still sees Rashin's own programs wherever they run from. Every path is
// resolved, as the sandbox compares them as written.
func (d *Daemon) sandboxBase() sandbox.Spec {
	spec := sandbox.Spec{RuntimeDir: paths.Resolved(filepath.Dir(paths.RuntimeDir())), ReadOnly: d.programDirs(), Hidden: privateDirs()}
	hidden := d.Settings().Teams.Hidden
	if hidden == nil {
		hidden = protocol.DefaultTeamsHidden()
	}
	for _, path := range hidden {
		spec.Hidden = append(spec.Hidden, paths.Resolved(paths.Expand(path)))
	}
	return spec
}

// privateDirs are hidden from every team agent whatever the settings say,
// resolved.
func privateDirs() []string {
	dirs := []string{paths.StateDir(), paths.CacheDir(), paths.TeamsHome(), paths.Keyrings(), paths.VaultDir(), paths.LegacyVaultDir()}
	for index, dir := range dirs {
		dirs[index] = paths.Resolved(dir)
	}
	return dirs
}

// withAccess adds what a team's own settings let its agents reach. A folder
// it may change that no longer exists is left out, as the sandbox binds only
// what exists, and so is one inside Rashin's private dirs other than the
// team's own folder, which a deeper writable path would otherwise uncover.
func withAccess(spec sandbox.Spec, access protocol.TeamAccess) sandbox.Spec {
	for _, path := range access.Hidden {
		spec.Hidden = append(spec.Hidden, paths.Resolved(path))
	}
	private := privateDirs()
	for _, path := range access.Writable {
		path = paths.Resolved(path)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		inside := slices.ContainsFunc(private, func(dir string) bool { return paths.Within(dir, path) })
		if inside && (spec.Folder == "" || !paths.Within(spec.Folder, path)) {
			continue
		}
		spec.Writable = append(spec.Writable, path)
	}
	return spec
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
// be, so the tools it is handed reach the daemon only as that session.
func (d *Daemon) agentSandbox(info protocol.SessionInfo) (*sandbox.Spec, error) {
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
