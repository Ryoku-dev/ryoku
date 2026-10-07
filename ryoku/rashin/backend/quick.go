package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// quick.go is the fast lane for launcher asks: a fabric-style pattern sent
// through Prowl's gateway. No Python spawn, no full agent: most questions come
// back in a second or two. The model may call a small set of read-only Go tools
// (quicktools.go) for live state; anything heavier escalates to the configured
// chat session.

const toolsSentinel = "TOOLS_REQUIRED"

// maxToolRounds bounds the fast-lane agent loop so a quick ask stays quick.
const maxToolRounds = 4

const quickPattern = `You are the Needle, Rashin's resident agent on this Ryoku (Arch Linux) machine, answering a quick ask from the launcher. This is the Ryoku lane: every question is about this machine, its desktop, or how to work on it.

You have read-only tools for live state: system_query (packages, updates, service, processes, disk, kernel, gpu, network), read_file, list_dir, search_code (the Ryoku source, via prowl), and fetch_url (public web pages). Use them when the map below is not enough, then answer.

Rules:
- Reply with just the answer: one or two sentences, or a tight list. No preamble, no follow-up questions, no markdown headers.
- The machine map below is current; prefer it and your tools over guessing. It names the window manager that runs here (Hyprland or niri); never assume one.
- Answer how-do-I desktop questions GUI-first: name the Ryoku Hub page (Super+comma, or "ryoku-shell hub open <section>"), the Super+W wallpaper/theme picker, or QS Bar Settings for the bar and dock, then the command behind it.
- When the user is new to Linux, Ryoku, or the compositor, point at the matching wiki page under ~/.local/share/ryoku/rashin/wiki/ (linux-basics, desktop, hyprland-lua, niri-kdl, quickshell-qml, go-tools, rashin) after the direct answer.
- Only escalate when the request needs something your tools cannot do: generating or editing files or images, an interactive browser, running a skill, or any action that changes the system. In that case reply with exactly TOOLS_REQUIRED and nothing else; the full agent in this lane picks the job up with its skills and the user's approval settings.`

type quickTarget struct {
	Route string
	Label string
}

type quickRouteJSON struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Sub   string `json:"sub"`
}

type prowlSetupHarness struct {
	ID       string `json:"id"`
	Injected bool   `json:"injected"`
	Active   bool   `json:"active"`
	Skills   string `json:"skills"`
	Note     string `json:"note,omitempty"`
}

type prowlSetupHarnesses struct {
	Routable  bool                `json:"routable"`
	Reason    string              `json:"reason,omitempty"`
	Harnesses []prowlSetupHarness `json:"harnesses"`
}

var quickAxes = []quickRouteJSON{
	{ID: "auto:smart", Label: "Smart", Sub: "Prioritize capability"},
	{ID: "auto:fast", Label: "Fast", Sub: "Prioritize response speed"},
	{ID: "auto:cheap", Label: "Cheap", Sub: "Prioritize lower cost"},
	{ID: "auto:reliable", Label: "Reliable", Sub: "Prioritize availability"},
	{ID: "auto:balanced", Label: "Balanced", Sub: "Balance capability, speed, and cost"},
	{ID: "auto:efficient", Label: "Efficient", Sub: "Favor capable models with lower cost"},
}

func gatewayRouting(ctx context.Context) (prowlSetupHarnesses, error) {
	var out prowlSetupHarnesses
	err := prowlGatewayJSON(ctx, http.MethodGet, "/api/setup/harnesses", nil, &out)
	return out, err
}

func quickGatewayError(err error) error {
	var gatewayErr *prowlGatewayError
	if errors.As(err, &gatewayErr) {
		if gatewayErr.Code == "gateway_down" {
			return errors.New("Prowl's gateway is not running")
		}
		if gatewayErr.Message != "" {
			return errors.New(gatewayErr.Message)
		}
	}
	return err
}

func resolveQuickTarget(cfg Config) (quickTarget, error) {
	route := strings.ToLower(strings.TrimSpace(cfg.Quick.Route))
	if route == "" {
		route = "auto"
	}
	target := quickTarget{Route: route, Label: route}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	routing, err := gatewayRouting(ctx)
	if err != nil {
		return target, quickGatewayError(err)
	}
	if !routing.Routable {
		return target, errors.New("Prowl has no provider connected; open Prowl > Providers in Rashin")
	}
	return target, nil
}

