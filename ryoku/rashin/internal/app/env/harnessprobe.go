package env

import (
	"ryoku-i18n"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func HarnessByID(harnesses []protocol.HarnessInfo, id string) (protocol.HarnessInfo, bool) {
	for _, harness := range harnesses {
		if harness.ID == id {
			return harness, true
		}
	}
	return protocol.HarnessInfo{}, false
}

// the model id a new session of the harness starts on, empty when neither the harness nor its settings said
func HarnessDefaultModel(info protocol.HarnessInfo) string {
	if info.Capabilities != nil && info.Capabilities.DefaultModel != "" {
		return info.Capabilities.DefaultModel
	}
	for _, fact := range info.Setup {
		if fact.Label == i18n.T("Model") && fact.Value != "" {
			return fact.Value
		}
	}
	return ""
}

func ModelName(capabilities *protocol.HarnessCapabilities, id string) string {
	if capabilities != nil {
		for _, choice := range capabilities.ModelList {
			if choice.ID == id && choice.Name != "" {
				return choice.Name
			}
		}
	}
	return id
}

// the choice that leaves the model to the harness, named when it is known
func HarnessDefaultLabel(info protocol.HarnessInfo) string {
	if model := HarnessDefaultModel(info); model != "" {
		return i18n.Tf("Harness default (%s)", ModelName(info.Capabilities, model))
	}
	return i18n.T("Harness default")
}

// starts a harness once in the sandbox without a prompt, one probe at a time, and keeps what it read
type HarnessProbe struct {
	Busy   bool
	Result string
	Failed bool
	// what the last probe read and the team it ran for, until the snapshot carries a list as new
	probed     protocol.HarnessInfo
	probedTeam string
}

// the harness as the snapshot has it in the user's own home, with what a probe just read there while the snapshot is older
func (p *HarnessProbe) Info(snapshot state.Snapshot, id string) protocol.HarnessInfo {
	info, _ := HarnessByID(snapshot.Harnesses, id)
	probed := p.probed.Capabilities
	if p.probed.ID == id && p.probedTeam == "" && probed != nil && (info.Capabilities == nil || info.Capabilities.Learned.Before(probed.Learned)) {
		info.Capabilities = probed
	}
	return info
}

// the harness as a team's home has it: Capabilities are the team's own, never the user's, so none for a team that has not run it or for no team at all
// Setup is read from the user's own config, so a team's home leaves it out too
func (p *HarnessProbe) TeamInfo(snapshot state.Snapshot, id, team string) protocol.HarnessInfo {
	info, _ := HarnessByID(snapshot.Harnesses, id)
	info.Capabilities, info.Setup = nil, nil
	if team == "" {
		return info
	}
	info.Capabilities = info.TeamCapabilities(team)
	probed := p.probed.TeamCapabilities(team)
	if p.probed.ID == id && p.probedTeam == team && probed != nil && (info.Capabilities == nil || info.Capabilities.Learned.Before(probed.Learned)) {
		info.Capabilities = probed
	}
	return info
}

func (p *HarnessProbe) Clear() {
	p.Result, p.Failed = "", false
}

// checking says what the harness took, reading only says when it offered no list
func (p *HarnessProbe) Run(e *Env, params protocol.HarnessProbeParams, name string, checking bool) {
	p.Busy, p.Result, p.Failed = true, "", false
	var result protocol.HarnessProbeResult
	e.Call(protocol.MHarnessesProbe, params, &result, func(err error) {
		p.Busy = false
		switch {
		case err != nil:
			p.Result, p.Failed = err.Error(), true
			return
		case !result.OK:
			p.Result, p.Failed = result.Message, true
			if p.Result == "" {
				p.Result = i18n.Tf("%s did not start.", name)
			}
		}
		p.probed, p.probedTeam = result.Harness, params.Team
		if !result.OK {
			return
		}
		// a team's probe reads the team's own list, which the user's never stands in for
		read := result.Harness
		if params.Team != "" {
			read.Capabilities, read.Setup = read.TeamCapabilities(params.Team), nil
		}
		capabilities := read.Capabilities
		switch {
		case checking:
			model := params.Model
			if model == "" {
				model = HarnessDefaultModel(read)
			}
			if model == "" {
				p.Result = i18n.Tf("%s started in the sandbox and took its default model.", name)
			} else {
				p.Result = i18n.Tf("%s started in the sandbox and took %s.", name, ModelName(capabilities, model))
			}
		case capabilities == nil || len(capabilities.ModelList) == 0:
			p.Result = i18n.Tf("%s started but offered no list of models.", name)
		}
	})
}
