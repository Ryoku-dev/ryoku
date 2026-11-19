package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// Batch limits. A local model has one llama.cpp slot, so its children run
// one after another unless the Spawner says it has more; a remote model runs
// up to MaxConcurrent at once.
const (
	MaxTasks      = 8
	MaxConcurrent = 4
	// inlineBytes caps one child's result told to the parent (omp's
	// INLINE_OUTPUT_CHARS); a small window cuts it further.
	inlineBytes   = 11500
	minChildBytes = 1024
	nameRunes     = 32
	taskLineRunes = 120
)

// Slotter is what a Spawner may also implement to say how many children of
// a model can run at once (1 for one local slot).
type Slotter interface {
	Slots(model string) int
}

// Tools are the subagent tools: task for a parent, yield for a child.
func Tools() []tool.Tool { return []tool.Tool{taskTool{}, yieldTool{}} }

type taskTool struct{}

func (taskTool) Name() string    { return tool.Task }
func (taskTool) Kind() tool.Kind { return protocol.ToolOther }

// ReadOnly: the children inherit the parent's profile and their changes pass
// the parent's Permissions, so task itself changes nothing.
func (taskTool) ReadOnly() bool { return true }

func (taskTool) Spec(p tool.Profile) llm.Tool {
	if p.Compact() {
		return llm.Tool{
			Name:        tool.Task,
			Description: "Hand separate tasks to helpers that work on them and return their results.",
			Parameters: json.RawMessage(`{"type":"object","properties":{` +
				`"context":{"type":"string","description":"Background every task needs."},` +
				`"tasks":{"type":"array","items":{"type":"string"},"description":"One complete task per item."}` +
				`},"required":["tasks"]}`),
		}
	}
	var kinds strings.Builder
	for _, agent := range bundled {
		fmt.Fprintf(&kinds, "\n- %s", agent.Name)
		if agent.ReadOnly {
			kinds.WriteString(" (read-only)")
		}
		fmt.Fprintf(&kinds, ": %s", agent.Description)
	}
	return llm.Tool{
		Name: tool.Task,
		Description: fmt.Sprintf(`Run subagents on self-contained tasks. Up to %d tasks per call; they run in parallel where the model allows, and each result comes back here.
- context: background every task shares (goal, interfaces, constraints). Write it once, here only.
- tasks[].task: self-contained, since a subagent starts blank: the files it targets, the change, and how to tell it is done.
- tasks[].name: CamelCase, at most 32 characters; made from the task when left out.
- tasks[].agent: the kind below; leave it out for worker. Use scout for research when you do not know where the code is.
Give each task its own files; one task owns any shared edit. Subagents skip builds, tests and formatters: run them once yourself afterwards.
Agents:%s`, MaxTasks, kinds.String()),
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"context":{"type":"string","description":"Shared background for every task."},` +
			`"tasks":{"type":"array","items":{"type":"object","properties":{` +
			`"name":{"type":"string","description":"CamelCase, at most 32 characters."},` +
			`"agent":{"type":"string","enum":["scout","worker"]},` +
			`"task":{"type":"string","description":"The complete assignment."}` +
			`},"required":["task"]}}` +
			`},"required":["tasks"]}`),
	}
}

func (taskTool) Present(args json.RawMessage) tool.Presentation {
	batch, err := parseBatch(args)
	if err != nil {
		return tool.Presentation{Title: i18n.T("Run subagents"), Input: string(args)}
	}
	title := i18n.Tf("Run %d subagents", len(batch.tasks))
	if len(batch.tasks) == 1 {
		title = i18n.T("Run a subagent")
	}
	lines := make([]string, len(batch.tasks))
	for i, item := range batch.tasks {
		lines[i] = taskLine(item.task)
	}
	return tool.Presentation{Title: title, Input: strings.Join(lines, "\n")}
}

// outcome is how one child ended.
type outcome struct {
	status string
	output string
	usage  llm.Usage
}

func (taskTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	if env.Spawner == nil {
		return tool.Failed("Subagents are not available in this session.")
	}
	batch, err := parseBatch(call.Args)
	if err != nil {
		return tool.Failed(err.Error())
	}
	write := mayWrite(env)
	work := make([]protocol.AgentWork, len(batch.tasks))
	reqs := make([]tool.SpawnRequest, len(batch.tasks))
	for i, item := range batch.tasks {
		work[i] = protocol.AgentWork{ID: item.name, Name: item.agent.Name, Task: taskLine(item.task), Status: protocol.ToolPending}
		reqs[i] = tool.SpawnRequest{
			ID: item.name, Name: item.agent.Name, Task: item.task, Context: batch.context,
			Write: write && !item.agent.ReadOnly, Parent: call.ID,
		}
	}
	b := newBoard(work, func(agents []protocol.AgentWork) {
		if env.Emit != nil {
			env.Emit(protocol.ToolData{Agents: agents})
		}
	}, nil)
	b.mu.Lock()
	b.publish()
	b.mu.Unlock()

	results := make([]outcome, len(reqs))
	slots := make(chan struct{}, concurrency(env, env.Session.Model))
	var wg sync.WaitGroup
spawn:
	for i := range reqs {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			break spawn
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-slots }()
			results[i] = runChild(ctx, env.Spawner, b, i, reqs[i])
		}(i)
	}
	wg.Wait()
	b.stop()

	return tellResults(env, b.snapshot(), results)
}

// runChild runs child i to its end and puts it on the board.
func runChild(ctx context.Context, spawner tool.Spawner, b *board, i int, req tool.SpawnRequest) outcome {
	if ctx.Err() != nil {
		return outcome{status: protocol.ToolStopped}
	}
	b.start(i)
	res, err := spawner.Spawn(ctx, req, func(latest protocol.AgentWork) { b.report(i, latest) })
	tokens := res.Usage.Input + res.Usage.Output
	out := outcome{usage: res.Usage, output: res.Output}
	switch {
	case err != nil && ctx.Err() != nil:
		out.status = protocol.ToolStopped
	case err != nil:
		out.status, out.output = protocol.ToolFailed, err.Error()
	case res.Failed:
		out.status = protocol.ToolFailed
	default:
		out.status = protocol.ToolDone
	}
	b.finish(i, out.status, res.Work, out.output, tokens)
	return out
}

// tellResults is the parent's tool result: each child's output in a
// task-result block, cut so the whole batch fits the window's share.
func tellResults(env tool.Env, final []protocol.AgentWork, results []outcome) tool.Result {
	told := env.Limits.Told
	if told <= 0 {
		told = env.Profile.Limits().Told
	}
	per := min(inlineBytes, max(minChildBytes, told/max(len(results), 1)-128))
	var text, shown strings.Builder
	var usage llm.Usage
	failed := 0
	for i, work := range final {
		res := results[i]
		if res.status == "" {
			res.status = work.Status
		}
		usage.Input += res.usage.Input
		usage.Output += res.usage.Output
		usage.CacheRead += res.usage.CacheRead
		usage.CacheWrite += res.usage.CacheWrite
		usage.Estimated = usage.Estimated || res.usage.Estimated
		if res.status != protocol.ToolDone {
			failed++
		}
		output := strings.TrimSpace(res.output)
		if output == "" {
			switch res.status {
			case protocol.ToolStopped:
				output = "Stopped before it finished."
			case protocol.ToolFailed:
				output = "Failed without saying why."
			default:
				output = "Returned nothing."
			}
		}
		if i > 0 {
			text.WriteString("\n")
			shown.WriteString("\n\n")
		}
		fmt.Fprintf(&text, "<task-result id=%q agent=%q status=%q ms=\"%d\">\n%s\n</task-result>",
			work.ID, work.Name, res.status, work.Millis, tool.HeadTail(output, per/2, per/2))
		fmt.Fprintf(&shown, "%s: %s", work.ID, output)
	}
	shownText, _ := tool.ClipRunes(shown.String(), tool.ShownRunes)
	return tool.Result{
		Text:    text.String(),
		Shown:   shownText,
		Agents:  final,
		Usage:   usage,
		IsError: failed == len(final),
	}
}

// concurrency is how many children run at once: the Spawner's slots for the
// model when it knows them, else one for a local or light model (one
// llama.cpp slot) and MaxConcurrent for a remote one.
func concurrency(env tool.Env, model string) int {
	if s, ok := env.Spawner.(Slotter); ok {
		return min(max(s.Slots(model), 1), MaxConcurrent)
	}
	if env.Profile.Compact() || localModel(env, model) {
		return 1
	}
	return MaxConcurrent
}

// localModel reports a model on a local provider. A route counts as local
// unless remote is first, since routes try local models first.
func localModel(env tool.Env, model string) bool {
	if model == "" || protocol.IsRoute(model) {
		return env.Settings == nil || !env.Settings().Providers.Models.RemoteFirst
	}
	provider, _, _ := strings.Cut(model, "/")
	for _, local := range protocol.LocalProviders {
		if provider == local {
			return true
		}
	}
	return false
}

// mayWrite reports whether children may change files: never under a
// read-only profile (plan, Ask read only, a read-only parent) nor under
// read-only approvals; their changes still pass the parent's Permissions.
func mayWrite(env tool.Env) bool {
	if env.Profile.ReadOnly {
		return false
	}
	approvals := env.Session.Approvals
	if approvals == "" && env.Settings != nil {
		approvals = env.Settings().Agent.Approvals
	}
	return approvals != protocol.ApprovalsReadOnly
}

// item is one task of a batch.
type item struct {
	name  string
	agent Agent
	task  string
}

type batch struct {
	context string
	tasks   []item
}

// rawItem accepts what models write for one task: the canonical fields plus
// the common stand-ins for them.
type rawItem struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Agent       string `json:"agent"`
	Task        string `json:"task"`
	Assignment  string `json:"assignment"`
	Prompt      string `json:"prompt"`
	Description string `json:"description"`
}

func (r rawItem) text() string {
	for _, text := range []string{r.Task, r.Assignment, r.Prompt, r.Description} {
		if strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

// parseBatch reads the batch shape {context, tasks[]} and the single-task
// shape {name, agent, task}; tasks may be strings, objects, or a JSON array
// written as a string.
func parseBatch(raw json.RawMessage) (batch, error) {
	var args struct {
		Context string          `json:"context"`
		Tasks   json.RawMessage `json:"tasks"`
		rawItem
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return batch{}, fmt.Errorf("The arguments are not a JSON object: %v", err)
	}
	items, err := parseItems(args.Tasks)
	if err != nil {
		return batch{}, err
	}
	if len(items) == 0 && args.rawItem.text() != "" {
		items = []rawItem{args.rawItem}
	}
	if len(items) == 0 {
		return batch{}, errors.New("Give tasks: a list with one complete task per item.")
	}
	if len(items) > MaxTasks {
		return batch{}, fmt.Errorf("At most %d tasks per call; send the other %d in a later call.", MaxTasks, len(items)-MaxTasks)
	}
	out := batch{context: strings.TrimSpace(args.Context)}
	used := make(map[string]bool)
	for n, r := range items {
		text := r.text()
		if text == "" {
			return batch{}, fmt.Errorf("Task %d is empty: give its task text.", n+1)
		}
		agent, ok := Lookup(r.Agent)
		if !ok {
			return batch{}, fmt.Errorf("Task %d names the agent %q; use scout or worker.", n+1, r.Agent)
		}
		name := camelName(r.Name)
		if name == "" {
			name = camelName(r.ID)
		}
		if name == "" {
			name = camelName(text)
		}
		if name == "" {
			name = fmt.Sprintf("Task%d", n+1)
		}
		name = unique(name, used)
		out.tasks = append(out.tasks, item{name: name, agent: agent, task: text})
	}
	return out, nil
}

func parseItems(raw json.RawMessage) ([]rawItem, error) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("tasks is not readable: %v", err)
		}
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "[") {
			return parseItems(json.RawMessage(s))
		}
		if s == "" {
			return nil, nil
		}
		return []rawItem{{Task: s}}, nil
	}
	if raw[0] == '{' {
		var one rawItem
		if err := json.Unmarshal(raw, &one); err != nil {
			return nil, fmt.Errorf("tasks is not readable: %v", err)
		}
		return []rawItem{one}, nil
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("tasks must be a list: %v", err)
	}
	items := make([]rawItem, 0, len(list))
	for _, entry := range list {
		entry = json.RawMessage(strings.TrimSpace(string(entry)))
		var r rawItem
		if len(entry) > 0 && entry[0] == '"' {
			if err := json.Unmarshal(entry, &r.Task); err != nil {
				return nil, fmt.Errorf("a task is not readable: %v", err)
			}
		} else if err := json.Unmarshal(entry, &r); err != nil {
			return nil, fmt.Errorf("a task is not readable: %v", err)
		}
		items = append(items, r)
	}
	return items, nil
}

// camelName makes a CamelCase name of at most nameRunes from the first words
// of text: "map the IPC server" is MapTheIPCServer.
func camelName(text string) string {
	words := strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	var b strings.Builder
	count := 0
	for _, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		if count+len(runes) > nameRunes {
			break
		}
		b.WriteString(string(runes))
		count += len(runes)
		if count >= 24 {
			break
		}
	}
	name := b.String()
	if name != "" && unicode.IsDigit([]rune(name)[0]) {
		name = "T" + name
		if len([]rune(name)) > nameRunes {
			name = string([]rune(name)[:nameRunes])
		}
	}
	return name
}

func unique(name string, used map[string]bool) string {
	candidate := name
	for n := 2; used[candidate]; n++ {
		suffix := fmt.Sprint(n)
		base := []rune(name)
		if len(base)+len(suffix) > nameRunes {
			base = base[:nameRunes-len(suffix)]
		}
		candidate = string(base) + suffix
	}
	used[candidate] = true
	return candidate
}

// taskLine is a task in one line for its box: the first line with words,
// heading marks dropped.
func taskLine(task string) string {
	for _, line := range strings.Split(task, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#*-> "))
		if line != "" {
			return protocol.ClipWords(line, taskLineRunes)
		}
	}
	return ""
}
