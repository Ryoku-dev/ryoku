package memory

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/protocol"
)

// conversation is what the learner reads of a session past its watermark:
// the finished turns, the seq and number of the last of them, and the model
// that answered.
type conversation struct {
	turns []backend.Turn
	last  int64
	turn  int
	model string
}

// readConversation collects the turns that ended after seq after: the user's
// words, the final reply and the user's answers to permission requests. Tool
// calls, their output, thoughts, plans and terminals are never read. A turn
// still running is left for later.
func readConversation(events []protocol.SessionEvent, after int64) conversation {
	var c conversation
	var open *backend.Turn
	var messages []string
	texts := map[string]*strings.Builder{}
	asked := map[string]protocol.PermissionData{}
	begin := func(ev protocol.SessionEvent) {
		if open == nil {
			open = &backend.Turn{At: ev.TS}
			messages = messages[:0]
			clear(texts)
		}
	}
	for _, ev := range events {
		switch ev.Kind {
		case protocol.EvTurnStart:
			begin(ev)
		case protocol.EvUser:
			var d protocol.UserData
			if json.Unmarshal(ev.Data, &d) != nil {
				continue
			}
			begin(ev)
			// a goal turn is Rashin's own prompt, not the user's words
			if d.Goal {
				continue
			}
			if text := userRow(d.Text); text != "" {
				open.User = strings.TrimSpace(open.User + "\n" + text)
			}
		case protocol.EvText:
			var d protocol.TextData
			if open == nil || json.Unmarshal(ev.Data, &d) != nil {
				continue
			}
			b, ok := texts[d.Message]
			if !ok {
				b = &strings.Builder{}
				texts[d.Message] = b
				messages = append(messages, d.Message)
			}
			b.WriteString(d.Delta)
		case protocol.EvPermission:
			var d protocol.PermissionData
			if json.Unmarshal(ev.Data, &d) == nil {
				asked[d.Request] = d
			}
		case protocol.EvPermissionResolved:
			var d protocol.PermissionResolvedData
			if open == nil || json.Unmarshal(ev.Data, &d) != nil || d.By != "user" || d.Outcome != "selected" {
				continue
			}
			if answer := answerRow(asked[d.Request], d.Option); answer != "" {
				open.Answers = append(open.Answers, answer)
			}
		case protocol.EvModel:
			var d protocol.ChoiceData
			if json.Unmarshal(ev.Data, &d) == nil && d.Current != "" {
				c.model = d.Current
			}
		case protocol.EvAnswer:
			var d protocol.AnswerData
			if json.Unmarshal(ev.Data, &d) == nil && d.Via != "" {
				c.model = d.Via
			}
		case protocol.EvTurnEnd:
			var d protocol.TurnEndData
			_ = json.Unmarshal(ev.Data, &d)
			if open != nil && ev.Seq > after {
				if len(messages) > 0 {
					open.Reply = replyRow(texts[messages[len(messages)-1]].String())
				}
				open.Seq = ev.Seq
				if open.User != "" || len(open.Answers) > 0 {
					c.turns = append(c.turns, *open)
				}
			}
			if ev.Seq > after {
				c.last, c.turn = ev.Seq, d.Turn
			}
			open = nil
		}
	}
	return c
}

// answerRow is the label the user picked on a request that asked them a
// question of the agent's own: one whose options are not the usual allow and
// reject. The request's title and an allow or reject pick are tool text, not
// the user's words, and are never read.
func answerRow(asked protocol.PermissionData, option string) string {
	for _, choice := range asked.Options {
		if choice.ID != option {
			continue
		}
		switch choice.Kind {
		case protocol.OptAllowOnce, protocol.OptAllowAlways, protocol.OptRejectOnce, protocol.OptRejectAlways:
			return ""
		}
		return strings.TrimSpace(choice.Label)
	}
	return ""
}

