package daemon

import (
	"encoding/json"
	"slices"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/feature"
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
	protocol.MSessionRename:     true,
	protocol.MSessionDelete:     true,
	protocol.MSessionHandoff:    true,
	protocol.MSessionFork:       true,
	protocol.MSessionRevertTurn: true,
	protocol.MSessionSetMemory:  true,
}

// isAgentMethod reports a method that only a team task's own tools call.
func isAgentMethod(method string) bool { return strings.HasPrefix(method, "teams.agent_") }

// agentGate is what a connection on a team session's own socket may call:
// reading the indexes inside its team's folders, which the rashin command
// typed into the session's shell does. The team tools run in the daemon and
// never come this way, so everything else is refused.
func agentGate(method string) error {
	if agentIndex[method] {
		return nil
	}
	return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("An agent can use only its own tools here."))
}

// connectionBound are the methods that answer through a connection of their
// own: subscribing, attaching, watching and the calls that attach a session.
// A session's tools call in-process and have none.
var connectionBound = map[string]bool{
	protocol.MHello:         true,
	protocol.MSubscribe:     true,
	protocol.MShutdown:      true,
	protocol.MSessionAttach: true,
	protocol.MSessionDetach: true,
	protocol.MProjectsWatch: true,
	protocol.MAsk:           true,
	protocol.MGlance:        true,
	protocol.MOniOpen:       true,
	protocol.MOniClose:      true,
}

// oniHands are the Oni calls that stand for the user's own hand on Oni, or
// for the surface drawing it: moving it, touching it, reporting where the
// strand landed and where it draws, perching it, letting go, looking at
// and asking about the perched window, the user's own voice, guides,
// keeping them and picking a spot to ask about, the reminders, routines
// and reflex ledger kept for the user, Hands in the browser tab the user
// authorized with its ledger, and rewriting a selection, recall with its
// chat import, the focus guard and assignment mode with its ledger. No
// agent makes them.
var oniHands = map[string]bool{
	protocol.MOniPlace:         true,
	protocol.MOniInteract:      true,
	protocol.MOniStrandArrived: true,
	protocol.MOniPerch:         true,
	protocol.MOniDetach:        true,
	protocol.MOniInspect:       true,
	protocol.MOniAsk:           true,
	protocol.MOniConsentAnswer: true,
	protocol.MOniOverlay:       true,
	protocol.MOniTalk:          true,
	protocol.MOniSpeak:         true,
	protocol.MOniDictate:       true,
	protocol.MOniVoiceTest:     true,
	protocol.MOniGuide:         true,
	protocol.MOniKeep:          true,
	protocol.MOniSkip:          true,
	protocol.MOniPick:          true,
	protocol.MOniReminders:     true,
	protocol.MOniRoutines:      true,
	protocol.MOniReflexLog:     true,
	protocol.MOniHands:         true,
	protocol.MOniHandsLog:      true,
	protocol.MOniRewrite:       true,
	protocol.MOniRecall:        true,
	protocol.MOniRecallImport:  true,
	protocol.MOniFocus:         true,
	protocol.MOniAssignment:    true,
	protocol.MOniAssignmentLog: true,
}

// oniReflex are the reflex calls: listing and cancelling reminders,
// listing, replaying and forgetting routines, and reading the reflex
// ledger. Only the app and the CLI make them; the surface drawing Oni, an
// observer, the bar and the Hub never do. A routine offer is answered with
// oni.keep and oni.skip, and a decision card with oni.consent.answer, as
// before.
var oniReflex = map[string]bool{
	protocol.MOniReminders: true,
	protocol.MOniRoutines:  true,
	protocol.MOniReflexLog: true,
}

