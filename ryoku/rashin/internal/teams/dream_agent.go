package teams

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	i18n "ryoku-i18n"

	registry "ryoku-rashin/internal/harness"
	"ryoku-rashin/internal/protocol"
)

// managerCall finds the dream a call from an agent's MCP server is for and
// refuses one that does not come from that dream's manager: a member of a
// dream, or any agent of a team, may not hire, release, say where a dream
// stands or present it.
func (s *Service) managerCall(ctx context.Context, ref protocol.AgentRef) (caller, protocol.Team, error) {
	c, err := s.caller(ctx, ref)
	if err != nil {
		return caller{}, protocol.Team{}, err
	}
	team, ok := s.state.team(c.Team)
	if !ok {
		return caller{}, protocol.Team{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", c.Team))
	}
	if !isDream(team) || team.Dream.Manager != c.Member {
		return caller{}, protocol.Team{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only a dream's manager can do this."))
	}
	if team.Dream.Done != nil {
		return caller{}, protocol.Team{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This dream is done."))
	}
	return c, team, nil
}

// agentHire lets a dream's manager add a member, on its own engine or by
// default the manager's. The engine must be signed in for this dream: a
// hire on one that is not is refused with the harnesses that are.
func (s *Service) agentHire(ctx context.Context, p protocol.AgentHireParams) (protocol.Member, error) {
	c, team, err := s.managerCall(ctx, p.AgentRef)
	if err != nil {
		return protocol.Member{}, err
	}
	manager, _ := taskMember(team, c.Member)
	hire := protocol.Member{
		Name: strings.TrimSpace(p.Name), Title: strings.TrimSpace(p.Title), Instructions: strings.TrimSpace(p.Instructions),
		Harness: strings.TrimSpace(p.Harness), Model: strings.TrimSpace(p.Model),
	}
	if hire.Name == "" || hire.Instructions == "" {
		return protocol.Member{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A hire needs a name and a short paragraph on the responsibility it owns."))
	}
	if hire.Harness == "" {
		hire.Harness, hire.Options = manager.Harness, manager.Options
		if hire.Model == "" {
			hire.Model = manager.Model
		}
	}
	settings := s.settings()
	harness, _, _ := protocol.ResolveEngine(hire, settings, s.defaultHarness(settings))
	if harness == "" {
		return protocol.Member{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("No harness is installed to run a hire."))
	}
	if !registry.Known(harness, settings) {
		return protocol.Member{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("No harness %s on this computer. %s", harness, s.signedInWords(team)))
	}
	if _, signed := s.signedIn(harness, team.ID); !signed {
		return protocol.Member{}, protocol.Errorf(protocol.CodeConflict, "%s",
			i18n.Tf("This dream is not signed in to %s, so a hire cannot work on it. %s", registry.Name(harness, settings), s.signedInWords(team)))
	}
	s.mu.Lock()
	current, ok := s.state.team(team.ID)
	if !ok {
		s.mu.Unlock()
		return protocol.Member{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", team.ID))
	}
	next := current
	next.Members = append(slices.Clone(current.Members), hire)
	kept := make(map[string]bool, len(current.Members))
	for _, m := range current.Members {
		kept[m.Harness] = true
	}
	if _, err := normaliseTeam(&next, false, settings, kept); err != nil {
		s.mu.Unlock()
		return protocol.Member{}, err
	}
	dream := cloneDream(*next.Dream)
	dream.Hired++
	next.Dream, next.Updated = &dream, s.now()
	err = s.state.putTeam(next)
	s.mu.Unlock()
	if err != nil {
		return protocol.Member{}, err
	}
	hired := next.Members[len(next.Members)-1]
	s.resetIdle(c.Task)
	detail := i18n.Tf("Hired %s, %s, on %s.", hired.Name, hired.Title, s.engineWords(harness, hired.Model))
	if why := strings.TrimSpace(p.Why); why != "" {
		detail += " " + why
	}
	s.record(protocol.Activity{Actor: manager.Name, Action: "dream.hire", Target: hired.Name, OK: true, Team: team.ID, Task: c.Task, Initiator: protocol.InitiatorAgent, Detail: s.redact(detail)})
	s.emitTeam(team.ID)
	return hired, nil
}

// signedInWords names the harnesses this dream is signed in to, for a hire
// that cannot go ahead.
func (s *Service) signedInWords(team protocol.Team) string {
	settings := s.settings()
	candidates := []string{settings.Teams.Defaults.Harness, settings.Agent.Harness, s.defaultHarness(settings)}
	for _, m := range team.Members {
		id, _, _ := protocol.ResolveEngine(m, settings, s.defaultHarness(settings))
		candidates = append(candidates, id)
	}
	for _, instance := range settings.Agent.Instances {
		candidates = append(candidates, strings.TrimSpace(instance.ID))
	}
	var names []string
	seen := map[string]bool{}
	for _, id := range candidates {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if _, ok := s.signedIn(id, team.ID); ok {
			names = append(names, registry.Name(id, settings))
		}
	}
	if len(names) == 0 {
		return i18n.T("It is signed in to no harness besides yours; hire on your own harness, or ask the user to sign the dream in to another.")
	}
	return i18n.Tf("It is signed in to: %s.", strings.Join(names, ", "))
}

// agentRelease lets a dream's manager let a member go once its work is in;
// one with a task still open stays.
func (s *Service) agentRelease(ctx context.Context, p protocol.AgentReleaseParams) (protocol.AgentAnswer, error) {
	c, team, err := s.managerCall(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	member, ok := taskMember(team, p.Member)
	if !ok || member.ID == team.Dream.Manager {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No member %s to release.", p.Member))
	}
	s.mu.Lock()
	open := s.state.tasks(func(t protocol.TeamTask) bool { return t.Team == team.ID && t.Member == member.ID && taskIsOpen(t) })
	if len(open) > 0 {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("%s still works on \"%s\". Release them once it is in.", member.Name, open[0].Title))
	}
	current, _ := s.state.team(team.ID)
	current.Members = slices.DeleteFunc(slices.Clone(current.Members), func(m protocol.Member) bool { return m.ID == member.ID })
	current.Updated = s.now()
	err = s.state.putTeam(current)
	s.mu.Unlock()
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	manager, _ := taskMember(team, c.Member)
	detail := i18n.Tf("Released %s.", member.Name)
	if why := strings.TrimSpace(p.Why); why != "" {
		detail += " " + why
	}
	s.resetIdle(c.Task)
	s.record(protocol.Activity{Actor: manager.Name, Action: "dream.release", Target: member.Name, OK: true, Team: team.ID, Task: c.Task, Initiator: protocol.InitiatorAgent, Detail: s.redact(detail)})
	s.emitTeam(team.ID)
	return protocol.AgentAnswer{Text: i18n.Tf("%s has left the team.", member.Name)}, nil
}

// agentDream is the manager saying where the dream stands: its phase, a
// one-sentence summary, its own estimate of progress, and what the
// interview settled. Presenting and done are reached only by presenting
// and by the user's accepting.
func (s *Service) agentDream(ctx context.Context, p protocol.AgentDreamParams) (protocol.AgentAnswer, error) {
	c, team, err := s.managerCall(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	phase := strings.TrimSpace(p.Phase)
	if !slices.Contains(protocol.DreamPhases, phase) || phase == protocol.DreamPresenting || phase == protocol.DreamDone {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s",
			i18n.T("The phase is one of interviewing, designing, planning, building or testing; present the dream to reach presenting."))
	}
	summary := strings.TrimSpace(p.Summary)
	if summary == "" || utf8.RuneCountInString(summary) > dreamSummaryMax {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Say in one sentence of at most %d characters where the dream stands.", dreamSummaryMax))
	}
	if p.Progress != nil && (*p.Progress < 0 || *p.Progress > 100) {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Progress is a number from 0 to 100."))
	}
	brief := strings.TrimSpace(p.Brief)
	if utf8.RuneCountInString(brief) > dreamBriefMax {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The brief has at most %d characters.", dreamBriefMax))
	}
	var before string
	_, err = s.changeDream(team.ID, func(_ *protocol.Team, d *protocol.Dream) error {
		if d.Phase == protocol.DreamPresenting {
			return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The dream waits on the user's answer to your presentation."))
		}
		before = d.Phase
		d.Phase, d.Summary = phase, s.redact(summary)
		if p.Progress != nil {
			progress := *p.Progress
			d.Progress = &progress
		}
		if brief != "" {
			d.Brief = s.redact(brief)
		}
		return nil
	})
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	s.resetIdle(c.Task)
	if before != phase {
		manager, _ := taskMember(team, c.Member)
		s.record(protocol.Activity{Actor: manager.Name, Action: "dream.phase", Target: team.Name, OK: true, Team: team.ID, Task: c.Task, Initiator: protocol.InitiatorAgent,
			Detail: s.redact(i18n.Tf("Now %s: %s", phaseWord(phase), summary))})
	}
	return protocol.AgentAnswer{Text: i18n.Tf("Noted: the dream is %s. Take the next step now.", phaseWord(phase))}, nil
}

// agentPresent is the manager handing the built and tested dream to the
// user: it waits in presenting, as a card in the Inbox, until the user
// accepts it or sends notes. A dream is presented only after the user
// answered a question of the manager's and a hired member finished a piece
// in this round (readyToPresent).
func (s *Service) agentPresent(ctx context.Context, p protocol.AgentPresentParams) (protocol.AgentAnswer, error) {
	c, team, err := s.managerCall(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	title, body := strings.TrimSpace(p.Title), strings.TrimSpace(p.Body)
	if title == "" || body == "" {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A presentation needs a title and a body: what was built, how to use it, and the evidence it works."))
	}
	if utf8.RuneCountInString(body) > dreamPresentationMax {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A presentation's body has at most %d characters.", dreamPresentationMax))
	}
	if err := s.readyToPresent(team); err != nil {
		return protocol.AgentAnswer{}, err
	}
	presented := &protocol.DreamPresentation{
		Title: s.redact(title), Body: s.redact(body), Tests: s.redact(strings.TrimSpace(p.Tests)), Files: teamFiles(team.Folder, p.Files), At: s.now(),
	}
	team, err = s.changeDream(team.ID, func(_ *protocol.Team, d *protocol.Dream) error {
		if d.Phase == protocol.DreamPresenting {
			return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The user has not answered your last presentation yet."))
		}
		d.Phase, d.Presentation = protocol.DreamPresenting, presented
		d.Rounds++
		return nil
	})
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	manager, _ := taskMember(team, c.Member)
	s.record(protocol.Activity{Actor: manager.Name, Action: "dream.present", Target: presented.Title, OK: true, Team: team.ID, Task: c.Task, Initiator: protocol.InitiatorAgent,
		Detail: i18n.Tf("Presentation %d of %s.", team.Dream.Rounds, team.Name)})
	detail := presented.Body
	if presented.Tests != "" {
		detail += "\n\n" + i18n.Tf("Tests: %s", presented.Tests)
	}
	asked := protocol.Approval{
		Team: team.ID, Kind: protocol.ApprovalAction, Title: i18n.Tf("%s is ready: %s", team.Name, presented.Title), Detail: detail,
		Options: []string{i18n.T("Accept"), i18n.T("Send notes")},
	}
	if _, err := s.ask(ctx, c, asked, dreamCard{Dream: cardPresent}); err != nil {
		return protocol.AgentAnswer{}, err
	}
	s.resetIdle(c.Task)
	return protocol.AgentAnswer{Text: i18n.T("Your presentation is with the user. End your turn now: Rashin continues you with their notes, or ends the dream when they accept it.")}, nil
}

// readyToPresent refuses a presentation the dream's process has not earned:
// the user must have answered at least one of the manager's questions in
// this dream, and a member the manager hired must have finished a piece it
// was handed since the last presentation. The refusal says what to do next.
func (s *Service) readyToPresent(team protocol.Team) error {
	machine := []string{i18n.T("The task ended."), i18n.T("The task was stopped.")}
	asked := s.state.approvals(func(r approvalRecord) bool {
		answered := r.Status == protocol.ApprovalApproved || r.Status == protocol.ApprovalDenied && !slices.Contains(machine, r.Note)
		return r.Team == team.ID && r.Kind == protocol.ApprovalAction && r.Member == team.Dream.Manager && cardOf(r) == "" && answered
	})
	if len(asked) == 0 {
		return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Not yet: the CEO has not answered any question of yours. Ask them one round of design or task questions with ask_user, including your plan, and end your turn; present after they answer."))
	}
	since := team.Dream.Started
	if prior := team.Dream.Presentation; prior != nil {
		since = prior.At
	}
	pieces := s.state.tasks(func(t protocol.TeamTask) bool {
		return t.Team == team.ID && t.Parent != "" && t.Member != team.Dream.Manager && t.Status == protocol.TaskDone && t.Ended != nil && t.Ended.After(since)
	})
	if len(pieces) == 0 {
		return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Not yet: no member of your team finished a piece of this round. Hire a member, hand it the work or the review and tests with delegate, and end your turn; present once its report is in."))
	}
	return nil
}

// phaseWord is a dream phase as a sentence says it.
func phaseWord(phase string) string {
	switch phase {
	case protocol.DreamInterviewing:
		return i18n.T("interviewing")
	case protocol.DreamDesigning:
		return i18n.T("designing")
	case protocol.DreamPlanning:
		return i18n.T("planning")
	case protocol.DreamBuilding:
		return i18n.T("building")
	case protocol.DreamTesting:
		return i18n.T("testing")
	case protocol.DreamPresenting:
		return i18n.T("presenting")
	}
	return i18n.T("done")
}
