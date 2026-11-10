package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/gitgraph"
	"ryoku-rashin/internal/protocol"
)

func init() {
	register(&Command{Name: "projects", Group: "work", Usage: "[add <path>|rm <id>]", Summary: "Manage coding projects.", Run: runProjects})
	register(&Command{Name: "worktrees", Group: "work", Usage: "[list|new|rm|lock|unlock|prune|cleanup]", Summary: "Manage project worktrees.", Run: runWorktrees})
	register(&Command{Name: "wt", Group: "work", Usage: "[list|new|rm|lock|unlock|prune|cleanup]", Summary: "Manage project worktrees.", Run: runWorktrees})
	register(&Command{Name: "tasks", Group: "work", Usage: "[list|add|start|done]", Summary: "Manage project tasks.", Run: runTasks})
	register(&Command{Name: "graph", Group: "work", Usage: "[--project p] [--limit n]", Summary: "Show the project commit graph.", Run: runGraph})
}

func workConn() (*client.Conn, context.CancelFunc, int) {
	conn, cancel, err := dial(true)
	if err != nil {
		return nil, func() {}, report(err)
	}
	return conn, cancel, 0
}

func runProjects(args []string) int {
	conn, cancel, code := workConn()
	if code != 0 {
		return code
	}
	defer cancel()
	defer conn.Close()
	if len(args) == 0 || len(args) == 1 && args[0] == "list" {
		var result protocol.ProjectsResult
		if err := conn.Call(context.Background(), protocol.MProjectsList, protocol.Empty{}, &result); err != nil {
			return report(err)
		}
		for _, project := range result.Projects {
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", project.ID, project.Name, project.Root)
		}
		return 0
	}
	switch args[0] {
	case "add":
		if len(args) != 2 {
			return usageError("projects add <path>")
		}
		path, err := absoluteUserPath(args[1])
		if err != nil {
			return report(err)
		}
		var project protocol.ProjectInfo
		if err := conn.Call(context.Background(), protocol.MProjectsAdd, protocol.ProjectAddParams{Path: path}, &project); err != nil {
			return report(err)
		}
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", project.ID, project.Name, project.Root)
		return 0
	case "rm":
		if len(args) != 2 {
			return usageError("projects rm <id>")
		}
		return callResult(conn.Call(context.Background(), protocol.MProjectsRemove, protocol.ProjectRef{Project: args[1]}, nil))
	default:
		return usageError("projects [add <path>|rm <id>]")
	}
}

