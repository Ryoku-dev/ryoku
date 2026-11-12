package main

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/mcp"
	"ryoku-rashin/internal/protocol"
)

// teamTools are the tools of an agent working a team task: its report, its
// questions to the user, and the credentials and mail it uses without ever
// seeing a value. Only a lead hands out pieces and reads how they went, and
// only a dream's manager shapes its team and presents the dream, which no
// report ends.
func teamTools(session mcpSession) []mcp.Tool {
	choices := map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"minItems":    2,
		"maxItems":    protocol.ApprovalOptionsMax,
		"description": "For a choice: 2 to 6 short, different answers the user picks one of. Leave it out for a yes or no question.",
	}
	ask := "Ask the user to decide before you do something that leaves this computer or cannot be undone, or to choose between ways to go on by giving 2 to 6 options. Never ask about a mail or a request: send_mail and http_request show the user the exact mail or request and ask by themselves. End your turn after asking; Rashin continues the task with their answer or the option they chose."
	if session.dream {
		ask = dreamAsk
	}
	tools := []mcp.Tool{
		{
			Name:        "report",
			Description: "Hand your report to the user: what you found or did, written for them, with sources or files. This ends your task. In a run of a routine where nothing changed since the last run, set quiet and say so in one line: the run ends without notifying the user.",
			Input: schema([]string{"title", "body"}, map[string]any{
				"title": stringProperty("One line saying what the report is."),
				"body":  stringProperty("The report in Markdown, written for the user, with sources."),
				"files": arrayProperty("Paths of files you made, inside the team's folder."),
				"quiet": map[string]any{"type": "boolean", "description": "Only for a run of a routine: nothing changed since the last run, so the user is not notified. Leave it off whenever there is anything the user should read."},
			}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Title string   `json:"title"`
					Body  string   `json:"body"`
					Files []string `json:"files"`
					Quiet bool     `json:"quiet"`
				}
				if err := toolArguments(raw, &args, "title", "body"); err != nil {
					return mcp.Failure(err)
				}
				files := make([]string, 0, len(args.Files))
				for _, file := range args.Files {
					files = append(files, session.path(file))
				}
				return teamAnswer(ctx, session, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: session.agent(), Title: args.Title, Body: args.Body, Files: files, Quiet: args.Quiet})
			},
		},
		{
			Name:        "ask_user",
			Description: ask,
			Input: schema([]string{"title"}, map[string]any{
				"title":    stringProperty("One line saying what you want to do, or what the user should choose."),
				"question": stringProperty("The question or questions in full, as the user should read them."),
				"detail":   stringProperty("Exactly what would happen, so the user can judge it."),
				"options":  choices,
			}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Title    string   `json:"title"`
					Question string   `json:"question"`
					Detail   string   `json:"detail"`
					Options  []string `json:"options"`
				}
				if err := toolArguments(raw, &args, "title"); err != nil {
					return mcp.Failure(err)
				}
				// the card shows one body, so the question leads what it says
				detail := strings.TrimSpace(strings.TrimSpace(args.Question) + "\n\n" + strings.TrimSpace(args.Detail))
				return teamAnswer(ctx, session, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: session.agent(), Title: args.Title, Detail: detail, Options: args.Options})
			},
		},
		{
			Name:        "credentials",
			Description: "List the credentials your team may use and where each may go. You never see their values.",
			Input:       schema([]string{}, map[string]any{}),
			ReadOnly:    true,
			Call: func(ctx context.Context, _ json.RawMessage) mcp.Result {
				if err := session.teamReady(); err != nil {
					return mcp.Failure(err)
				}
				var result protocol.AgentCredentialsResult
				return toolCall(ctx, protocol.MTeamsAgentCredentials, session.agent(), &result)
			},
		},
		{
			Name:        "http_request",
			Description: "Make one HTTPS request carrying one of your team's credentials. Rashin adds the credential itself, only for the hosts it may go to, and you never see its value. A first use on a host waits for the user's approval. With save_as, the whole answer goes into the team's folder at that path, for answers too long to read in the reply.",
			Input: schema([]string{"method", "url", "credential"}, map[string]any{
				"method":     map[string]any{"type": "string", "enum": []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"}, "description": "The HTTP method."},
				"url":        stringProperty("The https URL to call."),
				"credential": stringProperty("The name of the credential to send, as the credentials tool lists it."),
				"headers":    map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}, "description": "Other request headers, by name."},
				"body":       stringProperty("The request body."),
				"save_as":    stringProperty("A file path relative to the team's folder, such as reviews/issues.json, to save the whole answer to."),
			}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Method     string            `json:"method"`
					URL        string            `json:"url"`
					Credential string            `json:"credential"`
					Headers    map[string]string `json:"headers"`
					Body       string            `json:"body"`
					SaveAs     string            `json:"save_as"`
				}
				if err := toolArguments(raw, &args, "method", "url", "credential"); err != nil {
					return mcp.Failure(err)
				}
				if err := session.teamReady(); err != nil {
					return mcp.Failure(err)
				}
				var result protocol.AgentHTTPResult
				params := protocol.AgentHTTPParams{AgentRef: session.agent(), Method: strings.ToUpper(args.Method), URL: args.URL, Credential: args.Credential, Headers: args.Headers, Body: args.Body, SaveAs: args.SaveAs}
				if err := daemonCall(ctx, protocol.MTeamsAgentHTTP, params, &result); err != nil {
					return mcp.Failure(err)
				}
				if result.Waiting != "" {
					return mcp.Text(result.Waiting)
				}
				encoded, err := json.Marshal(result)
				if err != nil {
					return mcp.Failure(err)
				}
				return mcp.Text(string(encoded))
			},
		},
		{
			Name:        "send_mail",
			Description: "Send a mail from the user's mail account. Call it with the finished mail and do not ask with ask_user first: the user sees this exact mail and approves it before it goes, unless they trust the recipient for your team. End your turn after sending; Rashin continues you with the outcome.",
			Input: schema([]string{"to", "subject", "body"}, map[string]any{
				"to":      arrayProperty("Recipient addresses."),
				"cc":      arrayProperty("Addresses to copy."),
				"subject": stringProperty("The subject line."),
				"body":    stringProperty("The mail's text."),
			}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					To      []string `json:"to"`
					Cc      []string `json:"cc"`
					Subject string   `json:"subject"`
					Body    string   `json:"body"`
				}
				if err := toolArguments(raw, &args, "to", "subject", "body"); err != nil {
					return mcp.Failure(err)
				}
				return teamAnswer(ctx, session, protocol.MTeamsAgentMail, protocol.AgentMailParams{AgentRef: session.agent(), To: args.To, Cc: args.Cc, Subject: args.Subject, Body: args.Body})
			},
		},
		{
			Name:        "request_credential",
			Description: "Ask the user to add a credential your team needs, saying why and where it would be sent. They type the value into Rashin; you never see it. End your turn after asking.",
			Input: schema([]string{"name", "reason", "hosts"}, map[string]any{
				"name":   stringProperty("The credential's name in capitals, such as GITHUB_TOKEN."),
				"reason": stringProperty("Why the team needs it."),
				"hosts":  arrayProperty("The hosts it would be sent to, such as api.github.com."),
			}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Name   string   `json:"name"`
					Reason string   `json:"reason"`
					Hosts  []string `json:"hosts"`
				}
				if err := toolArguments(raw, &args, "name", "reason", "hosts"); err != nil {
					return mcp.Failure(err)
				}
				return teamAnswer(ctx, session, protocol.MTeamsAgentRequest, protocol.AgentCredentialRequestParams{AgentRef: session.agent(), Name: args.Name, Reason: args.Reason, Hosts: args.Hosts})
			},
		},
	}
	if session.dream {
		tools = slices.DeleteFunc(tools, func(tool mcp.Tool) bool { return tool.Name == "report" })
	}
	if !session.lead {
		return tools
	}
	tools = append(tools,
		mcp.Tool{
			Name:        "delegate",
			Description: "Give one piece of your task to a member of your team, by name. They work on it in their own session and report back to you. End your turn after delegating; Rashin continues you with their reports.",
			Input: schema([]string{"member", "title", "brief"}, map[string]any{
				"member": stringProperty("The member's name."),
				"title":  stringProperty("One line saying what the piece is."),
				"brief":  stringProperty("What the member should do and hand back."),
			}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Member string `json:"member"`
					Title  string `json:"title"`
					Brief  string `json:"brief"`
				}
				if err := toolArguments(raw, &args, "member", "title", "brief"); err != nil {
					return mcp.Failure(err)
				}
				return teamAnswer(ctx, session, protocol.MTeamsAgentDelegate, protocol.AgentDelegateParams{AgentRef: session.agent(), Member: args.Member, Title: args.Title, Brief: args.Brief})
			},
		},
		mcp.Tool{
			Name:        "team_status",
			Description: "See the pieces you handed out: each one's state and report.",
			Input:       schema([]string{}, map[string]any{}),
			ReadOnly:    true,
			Call: func(ctx context.Context, _ json.RawMessage) mcp.Result {
				if err := session.teamReady(); err != nil {
					return mcp.Failure(err)
				}
				var result protocol.AgentStatusResult
				if err := daemonCall(ctx, protocol.MTeamsAgentStatus, session.agent(), &result); err != nil {
					return mcp.Failure(err)
				}
				return mcp.Text(piecesStatus(result.Tasks))
			},
		},
	)
	if session.dream {
		tools = append(tools, dreamTools(session)...)
	}
	return tools
}