// oniPresenterActions are the guide, keep and pick calls, each with the
// actions the surface drawing Oni may send: its card's Next, Stop and circle
// pills, Skip on a keep offer or a hint, and an armed pick's one click or
// its cancel (the overlay takes every click and Esc never reaches it). The
// app and the CLI send every action. The presenter never starts a guide,
// keeps one or arms a pick, so a keep is answered by the app, the CLI or a
// spoken yes from a press. Skip and keep carry no action; "" stands for
// the call.
var oniPresenterActions = map[string][]string{
	protocol.MOniGuide: {protocol.OniGuideNext, protocol.OniGuideStop, protocol.OniGuideCircle},
	protocol.MOniKeep:  nil,
	protocol.MOniSkip:  {""},
	protocol.MOniPick:  {protocol.OniPickPoint, protocol.OniPickCancel},
}

// Who may send each oni.hands action besides the app, which sends every
// one. Authorizing a tab, starting a task and resuming a paused one are
// the user's own press, so only the app or the surface drawing Oni sends
// them: never the CLI, a voice, or a program speaking as the user. Stop is
// open to everyone who may call Oni at all. The CLI may open the Hands
// browser, list its tabs, ask where Hands stands and read the task
// journal; the presenter, whose pill and card draw the task, may not list
// the tabs or open the browser. Forgetting the Hands browser deletes the
// user's signed-in profile, picking a live window binds the user's own
// app, and clearing the journal deletes it, so those are the app's alone,
// and so is starting a task on the separate desktop, the one scope whose
// start is a per-task approval in the app.
var (
	oniHandsCLI       = []string{protocol.OniHandsOpen, protocol.OniHandsTabs, protocol.OniHandsStop, protocol.OniHandsStatus, protocol.OniHandsTasks}
	oniHandsPresenter = []string{protocol.OniHandsAuthorize, protocol.OniHandsStart, protocol.OniHandsStop, protocol.OniHandsResume}
)

// oniUser are the Phase 16 calls, each with the actions the surface drawing
// Oni may send besides the app and the CLI, which send every one: the
// guard's stop pill, and an assignment card's Next, Back, Done and Stop.
// Rewriting a selection, recall, importing chats and reading the
// assignment ledger are the app's and the CLI's alone; the presenter never
// starts anything, arms the guard or brings a closed feed back.
var oniUser = map[string][]string{
	protocol.MOniRewrite:       nil,
	protocol.MOniRecall:        nil,
	protocol.MOniRecallImport:  nil,
	protocol.MOniFocus:         {protocol.OniFocusStop},
	protocol.MOniAssignment:    {protocol.OniAssignmentNext, protocol.OniAssignmentBack, protocol.OniAssignmentDone, protocol.OniAssignmentStop},
	protocol.MOniAssignmentLog: nil,
}

// oniVoice are the voice calls, which open the microphone or stop Oni
// speaking. Only the app and the CLI make them, and, where the value is
// true, the surface drawing Oni, whose chord reader is a key source. The
// mic meter is Settings' alone. Another surface, a connection watching as
// an observer, the bar and the hub never make them.
var oniVoice = map[string]bool{
	protocol.MOniTalk:      true,
	protocol.MOniSpeak:     true,
	protocol.MOniDictate:   true,
	protocol.MOniVoiceTest: false,
}

// surfaceMethods are all a surface drawing Oni may call. It may perch Oni
// where the user dropped it and let go, pass on the user's talk chord,
// press a guide card's pills, skip a keep offer, report a pick's click,
// press Hands' authorize, start, stop and resume, stop the focus guard,
// press an assignment card's pills and a Yokai approval card's buttons
// (oni.consent.answer answers it no other card), but never looks or asks.
var surfaceMethods = map[string]bool{
	protocol.MStatus:             true,
	protocol.MOniStatus:          true,
	protocol.MOniCharacters:      true,
	protocol.MOniSpec:            true,
	protocol.MOniOpen:            true,
	protocol.MOniClose:           true,
	protocol.MOniPlace:           true,
	protocol.MOniInteract:        true,
	protocol.MOniCharacterAssets: true,
	protocol.MOniStrandArrived:   true,
	protocol.MOniPerch:           true,
	protocol.MOniDetach:          true,
	protocol.MOniOverlay:         true,
	protocol.MOniTalk:            true,
	protocol.MOniSpeak:           true,
	protocol.MOniDictate:         true,
	protocol.MOniGuide:           true,
	protocol.MOniSkip:            true,
	protocol.MOniPick:            true,
	protocol.MOniConsentAnswer:   true,
	protocol.MOniHands:           true,
	protocol.MOniFocus:           true,
	protocol.MOniAssignment:      true,
}

