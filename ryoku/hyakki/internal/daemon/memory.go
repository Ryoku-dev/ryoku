package daemon

import (
	"context"
	"encoding/json"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/memory"
	"ryoku-rashin/internal/protocol"
)

// teamMemory are the memory methods a team task's tools may call: search,
// remember and forget. What each may reach is memory's to decide from the
// team the session belongs to.
var teamMemory = map[string]bool{
	protocol.MMemorySearch: true,
	protocol.MMemoryAdd:    true,
	protocol.MMemoryForget: true,
}

// isMemoryMethod reports a method memory serves.
func isMemoryMethod(method string) bool { return strings.HasPrefix(method, "memory.") }

// memoryCall serves one memory.* method for the caller the connection is:
// only the Rashin app makes the calls memory keeps to the user.
func (d *Daemon) memoryCall(ctx context.Context, c *connection, method string, raw json.RawMessage) (any, error) {
	if d.deps.Memory == nil {
		return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Memory is not available."))
	}
	c.mu.RLock()
	kind, name := c.kind, c.name
	c.mu.RUnlock()
	if kind == protocol.ClientMCP {
		// mcpGate let through only a search and a suggestion; memory serves
		// them as the bridge's, never as the user's
		return d.deps.Memory.Handle(ctx, core.MemoryCaller{Bridge: name}, method, raw)
	}
	if memory.UserOnly(method) {
		if err := requireApp(c); err != nil {
			return nil, err
		}
	}
	from, err := d.memoryCaller(c, method, raw)
	if err != nil {
		return nil, err
	}
	return d.deps.Memory.Handle(ctx, from, method, raw)
}

// sessionMemory are the memory methods a session's memory tools call, or
// another program calls for a conversation: memory reads within what the
// conversation may read, and writes show in it.
var sessionMemory = map[string]bool{
	protocol.MMemorySearch:   true,
	protocol.MMemorySessions: true,
	protocol.MMemoryAdd:      true,
	protocol.MMemoryForget:   true,
}

// memoryCaller fills who calls from the caller, never from what it claims.
// A session's own tools (the tool host) are a team task's agent for a team
// session, else that session. On the main socket the caller is the app or
// another program, such as the CLI. Another program's search, remember or
// forget names the conversation it runs for, which is taken only when it is
// no team's; the app's own calls are the user's, whatever they name.
func (d *Daemon) memoryCaller(c *connection, method string, raw json.RawMessage) (core.MemoryCaller, error) {
	if c.session != "" {
		info, ok := d.manager.Info(c.session)
		switch {
		case !ok:
			return core.MemoryCaller{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("An agent can use only its own tools here."))
		case info.Team == "":
			return core.MemoryCaller{Session: info.ID}, nil
		case info.TeamTask == "":
			return core.MemoryCaller{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("An agent can use only its own tools here."))
		}
		return core.MemoryCaller{Team: info.Team, Task: info.TeamTask}, nil
	}
	from := core.MemoryCaller{App: c.app}
	if c.app || !sessionMemory[method] {
		return from, nil
	}
	var asked struct {
		Session string `json:"session"`
	}
	if err := json.Unmarshal(raw, &asked); err != nil || asked.Session == "" {
		return from, nil
	}
	if info, ok := d.manager.Info(asked.Session); ok && info.Team == "" {
		from.Session = info.ID
	}
	return from, nil
}

// SetMemoryEngine makes engine the one memory keeps everything in, as the
// app would in Settings; memory calls it once a move has copied everything.
func (d *Daemon) SetMemoryEngine(engine string) error {
	patch, err := json.Marshal(map[string]any{"memory": map[string]string{"engine": engine}})
	if err != nil {
		return err
	}
	_, err = d.applySettings(context.Background(), patch)
	return err
}
