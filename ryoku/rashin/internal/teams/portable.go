package teams

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// exportVersion is the TeamExport format this Rashin writes and reads.
const exportVersion = 1

// maxImportBytes bounds what an import reads; a team's definition is small.
const maxImportBytes = 1 << 20

// exportTeam writes a team's definition, without history or credential
// values, to <folder>/<slug>.rashin.json and returns it.
func (s *Service) exportTeam(id string) (protocol.TeamExportResult, error) {
	team, ok := s.state.team(id)
	if !ok {
		return protocol.TeamExportResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", id))
	}
	doc := protocol.TeamExport{
		Version: exportVersion, Name: team.Name, Purpose: team.Purpose, Template: team.Template,
		Members: team.Members, Tools: team.Tools, Approvals: team.Approvals, Budget: team.Budget, Notify: team.Notify,
	}
	for _, r := range s.state.routines() {
		if r.Team == id {
			doc.Routines = append(doc.Routines, protocol.RoutineDraft{
				Title: r.Title, Brief: r.Brief, Member: r.Member, Schedule: r.Schedule,
			})
		}
	}
	for _, c := range s.vault.List() {
		if slices.Contains(c.Teams, id) {
			doc.Credentials = append(doc.Credentials, c.Name)
		}
	}
	slices.Sort(doc.Credentials)
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return protocol.TeamExportResult{}, err
	}
	name := slugName(team.Name) + ".rashin.json"
	path := filepath.Join(team.Folder, name)
	if err := writeStateFile(team.Folder, name, append(data, '\n')); err != nil {
		return protocol.TeamExportResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("Rashin cannot write %s: %v.", paths.Short(path), err))
	}
	s.record(protocol.Activity{
		Actor: "you", Action: "team.export", Target: team.Name, OK: true,
		Detail: i18n.Tf("Saved the team's definition to %s.", paths.Short(path)), Team: id, Initiator: protocol.InitiatorYou,
	})
	return protocol.TeamExportResult{Export: doc, Path: path}, nil
}

// importTeam makes a team from an exported definition. It arrives paused,
// with its routines off and no credential granted, so nothing it does
// starts before the user has looked at it.
func (s *Service) importTeam(ctx context.Context, p protocol.TeamImportParams) (protocol.Team, error) {
	path := expandHome(strings.TrimSpace(p.Path))
	if !filepath.IsAbs(path) {
		return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Pick the team file by its full path."))
	}
	info, err := os.Stat(path)
	if err != nil {
		return protocol.Team{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("Rashin cannot read %s.", paths.Short(path)))
	}
	if info.Size() > maxImportBytes {
		return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("That file is too big to be a team."))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return protocol.Team{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("Rashin cannot read %s.", paths.Short(path)))
	}
	var doc protocol.TeamExport
	if err := json.Unmarshal(data, &doc); err != nil || doc.Version < 1 {
		return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("That file is not a Rashin team."))
	}
	if doc.Version > exportVersion {
		return protocol.Team{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("That team was saved by a newer Rashin."))
	}
	now := s.now()
	team := protocol.Team{
		ID: newID("team"), Name: doc.Name, Purpose: doc.Purpose, Template: doc.Template,
		Members: doc.Members, Tools: doc.Tools, Approvals: doc.Approvals, Budget: doc.Budget, Notify: doc.Notify,
		Paused: true, PauseReason: protocol.PausedByYou, Created: now, Updated: now,
	}
	drafts := make([]protocol.RoutineDraft, 0, len(doc.Routines))
	for _, draft := range doc.Routines {
		draft.Enabled = false
		drafts = append(drafts, draft)
	}
	detail := func(protocol.Team) string {
		return i18n.Tf("Imported from %s; it waits paused until you let it work.", paths.Short(path))
	}
	imported, err := s.addTeam(ctx, team, drafts, "team.import", detail)
	if err != nil {
		return protocol.Team{}, err
	}
	s.clock.rearm()
	return imported, nil
}
