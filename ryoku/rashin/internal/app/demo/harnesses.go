package demo

import (
	"path/filepath"
	"slices"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// the harnesses on the demo computer: two that ran and know their models, one of your own that never told its list, and one that was removed after it ran;
// for teams Claude Code is signed in for three teams and not for Repo watch, Codex only for Mail desk, and Rashin knows no sign-in for the one of your own
func demoHarnesses(now time.Time) []protocol.HarnessInfo {
	home := func(harness, team string, days int) protocol.TeamHome {
		at := now.AddDate(0, 0, -days)
		return protocol.TeamHome{Team: team, Home: demoTeamsHome(harness, team), SignedIn: &at}
	}
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
			State: []string{"~/.codex"},
			Teams: &protocol.HarnessTeams{
				Login: []string{"codex", "login"}, Guarded: []string{".codex/auth.json", ".codex/config.toml"},
				Homes: []protocol.TeamHome{home("codex", demoTeamMail, 2)},
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
			State: []string{"~/.claude", "~/.claude.json"},
			Teams: &protocol.HarnessTeams{
				Login: []string{"claude", "/login"}, Guarded: []string{".claude/.credentials.json", ".claude.json", ".claude/settings.json"},
				Homes: []protocol.TeamHome{home("claude", demoTeamResearch, 2), home("claude", demoTeamSystem, 6), home("claude", demoTeamNews, 1)},
			},
		},
		{
			ID:        "custom-review",
			Name:      i18n.T("Review account"),
			Installed: true,
			Custom:    true,
			Command:   []string{"codex-acp", "--profile", "review"},
			Teams:     &protocol.HarnessTeams{Guarded: []string{".codex/config.toml"}},
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
			State: []string{"~/.gemini"},
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

// starts a harness the way a team task would, without a prompt: one not installed fails as the daemon says, one of your own learns the Codex list it runs on;
// a Check runs in a saved team's home, so one without a team, or for a team not signed in, does not start
func (s *server) probeHarness(p protocol.HarnessProbeParams) (protocol.HarnessProbeResult, error) {
	index := slices.IndexFunc(s.data.snapshot.Harnesses, func(harness protocol.HarnessInfo) bool { return harness.ID == p.ID })
	if index < 0 {
		return protocol.HarnessProbeResult{}, badRequest(i18n.Tf("No harness %s on this computer.", p.ID))
	}
	harness := &s.data.snapshot.Harnesses[index]
	if !harness.Installed {
		return protocol.HarnessProbeResult{Harness: *harness, Message: i18n.Tf("%s is not installed.", harness.Name)}, nil
	}
	// reading the list alone needs no team; asking for a model or an option is a Check
	checking := p.Model != "" || len(p.Options) > 0
	switch {
	case checking && p.Team == "":
		return protocol.HarnessProbeResult{Harness: *harness, Message: i18n.T("Save the team and sign it in first: a Check runs in the team's own home.")}, nil
	case p.Team != "" && harness.Teams != nil:
		if home, ok := harness.Teams.HomeOf(p.Team); !ok || home.SignedIn == nil {
			return protocol.HarnessProbeResult{Harness: *harness, Message: i18n.Tf("%s is not signed in for %s. Sign it in first.", harness.Name, s.teamName(p.Team))}, nil
		}
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

func demoTeamsHome(harness, team string) string {
	return filepath.Join(paths.HarnessHomes(), harness, team)
}

// what the daemon hands the terminal: in the team home as HOME, the sign-in or a shell, then the call that finishes it with the status
const demoTeamsScript = `cd "$1" && token=$2 && shift 2 && HOME="$PWD" "$@"; rashin call harnesses.teams_done "{\"token\":\"$token\",\"status\":$?}"`

func demoTeamsToken(harness, team string) string { return "demo-" + harness + ":" + team }

func (s *server) openTeamsHome(p protocol.HarnessTeamsOpenParams) (protocol.HarnessTeamsOpenResult, error) {
	index := slices.IndexFunc(s.data.snapshot.Harnesses, func(harness protocol.HarnessInfo) bool { return harness.ID == p.ID })
	if index < 0 {
		return protocol.HarnessTeamsOpenResult{}, badRequest(i18n.Tf("No harness %s on this computer.", p.ID))
	}
	harness := &s.data.snapshot.Harnesses[index]
	if !harness.Installed || harness.Teams == nil {
		return protocol.HarnessTeamsOpenResult{}, badRequest(i18n.Tf("%s is not installed.", harness.Name))
	}
	if s.data.teams.teamIndex(p.Team) < 0 {
		return protocol.HarnessTeamsOpenResult{}, badRequest(i18n.T("Pick the team to sign in for."))
	}
	if p.CopyFrom != "" {
		from, ok := harness.Teams.HomeOf(p.CopyFrom)
		if !p.Setup || p.CopyFrom == p.Team || !ok || from.SignedIn == nil {
			return protocol.HarnessTeamsOpenResult{}, badRequest(i18n.Tf("%s is not signed in for %s, so there is no config to start from.", harness.Name, s.teamName(p.CopyFrom)))
		}
	}
	teams := *harness.Teams
	teams.Homes = slices.Clone(teams.Homes)
	at := slices.IndexFunc(teams.Homes, func(home protocol.TeamHome) bool { return home.Team == p.Team })
	if at < 0 {
		teams.Homes = append(teams.Homes, protocol.TeamHome{Team: p.Team, Home: demoTeamsHome(harness.ID, p.Team)})
		at = len(teams.Homes) - 1
	}
	if teams.Homes[at].Opening {
		return protocol.HarnessTeamsOpenResult{}, badRequest(i18n.Tf("A terminal for %s in %s is open already. Finish there first.", harness.Name, s.teamName(p.Team)))
	}
	run := teams.Login
	if p.Setup || len(run) == 0 {
		run = []string{"sh"}
	}
	teams.Homes[at].Opening, teams.Homes[at].SignedIn = true, nil
	harness.Teams = &teams
	home := teams.Homes[at].Home
	argv := append([]string{"sh", "-c", demoTeamsScript, "sh", home, demoTeamsToken(harness.ID, p.Team)}, run...)
	return protocol.HarnessTeamsOpenResult{Argv: argv, Home: home}, nil
}

// the terminal closed: a sign-in or setup that ended well signs the team in to the harness
func (s *server) finishTeamsHome(p protocol.HarnessTeamsDoneParams) (protocol.HarnessTeamsDoneResult, error) {
	for index := range s.data.snapshot.Harnesses {
		harness := &s.data.snapshot.Harnesses[index]
		if harness.Teams == nil {
			continue
		}
		for at, home := range harness.Teams.Homes {
			if !home.Opening || demoTeamsToken(harness.ID, home.Team) != p.Token {
				continue
			}
			teams := *harness.Teams
			teams.Homes = slices.Clone(teams.Homes)
			teams.Homes[at].Opening = false
			harness.Teams = &teams
			name := s.teamName(home.Team)
			if p.Status != 0 {
				return protocol.HarnessTeamsDoneResult{Text: i18n.Tf("%s is not signed in for %s.", harness.Name, name)}, nil
			}
			now := time.Now()
			teams.Homes[at].SignedIn = &now
			s.continueSignedInDream(home.Team, harness.ID)
			return protocol.HarnessTeamsDoneResult{SignedIn: true, Text: i18n.Tf("%s is signed in for %s.", harness.Name, name)}, nil
		}
	}
	return protocol.HarnessTeamsDoneResult{}, badRequest(i18n.T("That sign-in is already finished."))
}
