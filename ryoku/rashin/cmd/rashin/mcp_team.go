package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/mcp"
	"ryoku-rashin/internal/protocol"
)

// teamTools are the tools of an agent working a team task: its report, its
// questions to the user, and the credentials and mail it uses without ever
// seeing a value. Only a lead hands out pieces and reads how they went.
func teamTools(session mcpSession) []mcp.Tool {
	tools := []mcp.Tool{
		{
			Name:        "report",
			Description: "Hand your report to the user: what you found or did, written for them, with sources or files. This ends your task.",
			Input: schema([]string{"title", "body"}, map[string]any{
				"title": stringProperty("One line saying what the report is."),
				"body":  stringProperty("The report in Markdown, written for the user, with sources."),
				"files": arrayProperty("Paths of files you made, inside the team's folder."),
			}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Title string   `json:"title"`
					Body  string   `json:"body"`
					Files []string `json:"files"`
				}
				if err := toolArguments(raw, &args, "title", "body"); err != nil {
					return mcp.Failure(err)
				}
				files := make([]string, 0, len(args.Files))
				for _, file := range args.Files {
					files = append(files, session.path(file))
				}
				return teamAnswer(ctx, session, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: session.agent(), Title: args.Title, Body: args.Body, Files: files})
			},
		},
		{
			Name:        "ask_user",
			Description: "Ask the user to decide before you do something that leaves this computer or cannot be undone. Never ask about a mail or a request: send_mail and http_request show the user the exact mail or request and ask by themselves. End your turn after asking; Rashin continues the task with their answer.",
			Input: schema([]string{"title"}, map[string]any{
				"title":  stringProperty("One line saying what you want to do."),
				"detail": stringProperty("Exactly what would happen, so the user can judge it."),
			}),
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				var args struct {
					Title  string `json:"title"`
					Detail string `json:"detail"`
				}
				if err := toolArguments(raw, &args, "title"); err != nil {
					return mcp.Failure(err)
				}
				return teamAnswer(ctx, session, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: session.agent(), Title: args.Title, Detail: args.Detail})
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
	if !session.lead {
		return tools
	}
	return append(tools,
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
