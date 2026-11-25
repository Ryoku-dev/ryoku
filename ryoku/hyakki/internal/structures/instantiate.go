package structures

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"ryoku-rashin/internal/protocol"
)

var keyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Validate checks every selectable seat count, not just the default, so a
// template cannot save a graph which breaks when optional seats are omitted.
func Validate(t protocol.StructureTemplate) error {
	id := strings.TrimPrefix(t.ID, "custom:")
	if !keyPattern.MatchString(id) || strings.TrimSpace(t.Name) == "" || utf8.RuneCountInString(t.Name) > 160 {
		return fmt.Errorf("structure requires a valid ID and a name of at most 160 characters")
	}
	if len(t.Description) > 16000 || len(t.Culture) > 80 || len(t.Source) > 4000 || len(t.Notes) > 32 || len(t.JobFit) > 32 {
		return fmt.Errorf("structure metadata exceeds limits")
	}
	for _, v := range t.Notes {
		if len(v) > 8000 {
			return fmt.Errorf("structure note exceeds limit")
		}
	}
	for _, v := range t.JobFit {
		if !keyPattern.MatchString(v) {
			return fmt.Errorf("invalid job fit %q", v)
		}
	}
	switch t.Shape {
	case "deep", "wide", "flat", "court", "ring", "horde":
	default:
		return fmt.Errorf("unknown structure shape %q", t.Shape)
	}
	if t.Reporting != protocol.ReportingTree && t.Reporting != protocol.ReportingPeers {
		return fmt.Errorf("reporting must be tree or peers")
	}
	if t.MinSeats < 1 || t.MaxSeats > protocol.TeamsMembersMax || t.MaxSeats < t.MinSeats || t.DefaultSeats < t.MinSeats || t.DefaultSeats > t.MaxSeats {
		return fmt.Errorf("invalid seat bounds (maximum %d, including lead)", protocol.TeamsMembersMax)
	}
	if len(t.Seats) == 0 || len(t.Seats) > protocol.TeamsMembersMax || (!t.Variable && len(t.Seats) != t.MaxSeats) {
		return fmt.Errorf("seat roster must cover all selectable seats")
	}
	if t.Variable && (len(t.Seats) < 2 || t.MinSeats < 2 || t.Reporting != protocol.ReportingTree || !t.Seats[0].Lead || t.Seats[0].ReportsToSeat != "") {
		return fmt.Errorf("variable procession requires a root marshal and worker prototypes")
	}
	seen := make(map[string]bool, len(t.Seats))
	for i, s := range t.Seats {
		if !keyPattern.MatchString(s.ID) || seen[s.ID] || strings.TrimSpace(s.Title) == "" || strings.TrimSpace(s.Rank) == "" || strings.TrimSpace(s.Duty) == "" {
			return fmt.Errorf("seat %d requires unique ID, title, rank and duty", i+1)
		}
		if utf8.RuneCountInString(s.Title) > 160 || len(s.Rank) > 80 || len(s.Duty) > 16000 || utf8.RuneCountInString(s.DefaultSpirit) > 160 {
			return fmt.Errorf("seat %q exceeds text limits", s.ID)
		}
		if utf8.RuneCountInString(strings.TrimSpace(s.CapabilitiesHint)) > protocol.MemberCapabilitiesMaxRunes {
			return fmt.Errorf("seat %q capabilities exceed %d characters", s.ID, protocol.MemberCapabilitiesMaxRunes)
		}
		if strings.ContainsRune(s.Rank, 0) || strings.ContainsRune(s.DefaultSpirit, 0) {
			return fmt.Errorf("seat %q identity contains a zero byte", s.ID)
		}
		if t.Reporting == protocol.ReportingPeers && s.ReportsToSeat != "" {
			return fmt.Errorf("peer seat %q must report directly to the Summoner", s.ID)
		}
		switch s.SuggestedModelClass {
		case "", "smart", "fast", "fast-tool-capable", "specialist", "reasoning", "vision":
		default:
			return fmt.Errorf("seat %q model class is advice, not a provider/model ID", s.ID)
		}
		if s.ReportsToSeat == s.ID {
			return fmt.Errorf("seat %q reports to itself", s.ID)
		}
		if s.Lead && s.ReportsToSeat != "" {
			return fmt.Errorf("lead %q cannot report to another seat", s.ID)
		}
		if t.Variable && i > 0 && (s.Lead || s.ReportsToSeat != t.Seats[0].ID) {
			return fmt.Errorf("variable workers must report only to the marshal")
		}
		seen[s.ID] = true
	}
	for count := t.MinSeats; count <= t.MaxSeats; count++ {
		if t.Variable {
			break
		}
		roots, leads := 0, 0
		selected := make(map[string]protocol.StructureSeat, count)
		for _, s := range t.Seats[:count] {
			selected[s.ID] = s
			if s.ReportsToSeat == "" {
				roots++
			}
			if s.Lead {
				leads++
			}
		}
		for _, start := range t.Seats[:count] {
			chain := make(map[string]bool, count)
			for s := start; ; {
				if chain[s.ID] {
					return fmt.Errorf("reporting cycle at seat %q", s.ID)
				}
				chain[s.ID] = true
				if s.ReportsToSeat == "" {
					if t.Reporting == protocol.ReportingTree && !s.Lead {
						return fmt.Errorf("seat %q does not reach the lead", start.ID)
					}
					break
				}
				next, ok := selected[s.ReportsToSeat]
				if !ok {
					return fmt.Errorf("seat %q manager is absent at count %d", s.ID, count)
				}
				s = next
			}
		}
		if t.Reporting == protocol.ReportingTree && (roots != 1 || leads != 1) {
			return fmt.Errorf("tree at count %d needs one reachable lead root", count)
		}
		if t.Reporting == protocol.ReportingPeers && leads > 1 {
			return fmt.Errorf("peer roster may have at most one synthesizer")
		}
	}
	// Every supported roster is a prefix of the maximum expansion, including
	// variable prototypes. Check the actual names once without building members:
	// suffixes must fit too, and no owner text may be clipped to make them fit.
	names := make(map[string]bool, t.MaxSeats)
	for i := range t.MaxSeats {
		name := rosterName(t, i, names)
		if name == "" || utf8.RuneCountInString(name) > protocol.TeamNameMaxRunes {
			return fmt.Errorf("seat %q needs a generated name of 1..%d characters", rosterSeat(t, i).ID, protocol.TeamNameMaxRunes)
		}
	}
	return nil
}

