package daemon

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/protocol"
)

// isSkillsMethod reports a method the built-in skills serve. Every one only
// reads, so a team task's agent may call them on its own socket, held to its
// team's sets.
func isSkillsMethod(method string) bool { return strings.HasPrefix(method, "skills.") }

// skillsCall serves one skills.* method for the caller the connection is: a
// team task's agent sees only its team's sets, anyone else every set that is
// on.
func (d *Daemon) skillsCall(c *connection, method string, raw json.RawMessage) (any, error) {
	if d.deps.Skills == nil {
		return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Skills are not available."))
	}
	off := d.Settings().Skills.Off
	if c.session != "" {
		info, _ := d.manager.Info(c.session)
		off = d.skillsOff(info)
	}
	switch method {
	case protocol.MSkillsList:
		return d.deps.Skills.List(off)
	case protocol.MSkillsGet:
		p, err := unmarshal[protocol.SkillParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Skills.Get(p, off)
	case protocol.MSkillsSearch:
		p, err := unmarshal[protocol.SkillsSearchParams](raw)
		if err != nil {
			return nil, err
		}
		return d.deps.Skills.Search(p, off)
	}
	return nil, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.Tf("Method %s is not supported.", method))
}

// skillsOff are the sets a session's agent is not given: those off for every
// model and, for a team task, those its team leaves out.
func (d *Daemon) skillsOff(info protocol.SessionInfo) []string {
	off := d.Settings().Skills.Off
	if info.Team != "" && d.deps.Teams != nil {
		off = append(slices.Clone(off), d.deps.Teams.SkillsOff(info.Team)...)
	}
	return off
}

// sessionSkills are the sets as a session's agent is given them, written
// out; false when there are none to give or they cannot be written out.
func (d *Daemon) sessionSkills(info protocol.SessionInfo) (protocol.SkillsResult, bool) {
	if d.deps.Skills == nil {
		return protocol.SkillsResult{}, false
	}
	result, err := d.deps.Skills.List(d.skillsOff(info))
	if err != nil {
		d.warn("built-in skills: %v", err)
		return protocol.SkillsResult{}, false
	}
	return result, true
}

// skillsBrief is what an agent session that is no team's is told of the
// built-in skills; a team task hears its team's through the team's brief.
func (d *Daemon) skillsBrief(info protocol.SessionInfo, tools bool) string {
	if info.Team != "" {
		return ""
	}
	result, ok := d.sessionSkills(info)
	if !ok {
		return ""
	}
	return brief.Skills(result, tools)
}

// skillDirs are the folders of the sets a team session's agent is given, for
// its sandbox to show read-only where they lie.
func (d *Daemon) skillDirs(info protocol.SessionInfo) []string {
	result, _ := d.sessionSkills(info)
	var dirs []string
	for _, set := range result.Sets {
		if set.On {
			dirs = append(dirs, filepath.Join(result.Root, set.ID))
		}
	}
	return dirs
}