// partsAndActivity are the Phase 17 calls on the main socket, each with
// whether only the Hyakki app may make it. Listing the optional parts and
// reading Oni's activity is open to the app, the CLI and the Hub; removing
// a part and deleting activity delete what the user downloaded or kept, so
// they are the app's alone. No observer, bar, surface, agent or MCP bridge
// makes any of them.
var partsAndActivity = map[string]bool{
	protocol.MComponents:       false,
	protocol.MComponentRemove:  true,
	protocol.MOniActivity:      false,
	protocol.MOniActivityClear: true,
}

// partsCaller lets the app, the CLI and the Hub make a Phase 17 read, and
// the app alone a removal or a delete. A connection watching Oni as an
// observer or bound to a session makes none.
func (d *Daemon) partsCaller(c *connection, appOnly bool) error {
	d.oni.clientsMu.Lock()
	client := d.oni.clients[c]
	d.oni.clientsMu.Unlock()
	c.mu.RLock()
	kind := c.kind
	c.mu.RUnlock()
	watching := c.session != "" || client != nil && client.renderer == protocol.OniRendererObserver
	switch {
	case !watching && c.app, !watching && !appOnly && (kind == protocol.ClientCLI || kind == protocol.ClientHub):
		return nil
	case appOnly:
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app can do this."))
	}
	return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app, the hyakki command or the Hub can read this."))
}

// mcpGate is all a connection from the MCP bridge may call: its own state
// while the bridge is on, and, once Settings approves the program by the
// name it gave at hello, the calls behind each tool turned on. Every other
// call, and every other action of those calls, is refused.
func (d *Daemon) mcpGate(c *connection, method string, raw json.RawMessage) error {
	settings := protocol.EffectiveOni(d.Settings()).MCP
	if !settings.Enabled {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("The MCP bridge is off. Turn it on in Hyakki's Settings first."))
	}
	if method == protocol.MOniMCP {
		return nil
	}
	c.mu.RLock()
	name := c.name
	c.mu.RUnlock()
	if !slices.Contains(settings.Clients, name) {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.Tf("Approve %s in Hyakki's Settings first.", name))
	}
	tool, err := mcpTool(method, raw)
	if err != nil {
		return err
	}
	if tool == "" {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("A program on the MCP bridge can use only the tools Hyakki offers it."))
	}
	if !settings.Tools[tool] {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("This tool is off in Hyakki's Settings."))
	}
	return nil
}

// mcpTool is the MCP bridge tool a call stands for, "" for none: listing or
// adding reminders, searching memory, suggesting a note (memory.import,
// whose result waits in the Inbox as suggestions), reading the focus guard,
// a line on Oni's pill, and asking about the perched window.
func mcpTool(method string, raw json.RawMessage) (string, error) {
	switch method {
	case protocol.MMemorySearch:
		return protocol.OniMCPNotesSearch, nil
	case protocol.MMemoryImport:
		return protocol.OniMCPNotesAdd, nil
	case protocol.MOniNotify:
		return protocol.OniMCPNotify, nil
	case protocol.MOniAsk:
		return protocol.OniMCPScreenAsk, nil
	case protocol.MOniReminders, protocol.MOniFocus:
		params, err := feature.DecodeCall(method, raw)
		if err != nil {
			return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni call is invalid: %v.", err))
		}
		switch p := params.(type) {
		case *protocol.OniRemindersParams:
			switch p.Action {
			case protocol.OniRemindersList:
				return protocol.OniMCPRemindersList, nil
			case protocol.OniRemindersAdd:
				return protocol.OniMCPRemindersAdd, nil
			}
		case *protocol.OniFocusParams:
			if p.Action == protocol.OniFocusStatus {
				return protocol.OniMCPFocusStatus, nil
			}
		}
	}
	return "", nil
}

