package ask

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

var (
	needsAgentLine = regexp.MustCompile(`(?s)(^|\n)[ \t]*NEEDS_AGENT:[ \t]*([^\n]+)[ \t]*\n?[ \t]*$`)
	answerPath     = regexp.MustCompile(`(~|/)[A-Za-z0-9._+@%/-]+`)
	answerURL      = regexp.MustCompile("https?://[^\\s\"'`<>()\\]]+")
	answerCode     = regexp.MustCompile("`([^`\\n]{1,240})`")
	answerColor    = regexp.MustCompile(`#[0-9a-fA-F]{6}\b|#[0-9a-fA-F]{3}\b`)
)

// Answer returns the model's final Ask event, including useful open actions.
func (m *Mode) Answer(raw, visible string, plan *protocol.CommandPlan, via string) protocol.AnswerData {
	_, needed, reason := stripNeedsAgent(raw)
	return protocol.AnswerData{Actions: extractActions(visible), Plan: plan, NeedsAgent: needed, Reason: reason, Via: via}
}

func stripNeedsAgent(text string) (visible string, needed bool, reason string) {
	match := needsAgentLine.FindStringSubmatchIndex(text)
	if match == nil {
		return text, false, ""
	}
	reason = strings.TrimSpace(text[match[4]:match[5]])
	visible = strings.TrimRight(text[:match[0]], " \t\n")
	return visible, true, reason
}

func extractActions(text string) []protocol.Action {
	var actions []protocol.Action
	seen := make(map[string]bool)
	add := func(kind, value, label string) {
		key := kind + "\x00" + value
		if value == "" || seen[key] || len(actions) >= 12 {
			return
		}
		seen[key] = true
		actions = append(actions, protocol.Action{Kind: kind, Value: value, Label: label})
	}
	for _, match := range answerURL.FindAllString(text, 8) {
		value := strings.TrimRight(match, ".,;:!?")
		label := strings.TrimPrefix(strings.TrimPrefix(value, "https://"), "http://")
		add(protocol.ActURL, value, truncateRunes(label, 48))
	}
	for _, match := range answerPath.FindAllString(text, 24) {
		value := strings.TrimSpace(strings.TrimRight(match, ".,;:!?"))
		if strings.HasPrefix(value, "~") {
			value = filepath.Join(paths.Home(), strings.TrimPrefix(value, "~/"))
		}
		info, err := os.Stat(value)
		if err != nil || systemRoot(value) || (!info.IsDir() && !info.Mode().IsRegular()) {
			continue
		}
		kind := protocol.ActFile
		label := filepath.Base(value)
		if info.IsDir() {
			kind = protocol.ActDir
			label += "/"
		}
		add(kind, value, label)
	}
	for _, match := range answerCode.FindAllStringSubmatch(text, 16) {
		value := strings.TrimSpace(match[1])
		fields := strings.Fields(value)
		if len(fields) == 0 || !commandLike(fields[0]) {
			continue
		}
		add(protocol.ActCmd, value, truncateRunes(value, 48))
	}
	for _, value := range answerColor.FindAllString(text, 8) {
		add(protocol.ActColor, value, value)
	}
	return actions
}

func truncateRunes(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	keep := max(0, limit-3)
	count := 0
	for index := range text {
		if count == keep {
			return text[:index] + strings.Repeat(".", limit-keep)
		}
		count++
	}
	return text
}

func systemRoot(path string) bool {
	clean := filepath.Clean(path)
	return clean == "/" || filepath.Dir(clean) == "/"
}

func commandLike(first string) bool {
	if strings.Contains(first, "/") {
		info, err := os.Stat(first)
		return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
	}
	if _, err := exec.LookPath(first); err == nil {
		return true
	}
	switch first {
	case "cd", "alias", "set", "export", "source", "type", "command", "printf", "echo":
		return true
	}
	return false
}
