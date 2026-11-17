package main

import "strings"

const handoffPrefix = "[handoff from Ask]\nQuestion: "
const handoffSuffix = "\n[/handoff]\n\n"

func formatHandoffBlock(question, answer, text string) string {
	if strings.TrimSpace(answer) == "" {
		answer = "(none yet)"
	}
	return handoffPrefix + question + "\nAnswer: " + answer + handoffSuffix + text
}

// splitHandoffBlock accepts the identity notes that may precede a first turn,
// then returns only the user-authored remainder after the hand-off card.
func splitHandoffBlock(s string) (question, answer, rest string, ok bool) {
	s, ok = cutIdentityNotes(s)
	if !ok {
		return "", "", "", false
	}
	if !strings.HasPrefix(s, handoffPrefix) {
		return "", "", "", false
	}
	body := strings.TrimPrefix(s, handoffPrefix)
	lineEnd := strings.IndexByte(body, '\n')
	if lineEnd < 0 {
		return "", "", "", false
	}
	question = body[:lineEnd]
	body = body[lineEnd+1:]
	if !strings.HasPrefix(body, "Answer: ") {
		return "", "", "", false
	}
	body = strings.TrimPrefix(body, "Answer: ")
	end := strings.LastIndex(body, handoffSuffix)
	if end < 0 {
		return "", "", "", false
	}
	answer = body[:end]
	rest = body[end+len(handoffSuffix):]
	return question, answer, rest, true
}