// hostGate is what a session's own tools may call through the tool host
// (toolhost.go). A team task's tools call only what its agent socket once
// served: its team tools, its own goal and screen, the indexes inside its
// team's folders, searching, remembering and forgetting in its team's
// memory, its team's skills, and reading or proposing teams. Any other
// session's tools call what any program on the main socket may, never as
// the app, never a team tool, never the settings and never Oni's presenter
// calls, and a goal or screen only their own.
func (d *Daemon) hostGate(info protocol.SessionInfo, method string, raw json.RawMessage) error {
	refused := protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("An agent can use only its own tools here."))
	if connectionBound[method] || oniHands[method] || method == protocol.MSettingsSet {
		return refused
	}
	switch method {
	case protocol.MTeamsMessages, protocol.MTeamsTaskContext, protocol.MTeamsTaskComment, protocol.MTeamsTaskReassign, protocol.MTeamsTaskPlan:
		// These are the owner's views and interventions, not agent callbacks,
		// even when a non-team tool host can otherwise reach the main socket.
		return refused
	case protocol.MSessionGoal:
		p, err := unmarshal[protocol.GoalParams](raw)
		if err != nil {
			return err
		}
		if p.ID != info.ID {
			return refused
		}
		return nil
	case protocol.MScreenAct:
		p, err := unmarshal[protocol.ScreenActParams](raw)
		if err != nil {
			return err
		}
		if p.ID != info.ID || p.By != protocol.HolderAgent {
			return refused
		}
		return nil
	case protocol.MScreenControl:
		p, err := unmarshal[protocol.ScreenControlParams](raw)
		if err != nil {
			return err
		}
		if p.ID != info.ID || p.Op != protocol.ScreenAsk && p.Op != protocol.ScreenStop {
			return refused
		}
		return nil
	}
	if info.Team == "" {
		if isAgentMethod(method) {
			return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("This call does not come from the task's own conversation."))
		}
		return d.mainGate(&connection{daemon: d}, method, raw)
	}
	switch {
	case isAgentMethod(method), agentIndex[method], teamMemory[method], isSkillsMethod(method):
		return nil
	case method == protocol.MTeamsList, method == protocol.MTeamsTemplates, method == protocol.MTeamsPropose:
		return nil
	}
	return refused
}