// Instantiate produces a non-running draft. Count zero chooses DefaultSeats;
// every other count is checked, never clamped. Stable temporary IDs and all
// ReportsTo values must be rebased together by team normalization. Model is
// deliberately empty: suggested class never pins a paid provider.
func Instantiate(t protocol.StructureTemplate, count int) ([]protocol.Member, error) {
	if err := Validate(t); err != nil {
		return nil, err
	}
	if count == 0 {
		count = t.DefaultSeats
	}
	if count < t.MinSeats || count > t.MaxSeats {
		return nil, fmt.Errorf("seat count must be %d..%d including the lead", t.MinSeats, t.MaxSeats)
	}
	return instantiate(t, count), nil
}

// instantiate expands an already validated recipe and supported total count.
func instantiate(t protocol.StructureTemplate, count int) []protocol.Member {
	members := make([]protocol.Member, 0, count)
	names := make(map[string]bool, count)
	for i := range count {
		s := rosterSeat(t, i)
		seatID := s.ID
		if t.Variable {
			seatID = fmt.Sprintf("parade-seat-%03d", i)
		}
		name := rosterName(t, i, names)
		manager := ""
		if s.ReportsToSeat != "" {
			manager = "draft-" + s.ReportsToSeat
		}
		if t.Variable && i > 0 {
			manager = "draft-parade-seat-000"
		}
		members = append(members, protocol.Member{ID: "draft-" + seatID, Seat: seatID, Name: name, Title: s.Title, Rank: s.Rank, Spirit: s.DefaultSpirit, Lead: s.Lead, ReportsTo: manager, Capabilities: s.CapabilitiesHint, Instructions: s.Duty})
	}
	return members
}

func rosterSeat(t protocol.StructureTemplate, i int) protocol.StructureSeat {
	if t.Variable && i > 0 {
		i = 1 + (i-1)%(len(t.Seats)-1)
	}
	return t.Seats[i]
}

// rosterName is shared by validation and generation so every selectable prefix
// has the same canonical-spirit fallback and case-insensitive suffix allocation.
func rosterName(t protocol.StructureTemplate, i int, names map[string]bool) string {
	s := rosterSeat(t, i)
	name := s.Title
	if spirit, ok := spiritIndex[s.DefaultSpirit]; ok {
		name = spirit.Name
	} else if s.DefaultSpirit != "" {
		name = s.DefaultSpirit
	}
	name = strings.TrimSpace(name)
	// All repeated variable prototypes get an index, including their first copy.
	if t.Variable && i > 0 {
		name = fmt.Sprintf("%s-%d", name, i)
	}
	base := name
	// At most 99 prior seats can occupy names; one of these 100 candidates is free.
	for suffix := 2; suffix <= protocol.TeamsMembersMax && names[strings.ToLower(name)]; suffix++ {
		name = fmt.Sprintf("%s-%d", base, suffix)
	}
	names[strings.ToLower(name)] = true
	return name
}
