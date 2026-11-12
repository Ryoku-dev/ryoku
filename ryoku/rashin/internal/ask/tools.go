package ask

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/ask/command"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

var askToolset = []llm.Tool{
	{Name: "system_query", Description: "Run one command only when it is provably read-only.", Parameters: schema(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"],"additionalProperties":false}`)},
	{Name: "read_file", Description: "Read a bounded range from a non-secret local text file.", Parameters: schema(`{"type":"object","properties":{"path":{"type":"string"},"start":{"type":"integer","minimum":1},"end":{"type":"integer","minimum":1}},"required":["path"],"additionalProperties":false}`)},
	{Name: "list_dir", Description: "List a local directory without changing it.", Parameters: schema(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`)},
	{Name: "search_system", Description: "Search the indexed Ryoku system and vault.", Parameters: schema(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"],"additionalProperties":false}`)},
	{Name: "search_code", Description: "Search an indexed project rooted at path or the terminal working directory.", Parameters: schema(`{"type":"object","properties":{"query":{"type":"string"},"path":{"type":"string"}},"required":["query"],"additionalProperties":false}`)},
	{Name: "read_wiki", Description: "Read one named page from the Ryoku vault wiki.", Parameters: schema(`{"type":"object","properties":{"page":{"type":"string"}},"required":["page"],"additionalProperties":false}`)},
	{Name: "fetch_url", Description: "Fetch public HTTP or HTTPS text with GET.", Parameters: schema(`{"type":"object","properties":{"url":{"type":"string"}},"required":["url"],"additionalProperties":false}`)},
	{Name: "doctor", Description: "Run Ryoku doctor and return only todo, warn, and fail findings.", Parameters: schema(`{"type":"object","properties":{},"additionalProperties":false}`)},
	{Name: "propose", Description: "Propose exact commands for the user to run. This tool never runs them.", Parameters: schema(`{"type":"object","properties":{"summary":{"type":"string"},"commands":{"type":"array","items":{"type":"object","properties":{"command":{"type":"string"},"why":{"type":"string"}},"required":["command"],"additionalProperties":false}}},"required":["commands"],"additionalProperties":false}`)},
}

// askTools is what Ask is handed in mode: the tools that change the machine
// only when the session allows it, and the team tools in every mode since
// they only read or ask.
func askTools(mode string) []llm.Tool {
	tools := make([]llm.Tool, 0, len(askToolset)+len(actToolset)+len(teamToolset))
	tools = append(tools, askToolset...)
	if acts(mode) {
		tools = append(tools, actToolset...)
	}
	return append(tools, teamToolset...)
}

// commandTimeout is how long one command may run. A walk over a whole home
// folder takes a minute on a full disk, and cutting it at fifteen seconds
// turned "which file is largest" into a command for the user to run.
const commandTimeout = 2 * time.Minute

func schema(value string) json.RawMessage { return json.RawMessage(value) }

func (f *Factory) runTool(ctx context.Context, call llm.ToolCall, terminal *protocol.TerminalContext, session string) (string, *protocol.CommandPlan, error) {
	cwd := paths.Home()
	if terminal != nil && terminal.Cwd != "" {
		cwd = terminal.Cwd
	}
	switch call.Name {
	case "read_file":
		var args struct {
			Path  string `json:"path"`
			Start int    `json:"start"`
			End   int    `json:"end"`
		}
		if err := decodeArgs(call.Arguments, &args); err != nil {
			return "", nil, err
		}
		out, err := readFileTool(args.Path, args.Start, args.End, cwd)
		return out, nil, err
	case "list_dir":
		var args struct {
			Path string `json:"path"`
		}
		if err := decodeArgs(call.Arguments, &args); err != nil {
			return "", nil, err
		}
		out, err := listDirTool(args.Path, cwd)
		return out, nil, err
	case "search_system":
		var args struct {
			Query string `json:"query"`
		}
		if err := decodeArgs(call.Arguments, &args); err != nil {
			return "", nil, err
		}
		if f.index == nil {
			return "", nil, fmt.Errorf("%s", i18n.T("The system index is unavailable."))
		}
		result, err := f.index.Search(ctx, protocol.SearchParams{IndexRef: protocol.IndexRef{ID: protocol.IndexSystem}, Query: args.Query, Limit: 8, Compact: true, Existing: true})
		return marshalToolResult(result, err), nil, err
	case "search_code":
		var args struct {
			Query string `json:"query"`
			Path  string `json:"path"`
		}
		if err := decodeArgs(call.Arguments, &args); err != nil {
			return "", nil, err
		}
		if f.index == nil {
			return "", nil, fmt.Errorf("%s", i18n.T("The code index is unavailable."))
		}
		root := args.Path
		if root == "" {
			root = cwd
		} else {
			root = resolvePath(root, cwd)
		}
		indexes, err := f.index.List(ctx)
		if err != nil {
			return "", nil, err
		}
		ref, ok := registeredProjectIndex(indexes, root)
		if !ok {
			gitRoot := enclosingGitRoot(root)
			return i18n.Tf("This project is not indexed. Add it in Rashin's Index page or run `rashin index add %s`.", quoteShellArg(gitRoot)), nil, nil
		}
		result, err := f.index.Search(ctx, protocol.SearchParams{IndexRef: ref, Query: args.Query, Limit: 8, Compact: true, Existing: true})
		return marshalToolResult(result, err), nil, err
	case "read_wiki":
		var args struct {
			Page string `json:"page"`
		}
		if err := decodeArgs(call.Arguments, &args); err != nil {
			return "", nil, err
		}
		out, err := readWiki(args.Page)
		return out, nil, err
	case "fetch_url":
		var args struct {
			URL string `json:"url"`
		}
		if err := decodeArgs(call.Arguments, &args); err != nil {
			return "", nil, err
		}
		out, err := fetchPublic(ctx, args.URL)
		return out, nil, err
	case "doctor":
		if err := decodeArgs(call.Arguments, &struct{}{}); err != nil {
			return "", nil, err
		}
		out, err := f.doctor.run(ctx)
		return out, nil, err
	case "propose":
		plan, err := proposedPlan(call.Arguments)
		if err != nil {
			return "", nil, err
		}
		body, _ := json.Marshal(plan)
		return string(body), plan, nil
	default:
		if out, handled, err := runTeamTool(ctx, f.teams, call, session); handled {
			return out, nil, err
		}
		return "", nil, fmt.Errorf(i18n.T("Unknown Ask tool %q."), call.Name)
	}
}

// registeredProjectIndex resolves only from the registry snapshot. Search must
// never turn an Ask query into index registration work.
func registeredProjectIndex(indexes []protocol.IndexInfo, target string) (protocol.IndexRef, bool) {
	target = canonicalToolPath(target)
	bestID := ""
	bestLen := -1
	for _, info := range indexes {
		if info.Kind != protocol.IndexProject || info.ID == "" || info.ID == protocol.IndexSystem {
			continue
		}
		if rootLen := matchingRootLength(info.Root, target); rootLen > bestLen {
			bestID = info.ID
			bestLen = rootLen
		}
		for _, root := range info.Roots {
			if rootLen := matchingRootLength(root, target); rootLen > bestLen {
				bestID = info.ID
				bestLen = rootLen
			}
		}
	}
	if bestID == "" {
		return protocol.IndexRef{}, false
	}
	return protocol.IndexRef{ID: bestID}, true
}

func matchingRootLength(root, target string) int {
	root = canonicalToolPath(root)
	if root == "" || !pathContains(root, target) {
		return -1
	}
	return len(root)
}

func canonicalToolPath(path string) string {
	if path == "" {
		return ""
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = resolved
	}
	return filepath.Clean(absolute)
}

func pathContains(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func enclosingGitRoot(path string) string {
	current := canonicalToolPath(path)
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for current != "" {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return canonicalToolPath(path)
}

func quoteShellArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func decodeArgs(raw string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf(i18n.T("The tool input is invalid: %v"), err)
	}
	return nil
}

func resolvePath(path, cwd string) string {
	path = strings.TrimSpace(path)
	if path == "~" {
		return paths.Home()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(paths.Home(), strings.TrimPrefix(path, "~/"))
	}
	if !filepath.IsAbs(path) {
		return filepath.Join(cwd, path)
	}
	return filepath.Clean(path)
}

func readFileTool(path string, start, end int, cwd string) (string, error) {
	path = resolvePath(path, cwd)
	if marker, blocked := command.SecretPath(path); blocked {
		return "", fmt.Errorf(i18n.T("Refused to read %s because it contains %s."), path, marker)
	}
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !linkInfo.Mode().IsRegular() && linkInfo.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("%s", i18n.T("Refused to read a non-regular file."))
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s", i18n.T("Refused to read a non-regular file."))
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return "", err
	}
	if !openedInfo.Mode().IsRegular() {
		_ = file.Close()
		return "", fmt.Errorf("%s", i18n.T("Refused to read a non-regular file."))
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 64*1024+1))
	if err != nil {
		return "", err
	}
	truncated := len(body) > 64*1024
	if truncated {
		body = body[:64*1024]
	}
	lines := strings.Split(string(body), "\n")
	if start < 1 {
		start = 1
	}
	if end < 1 || end > len(lines) {
		end = len(lines)
	}
	if start > end || start > len(lines) {
		return "", fmt.Errorf("%s", i18n.T("The requested line range is outside the file."))
	}
	out := strings.Join(lines[start-1:end], "\n")
	if truncated {
		out += "\n[trimmed at 64 KiB]"
	}
	return out, nil
}

func listDirTool(path, cwd string) (string, error) {
	path = resolvePath(path, cwd)
	if marker, blocked := command.SecretPath(path); blocked {
		return "", fmt.Errorf(i18n.T("Refused to list %s because it contains %s."), path, marker)
	}
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !linkInfo.IsDir() && linkInfo.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("%s", i18n.T("Refused to list a path that is not a directory."))
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s", i18n.T("Refused to list a path that is not a directory."))
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for i, entry := range entries {
		if i == 256 {
			b.WriteString("[more entries omitted]\n")
			break
		}
		kind := "file"
		if entry.IsDir() {
			kind = "dir"
		} else if entry.Type()&os.ModeSymlink != 0 {
			kind = "link"
		}
		fmt.Fprintf(&b, "%s\t%s\n", kind, entry.Name())
	}
	return strings.TrimSpace(b.String()), nil
}

func readWiki(page string) (string, error) {
	root := paths.VaultDir()
	if _, err := os.Stat(root); err != nil {
		root = paths.LegacyVaultDir()
	}
	wiki := filepath.Join(root, "wiki")
	page = strings.TrimSpace(page)
	if filepath.Ext(page) == "" {
		page += ".md"
	}
	clean := filepath.Clean(filepath.FromSlash(page))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
		return "", fmt.Errorf("%s", i18n.T("The wiki page name is invalid."))
	}
	body, err := readAtMost(filepath.Join(wiki, clean), 64*1024)
	if err == nil {
		return body, nil
	}
	var pages []string
	_ = filepath.WalkDir(wiki, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		if rel, relErr := filepath.Rel(wiki, path); relErr == nil {
			pages = append(pages, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(pages)
	return "", fmt.Errorf(i18n.T("Wiki page %q was not found. Available pages: %s"), page, strings.Join(pages, ", "))
}

func readAtMost(path string, limit int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return "", err
	}
	if len(body) > limit {
		return string(body[:limit]) + "\n[trimmed]", nil
	}
	return string(body), nil
}

func proposedPlan(raw string) (*protocol.CommandPlan, error) {
	var args struct {
		Summary  string `json:"summary"`
		Commands []struct {
			Command string `json:"command"`
			Why     string `json:"why"`
		} `json:"commands"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	plan := &protocol.CommandPlan{Summary: strings.TrimSpace(args.Summary)}
	for _, item := range args.Commands {
		cmd := strings.TrimSpace(item.Command)
		if cmd == "" {
			continue
		}
		tier, _ := command.Classify(cmd)
		_, terminal := command.NeedsTerminal(cmd)
		plan.Commands = append(plan.Commands, protocol.PlannedCommand{Command: cmd, Why: strings.TrimSpace(item.Why), Tier: tier, Terminal: terminal})
	}
	if len(plan.Commands) == 0 {
		return nil, fmt.Errorf("%s", i18n.T("The proposed plan has no commands."))
	}
	return plan, nil
}

func runBounded(parent context.Context, timeout time.Duration, cwd string, capBytes int, name string, args ...string) (string, error) {
	output := &limitedBuffer{limit: capBytes}
	_, err := runInto(parent, timeout, cwd, output, name, args...)
	return output.text(), err
}

// runInto runs a program with both its streams written to out as they come,
// and returns its exit status, -1 when it did not exit by itself.
func runInto(parent context.Context, timeout time.Duration, cwd string, out io.Writer, name string, args ...string) (int, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return nil
		}
		return err
	}
	cmd.WaitDelay = 2 * time.Second
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	exit := -1
	if cmd.ProcessState != nil {
		exit = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() == context.DeadlineExceeded {
		return exit, ranOut(timeout)
	}
	if err != nil {
		return exit, fmt.Errorf(i18n.T("The command failed: %v"), err)
	}
	return exit, nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit     int
	truncated bool
}

// ranOut is the error of a command stopped for taking too long.
type ranOut time.Duration

func (limit ranOut) Error() string {
	return i18n.Tf("The command was stopped after %d seconds without finishing.", int(time.Duration(limit).Seconds()))
}

func (b *limitedBuffer) text() string {
	text := strings.TrimSpace(b.String())
	if b.truncated {
		text += "\n[output trimmed]"
	}
	return text
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		b.truncated = true
		return original, nil
	}
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.Buffer.Write(p)
	return original, nil
}

func marshalToolResult(value any, err error) string {
	if err != nil {
		return ""
	}
	body, marshalErr := json.Marshal(value)
	if marshalErr != nil {
		return marshalErr.Error()
	}
	return string(body)
}

type doctorFinding struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Remedy string `json:"remedy,omitempty"`
}

type doctorCache struct {
	mu      sync.Mutex
	at      time.Time
	out     string
	err     error
	loading chan struct{}
}

func (c *doctorCache) run(ctx context.Context) (string, error) {
	for {
		c.mu.Lock()
		if !c.at.IsZero() && time.Since(c.at) < 2*time.Minute {
			out, err := c.out, c.err
			c.mu.Unlock()
			return out, err
		}
		if c.loading != nil {
			loading := c.loading
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-loading:
				continue
			}
		}
		loading := make(chan struct{})
		c.loading = loading
		c.mu.Unlock()

		out, err := runDoctor(ctx)

		c.mu.Lock()
		if ctx.Err() == nil {
			c.at, c.out, c.err = time.Now(), out, err
		}
		c.loading = nil
		close(loading)
		c.mu.Unlock()
		return out, err
	}
}

func runDoctor(ctx context.Context) (string, error) {
	out, runErr := runBounded(ctx, 90*time.Second, paths.Home(), 1024*1024, "ryoku", "doctor", "--json")
	var findings []doctorFinding
	err := json.Unmarshal([]byte(out), &findings)
	if err == nil {
		issues := findings[:0]
		for _, finding := range findings {
			switch finding.Status {
			case "todo", "warn", "fail":
				issues = append(issues, finding)
			}
		}
		body, _ := json.Marshal(issues)
		return string(body), nil
	}
	if runErr != nil {
		return out, runErr
	}
	return out, fmt.Errorf(i18n.T("Ryoku doctor returned invalid JSON: %v"), err)
}

// toolPresentation is how a read tool's call reads in the transcript: what it
// is doing, its kind, and its input. A command is shown as the command, the
// way a person would type it, and every other call as its arguments.
func toolPresentation(call llm.ToolCall) (title, kind, input string) {
	var args map[string]any
	_ = json.Unmarshal([]byte(call.Arguments), &args)
	value := func(key string) string {
		if text, ok := args[key].(string); ok {
			return truncateRunes(text, 80)
		}
		return ""
	}
	input = truncateRunes(compactInput(call.Arguments), 400)
	if title, kind, ok := teamToolPresentation(call); ok {
		return title, kind, input
	}
	if title, kind, ok := skillToolPresentation(call); ok {
		return title, kind, input
	}
	switch call.Name {
	case "system_query":
		command, _ := args["command"].(string)
		return i18n.T("Checking"), protocol.ToolExecute, truncateRunes(command, 400)
	case "read_file":
		return i18n.Tf("Reading %s", value("path")), protocol.ToolRead, input
	case "list_dir":
		return i18n.Tf("Listing %s", value("path")), protocol.ToolRead, input
	case "search_system", "search_code":
		return i18n.Tf("Searching for %s", value("query")), protocol.ToolSearch, input
	case "read_wiki":
		return i18n.Tf("Reading wiki page %s", value("page")), protocol.ToolRead, input
	case "fetch_url":
		return i18n.Tf("Fetching %s", value("url")), protocol.ToolFetch, input
	case "doctor":
		return i18n.T("Checking Ryoku health"), protocol.ToolExecute, ""
	case "propose":
		return i18n.T("Preparing a command plan"), protocol.ToolThink, input
	default:
		return call.Name, protocol.ToolOther, input
	}
}
