// Package memoryui holds the memory pieces more than one page shows: a
// memory's row and its editor, the dialogs that forget a scope or import
// from another assistant, the read-only sheet, and the words they share.
package memoryui

import (
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// ProofParts is a row's proof sentence cut around the source's title, so the
// title can be drawn as a link: Before, Title, After read as one sentence.
type ProofParts struct {
	Before, Title, After string
}

// String is the whole sentence.
func (parts ProofParts) String() string { return parts.Before + parts.Title + parts.After }

// Proof says where a memory came from, when, and how much it was used.
func Proof(now time.Time, memory protocol.Memory) string { return ProofOf(now, memory).String() }

// ProofOf is Proof with the source's title apart.
func ProofOf(now time.Time, memory protocol.Memory) ProofParts {
	parts := SourceOf(now, memory)
	if memory.Status == protocol.MemoryActive || memory.Status == "" {
		parts.After += " " + Used(now, memory)
	}
	return parts
}

// SourceOf says where a memory came from and when, with the source's title
// apart.
func SourceOf(now time.Time, memory protocol.Memory) ProofParts {
	source, day := memory.Source, Day(now, memory.Created)
	var parts ProofParts
	switch {
	case source.Kind == protocol.MemoryFromTeam && source.Title != "":
		parts = ProofParts{Before: i18n.T("From the run of "), Title: source.Title, After: " " + On(now, memory.Created) + "."}
	case (source.Kind == protocol.MemoryFromAsk || source.Kind == protocol.MemoryFromAgent) && source.Title != "":
		parts = ProofParts{Before: i18n.T("From "), Title: source.Title, After: i18n.Tf(", %s.", day)}
	case source.Kind == protocol.MemoryFromYou:
		parts.Before = i18n.Tf("You added this, %s.", day)
	case source.Kind == protocol.MemoryFromImport:
		from := source.From
		if from == "" {
			from = i18n.T("another assistant")
		}
		parts.Before = i18n.Tf("Imported from %s, %s.", from, day)
	case source.Kind == protocol.MemoryFromFile:
		parts.Before = i18n.Tf("Written into the file, %s.", day)
	case source.Model != "":
		parts.Before = i18n.Tf("Learned by %s, %s.", source.Model, day)
	default:
		parts.Before = i18n.Tf("Learned %s.", day)
	}
	return parts
}

// Used says how often answers drew on a memory and when last.
func Used(now time.Time, memory protocol.Memory) string {
	switch {
	case memory.Uses == 0 || memory.LastUsed == nil:
		return i18n.T("Not used yet.")
	case memory.Uses == 1:
		return i18n.Tf("Used once, %s.", Day(now, *memory.LastUsed))
	}
	return i18n.Tf("Used %d times, last %s.", memory.Uses, Day(now, *memory.LastUsed))
}

// Day is a date as a proof says it: today, yesterday, or "3 Nov".
func Day(now, at time.Time) string {
	if at.IsZero() {
		return "-"
	}
	local, today := at.Local(), now.Local()
	midnight := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local) }
	switch int(midnight(today).Sub(midnight(local)).Hours() / 24) {
	case 0:
		return i18n.T("today")
	case 1:
		return i18n.T("yesterday")
	}
	if local.Year() != today.Year() {
		return local.Format("2 Jan 2006")
	}
	return local.Format("2 Jan")
}

// On is a date after a verb: today, yesterday, or "on 3 Nov".
func On(now, at time.Time) string {
	day := Day(now, at)
	if day == i18n.T("today") || day == i18n.T("yesterday") {
		return day
	}
	return i18n.Tf("on %s", day)
}

// StateWords are the claim and proof of a row that has a state: a suggestion
// waiting for the reader, or a memory a newer one replaced. Ordinary
// memories have none. Teams name the team a suggestion came from.
func StateWords(now time.Time, memory protocol.Memory, teams []protocol.Team) (claim, proof string) {
	switch memory.Status {
	case protocol.MemorySuggested:
		by := ""
		switch {
		case memory.Source.Kind == protocol.MemoryFromImport:
			by = i18n.T("an import")
		case memory.Source.Team != "":
			for _, team := range teams {
				if team.ID == memory.Source.Team {
					by = team.Name
				}
			}
		}
		if by == "" {
			return i18n.T("Needs you"), i18n.Tf("suggested %s", look.Ago(now, memory.Created))
		}
		return i18n.T("Needs you"), i18n.Tf("suggested by %s %s", by, look.Ago(now, memory.Created))
	case protocol.MemoryReplaced:
		when := ""
		if memory.Replaced != nil {
			when = On(now, *memory.Replaced)
		}
		if memory.ReplacedByText == "" {
			return i18n.T("Replaced"), when
		}
		return i18n.T("Replaced"), i18n.Tf("by \"%s\" %s", env.PlainMemory(memory.ReplacedByText), when)
	case protocol.MemoryBlocked:
		return i18n.T("Not used"), memory.Blocked
	}
	return "", ""
}

// Words cuts a memory's text into words, each marked literal when it sits in
// a code span, which the engine uses for paths and commands.
func Words(text string) []Word {
	var words []Word
	for index, piece := range strings.Split(text, "`") {
		literal := index%2 == 1
		for _, word := range strings.Fields(piece) {
			words = append(words, Word{Text: word, Literal: literal})
		}
	}
	return words
}

// Word is one word of a memory's text.
type Word struct {
	Text    string
	Literal bool
}
