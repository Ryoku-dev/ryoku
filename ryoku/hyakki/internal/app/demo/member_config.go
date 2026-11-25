package demo

import (
	"bytes"
	"encoding/json"

	"ryoku-rashin/internal/memberconfig"
	"ryoku-rashin/internal/protocol"
)

func memberConfigCall[T any](raw json.RawMessage, run func(T) (any, error)) (any, error) {
	var params T
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&params); err != nil {
		return nil, badRequest("The request's parameters are not valid.")
	}
	if len(bytes.TrimSpace(raw[decoder.InputOffset():])) != 0 {
		return nil, badRequest("The request must contain exactly one parameter object.")
	}
	return run(params)
}

func copyConfigMember(member protocol.Member) protocol.Member {
	// Fixtures own their canonical objects; snapshots must not share mutable maps.
	raw, _ := json.Marshal(member)
	var copy protocol.Member
	_ = json.Unmarshal(raw, &copy)
	return copy
}

func (s *server) ensureMemberHistory(team protocol.Team) protocol.Team {
	t := &s.data.teams
	if t.memberHistories == nil {
		t.memberHistories = make(map[string]map[string][]memberconfig.Snapshot)
	}
	if t.memberHistories[team.ID] != nil {
		return team
	}
	team.Members = append([]protocol.Member(nil), team.Members...)
	for i := range team.Members {
		if team.Members[i].Revision == 0 {
			team.Members[i].Revision = 1
		}
	}
	if index := t.teamIndex(team.ID); index >= 0 {
		t.teams[index] = team
	}
	s.captureMemberHistory(team)
	return team
}

func (s *server) captureMemberHistory(team protocol.Team) {
	t := &s.data.teams
	if t.memberHistories == nil {
		t.memberHistories = make(map[string]map[string][]memberconfig.Snapshot)
	}
	previous := t.memberHistories[team.ID]
	next := make(map[string][]memberconfig.Snapshot, len(team.Members))
	for _, member := range team.Members {
		entries := append([]memberconfig.Snapshot(nil), previous[member.ID]...)
		if len(entries) == 0 || entries[len(entries)-1].Member.Revision != member.Revision {
			at := team.Updated
			if at.IsZero() {
				at = team.Created
			}
			entries = append(entries, memberconfig.Snapshot{Member: copyConfigMember(member), At: at})
		}
		if len(entries) > protocol.MemberConfigHistoryLimit {
			entries = entries[len(entries)-protocol.MemberConfigHistoryLimit:]
		}
		next[member.ID] = entries
	}
	t.memberHistories[team.ID] = next
}

func (s *server) memberSnapshots(teamID, memberID string) (protocol.Member, []memberconfig.Snapshot, error) {
	index := s.data.teams.teamIndex(teamID)
	if index < 0 {
		return protocol.Member{}, nil, notFound("team")
	}
	team := s.ensureMemberHistory(s.data.teams.teams[index])
	member, found := protocol.MemberByID(team.Members, memberID)
	if !found {
		return protocol.Member{}, nil, notFound("member")
	}
	return member, s.data.teams.memberHistories[teamID][memberID], nil
}

func (s *server) memberConfigGet(p protocol.MemberConfigRef) (protocol.MemberConfigDocument, error) {
	current, entries, err := s.memberSnapshots(p.Team, p.Member)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if p.Revision == 0 || entries[i].Member.Revision == p.Revision {
			return memberconfig.Encode(p.Team, entries[i], current.Revision)
		}
	}
	return protocol.MemberConfigDocument{}, notFound("retained revision")
}

func (s *server) memberConfigHistory(p protocol.MemberConfigRef) (protocol.MemberConfigHistoryResult, error) {
	current, entries, err := s.memberSnapshots(p.Team, p.Member)
	if err != nil {
		return protocol.MemberConfigHistoryResult{}, err
	}
	result := protocol.MemberConfigHistoryResult{RetentionLimit: protocol.MemberConfigHistoryLimit, Revisions: make([]protocol.MemberConfigRevision, 0, len(entries))}
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		result.Revisions = append(result.Revisions, protocol.MemberConfigRevision{Revision: entry.Member.Revision, At: entry.At, Name: entry.Member.Name, Current: entry.Member.Revision == current.Revision})
	}
	return result, nil
}

func (s *server) memberConfigSave(p protocol.MemberConfigSaveParams) (protocol.MemberConfigDocument, error) {
	current, _, err := s.memberSnapshots(p.Team, p.Member)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	member, err := memberconfig.Decode(p.YAML, p.Team, p.Member, p.ExpectedRevision, current)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	return s.commitMemberConfig(p.Team, member, false)
}

func (s *server) memberConfigRestore(p protocol.MemberConfigRestoreParams) (protocol.MemberConfigDocument, error) {
	current, entries, err := s.memberSnapshots(p.Team, p.Member)
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	if p.ExpectedRevision == 0 || current.Revision != p.ExpectedRevision {
		return protocol.MemberConfigDocument{}, protocol.Errorf(protocol.CodeConflict, "The member changed; reload before restoring.")
	}
	for _, entry := range entries {
		if p.Revision != 0 && entry.Member.Revision == p.Revision {
			member := copyConfigMember(entry.Member)
			member.Revision = current.Revision
			return s.commitMemberConfig(p.Team, member, true)
		}
	}
	return protocol.MemberConfigDocument{}, notFound("retained revision")
}

func (s *server) commitMemberConfig(teamID string, member protocol.Member, restore bool) (protocol.MemberConfigDocument, error) {
	index := s.data.teams.teamIndex(teamID)
	if index < 0 {
		return protocol.MemberConfigDocument{}, notFound("team")
	}
	team := s.data.teams.teams[index]
	members := append([]protocol.Member(nil), team.Members...)
	for i := range members {
		if members[i].ID == member.ID {
			members[i] = member
		}
	}
	patch, err := json.Marshal(map[string]any{"members": members})
	if err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	force := ""
	if restore {
		force = member.ID
	}
	if _, err := s.updateTeamConfig(protocol.TeamUpdateParams{ID: teamID, Patch: patch}, force); err != nil {
		return protocol.MemberConfigDocument{}, err
	}
	return s.memberConfigGet(protocol.MemberConfigRef{Team: teamID, Member: member.ID})
}

func (s *server) checkMemberReferences(team string, member protocol.Member) error {
	return memberconfig.CheckReferences(team, member, func(ref string) (protocol.Credential, bool) {
		for _, credential := range s.data.teams.vault.Credentials {
			if credential.ID == ref || credential.Name == ref {
				return credential, true
			}
		}
		return protocol.Credential{}, false
	})
}