// Portions adapted from openai/codex codex-rs/memories/write/src/rollout_input.rs
// and codex-rs/rollout/src/policy.rs @ b75ec48a2c90 (Apache-2.0, Copyright 2025
// OpenAI): dropping what the harness injected. Portions adapted from
// openclaw/openclaw src/hooks/bundled/session-memory/transcript.ts @
// 3978ab104ee6 (MIT, Copyright (c) 2026 OpenClaw Foundation): tool-call blocks,
// role tags and slash commands. Modified by Ryoku: Rashin's own markers; the
// role-block back-reference is three alternations.
var (
	injectedBlocks = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<\s*memory-context\s*>.*?</\s*memory-context\s*>`),
		regexp.MustCompile(`(?is)<user_card\b[^>]*>.*?</user_card>`),
		regexp.MustCompile(`(?is)<memory_metadata>.*?</memory_metadata>`),
		regexp.MustCompile(`(?is)<(?:\|DSML\||\x{FF5C}DSML\x{FF5C})?(?:tool_calls?|function_calls?|tool_use_error)\b[^>]*>.*?(?:</(?:\|DSML\||\x{FF5C}DSML\x{FF5C})?(?:tool_calls?|function_calls?|tool_use_error)>|$)`),
		regexp.MustCompile(`(?is)<system\b[^>]*>.*?</system>`),
		regexp.MustCompile(`(?is)<assistant\b[^>]*>.*?</assistant>`),
		regexp.MustCompile(`(?is)<user\b[^>]*>.*?</user>`),
		regexp.MustCompile(`(?i)</?(?:system|assistant|user)\b[^>]*>`),
	}
	// briefLead opens the context an agent session is briefed with
	briefLead = "Context from Rashin. The user did not write this part"
)

// Portions adapted from mem0ai/mem0 integrations/openclaw/filtering.ts @
// f1791daa86d4 (Apache-2.0, Copyright 2023 Taranjeet Singh and the
// mem0ai/mem0 contributors). Modified by Ryoku: the chat-platform patterns
// are gone; emoji are a range table; approval boilerplate is added.
var (
	noiseRows = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(?:HEARTBEAT_OK|NO_REPLY)$`),
		regexp.MustCompile(`^Current time:.*\d{4}`),
		regexp.MustCompile(`(?i)^(?:ok|yes|no|sure|thanks|done|good|nice|cool|got it|continue|alright|okay|yep|nope|uh-huh|mm-hmm|hmm)$`),
		regexp.MustCompile(`^\s*\{[\s\S]*\}\s*$`),
		regexp.MustCompile(`^\s*\[[\s\S]*\]\s*$`),
		regexp.MustCompile(`(?i)^(?:Error|Warning|Info|Debug):`),
		regexp.MustCompile(`(?i)^(?:Loading|Loaded|Fetching|Fetched|Processing|Processed)\b`),
		regexp.MustCompile(`^\[[\d:T\-.Z]+\]`),
		regexp.MustCompile(`(?i)^(?:SUCCESS|FAILURE|PENDING|COMPLETED|FAILED)$`),
		regexp.MustCompile(`(?i)^(?:Calling|Called|Invoking|Invoked|Executing|Executed)\s+(?:function|tool|method)`),
		regexp.MustCompile(`(?i)^Tool (?:call|result|output):`),
		regexp.MustCompile(`(?i)^(?:allow|allow once|always allow|deny|reject|approve|approved|yes,? allow)$`),
		regexp.MustCompile(`^[\x{1F000}-\x{1FAFF}\x{2600}-\x{27BF}\x{2B00}-\x{2BFF}\x{FE0F}\x{200D}\s]{1,5}$`),
	}
	noiseContent = []*regexp.Regexp{
		regexp.MustCompile("(?s)```json\\s*\\{.*?\\}\\s*```"),
		regexp.MustCompile("(?is)```(?:text|output|result|log)\\s*.*?```"),
		regexp.MustCompile(`\[tool_call_id:[^\]]+\]`),
		regexp.MustCompile(`(?i)\(id:\s*[a-f0-9-]+\)`),
	}
	genericReply = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(?:I see you'?ve shared|Thanks for sharing|Got it[.!]?\s*(?:I see|Let me|How can)|I understand[.!]?\s*(?:How can|Is there|Would you))`),
		regexp.MustCompile(`(?i)^(?:How can I help|Is there anything|Would you like me to|Let me know (?:if|how|what))`),
		regexp.MustCompile(`(?i)^(?:I(?:'?ll| will) (?:help|assist|look into|review|take a look))`),
		regexp.MustCompile(`(?i)^(?:Sure[.!]?\s*(?:How|What|Is)|Understood[.!]?\s*(?:How|What|Is))`),
		regexp.MustCompile(`(?i)^(?:That(?:'?s| is) (?:noted|understood|clear))`),
	}
)

// clean takes injected blocks and noise fragments out of a row.
func clean(text string) string {
	for _, block := range injectedBlocks {
		text = block.ReplaceAllString(text, "")
	}
	for _, fragment := range noiseContent {
		text = fragment.ReplaceAllString(text, "")
	}
	return strings.TrimSpace(text)
}

