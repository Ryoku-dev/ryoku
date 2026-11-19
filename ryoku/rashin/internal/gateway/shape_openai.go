package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/gateway/textcalls"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// OpenAI chat completions: the one wire shape every provider here speaks.
// renderOpenAI turns an llm.Request into it for the model a request is routed
// to, and sseAttempt reads its server-sent events back into llm.Deltas.

type wireMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content,omitempty"`
	Reasoning  string         `json:"reasoning_content,omitempty"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
}

type wireToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Arguments   string          `json:"arguments,omitempty"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []wireMessage `json:"messages"`
	Tools    []wireTool    `json:"tools,omitempty"`
	// ToolChoice is auto, none, required or {"type":"function",...}.
	ToolChoice any `json:"tool_choice,omitempty"`
	MaxTokens  int `json:"max_tokens,omitempty"`
	// MaxCompletionTokens replaces MaxTokens for a provider whose reasoning
	// models take only it.
	MaxCompletionTokens int      `json:"max_completion_tokens,omitempty"`
	Temperature         *float64 `json:"temperature,omitempty"`
	Stop                []string `json:"stop,omitempty"`
	ReasoningEffort     string   `json:"reasoning_effort,omitempty"`
	// TemplateArgs reach llama.cpp's chat template; enable_thinking false
	// turns reasoning off where reasoning_effort has no "none".
	TemplateArgs  map[string]any    `json:"chat_template_kwargs,omitempty"`
	Stream        bool              `json:"stream"`
	StreamOptions wireStreamOptions `json:"stream_options"`
	// Extra are a caller's own parameters, sent as they came.
	Extra map[string]json.RawMessage `json:"-"`
}

// MarshalJSON adds Extra beside the fields the gateway sets itself, which win.
func (r chatRequest) MarshalJSON() ([]byte, error) {
	type plain chatRequest
	body, err := json.Marshal(plain(r))
	if err != nil || len(r.Extra) == 0 {
		return body, err
	}
	fields := make(map[string]json.RawMessage, len(r.Extra)+8)
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	for name, value := range r.Extra {
		if _, set := fields[name]; !set {
			fields[name] = value
		}
	}
	return json.Marshal(fields)
}

// imageOmitted stands in for pictures a model without vision is not sent.
// The model reads it, so it stays English like the rest of the prompt.
const imageOmitted = "[image omitted: this model cannot read pictures]"

