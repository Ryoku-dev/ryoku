package gateway

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// chatCall is one request to the local API, as the router takes it.
type chatCall struct {
	request llm.Request
	stream  bool
	// usage says the caller wants a last stream frame that counts tokens
	usage bool
}

// passedOn are the OpenAI parameters handed upstream as they came. The rest
// of what a client sends is dropped: free providers reject fields they don't know.
var passedOn = [...]string{
	"top_p", "stop", "seed", "presence_penalty", "frequency_penalty",
	"response_format", "tool_choice", "parallel_tool_calls", "reasoning_effort",
}

type apiChat struct {
	Model         string       `json:"model"`
	Messages      []apiMessage `json:"messages"`
	Tools         []wireTool   `json:"tools"`
	Stream        bool         `json:"stream"`
	StreamOptions struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
	Temperature         *float64 `json:"temperature"`
	MaxTokens           int      `json:"max_tokens"`
	MaxCompletionTokens int      `json:"max_completion_tokens"`
	N                   int      `json:"n"`
}

type apiMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	ToolCalls  []wireToolCall  `json:"tool_calls"`
	ToolCallID string          `json:"tool_call_id"`
	Name       string          `json:"name"`
}

type apiPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	ImageURL json.RawMessage `json:"image_url"`
}

func decodeChat(body []byte) (chatCall, error) {
	var fields map[string]json.RawMessage
	var in apiChat
	if json.Unmarshal(body, &fields) != nil || json.Unmarshal(body, &in) != nil {
		return chatCall{}, errors.New(i18n.T("The request is not a chat completion in JSON."))
	}
	if len(in.Messages) == 0 {
		return chatCall{}, errors.New(i18n.T("The request has no messages."))
	}
	if in.N > 1 {
		return chatCall{}, errors.New(i18n.T("Rashin answers one choice at a time: leave n out or set it to 1."))
	}
	model := apiRoute(in.Model)
	call := chatCall{stream: in.Stream, usage: in.StreamOptions.IncludeUsage}
	call.request = llm.Request{
		// a caller that names a model gets that model or an error, never another one in its place
		Model: model, Strict: model != protocol.RouteAuto && model != protocol.RouteFast && model != protocol.RouteSmart,
		Temperature: in.Temperature, MaxTokens: in.MaxTokens,
	}
	if call.request.MaxTokens == 0 {
		call.request.MaxTokens = in.MaxCompletionTokens
	}
	for _, message := range in.Messages {
		converted, err := apiToMessage(message)
		if err != nil {
			return chatCall{}, err
		}
		call.request.Messages = append(call.request.Messages, converted)
	}
	for _, tool := range in.Tools {
		if tool.Type != "" && tool.Type != "function" {
			return chatCall{}, errors.New(i18n.Tf("Rashin passes function tools on, not %s tools.", tool.Type))
		}
		if strings.TrimSpace(tool.Function.Name) == "" {
			return chatCall{}, errors.New(i18n.T("A tool has no name."))
		}
		call.request.Tools = append(call.request.Tools, llm.Tool{
			Name: tool.Function.Name, Description: tool.Function.Description, Parameters: tool.Function.Parameters,
		})
	}
	for _, name := range passedOn {
		if value, set := fields[name]; set && string(value) != "null" {
			if call.request.Extra == nil {
				call.request.Extra = make(map[string]json.RawMessage)
			}
			call.request.Extra[name] = value
		}
	}
	return call, nil
}

// apiRoute reads the model a caller named. No model is the Auto route, and so
// is any other "auto:" name: Prowl had more of them and old configs still ask.
func apiRoute(model string) string {
	model = strings.TrimSpace(model)
	switch {
	case model == "", model == protocol.RouteAuto:
		return protocol.RouteAuto
	case model == protocol.RouteFast, model == protocol.RouteSmart:
		return model
	case strings.HasPrefix(model, protocol.RouteAuto+":"):
		return protocol.RouteAuto
	}
	return model
}

func apiToMessage(message apiMessage) (llm.Message, error) {
	text, images, err := apiContent(message.Content)
	if err != nil {
		return llm.Message{}, err
	}
	switch message.Role {
	case "system", "developer":
		return llm.Message{Role: llm.RoleSystem, Content: text}, nil
	case "user":
		return llm.Message{Role: llm.RoleUser, Content: text, Images: images}, nil
	case "assistant":
		converted := llm.Message{Role: llm.RoleAssistant, Content: text}
		for _, call := range message.ToolCalls {
			converted.ToolCalls = append(converted.ToolCalls, llm.ToolCall{ID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
		}
		return converted, nil
	case "tool", "function":
		return llm.Message{Role: llm.RoleTool, Content: text, ToolCallID: message.ToolCallID, Name: message.Name}, nil
	}
	return llm.Message{}, errors.New(i18n.Tf("Rashin does not know the message role %s.", message.Role))
}

// apiContent reads a message's content: a string, or parts of text and images.
func apiContent(raw json.RawMessage) (string, []llm.Image, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil, nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil, nil
	}
	var parts []apiPart
	if json.Unmarshal(raw, &parts) != nil {
		return "", nil, errors.New(i18n.T("A message's content must be text or a list of parts."))
	}
	var lines []string
	var images []llm.Image
	for _, part := range parts {
		switch part.Type {
		case "text", "input_text":
			lines = append(lines, part.Text)
		case "image_url":
			image, err := apiImage(part.ImageURL)
			if err != nil {
				return "", nil, err
			}
			images = append(images, image)
		default:
			return "", nil, errors.New(i18n.Tf("Rashin does not pass %s parts on.", part.Type))
		}
	}
	return strings.Join(lines, "\n"), images, nil
}

// apiImage takes an image sent inline. One named by a web address is refused:
// fetching it here would let a caller point the daemon at any address it can reach.
func apiImage(raw json.RawMessage) (llm.Image, error) {
	var address string
	if json.Unmarshal(raw, &address) != nil {
		var object struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(raw, &object) != nil {
			return llm.Image{}, errors.New(i18n.T("An image part has no image_url."))
		}
		address = object.URL
	}
	header, encoded, found := strings.Cut(address, ",")
	if !found || !strings.HasPrefix(header, "data:") || !strings.HasSuffix(header, ";base64") {
		return llm.Image{}, errors.New(i18n.T("Send images inline as base64 data URLs."))
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return llm.Image{}, errors.New(i18n.T("An inline image is not valid base64."))
	}
	mime := strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
	return llm.Image{Mime: mime, Data: data}, nil
}
