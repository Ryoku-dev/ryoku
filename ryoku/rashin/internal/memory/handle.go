package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/memory/scan"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
)

// Scope shorthands a model may use: the caller's project and its team.
const (
	scopeProject = "project"
	scopeTeam    = "team"
)

// Search limits.
const (
	searchDefault = 10
	searchMax     = 20
)

// UserOnly reports a memory method only the Rashin app may call.
func UserOnly(method string) bool {
	switch method {
	case protocol.MMemoryForgetAll, protocol.MMemoryKeep, protocol.MMemoryImport, protocol.MMemoryEngineSetup, protocol.MMemoryEngineMove:
		return true
	}
	return false
}

// callerMethods are what an agent session and the command line may call:
// reading, adding, editing and forgetting. The command line may also learn
// from a conversation now.
var callerMethods = map[string]bool{
	protocol.MMemoryStatus: true, protocol.MMemoryList: true, protocol.MMemoryGet: true, protocol.MMemorySearch: true,
	protocol.MMemorySessions: true, protocol.MMemoryCard: true, protocol.MMemoryChanges: true,
	protocol.MMemoryAdd: true, protocol.MMemoryEdit: true, protocol.MMemoryForget: true,
}

// writeMethods change memory; a conversation that only uses memory may not
// call them.
var writeMethods = map[string]bool{protocol.MMemoryAdd: true, protocol.MMemoryEdit: true, protocol.MMemoryForget: true}

// errTeamOnly refuses a team agent anything beyond its team's memory.
func errTeamOnly() error {
	return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("A team agent can only use its team's memory."))
}

// errAppOnly refuses a caller that is not the Rashin app.
func errAppOnly() error {
	return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Rashin app can do that."))
}

// errOutOfReach refuses a memory outside what the calling conversation may
// reach.
func errOutOfReach() error {
	return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("This conversation can't reach that memory."))
}

// decode reads a method's params; none is the zero value.
func decode[T any](params json.RawMessage) (T, error) {
	var v T
	if len(params) == 0 || string(params) == "null" {
		return v, nil
	}
	if err := json.Unmarshal(params, &v); err != nil {
		return v, protocol.Errorf(protocol.CodeBadRequest, "%s", err.Error())
	}
	return v, nil
}

// caller is who calls and what it may reach: info is the calling
// conversation (zero for none), within the scopes it may read and write,
// nil for the app, which reaches everything.
type caller struct {
	from   core.MemoryCaller
	info   protocol.SessionInfo
	within []string
}

// callerOf works out what a caller may reach: an agent session its
// conversation's scopes, anything else on the socket only what is about the
// user.
func (e *Engine) callerOf(from core.MemoryCaller) caller {
	c := caller{from: from}
	if from.App {
		return c
	}
	if from.Session == "" {
		c.within = []string{protocol.MemoryYou}
		return c
	}
	c.info = e.callerInfo(from.Session)
	c.within = e.readableScopes(c.info)
	return c
}

// reaches reports whether the caller may read and write scope.
func (c caller) reaches(scope string) bool {
	return c.within == nil || inScopes(scope, c.within)
}

