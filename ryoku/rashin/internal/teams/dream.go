package teams

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Dream limits.
const (
	// dreamStatementMax bounds what the user wants, in their words.
	dreamStatementMax = 8000
	// dreamBriefMax bounds what the interview settled, as the manager keeps it.
	dreamBriefMax = 8000
	// dreamSummaryMax bounds the manager's account of where the dream stands.
	dreamSummaryMax = 400
	// dreamPresentationMax bounds a presentation's body.
	dreamPresentationMax = 20000
)

// dreamCard marks an approval a dream raised, so its answer is carried out
// as the dream's: a presentation, the stall guard's question, or the
// manager's blocker put to the user. It is the approval's stored action.
type dreamCard struct {
	Dream string `json:"dream"`
}

// Kinds of dreamCard.
const (
	cardPresent = "present"
	cardStall   = "stall"
	cardBlocked = "blocked"
)

// isDream reports a team that is a dream.
func isDream(team protocol.Team) bool { return team.Dream != nil }

// withoutDreams leaves the dreams out of a list of teams: teams.list, the
// teams agents read and the CLI's list, never shows them.
func withoutDreams(result protocol.TeamsResult) protocol.TeamsResult {
	result.Teams = slices.DeleteFunc(result.Teams, isDream)
	return result
}

// dreamCreate starts a dream: a team of one manager, working in its own
// folder under the Dreams home without asking for confirmation, with no
// token cap unless one is given, and the manager's standing task, which
// starts at once. A harness the dream is not signed in to blocks that task
// as any team's would; resuming the dream starts it again.
func (s *Service) dreamCreate(ctx context.Context, p protocol.DreamCreateParams) (protocol.Team, error) {
	statement := strings.TrimSpace(p.Statement)
	if statement == "" {
		return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A dream needs a statement: what you want made."))
	}
	if utf8.RuneCountInString(statement) > dreamStatementMax {
		return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A dream's statement has at most %d characters.", dreamStatementMax))
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = clipRunes(protocol.BriefTitle(statement), maxNameRunes)
	}
	folder := strings.TrimSpace(p.Folder)
	if folder == "" {
		folder = filepath.Join(paths.DreamsHome(), slugName(name))
	}
	settings := s.settings()
	defaults := settings.Teams.Defaults
	now := s.now()
	team := newTeam(protocol.TeamCreateParams{
		Name: name, Purpose: statement, Folder: folder,
		Members: []protocol.Member{{
			Name: i18n.T("Manager"), Title: i18n.T("Runs the dream"), Lead: true,
			Harness: strings.TrimSpace(p.Harness), Model: strings.TrimSpace(p.Model), Options: p.Options,
		}},
		Approvals: protocol.ApprovalsAuto,
		Budget: protocol.TeamBudget{
			MonthTokens: p.MonthTokens, WarnPercent: defaults.Budget.WarnPercent, TaskTurns: protocol.TeamTaskTurnsMax,
		},
		Running: protocol.TeamsPerTeamMax,
	}, now)
	team.Dream = &protocol.Dream{Statement: statement, Phase: protocol.DreamInterviewing, Started: now}
	created, err := s.addTeam(ctx, team, nil, "dream.start", func(t protocol.Team) string {
		return i18n.Tf("Started the dream, working in %s.", paths.Short(t.Folder))
	})
	if err != nil {
		return protocol.Team{}, err
	}
	manager := teamLead(created)
	if _, err := s.changeDream(created.ID, func(_ *protocol.Team, d *protocol.Dream) error {
		d.Manager = manager.ID
		return nil
	}); err != nil {
		return protocol.Team{}, err
	}
	if _, err := s.standingTask(ctx, created.ID); err != nil {
		return protocol.Team{}, err
	}
	started, _ := s.teamWithStats(created.ID)
	return started, nil
}

