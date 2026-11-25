package engine

import (
	"ryoku-rashin/internal/engine/tool"
)

// Ask asked for at most 4096 tokens a round; an answer longer than that is a
// document, not an answer.
const askMaxOutput = 4096

// modeProfiles holds what each mode is beyond its tools. Ask keeps its round
// caps (a change is read, cleared, made and checked, which takes more rounds
// than a look); the coding modes get room for real work, a subagent less
// than its parent, and a dream's lead fewer because it delegates.
var modeProfiles = map[tool.Mode]ModeProfile{
	tool.ModeAskRead:      {Mode: tool.ModeAskRead, ReadOnly: true, Rounds: 8, MaxOutput: askMaxOutput, Prompt: string(tool.ModeAskRead)},
	tool.ModeAskAct:       {Mode: tool.ModeAskAct, Rounds: 14, MaxOutput: askMaxOutput, Prompt: string(tool.ModeAskAct)},
	tool.ModeAgent:        {Mode: tool.ModeAgent, Rounds: 60, Prompt: string(tool.ModeAgent)},
	tool.ModePlan:         {Mode: tool.ModePlan, ReadOnly: true, Rounds: 60, Prompt: string(tool.ModePlan)},
	tool.ModeTeam:         {Mode: tool.ModeTeam, Rounds: 60, Prompt: string(tool.ModeTeam)},
	tool.ModeDreamManager: {Mode: tool.ModeDreamManager, Rounds: 40, Prompt: string(tool.ModeDreamManager)},
	tool.ModeSubagent:     {Mode: tool.ModeSubagent, Rounds: 30, Prompt: string(tool.ModeSubagent)},
}

// ProfileFor is a mode's profile; an unknown mode reads as agent.
func ProfileFor(mode tool.Mode) ModeProfile {
	if p, ok := modeProfiles[mode]; ok {
		return p
	}
	return modeProfiles[tool.ModeAgent]
}

// Full-class temperatures: nil leaves the provider's default, which the free
// remote routes are tuned for. Compact-class temperatures stay at or under
// CompactTemperature, lower where the work is lookup rather than writing.
var compactTemperatures = map[tool.Mode]float64{
	tool.ModeAskRead:      0.2,
	tool.ModeAskAct:       0.2,
	tool.ModeAgent:        CompactTemperature,
	tool.ModePlan:         CompactTemperature,
	tool.ModeTeam:         CompactTemperature,
	tool.ModeDreamManager: CompactTemperature,
	tool.ModeSubagent:     0.2,
}

// temperatureFor is the temperature a request of profile p asks for.
func temperatureFor(p tool.Profile) *float64 {
	if !p.Compact() {
		return nil
	}
	t, ok := compactTemperatures[p.Mode]
	if !ok || t > CompactTemperature {
		t = CompactTemperature
	}
	return &t
}

// isAsk reports the Ask modes.
func isAsk(mode tool.Mode) bool { return mode == tool.ModeAskRead || mode == tool.ModeAskAct }

// detectsUnexpectedStop reports the modes whose text-only stop may be a
// promise left undone. Ask's final answers are text-only by design, plan
// mode ends on its plan card, and a subagent is held to yield instead.
func detectsUnexpectedStop(mode tool.Mode) bool {
	switch mode {
	case tool.ModeAgent, tool.ModeTeam, tool.ModeDreamManager:
		return true
	}
	return false
}