func runWorktrees(args []string) int {
	if len(args) == 0 {
		args = []string{"list"}
	}
	conn, cancel, code := workConn()
	if code != 0 {
		return code
	}
	defer cancel()
	defer conn.Close()
	switch args[0] {
	case "list":
		if len(args) > 2 {
			return usageError("worktrees list [project]")
		}
		project, err := projectFor(conn, optionalArg(args, 1))
		if err != nil {
			return report(err)
		}
		var result protocol.WorktreesResult
		if err := conn.Call(context.Background(), protocol.MWorktreesList, protocol.ProjectRef{Project: project}, &result); err != nil {
			return report(err)
		}
		for _, wt := range result.Worktrees {
			state := wt.Branch
			if state == "" {
				state = i18n.T("detached")
			}
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", wt.Path, state, wt.Head)
		}
		return 0
	case "new":
		if len(args) < 2 {
			return usageError("worktrees new <name> [--project p] [--base ref] [--branch b]")
		}
		values, err := parseOptions(args[2:], map[string]bool{"project": true, "base": true, "branch": true})
		if err != nil {
			return report(err)
		}
		project, err := projectFor(conn, values["project"])
		if err != nil {
			return report(err)
		}
		params := protocol.WorktreeCreateParams{Project: project, Name: args[1], Mode: protocol.WorktreeNewBranch, Base: values["base"], Branch: values["branch"]}
		var wt protocol.WorktreeInfo
		if err := conn.Call(context.Background(), protocol.MWorktreesCreate, params, &wt); err != nil {
			return report(err)
		}
		fmt.Fprintln(os.Stdout, wt.Path)
		return 0
	case "rm":
		if len(args) < 2 {
			return usageError("worktrees rm <path> [--force] [--delete-branch]")
		}
		values, err := parseOptions(args[2:], map[string]bool{"force": false, "delete-branch": false})
		if err != nil {
			return report(err)
		}
		path, err := absoluteUserPath(args[1])
		if err != nil {
			return report(err)
		}
		params := protocol.WorktreeRemoveParams{Path: path, Force: values["force"] == "true", DeleteBranch: values["delete-branch"] == "true"}
		return callResult(conn.Call(context.Background(), protocol.MWorktreesRemove, params, nil))
	case "lock":
		if len(args) < 2 {
			return usageError("worktrees lock <path> [reason]")
		}
		path, err := absoluteUserPath(args[1])
		if err != nil {
			return report(err)
		}
		var wt protocol.WorktreeInfo
		err = conn.Call(context.Background(), protocol.MWorktreesLock, protocol.WorktreeLockParams{Path: path, Lock: true, Reason: strings.Join(args[2:], " ")}, &wt)
		return callResult(err)
	case "unlock":
		if len(args) != 2 {
			return usageError("worktrees unlock <path>")
		}
		path, err := absoluteUserPath(args[1])
		if err != nil {
			return report(err)
		}
		var wt protocol.WorktreeInfo
		return callResult(conn.Call(context.Background(), protocol.MWorktreesLock, protocol.WorktreeLockParams{Path: path}, &wt))
	case "prune", "cleanup":
		values, err := parseOptionalProject(args[1:])
		if err != nil {
			return report(err)
		}
		project, err := projectFor(conn, values["project"])
		if err != nil {
			return report(err)
		}
		dryRun := values["dry-run"] == "true"
		if args[0] == "prune" {
			var result protocol.PruneResult
			if err := conn.Call(context.Background(), protocol.MWorktreesPrune, protocol.PruneParams{Project: project, DryRun: dryRun}, &result); err != nil {
				return report(err)
			}
			for _, path := range result.Pruned {
				fmt.Fprintln(os.Stdout, path)
			}
			return 0
		}
		var result protocol.CleanupResult
		if err := conn.Call(context.Background(), protocol.MWorktreesCleanup, protocol.CleanupParams{Project: project, DryRun: dryRun}, &result); err != nil {
			return report(err)
		}
		for _, item := range result.Items {
			fmt.Fprintf(os.Stdout, "%s\t%t\t%s\n", item.Path, item.Removed, item.Reason)
		}
		return 0
	default:
		return usageError("worktrees [list|new|rm|lock|unlock|prune|cleanup]")
	}
}

func runTasks(args []string) int {
	if len(args) == 0 {
		args = []string{"list"}
	}
	conn, cancel, code := workConn()
	if code != 0 {
		return code
	}
	defer cancel()
	defer conn.Close()
	switch args[0] {
	case "list":
		if len(args) > 2 {
			return usageError("tasks list [project]")
		}
		project, err := projectFor(conn, optionalArg(args, 1))
		if err != nil {
			return report(err)
		}
		var result protocol.TasksResult
		if err := conn.Call(context.Background(), protocol.MTasksList, protocol.TasksParams{Project: project}, &result); err != nil {
			return report(err)
		}
		for _, task := range result.Tasks {
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", task.ID, task.Status, task.Title)
		}
		return 0
	case "add":
		if len(args) < 2 {
			return usageError("tasks add <title> [--project p] [--goal text]")
		}
		values, err := parseOptions(args[2:], map[string]bool{"project": true, "goal": true})
		if err != nil {
			return report(err)
		}
		project, err := projectFor(conn, values["project"])
		if err != nil {
			return report(err)
		}
		var task protocol.TaskInfo
		if err := conn.Call(context.Background(), protocol.MTasksCreate, protocol.TaskCreateParams{Project: project, Title: args[1], Goal: values["goal"]}, &task); err != nil {
			return report(err)
		}
		fmt.Fprintln(os.Stdout, task.ID)
		return 0
	case "start":
		if len(args) < 2 {
			return usageError("tasks start <id> [--harness h]")
		}
		values, err := parseOptions(args[2:], map[string]bool{"harness": true})
		if err != nil {
			return report(err)
		}
		var result protocol.TaskStartResult
		if err := conn.Call(context.Background(), protocol.MTasksStart, protocol.TaskStartParams{ID: args[1], Harness: values["harness"]}, &result); err != nil {
			return report(err)
		}
		fmt.Fprintf(os.Stdout, "%s\t%s\n", result.Session.ID, result.Worktree.Path)
		return 0
	case "done":
		if len(args) != 2 {
			return usageError("tasks done <id>")
		}
		patch := json.RawMessage(`{"status":"done"}`)
		var task protocol.TaskInfo
		return callResult(conn.Call(context.Background(), protocol.MTasksUpdate, protocol.TaskUpdateParams{ID: args[1], Patch: patch}, &task))
	default:
		return usageError("tasks [list|add|start|done]")
	}
}