// renderOpenAI renders request for model m of provider p. A model with a
// text tool dialect takes its tools and history as text; a tool's pictures
// follow its results as one user message when m reads pictures, and are
// left out with a note when it does not.
func renderOpenAI(request llm.Request, p *provider, m *model) (chatRequest, error) {
	wire := chatRequest{
		Model: m.ID, MaxTokens: request.MaxTokens, Temperature: request.Temperature,
		Stream: true, StreamOptions: wireStreamOptions{IncludeUsage: true}, Extra: request.Extra,
	}
	messages, tools := request.Messages, request.Tools
	stop := append([]string(nil), request.Stop...)
	if m.ToolDialect != "" && len(tools) > 0 {
		messages = textcalls.Encode(m.ToolDialect, messages, tools)
		stop = append(stop, textcalls.Stops(m.ToolDialect)...)
		tools = nil
	}
	wire.Stop = stop
	replay := m.ReplayReasoning || (p.Tier == protocol.TierLocal && m.Reasoning)
	var pending []wireMessage
	flush := func() {
		if len(pending) > 0 {
			wire.Messages = append(wire.Messages, pending...)
			pending = nil
		}
	}
	for _, message := range messages {
		if message.Role != llm.RoleTool {
			flush()
		}
		m2 := wireMessage{Role: string(message.Role)}
		switch message.Role {
		case llm.RoleUser:
			m2.Content = userContent(message.Content, message.Images, m.Vision)
		case llm.RoleAssistant:
			m2.Content = message.Content
			if replay {
				m2.Reasoning = thinkingText(message.Thinking)
			}
			for _, call := range message.ToolCalls {
				m2.ToolCalls = append(m2.ToolCalls, wireToolCall{ID: call.ID, Type: "function", Function: wireFunction{Name: call.Name, Arguments: call.Arguments}})
			}
		case llm.RoleSystem:
			m2.Content = message.Content
		case llm.RoleTool:
			m2.ToolCallID, m2.Name = message.ToolCallID, message.Name
			text := message.Content
			if message.IsError {
				text = "Error: " + text
			}
			if len(message.Images) > 0 && !m.Vision {
				text = strings.TrimRight(text, "\n") + "\n" + imageOmitted
			}
			m2.Content = text
			wire.Messages = append(wire.Messages, m2)
			if len(message.Images) > 0 && m.Vision {
				// a tool message cannot carry pictures for most providers, so
				// they follow the run of tool results as the user's
				label := fmt.Sprintf("Images returned by %s (%s):", firstText(message.Name, "a tool"), message.ToolCallID)
				pending = append(pending, wireMessage{Role: string(llm.RoleUser), Content: userContent(label, message.Images, true)})
			}
			continue
		default:
			return chatRequest{}, fmt.Errorf(i18n.T("Unsupported chat role: %s"), message.Role)
		}
		wire.Messages = append(wire.Messages, m2)
	}
	flush()
	for _, tool := range tools {
		parameters := tool.Parameters
		if len(parameters) == 0 {
			parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		if !json.Valid(parameters) {
			return chatRequest{}, fmt.Errorf(i18n.T("Tool schema for %s is not valid JSON"), tool.Name)
		}
		wire.Tools = append(wire.Tools, wireTool{Type: "function", Function: wireFunction{Name: tool.Name, Description: tool.Description, Parameters: parameters}})
	}
	// a caller that says how tools are chosen is passed on as is
	if _, chosen := request.Extra["tool_choice"]; len(wire.Tools) > 0 && !chosen {
		wire.ToolChoice = toolChoice(request.ToolChoice)
	}
	if _, chosen := request.Extra["reasoning_effort"]; request.Reasoning != "" && m.Reasoning && !chosen {
		switch {
		case request.Reasoning == llm.EffortOff && llamaServer(p):
			wire.TemplateArgs = map[string]any{"enable_thinking": false}
		case request.Reasoning == llm.EffortOff:
			wire.ReasoningEffort = "none"
		default:
			wire.ReasoningEffort = request.Reasoning
		}
	}
	return wire, nil
}

// llamaServer is a provider served by llama.cpp's llama-server.
func llamaServer(p *provider) bool {
	return p.ID == protocol.ProviderRashinModels || p.ID == protocol.ProviderLlamaCpp
}

func toolChoice(choice string) any {
	switch choice {
	case "", llm.ToolChoiceAuto:
		return llm.ToolChoiceAuto
	case llm.ToolChoiceNone, llm.ToolChoiceRequired:
		return choice
	}
	return map[string]any{"type": "function", "function": map[string]string{"name": choice}}
}

// userContent is a user message's text, with its images as parts when the
// model reads them and a note in their place when it does not.
func userContent(text string, images []llm.Image, vision bool) any {
	if len(images) == 0 {
		return text
	}
	if !vision {
		return strings.TrimRight(text, "\n") + "\n" + imageOmitted
	}
	parts := []map[string]any{{"type": "text", "text": text}}
	for _, image := range images {
		mime := strings.TrimSpace(image.Mime)
		if mime == "" {
			mime = "application/octet-stream"
		}
		parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]string{"url": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(image.Data)}})
	}
	return parts
}

func thinkingText(blocks []llm.Thinking) string {
	var text strings.Builder
	for _, block := range blocks {
		text.WriteString(block.Text)
	}
	return text.String()
}

// userImages reports pictures the user sent: only a model with vision may
// answer them.
func userImages(request llm.Request) bool {
	for _, message := range request.Messages {
		if message.Role != llm.RoleTool && len(message.Images) > 0 {
			return true
		}
	}
	return false
}

// toolImages reports pictures a tool returned: a model with vision is
// preferred, one without is sent the results with the pictures left out.
func toolImages(request llm.Request) bool {
	for _, message := range request.Messages {
		if message.Role == llm.RoleTool && len(message.Images) > 0 {
			return true
		}
	}
	return false
}

