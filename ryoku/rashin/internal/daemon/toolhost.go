package daemon

import (
	"context"
	"encoding/json"
	"errors"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
	"ryoku-rashin/internal/teams"
)

// ToolHost is the daemon's method dispatch as one session's tools call it,
// for engine.Deps.Host. A call carries the session's own identity, filled
// by the daemon and never claimed by the tool: memory sees MemoryCaller
// Session for the session, or Team and Task for a team task; skills keep to
// the sets the session is given; a team task's index reads keep to its
// team's folders, and Teams checks its team tools against the session.
// What a session's tools may call at all is hostGate's to decide.
func (d *Daemon) ToolHost(info protocol.SessionInfo) tool.Host {
	return sessionHost{daemon: d, info: info}
}

// sessionHost is ToolHost's tool.Host.
type sessionHost struct {
	daemon *Daemon
	info   protocol.SessionInfo
}

// Call runs method in the daemon as the session and decodes what it returns
// into result, which may be nil. A refused or failed call is the method's
// own error, so a tool says what the user would read.
func (h sessionHost) Call(ctx context.Context, method string, params, result any) error {
	d := h.daemon
	if _, ok := protocol.Lookup(method); !ok {
		return protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.Tf("Method %s is not supported.", method))
	}
	info := h.info
	if live, ok := d.manager.Info(info.ID); ok {
		info = live
	}
	raw, err := protocol.Marshal(params)
	if err != nil {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Request parameters are invalid."))
	}
	if err := d.hostGate(info, method, raw); err != nil {
		return err
	}
	c := &connection{daemon: d, session: info.ID}
	var out any
	if info.Team != "" {
		ctx = teams.WithAgentSession(ctx, info.ID)
	}
	if info.Team != "" && agentIndex[method] {
		out, err = d.agentIndexCall(ctx, c, method, raw)
	} else {
		out, err = d.dispatch(ctx, c, method, raw)
	}
	if err != nil || result == nil {
		return err
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, result)
}

// toolHostArg is the hidden rashin subcommand that serves a team task's file
// and shell tools inside its sandbox.
const toolHostArg = "__toolhost"

// ToolHostCommand is the command line that starts a team task's tool host:
// `rashin __toolhost` inside spec's bubblewrap sandbox, through `rashin
// scope` where the command knows it. The engine's sandbox executor runs it
// at its first tool call. It fails when bubblewrap cannot run here or no
// rashin command is found, so a team task without a sandbox never runs its
// tools unsandboxed.
func (d *Daemon) ToolHostCommand(spec sandbox.Spec) ([]string, error) {
	if err := sandbox.Available(); err != nil {
		return nil, err
	}
	if spec.Scope == "" {
		return nil, errors.New(i18n.T("The rashin command cannot scope this team's tools. Update it before starting the task."))
	}
	rashin := d.commandPath()
	if rashin == "" {
		return nil, errors.New(i18n.T("The rashin command was not found, so the team's tools cannot start."))
	}
	return sandbox.Command(spec, []string{rashin, toolHostArg})
}
