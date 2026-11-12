package daemon

import (
	"context"
	"encoding/json"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/harness"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// askHarness starts the harness that answers an Ask: Ask's brief ahead of
// the first question, Rashin's MCP tools, and Ask's own mode deciding every
// permission. While the Ask is read-only the harness runs in askSandbox, so
// what it does in its own process cannot change anything either. A harness
// that cannot start says why as this Ask's error, and no other engine
// answers in its place.
func (m *sessionManager) askHarness(ctx context.Context, s *managedSession, info protocol.SessionInfo, resume json.RawMessage) (core.Runner, error) {
	if info.Cwd == "" {
		info.Cwd = paths.Home()
	}
	options := core.HarnessOptions{
		Session: info, Harness: info.Harness, Cwd: info.Cwd, Resume: resume, Ask: true,
		Approvals: func() string { return s.log.Info().Mode },
	}
	if m.askBrief != nil {
		options.Brief = func() string { return m.askBrief(s.log.Info()) }
	}
	if m.tools != nil {
		options.Tools = m.tools(info)
	}
	if m.askSandbox != nil {
		var err error
		if options.Sandbox, err = m.askSandbox(info); err != nil {
			return nil, err
		}
	}
	runner, err := m.harnesses.NewAgent(ctx, options)
	if err != nil {
		return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("%s answers this Ask and cannot start: %v", harness.Name(info.Harness, m.settings()), err))
	}
	return runner, nil
}

// askBrief is what a harness that answers an Ask hears ahead of the first
// question: who Ask is and what its mode lets it do, the vault, Rashin, the
// built-in skills, teams and what Rashin remembers that it may use.
func (d *Daemon) askBrief(info protocol.SessionInfo) string {
	tools := len(d.agentTools(info)) > 0
	session := brief.Session{Info: info, Tools: tools, Skills: d.skillsBrief(info, tools)}
	if d.deps.Memory != nil {
		session.Memory = d.deps.Memory.Brief(info)
	}
	return brief.Ask(session)
}
