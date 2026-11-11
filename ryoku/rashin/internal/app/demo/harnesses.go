package demo

import (
	"slices"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// the harnesses on the demo computer: two that ran and know their models, one of your own that never told its list, and one that was removed after it ran
func demoHarnesses(now time.Time) []protocol.HarnessInfo {
	return []protocol.HarnessInfo{
		{
			ID:        "codex",
			Name:      "Codex",
			Installed: true,
			Version:   "0.62.0",
			Default:   true,
			Capabilities: &protocol.HarnessCapabilities{
				LoadSession:  true,
				Images:       true,
				Context:      true,
				Models:       true,
				Modes:        []protocol.Choice{{ID: "default", Name: i18n.T("Default")}, {ID: "plan", Name: i18n.T("Plan")}},
				Started:      []protocol.HarnessFact{{Label: i18n.T("Model"), Value: "gpt-5.6-sol"}, {Label: i18n.T("Thinking"), Value: i18n.T("High")}},
				ModelList:    codexModels(),
				DefaultModel: "gpt-5.6-sol",
				Options:      demoOptions("high"),
				Learned:      now.Add(-3 * time.Hour),
			},
		},
		{
			ID:        "claude",
			Name:      "Claude Code",
			Installed: true,
			Version:   "2.1.3",
			Capabilities: &protocol.HarnessCapabilities{
				LoadSession: true,
				Images:      true,
				Models:      true,
				ModelList: []protocol.Choice{
					{ID: "opus", Name: "Opus 5.5", Description: i18n.T("For the hardest work")},
					{ID: "sonnet", Name: "Sonnet 5", Description: i18n.T("For everyday work")},
					{ID: "haiku", Name: "Haiku 5", Description: i18n.T("Fastest for quick answers")},
				},
				DefaultModel: "opus",
				Options: []protocol.SessionOption{{
					ID: "effort", Name: i18n.T("Effort"), Category: "thought_level", Current: "medium",
					Choices: []protocol.Choice{
						{ID: "low", Name: i18n.T("Low")}, {ID: "medium", Name: i18n.T("Medium")}, {ID: "high", Name: i18n.T("High")},
					},
				}},
				Learned: now.Add(-26 * time.Minute),
			},
			Setup: []protocol.HarnessFact{
				{Label: i18n.T("Model"), Value: "anthropic/claude-opus-5-5"},
				{Label: i18n.Tf("%s model", "smol"), Value: "anthropic/claude-haiku-5"},
			},
		},
		{
			ID:        "custom-review",
			Name:      i18n.T("Review account"),
			Installed: true,
			Custom:    true,
			Command:   []string{"codex-acp", "--profile", "review"},
		},
		{
			ID:   "gemini",
			Name: "Gemini CLI",
			Capabilities: &protocol.HarnessCapabilities{
				LoadSession: true,
				Models:      true,
				ModelList: []protocol.Choice{
					{ID: "gemini-3-pro", Name: "Gemini 3 Pro"},
					{ID: "gemini-3-flash", Name: "Gemini 3 Flash"},
				},
				DefaultModel: "gemini-3-pro",
				Learned:      now.AddDate(0, 0, -12),
			},
		},
	}
}

func codexModels() []protocol.Choice {
	return []protocol.Choice{
		{ID: "gpt-5.6-sol", Name: "GPT-5.6 Sol", Description: i18n.T("The most capable")},
		{ID: "gpt-5.6", Name: "GPT-5.6"},
		{ID: "gpt-5.6-mini", Name: "GPT-5.6 mini", Description: i18n.T("Quick and cheap")},
	}
}

// starts a harness the way a team task would, without a prompt: one not installed fails as the daemon says, one of your own learns the Codex list it runs on
func (s *server) probeHarness(p protocol.HarnessProbeParams) (protocol.HarnessProbeResult, error) {
	index := slices.IndexFunc(s.data.snapshot.Harnesses, func(harness protocol.HarnessInfo) bool { return harness.ID == p.ID })
	if index < 0 {
		return protocol.HarnessProbeResult{}, badRequest(i18n.Tf("No harness %s on this computer.", p.ID))
	}
	harness := &s.data.snapshot.Harnesses[index]
	if !harness.Installed {
		return protocol.HarnessProbeResult{Harness: *harness, Message: i18n.Tf("%s is not installed.", harness.Name)}, nil
	}
	if harness.Capabilities == nil {
		harness.Capabilities = &protocol.HarnessCapabilities{LoadSession: true, Models: true, ModelList: codexModels(), DefaultModel: "gpt-5.6-sol", Options: demoOptions("auto")}
	}
	capabilities := *harness.Capabilities
	capabilities.Learned = time.Now()
	harness.Capabilities = &capabilities
	model := p.Model
	if model == "" {
		model = capabilities.DefaultModel
	}
	listed := slices.IndexFunc(capabilities.ModelList, func(choice protocol.Choice) bool { return choice.ID == model })
	if listed < 0 {
		return protocol.HarnessProbeResult{Harness: *harness, Message: i18n.Tf("%s refused the model %s.", harness.Name, model)}, nil
	}
	return protocol.HarnessProbeResult{
		Harness: *harness, OK: true,
		Message: i18n.Tf("%s started in the sandbox and took %s.", harness.Name, capabilities.ModelList[listed].Name),
	}, nil
}

// the registry starts the harness marked default when neither the member, the teams defaults nor Agents name one
func (s *server) registryHarness() string {
	for _, harness := range s.data.snapshot.Harnesses {
		if harness.Default {
			return harness.ID
		}
	}
	return ""
}
