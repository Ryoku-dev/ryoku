package rashin

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// Governance communicates a request, never an assignment or an action grant.
// A successful callback has parked the run; a refusal leaves it able to retry.
func governanceTools() []tool.Tool {
	live := func(env tool.Env) bool {
		_, err := agentRef(env)
		return err == nil
	}
	return []tool.Tool{
		&def{
			name: tool.ProposePlan, kind: protocol.ToolOther, show: "summary",
			title:    func() string { return i18n.T("Proposing a strategy") },
			describe: static("Propose your task's strategy to the Summoner with a summary and 1 to 32 ordered steps, then yield for their decision. Strategy acceptance never grants tools, access, budget or action permission."),
			brief:    "Propose a strategy to the Summoner, then yield; acceptance grants no extra permissions.",
			schema: fixed(newObject([]string{"summary", "steps"},
				str("summary", "A nonempty strategy summary, at most 4096 bytes."),
				field{"steps", map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}, "minItems": 1, "maxItems": 32, "description": "Ordered nonempty steps, each at most 2048 bytes."}},
			)),
			enabled: live,
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var plan struct {
					Summary string   `json:"summary"`
					Steps   []string `json:"steps"`
				}
				if err := args.decode(&plan); err != nil {
					return failed(err)
				}
				if strings.TrimSpace(plan.Summary) == "" || len(plan.Summary) > 4096 || len(plan.Steps) < 1 || len(plan.Steps) > 32 {
					return tool.Failed(i18n.T("Give a nonempty summary of at most 4096 bytes and 1 to 32 steps."))
				}
				for _, step := range plan.Steps {
					if strings.TrimSpace(step) == "" || len(step) > 2048 {
						return tool.Failed(i18n.T("Each plan step must be nonempty and at most 2048 bytes."))
					}
				}
				return governanceAnswer(ctx, env, protocol.MTeamsAgentPlan, func(ref protocol.AgentRef) any {
					return protocol.AgentPlanParams{AgentRef: ref, Summary: plan.Summary, Steps: plan.Steps}
				})
			},
		},
		&def{
			name: tool.RequestHelp, kind: protocol.ToolOther, show: "title",
			title:    func() string { return i18n.T("Requesting a peer's help") },
			describe: static("Request a peer's help by stable member ID or unique name, then yield. Hyakki routes the request through your common manager or the Summoner; this does not assign the peer or change your task's assignee."),
			brief:    "Request peer help through the common manager, without assigning the peer, then yield.",
			schema: fixed(newObject([]string{"member", "title", "brief"},
				str("member", "The requested peer's stable member ID or unique name."),
				str("title", "A nonempty request title, at most 200 characters."),
				str("brief", "What help is needed, nonempty and at most 16384 bytes."),
			)),
			enabled: live,
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var help struct {
					Member string `json:"member"`
					Title  string `json:"title"`
					Brief  string `json:"brief"`
				}
				if err := args.decode(&help); err != nil {
					return failed(err)
				}
				if strings.TrimSpace(help.Member) == "" {
					return tool.Failed(i18n.T("Name the peer whose help you need."))
				}
				if err := governanceMessage(help.Title, help.Brief); err != nil {
					return failed(err)
				}
				return governanceAnswer(ctx, env, protocol.MTeamsAgentHelp, func(ref protocol.AgentRef) any {
					return protocol.AgentHelpParams{AgentRef: ref, Member: help.Member, Title: help.Title, Brief: help.Brief}
				})
			},
		},
		&def{
			name: tool.Escalate, kind: protocol.ToolOther, show: "title",
			title:    func() string { return i18n.T("Escalating a blocker") },
			describe: static("Escalate a blocker or decision to your nearest actual manager, or the Summoner when you have none, then yield. Reporting lines select the recipient; rank never grants permission."),
			brief:    "Escalate a blocker to the nearest manager or Summoner, then yield.",
			schema: fixed(newObject([]string{"title", "detail"},
				str("title", "A nonempty blocker title, at most 200 characters."),
				str("detail", "The blocker and the decision needed, nonempty and at most 16384 bytes."),
			)),
			enabled: live,
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var escalation struct {
					Title  string `json:"title"`
					Detail string `json:"detail"`
				}
				if err := args.decode(&escalation); err != nil {
					return failed(err)
				}
				if err := governanceMessage(escalation.Title, escalation.Detail); err != nil {
					return failed(err)
				}
				return governanceAnswer(ctx, env, protocol.MTeamsAgentEscalate, func(ref protocol.AgentRef) any {
					return protocol.AgentEscalateParams{AgentRef: ref, Title: escalation.Title, Detail: escalation.Detail}
				})
			},
		},
	}
}

func governanceMessage(title, body string) error {
	if strings.TrimSpace(title) == "" || utf8.RuneCountInString(title) > 200 || strings.TrimSpace(body) == "" || len(body) > 16*1024 {
		return errors.New(i18n.T("Give a nonempty title of at most 200 characters and a nonempty body of at most 16384 bytes."))
	}
	return nil
}

func governanceAnswer(ctx context.Context, env tool.Env, method string, params func(protocol.AgentRef) any) tool.Result {
	result := teamAnswer(ctx, env, method, params)
	result.End = !result.IsError
	return result
}
