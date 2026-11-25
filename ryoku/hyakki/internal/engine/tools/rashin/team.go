package rashin

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// teamTaskTools keeps reports, requests and delegation bound to one live run.
// Reporting lines select who may receive work; rank never grants permission.
func teamTaskTools() []tool.Tool {
	onTeam := func(env tool.Env) bool { return env.Context.Team != nil }
	delegates := func(env tool.Env) bool { return env.Context.Team != nil && env.Context.Team.Delegate }
	return []tool.Tool{
		&def{
			name: tool.Report, kind: protocol.ToolOther, show: "title",
			title:    func() string { return i18n.T("Handing in the report") },
			describe: static("Hand your report to the user: what you found or did, written for them, with sources or files. This ends your task. In a run of a routine where nothing changed since the last run, set quiet and say so in one line: the run ends without notifying the user."),
			brief:    "Hand your finished report to the user; this ends your task.",
			schema: fixed(newObject([]string{"title", "body"},
				str("title", "One line saying what the report is."),
				str("body", "The report in Markdown, written for the user, with sources."),
				list("files", "Paths of files you made, inside the team's folder."),
				boolean("quiet", "Only for a run of a routine: nothing changed since the last run, so the user is not notified. Leave it off whenever there is anything the user should read."),
			)),
			enabled: func(env tool.Env) bool { return onTeam(env) && !env.Context.Team.Dream },
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var report struct {
					Title string   `json:"title"`
					Body  string   `json:"body"`
					Files []string `json:"files"`
					Quiet bool     `json:"quiet"`
				}
				if err := args.decode(&report); err != nil {
					return failed(err)
				}
				return governanceAnswer(ctx, env, protocol.MTeamsAgentReport, func(ref protocol.AgentRef) any {
					return protocol.AgentReportParams{AgentRef: ref, Title: report.Title, Body: report.Body, Files: inFolder(env, report.Files), Quiet: report.Quiet}
				})
			},
		},
		&def{
			name: tool.AskUser, kind: protocol.ToolOther, show: "title",
			title: func() string { return i18n.T("Asking the user") },
			describe: func(p tool.Profile) string {
				if p.Mode == tool.ModeDreamManager {
					return dreamAsk
				}
				return teamAsk
			},
			schema: fixed(newObject([]string{},
				str("title", "One line saying what you want to do, or what the user should choose. Taken from the question when left out."),
				str("question", "The question or questions in full, as the user should read them."),
				str("detail", "Exactly what would happen, so the user can judge it."),
				field{"options", map[string]any{
					"type": "array", "items": map[string]any{"type": "string"}, "minItems": 2, "maxItems": protocol.ApprovalOptionsMax,
					"description": "For a choice: 2 to 6 short, different answers the user picks one of. Leave it out for a yes or no question.",
				}},
			)),
			enabled: onTeam,
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var asked askArguments
				if err := args.decode(&asked); err != nil {
					return failed(err)
				}
				title, detail, err := asked.card()
				if err != nil {
					return failed(err)
				}
				return teamAnswer(ctx, env, protocol.MTeamsAgentAsk, func(ref protocol.AgentRef) any {
					return protocol.AgentAskParams{AgentRef: ref, Title: title, Detail: detail, Options: asked.Options}
				})
			},
		},
		&def{
			name: tool.Credentials, kind: protocol.ToolRead,
			title:    func() string { return i18n.T("Reading the team's credentials") },
			describe: static("List the credentials your team may use and where each may go. You never see their values."),
			schema:   fixed(newObject(nil)),
			enabled:  onTeam,
			run: func(ctx context.Context, _ arguments, env tool.Env) tool.Result {
				ref, err := agentRef(env)
				if err != nil {
					return failed(err)
				}
				var result protocol.AgentCredentialsResult
				if err := env.Host.Call(ctx, protocol.MTeamsAgentCredentials, ref, &result); err != nil {
					return failed(err)
				}
				return answer(env, toon(&result))
			},
		},
		&def{
			name: tool.HTTPRequest, kind: protocol.ToolFetch, show: "url",
			title:    func() string { return i18n.T("Calling a service") },
			describe: static("Make one HTTPS request carrying one of your team's credentials. Rashin adds the credential itself, only for the hosts it may go to, and you never see its value. A first use on a host waits for the user's approval. With save_as, the whole answer goes into the team's folder at that path, for answers too long to read in the reply."),
			schema: fixed(newObject([]string{"method", "url", "credential"},
				enum("method", "The HTTP method.", "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"),
				str("url", "The https URL to call."),
				str("credential", "The name of the credential to send, as the credentials tool lists it."),
				field{"headers", map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}, "description": "Other request headers, by name."}},
				str("body", "The request body."),
				str("save_as", "A file path relative to the team's folder, such as reviews/issues.json, to save the whole answer to."),
			)),
			enabled: onTeam,
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var request struct {
					Method     string            `json:"method"`
					URL        string            `json:"url"`
					Credential string            `json:"credential"`
					Headers    map[string]string `json:"headers"`
					Body       string            `json:"body"`
					SaveAs     string            `json:"save_as"`
				}
				if err := args.decode(&request); err != nil {
					return failed(err)
				}
				ref, err := agentRef(env)
				if err != nil {
					return failed(err)
				}
				params := protocol.AgentHTTPParams{AgentRef: ref, Method: strings.ToUpper(request.Method), URL: request.URL, Credential: request.Credential, Headers: request.Headers, Body: request.Body, SaveAs: request.SaveAs}
				var result protocol.AgentHTTPResult
				if err := env.Host.Call(ctx, protocol.MTeamsAgentHTTP, params, &result); err != nil {
					return failed(err)
				}
				if result.Waiting != "" {
					return tool.Result{Text: result.Waiting}
				}
				encoded, err := json.Marshal(result)
				if err != nil {
					return failed(err)
				}
				return answer(env, string(encoded))
			},
		},
		&def{
			name: tool.SendMail, kind: protocol.ToolOther, show: "subject",
			title:    func() string { return i18n.T("Sending a mail") },
			describe: static("Send a mail from the user's mail account. Call it with the finished mail and do not ask with ask_user first: the user sees this exact mail and approves it before it goes, unless they trust the recipient for your team. End your turn after sending; Rashin continues you with the outcome."),
			schema: fixed(newObject([]string{"to", "subject", "body"},
				list("to", "Recipient addresses."),
				list("cc", "Addresses to copy."),
				str("subject", "The subject line."),
				str("body", "The mail's text."),
			)),
			enabled: onTeam,
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var mail struct {
					To      []string `json:"to"`
					Cc      []string `json:"cc"`
					Subject string   `json:"subject"`
					Body    string   `json:"body"`
				}
				if err := args.decode(&mail); err != nil {
					return failed(err)
				}
				return teamAnswer(ctx, env, protocol.MTeamsAgentMail, func(ref protocol.AgentRef) any {
					return protocol.AgentMailParams{AgentRef: ref, To: mail.To, Cc: mail.Cc, Subject: mail.Subject, Body: mail.Body}
				})
			},
		},
		&def{
			name: tool.RequestCredential, kind: protocol.ToolOther, show: "name",
			title:    func() string { return i18n.T("Asking for a credential") },
			describe: static("Ask the user to add a credential your team needs, saying why and where it would be sent. They type the value into Rashin; you never see it. End your turn after asking."),
			schema: fixed(newObject([]string{"name", "reason", "hosts"},
				str("name", "The credential's name in capitals, such as GITHUB_TOKEN."),
				str("reason", "Why the team needs it."),
				list("hosts", "The hosts it would be sent to, such as api.github.com."),
			)),
			enabled: onTeam,
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var request struct {
					Name   string   `json:"name"`
					Reason string   `json:"reason"`
					Hosts  []string `json:"hosts"`
				}
				if err := args.decode(&request); err != nil {
					return failed(err)
				}
				return teamAnswer(ctx, env, protocol.MTeamsAgentRequest, func(ref protocol.AgentRef) any {
					return protocol.AgentCredentialRequestParams{AgentRef: ref, Name: request.Name, Reason: request.Reason, Hosts: request.Hosts}
				})
			},
		},
		&def{
			name: tool.RequestSetup, kind: protocol.ToolOther, show: "title",
			title:    func() string { return i18n.T("Asking the user to set something up") },
			describe: static("Ask the user to finish setup you cannot do. Give clear ordered steps and why they are needed. End your turn after asking; Rashin continues you when the user finishes or skips."),
			schema: fixed(newObject([]string{"title", "steps", "why"},
				str("title", "What needs setting up."),
				list("steps", "Ordered steps the user can follow."),
				str("why", "Why this setup is needed."),
				str("check", "An optional way to check it worked."),
			)),
			enabled: func(env tool.Env) bool { return onTeam(env) && env.Context.Team.SetupCards },
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var setup struct {
					Title string   `json:"title"`
					Steps []string `json:"steps"`
					Why   string   `json:"why"`
					Check string   `json:"check"`
				}
				if err := args.decode(&setup); err != nil {
					return failed(err)
				}
				return teamAnswer(ctx, env, protocol.MTeamsAgentSetup, func(ref protocol.AgentRef) any {
					return protocol.AgentSetupParams{AgentRef: ref, Title: setup.Title, Steps: setup.Steps, Why: setup.Why, Check: setup.Check}
				})
			},
		},
		&def{
			name: tool.RequestHands, kind: protocol.ToolOther, show: "goal",
			title: func() string { return i18n.T("Asking the user to let Hands work") },
			describe: static("Ask the user to let Hyakki's Hands work toward a goal on screen: in the nested desktop Hyakki owns, or in one live window the user picks (by its id from the window picker). " +
				"You give the goal only; Hands acts by itself, and you get no stream of its actions. Never ask for the user's whole desktop. End your turn after asking; Hyakki continues you with the answer."),
			schema: fixed(newObject([]string{"goal", "scope"},
				str("goal", "What Hands should get done, in one line."),
				enum("scope", "Where Hands works.", protocol.OniHandsScopeNested, protocol.OniHandsScopeWindow),
				str("window", "With scope window only: the window's id."),
			)),
			enabled: func(env tool.Env) bool { return onTeam(env) && env.Context.Team.Hands },
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var asked struct {
					Goal   string `json:"goal"`
					Scope  string `json:"scope"`
					Window string `json:"window"`
				}
				if err := args.decode(&asked); err != nil {
					return failed(err)
				}
				return teamAnswer(ctx, env, protocol.MTeamsAgentHands, func(ref protocol.AgentRef) any {
					return protocol.AgentHandsParams{AgentRef: ref, Goal: asked.Goal, Scope: asked.Scope, Window: asked.Window}
				})
			},
		},
		&def{
			name: tool.Delegate, kind: protocol.ToolOther, show: "title",
			title:    func() string { return i18n.T("Handing out a piece") },
			describe: static("Give one piece of your task to a permitted report by ID or name. Reporting lines and delegation depth bound who may receive it. They work in their own session and report back to you. End your turn; Hyakki continues you with their report."),
			brief:    "Delegate one piece downward through your reporting lines, then end your turn.",
			schema: fixed(newObject([]string{"member", "title", "brief"},
				str("member", "The report's stable member ID or unique name."),
				str("title", "One line saying what the piece is."),
				str("brief", "What the member should do and hand back."),
			)),
			enabled: delegates,
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				var piece struct {
					Member string `json:"member"`
					Title  string `json:"title"`
					Brief  string `json:"brief"`
				}
				if err := args.decode(&piece); err != nil {
					return failed(err)
				}
				return governanceAnswer(ctx, env, protocol.MTeamsAgentDelegate, func(ref protocol.AgentRef) any {
					return protocol.AgentDelegateParams{AgentRef: ref, Member: piece.Member, Title: piece.Title, Brief: piece.Brief}
				})
			},
		},
		&def{
			name: tool.TeamStatus, kind: protocol.ToolRead,
			title:    func() string { return i18n.T("Reading the team's work and reporting lines") },
			describe: static("Read your task's preserved goal and because-chain, the team's safe member/capability directory, and the state and reports of work you delegated. For a Dream, also read its current phase, brief, presentation and the CEO's presentation verdict and notes."),
			brief:    "Read goal, lineage, safe member discovery, delegated reports and the Dream's phase and CEO feedback.",
			schema:   fixed(newObject(nil)),
			enabled:  onTeam,
			run: func(ctx context.Context, _ arguments, env tool.Env) tool.Result {
				ref, err := agentRef(env)
				if err != nil {
					return failed(err)
				}
				var result protocol.AgentStatusResult
				if err := env.Host.Call(ctx, protocol.MTeamsAgentStatus, ref, &result); err != nil {
					return failed(err)
				}
				directory := struct {
					Context *protocol.TeamTaskContext       `json:"context,omitempty"`
					Members []protocol.MemberDirectoryEntry `json:"members,omitempty"`
					Dream   *protocol.Dream                 `json:"dream,omitempty"`
				}{Context: result.Context, Members: result.Members, Dream: result.Dream}
				return answer(env, toon(&directory)+"\n\n"+piecesStatus(result.Tasks))
			},
		},
	}
}

