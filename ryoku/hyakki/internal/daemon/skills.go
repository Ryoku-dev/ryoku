package daemon

import (
	"encoding/json"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

// isSkillsMethod reports a method the skills serve. Every one only reads,
// so a team task's tools may call them, held to its team's sets.
func isSkillsMethod(method string) bool { return strings.HasPrefix(method, "skills.") }

// skillsCall serves one skills.* method for the caller the connection is: a
// session's tools see only the sets that session is given, anyone else every
// set that is on. Managed skills count as the set "managed", off while
// autolearn is.
func (d *Daemon) skillsCall(c *connection, method string, raw json.RawMessage) (any, error) {
	if d.deps.Skills == nil {
		return nil, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Skills are not available."))
	}
	off := skills.Off(d.Settings())
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

// skillsOff combines global restrictions with the seat's team and allowlist.
func (d *Daemon) skillsOff(info protocol.SessionInfo) []string {
	var team []string
	if info.Team != "" && d.deps.Teams != nil {
		team = d.deps.Teams.SkillsOff(info.Team, info.Member)
	}
	return skills.Off(d.Settings(), team...)
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

// skillDirs are the team's enabled skill roots, mounted read-only.
func (d *Daemon) skillDirs(info protocol.SessionInfo) []string {
	result, _ := d.sessionSkills(info)
	var dirs []string
	for _, set := range result.Sets {
		if !set.On {
			continue
		}
		if set.ID == protocol.ManagedSkillSet {
			dirs = append(dirs, paths.ManagedSkillsDir())
		} else {
			dirs = append(dirs, filepath.Join(result.Root, set.ID))
		}
	}
	return dirs
}
