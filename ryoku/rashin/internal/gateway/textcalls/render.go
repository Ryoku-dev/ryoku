package textcalls

// Derived from oh-my-pi (MIT), packages/ai/src/dialect/rendering.ts@c7fff67
// Derived from oh-my-pi (MIT), packages/ai/src/dialect/owned-stream.ts@c7fff67

import (
	"encoding/json"
	"html"
	"strings"

	"ryoku-rashin/internal/llm"
)

// Prompt gives a text-dialect model its offered tools and its exact call form.
// It is appended to the existing system prompt, not substituted for it.
func Prompt(d string, tools []llm.Tool) string {
	if !Valid(d) || len(tools) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nTools: write calls in the format below. Use only listed names and arguments. Wait for the result after each call.\n<tools>\n")
	for _, t := range tools {
		entry := struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
		}{t.Name, t.Description, t.Parameters}
		if len(entry.Parameters) == 0 || !json.Valid(entry.Parameters) {
			entry.Parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		data, _ := json.Marshal(entry)
		b.Write(data)
		b.WriteByte('\n')
	}
	b.WriteString("</tools>\n")
	switch d {
	case XMLFn:
		b.WriteString("Call: <tool_call><function=NAME><parameter=KEY>VALUE</parameter></function></tool_call>. Use plain text for string values and JSON for other values. Example: <tool_call><function=read><parameter=path>notes.txt</parameter></function></tool_call>\n")
	case Pythonic:
		b.WriteString("Call: <|tool_call_start|>[NAME(key=VALUE)]<|tool_call_end|>. Use quoted strings, True/False/None, lists and dictionaries. Example: <|tool_call_start|>[read(path=\"notes.txt\")]<|tool_call_end|>\n")
	case GLM:
		b.WriteString("Call: <tool_call>NAME<arg_key>KEY</arg_key><arg_value>VALUE</arg_value></tool_call>. Use plain text for string values and JSON for other values. Example: <tool_call>read<arg_key>path</arg_key><arg_value>notes.txt</arg_value></tool_call>\n")
	default:
		b.WriteString("Call: <tool_call>{\"name\":\"NAME\",\"arguments\":{\"key\":\"value\"}}</tool_call>. Example: <tool_call>{\"name\":\"read\",\"arguments\":{\"path\":\"notes.txt\"}}</tool_call>\n")
	}
	return b.String()
}

// RenderCall writes one native call into a text-dialect assistant turn.
func RenderCall(d string, call llm.ToolCall, tools []llm.Tool) string {
	if !Valid(d) {
		return ""
	}
	args, err := parseRelaxed(call.Arguments)
	if err != nil {
		args = object{}
	}
	o, ok := args.(object)
	if !ok {
		o = object{}
	}
	var b strings.Builder
	switch d {
	case XMLFn:
		b.WriteString(tagToolCall + tagFunction + call.Name + ">")
		sh := buildShapes(tools)[call.Name]
		for _, m := range o {
			b.WriteString(tagParamEq)
			b.WriteString(html.EscapeString(m.key))
			b.WriteByte('>')
			if _, isString := m.val.(string); isString && stringOnly(sh.prop(m.key)) {
				b.WriteString(html.EscapeString(m.val.(string)))
			} else {
				b.WriteString(html.EscapeString(marshal(m.val)))
			}
			b.WriteString(tagParamEnd)
		}
		b.WriteString(tagFunctionEnd + tagToolCallEnd)
	case GLM:
		b.WriteString(tagToolCall)
		b.WriteString(call.Name)
		sh := buildShapes(tools)[call.Name]
		for _, m := range o {
			b.WriteString(tagArgKey)
			b.WriteString(html.EscapeString(m.key))
			b.WriteString(tagArgKeyEnd + tagArgValue)
			if _, isString := m.val.(string); isString && stringOnly(sh.prop(m.key)) {
				b.WriteString(html.EscapeString(m.val.(string)))
			} else {
				b.WriteString(html.EscapeString(marshal(m.val)))
			}
			b.WriteString(tagArgValueEnd)
		}
		b.WriteString(tagToolCallEnd)
	case Pythonic:
		b.WriteString(tagPyStart + "[" + call.Name + "(")
		for i, m := range o {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(m.key)
			b.WriteByte('=')
			writePy(&b, m.val)
		}
		b.WriteString(")]" + tagPyEnd)
	default:
		var inner object
		inner.set("name", call.Name)
		inner.set("arguments", o)
		b.WriteString(tagToolCall)
		b.WriteString(marshal(inner))
		b.WriteString(tagToolCallEnd)
	}
	return b.String()
}

func writePy(b *strings.Builder, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("None")
	case bool:
		if x {
			b.WriteString("True")
		} else {
			b.WriteString("False")
		}
	case []any:
		b.WriteByte('[')
		for i, child := range x {
			if i > 0 {
				b.WriteString(", ")
			}
			writePy(b, child)
		}
		b.WriteByte(']')
	case object:
		b.WriteByte('{')
		for i, child := range x {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(marshal(child.key))
			b.WriteString(": ")
			writePy(b, child.val)
		}
		b.WriteByte('}')
	default:
		b.WriteString(marshal(v))
	}
}

// RenderResult presents a tool reply to a text-dialect model. The tool ID
// and name remain in the transcript even when results arrive out of order.
func RenderResult(d string, result llm.Message) string {
	if !Valid(d) {
		return ""
	}
	var b strings.Builder
	b.WriteString(tagResponse)
	b.WriteString("\n")
	if result.Name != "" {
		b.WriteString(result.Name)
		if result.ToolCallID != "" {
			b.WriteString(" (")
			b.WriteString(result.ToolCallID)
			b.WriteByte(')')
		}
		b.WriteString(": ")
	}
	if result.IsError {
		b.WriteString("Error: ")
	}
	b.WriteString(result.Content)
	b.WriteString("\n" + tagResponseEnd)
	return b.String()
}

// Encode converts tool schemas and earlier calls/results into text for a
// text-only tool model. It leaves images on messages for the gateway's usual
// vision handling, and never edits the caller's messages in place.
func Encode(d string, messages []llm.Message, tools []llm.Tool) []llm.Message {
	if !Valid(d) || len(tools) == 0 {
		return messages
	}
	out := make([]llm.Message, 0, len(messages)+1)
	instruct := Prompt(d, tools)
	placed := false
	for _, m := range messages {
		copy := m
		if m.Role == llm.RoleSystem && !placed {
			copy.Content += instruct
			placed = true
		}
		if m.Role == llm.RoleAssistant && len(m.ToolCalls) > 0 {
			var b strings.Builder
			b.WriteString(m.Content)
			for _, c := range m.ToolCalls {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(RenderCall(d, c, tools))
			}
			copy.Content = b.String()
			copy.ToolCalls = nil
		}
		if m.Role == llm.RoleTool {
			copy.Role = llm.RoleUser
			copy.Content = RenderResult(d, m)
			copy.ToolCallID = ""
			copy.Name = ""
			copy.IsError = false
		}
		out = append(out, copy)
	}
	if !placed {
		out = append([]llm.Message{{Role: llm.RoleSystem, Content: instruct}}, out...)
	}
	return out
}

// Stops prevent a text-dialect model from writing a tool's result itself.
func Stops(d string) []string {
	if !Valid(d) {
		return nil
	}
	return []string{tagResponse, tagPyResponse}
}