// Handle serves one memory.* method for from, with the caller rules: the
// app may do anything; an agent session and the command line read, add,
// edit and forget within their reach; a team agent only searches, adds to
// and forgets its team's memory and suggests the rest.
func (e *Engine) Handle(ctx context.Context, from core.MemoryCaller, method string, params json.RawMessage) (any, error) {
	if from.Team != "" {
		return e.handleTeam(ctx, from, method, params)
	}
	c := e.callerOf(from)
	if !from.App {
		cli := from.Session == ""
		if UserOnly(method) || !callerMethods[method] && !(cli && method == protocol.MMemoryLearn) {
			return nil, errAppOnly()
		}
		if method != protocol.MMemoryStatus && !e.on() {
			e.emit(from.Session, protocol.MemoryEventData{Kind: protocol.MemoryRefused, Reason: protocol.MemoryRefusedOff})
			return nil, protocol.Errorf(protocol.CodeDisabled, "%s", i18n.T("Memory is off."))
		}
		switch {
		case cli:
		case c.info.Memory == protocol.MemoryNone:
			return nil, protocol.Errorf(protocol.CodeDisabled, "%s", i18n.T("This conversation doesn't use memory."))
		case c.info.Memory == protocol.MemoryUseOnly && writeMethods[method]:
			return nil, protocol.Errorf(protocol.CodeDisabled, "%s", i18n.T("This conversation only reads memory."))
		}
	}
	switch method {
	case protocol.MMemoryStatus:
		return e.Status(ctx), nil
	case protocol.MMemoryList:
		p, err := decode[protocol.MemoryListParams](params)
		if err != nil {
			return nil, err
		}
		return e.list(ctx, p, c.within)
	case protocol.MMemoryGet:
		p, err := decode[protocol.MemoryRefParams](params)
		if err != nil {
			return nil, err
		}
		m, err := e.get(ctx, p.ID)
		if err == nil && !c.reaches(m.Scope) {
			return nil, errNotFound(p.ID)
		}
		return m, err
	case protocol.MMemoryAdd:
		p, err := decode[protocol.MemoryAddParams](params)
		if err != nil {
			return nil, err
		}
		return e.add(ctx, c, p)
	case protocol.MMemoryEdit:
		p, err := decode[protocol.MemoryEditParams](params)
		if err != nil {
			return nil, err
		}
		return e.edit(ctx, c, p)
	case protocol.MMemoryForget:
		p, err := decode[protocol.MemoryForgetParams](params)
		if err != nil {
			return nil, err
		}
		return e.forget(ctx, c, p)
	case protocol.MMemoryRestore:
		p, err := decode[protocol.MemoryRestoreParams](params)
		if err != nil {
			return nil, err
		}
		return e.apply(from, "", []Op{{Kind: OpRestore, ID: p.ID, Version: p.Version}})
	case protocol.MMemoryKeep:
		p, err := decode[protocol.MemoryIDsParams](params)
		if err != nil {
			return nil, err
		}
		return e.keep(ctx, from, p.IDs)
	case protocol.MMemoryDismiss:
		p, err := decode[protocol.MemoryIDsParams](params)
		if err != nil {
			return nil, err
		}
		return e.apply(from, "", idOps(OpDismiss, p.IDs))
	case protocol.MMemoryUndo, protocol.MMemoryRedo:
		p, err := decode[protocol.MemoryChangeParams](params)
		if err != nil {
			return nil, err
		}
		return e.flip(ctx, p.Change, method == protocol.MMemoryUndo)
	case protocol.MMemoryChanges:
		p, err := decode[protocol.MemoryChangesParams](params)
		if err != nil {
			return nil, err
		}
		return e.changes(c, p.IDs)
	case protocol.MMemoryForgetAll:
		p, err := decode[protocol.MemoryForgetAllParams](params)
		if err != nil {
			return nil, err
		}
		return e.forgetAll(ctx, p.Scope)
	case protocol.MMemorySearch:
		p, err := decode[protocol.MemorySearchParams](params)
		if err != nil {
			return nil, err
		}
		return e.search(ctx, c, p)
	case protocol.MMemorySessions:
		p, err := decode[protocol.MemorySessionsParams](params)
		if err != nil {
			return nil, err
		}
		result, err := e.sessionsSearch(ctx, c.info, p)
		if err == nil && len(result.Sessions) > 0 {
			refs := make([]protocol.MemoryRef, len(result.Sessions))
			for index, hit := range result.Sessions {
				refs[index] = protocol.MemoryRef{Session: hit.Session, Kind: hit.Kind, Title: hit.Title}
			}
			e.emit(from.Session, protocol.MemoryEventData{Kind: protocol.MemoryLooked, Sessions: refs})
		}
		return result, err
	case protocol.MMemoryCard:
		p, err := decode[protocol.MemoryCardParams](params)
		if err != nil {
			return nil, err
		}
		if !c.reaches(protocol.MemoryYou) {
			return nil, errOutOfReach()
		}
		return e.cardGet(ctx, p.Rebuild)
	case protocol.MMemoryImport:
		p, err := decode[protocol.MemoryImportParams](params)
		if err != nil {
			return nil, err
		}
		change, err := e.importText(ctx, p)
		if err == nil {
			e.changed(change)
		}
		return change, err
	case protocol.MMemorySuppress:
		p, err := decode[protocol.MemorySuppressParams](params)
		if err != nil {
			return nil, err
		}
		return protocol.Empty{}, e.suppress(e.callerInfo(p.Session), p.ID)
	case protocol.MMemoryLearn:
		p, err := decode[protocol.MemoryLearnParams](params)
		if err != nil {
			return nil, err
		}
		return e.learnNow(ctx, p.Session)
	case protocol.MMemoryEngineTest:
		p, err := decode[protocol.MemoryEngineParams](params)
		if err != nil {
			return nil, err
		}
		if p.Engine == protocol.MemoryBuiltin || p.Engine == "" {
			return protocol.MemoryEngineTest{OK: true}, nil
		}
		if e.deps.Engines == nil {
			return protocol.MemoryEngineTest{Detail: i18n.T("This build has only the built-in engine.")}, nil
		}
		return e.deps.Engines.Test(ctx, e.settings(), p.Engine), nil
	case protocol.MMemoryEngineSetup:
		p, err := decode[protocol.MemoryEngineParams](params)
		if err != nil {
			return nil, err
		}
		if e.deps.Engines == nil {
			return nil, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.T("This build has only the built-in engine."))
		}
		return protocol.Empty{}, e.deps.Engines.Setup(ctx, e.settings(), p.Engine, func(progress protocol.MemorySetupProgress) {
			if e.deps.Broadcast != nil {
				e.deps.Broadcast(protocol.EvtMemorySetup, progress)
			}
		})
	case protocol.MMemoryEngineMove:
		p, err := decode[protocol.MemoryEngineParams](params)
		if err != nil {
			return nil, err
		}
		return e.move(ctx, p.Engine)
	}
	return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("Unknown method %s.", method))
}

