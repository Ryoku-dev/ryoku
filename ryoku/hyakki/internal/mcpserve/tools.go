package mcpserve

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// tool is one tool the bridge may offer. Its description is written for an
// agent: what it does, its limits and what it never does.
type tool struct {
	id     string
	about  func() string
	schema map[string]any
	run    func(ctx context.Context, d Daemon, args json.RawMessage) (string, error)
}

func (t tool) description() string { return t.about() }

func object(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func text(about string, limit int) map[string]any {
	return map[string]any{"type": "string", "description": about, "maxLength": limit}
}

// decode reads a tool's arguments strictly: no field it does not know.
func decode(args json.RawMessage, into any) error {
	dec := json.NewDecoder(strings.NewReader(string(args)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil || dec.More() {
		return badArgs(i18n.T("The arguments do not match the tool's input schema."))
	}
	return nil
}

// tools is the frozen table, in the order they are listed. No task store
// and no habits: Hyakki keeps neither.
var tools = []tool{
	{
		id: protocol.OniMCPRemindersList,
		about: func() string {
			return i18n.T("List the user's pending reminders and timers in Hyakki, soonest first, with each one's id, kind, due time and words. Read only. Needs the user's reminders switch.")
		},
		schema: object(map[string]any{}),
		run:    remindersList,
	},
	{
		id: protocol.OniMCPRemindersAdd,
		about: func() string {
			return i18n.T("Set one reminder (or a timer) the user will hear or see from Oni when it is due. At most a week ahead (a timer less), words up to 120 characters, no private numbers or secrets. Refused while the user's reminders switch is off or too many are pending. It never cancels or changes another reminder.")
		},
		schema: object(map[string]any{
			"kind": map[string]any{"type": "string", "enum": []string{protocol.OniReminderKindReminder, protocol.OniReminderKindTimer}, "description": "reminder (the default) or timer"},
			"due":  map[string]any{"type": "string", "format": "date-time", "description": "when it is due, RFC 3339 with a time zone"},
			"text": text("what to remind the user of", TextMax),
		}, "due"),
		run: remindersAdd,
	},
	{
		id: protocol.OniMCPNotesSearch,
		about: func() string {
			return i18n.T("Search what Hyakki remembers about the user, best match first, at most 10 notes. Read only; sensitive notes come back hidden. It never reads conversations, projects or teams.")
		},
		schema: object(map[string]any{"query": text("what to look for", QueryMax), "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": SearchMax}}, "query"),
		run:    notesSearch,
	},
	{
		id: protocol.OniMCPNotesAdd,
		about: func() string {
			return i18n.T("Suggest one note about the user, up to 500 characters. It is never kept as a fact: it waits in Hyakki's Inbox, marked as coming from you, until the user keeps or dismisses it. Secrets, private numbers and instructions are refused.")
		},
		schema: object(map[string]any{"text": text("the note, one plain statement", NoteMax)}, "text"),
		run:    notesAdd,
	},
	{
		id: protocol.OniMCPFocusStatus,
		about: func() string {
			return i18n.T("Say whether the user's focus guard is on and how long it has left. Read only: it never starts, stops or changes the guard.")
		},
		schema: object(map[string]any{}),
		run:    focusStatus,
	},
	{
		id: protocol.OniMCPNotify,
		about: func() string {
			return i18n.T("Show one short line, up to 120 characters, in Oni's bubble on the user's screen, headed with your name. No links, private numbers or secrets. At most 3 a minute; refused while Oni is off, hidden or not on screen. Nothing is spoken and nothing is kept.")
		},
		schema: object(map[string]any{"text": text("the line to show", TextMax)}, "text"),
		run:    notify,
	},
	{
		id: protocol.OniMCPScreenAsk,
		about: func() string {
			return i18n.T("Ask a question about the window Oni is perched on. Oni takes a picture and asks the user on screen whether to send it, and where; nothing is sent unless the user says yes. You get the answer's words only, never the picture. Waits up to a few minutes; refused while looking is off or Oni is perched on nothing.")
		},
		schema: object(map[string]any{"question": text("the question about the window", NoteMax)}, "question"),
		run:    screenAsk,
	},
}

func remindersList(ctx context.Context, d Daemon, args json.RawMessage) (string, error) {
	if err := decode(args, &struct{}{}); err != nil {
		return "", err
	}
	var res protocol.OniRemindersResult
	if err := d.Call(ctx, protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersList}, &res); err != nil {
		return "", err
	}
	return reminderLines(res), nil
}

func reminderLines(res protocol.OniRemindersResult) string {
	if len(res.Reminders) == 0 {
		return i18n.T("No reminders or timers are pending.")
	}
	var b strings.Builder
	for _, r := range res.Reminders {
		fmt.Fprintf(&b, "%s\t%s\t%s", r.ID, r.Kind, r.Due.Format(time.RFC3339))
		if r.Text != "" {
			b.WriteString("\t" + r.Text)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func remindersAdd(ctx context.Context, d Daemon, args json.RawMessage) (string, error) {
	var a struct {
		Kind string `json:"kind"`
		Due  string `json:"due"`
		Text string `json:"text"`
	}
	if err := decode(args, &a); err != nil {
		return "", err
	}
	kind := a.Kind
	if kind == "" {
		kind = protocol.OniReminderKindReminder
	}
	if kind != protocol.OniReminderKindReminder && kind != protocol.OniReminderKindTimer {
		return "", badArgs(i18n.T("kind is reminder or timer."))
	}
	due, err := time.Parse(time.RFC3339, a.Due)
	if err != nil {
		return "", badArgs(i18n.T("due must be an RFC 3339 time with a time zone."))
	}
	words := ""
	if strings.TrimSpace(a.Text) != "" {
		if words, err = plain(a.Text, TextMax); err != nil {
			return "", err
		}
	} else if kind == protocol.OniReminderKindReminder {
		return "", badArgs(i18n.T("Say what to remind the user of."))
	}
	var res protocol.OniRemindersResult
	if err := d.Call(ctx, protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersAdd, Kind: kind, Due: due, Text: words}, &res); err != nil {
		return "", err
	}
	return i18n.Tf("Set for %s.", due.Format(time.RFC3339)), nil
}

func notesSearch(ctx context.Context, d Daemon, args json.RawMessage) (string, error) {
	var a struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := decode(args, &a); err != nil {
		return "", err
	}
	query, err := plain(a.Query, QueryMax)
	if err != nil {
		return "", err
	}
	limit := a.Limit
	if limit <= 0 || limit > SearchMax {
		limit = SearchMax
	}
	var res protocol.MemorySearchResult
	if err := d.Call(ctx, protocol.MMemorySearch, protocol.MemorySearchParams{Query: query, Scope: protocol.MemoryYou, Limit: limit}, &res); err != nil {
		return "", err
	}
	if len(res.Memories) == 0 {
		return i18n.T("Nothing found."), nil
	}
	var b strings.Builder
	for index, m := range res.Memories {
		if index == limit {
			break
		}
		line := m.Text
		if m.Sensitive {
			line = i18n.T("(a sensitive note, hidden)")
		}
		b.WriteString("- " + strings.ReplaceAll(line, "\n", " ") + "\n")
	}
	return b.String(), nil
}

func notesAdd(ctx context.Context, d Daemon, args json.RawMessage) (string, error) {
	var a struct {
		Text string `json:"text"`
	}
	if err := decode(args, &a); err != nil {
		return "", err
	}
	note, err := plain(a.Text, NoteMax)
	if err != nil {
		return "", err
	}
	var change protocol.MemoryChange
	// the daemon names the suggestion's source from the approved client,
	// whatever is sent here
	if err := d.Call(ctx, protocol.MMemoryImport, protocol.MemoryImportParams{Text: note, Scope: protocol.MemoryYou}, &change); err != nil {
		return "", err
	}
	if len(change.Items) == 0 {
		return i18n.T("Hyakki already has that, or the user asked it to forget it. Nothing new was suggested."), nil
	}
	return i18n.T("Suggested. It waits in Hyakki's Inbox until the user keeps or dismisses it."), nil
}

func focusStatus(ctx context.Context, d Daemon, args json.RawMessage) (string, error) {
	if err := decode(args, &struct{}{}); err != nil {
		return "", err
	}
	var res protocol.OniFocusResult
	if err := d.Call(ctx, protocol.MOniFocus, protocol.OniFocusParams{Action: protocol.OniFocusStatus}, &res); err != nil {
		return "", err
	}
	switch {
	case !res.Armed:
		return i18n.T("The focus guard is off."), nil
	case res.UntilApp != "":
		return i18n.Tf("The focus guard is on until %s opens.", res.UntilApp), nil
	case res.LeftMs > 0:
		return i18n.Tf("The focus guard is on for %d more minutes.", (res.LeftMs+59_999)/60_000), nil
	}
	return i18n.T("The focus guard is on."), nil
}

func notify(ctx context.Context, d Daemon, args json.RawMessage) (string, error) {
	var a struct {
		Text string `json:"text"`
	}
	if err := decode(args, &a); err != nil {
		return "", err
	}
	line, err := plain(a.Text, TextMax)
	if err != nil {
		return "", err
	}
	if strings.Contains(strings.ToLower(line), "://") || strings.Contains(strings.ToLower(line), "www.") {
		return "", badArgs(i18n.T("A notice carries no links."))
	}
	if err := d.Call(ctx, protocol.MOniNotify, protocol.OniNotifyParams{Text: line}, nil); err != nil {
		return "", err
	}
	return i18n.T("Shown."), nil
}

func screenAsk(ctx context.Context, d Daemon, args json.RawMessage) (string, error) {
	var a struct {
		Question string `json:"question"`
	}
	if err := decode(args, &a); err != nil {
		return "", err
	}
	question, err := plain(a.Question, NoteMax)
	if err != nil {
		return "", err
	}
	var res protocol.OniAskResult
	if err := d.Call(ctx, protocol.MOniAsk, protocol.OniAskParams{Text: question}, &res); err != nil {
		return "", err
	}
	switch {
	case res.Status == protocol.OniAskAnswered && res.Answer != nil:
		return res.Answer.Text, nil
	case res.Status == protocol.OniAskDeclined:
		return "", &protocol.Error{Code: protocol.CodeForbidden, Reason: "declined", Message: i18n.T("The user said no, or did not answer in time. Nothing was sent.")}
	}
	return "", &protocol.Error{Code: protocol.CodeInternal, Message: i18n.T("Hyakki answered without an answer. Nothing more was sent.")}
}