// teamAsk is ask_user for a team task's agent.
const teamAsk = "Ask the user to decide before you do something that leaves this computer or cannot be undone, or to choose between ways to go on by giving 2 to 6 options. Never ask about a mail or a request: send_mail and http_request show the user the exact mail or request and ask by themselves. End your turn after asking; Rashin continues the task with their answer or the option they chose."

// Portions adapted from paperclipai/paperclip skills/paperclip/SKILL.md
// "The Heartbeat Procedure", human input questions @ 4e519bd4286e (MIT,
// Copyright (c) 2025 Paperclip AI). Modified by Ryoku: one ask_user card to
// the dream's CEO, about design or task only, never a confirmation.
const dreamAsk = "Ask the CEO, the user, a design or task question; never ask for confirmation or permission, as a dream runs without them. Your first turn always asks one round, even when the statement looks complete: then share your plan and ask what to change. Put every question that does not depend on another's answer in one card, written out in full in its question field, rather than one card each; give 2 to 6 options when the CEO should pick between ways to go on. End your turn after asking; Rashin continues you with their answer."

// askTitleRunes bounds a card title taken from the question.
const askTitleRunes = 60

// askArguments are what ask_user takes.
type askArguments struct {
	Title    string   `json:"title"`
	Question string   `json:"question"`
	Detail   string   `json:"detail"`
	Options  []string `json:"options"`
}