// handleTeam serves a team agent: search, add and forget in its team's
// memory, suggestions about the user or a project when the team and the
// settings allow them, and nothing else.
func (e *Engine) handleTeam(ctx context.Context, from core.MemoryCaller, method string, params json.RawMessage) (any, error) {
	if e.deps.Names == nil {
		return nil, errTeamOnly()
	}
	team, ok := e.deps.Names.Team(from.Team)
	if !ok || !team.TeamMemory().Remember {
		return nil, errTeamOnly()
	}
	if !e.on() {
		return nil, protocol.Errorf(protocol.CodeDisabled, "%s", i18n.T("Memory is off."))
	}
	own := protocol.MemoryTeamPrefix + from.Team
	switch method {
	case protocol.MMemorySearch:
		p, err := decode[protocol.MemorySearchParams](params)
		if err != nil {
			return nil, err
		}
		var scopes []string
		switch p.Scope {
		case "":
			scopes = []string{own}
			if team.TeamMemory().ReadsYou {
				scopes = append(scopes, protocol.MemoryYou)
			}
		case scopeTeam, own:
			scopes = []string{own}
		case protocol.MemoryYou:
			if !team.TeamMemory().ReadsYou {
				return nil, errTeamOnly()
			}
			scopes = []string{protocol.MemoryYou}
		default:
			return nil, errTeamOnly()
		}
		return e.searchScopes(ctx, protocol.SessionInfo{}, p.Query, scopes, p.Limit)
	case protocol.MMemoryAdd:
		p, err := decode[protocol.MemoryAddParams](params)
		if err != nil {
			return nil, err
		}
		kind := OpAdd
		switch p.Scope {
		case "", scopeTeam, own:
			p.Scope = own
		case scopeProject:
			root := e.keys.root(team.Folder)
			if root == "" {
				return nil, errTeamOnly()
			}
			p.Scope = e.projectAt(root).Scope
			kind = OpSuggest
		default:
			if p.Scope != protocol.MemoryYou && !strings.HasPrefix(p.Scope, protocol.MemoryProjectPrefix) {
				return nil, errTeamOnly()
			}
			kind = OpSuggest
		}
		if kind == OpSuggest && (!team.TeamMemory().Suggest || e.settings().TeamSuggestions != protocol.TeamSuggestAsk) {
			return nil, errTeamOnly()
		}
		return e.write(ctx, from, "", Op{Kind: kind, Scope: p.Scope, Text: p.Text, Sensitive: p.Sensitive,
			Source: protocol.MemorySource{Kind: protocol.MemoryFromTeam, Team: from.Team, Task: from.Task}})
	case protocol.MMemoryForget:
		p, err := decode[protocol.MemoryForgetParams](params)
		if err != nil {
			return nil, err
		}
		return e.forget(ctx, caller{from: from, within: []string{own}}, p)
	}
	return nil, errTeamOnly()
}

// origin is who a caller's change comes from.
func origin(from core.MemoryCaller, session string) Origin {
	switch {
	case from.Team != "":
		return Origin{By: protocol.MemoryByTeam, Team: from.Team, Task: from.Task}
	case from.Session != "":
		return Origin{By: protocol.MemoryByTool, Session: from.Session}
	}
	return Origin{By: protocol.MemoryByYou, Session: session}
}

// idOps is one op of a kind per id.
func idOps(kind string, ids []string) []Op {
	ops := make([]Op, len(ids))
	for index, id := range ids {
		ops[index] = Op{Kind: kind, ID: id}
	}
	return ops
}