// mainGate keeps the main socket from doing what is not its to do: a team
// task's tools call in only from inside the daemon, team sessions are made
// by the daemon alone, and only the Rashin app controls one.
func (d *Daemon) mainGate(c *connection, method string, raw json.RawMessage) error {
	if isAgentMethod(method) {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("This call does not come from the task's own conversation."))
	}
	c.mu.RLock()
	surface := c.kind == protocol.ClientSurface
	mcp := c.kind == protocol.ClientMCP
	c.mu.RUnlock()
	if mcp {
		return d.mcpGate(c, method, raw)
	}
	if surface && !surfaceMethods[method] {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("A surface drawing Oni can call only Oni's methods."))
	}
	if appOnly, ok := partsAndActivity[method]; ok {
		return d.partsCaller(c, appOnly)
	}
	switch method {
	case protocol.MOniNotify:
		return requireApp(c)
	case protocol.MOniMCP:
		return d.oniCaller(c, false)
	}
	if presenter, ok := oniVoice[method]; ok {
		return d.oniCaller(c, presenter)
	}
	if actions, ok := oniPresenterActions[method]; ok {
		return d.oniActionCaller(c, method, raw, actions)
	}
	if method == protocol.MOniHands || method == protocol.MOniHandsLog {
		return d.oniHandsCaller(c, method, raw)
	}
	if oniReflex[method] {
		return d.oniCaller(c, false)
	}
	if actions, ok := oniUser[method]; ok {
		return d.oniUserCaller(c, method, raw, actions)
	}
	var models []string
	switch method {
	case protocol.MTeamsApprovalDecide:
		if err := d.handsDecideGate(c, raw); err != nil {
			return err
		}
	case protocol.MSessionNew:
		p, err := unmarshal[protocol.SessionNewParams](raw)
		if err != nil {
			return err
		}
		if p.Team != "" || p.TeamTask != "" || p.Member != "" || p.Runtime != "" {
			return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only Hyakki starts a team task's conversation."))
		}
		return hostedNeedsApp(c, p.Model)
	case protocol.MSessionSetModel:
		p, err := unmarshal[protocol.SetModelParams](raw)
		if err != nil {
			return err
		}
		models = []string{p.Model}
	case protocol.MAsk:
		p, err := unmarshal[protocol.AskParams](raw)
		if err != nil {
			return err
		}
		models = []string{p.Model}
	case protocol.MGlance:
		p, err := unmarshal[protocol.GlanceParams](raw)
		if err != nil {
			return err
		}
		models = []string{p.Model}
	case protocol.MCompareStart:
		p, err := unmarshal[protocol.CompareStartParams](raw)
		if err != nil {
			return err
		}
		models = p.Models
	}
	if err := hostedNeedsApp(c, models...); err != nil {
		return err
	}
	if !teamControl[method] || c.app {
		return nil
	}
	p, err := unmarshal[protocol.SessionRef](raw)
	if err != nil {
		return err
	}
	if info, ok := d.manager.Info(p.ID); ok && info.Team != "" {
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app can do this."))
	}
	return nil
}

// oniActionCaller lets the app and the CLI make a guide, keep or pick call,
// and the presenter only the actions it may send; nobody else makes one.
func (d *Daemon) oniActionCaller(c *connection, method string, raw json.RawMessage, presenter []string) error {
	if err := d.oniCaller(c, true); err != nil {
		return err
	}
	d.oni.clientsMu.Lock()
	isPresenter := d.oni.presenter == c
	d.oni.clientsMu.Unlock()
	if !isPresenter {
		return nil
	}
	params, err := feature.DecodeCall(method, raw)
	if err != nil {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni call is invalid: %v.", err))
	}
	action := ""
	switch p := params.(type) {
	case *protocol.OniGuideParams:
		action = p.Action
	case *protocol.OniPickParams:
		action = p.Action
	}
	if slices.Contains(presenter, action) {
		return nil
	}
	return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("The surface drawing Oni can press Next, Stop and circle, skip an offer, and report or cancel a pick, but nothing more."))
}

// oniHandsCaller lets the app send every oni.hands action, the CLI and the
// presenter only theirs, and the app and the CLI read the Hands ledger;
// nobody else makes a Hands call.
func (d *Daemon) oniHandsCaller(c *connection, method string, raw json.RawMessage) error {
	if method == protocol.MOniHandsLog {
		return d.oniCaller(c, false)
	}
	if err := d.oniCaller(c, true); err != nil {
		return err
	}
	params, err := feature.DecodeCall(method, raw)
	if err != nil {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni call is invalid: %v.", err))
	}
	p := params.(*protocol.OniHandsParams)
	action := p.Action
	d.oni.clientsMu.Lock()
	isPresenter := d.oni.presenter == c
	d.oni.clientsMu.Unlock()
	switch {
	case isPresenter && action == protocol.OniHandsStart && handsScopeOf(protocol.EffectiveOni(d.Settings()).Hands, p.Scope) == protocol.OniHandsScopeDesktop:
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app starts Hands on its separate desktop."))
	case isPresenter:
		if slices.Contains(oniHandsPresenter, action) {
			return nil
		}
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("The surface drawing Oni can authorize a tab, start Hands, stop it and resume it, but nothing more."))
	case c.app, slices.Contains(oniHandsCLI, action):
		return nil
	case action == protocol.OniHandsForget:
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app can forget the Hands browser."))
	case action == protocol.OniHandsSelect || action == protocol.OniHandsWindows || action == protocol.OniHandsClear:
		return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app can pick the window Hands works in or clear its tasks."))
	}
	return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app or the surface drawing Oni can authorize a tab, start Hands or resume it."))
}

