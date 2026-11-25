package teams

import (
	"context"
	"encoding/json"

	"ryoku-rashin/internal/memberconfig"
	"ryoku-rashin/internal/protocol"
)

func (s *Service) memberConfigGet(p protocol.MemberConfigRef) (protocol.MemberConfigDocument, error) {
	current, entries, err := s.state.memberSnapshots(p.Team, p.Member)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	snapshot, err := snapshotAt(entries, p.Revision)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	return memberconfig.Encode(p.Team, snapshot, current.Revision)
}

func (s *Service) memberConfigHistory(p protocol.MemberConfigRef) (protocol.MemberConfigHistoryResult, error) {
	current, entries, err := s.state.memberSnapshots(p.Team, p.Member)
	if err != nil {
		return protocol.MemberConfigHistoryResult{}, err
	}
	return historyResult(current, entries), nil
}

func (s *Service) memberConfigSave(ctx context.Context, p protocol.MemberConfigSaveParams) (protocol.MemberConfigDocument, error) {
	current, _, err := s.state.memberSnapshots(p.Team, p.Member)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	member, err := memberconfig.Decode(p.YAML, p.Team, p.Member, p.ExpectedRevision, current)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	return s.commitMemberConfig(ctx, p.Team, member, p.ExpectedRevision, false)
}

func (s *Service) memberConfigRestore(ctx context.Context, p protocol.MemberConfigRestoreParams) (protocol.MemberConfigDocument, error) {
	current, entries, err := s.state.memberSnapshots(p.Team, p.Member)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	if p.ExpectedRevision == 0 || p.ExpectedRevision != current.Revision {
		return protocol.MemberConfigDocument{}, protocol.Errorf(protocol.CodeConflict, "The member changed; reload before restoring.")
	}
	if p.Revision == 0 {
		return protocol.MemberConfigDocument{}, protocol.Errorf(protocol.CodeBadRequest, "Choose a retained revision to restore.")
	}
	snapshot, err := snapshotAt(entries, p.Revision)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	member := snapshot.Member
	member.Revision = current.Revision
	return s.commitMemberConfig(ctx, p.Team, member, p.ExpectedRevision, true)
}

func (s *Service) commitMemberConfig(ctx context.Context, teamID string, member protocol.Member, expected uint64, restore bool) (protocol.MemberConfigDocument, error) {
	team, exists := s.state.team(teamID)
	current, found := protocol.MemberByID(team.Members, member.ID)
	if !exists || !found {
		return protocol.MemberConfigDocument{}, protocol.Errorf(protocol.CodeNotFound, "This member no longer exists.")
	}
	if current.Revision != expected {
		return protocol.MemberConfigDocument{}, protocol.Errorf(protocol.CodeConflict, "The member changed; reload before saving.")
	}
	for i := range team.Members {
		if team.Members[i].ID == member.ID {
			team.Members[i] = member
		}
	}
	patch, err := json.Marshal(map[string]any{"members": team.Members})
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	force := ""
	if restore {
		force = member.ID
	}
	updated, err := s.updateTeamConfig(ctx, protocol.TeamUpdateParams{ID: teamID, Patch: patch, ExpectedUpdated: team.Updated}, force)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	// Resolve this commit's retained timestamp, including a no-op save.
	committed, _ := protocol.MemberByID(updated.Members, member.ID)
	current, entries, err := s.state.memberSnapshots(teamID, member.ID)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	snapshot, err := snapshotAt(entries, committed.Revision)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	return memberconfig.Encode(teamID, snapshot, current.Revision)
}

func (s *Service) checkConfigReferences(team protocol.Team, previous protocol.Team, force string) error {
	for _, member := range team.Members {
		old, _ := protocol.MemberByID(previous.Members, member.ID)
		if force != member.ID && memberConfigEqual(old, member) {
			continue
		}
		if err := memberconfig.CheckReferences(team.ID, member, func(ref string) (protocol.Credential, bool) {
			c, exists := s.vault.Get(ref)
			if !exists {
				c, exists = s.vault.Find(ref)
			}
			return c, exists
		}); err != nil {
			return err
		}
	}
	return nil
}