// apply makes ops one change in the built-in store and announces it.
func (e *Engine) apply(from core.MemoryCaller, session string, ops []Op) (protocol.MemoryChange, error) {
	st, err := e.store()
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	change, err := st.Apply(origin(from, session), ops)
	if err != nil {
		if refused, ok := errRefused(err); ok {
			e.emit(origin(from, session).Session, protocol.MemoryEventData{Kind: protocol.MemoryRefused, Reason: refused.Reason, Text: refused.Text})
			return change, protocol.Errorf(protocol.CodeBadRequest, "%s", refused.Detail)
		}
		return change, err
	}
	e.changed(change)
	return change, nil
}

// refuse turns text Rashin will not keep into a refusal in the caller's
// conversation and an error for the caller.
func (e *Engine) refuse(session, reason, text, detail string) error {
	e.emit(session, protocol.MemoryEventData{Kind: protocol.MemoryRefused, Reason: reason, Text: text})
	return protocol.Errorf(protocol.CodeBadRequest, "%s", detail)
}

// screen refuses text Rashin will not keep, whatever engine keeps it: a
// secret never, a sensitive topic only when the settings or the user allow
// it, an instruction never.
func (e *Engine) screen(from core.MemoryCaller, session, text string, sensitive bool) error {
	if redact.Strict(text) != text {
		return e.refuse(session, protocol.MemoryRefusedSecret, "", i18n.T("Rashin doesn't keep passwords, keys or tokens. Store it in the vault."))
	}
	if sensitive && !e.settings().Sensitive && !from.App {
		return e.refuse(session, protocol.MemoryRefusedSensitive, text, i18n.T("Sensitive topics are left out of memory."))
	}
	if th, hit := scan.Strict(text); hit {
		reported := text
		if th.Secret {
			reported = ""
		}
		return e.refuse(session, threatRefusal(th), reported, threatReason(th))
	}
	return nil
}

// write adds or suggests one memory for a caller after the refusals; a model
// may not bring back what the user forgot.
func (e *Engine) write(ctx context.Context, from core.MemoryCaller, session string, op Op) (protocol.MemoryChange, error) {
	o := origin(from, session)
	text := strings.TrimSpace(op.Text)
	if err := e.screen(from, o.Session, text, op.Sensitive); err != nil {
		return protocol.MemoryChange{}, err
	}
	if !validScope(op.Scope) {
		return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Memory can't keep anything in %s.", op.Scope))
	}
	st, err := e.store()
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	if !from.App && from.Session != "" || from.Team != "" {
		if buried, err := st.Tombstoned(ctx, text, op.Scope); err == nil && buried {
			return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The user asked Rashin to forget that. Only they can add it again."))
		}
	}
	op.Text = text
	if b, err := e.external(ctx); err != nil {
		return protocol.MemoryChange{}, err
	} else if b != nil && op.Kind == OpAdd {
		return e.remember(ctx, b, o, []Op{op})
	}
	return e.apply(from, session, []Op{op})
}

// externalChange is a change made in an external engine: shown like any
// other, but not kept for undo.
func externalChange(o Origin) protocol.MemoryChange {
	now := time.Now()
	return protocol.MemoryChange{ID: fmt.Sprintf("x%016x", now.UnixNano()), At: now, Origin: o.By, Session: o.Session, Team: o.Team, Task: o.Task, Model: o.Model}
}

// remember writes adds to an external engine and announces them.
func (e *Engine) remember(ctx context.Context, b backend.Backend, o Origin, ops []Op) (protocol.MemoryChange, error) {
	facts := make([]protocol.Memory, len(ops))
	now := time.Now()
	for index, op := range ops {
		source := op.Source
		if source.Kind == "" {
			source.Kind = sourceKind(o.By)
		}
		if source.Session == "" {
			source.Session = o.Session
		}
		facts[index] = protocol.Memory{Scope: op.Scope, Text: cleanText(op.Text), Status: protocol.MemoryActive, Kind: op.Retain,
			Pinned: op.Pinned, Sensitive: op.Sensitive, Source: source, Created: now, Updated: now}
	}
	kept, err := b.Remember(ctx, facts)
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	change := externalChange(o)
	for _, m := range kept {
		change.Items = append(change.Items, protocol.MemoryItem{ID: m.ID, Text: m.Text, Scope: m.Scope, ScopeName: e.scopeName(m.Scope), Op: protocol.MemoryOpRemembered})
	}
	e.changed(change)
	return change, nil
}

