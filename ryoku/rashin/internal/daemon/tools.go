package daemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/memory"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/screen"
)

// agentTools hands an agent session Rashin's own MCP server, which serves the
// code search over its folder, the session's goal, Rashin's teams and their
// templates with propose_team, where the desktop can host one its screen,
// the memory tools when the session uses memory and, for a team task, the
// team's tools. Without the rashin command beside the daemon or on PATH the
// session gets none.
func (d *Daemon) agentTools(info protocol.SessionInfo) []core.MCPServer {
	command := d.commandPath()
	if command == "" {
		return nil
	}
	env := map[string]string{"RASHIN_SESSION": info.ID, "RASHIN_CWD": info.Cwd}
	if d.screenFor(info) {
		env["RASHIN_SCREEN"] = "1"
	}
	// a team task's memory is its team's, which Teams says
	if settings := d.Settings(); d.deps.Memory != nil && info.Team == "" && settings.Memory.Agents && memory.Uses(settings, info) {
		env["RASHIN_MEMORY"] = "1"
		if settings.Memory.Sessions {
			env["RASHIN_MEMORY_SESSIONS"] = "1"
		}
	}
	if d.deps.Teams != nil {
		for key, value := range d.deps.Teams.ToolEnv(info) {
			env[key] = value
		}
	}
	return []core.MCPServer{{Name: "rashin", Command: command, Args: []string{"mcp"}, Env: env}}
}

// screenFor says a session may be offered a screen of its own: the desktop
// can host one and, for a team task, its team was given one.
func (d *Daemon) screenFor(info protocol.SessionInfo) bool {
	if info.Team != "" && (d.deps.Teams == nil || !d.deps.Teams.ScreenGranted(info)) {
		return false
	}
	return screenable()
}

// screenable says this desktop can host a screen for an agent
func screenable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return screen.Available(ctx)
}

// commandPath is the rashin command installed beside this daemon, or the one
// on PATH.
func (d *Daemon) commandPath() string {
	if d.deps.Program != "" {
		candidate := filepath.Join(filepath.Dir(d.deps.Program), "rashin")
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate
		}
	}
	path, _ := exec.LookPath("rashin")
	return path
}