// teamAnswer runs one team method that answers in words and returns them
func teamAnswer(ctx context.Context, session mcpSession, method string, params any) mcp.Result {
	if err := session.teamReady(); err != nil {
		return mcp.Failure(err)
	}
	var answer protocol.AgentAnswer
	if err := daemonCall(ctx, method, params, &answer); err != nil {
		return mcp.Failure(err)
	}
	return mcp.Text(answer.Text)
}

// piecesStatus says how each handed-out piece stands, oldest first, with its
// report or why it ended short
func piecesStatus(pieces []protocol.TeamTask) string {
	if len(pieces) == 0 {
		return "You have not handed out any piece of this task."
	}
	var b strings.Builder
	for i := len(pieces) - 1; i >= 0; i-- {
		piece := pieces[i]
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(piece.Title)
		b.WriteString(": ")
		b.WriteString(piece.Status)
		switch {
		case piece.Report != nil:
			b.WriteString("\n")
			b.WriteString(piece.Report.Body)
		case piece.Reason != "":
			b.WriteString("\n")
			b.WriteString(piece.Reason)
		}
	}
	return b.String()
}

// agent is how the team methods know the calling run
func (session mcpSession) agent() protocol.AgentRef {
	return protocol.AgentRef{Session: session.id, Token: session.token}
}