// postOpenAI sends one rendered request and returns its event stream, or the
// provider's refusal with its status and body.
func (g *Gateway) postOpenAI(ctx context.Context, client *http.Client, endpoint, bearer string, p *provider, request chatRequest) (*sseAttempt, int, string, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, 0, "", err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, 0, "", err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "text/event-stream")
	if bearer != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+bearer)
	}
	for key, value := range p.ExtraHeaders {
		httpRequest.Header.Set(key, value)
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, 0, "", err
	}
	g.observeLimits(p, response.Header)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		data, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if readErr != nil {
			return nil, response.StatusCode, "", readErr
		}
		bodyText := string(data)
		retryAfter := parseRetryAfter(response.Header.Get("Retry-After"), g.now())
		return nil, response.StatusCode, bodyText, &upstreamError{status: response.StatusCode, body: bodyText, retryAfter: retryAfter}
	}
	return newSSEAttempt(response), 0, "", nil
}

type toolAccumulator struct {
	ID        string
	Name      string
	Arguments strings.Builder
	started   bool
}

// sseAttempt reads one provider's OpenAI event stream.
type sseAttempt struct {
	response   *http.Response
	cancel     context.CancelFunc
	scanner    *bufio.Scanner
	queue      []llm.Delta
	tools      map[int]*toolAccumulator
	done       bool
	emitted    int
	finish     string
	usage      llm.Usage
	usageKnown bool
	// cacheN is llama.cpp's count of prompt tokens served from its cache,
	// from the timings it adds to the last event
	cacheN  int
	promptN int
	close   sync.Once
}

func newSSEAttempt(response *http.Response) *sseAttempt {
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	return &sseAttempt{response: response, scanner: scanner, tools: make(map[int]*toolAccumulator)}
}

func (s *sseAttempt) Next() (llm.Delta, error) {
	if len(s.queue) > 0 {
		delta := s.queue[0]
		s.queue = s.queue[1:]
		s.emitted++
		return delta, nil
	}
	if s.done {
		return llm.Delta{}, io.EOF
	}
	for {
		data, err := s.nextEvent()
		if err != nil {
			if errors.Is(err, io.EOF) {
				s.flushTools()
				s.done = true
				if len(s.queue) > 0 {
					return s.Next()
				}
			}
			return llm.Delta{}, err
		}
		if data == "[DONE]" {
			s.flushTools()
			s.done = true
			if len(s.queue) > 0 {
				return s.Next()
			}
			return llm.Delta{}, io.EOF
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return llm.Delta{}, fmt.Errorf("decode provider stream: %w", err)
		}
		if chunk.Error != nil && len(chunk.Choices) == 0 {
			return llm.Delta{}, errors.New(chunk.Error.message())
		}
		if chunk.Usage != nil {
			s.usage = chunk.Usage.usage()
			s.usageKnown = true
		}
		if chunk.Timings != nil {
			s.cacheN, s.promptN = chunk.Timings.CacheN, chunk.Timings.PromptN
		}
		var delta llm.Delta
		for _, choice := range chunk.Choices {
			delta.Text += textFromJSON(choice.Delta.Content)
			delta.Reasoning += firstText(choice.Delta.ReasoningContent, choice.Delta.Reasoning, choice.Delta.Thinking)
			for _, call := range choice.Delta.ToolCalls {
				acc := s.tools[call.Index]
				if acc == nil {
					acc = &toolAccumulator{}
					s.tools[call.Index] = acc
				}
				if call.ID != "" && !acc.started {
					acc.ID = call.ID
				}
				if call.Function.Name != "" {
					acc.Name += call.Function.Name
				}
				acc.Arguments.WriteString(call.Function.Arguments)
				if !acc.started && acc.Name != "" {
					// the pending tool line shows while the arguments stream
					acc.started = true
					if acc.ID == "" {
						acc.ID = newCallID()
					}
					s.queue = append(s.queue, llm.Delta{ToolStart: &llm.ToolCall{ID: acc.ID, Name: acc.Name}})
				}
			}
			if choice.FinishReason != "" {
				s.finish = normalizeFinish(choice.FinishReason)
				s.flushTools()
			}
		}
		if delta.Text != "" || delta.Reasoning != "" {
			s.emitted++
			return delta, nil
		}
		if len(s.queue) > 0 {
			return s.Next()
		}
		return llm.Delta{}, nil
	}
}

