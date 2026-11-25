package teams

import (
	"context"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// managerCall finds the dream a call from a session's team tools is for and
// refuses one that does not come from that dream's manager: a member of a
// dream, or any agent of a team, may not hire, release, say where a dream
// stands or present it.
func (s *Service) managerCall(ctx context.Context, ref protocol.AgentRef) (caller, protocol.Team, error) {
	c, err := s.caller(ctx, ref)
	if err != nil {
		return caller{}, protocol.Team{}, err
	}
	s.mu.Lock()
	team, _, _, err := s.currentDreamManagerLocked(c, false)
	s.mu.Unlock()
	return c, team, err
}

// currentDreamManagerLocked keeps Dream administration bound to the live root
// execution, not a role/rank or the snapshot taken before a model lookup.
// Presentation is communication; only administrative changes require write trust.
func (s *Service) currentDreamManagerLocked(c caller, write bool) (protocol.Team, protocol.Member, protocol.TeamTask, error) {
	team, member, task, err := s.currentCallerLocked(c)
	if err != nil {
		return protocol.Team{}, protocol.Member{}, protocol.TeamTask{}, err
	}
	if !isDream(team) || team.Dream.Manager != c.Member || team.Reporting != protocol.ReportingTree || teamLead(team).ID != c.Member {
		return protocol.Team{}, protocol.Member{}, protocol.TeamTask{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only a dream's manager can do this."))
	}
	if err := protocol.ValidateReporting(team); err != nil {
		return protocol.Team{}, protocol.Member{}, protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "The dream's reporting tree is no longer valid: %v", err)
	}
	if team.Dream.Done != nil {
		return protocol.Team{}, protocol.Member{}, protocol.TeamTask{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("This dream is done."))
	}
	if write && taskApprovals(team, member, task) == protocol.ApprovalsReadOnly {
		return protocol.Team{}, protocol.Member{}, protocol.TeamTask{}, protocol.Errorf(protocol.CodeForbidden, "Read-only or unapproved strategy work cannot change Dream administration.")
	}
	return team, member, task, nil
}

// agentHire lets a dream's manager add a member, on a model of its own or by
// default the manager's model and options. A model Rashin cannot run now is
// refused with what is wrong, so the manager can hire on another.
func (s *Service) agentHire(ctx context.Context, p protocol.AgentHireParams) (protocol.Member, error) {
	c, team, err := s.managerCall(ctx, p.AgentRef)
	if err != nil {
		return protocol.Member{}, err
	}
	s.mu.Lock()
	_, manager, _, err := s.currentDreamManagerLocked(c, true)
	s.mu.Unlock()
	if err != nil {
		return protocol.Member{}, err
	}
	hire := protocol.Member{
		Name: strings.TrimSpace(p.Name), Title: strings.TrimSpace(p.Title), Instructions: strings.TrimSpace(p.Instructions),
		Model: strings.TrimSpace(p.Model),
	}
	if hire.Name == "" || hire.Instructions == "" {
		return protocol.Member{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A hire needs a name and a short paragraph on the responsibility it owns."))
	}
	if hire.Model == "" {
		hire.Model, hire.Options = manager.Model, maps.Clone(manager.Options)
	}
	settings := s.settings()
	model, _ := protocol.ResolveEngine(hire, settings)
	problem, tier := s.modelStatus(ctx, model)
	// Paid-model consent is checked against the current pins at commit below.
	if problem != "" {
		return protocol.Member{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A hire cannot work on %s. %s Hire on another model, or leave the model empty to take yours.", s.engineWords(model), problem))
	}
	s.mu.Lock()
	current, manager, task, err := s.currentDreamManagerLocked(c, true)
	if err != nil {
		s.mu.Unlock()
		return protocol.Member{}, err
	}
	settings = s.settings()
	if strings.TrimSpace(p.Model) == "" {
		hire.Model, hire.Options = manager.Model, maps.Clone(manager.Options)
	}
	currentModel, _ := protocol.ResolveEngine(hire, settings)
	if currentModel != model {
		s.mu.Unlock()
		return protocol.Member{}, protocol.Errorf(protocol.CodeConflict, "The hire's model changed while it was checked. Retry with the current configuration.")
	}
	if tier == protocol.TierPaid &&
		model != manager.Model && model != settings.Teams.Defaults.Model && model != settings.Agent.Model {
		s.mu.Unlock()
		return protocol.Member{}, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the user can pin a paid model for a dream hire. Choose the manager's model or a model the user set in Settings."))
	}
	// New seats use the native runtime. Neither a foreign command nor its
	// environment/credential references are implicit grants to a new worker.
	hire.ReportsTo, hire.Trust = manager.ID, taskApprovals(current, manager, task)
	if manager.Tools != nil {
		tools := *manager.Tools
		hire.Tools = &tools
	}
	if manager.Skills != nil {
		skills := slices.Clone(*manager.Skills)
		hire.Skills = &skills
	}
	next := current
	next.Members = append(slices.Clone(current.Members), hire)
	if _, err := normaliseTeam(&next, false, settings); err != nil {
		s.mu.Unlock()
		return protocol.Member{}, err
	}
	if err := AdvanceMemberRevisions(current.Members, next.Members); err != nil {
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
	detail := i18n.Tf("Hired %s, %s, on %s.", hired.Name, hired.Title, s.engineWords(model))
	if why := strings.TrimSpace(p.Why); why != "" {
		detail += " " + why
	}
	s.record(protocol.Activity{Actor: manager.Name, Action: "dream.hire", Target: hired.Name, OK: true, Team: team.ID, Task: c.Task, Initiator: protocol.InitiatorAgent, Detail: s.redact(detail)})
	s.emitTeam(team.ID)
	return hired, nil
}

// agentRelease lets a dream's manager let a member go once its work is in;
// one with a task still open stays.
func (s *Service) agentRelease(ctx context.Context, p protocol.AgentReleaseParams) (protocol.AgentAnswer, error) {
	c, _, err := s.managerCall(ctx, p.AgentRef)
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
	s.mu.Lock()
	team, manager, _, err := s.currentDreamManagerLocked(c, true)
	if err != nil {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, err
	}
	member, ok := taskMember(team, p.Member)
	if !ok || member.ID == team.Dream.Manager {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No member %s to release.", p.Member))
	}
	if slices.ContainsFunc(team.Members, func(report protocol.Member) bool { return report.ReportsTo == member.ID }) {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeConflict, "Reassign this member's reports before releasing it.")
	}
	open := s.state.tasks(func(t protocol.TeamTask) bool { return t.Team == team.ID && t.Member == member.ID && taskIsOpen(t) })
	if len(open) > 0 {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("%s still works on \"%s\". Release them once it is in.", member.Name, open[0].Title))
	}
	current := team
	current.Members = slices.DeleteFunc(slices.Clone(team.Members), func(m protocol.Member) bool { return m.ID == member.ID })
	if _, err := normaliseTeam(&current, false, s.settings()); err != nil {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, err
	}
	if err := AdvanceMemberRevisions(team.Members, current.Members); err != nil {
		s.mu.Unlock()
		return protocol.AgentAnswer{}, err
	}
	current.Updated = s.now()
	err = s.state.putTeam(current)
	s.mu.Unlock()
	if err != nil {
		return protocol.AgentAnswer{}, err
	}
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
// one-sentence summary and what the interview settled. Presenting and done
// are reached only by presenting and by the user's accepting.
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
	brief := strings.TrimSpace(p.Brief)
	if utf8.RuneCountInString(brief) > dreamBriefMax {
		return protocol.AgentAnswer{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The brief has at most %d characters.", dreamBriefMax))
	}
	var before string
	_, err = s.changeDream(team.ID, func(_ *protocol.Team, d *protocol.Dream) error {
		if _, _, _, err := s.currentDreamManagerLocked(c, true); err != nil {
			return err
		}
		if d.Phase == protocol.DreamPresenting {
			return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The dream waits on the user's answer to your presentation."))
		}
		before = d.Phase
		d.Phase, d.Summary = phase, s.redact(summary)
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
	presented := &protocol.DreamPresentation{
		Title: s.redact(title), Body: s.redact(body), Tests: s.redact(strings.TrimSpace(p.Tests)), At: s.now(),
		Sources: slices.Clone(p.Sources),
	}
	team, err = s.changeDream(team.ID, func(current *protocol.Team, d *protocol.Dream) error {
		if _, _, _, err := s.currentDreamManagerLocked(c, false); err != nil {
			return err
		}
		if err := s.readyToPresent(*current); err != nil {
			return err
		}
		presented.Files = s.teamFiles(current.Folder, p.Files)
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
	return protocol.AgentAnswer{Text: i18n.T("Your presentation is with the user. End your turn now: Hyakki continues you with their notes, or ends the dream when they accept it.")}, nil
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
