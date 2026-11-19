package rashin

import (
	"context"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// dreamTools are the tools only a dream's manager has: hiring and releasing
// members, saying where the dream stands and presenting it.
func dreamTools() []tool.Tool {
	manager := func(env tool.Env) bool {
		return env.Context.Team != nil && env.Context.Team.Lead && env.Context.Team.Dream
	}
	phases := []string{protocol.DreamInterviewing, protocol.DreamDesigning, protocol.DreamPlanning, protocol.DreamBuilding, protocol.DreamTesting}
	presentation := newObject([]string{"title", "body"},
		str("title", "One line naming what was built."),
		str("body", "In Markdown: what was built, how to use it, and the evidence it works."),
		str("tests", "The tests you ran and what they showed."),
		list("files", "Paths of the files to look at, inside the dream's folder."),
		list("sources", "Source URLs, for a research report."),
	)
	return []tool.Tool{
		&def{
			name: tool.Hire, kind: protocol.ToolOther, show: "name",
			title: func() string { return i18n.T("Hiring a member") },
			// Portions adapted from paperclipai/paperclip
			// skills/paperclip-create-agent/references/baseline-role-guide.md @ 4e519bd4286e
			// (MIT, Copyright (c) 2025 Paperclip AI). Modified by Ryoku: a dream
			// member on a model of the manager's choosing.
			describe: static("Add a member to your dream's team, for a responsibility it can own end to end. Its instructions are a short paragraph on its identity and the responsibility it owns, not an operating manual: Rashin gives every member the rules of the team. A small hire is not an incomplete hire. Leave model out to give it your own model."),
			brief:    "Add a team member for one responsibility it owns end to end.",
			schema: fixed(newObject([]string{"name", "title", "instructions"},
				str("name", "The member's name, such as Ada."),
				str("title", "Its role in a few words, such as Firmware engineer."),
				str("instructions", "A short paragraph: who the member is and the responsibility it owns."),
				str("model", "A model id such as auto, auto:smart or provider/model; empty for yours."),
				str("why", "One sentence for the user on why the dream needs this member."),
			)),
			enabled: manager,
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var hire struct {
					Name         string `json:"name"`
					Title        string `json:"title"`
					Instructions string `json:"instructions"`
					Model        string `json:"model"`
					Why          string `json:"why"`
				}
				if err := args.decode(&hire); err != nil {
					return failed(err)
				}
				ref, err := agentRef(env)
				if err != nil {
					return failed(err)
				}
				var hired protocol.Member
				params := protocol.AgentHireParams{AgentRef: ref, Name: hire.Name, Title: hire.Title, Instructions: hire.Instructions, Model: hire.Model, Why: hire.Why}
				if err := env.Host.Call(ctx, protocol.MTeamsAgentHire, params, &hired); err != nil {
					return failed(err)
				}
				return tool.Result{Text: i18n.Tf("%s is on the team. Hand them a piece with delegate.", hired.Name)}
			},
		},
		&def{
			name: tool.Release, kind: protocol.ToolOther, show: "member",
			title:    func() string { return i18n.T("Releasing a member") },
			describe: static("Let a member go once its work is in, so the team stays small. A member still working on a piece stays."),
			brief:    "Let a member go once its work is in.",
			schema: fixed(newObject([]string{"member"},
				str("member", "The member's name."),
				str("why", "One sentence for the user on why."),
			)),
			enabled: manager,
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var release struct {
					Member string `json:"member"`
					Why    string `json:"why"`
				}
				if err := args.decode(&release); err != nil {
					return failed(err)
				}
				return teamAnswer(ctx, env, protocol.MTeamsAgentRelease, func(ref protocol.AgentRef) any {
					return protocol.AgentReleaseParams{AgentRef: ref, Member: release.Member, Why: release.Why}
				})
			},
		},
		&def{
			name: tool.DreamStatus, kind: protocol.ToolOther, show: "summary",
			title:    func() string { return i18n.T("Updating the dream") },
			describe: static("Tell the user where the dream stands: its phase, one sentence and, once the interview settled it, the brief. Call it whenever the phase changes and after each meaningful step. Presenting is reached with present, and done only when the user accepts."),
			brief:    "Tell the user the dream's phase and where it stands, in one sentence.",
			schema: fixed(newObject([]string{"phase", "summary"},
				enum("phase", "The phase the dream is in.", phases...),
				str("summary", "One sentence on where the dream stands, for the user."),
				str("brief", "What the interview settled: what will be built, for whom, and how it will be checked."),
			)),
			enabled: manager,
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var status struct {
					Phase   string `json:"phase"`
					Summary string `json:"summary"`
					Brief   string `json:"brief"`
				}
				if err := args.decode(&status); err != nil {
					return failed(err)
				}
				return teamAnswer(ctx, env, protocol.MTeamsAgentDream, func(ref protocol.AgentRef) any {
					return protocol.AgentDreamParams{AgentRef: ref, Phase: status.Phase, Summary: status.Summary, Brief: status.Brief}
				})
			},
		},
		&def{
			name: tool.Present, kind: protocol.ToolOther, show: "title",
			title:    func() string { return i18n.T("Presenting the dream") },
			describe: static("Present the finished dream to the user once it is built, tested and reviewed: what was built, how to use it, the tests you ran and what they showed, and the files to look at. The user accepts it, which ends the dream, or sends notes, which continue you. End your turn after presenting."),
			brief:    "Present the finished, tested dream to the user, then end your turn.",
			schema: func(p tool.Profile) object {
				// a compact schema carries one list at most
				if p.Compact() {
					return presentation.without("sources")
				}
				return presentation
			},
			enabled: manager,
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var present struct {
					Title   string   `json:"title"`
					Body    string   `json:"body"`
					Tests   string   `json:"tests"`
					Files   []string `json:"files"`
					Sources []string `json:"sources"`
				}
				if err := args.decode(&present); err != nil {
					return failed(err)
				}
				return teamAnswer(ctx, env, protocol.MTeamsAgentPresent, func(ref protocol.AgentRef) any {
					return protocol.AgentPresentParams{AgentRef: ref, Title: present.Title, Body: present.Body, Tests: present.Tests, Files: inFolder(env, present.Files), Sources: present.Sources}
				})
			},
		},
	}
}