// teamReady refuses a call from a server that was not started for a team
// task
func (session mcpSession) teamReady() error {
	if session.id == "" || session.token == "" {
		return errors.New(i18n.T("This server was not started for a team task."))
	}
	return nil
}

func arrayProperty(description string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": description}
}

// Portions adapted from paperclipai/paperclip skills/paperclip/SKILL.md
// "The Heartbeat Procedure", human input questions @ 4e519bd4286e (MIT,
// Copyright (c) 2025 Paperclip AI). Modified by Ryoku: one ask_user card to
// the dream's CEO, about design or task only, never a confirmation.
const dreamAsk = "Ask the CEO, the user, a design or task question; never ask for confirmation or permission, as a dream runs without them. Your first turn always asks one round, even when the statement looks complete: then share your plan and ask what to change. Put every question that does not depend on another's answer in one card, written out in full in its question field, rather than one card each; give 2 to 6 options when the CEO should pick between ways to go on. End your turn after asking; Rashin continues you with their answer."

// dreamTools are the tools only a dream's manager has: hiring and releasing
// members, saying where the dream stands and presenting it
func dreamTools(session mcpSession) []mcp.Tool {
	phases := []string{protocol.DreamInterviewing, protocol.DreamDesigning, protocol.DreamPlanning, protocol.DreamBuilding, protocol.DreamTesting}
	return []mcp.Tool{
		{
			Name: "hire",
			// Portions adapted from paperclipai/paperclip
			// skills/paperclip-create-agent/references/baseline-role-guide.md @ 4e519bd4286e
			// (MIT, Copyright (c) 2025 Paperclip AI). Modified by Ryoku: a dream member on a
			// harness the dream is signed in to.
			Description: "Add a member to your dream's team, for a responsibility it can own end to end. Its instructions are a short paragraph on its identity and the responsibility it owns, not an operating manual: Rashin gives every member the rules of the team. A small hire is not an incomplete hire. Leave harness and model out to use your own engine; the dream must be signed in to the harness, and a refusal names the ones it is.",
			Input: schema([]string{"name", "title", "instructions"}, map[string]any{
				"name":         stringProperty("The member's name, such as Ada."),
				"title":        stringProperty("Its role in a few words, such as Firmware engineer."),
				"instructions": stringProperty("A short paragraph: who the member is and the responsibility it owns."),
				"harness":      stringProperty("The harness id to run it on, empty for yours."),
				"model":        stringProperty("The model id, empty for the harness's default or yours."),
				"why":          stringProperty("One sentence for the user on why the dream needs this member."),
			}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Name         string `json:"name"`
					Title        string `json:"title"`
					Instructions string `json:"instructions"`
					Harness      string `json:"harness"`
					Model        string `json:"model"`
					Why          string `json:"why"`
				}
				if err := toolArguments(raw, &args, "name", "title", "instructions"); err != nil {
					return mcp.Failure(err)
				}
				if err := session.teamReady(); err != nil {
					return mcp.Failure(err)
				}
				var hired protocol.Member
				params := protocol.AgentHireParams{AgentRef: session.agent(), Name: args.Name, Title: args.Title, Instructions: args.Instructions, Harness: args.Harness, Model: args.Model, Why: args.Why}
				if err := daemonCall(ctx, protocol.MTeamsAgentHire, params, &hired); err != nil {
					return mcp.Failure(err)
				}
				return mcp.Text(hired.Name + " is on the team. Hand them a piece with delegate.")
			},
		},
		{
			Name:        "release",
			Description: "Let a member go once its work is in, so the team stays small. A member still working on a piece stays.",
			Input: schema([]string{"member"}, map[string]any{
				"member": stringProperty("The member's name."),
				"why":    stringProperty("One sentence for the user on why."),
			}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Member string `json:"member"`
					Why    string `json:"why"`
				}
				if err := toolArguments(raw, &args, "member"); err != nil {
					return mcp.Failure(err)
				}
				return teamAnswer(ctx, session, protocol.MTeamsAgentRelease, protocol.AgentReleaseParams{AgentRef: session.agent(), Member: args.Member, Why: args.Why})
			},
		},
		{
			Name:        "dream_status",
			Description: "Tell the user where the dream stands: its phase, one sentence, your estimate of progress and, once the interview settled it, the brief. Call it whenever the phase changes and after each meaningful step. Presenting is reached with present, and done only when the user accepts.",
			Input: schema([]string{"phase", "summary"}, map[string]any{
				"phase":    map[string]any{"type": "string", "enum": phases, "description": "The phase the dream is in."},
				"summary":  stringProperty("One sentence on where the dream stands, for the user."),
				"progress": map[string]any{"type": "integer", "minimum": 0, "maximum": 100, "description": "Your estimate of how far the dream is, 0 to 100."},
				"brief":    stringProperty("What the interview settled: what will be built, for whom, and how it will be checked."),
			}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Phase    string `json:"phase"`
					Summary  string `json:"summary"`
					Progress *int   `json:"progress"`
					Brief    string `json:"brief"`
				}
				if err := toolArguments(raw, &args, "phase", "summary"); err != nil {
					return mcp.Failure(err)
				}
				return teamAnswer(ctx, session, protocol.MTeamsAgentDream, protocol.AgentDreamParams{AgentRef: session.agent(), Phase: args.Phase, Summary: args.Summary, Progress: args.Progress, Brief: args.Brief})
			},
		},
		{
			Name:        "present",
			Description: "Present the finished dream to the user once it is built, tested and reviewed: what was built, how to use it, the tests you ran and what they showed, and the files to look at. The user accepts it, which ends the dream, or sends notes, which continue you. End your turn after presenting.",
			Input: schema([]string{"title", "body"}, map[string]any{
				"title": stringProperty("One line naming what was built."),
				"body":  stringProperty("In Markdown: what was built, how to use it, and the evidence it works."),
				"tests": stringProperty("The tests you ran and what they showed."),
				"files": arrayProperty("Paths of the files to look at, inside the dream's folder."),
			}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Title string   `json:"title"`
					Body  string   `json:"body"`
					Tests string   `json:"tests"`
					Files []string `json:"files"`
				}
				if err := toolArguments(raw, &args, "title", "body"); err != nil {
					return mcp.Failure(err)
				}
				files := make([]string, 0, len(args.Files))
				for _, file := range args.Files {
					files = append(files, session.path(file))
				}
				return teamAnswer(ctx, session, protocol.MTeamsAgentPresent, protocol.AgentPresentParams{AgentRef: session.agent(), Title: args.Title, Body: args.Body, Tests: args.Tests, Files: files})
			},
		},
	}
}