// standingTask gives a dream's manager the task it works the dream in, as
// its goal, and keeps it as the dream's. A dream already worked in an open
// standing task keeps that one, and one whose standing task ended short, a
// sign-in it waited on or a stop, runs that task again in a new
// conversation rather than adding one.
func (s *Service) standingTask(ctx context.Context, teamID string) (protocol.TeamTask, error) {
	team, ok := s.state.team(teamID)
	if !ok || !isDream(team) {
		return protocol.TeamTask{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No dream %s.", teamID))
	}
	if open, ok := s.openStanding(team); ok {
		return open, nil
	}
	if again, ok := s.reopenStanding(team); ok {
		return again, nil
	}
	task, err := s.newTask(ctx, team.ID, team.Dream.Manager, i18n.Tf("Dream: %s", team.Name), team.Dream.Statement, protocol.TaskFromYou, "", "", "", nil)
	if err != nil {
		return protocol.TeamTask{}, err
	}
	if _, err := s.changeDream(team.ID, func(_ *protocol.Team, d *protocol.Dream) error {
		d.Task = task.ID
		return nil
	}); err != nil {
		return protocol.TeamTask{}, err
	}
	return task, nil
}

// reopenStanding runs the dream's standing task again when it ended
// blocked or stopped, so a dream keeps one standing task and never counts
// an attempt that did not run as a finished one.
func (s *Service) reopenStanding(team protocol.Team) (protocol.TeamTask, bool) {
	s.mu.Lock()
	t, ok := s.state.task(team.Dream.Task)
	if !ok || !standing(team, t) || (t.Status != protocol.TaskBlocked && t.Status != protocol.TaskStopped) {
		s.mu.Unlock()
		return protocol.TeamTask{}, false
	}
	runAgain(&t, s.now())
	err := s.state.putTask(t)
	s.mu.Unlock()
	if err != nil {
		return protocol.TeamTask{}, false
	}
	s.emitTask(t)
	s.pump()
	if current, ok := s.state.task(t.ID); ok {
		t = current
	}
	return t, true
}

// HomeSignedIn goes on with a dream whose standing task was blocked only
// because the dream was not signed in to harness: the task runs again by
// itself, so the user need not resume it.
func (s *Service) HomeSignedIn(harness, teamID string) {
	team, ok := s.state.team(teamID)
	if !ok || !isDream(team) {
		return
	}
	if t, ok := s.state.task(team.Dream.Task); ok && t.Status == protocol.TaskBlocked && t.SignIn == harness {
		s.runDream(teamID, protocol.InitiatorRashin)
	}
}

// standing reports a task a dream's manager works the dream in: a task of
// the manager that is no piece of another.
func standing(team protocol.Team, t protocol.TeamTask) bool {
	return isDream(team) && t.Team == team.ID && t.Member == team.Dream.Manager && t.Parent == ""
}

// openStanding is the dream's standing task while it is open.
func (s *Service) openStanding(team protocol.Team) (protocol.TeamTask, bool) {
	open := s.state.tasks(func(t protocol.TeamTask) bool { return standing(team, t) && taskIsOpen(t) })
	if len(open) == 0 {
		return protocol.TeamTask{}, false
	}
	return open[0], true
}

// runDream starts a dream's manager again when no standing task is open,
// unless the dream is done or paused; initiator says who asked: the user
// resuming it or lifting its spending limit, or Rashin once its sign-in
// finished. A presentation whose card a pause took away is withdrawn: the
// dream goes back to testing and the manager presents again.
func (s *Service) runDream(teamID, initiator string) {
	team, ok := s.state.team(teamID)
	if !ok || !isDream(team) || team.Paused || team.Dream.Done != nil {
		return
	}
	if _, open := s.openStanding(team); open {
		return
	}
	if team.Dream.Phase == protocol.DreamPresenting {
		if _, err := s.changeDream(teamID, func(_ *protocol.Team, d *protocol.Dream) error {
			d.Phase = protocol.DreamTesting
			return nil
		}); err != nil {
			return
		}
	}
	if _, err := s.standingTask(s.background, teamID); err == nil {
		actor := "Rashin"
		if initiator == protocol.InitiatorYou {
			actor = protocol.InitiatorYou
		}
		s.record(protocol.Activity{
			Actor: actor, Action: "dream.resume", Target: team.Name, OK: true, Team: team.ID, Initiator: initiator,
			Detail: i18n.T("The manager carries on in a new conversation."),
		})
	}
}

// changeDream changes a dream under the lock on a copy of its record, keeps
// it and tells clients. A change that fails changes nothing.
func (s *Service) changeDream(teamID string, change func(*protocol.Team, *protocol.Dream) error) (protocol.Team, error) {
	s.mu.Lock()
	team, ok := s.state.team(teamID)
	if !ok || !isDream(team) {
		s.mu.Unlock()
		return protocol.Team{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No dream %s.", teamID))
	}
	dream := cloneDream(*team.Dream)
	if err := change(&team, &dream); err != nil {
		s.mu.Unlock()
		return protocol.Team{}, err
	}
	team.Dream, team.Updated = &dream, s.now()
	err := s.state.putTeam(team)
	s.mu.Unlock()
	if err != nil {
		return protocol.Team{}, err
	}
	s.emitTeam(team.ID)
	return team, nil
}

// cloneDream copies what a change could alter through a shared pointer or
// slice.
func cloneDream(d protocol.Dream) protocol.Dream {
	if d.Presentation != nil {
		presentation := *d.Presentation
		presentation.Files = slices.Clone(presentation.Files)
		d.Presentation = &presentation
	}
	if d.Done != nil {
		done := *d.Done
		d.Done = &done
	}
	return d
}

// dreamAnswer takes the user's verdict on a presentation through its Inbox
// card, so the card and the dream page answer the same way: accepted ends
// the dream, notes send its team back to work.
func (s *Service) dreamAnswer(ctx context.Context, p protocol.DreamAnswerParams) (protocol.Team, error) {
	team, ok := s.state.team(p.Team)
	if !ok || !isDream(team) {
		return protocol.Team{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No dream %s.", p.Team))
	}
	note := strings.TrimSpace(p.Note)
	choice := 0
	switch p.Verdict {
	case protocol.DreamAccepted:
	case protocol.DreamNotes:
		if note == "" {
			return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Write the notes the team should work in."))
		}
		choice = 1
	default:
		return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not an answer to a presentation.", p.Verdict))
	}
	cards := s.state.approvals(func(r approvalRecord) bool {
		return r.Team == team.ID && r.Status == protocol.ApprovalPending && cardOf(r) == cardPresent
	})
	if len(cards) == 0 {
		return protocol.Team{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("%s has nothing presented to answer.", team.Name))
	}
	card := cards[0]
	if _, err := s.decideApproval(ctx, protocol.ApprovalDecideParams{ID: card.ID, Decision: protocol.DecideChoose, Choice: card.Options[choice], Note: note}); err != nil {
		return protocol.Team{}, err
	}
	answered, _ := s.teamWithStats(team.ID)
	return answered, nil
}

// acceptDream ends a dream as the user accepted it: the presentation is the
// standing task's report, the dream is done, and whatever its team still
// worked on stops.
func (s *Service) acceptDream(teamID, note string) error {
	now := s.now()
	team, err := s.changeDream(teamID, func(_ *protocol.Team, d *protocol.Dream) error {
		if d.Presentation == nil || d.Done != nil {
			return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("There is nothing presented to accept."))
		}
		d.Presentation.Verdict, d.Presentation.Note = protocol.DreamAccepted, note
		d.Phase, d.Done = protocol.DreamDone, &now
		return nil
	})
	if err != nil {
		return err
	}
	presentation := team.Dream.Presentation
	detail := i18n.Tf("You accepted %s.", presentation.Title)
	if note != "" {
		detail = i18n.Tf("You accepted %s: %s", presentation.Title, note)
	}
	s.record(protocol.Activity{Actor: protocol.InitiatorYou, Action: "dream.accept", Target: team.Name, OK: true, Team: team.ID, Task: team.Dream.Task, Initiator: protocol.InitiatorYou, Detail: s.redact(detail)})
	if task, ok := s.openStanding(team); ok {
		report := &protocol.TeamReport{Title: presentation.Title, Body: presentation.Body, Files: presentation.Files, At: now}
		if ended, ok := s.endTask(task.ID, func(t *protocol.TeamTask) {
			t.Status, t.Report, t.Reason, t.Read = protocol.TaskDone, report, "", true
		}); ok {
			s.halt(ended.Session)
			s.afterEnd(ended)
		}
	}
	s.stopTeamTasks(team.ID, i18n.T("The dream is done."))
	return nil
}

// dreamNotes sends a presentation back with the user's notes: the dream
// is building again, and the text returned continues its manager.
func (s *Service) dreamNotes(teamID, note string) (string, error) {
	team, err := s.changeDream(teamID, func(_ *protocol.Team, d *protocol.Dream) error {
		if d.Presentation == nil || d.Done != nil {
			return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("There is nothing presented to send notes on."))
		}
		d.Presentation.Verdict, d.Presentation.Note = protocol.DreamNotes, note
		d.Phase = protocol.DreamBuilding
		return nil
	})
	if err != nil {
		return "", err
	}
	s.record(protocol.Activity{Actor: protocol.InitiatorYou, Action: "dream.notes", Target: team.Name, OK: true, Team: team.ID, Task: team.Dream.Task, Initiator: protocol.InitiatorYou, Detail: s.redact(note)})
	s.resetIdle(team.Dream.Task)
	return notesText(team.Dream.Presentation.Title, note), nil
}