// noise reports a row that says nothing to learn from.
func noise(text string) bool {
	for _, pattern := range noiseRows {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// userRow is what the learner reads of the user's prompt, "" for none.
func userRow(text string) string {
	if strings.HasPrefix(strings.TrimSpace(text), briefLead) {
		return ""
	}
	text = clean(text)
	if strings.HasPrefix(text, "/") || noise(text) {
		return ""
	}
	return text
}

// replyRow is what the learner reads of a final reply, "" for a generic one.
func replyRow(text string) string {
	text = clean(stripThink(text))
	for _, pattern := range genericReply {
		if pattern.MatchString(text) {
			return ""
		}
	}
	if noise(text) {
		return ""
	}
	return text
}

// rowMax caps one row, and rowReserve leaves room for the cut marker.
const (
	rowMax     = 10_000
	rowReserve = 96
	omitted    = "[... earlier turns omitted ...]\n"
)

// tier orders rows for the budget: the user's words and answers first, the
// final replies after.
type tier int

const (
	tierHuman tier = iota
	tierFinal
)

type row struct {
	tier tier
	text string
}

// Portions adapted from openai/codex codex-rs/memories/write/src/rollout_input.rs
// @ b75ec48a2c90 (Apache-2.0, Copyright 2025 OpenAI). Modified by Ryoku: only
// the human and final tiers, with the user's answers in the human tier.

// renderTurns writes the turns for the learner in source order, spending
// limit bytes on the user's rows first, newest first, then on the replies,
// and marking each run of rows left out.
func renderTurns(turns []backend.Turn, limit int) string {
	var rows []row
	for _, turn := range turns {
		if turn.User != "" {
			rows = append(rows, row{tierHuman, "[human user]\n" + cutBytes(turn.User, rowMax-rowReserve) + "\n"})
		}
		for _, answer := range turn.Answers {
			rows = append(rows, row{tierHuman, "[human user]\n" + cutBytes(answer, rowMax-rowReserve) + "\n"})
		}
		if turn.Reply != "" {
			rows = append(rows, row{tierFinal, "[assistant final]\n" + cutBytes(turn.Reply, rowMax-rowReserve) + "\n"})
		}
	}
	remaining := limit - len(omitted)
	selected := make([]string, len(rows))
	for _, want := range []tier{tierHuman, tierFinal} {
		for index := len(rows) - 1; index >= 0; index-- {
			if rows[index].tier != want || remaining <= len(omitted)+rowReserve {
				continue
			}
			text := rows[index].text
			if budget := remaining - len(omitted); len(text) > budget {
				text = cutBytes(text, budget-rowReserve) + "\n"
			}
			remaining -= len(text) + len(omitted)
			selected[index] = text
		}
	}
	var b strings.Builder
	gap := false
	for _, text := range selected {
		switch {
		case text != "":
			b.WriteString(text)
			gap = false
		case !gap:
			b.WriteString(omitted)
			gap = true
		}
	}
	return b.String()
}

// cutBytes shortens text to at most max bytes at a rune boundary, marking
// the cut.
func cutBytes(text string, max int) string {
	if len(text) <= max {
		return text
	}
	const mark = " [...]"
	end := max - len(mark)
	for end > 0 && !utf8.RuneStart(text[end]) {
		end--
	}
	if end <= 0 {
		return ""
	}
	return text[:end] + mark
}

// userWords is everything the user said in the turns, for checking that a
// fact's evidence is the user's own.
func userWords(turns []backend.Turn) string {
	var b strings.Builder
	for _, turn := range turns {
		b.WriteString(turn.User + "\n")
		for _, answer := range turn.Answers {
			b.WriteString(answer + "\n")
		}
	}
	return b.String()
}

// scrubTurns scrubs every row of the turns as one hand-off.
func scrubTurns(turns []backend.Turn) {
	var texts []*string
	for index := range turns {
		texts = append(texts, &turns[index].User, &turns[index].Reply)
		for answer := range turns[index].Answers {
			texts = append(texts, &turns[index].Answers[answer])
		}
	}
	scrubTexts(texts)
}

// allTrivial reports turns in which the user only greeted, acknowledged,
// answered permission requests or ran commands.
func allTrivial(turns []backend.Turn) bool {
	for _, turn := range turns {
		if !trivial(turn.User) {
			return false
		}
	}
	return true
}
