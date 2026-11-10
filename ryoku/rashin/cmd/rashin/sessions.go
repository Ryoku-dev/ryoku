package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	register(&Command{Name: "open", Group: "sessions", Usage: "[--session id] [--tab tab] [--project id-or-path] [--view overview|graph|changes|tasks|activity] [--worktree path]", Summary: "Open the Rashin app.", Run: runOpen})
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
			case "overview", "graph", "changes", "tasks", "activity":
				p.View = args[1]
			default:
				return p, fmt.Errorf("%s", i18n.T("Open view must be overview, graph, changes, tasks, or activity."))
			}
		case "--worktree":
			path, err := absoluteUserPath(args[1])
			if err != nil {
				return p, err
			}
			p.Worktree = path
		default:
			return p, fmt.Errorf("unknown option %s", args[0])
		}
		args = args[2:]
	}
	return p, nil
}

func runOpen(args []string) int {
	p, err := parseOpen(args)
	if err != nil {
		return report(err)
	}
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
		if err := exec.Command("ryoku", "wm", "act", "window.summon", "Rashin").Run(); err != nil {
			return report(err)
		}
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
	if p.Tab != "" {
		args = append(args, "--tab", p.Tab)
	}
	if p.Project != "" {
		args = append(args, "--project", p.Project)
	}
	if p.View != "" {
		args = append(args, "--view", p.View)
	}
	if p.Worktree != "" {
		args = append(args, "--worktree", p.Worktree)
	}
	var command *exec.Cmd
	if systemdPresent() {
		runArgs := []string{"--user", "--scope", "--collect", "--quiet", "rashin-app"}
		runArgs = append(runArgs, args...)
		command = exec.Command("systemd-run", runArgs...)
	} else {
		command = exec.Command("rashin-app", args...)
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		command.Stdin = nil
		command.Stdout = nil
		command.Stderr = nil
	}
	if err := command.Start(); err != nil {
		return report(err)
	}
	return 0
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
	kind, harness := protocol.KindAsk, ""
	var harnesses protocol.HarnessesResult
	if err := conn.Call(context.Background(), protocol.MHarnessesList, protocol.HarnessesListParams{Refresh: true}, &harnesses); err == nil {
		for _, item := range harnesses.Harnesses {
			if !item.Installed {
				continue
			}
			kind = protocol.KindAgent
			if harness == "" {
				harness = item.ID
			}
			if item.Default {
				harness = item.ID
				break
			}
		}
	}
	info := protocol.SessionInfo{}
	if err := conn.Call(context.Background(), protocol.MSessionNew, protocol.SessionNewParams{Kind: kind, Harness: harness, Origin: protocol.OriginFix, Title: title}, &info); err != nil {
		return report(err)
	}
	if err := conn.Call(context.Background(), protocol.MSessionPrompt, protocol.PromptParams{ID: info.ID, Text: brief}, &protocol.PromptResult{}); err != nil {
		return report(err)
	}
	return runOpen([]string{"--session", info.ID, "--tab", string(kind)})
}
