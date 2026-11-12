package daemon

import (
	"encoding/json"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// agentIndex are the index methods an agent may call from its sandbox: the
// ones that read an index, never the ones that add, remove or rebuild one.
// agentIndexCall holds them to the indexes inside the team's folders.
var agentIndex = map[string]bool{
	protocol.MIndexList:       true,
	protocol.MIndexStatus:     true,
	protocol.MIndexSearch:     true,
	protocol.MIndexFind:       true,
	protocol.MIndexDef:        true,
	protocol.MIndexOutline:    true,
	protocol.MIndexReferences: true,
	protocol.MIndexPeek:       true,
	protocol.MIndexOverview:   true,
	protocol.MIndexImpact:     true,
	protocol.MIndexWip:        true,
	protocol.MIndexChanged:    true,
}

// teamControl are the methods only the Rashin app may call on a team task's
// session: answering for it, steering it, or ending it is the user's.
var teamControl = map[string]bool{
	protocol.MSessionPermission: true,
	protocol.MSessionSetModel:   true,
	protocol.MSessionSetMode:    true,
	protocol.MSessionSetOption:  true,
	protocol.MSessionPrompt:     true,
	protocol.MSessionQueue:      true,
	protocol.MSessionUnqueue:    true,
	protocol.MSessionSteer:      true,
	protocol.MSessionGoal:       true,
	protocol.MSessionCancel:     true,
	protocol.MSessionStop:       true,
	protocol.MSessionStart:      true,
	protocol.MSessionRename:     true,
	protocol.MSessionDelete:     true,
	protocol.MSessionHandoff:    true,
	protocol.MSessionFork:       true,
	protocol.MSessionRevertTurn: true,
	protocol.MSessionSetMemory:  true,
}

// isAgentMethod reports a method that only a team task's agent calls.
func isAgentMethod(method string) bool { return strings.HasPrefix(method, "teams.agent_") }

// agentGate is what a connection on a team session's own socket may call:
// the team tools, that session's goal and screen, acting as its agent,
// reading the indexes inside its team's folders, searching, remembering
// and forgetting in its team's memory, and reading its team's built-in
// skills. Everything else is refused.
func agentGate(session, method string, raw json.RawMessage) error {
	if isAgentMethod(method) || agentIndex[method] || teamMemory[method] || isSkillsMethod(method) {
		return nil
	}
	switch method {
	case protocol.MSessionGoal:
		p, err := unmarshal[protocol.GoalParams](raw)
		if err != nil {
			return err
		}
		if p.ID == session {
			return nil
		}
	case protocol.MScreenAct:
		p, err := unmarshal[protocol.ScreenActParams](raw)
		if err != nil {
			return err
		}
		if p.ID == session && p.By == protocol.HolderAgent {
			return nil
		}
	case protocol.MScreenControl:
		p, err := unmarshal[protocol.ScreenControlParams](raw)
		if err != nil {
			return err
		}
		if p.ID == session && (p.Op == protocol.ScreenAsk || p.Op == protocol.ScreenStop) {
			return nil
		}
	}
	return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("An agent can use only its own tools here."))
}

// mainGate keeps the main socket from doing what is not its to do: a team
// task's agent calls in only on its session's own socket, team sessions are
// made by the daemon alone, and only the Rashin app controls one.
func (d *Daemon) mainGate(c *connection, method string, raw json.RawMessage) error {
	if isAgentMethod(method) {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("This call does not come from the task's own conversation."))
	}
	if method == protocol.MSessionNew {
		p, err := unmarshal[protocol.SessionNewParams](raw)
		if err != nil {
			return err
		}
		if p.Team != "" || p.TeamTask != "" || p.Member != "" {
			return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only Rashin starts a team task's conversation."))
		}
		return nil
	}
	if !teamControl[method] || c.app {
		return nil
	}
	p, err := unmarshal[protocol.SessionRef](raw)
	if err != nil {
		return err
	}
	if info, ok := d.manager.Info(p.ID); ok && info.Team != "" {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Rashin app can do this."))
	}
	return nil
}

// requireApp refuses a call only the Rashin app may make.
func requireApp(c *connection) error {
	if c.app {
		return nil
	}
	return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Rashin app can do this."))
}

// settingsForApp reports whether a settings patch reaches what only the
// Rashin app may change: the teams' settings, which set what their agents
// may reach, the harness instances, whose command and environment team
// agents run with, the providers' settings, which name the addresses the
// gateway sends requests to, memory's, so no agent can turn memory on,
// widen what it reads or point it at another server, or the skills', so no
// agent chooses what every model is told. A patch that replaces a whole
// object, or is no object at all, reaches everything under it.
func settingsForApp(patch json.RawMessage) bool {
	var top map[string]json.RawMessage
	if json.Unmarshal(patch, &top) != nil || top == nil {
		return true
	}
	for _, key := range []string{"teams", "providers", "memory", "skills"} {
		if _, ok := top[key]; ok {
			return true
		}
	}
	for object, field := range map[string]string{"agent": "instances"} {
		raw, ok := top[object]
		if !ok {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			return true
		}
		if _, ok := fields[field]; ok {
			return true
		}
	}
	return false
}
