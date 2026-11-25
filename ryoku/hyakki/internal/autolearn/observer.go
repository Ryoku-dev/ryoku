package autolearn

// Derived from oh-my-pi (MIT), packages/coding-agent/src/autolearn/controller.ts@c7fff67
// Derived from oh-my-pi (MIT), packages/coding-agent/src/prompts/system/autolearn-nudge-autocontinue.md@c7fff67

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
	"ryoku-rashin/internal/skills"
)

// MinToolCalls is how many tool calls of its own a turn needs before a
// capture looks at it: a shorter turn rarely holds a procedure.
const MinToolCalls = 5

// Capture limits: the request stays well inside a 16K window, and a local
// model gets minutes for its first byte.
const (
	captureTimeout   = 5 * time.Minute
	captureMaxTokens = 1536
	captureTemp      = 0.2
	promptBytes      = 1500
	replyBytes       = 1000
	argsBytes        = 240
	resultBytes      = 200
	maxSteps         = 40
	maxListed        = 20
)

// saveTool is the one tool a capture offers.
const saveTool = "save_skill"

// captureSystem is omp's capture nudge rewritten for small models: one
// task, the output shape first, positive rules, one example.
const captureSystem = `You keep a library of reusable procedures for a coding assistant. Read the finished task in <task>, <steps> and <reply>.
When it shows a procedure worth repeating next time, such as a setup sequence, a debugging recipe or a project workflow, call save_skill once.
Name it in kebab-case, start the description with "Use when", and write the body as short numbered steps with the exact commands.
To improve a skill listed in <skills>, use its name.
When the task holds nothing worth repeating, answer with the word none.

Example call:
save_skill {"name": "rebuild-vendored-go-module", "description": "Use when Go builds fail on missing modules in a vendored repository.", "body": "1. Run ` + "`go mod vendor`" + `.\n2. Build again with ` + "`go build -mod=vendor ./...`" + `."}`

var saveSpec = llm.Tool{
	Name:        saveTool,
	Description: "Save a reusable procedure as a skill.",
	Parameters: json.RawMessage(`{"type":"object","properties":{` +
		`"name":{"type":"string","description":"kebab-case name"},` +
		`"description":{"type":"string","description":"one line starting with Use when"},` +
		`"body":{"type":"string","description":"numbered markdown steps with the exact commands"}},` +
		`"required":["name","description","body"],"additionalProperties":false}`),
}

// Observer is the auto-learn engine.TurnObserver. One capture runs at a
// time; a turn that ends while one runs waits as the single pending one,
// a later turn replacing it.
type Observer struct {
	client   llm.Client
	managed  *skills.Managed
	settings func() protocol.Settings
	events   core.Broadcaster

	mu      sync.Mutex
	running bool
	pending *engine.TurnRecord
	closed  bool
	done    sync.WaitGroup
}

// NewObserver captures through client (the gateway, asked with Light) into
// managed, while settings has feature autolearn on. events, when set, hears
// learning.changed after a skill is written.
func NewObserver(client llm.Client, managed *skills.Managed, settings func() protocol.Settings, events core.Broadcaster) *Observer {
	return &Observer{client: client, managed: managed, settings: settings, events: events}
}

// TurnEnded starts a capture in the background when the turn qualifies.
func (o *Observer) TurnEnded(ctx context.Context, rec engine.TurnRecord) {
	if !o.eligible(rec) {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	if o.running {
		o.pending = &rec
		return
	}
	o.running = true
	o.done.Add(1)
	go o.loop(ctx, rec)
}

// Busy reports a capture at work, so the daemon does not exit under it.
func (o *Observer) Busy() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.running
}

// Close drops any pending capture and waits for the running one.
func (o *Observer) Close() {
	o.mu.Lock()
	o.closed = true
	o.pending = nil
	o.mu.Unlock()
	o.done.Wait()
}

func (o *Observer) loop(ctx context.Context, rec engine.TurnRecord) {
	defer o.done.Done()
	for {
		if o.on() {
			if err := o.capture(ctx, rec); err != nil && ctx.Err() == nil {
				// a failed capture leaves everything as it was
				log.Printf("rashin: auto-learn capture for session %s: %v", rec.Session.ID, err)
			}
		}
		o.mu.Lock()
		next := o.pending
		o.pending = nil
		if next == nil || o.closed || ctx.Err() != nil {
			o.running = false
			o.mu.Unlock()
			return
		}
		o.mu.Unlock()
		rec = *next
	}
}

func (o *Observer) on() bool {
	return o.settings != nil && o.settings().Features[protocol.FeatureAutolearn]
}

// eligible is omp's stop check: the feature on, a turn the user asked for
// (no goal continuation) that ended normally, outside plan review and
// subagents, with enough calls of its own, none of which already learned.
func (o *Observer) eligible(rec engine.TurnRecord) bool {
	if o.client == nil || o.managed == nil || !o.on() || rec.Session.Temporary || rec.Goal {
		return false
	}
	switch rec.Profile.Mode {
	case tool.ModePlan, tool.ModeSubagent, tool.ModeTeam, tool.ModeDreamManager:
		return false
	}
	switch rec.Stop {
	case protocol.StopCancelled, protocol.StopError:
		return false
	}
	if strings.TrimSpace(rec.Prompt) == "" {
		return false
	}
	own := 0
	for _, c := range rec.Calls {
		if c.Parent != "" {
			continue
		}
		if c.Name == tool.Learn || c.Name == tool.ManageSkill {
			return false
		}
		own++
	}
	return own >= MinToolCalls
}

