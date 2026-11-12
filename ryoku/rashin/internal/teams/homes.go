package teams

import (
	"strings"

	i18n "ryoku-i18n"

	registry "ryoku-rashin/internal/harness"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// restoreShown is how many paths a restore notice names of each kind; the
// rest are counted.
const restoreShown = 8

// RestoreHome puts team's home for a harness back as the user signed it in
// and, when anything moved, records it in the ledger and leaves a notice in
// the Inbox under the team. A notice the same as one still unread is folded
// into it: its count goes up and it names the newest quarantine folder.
func (s *Service) RestoreHome(harness, team string) error {
	restored, err := registry.Restore(harness, team)
	if err != nil {
		return err
	}
	if restored.Empty() {
		return nil
	}
	name := registry.Name(harness, s.settings())
	title := i18n.Tf("Rashin put back %s's home for this team", name)
	var what []string
	if len(restored.PutBack) > 0 {
		what = append(what, i18n.Tf("Put back as you signed it in: %s", pathList(restored.PutBack)))
	}
	if len(restored.Quarantined) > 0 {
		what = append(what, i18n.Tf("Moved out, as a team run added or changed them: %s", pathList(restored.Quarantined)))
	}
	text := strings.Join(what, "\n")
	if restored.Folder != "" {
		text += "\n" + i18n.Tf("What moved is in %s.", paths.Short(restored.Folder))
	}
	s.record(protocol.Activity{Actor: "Rashin", Action: "home.restore", Target: title, OK: true, Team: team, Initiator: protocol.InitiatorRashin, Detail: text})
	if s.foldRestore(team, title, strings.Join(what, "\n"), text) {
		return nil
	}
	_, err = s.ask(s.background, caller{Team: team}, protocol.Approval{Team: team, Kind: protocol.ApprovalRestore, Title: title, Detail: text}, nil)
	return err
}

// foldRestore counts a restore into an unread notice of team that said the
// same, what, and reports whether there was one.
func (s *Service) foldRestore(team, title, what, text string) bool {
	s.mu.Lock()
	unread := s.state.approvals(func(r approvalRecord) bool {
		return r.Kind == protocol.ApprovalRestore && r.Status == protocol.ApprovalPending && r.Team == team && r.Title == title &&
			(r.Detail == what || strings.HasPrefix(r.Detail, what+"\n"))
	})
	if len(unread) == 0 {
		s.mu.Unlock()
		return false
	}
	rec := unread[0]
	rec.Repeats++
	rec.Detail = text
	err := s.state.putApproval(rec)
	s.mu.Unlock()
	if err != nil {
		return false
	}
	s.emitApproval(rec.Approval)
	return true
}

// pathList names paths under a team home, at most restoreShown of them.
func pathList(rel []string) string {
	shown := rel[:min(len(rel), restoreShown)]
	out := strings.Join(shown, ", ")
	if more := len(rel) - len(shown); more > 0 {
		out += " " + i18n.Tf("and %d more", more)
	}
	return out
}

// Working reports whether a task of team works on harness now: it holds a
// session, or waits on the user or its members in one.
func (s *Service) Working(harness, team string) bool {
	return len(s.state.tasks(func(t protocol.TeamTask) bool {
		return t.Harness == harness && t.Team == team && taskIsOpen(t) && t.Session != ""
	})) > 0
}

// TeamName is a team's name, false for a team that does not exist.
func (s *Service) TeamName(team string) (string, bool) {
	t, ok := s.state.team(team)
	return t.Name, ok
}

// migrateHomes makes each team home that every team on a harness shared, as
// Rashin once kept them, the home of one team: the team whose task last ran
// on that harness, else the one team that uses it. The other teams are not
// signed in to it. A home no team can be found for stays as it was.
func (s *Service) migrateHomes() {
	for _, harness := range registry.LegacyHomes() {
		team := s.homeOwner(harness)
		if team == "" {
			continue
		}
		if err := registry.MigrateHome(harness, team); err != nil {
			continue
		}
		t, _ := s.state.team(team)
		s.record(protocol.Activity{
			Actor: "Rashin", Action: "home.migrate", Target: registry.Name(harness, s.settings()), OK: true, Team: team, Initiator: protocol.InitiatorRashin,
			Detail: i18n.Tf("Its home for teams is now %s's alone; other teams sign in to it on their own.", t.Name),
		})
	}
}

// homeOwner is the team a shared home of harness goes to, empty for none.
func (s *Service) homeOwner(harness string) string {
	latest := protocol.TeamTask{}
	for _, task := range s.state.tasks(func(t protocol.TeamTask) bool { return t.Harness == harness }) {
		if _, ok := s.state.team(task.Team); ok && task.Updated.After(latest.Updated) {
			latest = task
		}
	}
	if latest.Team != "" {
		return latest.Team
	}
	settings := s.settings()
	var users []string
	for _, team := range s.state.teams() {
		for _, member := range team.Members {
			if id, _, _ := protocol.ResolveEngine(member, settings, s.defaultHarness(settings)); id == harness {
				users = append(users, team.ID)
				break
			}
		}
	}
	if len(users) == 1 {
		return users[0]
	}
	return ""
}