func (s *sseAttempt) nextEvent() (string, error) {
	var lines []string
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if line == "" {
			if len(lines) > 0 {
				return strings.Join(lines, "\n"), nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			lines = append(lines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := s.scanner.Err(); err != nil {
		return "", err
	}
	if len(lines) > 0 {
		return strings.Join(lines, "\n"), nil
	}
	return "", io.EOF
}

func (s *sseAttempt) flushTools() {
	if len(s.tools) == 0 {
		return
	}
	indices := make([]int, 0, len(s.tools))
	for index := range s.tools {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	calls := make([]llm.ToolCall, 0, len(indices))
	for _, index := range indices {
		tool := s.tools[index]
		if tool.ID == "" {
			tool.ID = newCallID()
		}
		calls = append(calls, llm.ToolCall{ID: tool.ID, Name: tool.Name, Arguments: tool.Arguments.String()})
	}
	if s.finish == "" {
		s.finish = llm.FinishToolCalls
	}
	s.queue = append(s.queue, llm.Delta{ToolCalls: calls})
	s.tools = make(map[int]*toolAccumulator)
}

func (s *sseAttempt) outcome() attemptOutcome {
	usage := s.usage
	if s.cacheN > 0 && usage.CacheRead == 0 {
		usage.CacheRead = s.cacheN
		// llama.cpp counts only the tokens it evaluated as the prompt
		if usage.Input == s.promptN {
			usage.Input += s.cacheN
		}
	}
	return attemptOutcome{finish: s.finish, usage: usage, usageKnown: s.usageKnown, emitted: s.emitted}
}

func (s *sseAttempt) Close() error {
	var err error
	s.close.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		err = s.response.Body.Close()
	})
	return err
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          json.RawMessage `json:"content"`
			ReasoningContent string          `json:"reasoning_content"`
			Reasoning        string          `json:"reasoning"`
			Thinking         string          `json:"thinking"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage   *wireUsage `json:"usage"`
	Timings *struct {
		CacheN  int `json:"cache_n"`
		PromptN int `json:"prompt_n"`
	} `json:"timings"`
	Error *streamError `json:"error"`
}

// wireUsage is every way the providers here count tokens, prompt cache
// included: OpenAI's details, DeepSeek's hit counts and the Anthropic names
// some compatible endpoints keep.
type wireUsage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	PromptTokensDetails *struct {
		CachedTokens     int `json:"cached_tokens"`
		CacheWriteTokens int `json:"cache_write_tokens"`
	} `json:"prompt_tokens_details"`
	PromptCacheHitTokens     int `json:"prompt_cache_hit_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

func (u *wireUsage) usage() llm.Usage {
	usage := llm.Usage{Input: firstNonZero(u.PromptTokens, u.InputTokens), Output: firstNonZero(u.CompletionTokens, u.OutputTokens)}
	write := u.CacheCreationInputTokens
	read := firstNonZero(u.PromptCacheHitTokens, u.CacheReadInputTokens)
	if u.PromptTokensDetails != nil {
		read = firstNonZero(u.PromptTokensDetails.CachedTokens, read)
		write = firstNonZero(u.PromptTokensDetails.CacheWriteTokens, write)
	}
	usage.CacheRead, usage.CacheWrite = read, write
	// Anthropic's names count the cache beside the input, not in it
	if usage.Input < read+write {
		usage.Input += read + write
	}
	return usage
}

type streamError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

func (e *streamError) message() string {
	if e.Message != "" {
		return e.Message
	}
	return "provider stream returned an error"
}

func textFromJSON(raw json.RawMessage) string {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		for _, part := range parts {
			text += part.Text
		}
	}
	return text
}

func firstText(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

func normalizeFinish(reason string) string {
	switch reason {
	case "stop", "eos":
		return llm.FinishStop
	case "tool_calls", "function_call":
		return llm.FinishToolCalls
	case "length", "max_tokens":
		return llm.FinishLength
	case "content_filter":
		return llm.FinishFilter
	default:
		return reason
	}
}

// newCallID names a tool call the provider left unnamed: the caller answers
// a call by its id.
func newCallID() string {
	id, err := randomID()
	if err != nil {
		return "call_0"
	}
	return "call_" + id
}

// estimateTokens counts what a request sends at a token per four bytes, for
// a provider that reports no usage.
func estimateTokens(request llm.Request) int {
	n := 0
	for _, message := range request.Messages {
		n += llm.EstimateMessage(message)
	}
	for _, tool := range request.Tools {
		n += llm.EstimateTool(tool)
	}
	return n
}

func bytesToTokens(n int) int {
	if n <= 0 {
		return 0
	}
	return (n + 3) / 4
}
