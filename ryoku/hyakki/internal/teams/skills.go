package teams

import (
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

// SetSkills hands the service the built-in skill sets its members are
// given; the daemon calls it once, before Bind.
func (s *Service) SetSkills(library *skills.Library) { s.skills = library }

// SkillsOff combines team exclusions with the seat's optional allowlist.
func (s *Service) SkillsOff(team, member string) []string {
	t, ok := s.state.team(team)
	m, known := protocol.MemberByID(t.Members, member)
	if !ok || !known {
		if s.skills != nil {
			return s.skills.Restrict(t.SkillsOff, nil)
		}
		return t.SkillsOff
	}
	return s.memberSkillsOff(t, m)
}

func (s *Service) memberSkillsOff(team protocol.Team, member protocol.Member) []string {
	if member.Skills == nil || s.skills == nil {
		return team.SkillsOff
	}
	return s.skills.Restrict(team.SkillsOff, *member.Skills)
}

// skillsBrief describes only sets the seat may actually read.
func (s *Service) skillsBrief(team protocol.Team, member protocol.Member) string {
	if s.skills == nil {
		return ""
	}
	off := skills.Off(s.settings(), s.memberSkillsOff(team, member)...)
	result, err := s.skills.List(off)
	if err != nil {
		return ""
	}
	text := brief.Skills(result, true)
	if text == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(i18n.T("Skills"))
	b.WriteString("\n")
	b.WriteString(text)
	if memberCanDelegate(team, member.ID) {
		if hint := leadSkills(result); hint != "" {
			b.WriteString("\n")
			b.WriteString(hint)
		}
	}
	return b.String()
}

// leadSkills suggests to a lead the sets for the work it hands out: Elephant
// and Goldfish for designs and reviews, Spec Kit or OpenSpec for features of
// many steps, as far as they are on for the team.
func leadSkills(result protocol.SkillsResult) string {
	names := map[string]string{}
	for _, set := range result.Sets {
		if set.On {
			names[set.ID] = set.Name
		}
	}
	design := names["egm"]
	var features []string
	for _, id := range []string{"speckit", "openspec"} {
		if names[id] != "" {
			features = append(features, names[id])
		}
	}
	feature := strings.Join(features, i18n.T(" or "))
	switch {
	case design != "" && feature != "":
		return i18n.Tf("As the lead: designs and reviews go best with %s, whose skills run when the task names one, and features of many steps with %s. Tell the member you hand such a piece which skill to follow.", design, feature)
	case design != "":
		return i18n.Tf("As the lead: designs and reviews go best with %s, whose skills run when the task names one. Tell the member you hand such a piece which skill to follow.", design)
	case feature != "":
		return i18n.Tf("As the lead: features of many steps go best with %s. Tell the member you hand such a piece which skill to follow.", feature)
	}
	return ""
}
