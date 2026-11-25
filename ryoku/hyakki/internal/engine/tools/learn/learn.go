// Package learn holds the tools that let a model grow Rashin's managed
// skills (feature autolearn): manage_skill creates, updates and deletes one,
// and learn keeps a lesson in memory and may write a skill in the same call.
// Every write goes through skills.Managed, which stays inside the managed
// folder and never overrides a built-in skill.
package learn

// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/manage-skill.ts@c7fff67
// Derived from oh-my-pi (MIT), packages/coding-agent/src/tools/learn.ts@c7fff67

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

// Tools are learn and manage_skill over the managed store. events, when
// set, hears learning.changed after every write.
func Tools(managed *skills.Managed, events core.Broadcaster) []tool.Tool {
	base := base{managed: managed, events: events}
	return []tool.Tool{&learnTool{base}, &manageTool{base}}
}

// base is what both tools share.
type base struct {
	managed *skills.Managed
	events  core.Broadcaster
}

// Enabled offers the tools while feature autolearn is on.
func (b base) Enabled(env tool.Env) bool {
	return b.managed != nil && env.Feature(protocol.FeatureAutolearn)
}

// ReadOnly: managed skills and memory are Rashin's own state.
func (base) ReadOnly() bool { return true }

func (b base) changed() {
	if b.events != nil {
		b.events.Broadcast(protocol.EvtLearningChanged, protocol.LearningChanged{Kind: protocol.LearningSkills})
	}
}

// skillID is how the skill tool and skills.get name a managed skill.
func skillID(name string) string { return protocol.ManagedSkillSet + "/" + name }

// errorText is what the model is told of a failed store call.
func errorText(err error) string { return err.Error() }

func schema(properties map[string]any, required ...string) json.RawMessage {
	if required == nil {
		required = []string{}
	}
	raw, _ := json.Marshal(map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false})
	return raw
}