func prowlProfileSets(ctx context.Context) (string, []string, error) {
	type profile struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		ModelCount int    `json:"modelCount"`
	}
	var profiles []profile
	if err := prowlGatewayJSON(ctx, http.MethodGet, "/api/profiles", nil, &profiles); err != nil {
		return "", nil, err
	}
	var active struct {
		ID *int64 `json:"activeProfileId"`
	}
	if err := prowlGatewayJSON(ctx, http.MethodGet, "/api/profiles/active", nil, &active); err != nil {
		return "", nil, err
	}
	activeName := "No active set"
	if active.ID != nil {
		for _, p := range profiles {
			if p.ID == *active.ID {
				activeName = p.Name
				break
			}
		}
	}
	sort.SliceStable(profiles, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(profiles[i].Name))
		right := strings.ToLower(strings.TrimSpace(profiles[j].Name))
		return left < right
	})
	names := make([]string, 0, len(profiles))
	seen := map[string]bool{}
	for _, p := range profiles {
		if p.ModelCount == 0 {
			continue
		}
		id := strings.ToLower(strings.TrimSpace(p.Name))
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		names = append(names, p.Name)
	}
	return activeName, names, nil
}

// Tests replace this seam so a chat session never reaches the live gateway.
var chatProwlProfileSets = prowlProfileSets

func quickRoutes(ctx context.Context) ([]quickRouteJSON, error) {
	activeName, names, err := prowlProfileSets(ctx)
	if err != nil {
		return append([]quickRouteJSON{{ID: "auto", Label: "Active set", Sub: "Unavailable"}}, quickAxes...), err
	}
	routes := append([]quickRouteJSON{{ID: "auto", Label: "Active set", Sub: activeName}}, quickAxes...)
	seen := map[string]bool{}
	for _, route := range routes {
		seen[route.ID] = true
	}
	for _, name := range names {
		id := "auto:" + strings.ToLower(strings.TrimSpace(name))
		if seen[id] {
			continue
		}
		seen[id] = true
		routes = append(routes, quickRouteJSON{ID: id, Label: name, Sub: "Routing set"})
	}
	return routes, nil
}

func validateQuickRoute(ctx context.Context, route string) (string, error) {
	route = strings.ToLower(strings.TrimSpace(route))
	if route == "" {
		return "", errors.New("route is required")
	}
	if route == "auto" {
		return route, nil
	}
	for _, axis := range quickAxes {
		if route == axis.ID {
			return route, nil
		}
	}
	routes, err := quickRoutes(ctx)
	if err != nil {
		return "", quickGatewayError(err)
	}
	for _, candidate := range routes {
		if route == candidate.ID && strings.HasPrefix(route, "auto:") {
			return route, nil
		}
	}
	return "", fmt.Errorf("unknown quick route %q", route)
}

func setQuickRoute(ctx context.Context, route string) error {
	route, err := validateQuickRoute(ctx, route)
	if err != nil {
		return err
	}
	cfg := LoadConfig()
	cfg.Quick.Route = route
	return SaveConfig(cfg)
}