// oniUserCaller lets the app and the CLI make a Phase 16 call, and the
// presenter only the actions it may send; nobody else makes one. What
// decodes for nobody is a bad request, never a pass.
func (d *Daemon) oniUserCaller(c *connection, method string, raw json.RawMessage, presenter []string) error {
	if err := d.oniCaller(c, len(presenter) > 0); err != nil {
		return err
	}
	params, err := feature.DecodeCall(method, raw)
	if err != nil {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni call is invalid: %v.", err))
	}
	d.oni.clientsMu.Lock()
	isPresenter := d.oni.presenter == c
	d.oni.clientsMu.Unlock()
	if !isPresenter {
		return nil
	}
	action := ""
	switch p := params.(type) {
	case *protocol.OniFocusParams:
		action = p.Action
	case *protocol.OniAssignmentParams:
		action = p.Action
	}
	if slices.Contains(presenter, action) {
		return nil
	}
	return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("The surface drawing Oni can stop the focus guard and press an assignment card's Next, Back, Done and Stop, but nothing more."))
}

// hostedNeedsApp keeps the Ryoku Account's models for the Rashin app: their
// credits are the user's, and a model named on the socket is no pick.
func hostedNeedsApp(c *connection, models ...string) error {
	if c.app {
		return nil
	}
	for _, model := range models {
		if protocol.IsHostedModel(model) {
			return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app can use Ryoku Ember through this linked Account session."))
		}
	}
	return nil
}

// requireApp refuses a call only the Rashin app may make.
func requireApp(c *connection) error {
	if c.app {
		return nil
	}
	return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app can do this."))
}

// settingsForApp reports whether a settings patch reaches what only the
// Rashin app may change: the teams' settings, which set what their agents
// may reach, the providers' settings, which name the addresses the gateway
// sends requests to and the llama-server Rashin runs, memory's, so no agent
// can turn memory on, widen what it reads or point it at another server,
// the skills', so no agent chooses what every model is told, Oni's hands
// stage, so no program but the app lets Hands drive input, or Oni's
// recall, rewrite and assignment blocks, so no program but the app widens
// what Oni reads, types back or sends, Oni's MCP bridge block, so no
// program turns the bridge on, approves itself or picks its own tools, or
// the first-run tour's state, which the app alone shows. A patch
// that replaces a whole object, or is no object at all, reaches everything
// under it. So does a default Ask model that is one of the Ryoku Account's.
// Keys are matched as the settings decoder matches them, without regard to
// case, so no spelling of a key slips past.
func settingsForApp(patch json.RawMessage) bool {
	top, ok := patchObject(patch)
	if !ok {
		return true
	}
	for key, value := range top {
		for _, app := range []string{"teams", "providers", "memory", "skills", "onboarding"} {
			if strings.EqualFold(key, app) {
				return true
			}
		}
		switch {
		case strings.EqualFold(key, "oni"):
			oni, ok := patchObject(value)
			if !ok {
				return true
			}
			for name := range oni {
				if strings.EqualFold(name, "hands") || strings.EqualFold(name, "recall") || strings.EqualFold(name, "rewrite") || strings.EqualFold(name, "assignment") || strings.EqualFold(name, "mcp") {
					return true
				}
			}
		case strings.EqualFold(key, "ask"):
			// a default that names a hosted model would spend the account
			// on every question a client asks, with no pick by the user
			var ask struct {
				Model string `json:"model"`
			}
			if json.Unmarshal(value, &ask) == nil && protocol.IsHostedModel(ask.Model) {
				return true
			}
		}
	}
	return false
}

// patchObject is raw as a JSON object's members; ok is false for null or
// anything that is no object.
func patchObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return nil, false
	}
	return object, true
}