// add serves memory.add for the app, an agent session or the command line.
func (e *Engine) add(ctx context.Context, c caller, p protocol.MemoryAddParams) (protocol.MemoryChange, error) {
	session := p.Session
	if c.from.Session != "" {
		session = ""
	}
	if p.Scope == "" && c.from.Session != "" {
		// in a project a fact may be about the user or the project; guessing put project facts in "you"
		if project, _ := e.sessionScope(c.info); project != "" {
			return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Say whether this is about the user (scope you) or about this project (scope project)."))
		}
	}
	scope, err := e.callerScope(c, p.Scope)
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	op := Op{Kind: OpAdd, Scope: scope, Text: p.Text, Pinned: p.Pinned, Sensitive: p.Sensitive, Explicit: c.from.Session == ""}
	if c.from.Session != "" {
		op.Source = e.sessionSource(c.info)
	}
	return e.write(ctx, c.from, session, op)
}

// callerScope resolves the scope a caller names, within its reach: empty is
// you, "project" the caller's conversation's project.
func (e *Engine) callerScope(c caller, scope string) (string, error) {
	switch scope {
	case "":
		scope = protocol.MemoryYou
	case scopeTeam:
		return "", errTeamOnly()
	case scopeProject:
		if c.from.Session == "" {
			return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Name the project's scope."))
		}
		project, _ := e.sessionScope(c.info)
		if project == "" {
			return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("This conversation is not in a project."))
		}
		scope = project
	}
	if !c.reaches(scope) {
		return "", errOutOfReach()
	}
	return scope, nil
}

// sessionSource is where a memory an agent session adds comes from.
func (e *Engine) sessionSource(info protocol.SessionInfo) protocol.MemorySource {
	source := protocol.MemorySource{Kind: protocol.MemoryFromAgent, Session: info.ID, Title: info.Title}
	if info.Kind == protocol.KindAsk {
		source.Kind = protocol.MemoryFromAsk
	}
	return source
}

// lookup finds a memory in the engine in use.
func (e *Engine) lookup(ctx context.Context, b backend.Backend, id string) (protocol.Memory, bool) {
	if b != nil {
		m, ok := e.externalFacts(ctx, b, []string{id})[id]
		return m, ok
	}
	st, ok := e.storeIfAny()
	if !ok {
		return protocol.Memory{}, false
	}
	return st.Get(id)
}

// edit serves memory.edit: text and scope as one edit, pin on its own. The
// new text meets the same refusals as a new memory.
func (e *Engine) edit(ctx context.Context, c caller, p protocol.MemoryEditParams) (protocol.MemoryChange, error) {
	b, err := e.external(ctx)
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	if c.within != nil {
		if m, ok := e.lookup(ctx, b, p.ID); ok && !c.reaches(m.Scope) {
			return protocol.MemoryChange{}, errOutOfReach()
		}
	}
	text := ""
	if p.Text != nil {
		text = cleanText(*p.Text)
		if err := e.screen(c.from, c.from.Session, text, false); err != nil {
			return protocol.MemoryChange{}, err
		}
	}
	if b != nil {
		if p.Scope != nil || p.Pinned != nil || p.Text == nil || !b.Caps().Edit {
			return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.T("This engine can't change that."))
		}
		m, err := b.Edit(ctx, p.ID, text)
		if err != nil {
			return protocol.MemoryChange{}, err
		}
		change := externalChange(origin(c.from, ""))
		change.Items = []protocol.MemoryItem{{ID: m.ID, Text: m.Text, Scope: m.Scope, ScopeName: e.scopeName(m.Scope), Op: protocol.MemoryOpEdited}}
		e.changed(change)
		return change, nil
	}
	var ops []Op
	if p.Text != nil || p.Scope != nil {
		op := Op{Kind: OpEdit, ID: p.ID, Text: text}
		if p.Scope != nil {
			scope, err := e.callerScope(c, *p.Scope)
			if err != nil {
				return protocol.MemoryChange{}, err
			}
			op.Scope = scope
		}
		ops = append(ops, op)
	}
	if p.Pinned != nil {
		kind := OpUnpin
		if *p.Pinned {
			kind = OpPin
		}
		ops = append(ops, Op{Kind: kind, ID: p.ID})
	}
	return e.apply(c.from, "", ops)
}

