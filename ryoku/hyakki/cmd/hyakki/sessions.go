package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

var fixAppName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+@-]{0,63}$`)

type doctorFinding struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Remedy string `json:"remedy,omitempty"`
}

type doctorScan struct {
	Findings []doctorFinding `json:"findings"`
	Report   string          `json:"report,omitempty"`
	Error    string          `json:"error,omitempty"`
}

const fixMethod = "How to work: investigate before changing anything and gather the evidence yourself. Read the journal, crash dumps, the app's own logs and recent package changes before asking the user for facts the machine has. Before editing a file, run `ryoku owner <path>` and never edit a Ryoku-owned generated file or tool store; edit the source it names. Prefer Ryoku's commands and reversible steps. Ask before destructive work or anything that needs sudo. Explain the cause in plain words, say what you will do, do it, verify it, and explain how to undo it. If nothing needs fixing, say so and stop. Write what you found and did to the vault journal at journal/YYYY-MM-DD.md."

func init() {
	register(&Command{Name: "sessions", Group: "sessions", Usage: "[ask|agent]", Summary: "List saved sessions.", Run: runSessions})
	register(&Command{Name: "show", Group: "sessions", Usage: "<id>", Summary: "Show a saved session.", Run: runShow})
	register(&Command{Name: "rm", Group: "sessions", Usage: "<id>", Summary: "Delete a saved session.", Run: runRemove})
	register(&Command{Name: "open", Group: "sessions", Usage: "[--session id] [--tab tab] [--project id-or-path] [--view view] [--worktree path] [--route route]", Summary: "Open the Hyakki app. Settings views include oni, oni-voice, oni-screen, oni-commands, oni-writing, hands, oni-privacy and parts.", Run: runOpen})
	register(&Command{Name: "fix", Group: "sessions", Usage: "doctor | app <name> [what happened]", Summary: "Open a repair session from machine evidence.", Run: runFix})
}

func runSessions(args []string) int {
	var kind protocol.SessionKind
	if len(args) > 1 {
		return usageError("sessions [ask|agent]")
	}
	if len(args) == 1 {
		kind = protocol.SessionKind(args[0])
		if kind != protocol.KindAsk && kind != protocol.KindAgent {
			return usageError("sessions [ask|agent]")
		}
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	var result protocol.SessionListResult
	if err := conn.Call(context.Background(), protocol.MSessionList, protocol.SessionListParams{Kind: kind, Limit: 200}, &result); err != nil {
		return report(err)
	}
	for _, info := range result.Sessions {
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\n", info.ID, info.State, info.Updated.Local().Format("2006-01-02 15:04"), sessionTableTitle(info.Title))
	}
	return 0
}

func sessionTableTitle(title string) string {
	title = strings.Join(strings.Fields(title), " ")
	runes := []rune(title)
	if len(runes) > 60 {
		return string(runes[:57]) + "..."
	}
	return title
}

func runShow(args []string) int {
	if len(args) != 1 {
		return usageError("show <id>")
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	result, err := attachAll(conn, args[0], 0)
	if err != nil {
		return report(err)
	}
	if result.Info.Kind == protocol.KindAsk {
		return printAskEvents(result.Events, askOptions{})
	}
	for _, event := range result.Events {
		encoded, _ := json.Marshal(event)
		fmt.Fprintln(os.Stdout, string(encoded))
	}
	return 0
}

func runRemove(args []string) int {
	if len(args) != 1 {
		return usageError("rm <id>")
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	if err := conn.Call(context.Background(), protocol.MSessionDelete, protocol.SessionRef{ID: args[0]}, &protocol.Empty{}); err != nil {
		return report(err)
	}
	return 0
}

func parseOpen(args []string) (protocol.UIOpenParams, error) {
	var p protocol.UIOpenParams
	for len(args) > 0 {
		if len(args) < 2 {
			return p, fmt.Errorf("an option value is missing")
		}
		switch args[0] {
		case "--session":
			p.Session = args[1]
		case "--tab":
			p.Tab = args[1]
		case "--project":
			p.Project = args[1]
		case "--view":
			switch args[1] {
			case "overview", "graph", "changes", "tasks", "activity", "rashin", "features", "providers", "api", "ask", "agents", "teams", "memory", "skills", "index", "oni", "hands", "appearance", "keys", "oni-voice", "oni-screen", "oni-commands", "oni-writing", "oni-privacy", "parts":
				p.View = args[1]
			default:
				return p, fmt.Errorf("%s", i18n.T("Open view must name a project view or a Settings section."))
			}
		case "--worktree":
			path, err := absoluteUserPath(args[1])
			if err != nil {
				return p, err
			}
			p.Worktree = path
		case "--route":
			if err := parseTeamsRoute(args[1], &p); err != nil {
				return p, err
			}
		default:
			return p, fmt.Errorf("unknown option %s", args[0])
		}
		args = args[2:]
	}
	if settingsView(p.View) && (p.Tab != protocol.TabSettings || p.Project != "") {
		return p, fmt.Errorf("%s", i18n.T("Settings views open with --tab settings and no project."))
	}
	return p, nil
}

func settingsView(view string) bool {
	switch view {
	case "rashin", "features", "providers", "api", "ask", "agents", "teams", "memory", "skills", "index", "oni", "hands", "appearance", "keys", "oni-voice", "oni-screen", "oni-commands", "oni-writing", "oni-privacy", "parts":
		return true
	}
	return false
}

func runOpen(args []string) int {
	p, err := parseOpen(args)
	if err != nil {
		return report(err)
	}
	p.ActivationToken = os.Getenv("XDG_ACTIVATION_TOKEN")
	_ = os.Unsetenv("XDG_ACTIVATION_TOKEN")
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	if err := resolveOpenProject(conn, &p); err != nil {
		return report(err)
	}
	var result protocol.UIOpenResult
	if err := conn.Call(context.Background(), protocol.MUIOpen, p, &result); err != nil {
		return report(err)
	}
	if result.Delivered {
		return 0
	}
	return launchApp(p)
}

func resolveOpenProject(conn *client.Conn, p *protocol.UIOpenParams) error {
	if p.Project == "" {
		return nil
	}
	var result protocol.ProjectsResult
	if err := conn.Call(context.Background(), protocol.MProjectsList, protocol.Empty{}, &result); err != nil {
		return err
	}
	for _, project := range result.Projects {
		if project.ID == p.Project {
			p.Project = project.ID
			return nil
		}
	}
	path, err := absoluteUserPath(p.Project)
	if err != nil {
		return err
	}
	for _, project := range result.Projects {
		if project.Root == path {
			p.Project = project.ID
			return nil
		}
	}
	return fmt.Errorf("%s", i18n.Tf("No registered project matches %s.", p.Project))
}

func launchApp(p protocol.UIOpenParams) int {
	args := []string{}
	if p.Session != "" {
		args = append(args, "--session", p.Session)
	}
	// a Teams mode place travels as one route, which carries its tab and view
	if route := teamsRoute(p); route != "" {
		args = append(args, "--route", route)
	} else {
		if p.Tab != "" {
			args = append(args, "--tab", p.Tab)
		}
		if p.Project != "" {
			args = append(args, "--project", p.Project)
		}
		if p.View != "" {
			args = append(args, "--view", p.View)
		}
	}
	if p.Worktree != "" {
		args = append(args, "--worktree", p.Worktree)
	}
	var command *exec.Cmd
	runner, runnerErr := exec.LookPath("systemd-run")
	if runnerErr == nil && systemdPresent() && userUnitAvailable("basic.target") {
		runArgs := []string{"--user", "--scope", "--collect", "--quiet", "hyakki-app"}
		runArgs = append(runArgs, args...)
		command = exec.Command(runner, runArgs...)
	} else {
		command = exec.Command("hyakki-app", args...)
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		command.Stdin = nil
		command.Stdout = nil
		command.Stderr = nil
	}
	command.Env = os.Environ()
	if p.ActivationToken != "" {
		command.Env = append(command.Env, "XDG_ACTIVATION_TOKEN="+p.ActivationToken)
	}
	if err := command.Start(); err != nil {
		return report(err)
	}
	return 0
}

// parseTeamsRoute reads a Teams mode route, the way a notification names
// what it is about: inbox, teams[/<team>[/<view>]], dreams[/<dream>|new],
// work[/<task>], routines[/<routine>], vault[/<credential>] or activity
func parseTeamsRoute(route string, p *protocol.UIOpenParams) error {
	path, query, hasQuery := strings.Cut(route, "?")
	if hasQuery {
		if path != protocol.TabDreams+"/new" || len(query) > protocol.DreamDraftMaxBytes*3+32 {
			return fmt.Errorf("%s", i18n.T("The dream draft route is not valid."))
		}
		values, err := url.ParseQuery(query)
		if err != nil || len(values) != 1 || len(values["draft"]) != 1 || len(values.Get("draft")) > protocol.DreamDraftMaxBytes {
			return fmt.Errorf("%s", i18n.T("The dream draft is too large or is not valid."))
		}
		var draft protocol.DreamCreateParams
		if err := json.Unmarshal([]byte(values.Get("draft")), &draft); err != nil {
			return err
		}
		p.DreamDraft = &draft
		route = path
	}
	parts := strings.Split(strings.Trim(route, "/"), "/")
	invalid := fmt.Errorf("%s", i18n.Tf("%s is not a Teams route.", route))
	switch parts[0] {
	case protocol.TabSettings:
		if len(parts) > 2 || len(parts) == 2 && !settingsView(parts[1]) {
			return invalid
		}
		if len(parts) == 2 {
			p.View = parts[1]
		}
	case protocol.TabInbox, protocol.TabActivity:
		if len(parts) > 1 {
			return invalid
		}
	case protocol.TabTeams:
		if len(parts) > 3 {
			return invalid
		}
		if len(parts) > 1 {
			p.Team = parts[1]
		}
		if len(parts) > 2 {
			p.View = parts[2]
		}
	case protocol.TabDreams:
		if len(parts) > 2 {
			return invalid
		}
		if len(parts) == 2 {
			p.Team = parts[1]
		}
	case protocol.TabWork, protocol.TabRoutines, protocol.TabVault:
		if len(parts) > 2 {
			return invalid
		}
		if len(parts) == 2 {
			switch parts[0] {
			case protocol.TabWork:
				p.TeamTask = parts[1]
			case protocol.TabRoutines:
				p.Routine = parts[1]
			default:
				p.Credential = parts[1]
			}
		}
	default:
		return invalid
	}
	p.Tab = parts[0]
	return nil
}

// teamsRoute is the route the app starts on for a Teams mode place, "" for
// any other
func teamsRoute(p protocol.UIOpenParams) string {
	switch p.Tab {
	case protocol.TabInbox, protocol.TabActivity:
		return p.Tab
	case protocol.TabTeams:
		route := p.Tab
		if p.Team != "" {
			route += "/" + p.Team
			if p.View != "" {
				route += "/" + p.View
			}
		}
		return route
	case protocol.TabDreams:
		route := joinRoute(p.Tab, p.Team)
		if p.Team == "new" && p.DreamDraft != nil {
			data, err := json.Marshal(p.DreamDraft)
			if err == nil && len(data) <= protocol.DreamDraftMaxBytes {
				route += "?draft=" + url.QueryEscape(string(data))
			}
		}
		return route
	case protocol.TabWork:
		return joinRoute(p.Tab, p.TeamTask)
	case protocol.TabRoutines:
		return joinRoute(p.Tab, p.Routine)
	case protocol.TabVault:
		return joinRoute(p.Tab, p.Credential)
	}
	return ""
}

func joinRoute(tab, id string) string {
	if id == "" {
		return tab
	}
	return tab + "/" + id
}

func runFix(args []string) int {
	if len(args) == 0 {
		return usageError("fix doctor | app <name> [what happened]")
	}
	var title, brief string
	switch args[0] {
	case "doctor":
		if len(args) != 1 {
			return usageError("fix doctor")
		}
		out, err := exec.Command("ryoku", "doctor", "--json").CombinedOutput()
		if err != nil && len(out) == 0 {
			return report(err)
		}
		var scan doctorScan
		if err := json.Unmarshal(out, &scan); err != nil {
			return report(fmt.Errorf("%s", i18n.Tf("The health check returned invalid JSON: %v.", err)))
		}
		if scan.Error != "" {
			return report(fmt.Errorf("%s", scan.Error))
		}
		var issues []doctorFinding
		for _, finding := range scan.Findings {
			if finding.Status == "todo" || finding.Status == "warn" || finding.Status == "fail" {
				issues = append(issues, finding)
			}
		}
		if len(issues) == 0 {
			return report(fmt.Errorf("%s", i18n.T("The health check has nothing left to fix.")))
		}
		if len(issues) == 1 {
			title = i18n.Tf("Fix %s", issues[0].Name)
		} else {
			title = i18n.Tf("Fix %d Ryoku health findings", len(issues))
		}
		var b strings.Builder
		b.WriteString("The user asked to repair findings from `ryoku doctor`. Findings that need attention:\n")
		for _, finding := range issues {
			fmt.Fprintf(&b, "- %s, %s: %s", finding.Status, finding.Name, strings.TrimSpace(finding.Detail))
			if finding.Remedy != "" {
				fmt.Fprintf(&b, " (doctor suggests: %s)", finding.Remedy)
			}
			b.WriteByte('\n')
		}
		b.WriteString("A todo is something `ryoku doctor` applies itself when run without flags. A warn or fail needs a person. `ryoku doctor --check` reruns checks without changing anything. ")
		if scan.Report != "" {
			fmt.Fprintf(&b, "The last full report is at %s. ", scan.Report)
		}
		b.WriteString("Work through findings one at a time, most serious first. ")
		b.WriteString(fixMethod)
		brief = b.String()
	case "app":
		if len(args) < 2 || !fixAppName.MatchString(args[1]) {
			return usageError("fix app <name> [what happened]")
		}
		note := strings.TrimSpace(strings.Join(args[2:], " "))
		if len([]rune(note)) > 2000 {
			return report(fmt.Errorf("%s", i18n.T("What happened is too long.")))
		}
		title = i18n.Tf("Fix %s", args[1])
		brief = fmt.Sprintf("The user says %s is broken", args[1])
		if note != "" {
			brief += fmt.Sprintf(", in their words: %q", note)
		}
		brief += ". " + fixMethod
	default:
		return usageError("fix doctor | app <name> [what happened]")
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	info := protocol.SessionInfo{}
	if err := conn.Call(context.Background(), protocol.MSessionNew, protocol.SessionNewParams{Kind: protocol.KindAgent, Origin: protocol.OriginFix, Title: title}, &info); err != nil {
		return report(err)
	}
	if err := conn.Call(context.Background(), protocol.MSessionPrompt, protocol.PromptParams{ID: info.ID, Text: brief}, &protocol.PromptResult{}); err != nil {
		return report(err)
	}
	return runOpen([]string{"--session", info.ID, "--tab", string(protocol.KindAgent)})
}
