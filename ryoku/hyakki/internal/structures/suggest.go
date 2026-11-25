package structures

import (
	"fmt"
	"sort"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// ValidateAnswers rejects malformed questions rather than silently treating
// invalid values as defaults. Culture remains a free preference for custom recipes.
func ValidateAnswers(a protocol.StructureAnswers) error {
	if len(a.Job) > 16000 || len(a.Culture) > 80 || a.Count < 0 || a.Count > protocol.TeamsMembersMax {
		return fmt.Errorf("question text or total seat count exceeds limits")
	}
	switch strings.ToLower(strings.TrimSpace(a.Work)) {
	case "", "build", "research", "watch", "decide", "triage", "create", "verify", "harden":
	default:
		return fmt.Errorf("unknown work kind")
	}
	switch strings.ToLower(strings.TrimSpace(a.Flow)) {
	case "", "deep", "wide", "ring", "horde":
	default:
		return fmt.Errorf("unknown work flow")
	}
	switch a.Size {
	case "", "few", "court", "horde", "choose":
	default:
		return fmt.Errorf("unknown roster size")
	}
	switch a.Oversight {
	case "", "every-step", "risky-steps", "within-budget":
	default:
		return fmt.Errorf("unknown oversight policy")
	}
	return nil
}

// Suggest ranks validated, visible templates using only the short-question
// answers. Nil templates selects shipped defaults; an explicitly empty slice
// yields no choices. Culture is a preference, not permission; mismatches remain
// available below matching choices. No account, network or model call occurs.
func Suggest(a protocol.StructureAnswers, templates []protocol.StructureTemplate) []protocol.StructureSuggestion {
	return suggest(a, templates, 0)
}

// SuggestTop bounds expanded rosters for a finite wire response. Ranking is
// identical to Suggest; candidates are ranked before allocating member drafts.
func SuggestTop(a protocol.StructureAnswers, templates []protocol.StructureTemplate, limit int) []protocol.StructureSuggestion {
	return suggest(a, templates, limit)
}

func suggest(a protocol.StructureAnswers, templates []protocol.StructureTemplate, limit int) []protocol.StructureSuggestion {
	if templates == nil {
		templates = Builtin()
	}
	work := strings.ToLower(strings.TrimSpace(a.Work))
	flow := strings.ToLower(strings.TrimSpace(a.Flow))
	culture := strings.ToLower(strings.TrimSpace(a.Culture))
	if work == "" {
		// One-line job is only a fallback when the work question is unanswered;
		// explicit answers always win over incidental words in a prompt.
		words := strings.FieldsFunc(strings.ToLower(a.Job), func(r rune) bool { return r < 'a' || r > 'z' })
		for _, w := range words {
			switch w {
			case "build", "research", "watch", "decide", "triage", "create", "verify", "harden":
				work = w
			}
			if work != "" {
				break
			}
		}
	}
	approvals, planApproval := protocol.ApprovalsAsk, false
	switch a.Oversight {
	case "every-step":
		planApproval = true
	case "within-budget":
		approvals = protocol.ApprovalsAuto
	}
	out := make([]protocol.StructureSuggestion, 0, len(templates))
	seen := make(map[string]bool)
	for _, t := range templates {
		if t.Hidden || seen[t.ID] {
			continue
		}
		if err := Validate(t); err != nil {
			continue
		}
		if a.Count > 0 && (a.Count < t.MinSeats || a.Count > t.MaxSeats) {
			continue
		}
		seen[t.ID] = true
		score := 0
		reasons := make([]string, 0, 4)
		for _, fit := range t.JobFit {
			if fit == work {
				score += 100
				reasons = append(reasons, "fits "+work+" work")
				break
			}
		}
		matchesFlow := flow != "" && (flow == t.Shape || flow == "deep" && t.Reporting == protocol.ReportingTree && !t.Variable || flow == "wide" && t.Reporting == protocol.ReportingPeers && t.Shape != "ring")
		if matchesFlow {
			score += 55
			reasons = append(reasons, "matches the "+flow+" flow")
		}
		if culture != "" && culture != "none" && culture != "no preference" && culture != "mixed" {
			if culture == t.Culture {
				score += 80
				reasons = append(reasons, "matches the "+culture+" culture preference")
			} else {
				score -= 20
			}
		}
		count := t.DefaultSeats
		target := a.Count
		if target <= 0 {
			switch a.Size {
			case "few":
				target = 4
			case "court":
				target = 8
			case "horde":
				target = 20
			}
		}
		if target > 0 {
			count = target
			if count < t.MinSeats {
				count = t.MinSeats
			}
			if count > t.MaxSeats {
				count = t.MaxSeats
			}
			distance := count - target
			if distance < 0 {
				distance = -distance
			}
			if distance == 0 {
				score += 30
				reasons = append(reasons, fmt.Sprintf("supports %d total seats", count))
			} else {
				score -= min(distance, 100)
			}
		}
		// With no narrowing answers the namesake parade is the documented default.
		if t.ID == "night-parade" {
			score++
		}
		why := strings.Join(reasons, "; ")
		if why == "" {
			why = "An editable " + t.Shape + " shape; review its roles and reporting lines before summoning"
		}
		if target > 0 && target != count {
			why += fmt.Sprintf("; nearest supported roster has %d total seats (requested %d)", count, target)
		}
		why += ". Oversight and budget remain the Summoner's controls."
		out = append(out, protocol.StructureSuggestion{Template: t, SeatCount: count, Score: score, Why: why, Approvals: approvals, PlanApproval: planApproval})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Template.ID < out[j].Template.ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	for i := range out {
		out[i].Template = cloneTemplate(out[i].Template)
		out[i].Members = instantiate(out[i].Template, out[i].SeatCount)
	}
	return out
}