// forget serves memory.forget; every forgotten fact leaves a tombstone.
// Memories out of the caller's reach are refused, ones already gone pass.
func (e *Engine) forget(ctx context.Context, c caller, p protocol.MemoryForgetParams) (protocol.MemoryChange, error) {
	session := p.Session
	if c.from.Session != "" {
		session = ""
	}
	b, err := e.external(ctx)
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	var found map[string]protocol.Memory
	if b != nil {
		found = e.externalFacts(ctx, b, p.IDs)
	}
	if c.within != nil {
		for _, id := range p.IDs {
			m, ok := found[id]
			if b == nil {
				m, ok = e.lookup(ctx, nil, id)
			}
			if ok && !c.reaches(m.Scope) {
				if c.from.Team != "" {
					return protocol.MemoryChange{}, errTeamOnly()
				}
				return protocol.MemoryChange{}, errOutOfReach()
			}
		}
	}
	if b == nil {
		return e.apply(c.from, session, idOps(OpForget, p.IDs))
	}
	st, err := e.store()
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	if err := b.Forget(ctx, p.IDs); err != nil {
		return protocol.MemoryChange{}, err
	}
	change := externalChange(origin(c.from, session))
	for _, id := range p.IDs {
		m, ok := found[id]
		if !ok {
			continue
		}
		if err := st.Bury(m.Text, m.Scope); err != nil {
			return change, err
		}
		change.Items = append(change.Items, protocol.MemoryItem{ID: id, Text: m.Text, Scope: m.Scope, ScopeName: e.scopeName(m.Scope), Op: protocol.MemoryOpForgot})
	}
	e.changed(change)
	return change, nil
}

// externalPage and externalMax bound the walk through an external engine's
// facts.
const (
	externalPage = 200
	externalMax  = 5000
)

// externalAll is every fact of an external engine in scope, up to
// externalMax.
func (e *Engine) externalAll(ctx context.Context, b backend.Backend, scope string) ([]protocol.Memory, error) {
	var out []protocol.Memory
	for offset := 0; offset < externalMax; offset += externalPage {
		page, total, err := b.List(ctx, scope, offset, externalPage)
		if err != nil {
			return out, err
		}
		out = append(out, page...)
		if offset+externalPage >= total || len(page) == 0 {
			break
		}
	}
	return out, nil
}

// externalFacts finds facts of an external engine by id.
func (e *Engine) externalFacts(ctx context.Context, b backend.Backend, ids []string) map[string]protocol.Memory {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	out := map[string]protocol.Memory{}
	all, _ := e.externalAll(ctx, b, "")
	for _, m := range all {
		if want[m.ID] {
			out[m.ID] = m
		}
	}
	return out
}

// list serves memory.list within the caller's reach. Suggestions and
// blocked lines are always Rashin's own; an external engine's facts are
// filtered, sorted and paged here when the engine cannot do it itself.
func (e *Engine) list(ctx context.Context, p protocol.MemoryListParams, within []string) (protocol.MemoryListResult, error) {
	empty := protocol.MemoryListResult{Memories: []protocol.Memory{}}
	own := p.Status == protocol.MemorySuggested || p.Status == protocol.MemoryBlocked
	if !own {
		if b, err := e.external(ctx); err != nil {
			return empty, err
		} else if b != nil {
			result, err := e.listExternal(ctx, b, p, within)
			if err != nil {
				return empty, err
			}
			for index := range result.Memories {
				result.Memories[index].ScopeName = e.scopeName(result.Memories[index].Scope)
			}
			return result, nil
		}
	}
	st, ok := e.storeIfAny()
	if !ok {
		return empty, nil
	}
	result := st.List(p, within...)
	e.titleRefs(result.Memories)
	return result, nil
}

// listExternal is one page of an external engine's facts.
func (e *Engine) listExternal(ctx context.Context, b backend.Backend, p protocol.MemoryListParams, within []string) (protocol.MemoryListResult, error) {
	plain := p.Session == "" && strings.TrimSpace(p.Query) == "" && (p.Sort == "" || p.Sort == protocol.MemorySortNewest) && within == nil
	if plain {
		limit := p.Limit
		if limit <= 0 || limit > listMax {
			limit = listMax
		}
		page, total, err := b.List(ctx, p.Scope, max(p.Offset, 0), limit)
		if page == nil {
			page = []protocol.Memory{}
		}
		return protocol.MemoryListResult{Memories: page, Total: total}, err
	}
	all, err := e.externalAll(ctx, b, p.Scope)
	if err != nil {
		return protocol.MemoryListResult{}, err
	}
	reachable := all[:0]
	for _, m := range all {
		if inScopes(m.Scope, within) {
			// the query also matches the scope's name, which an engine does not keep
			m.ScopeName = e.scopeName(m.Scope)
			reachable = append(reachable, m)
		}
	}
	return pageMemories(reachable, p), nil
}

// titleRefs fills the conversation a memory was last used in with its kind
// and title.
func (e *Engine) titleRefs(memories []protocol.Memory) {
	sessions := e.sessionsOf()
	if sessions == nil {
		return
	}
	for index := range memories {
		ref := memories[index].LastUsedIn
		if ref == nil {
			continue
		}
		if info, ok := sessions.Info(ref.Session); ok {
			ref.Kind, ref.Title = info.Kind, info.Title
		}
	}
}

