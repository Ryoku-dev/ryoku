package teams

import (
	"fmt"
	"math"
	"reflect"

	"ryoku-rashin/internal/memberconfig"
	"ryoku-rashin/internal/protocol"
)

type memberHistory map[string][]memberconfig.Snapshot

func cloneMember(member protocol.Member) protocol.Member {
	return cloneTeam(protocol.Team{Members: []protocol.Member{member}}).Members[0]
}

// nextMemberHistory is prepared before the one state.json transaction. Deleted
// members lose history; restoring a snapshot cannot recreate a removed seat.
func nextMemberHistory(previous protocol.Team, next *protocol.Team, history memberHistory) (memberHistory, error) {
	result := make(memberHistory, len(next.Members))
	for i := range next.Members {
		member := &next.Members[i]
		old, exists := protocol.MemberByID(previous.Members, member.ID)
		entries := history[member.ID]
		if !exists {
			member.Revision = 1
		} else if memberConfigEqual(old, *member) {
			if member.Revision != old.Revision && (old.Revision == math.MaxUint64 || member.Revision != old.Revision+1) {
				return nil, protocol.Errorf(protocol.CodeConflict, "Invalid member revision.")
			}
		} else {
			if old.Revision == math.MaxUint64 {
				return nil, protocol.Errorf(protocol.CodeConflict, "Member configuration revisions are exhausted.")
			}
			if member.Revision == old.Revision {
				member.Revision++
			} else if member.Revision != old.Revision+1 {
				return nil, protocol.Errorf(protocol.CodeConflict, "Member configuration changed before storage.")
			}
		}
		if len(entries) == 0 && exists {
			return nil, fmt.Errorf("missing member configuration history")
		}
		entries = append([]memberconfig.Snapshot(nil), entries...)
		if len(entries) == 0 || entries[len(entries)-1].Member.Revision != member.Revision {
			at := next.Updated
			if at.IsZero() {
				at = next.Created
			}
			entries = append(entries, memberconfig.Snapshot{Member: cloneMember(*member), At: at})
		}
		if len(entries) > protocol.MemberConfigHistoryLimit {
			entries = entries[len(entries)-protocol.MemberConfigHistoryLimit:]
		}
		result[member.ID] = entries
	}
	return result, nil
}

// migrateMemberHistory seeds only explicitly unversioned legacy files. A file
// declaring the new version must have complete, consistent snapshots or fail.
func (st *state) loadMemberHistory(version int, histories map[string]memberHistory) error {
	if version != 0 && version != 1 {
		return fmt.Errorf("unsupported member history version")
	}
	if version == 0 && len(histories) != 0 {
		return fmt.Errorf("unversioned member history")
	}
	st.memberHistories = make(map[string]memberHistory, len(st.teamSet))
	for id, team := range st.teamSet {
		if version == 0 {
			history := make(memberHistory, len(team.Members))
			at := team.Updated
			if at.IsZero() {
				at = team.Created
			}
			for _, member := range team.Members {
				history[member.ID] = []memberconfig.Snapshot{{Member: cloneMember(member), At: at}}
			}
			st.memberHistories[id] = history
			continue
		}
		history := histories[id]
		if len(history) != len(team.Members) {
			return fmt.Errorf("incomplete member history")
		}
		for _, member := range team.Members {
			entries := history[member.ID]
			if len(entries) == 0 || len(entries) > protocol.MemberConfigHistoryLimit {
				return fmt.Errorf("invalid member history retention")
			}
			var revision uint64
			for _, entry := range entries {
				if entry.Member.ID != member.ID || entry.Member.Revision == 0 || entry.Member.Revision <= revision || revision != 0 && entry.Member.Revision != revision+1 {
					return fmt.Errorf("corrupt member history")
				}
				revision = entry.Member.Revision
			}
			if !reflect.DeepEqual(entries[len(entries)-1].Member, member) {
				return fmt.Errorf("member history disagrees with canonical configuration")
			}
		}
		st.memberHistories[id] = history
	}
	if version == 1 && len(histories) != len(st.teamSet) {
		return fmt.Errorf("orphan member history")
	}
	return nil
}

func (st *state) memberSnapshots(teamID, memberID string) (protocol.Member, []memberconfig.Snapshot, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if err := st.writer.available(); err != nil {
		return protocol.Member{}, nil, err
	}
	team, exists := st.teamSet[teamID]
	member, found := protocol.MemberByID(team.Members, memberID)
	if !exists || !found {
		return protocol.Member{}, nil, protocol.Errorf(protocol.CodeNotFound, "This member no longer exists.")
	}
	entries := st.memberHistories[teamID][memberID]
	result := make([]memberconfig.Snapshot, len(entries))
	for i, entry := range entries {
		result[i] = memberconfig.Snapshot{Member: cloneMember(entry.Member), At: entry.At}
	}
	return cloneMember(member), result, nil
}

func snapshotAt(entries []memberconfig.Snapshot, revision uint64) (memberconfig.Snapshot, error) {
	for i := len(entries) - 1; i >= 0; i-- {
		if revision == 0 || entries[i].Member.Revision == revision {
			return entries[i], nil
		}
	}
	return memberconfig.Snapshot{}, protocol.Errorf(protocol.CodeNotFound, "This configuration revision is not retained.")
}

func historyResult(current protocol.Member, entries []memberconfig.Snapshot) protocol.MemberConfigHistoryResult {
	result := protocol.MemberConfigHistoryResult{RetentionLimit: protocol.MemberConfigHistoryLimit, Revisions: make([]protocol.MemberConfigRevision, 0, len(entries))}
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		result.Revisions = append(result.Revisions, protocol.MemberConfigRevision{Revision: entry.Member.Revision, At: entry.At, Name: entry.Member.Name, Current: entry.Member.Revision == current.Revision})
	}
	return result
}