func envFileValue(path, key string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if v, ok := strings.CutPrefix(line, key+"="); ok {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

var retiredQuickKeys = []struct {
	Env      string
	Platform string
}{
	{Env: "OPENROUTER_API_KEY", Platform: "openrouter"},
	{Env: "OPENAI_API_KEY"},
	{Env: "GROQ_API_KEY", Platform: "groq"},
	{Env: "DEEPSEEK_API_KEY", Platform: "deepseek"},
	{Env: "MISTRAL_API_KEY", Platform: "mistral"},
	{Env: "TOGETHER_API_KEY", Platform: "together-ai"},
	{Env: "XAI_API_KEY", Platform: "xai"},
	{Env: "CEREBRAS_API_KEY", Platform: "cerebras"},
	{Env: "OLLAMA_API_KEY", Platform: "ollama"},
	{Env: "LOCAL_API_KEY"},
}

func quickKeyMigrationMarker() string {
	return filepath.Join(xdgState(), "ryoku", "rashin", "prowl-quick-keys-v1")
}

func migrateQuickProviderKeys(ctx context.Context) error {
	marker := quickKeyMigrationMarker()
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	envPath := filepath.Join(filepath.Dir(ConfigPath()), "rashin.env")
	pending := make(map[string]string)
	for _, retired := range retiredQuickKeys {
		key := envFileValue(envPath, retired.Env)
		if key == "" {
			continue
		}
		if retired.Platform == "" {
			fmt.Fprintf(os.Stderr, "ryoku-rashin: not importing %s: Prowl has no adapter for it\n", retired.Env)
			continue
		}
		pending[retired.Platform] = key
	}
	if len(pending) > 0 {
		var existing []struct {
			Platform string `json:"platform"`
		}
		if err := prowlGatewayJSON(ctx, http.MethodGet, "/api/keys", nil, &existing); err != nil {
			return err
		}
		for _, key := range existing {
			delete(pending, key.Platform)
		}
		for platform, key := range pending {
			body := map[string]string{
				"platform": platform,
				"key":      key,
				"label":    "imported from rashin",
			}
			if err := prowlGatewayJSON(ctx, http.MethodPost, "/api/keys", body, nil); err != nil {
				return err
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte("done\n"), 0o600)
}

// vaultQuickContext inlines the generated maps (fence bodies only) as the
// pattern's knowledge. Small by construction; capped defensively.
func vaultQuickContext() string {
	var b strings.Builder
	for _, name := range []string{"system.md", "desktop.md", "user.md", "habits.md"} {
		raw, err := ReadVaultFile(name)
		if err != nil {
			continue
		}
		body := string(raw)
		if bi := strings.Index(body, vaultFenceBegin); bi >= 0 {
			if ei := strings.Index(body, vaultFenceEnd); ei > bi {
				body = body[bi+len(vaultFenceBegin) : ei]
			}
		}
		body = strings.TrimSpace(body)
		if len(body) > 8*1024 {
			body = body[:8*1024]
		}
		fmt.Fprintf(&b, "## %s\n%s\n\n", name, body)
	}
	return b.String()
}

// chatMessage is one turn in the fast-lane conversation.
type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// laneSpec parameterizes the fast-lane agent loop: the quick (launcher) and
// terminal lanes share the loop, the streaming, and the escalation sentinel,
// differing in persona, toolset, and prior turns.
type laneSpec struct {
	system  string                                              // full system prompt
	tools   []map[string]any                                    // advertised schemas
	exec    func(ctx context.Context, name, args string) string // tool executor
	history []chatMessage                                       // prior exchange (continuation)
}

func quickSpec() laneSpec {
	return laneSpec{
		system: quickPattern + "\n\n# The machine map\n\n" + vaultQuickContext(),
		tools:  quickToolSchemas(),
		exec:   execQuickTool,
	}
}

// quickComplete runs the launcher fast lane; see laneComplete.
func quickComplete(ctx context.Context, t quickTarget, question string,
	onDelta func(string), onTool func(id, title, status string)) (string, error) {
	return laneComplete(ctx, t, quickSpec(), question, onDelta, onTool)
}

// laneComplete runs the fast-lane agent loop: up to maxToolRounds of tool
// calls, then the final answer. onDelta streams the answer text (after the
// sentinel is ruled out); onTool fires as each tool runs so the launcher,
// terminal, and dashboard can show what it is doing.
func laneComplete(ctx context.Context, t quickTarget, spec laneSpec, question string,
	onDelta func(string), onTool func(id, title, status string)) (string, error) {
	msgs := make([]chatMessage, 0, len(spec.history)+2)
	msgs = append(msgs, chatMessage{Role: "system", Content: spec.system})
	msgs = append(msgs, spec.history...)
	msgs = append(msgs, chatMessage{Role: "user", Content: question})
	for round := 0; round <= maxToolRounds; round++ {
		// The last round forbids tools so the model must answer.
		tools := spec.tools
		if round == maxToolRounds {
			tools = nil
		}
		text, calls, err := quickRound(ctx, t, msgs, tools, onDelta)
		if err != nil {
			return "", err
		}
		if len(calls) == 0 {
			out := strings.TrimSpace(text)
			if strings.HasPrefix(out, toolsSentinel) {
				return "", errNeedsTools
			}
			if out == "" {
				return "", fmt.Errorf("empty answer")
			}
			return out, nil
		}
		// Record the assistant's tool-call turn, then each tool result.
		msgs = append(msgs, chatMessage{Role: "assistant", Content: text, ToolCalls: calls})
		for _, c := range calls {
			if onTool != nil {
				onTool(c.ID, toolTitle(c), "in_progress")
			}
			result := spec.exec(ctx, c.Function.Name, c.Function.Arguments)
			if onTool != nil {
				onTool(c.ID, toolTitle(c), "completed")
			}
			msgs = append(msgs, chatMessage{Role: "tool", ToolCallID: c.ID, Content: result})
		}
	}
	return "", fmt.Errorf("tool loop did not converge")
}

func toolTitle(c toolCall) string {
	var a struct {
		Topic, Arg, Path, Query, URL string
	}
	_ = json.Unmarshal([]byte(c.Function.Arguments), &a)
	switch c.Function.Name {
	case "system_query":
		if a.Arg != "" {
			return "checking " + a.Topic + ": " + a.Arg
		}
		return "checking " + a.Topic
	case "read_file":
		return "reading " + a.Path
	case "list_dir":
		return "listing " + a.Path
	case "search_code":
		return "searching code: " + a.Query
	case "propose":
		return "shaping the commands"
	case "fetch_url":
		return "fetching " + a.URL
	default:
		return c.Function.Name
	}
}

// quickRound performs one streaming model call, assembling both the content
// (streamed via onDelta, head held until the sentinel is ruled out) and any
// tool_calls (fragmented across deltas by index). Streaming keeps the answer
// fading in even though the loop can also call tools.
func quickRound(ctx context.Context, t quickTarget, msgs []chatMessage, tools []map[string]any,
	onDelta func(string)) (string, []toolCall, error) {
	payload := map[string]any{
		"model":    t.Route,
		"stream":   true,
		"messages": msgs,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		prowlGatewayBase()+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+prowlGatewayToken())
	resp, err := prowlGatewayHTTPClient.Do(req)
	if err != nil {
		return "", nil, errors.New("Prowl's gateway is not running")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := readCapped(resp, 2048)
		var envelope struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(msg), &envelope) == nil {
			if envelope.Error.Code == "not_routable" || envelope.Error.Code == "no_available_model" {
				return "", nil, errors.New("Prowl has no provider connected; open Prowl > Providers in Rashin")
			}
			if envelope.Error.Message != "" {
				return "", nil, errors.New(envelope.Error.Message)
			}
		}
		return "", nil, fmt.Errorf("Prowl gateway returned %d: %s", resp.StatusCode, msg)
	}

	var answer strings.Builder
	held := true
	byIndex := map[int]*toolCall{}
	var order []int
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil || len(chunk.Choices) == 0 {
			continue
		}
		d := chunk.Choices[0].Delta
		for _, tc := range d.ToolCalls {
			cur, ok := byIndex[tc.Index]
			if !ok {
				cur = &toolCall{Type: "function"}
				byIndex[tc.Index] = cur
				order = append(order, tc.Index)
			}
			if tc.ID != "" {
				cur.ID = tc.ID
			}
			if tc.Function.Name != "" {
				cur.Function.Name = tc.Function.Name
			}
			cur.Function.Arguments += tc.Function.Arguments
		}
		if d.Content == "" {
			continue
		}
		answer.WriteString(d.Content)
		if held {
			head := strings.TrimSpace(answer.String())
			if strings.HasPrefix(toolsSentinel, head) || strings.HasPrefix(head, toolsSentinel) {
				continue
			}
			held = false
			if onDelta != nil {
				onDelta(answer.String())
			}
			continue
		}
		if onDelta != nil {
			onDelta(d.Content)
		}
	}
	var calls []toolCall
	for _, idx := range order {
		c := byIndex[idx]
		if c.Function.Name != "" {
			if c.ID == "" {
				c.ID = fmt.Sprintf("call_%d", idx)
			}
			calls = append(calls, *c)
		}
	}
	return strings.TrimSpace(answer.String()), calls, nil
}

var errNeedsTools = fmt.Errorf("needs tools")

func readCapped(resp *http.Response, n int) (string, error) {
	buf := make([]byte, n)
	m, err := resp.Body.Read(buf)
	return strings.TrimSpace(string(buf[:m])), err
}

// warmHermes spawns the shared session at daemon start, so neither the
// dashboard's first message nor a session-lane ask pays the Python cold
// start. Costs hermes's resident memory from boot; that is the point of an
// enabled agent OS.
func (h *chatHub) warm() {
	if !HermesStatus().Configured {
		return
	}
	h.mu.Lock()
	h.ensureConnLocked()
	h.mu.Unlock()
}

// quickTime bounds the fast lane; escalation needs time for real model calls.
const quickTimeout = 75 * time.Second
