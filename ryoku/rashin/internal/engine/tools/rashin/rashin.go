// Package rashin holds the tools that reach Rashin itself rather than files
// or processes: memory, skills, teams, the agent's own screen, a team task's
// tools and a dream manager's. Each calls a daemon method through
// tool.Host, which applies the session's identity (memory caller, team gate,
// run token check), so nothing here holds a socket or a secret.
package rashin

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
)

// Tools are every Rashin tool, for tool.NewRegistry. Which ones a session
// gets is decided per call from Env.Context (memory, screen, team task,
// lead, dream) and the mode's name table.
func Tools() []tool.Tool {
	var tools []tool.Tool
	tools = append(tools, memoryTools()...)
	tools = append(tools, skillTools()...)
	tools = append(tools, teamsTools()...)
	tools = append(tools, screenTools()...)
	tools = append(tools, teamTaskTools()...)
	tools = append(tools, dreamTools()...)
	return tools
}

// def is one Rashin tool. Every Rashin tool is ReadOnly: it changes only
// Rashin's own state, or the daemon service it calls owns the consent.
type def struct {
	name string
	kind tool.Kind
	// title is the tool line's title; show names the argument shown under
	// it.
	title func() string
	show  string
	// describe is the full description for p; brief, when set, is the
	// compact profile's one line.
	describe func(p tool.Profile) string
	brief    string
	// schema is the arguments for p.
	schema func(p tool.Profile) object
	// aliases are other names a model may give an argument, as omp accepts.
	aliases map[string]string
	enabled func(env tool.Env) bool
	run     func(ctx context.Context, args arguments, env tool.Env) tool.Result
}

func (d *def) Name() string    { return d.name }
func (d *def) Kind() tool.Kind { return d.kind }
func (d *def) ReadOnly() bool  { return true }

func (d *def) Enabled(env tool.Env) bool {
	return d.enabled == nil || d.enabled(env)
}

func (d *def) Spec(p tool.Profile) llm.Tool {
	description := d.describe(p)
	if p.Compact() && d.brief != "" {
		description = d.brief
	}
	return llm.Tool{Name: d.name, Description: description, Parameters: d.schema(p).encode()}
}

func (d *def) Present(raw json.RawMessage) tool.Presentation {
	presentation := tool.Presentation{Title: d.title()}
	var fields map[string]any
	if json.Unmarshal(raw, &fields) != nil {
		presentation.Input, _ = tool.ClipRunes(string(raw), 200)
		return presentation
	}
	if d.show != "" {
		if value, ok := fields[d.show].(string); ok {
			presentation.Input, _ = tool.ClipRunes(value, 200)
		}
	}
	return presentation
}

func (d *def) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	args, err := checkArguments(call.Args, d.schema(env.Profile), d.aliases)
	if err != nil {
		return tool.Failed(err.Error())
	}
	if env.Host == nil {
		return tool.Failed(i18n.T("Rashin's daemon is not reachable from this session."))
	}
	return d.run(ctx, args, env)
}

// object is a JSON schema for a tool's arguments that remembers the order
// its properties were declared in, so a refusal names them in that order.
type object struct {
	order      []string
	required   []string
	properties map[string]any
}

// field is one property of an object.
type field struct {
	name   string
	schema map[string]any
}

func newObject(required []string, fields ...field) object {
	o := object{required: append([]string{}, required...), properties: make(map[string]any, len(fields))}
	for _, f := range fields {
		o.order = append(o.order, f.name)
		o.properties[f.name] = f.schema
	}
	return o
}

// without is o less the named properties.
func (o object) without(names ...string) object {
	out := object{properties: make(map[string]any, len(o.properties))}
	for _, name := range o.order {
		if !slices.Contains(names, name) {
			out.order = append(out.order, name)
			out.properties[name] = o.properties[name]
		}
	}
	for _, name := range o.required {
		if !slices.Contains(names, name) {
			out.required = append(out.required, name)
		}
	}
	if out.required == nil {
		out.required = []string{}
	}
	return out
}