// card is the Inbox card's title and body: the question leads the body,
// and a call that gives only a question takes its first sentence as the
// title.
func (args askArguments) card() (string, string, error) {
	title, question := strings.TrimSpace(args.Title), strings.TrimSpace(args.Question)
	if title == "" && question == "" {
		return "", "", errors.New(i18n.T("Give the question, the title, or both."))
	}
	if title == "" {
		title = protocol.ClipWords(firstSentence(question), askTitleRunes)
	}
	return title, strings.TrimSpace(question + "\n\n" + strings.TrimSpace(args.Detail)), nil
}

// firstSentence is text up to its first line break, or its first full stop,
// question or exclamation mark that ends a sentence.
func firstSentence(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	runes := []rune(line)
	for index, r := range runes {
		if strings.ContainsRune(".?!", r) && (index+1 == len(runes) || runes[index+1] == ' ') {
			return string(runes[:index+1])
		}
	}
	return strings.TrimSpace(line)
}

// agentRef names the calling run for Teams: the session and its run token.
func agentRef(env tool.Env) (protocol.AgentRef, error) {
	if env.Context.Team == nil || env.Context.Session == "" || env.Context.Team.Token == "" {
		return protocol.AgentRef{}, errors.New(i18n.T("Only an agent working a team task has this tool."))
	}
	return protocol.AgentRef{Session: env.Context.Session, Token: env.Context.Team.Token}, nil
}

