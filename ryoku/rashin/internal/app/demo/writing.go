package demo

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/writing"
)

var demoVoices sync.Map
var demoWords sync.Map

func demoVoice(s *server) protocol.WritingVoice {
	if stored, ok := demoVoices.Load(s); ok {
		return stored.(protocol.WritingVoice)
	}
	return protocol.WritingVoice{
		Samples: "I pushed the changes this morning, the settings page should feel a bit quicker now. Let me know if anything looks off.",
		Notes:   "Short, direct sentences with plain words. Friendly without a formal greeting. Connect related thoughts with commas, keep detail practical.",
		Updated: "2026-11-18T09:30:00Z",
	}
}

func (s *server) callWriting(method string, raw json.RawMessage) (any, bool, error) {
	switch method {
	case protocol.MWritingCheck:
		var p protocol.WritingCheckParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, true, err
		}
		kinds := p.Kinds
		if len(kinds) == 0 {
			kinds = []string{"grammar"}
		}
		local := make([]string, 0, len(kinds))
		grammar := false
		for _, kind := range kinds {
			if kind == "grammar" {
				grammar = true
			} else {
				local = append(local, kind)
			}
		}
		out := protocol.WritingCheckResult{Engine: "local", Issues: []protocol.WritingIssue{}, Stats: writing.Stats(p.Text)}
		if len(local) > 0 {
			var err error
			out, err = writing.Check(context.Background(), nil, protocol.WritingCheckParams{Text: p.Text, Kinds: local, Off: p.Off})
			if err != nil {
				return nil, true, err
			}
		}
		if grammar {
			out.Engine = "local"
			for _, fix := range []struct{ original, replacement, category string }{
				{"teh", "the", "spelling"}, {"recieve", "receive", "spelling"},
				{"Their going", "They're going", "grammar"}, {"an test", "a test", "grammar"},
				{"the the", "the", "repetition"}, {"has went", "has gone", "grammar"},
			} {
				off := false
				if stored, ok := demoWords.Load(s); ok && fix.category == "spelling" {
					off = off || slices.ContainsFunc(stored.([]string), func(word string) bool { return strings.EqualFold(word, fix.original) })
				}
				for _, category := range p.Off {
					off = off || category == fix.category
				}
				if off {
					continue
				}
				cursor := 0
				for cursor < len(p.Text) {
					at := strings.Index(p.Text[cursor:], fix.original)
					if at < 0 {
						break
					}
					start := cursor + at
					end := start + len(fix.original)
					kind := fix.category
					if kind == "repetition" {
						kind = "style"
					}
					out.Issues = append(out.Issues, protocol.WritingIssue{
						ID: fix.category + "-" + strconv.Itoa(start), Start: len(utf16.Encode([]rune(p.Text[:start]))),
						End: len(utf16.Encode([]rune(p.Text[:end]))), Kind: kind, Category: fix.category,
						Message: "Check this wording.", Replacements: []string{fix.replacement}, Severity: "warning",
					})
					cursor = end
				}
			}
		}
		return out, true, nil
	case protocol.MWritingTransform:
		var p protocol.WritingTransformParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, true, err
		}
		out := protocol.WritingTransformResult{Model: "demo/writing"}
		switch p.Action {
		case "translate":
			out.Text = "Gracias por tus comentarios. He actualizado el borrador y lo compartiré esta tarde."
		case "proofread":
			out.Text = strings.NewReplacer("teh", "the", "recieve", "receive", "the the", "the", "has went", "has gone", "Their going", "They're going", "an test", "a test").Replace(p.Text)
		case "rewrite":
			out.Text = "The settings update is ready. Please try it and let me know what needs work."
		case "tone":
			out.Text = "Thanks for the feedback. I've updated the draft and will share it this afternoon."
		case "fluent":
			out.Text = "I reviewed the changes this morning. Everything looks good, and I'll send the final version tomorrow."
		case "humanize":
			out.Text = "I tried the new layout this morning. The controls are easier to find, but the text still needs a little more space."
		case "enhance":
			out.Text = "Review this settings page for clarity and ease of navigation. Point out controls that are hard to find and suggest specific wording or layout changes. Keep the existing feature set."
		case "lines":
			out.Lines = make([]string, len(p.Lines))
			for i, line := range p.Lines {
				if strings.TrimSpace(line) == "" {
					out.Lines[i] = line
				} else {
					out.Lines[i] = "Gracias por tus comentarios."
				}
			}
			out.Text = strings.Join(out.Lines, "\n")
		default:
			return nil, true, protocol.Errorf(protocol.CodeBadRequest, "Choose a writing action.")
		}
		return out, true, nil
	case protocol.MWritingCancel:
		return protocol.Empty{}, true, nil
	case protocol.MWritingWords, protocol.MWritingWordsAdd, protocol.MWritingWordsRemove:
		words := []string{}
		if stored, ok := demoWords.Load(s); ok {
			words = slices.Clone(stored.([]string))
		}
		if method != protocol.MWritingWords {
			var p protocol.WritingWordParams
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, true, err
			}
			word := strings.TrimSpace(p.Word)
			if word == "" || strings.ContainsAny(word, " \t\r\n") {
				return nil, true, protocol.Errorf(protocol.CodeBadRequest, "Enter one word.")
			}
			index, found := slices.BinarySearchFunc(words, strings.ToLower(word), func(value, key string) int { return strings.Compare(strings.ToLower(value), key) })
			if method == protocol.MWritingWordsAdd && !found {
				words = slices.Insert(words, index, word)
			} else if method == protocol.MWritingWordsAdd && found {
				words[index] = word
			} else if method == protocol.MWritingWordsRemove && found {
				words = slices.Delete(words, index, index+1)
			}
			demoWords.Store(s, words)
		}
		return protocol.WritingWords{Words: words}, true, nil
	case protocol.MWritingVoiceGet:
		return demoVoice(s), true, nil
	case protocol.MWritingVoiceSet:
		var p protocol.WritingVoiceSetParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, true, err
		}
		voice := demoVoice(s)
		if p.Samples != nil {
			voice.Samples = *p.Samples
		}
		if p.Notes != nil {
			voice.Notes = *p.Notes
		}
		voice.Updated = time.Now().UTC().Format(time.RFC3339)
		demoVoices.Store(s, voice)
		return voice, true, nil
	case protocol.MWritingVoiceLearn:
		voice := demoVoice(s)
		voice.Notes = "Plain words, short sentences, practical examples. Friendly but direct, with light punctuation and no formal sign-off."
		voice.Updated = time.Now().UTC().Format(time.RFC3339)
		demoVoices.Store(s, voice)
		return voice, true, nil
	default:
		return nil, false, nil
	}
}
