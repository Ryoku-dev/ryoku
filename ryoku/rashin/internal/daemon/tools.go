package daemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/screen"
)

// agentTools hands an agent session Rashin's own MCP server, which serves the
// code search over its folder, the session's goal and, where the desktop can
// host one, its screen. Without the rashin command beside the daemon or on
// PATH the session gets none.
func (d *Daemon) agentTools(info protocol.SessionInfo) []core.MCPServer {
	command := d.commandPath()
	if command == "" {
		return nil
	}
	env := map[string]string{"RASHIN_SESSION": info.ID, "RASHIN_CWD": info.Cwd}
	if screenable() {
		env["RASHIN_SCREEN"] = "1"
	}
	return []core.MCPServer{{Name: "rashin", Command: command, Args: []string{"mcp"}, Env: env}}
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