// get serves memory.get from the engine in use; with an external engine the
// built-in store holds no facts to fall back on.
func (e *Engine) get(ctx context.Context, id string) (protocol.Memory, error) {
	b, err := e.external(ctx)
	if err != nil {
		return protocol.Memory{}, err
	}
	m, ok := e.lookup(ctx, b, id)
	if !ok && b != nil {
		// suggestions and blocked lines stay Rashin's own whatever the engine
		if st, open := e.storeIfAny(); open {
			if own, found := st.Get(id); found && own.Status != protocol.MemoryActive && own.Status != protocol.MemoryReplaced {
				m, ok = own, true
			}
		}
	}
	if !ok {
		return protocol.Memory{}, errNotFound(id)
	}
	if b != nil {
		m.ScopeName = e.scopeName(m.Scope)
	}
	memories := []protocol.Memory{m}
	e.titleRefs(memories)
	return memories[0], nil
}

// changes serves memory.changes; a caller sees only changes wholly within
// its reach.
func (e *Engine) changes(c caller, ids []string) (protocol.MemoryChangesResult, error) {
	out := protocol.MemoryChangesResult{Changes: []protocol.MemoryChange{}}
	st, ok := e.storeIfAny()
	if !ok {
		return out, nil
	}
	changes, err := st.Changes(ids)
	if err != nil {
		return out, err
	}
	for _, change := range changes {
		reachable := true
		for _, item := range change.Items {
			reachable = reachable && c.reaches(item.Scope)
		}
		if reachable {
			out.Changes = append(out.Changes, change)
		}
	}
	return out, nil
}

// keep serves memory.keep: suggestions become memories, in the external
// engine when one is in use.
func (e *Engine) keep(ctx context.Context, from core.MemoryCaller, ids []string) (protocol.MemoryChange, error) {
	b, err := e.external(ctx)
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	if b == nil {
		return e.apply(from, "", idOps(OpKeep, ids))
	}
	st, err := e.store()
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	var ops []Op
	for _, id := range ids {
		if m, ok := st.Get(id); ok && m.Status == protocol.MemorySuggested {
			ops = append(ops, Op{Kind: OpAdd, Scope: m.Scope, Text: m.Text, Retain: m.Kind, Sensitive: m.Sensitive, Source: m.Source})
		}
	}
	if len(ops) == 0 {
		return protocol.MemoryChange{}, nil
	}
	change, err := e.remember(ctx, b, origin(from, ""), ops)
	if err != nil {
		return change, err
	}
	return change, st.Drop(ids)
}

// flip serves undo and redo; the card follows at once.
func (e *Engine) flip(ctx context.Context, id string, undo bool) (protocol.MemoryChange, error) {
	st, err := e.store()
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	var change protocol.MemoryChange
	if undo {
		change, err = st.Undo(id)
	} else {
		change, err = st.Redo(id)
	}
	if err != nil {
		return change, err
	}
	e.touchCard(st, change, true)
	if e.deps.Broadcast != nil {
		e.deps.Broadcast(protocol.EvtMemoryChanged, protocol.MemoryChanged{Status: e.Status(ctx), Change: &change})
	}
	return change, nil
}

// forgetAll serves memory.forget_all, whatever Use says: the built-in store
// first, then the engine in use, whose error is returned. A local engine
// that is not running is not started for it; the forget waits in the ledger
// and runs before the engine is next used. It never creates the built-in
// store just to find it empty.
func (e *Engine) forgetAll(ctx context.Context, scope string) (protocol.MemoryChange, error) {
	if scope != "" && scope != protocol.MemoryProjects && scope != protocol.MemoryTeams && !validScope(scope) {
		return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Memory can't keep anything in %s.", scope))
	}
	change := protocol.MemoryChange{Items: []protocol.MemoryItem{}}
	if st, ok := e.storeIfAny(); ok {
		var err error
		if change, err = st.ForgetAll(scope); err != nil {
			return change, err
		}
		if inScope(protocol.MemoryYou, scope) {
			// the card went with the facts; rebuilding it would only make an empty one
			e.announce(change)
		} else {
			e.changed(change)
		}
	}
	b, err := e.openExternal(ctx)
	if err != nil || b == nil {
		return change, err
	}
	if b.Status(ctx).State != protocol.EngineStopped {
		return change, b.ForgetScope(ctx, scope)
	}
	st, err := e.store()
	if err != nil {
		return change, err
	}
	return change, st.putOffForget(scope)
}

