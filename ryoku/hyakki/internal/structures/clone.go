package structures

import (
	"maps"
	"slices"

	"ryoku-rashin/internal/protocol"
)

func cloneTemplate(t protocol.StructureTemplate) protocol.StructureTemplate {
	t.JobFit = slices.Clone(t.JobFit)
	t.Notes = slices.Clone(t.Notes)
	t.Seats = slices.Clone(t.Seats)
	t.Example.Members = slices.Clone(t.Example.Members)
	for i, m := range t.Example.Members {
		m.Options = maps.Clone(m.Options)
		if m.Tools != nil {
			v := *m.Tools
			m.Tools = &v
		}
		if m.Budget != nil {
			v := *m.Budget
			m.Budget = &v
		}
		if m.Skills != nil {
			v := slices.Clone(*m.Skills)
			m.Skills = &v
		}
		if m.Runtime != nil {
			v := *m.Runtime
			v.Command = slices.Clone(v.Command)
			v.Env, v.SecretEnv = maps.Clone(v.Env), maps.Clone(v.SecretEnv)
			m.Runtime = &v
		}
		if m.Heartbeat != nil {
			v := *m.Heartbeat
			v.Modes = slices.Clone(v.Modes)
			m.Heartbeat = &v
		}
		t.Example.Members[i] = m
	}
	t.Example.Steps = slices.Clone(t.Example.Steps)
	for i := range t.Example.Steps {
		t.Example.Steps[i].Members = slices.Clone(t.Example.Steps[i].Members)
	}
	return t
}
