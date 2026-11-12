package shell

// the model picked for a harness on the Agents board is the one its next session starts on; the default keeps no entry
func (s *uiShell) setAgentModel(harness, model string) {
	kept := make(map[string]string, len(s.preferences.AgentModels)+1)
	for id, picked := range s.preferences.AgentModels {
		kept[id] = picked
	}
	if model == "" {
		delete(kept, harness)
	} else {
		kept[harness] = model
	}
	s.preferences.AgentModels = kept
	s.runtime.hub.Invalidate()
}
