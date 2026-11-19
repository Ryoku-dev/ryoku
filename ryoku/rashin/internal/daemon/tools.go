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

// agentTools is which of Rashin's in-process tools an agent session has: a
// screen when it may use one, memory when the settings let agents use it,
// and for a team task its run as its team tools see it, whose memory is the
// team's. The user's MCP servers are not named here: the engine's MCP
// provider reads them while the feature is on.
func (d *Daemon) agentTools(info protocol.SessionInfo) core.ToolContext {
	tools := core.ToolContext{Session: info.ID, Cwd: info.Cwd, Screen: d.screenFor(info)}
	if info.Team != "" {
		if d.deps.Teams != nil {
			tools.Team = d.deps.Teams.ToolContext(info)
		}
		if tools.Team != nil {
			tools.Memory = d.deps.Memory != nil && tools.Team.Memory
		}
		return tools
	}
	if settings := d.Settings(); d.deps.Memory != nil && settings.Memory.Agents && memory.Uses(settings, info) {
		tools.Memory = true
		tools.MemorySessions = settings.Memory.Sessions
	}
	return tools
}

// screenFor says a session may be offered a screen of its own: it is not an
// Ask, which answers rather than works an app, the desktop can host one and,
// for a team task, its team was given one.
func (d *Daemon) screenFor(info protocol.SessionInfo) bool {
	if info.Kind == protocol.KindAsk {
		return false
	}
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
