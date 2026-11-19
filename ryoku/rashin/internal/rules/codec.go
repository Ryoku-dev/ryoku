package rules

import (
	"strconv"
	"strings"

	"ryoku-rashin/internal/protocol"
)

func decode(name, project, path, text string) protocol.Rule {
	r := protocol.Rule{Name: name, Project: project, Path: path, Enabled: true}
	front, body, ok := splitFrontmatter(text)
	if !ok {
		r.Error = "front matter has no closing ---"
		return r
	}
	r.Body = strings.TrimSpace(body)
	meta := parseFrontmatter(front)
	r.Description = meta["description"].scalar
	r.Condition = meta["condition"].strings()
	if len(r.Condition) == 0 {
		r.Condition = meta["ttsr_trigger"].strings()
	}
	r.Question = meta["question"].scalar
	r.Globs = meta["globs"].strings()
	r.Scope = meta["scope"].scalar
	if meta["scope"].isList {
		r.Scope = strings.Join(meta["scope"].list, ",")
	}
	r.AlwaysApply = truthy(meta["alwaysApply"].scalar)
	r.Forged = truthy(meta["forged"].scalar)
	r.Interrupt = meta["interruptMode"].scalar
	r.Repeat = meta["repeatMode"].scalar
	if enabled, exists := meta["enabled"]; exists {
		r.Enabled = truthy(enabled.scalar)
	}
	if r.Description == "" || r.Body == "" {
		r.Error = "rule needs description and body"
		return r
	}
	if c := compile(r, false); c.rule.Error != "" {
		r.Error = c.rule.Error
	}
	return r
}

func encode(r protocol.Rule) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: " + quote(r.Description) + "\n")
	if len(r.Condition) > 0 {
		b.WriteString("condition: [")
		for i, v := range r.Condition {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(quote(v))
		}
		b.WriteString("]\n")
	}
	if r.Question != "" {
		b.WriteString("question: " + quote(r.Question) + "\n")
	}
	if r.Scope != "" {
		b.WriteString("scope: " + quote(r.Scope) + "\n")
	}
	if len(r.Globs) > 0 {
		b.WriteString("globs: [")
		for i, v := range r.Globs {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(quote(v))
		}
		b.WriteString("]\n")
	}
	if r.AlwaysApply {
		b.WriteString("alwaysApply: true\n")
	}
	if r.Interrupt != "" {
		b.WriteString("interruptMode: " + quote(r.Interrupt) + "\n")
	}
	if r.Repeat != "" {
		b.WriteString("repeatMode: " + quote(r.Repeat) + "\n")
	}
	b.WriteString("enabled: " + strconv.FormatBool(r.Enabled) + "\n")
	if r.Forged {
		b.WriteString("forged: true\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(r.Body))
	b.WriteByte('\n')
	return b.String()
}
