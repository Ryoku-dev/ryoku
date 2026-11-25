package protocol

import "fmt"

// ValidateReporting checks the persisted graph, not rank names. Tree teams have
// one lead root; peer roots answer to the Summoner and may choose a synthesizer.
func ValidateReporting(team Team) error {
	if team.Reporting != ReportingTree && team.Reporting != ReportingPeers {
		return fmt.Errorf("reporting must be %q or %q", ReportingTree, ReportingPeers)
	}
	if len(team.Members) == 0 || len(team.Members) > TeamsMembersMax {
		return fmt.Errorf("a reporting graph needs between 1 and %d members", TeamsMembersMax)
	}
	ids := make(map[string]int, len(team.Members))
	leads := 0
	for i, member := range team.Members {
		if member.ID == "" {
			return fmt.Errorf("every reporting seat needs an ID")
		}
		if _, exists := ids[member.ID]; exists {
			return fmt.Errorf("reporting seat %q appears twice", member.ID)
		}
		ids[member.ID] = i
		if member.Lead {
			leads++
			if member.ReportsTo != "" {
				return fmt.Errorf("lead %q must report to the Summoner", member.Name)
			}
		}
		if team.Reporting == ReportingPeers && member.ReportsTo != "" {
			return fmt.Errorf("peer %q reports directly to the Summoner, not another peer", member.Name)
		}
		if member.DelegationDepth < 0 || member.DelegationDepth > TeamsMembersMax {
			return fmt.Errorf("delegation depth must be between 0 and %d", TeamsMembersMax)
		}
	}
	if team.Dream != nil && team.Dream.Manager != "" {
		manager, exists := ids[team.Dream.Manager]
		if team.Reporting != ReportingTree || !exists || !team.Members[manager].Lead {
			return fmt.Errorf("a Dream manager must be the root of its reporting tree")
		}
	}
	if team.Reporting == ReportingTree && leads != 1 {
		return fmt.Errorf("a reporting tree needs exactly one lead")
	}
	if team.Reporting == ReportingPeers && leads > 1 {
		return fmt.Errorf("peer teams may choose at most one synthesizer")
	}
	parents := make([]int, len(team.Members))
	for i, member := range team.Members {
		parents[i] = -1
		if member.ReportsTo == "" {
			if team.Reporting == ReportingTree && !member.Lead {
				return fmt.Errorf("%q must report to a member of its tree", member.Name)
			}
			continue
		}
		parent, exists := ids[member.ReportsTo]
		if !exists {
			return fmt.Errorf("%q reports to an unknown member %q", member.Name, member.ReportsTo)
		}
		if parent == i {
			return fmt.Errorf("%q cannot report to itself", member.Name)
		}
		parents[i] = parent
	}
	seen := make([]uint8, len(parents))
	for start := range parents {
		if seen[start] != 0 {
			continue
		}
		at := start
		for at >= 0 && seen[at] == 0 {
			seen[at] = 1
			at = parents[at]
		}
		if at >= 0 && seen[at] == 1 {
			return fmt.Errorf("reporting lines form a cycle at %q", team.Members[at].Name)
		}
		for at = start; at >= 0 && seen[at] == 1; at = parents[at] {
			seen[at] = 2
		}
	}
	return nil
}

// DirectReports is derived from the single stored manager reference.
func DirectReports(team Team, manager string) []Member {
	if manager == "" {
		return nil
	}
	var reports []Member
	for _, member := range team.Members {
		if member.ReportsTo == manager {
			reports = append(reports, member)
		}
	}
	return reports
}

// CanDelegate permits only downward routing within the configured distance.
// A rank, lead title, or a member's trust tier never bypasses the graph.
func CanDelegate(team Team, from, to string) bool {
	if from == "" || from == to {
		return false
	}
	manager, exists := MemberByID(team.Members, from)
	if !exists {
		return false
	}
	depth := manager.DelegationDepth
	if depth == 0 {
		depth = 1
	}
	if depth < 0 || depth > TeamsMembersMax {
		return false
	}
	for range depth {
		member, exists := MemberByID(team.Members, to)
		if !exists || member.ReportsTo == "" {
			return false
		}
		if member.ReportsTo == from {
			return true
		}
		to = member.ReportsTo
	}
	return false
}

// ReportingChain is nearest manager first; the Summoner is its implicit end.
func ReportingChain(team Team, member string) []Member {
	var chain []Member
	for range len(team.Members) {
		current, exists := MemberByID(team.Members, member)
		if !exists || current.ReportsTo == "" {
			break
		}
		manager, exists := MemberByID(team.Members, current.ReportsTo)
		if !exists {
			break
		}
		chain = append(chain, manager)
		member = manager.ID
	}
	return chain
}

// CommonManager routes a lateral request, not an assignment. Empty means the
// two branches meet only at the Summoner.
func CommonManager(team Team, left, right string) string {
	ancestors := ReportingChain(team, left)
	for range len(team.Members) {
		member, exists := MemberByID(team.Members, right)
		if !exists || member.ReportsTo == "" {
			return ""
		}
		for _, ancestor := range ancestors {
			if ancestor.ID == member.ReportsTo {
				return ancestor.ID
			}
		}
		right = member.ReportsTo
	}
	return ""
}

// MemberByID never resolves a deleted seat through another member's name.
func MemberByID(members []Member, id string) (Member, bool) {
	for _, member := range members {
		if member.ID == id {
			return member, true
		}
	}
	return Member{}, false
}