// fromSchema reads a schema built as a map (internal/brief's definitions);
// its properties are named in sorted order.
func fromSchema(schema map[string]any) object {
	o := object{properties: map[string]any{}, required: []string{}}
	if properties, ok := schema["properties"].(map[string]any); ok {
		o.properties = properties
		o.order = slices.Sorted(maps.Keys(properties))
	}
	if required, ok := schema["required"].([]string); ok {
		o.required = append([]string{}, required...)
	}
	return o
}

func (o object) encode() json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"type": "object", "properties": o.properties, "required": o.required, "additionalProperties": false})
	return raw
}

func str(name, description string) field {
	return field{name, map[string]any{"type": "string", "description": description}}
}

func boolean(name, description string) field {
	return field{name, map[string]any{"type": "boolean", "description": description}}
}

func list(name, description string) field {
	return field{name, map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": description}}
}

func enum(name, description string, values ...string) field {
	return field{name, map[string]any{"type": "string", "enum": values, "description": description}}
}

// arguments are a call's checked arguments.
type arguments map[string]json.RawMessage

// decode reads the arguments into the struct target points to.
func (a arguments) decode(target any) error {
	if err := json.Unmarshal(a.raw(), target); err != nil {
		return errors.New(i18n.T("The arguments are not valid JSON for this tool."))
	}
	return nil
}

// raw is the arguments as one JSON object again.
func (a arguments) raw() json.RawMessage {
	raw, _ := json.Marshal(map[string]json.RawMessage(a))
	return raw
}

// checkArguments reads a call's arguments as a JSON object, renames aliases
// and checks them against schema. An argument the tool does not take is
// refused with the ones it does, so a model retries instead of losing what
// it put there; a required one that is missing, empty or null is named.
func checkArguments(raw json.RawMessage, schema object, aliases map[string]string) (arguments, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		trimmed = "{}"
	}
	var present map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &present); err != nil || present == nil {
		return nil, errors.New(i18n.T("The arguments are not valid JSON for this tool."))
	}
	for alias, name := range aliases {
		value, ok := present[alias]
		if !ok {
			continue
		}
		if _, taken := present[name]; !taken {
			present[name] = value
		}
		delete(present, alias)
	}
	for _, name := range slices.Sorted(maps.Keys(present)) {
		if _, ok := schema.properties[name]; ok {
			continue
		}
		if len(schema.order) == 0 {
			return nil, errors.New(i18n.Tf("%s is not an argument of this tool; it takes none. Call it again without arguments.", name))
		}
		return nil, errors.New(i18n.Tf("%s is not an argument of this tool; it takes %s. Call it again with your words in one of those.", name, strings.Join(schema.order, ", ")))
	}
	for _, name := range schema.required {
		value, ok := present[name]
		if !ok || string(value) == `""` || string(value) == "null" || string(value) == "[]" {
			return nil, errors.New(i18n.Tf("Missing %s.", name))
		}
	}
	return arguments(present), nil
}

// answer is a Result from text, kept under the profile's cap with the one
// head and tail marker every tool uses.
func answer(env tool.Env, text string) tool.Result {
	if env.Limits.Told > 0 && len(text) > env.Limits.Told {
		text = tool.HeadTail(text, env.Limits.Head, env.Limits.Tail)
	}
	return tool.Result{Text: text}
}

func failed(err error) tool.Result { return tool.Failed(err.Error()) }

// askMode reports a mode of Ask.
func askMode(mode tool.Mode) bool { return mode == tool.ModeAskRead || mode == tool.ModeAskAct }

// teamMode reports a mode of a team task's run.
func teamMode(mode tool.Mode) bool { return mode == tool.ModeTeam || mode == tool.ModeDreamManager }

// static is a describe that does not depend on the profile.
func static(description string) func(tool.Profile) string {
	return func(tool.Profile) string { return description }
}

// fixed is a schema that does not depend on the profile.
func fixed(o object) func(tool.Profile) object {
	return func(tool.Profile) object { return o }
}