// capture asks one light model call about the turn and saves what it
// drafts; a turn that held nothing reusable writes nothing.
func (o *Observer) capture(ctx context.Context, rec engine.TurnRecord) error {
	ctx, cancel := context.WithTimeout(ctx, captureTimeout)
	defer cancel()
	temperature := captureTemp
	stream, err := o.client.Stream(ctx, llm.Request{
		Model: protocol.RouteAuto, Light: true, LocalOnly: true,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: captureSystem},
			{Role: llm.RoleUser, Content: turnText(rec, o.managed.Visible(nil))},
		},
		Tools: []llm.Tool{saveSpec}, ToolChoice: llm.ToolChoiceAuto,
		Temperature: &temperature, MaxTokens: captureMaxTokens,
	})
	if err != nil {
		return err
	}
	defer stream.Close()
	var text strings.Builder
	var calls []llm.ToolCall
	for {
		delta, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		text.WriteString(delta.Text)
		calls = append(calls, delta.ToolCalls...)
	}
	if stream.Final().Finish == llm.FinishLength || stream.Final().Finish == llm.FinishFilter {
		return nil
	}
	d, ok := parseDraft(calls, text.String())
	if !ok || ctx.Err() != nil || !o.on() {
		return nil
	}
	return o.save(d)
}

// save writes a draft as a managed skill: a new one, or the one of its name
// improved. A name a built-in skill claims is left alone.
func (o *Observer) save(d draft) error {
	name, err := skills.SkillName(d.Name)
	if err != nil {
		return err
	}
	if o.managed.Claimed(name) {
		return nil
	}
	if _, _, err := o.managed.Save(name, d.Description, d.Body); err != nil {
		return err
	}
	if o.events != nil {
		o.events.Broadcast(protocol.EvtLearningChanged, protocol.LearningChanged{Kind: protocol.LearningSkills})
	}
	return nil
}

// draft is the skill a capture proposes.
type draft struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

// parseDraft reads the model's answer: a save_skill call, else a JSON
// object in its text (a small model may write the call out), else nothing.
func parseDraft(calls []llm.ToolCall, text string) (draft, bool) {
	for _, call := range calls {
		if call.Name == saveTool {
			if d, ok := decodeDraft(call.Arguments); ok {
				return d, true
			}
		}
	}
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, saveTool) {
		text = strings.TrimSpace(strings.TrimPrefix(text, saveTool))
	} else if !strings.HasPrefix(text, `{"name":"save_skill"`) && !strings.HasPrefix(text, `{"name": "save_skill"`) {
		return draft{}, false
	}
	start, end := strings.IndexByte(text, '{'), strings.LastIndexByte(text, '}')
	if start < 0 || end <= start {
		return draft{}, false
	}
	return decodeDraft(text[start : end+1])
}

func decodeDraft(raw string) (draft, bool) {
	var d struct {
		draft
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal([]byte(raw), &d) != nil {
		return draft{}, false
	}
	if len(d.Arguments) > 0 {
		// {"name": "save_skill", "arguments": {...}}, the call written out
		var inner draft
		if json.Unmarshal(d.Arguments, &inner) == nil {
			d.draft = inner
		}
	}
	if strings.TrimSpace(d.Name) == "" || strings.TrimSpace(d.Description) == "" || strings.TrimSpace(d.Body) == "" {
		return draft{}, false
	}
	return d.draft, true
}

// turnText is the finished turn as the capture reads it: what the user
// asked, the session's own calls with their results cut short, the reply,
// and the managed skills it may improve instead.
func turnText(rec engine.TurnRecord, existing []protocol.ManagedSkill) string {
	var b strings.Builder
	values := redact.Snapshot()
	prompt, _ := tool.ClipBytes(values.Strict(strings.TrimSpace(rec.Prompt)), promptBytes)
	b.WriteString("<task>\n" + prompt + "\n</task>\n<steps>\n")
	step := 0
	for _, c := range rec.Calls {
		if c.Parent != "" {
			continue
		}
		if step == maxSteps {
			b.WriteString("...\n")
			break
		}
		step++
		args, _ := tool.ClipBytes(values.Strict(oneLine(string(c.Args))), argsBytes)
		result, _ := tool.ClipBytes(values.Strict(oneLine(c.Result)), resultBytes)
		outcome := "ok"
		if c.IsError {
			outcome = "failed"
		}
		b.WriteString(strconv.Itoa(step) + ". " + c.Name + " " + args + " -> " + outcome)
		if result != "" {
			b.WriteString(": " + result)
		}
		b.WriteByte('\n')
	}
	reply, _ := tool.ClipBytes(values.Strict(strings.TrimSpace(rec.Text)), replyBytes)
	b.WriteString("</steps>\n<reply>\n" + reply + "\n</reply>\n<skills>\n")
	if len(existing) == 0 {
		b.WriteString("none\n")
	}
	for i, s := range existing {
		if i == maxListed {
			break
		}
		b.WriteString("- " + s.Name + ": " + s.Description + "\n")
	}
	b.WriteString("</skills>")
	return b.String()
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
