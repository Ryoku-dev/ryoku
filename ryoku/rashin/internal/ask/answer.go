package ask

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

var (
	needsAgentLine = regexp.MustCompile(`(?s)(^|\n)[ \t]*NEEDS_AGENT:[ \t]*([^\n]+)[ \t]*\n?[ \t]*$`)
	answerPath     = regexp.MustCompile(`(~|/)[A-Za-z0-9._+@%/-]+`)
	answerURL      = regexp.MustCompile(`https?://[^\s"'` + "`" + `<>()\]]+`)
	answerCode     = regexp.MustCompile("`([^`\\n]{1,240})`")
	answerColor    = regexp.MustCompile(`#([0-9a-fA-F]{6}|[0-9a-fA-F]{3})\b`)
)

func stripNeedsAgent(text string) (visible string, needed bool, reason string) {
	match := needsAgentLine.FindStringSubmatchIndex(text)
	if match == nil {
		return text, false, ""
	}
	reason = strings.TrimSpace(text[match[4]:match[5]])
	visible = strings.TrimRight(text[:match[0]], " \t\n")
	return visible, true, reason
}

type needsFilter struct {
	pending  string
	suppress bool
	emit     func(string)
}

func newNeedsFilter(emit func(string)) *needsFilter { return &needsFilter{emit: emit} }

func (f *needsFilter) Write(delta string) {
	if f.suppress || delta == "" {
		return
	}
	f.pending += delta
	if index := completeNeedsMarker(f.pending); index >= 0 {
		prefix := strings.TrimRight(f.pending[:index], " \t\n")
		if prefix != "" {
			f.emit(prefix)
		}
		f.pending = ""
		f.suppress = true
		return
	}
	keep := partialNeedsSuffix(f.pending)
	if keep == len(f.pending) {
		return
	}
	f.emit(f.pending[:len(f.pending)-keep])
	f.pending = f.pending[len(f.pending)-keep:]
}

func (f *needsFilter) Close() {
	if !f.suppress && f.pending != "" {
		f.emit(f.pending)
	}
	f.pending = ""
}

func completeNeedsMarker(text string) int {
	from := 0
	for {
		i := strings.Index(text[from:], "NEEDS_AGENT:")
		if i < 0 {
			return -1
		}
		i += from
		line := strings.LastIndex(text[:i], "\n") + 1
		if strings.TrimSpace(text[line:i]) == "" {
			return line
		}
		from = i + 1
	}
}

func partialNeedsSuffix(text string) int {
	const marker = "NEEDS_AGENT:"
	line := strings.LastIndex(text, "\n") + 1
	candidate := text[line:]
	trimmed := strings.TrimLeft(candidate, " \t")
	if trimmed != "" && strings.HasPrefix(marker, trimmed) {
		keep := len(candidate)
		if line > 0 {
			keep++
		}
		return keep
	}
	if line == 0 && candidate == "" {
		return 0
	}
	return 0
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
		if err != nil {
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

func commandLike(first string) bool {
	if strings.Contains(first, "/") {
		_, err := os.Stat(first)
		return err == nil
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