// search serves memory.search within the caller's reach.
func (e *Engine) search(ctx context.Context, c caller, p protocol.MemorySearchParams) (protocol.MemorySearchResult, error) {
	scopes := c.within
	if p.Scope != "" {
		scope, err := e.callerScope(c, p.Scope)
		if err != nil {
			return protocol.MemorySearchResult{}, err
		}
		scopes = []string{scope}
	}
	return e.searchScopes(ctx, c.info, p.Query, scopes, p.Limit)
}

// readableScopes are what a conversation's memory reaches: you and its
// project, or only the project when the project keeps its memory.
func (e *Engine) readableScopes(info protocol.SessionInfo) []string {
	scope, kept := e.sessionScope(info)
	switch {
	case scope == "":
		return []string{protocol.MemoryYou}
	case kept:
		return []string{scope}
	}
	return []string{protocol.MemoryYou, scope}
}

// searchScopes finds memories in the scopes, without those the conversation
// set aside.
func (e *Engine) searchScopes(ctx context.Context, info protocol.SessionInfo, query string, scopes []string, limit int) (protocol.MemorySearchResult, error) {
	if limit <= 0 {
		limit = searchDefault
	}
	limit = min(limit, searchMax)
	out := protocol.MemorySearchResult{Memories: []protocol.Memory{}}
	suppressed := e.suppressed(info)
	var found []protocol.Memory
	if b, err := e.external(ctx); err != nil {
		return out, err
	} else if b != nil {
		found, err = b.Recall(ctx, backend.Query{Text: query, Scopes: scopes, Limit: limit + len(suppressed)})
		if err != nil {
			return out, err
		}
	} else {
		st, err := e.store()
		if err != nil {
			return out, err
		}
		found, err = st.Search(ctx, query, scopes, limit+len(suppressed))
		if err != nil {
			return out, err
		}
	}
	for _, m := range found {
		if suppressed[m.ID] || !inScopes(m.Scope, scopes) || len(out.Memories) >= limit {
			continue
		}
		if m.ScopeName == "" {
			m.ScopeName = e.scopeName(m.Scope)
		}
		out.Memories = append(out.Memories, m)
	}
	return out, nil
}

// move serves memory.engine_move: every active memory is copied into the
// chosen engine, which is then used. Moving out of the built-in store leaves
// no copy behind in it, so nothing is recalled twice or kept past a forget.
func (e *Engine) move(ctx context.Context, target string) (protocol.MemoryStatus, error) {
	s := e.settings()
	current := s.Engine
	if current == "" {
		current = protocol.MemoryBuiltin
	}
	if target == current {
		return e.Status(ctx), nil
	}
	if e.deps.SetEngine == nil || target != protocol.MemoryBuiltin && e.deps.Engines == nil {
		return protocol.MemoryStatus{}, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.T("This build has only the built-in engine."))
	}
	var active []protocol.Memory
	b, err := e.external(ctx)
	if err != nil {
		return protocol.MemoryStatus{}, err
	}
	st, open := e.storeIfAny()
	switch {
	case b != nil:
		if active, err = e.externalAll(ctx, b, ""); err != nil {
			return protocol.MemoryStatus{}, err
		}
	case open:
		active, _ = st.Active()
	}
	if target == protocol.MemoryBuiltin {
		if st, err = e.store(); err != nil {
			return protocol.MemoryStatus{}, err
		}
		// one add per memory, so a line the scan now refuses is left behind alone
		for _, m := range active {
			op := Op{Kind: OpAdd, Scope: m.Scope, Text: m.Text, Retain: m.Kind, Pinned: m.Pinned, Sensitive: m.Sensitive, Source: m.Source}
			if _, err := st.Apply(Origin{By: protocol.MemoryByImport}, []Op{op}); err != nil {
				if _, refused := errRefused(err); !refused {
					return protocol.MemoryStatus{}, err
				}
			}
		}
	} else if err := e.deps.Engines.Move(ctx, s, target, active); err != nil {
		return protocol.MemoryStatus{}, err
	}
	if err := e.deps.SetEngine(target); err != nil {
		return protocol.MemoryStatus{}, err
	}
	if current == protocol.MemoryBuiltin && open {
		ids := make([]string, len(active))
		for index, m := range active {
			ids[index] = m.ID
		}
		if err := st.Remove(ids); err != nil {
			return protocol.MemoryStatus{}, err
		}
	}
	status := e.Status(ctx)
	if e.deps.Broadcast != nil {
		e.deps.Broadcast(protocol.EvtMemoryChanged, protocol.MemoryChanged{Status: status})
	}
	return status, nil
}