func str(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

const (
	nameHelp        = "kebab-case skill name: lowercase letters, digits and hyphens"
	descriptionHelp = "one line saying when to use the skill, starting with \"Use when\""
	bodyHelp        = "the steps in markdown, with the exact commands; no frontmatter"
)

// manageTool is manage_skill.
type manageTool struct{ base }

const manageDescription = "Create, update or delete a managed skill: a SKILL.md Rashin keeps in its own folder and lists to future sessions. Use it for a procedure worth repeating, such as a setup sequence, a debugging recipe or a project workflow. create fails when the skill exists, update and delete when it does not. A built-in skill of the same name always wins, so pick a new name. Improve an existing managed skill rather than adding a near copy. The frontmatter is written from name and description."

const manageCompact = "Create, update or delete a managed skill (a procedure worth repeating)."

func (manageTool) Name() string    { return tool.ManageSkill }
func (manageTool) Kind() tool.Kind { return protocol.ToolEdit }

func (manageTool) Spec(p tool.Profile) llm.Tool {
	description := manageDescription
	if p.Compact() {
		description = manageCompact
	}
	return llm.Tool{Name: tool.ManageSkill, Description: description, Parameters: schema(map[string]any{
		"action":      map[string]any{"type": "string", "enum": []string{skills.ActionCreate, skills.ActionUpdate, skills.ActionDelete}},
		"name":        str(nameHelp),
		"description": str(descriptionHelp + "; needed for create and update"),
		"body":        str(bodyHelp + "; needed for create and update"),
	}, "action", "name")}
}

type manageArgs struct {
	Action      string `json:"action"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

func (manageTool) Present(raw json.RawMessage) tool.Presentation {
	var args manageArgs
	if json.Unmarshal(raw, &args) != nil {
		return tool.Presentation{Title: i18n.T("Managing a skill"), Input: string(raw)}
	}
	switch args.Action {
	case skills.ActionDelete:
		return tool.Presentation{Title: i18n.Tf("Deleting the skill %s", args.Name)}
	case skills.ActionUpdate:
		return tool.Presentation{Title: i18n.Tf("Updating the skill %s", args.Name), Input: args.Description}
	default:
		return tool.Presentation{Title: i18n.Tf("Writing the skill %s", args.Name), Input: args.Description}
	}
}

func (t manageTool) Run(_ context.Context, call tool.Call, _ tool.Env) tool.Result {
	var args manageArgs
	if err := json.Unmarshal(call.Args, &args); err != nil {
		return tool.Failed("The arguments are not a JSON object with action, name, description and body.")
	}
	switch args.Action {
	case skills.ActionDelete:
		if err := t.managed.Remove(args.Name); err != nil {
			return tool.Failed(errorText(err))
		}
		t.changed()
		return tool.Result{Text: fmt.Sprintf("Deleted the managed skill %q.", strings.ToLower(strings.TrimSpace(args.Name)))}
	case skills.ActionCreate, skills.ActionUpdate:
		if strings.TrimSpace(args.Description) == "" || strings.TrimSpace(args.Body) == "" {
			return tool.Failed(fmt.Sprintf("%s needs both description and body.", args.Action))
		}
		write, verb := t.managed.Create, "Created"
		if args.Action == skills.ActionUpdate {
			write, verb = t.managed.Update, "Updated"
		}
		path, err := write(args.Name, args.Description, args.Body)
		if err != nil {
			return tool.Failed(errorText(err))
		}
		t.changed()
		name, _ := skills.SkillName(args.Name)
		return tool.Result{
			Text:      fmt.Sprintf("%s the managed skill %q (%s, at %s).", verb, name, skillID(name), path),
			Locations: []protocol.Location{{Path: path}},
		}
	}
	return tool.Failed("action must be create, update or delete.")
}

// learnTool is learn.
type learnTool struct{ base }

const learnDescription = "Keep a lesson for later sessions, and optionally write a managed skill in the same call. Use it after an insight likely to pay off again: a fix that was not obvious, a convention of the project, or a workflow that worked. memory is one self-contained fact (what, when, why), kept in Rashin's memory when memory is on for this session. Give skill only for a procedure worth repeating, not for a fact; it creates the managed skill or improves the one of that name. Keep one strong lesson rather than several vague ones."

const learnCompact = "Keep a lesson for later; add skill_name, skill_description and skill_body to save a repeatable procedure as a skill."

func (learnTool) Name() string    { return tool.Learn }
func (learnTool) Kind() tool.Kind { return protocol.ToolThink }

func (learnTool) Spec(p tool.Profile) llm.Tool {
	memory := str("the lesson: one self-contained fact, what, when and why")
	if p.Compact() {
		// compact schemas stay flat
		return llm.Tool{Name: tool.Learn, Description: learnCompact, Parameters: schema(map[string]any{
			"memory":            memory,
			"skill_name":        str(nameHelp),
			"skill_description": str(descriptionHelp),
			"skill_body":        str(bodyHelp),
		}, "memory")}
	}
	return llm.Tool{Name: tool.Learn, Description: learnDescription, Parameters: schema(map[string]any{
		"memory": memory,
		"skill": map[string]any{
			"type":        "object",
			"description": "also create or improve a managed skill",
			"properties": map[string]any{
				"name":        str(nameHelp),
				"description": str(descriptionHelp),
				"body":        str(bodyHelp),
			},
			"required":             []string{"name", "description", "body"},
			"additionalProperties": false,
		},
	}, "memory")}
}

type learnArgs struct {
	Memory string `json:"memory"`
	Skill  *struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Body        string `json:"body"`
	} `json:"skill"`
	SkillName        string `json:"skill_name"`
	SkillDescription string `json:"skill_description"`
	SkillBody        string `json:"skill_body"`
}

// skill is the skill payload in either shape, nested (full) or flat
// (compact); ok is false when there is none.
func (a learnArgs) skill() (name, description, body string, ok bool) {
	if a.Skill != nil {
		name, description, body = a.Skill.Name, a.Skill.Description, a.Skill.Body
	} else {
		name, description, body = a.SkillName, a.SkillDescription, a.SkillBody
	}
	return name, description, body, strings.TrimSpace(name) != ""
}

func (learnTool) Present(raw json.RawMessage) tool.Presentation {
	var args learnArgs
	if json.Unmarshal(raw, &args) != nil {
		return tool.Presentation{Title: i18n.T("Learning"), Input: string(raw)}
	}
	if name, _, _, ok := args.skill(); ok {
		return tool.Presentation{Title: i18n.Tf("Learning, and writing the skill %s", name), Input: args.Memory}
	}
	return tool.Presentation{Title: i18n.T("Learning"), Input: args.Memory}
}

func (t learnTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	var args learnArgs
	if err := json.Unmarshal(call.Args, &args); err != nil {
		return tool.Failed("The arguments are not a JSON object with memory and an optional skill.")
	}
	memory := strings.TrimSpace(args.Memory)
	name, description, body, hasSkill := args.skill()
	if memory == "" && !hasSkill {
		return tool.Failed("memory is required.")
	}
	var said []string
	result := tool.Result{}
	if memory != "" {
		kept, err := remember(ctx, env, memory)
		switch {
		case err != nil && !hasSkill:
			return tool.Failed(err.Error())
		case err != nil:
			result.IsError = true
			said = append(said, err.Error())
		default:
			said = append(said, kept)
		}
	}
	if hasSkill {
		path, created, err := t.managed.Save(name, description, body)
		if err != nil {
			said = append(said, "The managed skill was not written: "+errorText(err))
			result.IsError = true
		} else {
			t.changed()
			verb := "Updated"
			if created {
				verb = "Created"
			}
			safe, _ := skills.SkillName(name)
			said = append(said, fmt.Sprintf("%s the managed skill %q (%s, at %s).", verb, safe, skillID(safe), path))
			result.Locations = []protocol.Location{{Path: path}}
		}
	}
	result.Text = strings.Join(said, "\n")
	if len(result.Text) > env.Limits.Told && env.Limits.Told > 0 {
		result.Text = tool.HeadTail(result.Text, env.Limits.Head, env.Limits.Tail)
	}
	return result
}

// remember keeps the lesson through the daemon's memory, as the session:
// for the team in a team task, the project in a project, else the user.
func remember(ctx context.Context, env tool.Env, text string) (string, error) {
	if !env.Context.Memory || env.Host == nil {
		return "", fmt.Errorf("memory is off for this session, so the lesson was not kept")
	}
	scope := protocol.MemoryYou
	switch {
	case env.Context.Team != nil:
		scope = "team"
	case env.Project != "":
		scope = "project"
	}
	var change protocol.MemoryChange
	err := env.Host.Call(ctx, protocol.MMemoryAdd, protocol.MemoryAddParams{Scope: scope, Text: text, Session: env.Session.ID}, &change)
	if err != nil {
		return "", fmt.Errorf("the lesson was not kept: %v", err)
	}
	if len(change.Items) == 0 {
		return "The lesson was not kept. If Rashin refused it, the conversation says why.", nil
	}
	var said []string
	for _, item := range change.Items {
		switch item.Op {
		case protocol.MemoryOpRemembered:
			said = append(said, "Remembered: "+item.Text)
		case protocol.MemoryOpReplaced:
			said = append(said, "Replaced by the new fact: "+item.Text)
		case protocol.MemoryOpSuggested:
			said = append(said, "Suggested to the user, who decides: "+item.Text)
		default:
			said = append(said, item.Op+": "+item.Text)
		}
	}
	return strings.Join(said, "\n"), nil
}
