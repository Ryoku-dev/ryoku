package teams

import (
	"slices"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

// SetSkills hands the service the built-in skill sets its members are
// given; the daemon calls it once, before Bind.
func (s *Service) SetSkills(library *skills.Library) { s.skills = library }

// SkillsOff are the built-in skill sets a team's members are not given,
// besides those off for every model.
func (s *Service) SkillsOff(team string) []string {
	t, ok := s.state.team(team)
	if !ok {
		return nil
	}
	return t.SkillsOff
}

// skillsBrief is what a member hears of its team's built-in skills: the
// sets that are on for every model and for the team, where they lie and how
// to use them, and for the lead which sets fit the work it hands out. Empty
// when the team has none.
func (s *Service) skillsBrief(team protocol.Team, lead bool) string {
	if s.skills == nil {
		return ""
	}
	off := append(slices.Clone(s.settings().Skills.Off), team.SkillsOff...)
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
	if lead {
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