// teamAnswer runs one team method that answers in words and returns them.
func teamAnswer(ctx context.Context, env tool.Env, method string, params func(protocol.AgentRef) any) tool.Result {
	ref, err := agentRef(env)
	if err != nil {
		return failed(err)
	}
	var answer protocol.AgentAnswer
	if err := env.Host.Call(ctx, method, params(ref), &answer); err != nil {
		return failed(err)
	}
	return tool.Result{Text: answer.Text}
}

// inFolder resolves paths the agent gave relative to its session folder.
func inFolder(env tool.Env, files []string) []string {
	out := make([]string, 0, len(files))
	for _, file := range files {
		if file != "" && !filepath.IsAbs(file) {
			file = filepath.Join(env.Cwd, file)
		}
		out = append(out, file)
	}
	return out
}

// piecesStatus says how each handed-out piece stands, oldest first, with
// its report or why it ended short.
func piecesStatus(pieces []protocol.TeamTask) string {
	if len(pieces) == 0 {
		return i18n.T("You have not handed out any piece of this task.")
	}
	var b strings.Builder
	for i := len(pieces) - 1; i >= 0; i-- {
		piece := pieces[i]
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(piece.Title + ": " + piece.Status)
		switch {
		case piece.Report != nil:
			b.WriteString("\n" + piece.Report.Body)
		case piece.Reason != "":
			b.WriteString("\n" + piece.Reason)
		}
	}
	return b.String()
}