func runGraph(args []string) int {
	values, err := parseOptions(args, map[string]bool{"project": true, "limit": true})
	if err != nil {
		return report(err)
	}
	limit := 50
	if values["limit"] != "" {
		parsed, err := strconv.Atoi(values["limit"])
		if err != nil || parsed <= 0 {
			return report(fmt.Errorf("%s", i18n.T("Graph limit must be a positive number.")))
		}
		limit = parsed
	}
	conn, cancel, code := workConn()
	if code != 0 {
		return code
	}
	defer cancel()
	defer conn.Close()
	project, err := projectFor(conn, values["project"])
	if err != nil {
		return report(err)
	}
	var result protocol.GitLogResult
	if err := conn.Call(context.Background(), protocol.MGitLog, protocol.GitLogParams{Project: project, Scope: protocol.ScopeAll, Limit: limit}, &result); err != nil {
		return report(err)
	}
	rows := gitgraph.New().Append(result.Commits)
	commits := make(map[string]protocol.GitCommit, len(result.Commits))
	for _, commit := range result.Commits {
		commits[commit.Hash] = commit
	}
	for _, row := range rows {
		commit := commits[row.Commit]
		fmt.Fprintf(os.Stdout, "%s %s %s\n", graphCells(row), shortHash(commit.Hash), commit.Subject)
	}
	return 0
}

func graphCells(row gitgraph.Row) string {
	width := row.Width
	if width <= row.Lane {
		width = row.Lane + 1
	}
	if width < 1 {
		width = 1
	}
	cells := make([]rune, width*2-1)
	for i := range cells {
		cells[i] = ' '
	}
	for _, segment := range row.Segments {
		from, to := segment.From*2, segment.To*2
		if from == to {
			if cells[from] == ' ' {
				cells[from] = '│'
			}
			continue
		}
		if from > to {
			from, to = to, from
		}
		for i := from + 1; i < to; i++ {
			cells[i] = '─'
		}
		if cells[from] == ' ' {
			cells[from] = '├'
		}
		if cells[to] == ' ' {
			cells[to] = '┤'
		}
	}
	marker := '●'
	if row.IsMerge {
		marker = '◆'
	} else if row.IsRoot {
		marker = '■'
	}
	cells[row.Lane*2] = marker
	return string(cells)
}

func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

func projectFor(conn *client.Conn, project string) (string, error) {
	if project != "" {
		return project, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	command := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("%s", i18n.T("The current directory is not in a Git project."))
	}
	path, err := absoluteUserPath(strings.TrimSpace(string(output)))
	if err != nil {
		return "", err
	}
	var info protocol.ProjectInfo
	if err := conn.Call(context.Background(), protocol.MProjectsAdd, protocol.ProjectAddParams{Path: path}, &info); err != nil {
		return "", err
	}
	return info.ID, nil
}

func parseOptionalProject(args []string) (map[string]string, error) {
	values := map[string]string{}
	for len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		if values["project"] != "" {
			return nil, fmt.Errorf("%s", i18n.T("Only one project may be given."))
		}
		values["project"] = args[0]
		args = args[1:]
	}
	extra, err := parseOptions(args, map[string]bool{"dry-run": false})
	if err != nil {
		return nil, err
	}
	values["dry-run"] = extra["dry-run"]
	return values, nil
}

func parseOptions(args []string, allowed map[string]bool) (map[string]string, error) {
	values := make(map[string]string)
	for len(args) > 0 {
		arg := args[0]
		if !strings.HasPrefix(arg, "--") {
			return nil, fmt.Errorf("%s", i18n.Tf("Unexpected argument: %s", arg))
		}
		name := strings.TrimPrefix(arg, "--")
		takesValue, ok := allowed[name]
		if !ok {
			return nil, fmt.Errorf("%s", i18n.Tf("Unknown option: %s", arg))
		}
		if !takesValue {
			values[name] = "true"
			args = args[1:]
			continue
		}
		if len(args) < 2 || strings.HasPrefix(args[1], "--") {
			return nil, fmt.Errorf("%s", i18n.Tf("Option %s needs a value.", arg))
		}
		values[name] = args[1]
		args = args[2:]
	}
	return values, nil
}

func optionalArg(args []string, index int) string {
	if index < len(args) {
		return args[index]
	}
	return ""
}
