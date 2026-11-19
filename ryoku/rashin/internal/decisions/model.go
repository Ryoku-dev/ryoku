package decisions

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"

	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// errNoCall is a reply that held neither the tool call nor its JSON.
var errNoCall = errors.New("decisions: the model did not answer with the expected call")

// errTruncated is a reply cut at its output limit before the call ended.
var errTruncated = errors.New("decisions: the model's answer was cut off")

// windower is the part of core.Gateway that tells a request's context
// window without sending it.
type windower interface {
	Window(ctx context.Context, req llm.Request) int
}

// request builds one light call that must answer through toolName.
func request(system, input, toolName, schema string, maxTokens int) llm.Request {
	temperature := float64(engine.CompactTemperature)
	return llm.Request{
		Model: protocol.RouteAuto,
		Light: true,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: system},
			{Role: llm.RoleUser, Content: input},
		},
		Tools:       []llm.Tool{{Name: toolName, Description: toolDescription(toolName), Parameters: json.RawMessage(schema)}},
		ToolChoice:  llm.ToolChoiceRequired,
		Temperature: &temperature,
		MaxTokens:   maxTokens,
	}
}

func toolDescription(name string) string {
	if name == extractTool {
		return "Record the project decisions the user's message makes."
	}
	return "Change the project's decision notes."
}

// window is the context window of the model a request routes to, or the
// engine's default when the gateway cannot tell.
func (s *Service) window(ctx context.Context, req llm.Request) int {
	if w, ok := s.deps.Client.(windower); ok {
		if n := w.Window(ctx, req); n > 0 {
			return n
		}
	}
	return 16384
}

// call runs req and returns the arguments of its tool call. A small model
// that writes the call as text instead (`name {...}` or a fenced object)
// is read from the text.
func (s *Service) call(ctx context.Context, req llm.Request) (string, llm.Final, error) {
	if s.deps.Client == nil {
		return "", llm.Final{}, errNoCall
	}
	stream, err := s.deps.Client.Stream(ctx, req)
	if err != nil {
		return "", llm.Final{}, err
	}
	defer stream.Close()
	name := req.Tools[0].Name
	var text strings.Builder
	var args string
	found := false
	for {
		delta, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", llm.Final{}, err
		}
		text.WriteString(delta.Text)
		for _, call := range delta.ToolCalls {
			if !found && (call.Name == name || len(req.Tools) == 1) {
				args, found = call.Arguments, true
			}
		}
	}
	final := stream.Final()
	if found {
		return args, final, nil
	}
	if final.Finish == llm.FinishLength {
		return "", final, errTruncated
	}
	if object := jsonIn(text.String()); object != "" {
		return object, final, nil
	}
	return "", final, errNoCall
}

var (
	thinkBlock = regexp.MustCompile(`(?s)<think>.*?</think>`)
	fenced     = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")
)

// jsonIn finds the JSON object in a reply: inside a code fence, else from
// the first { to the last }.
func jsonIn(reply string) string {
	reply = thinkBlock.ReplaceAllString(reply, "")
	if match := fenced.FindStringSubmatch(reply); match != nil {
		reply = match[1]
	}
	start, end := strings.Index(reply, "{"), strings.LastIndex(reply, "}")
	if start < 0 || end <= start {
		return ""
	}
	return reply[start : end+1]
}

// list reads the array under key from a tool call's arguments; a bare
// array is accepted as well.
func list(args, key string) ([]map[string]any, error) {
	args = strings.TrimSpace(args)
	var items []any
	if strings.HasPrefix(args, "[") {
		if err := json.Unmarshal([]byte(args), &items); err != nil {
			return nil, err
		}
	} else {
		var object map[string]any
		if err := json.Unmarshal([]byte(args), &object); err != nil {
			return nil, err
		}
		raw, ok := object[key]
		if !ok {
			return nil, errNoCall
		}
		items, ok = raw.([]any)
		if !ok && raw != nil {
			return nil, errNoCall
		}
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if object, ok := item.(map[string]any); ok {
			out = append(out, object)
		}
	}
	return out, nil
}

// field reads the first string value of any of keys.
func field(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key].(string); ok {
			return value
		}
	}
	return ""
}
