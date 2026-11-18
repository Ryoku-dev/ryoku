package writing

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"ryoku-rashin/internal/grammar"
	"ryoku-rashin/internal/protocol"
)

type checker func(context.Context, string, grammar.Options) ([]grammar.Issue, error)

func localCheck(ctx context.Context, text, dialect string, words []string, check checker) ([]protocol.WritingIssue, error) {
	if dialect == "" {
		dialect = "us"
	}
	switch dialect {
	case "us", "uk", "ca", "au", "in":
	default:
		return nil, badRequest("Choose us, uk, ca, au or in as the writing dialect.")
	}
	issues, err := check(ctx, text, grammar.Options{Dialect: dialect, Words: words})
	if err != nil {
		return nil, err
	}
	offsets := make([]int, 1, utf8.RuneCountInString(text)+1)
	n := 0
	for _, r := range text {
		n++
		if r > 0xffff {
			n++
		}
		offsets = append(offsets, n)
	}
	out := make([]protocol.WritingIssue, 0, len(issues))
	for _, issue := range issues {
		if issue.Start < 0 || issue.End < issue.Start || issue.End >= len(offsets) {
			return nil, errors.New("Correction outside checked text.")
		}
		kind, category := localCategory(issue.Kind, issue.Rule)
		severity := "warning"
		if category == "spelling" || category == "grammar" {
			severity = "error"
		} else if kind == "style" {
			severity = "hint"
		}
		out = append(out, protocol.WritingIssue{Start: offsets[issue.Start], End: offsets[issue.End], Kind: kind, Category: category, Message: issue.Message, Severity: severity, Replacements: issue.Replacements})
	}
	return out, ctx.Err()
}

func localCategory(kind, rule string) (string, string) {
	switch rule {
	case "RepeatedWords", "MultipleSequentialPronouns":
		return "style", "repetition"
	}
	switch strings.ReplaceAll(strings.ToLower(kind), " ", "") {
	case "spelling", "typo":
		return "spelling", "spelling"
	case "capitalization":
		return "grammar", "capitalization"
	case "punctuation":
		return "punctuation", "punctuation"
	case "formatting":
		return "style", "formatting"
	case "repetition":
		return "style", "repetition"
	case "redundancy", "wordchoice":
		return "clarity", "wordiness"
	case "readability", "enhancement":
		return "clarity", "readability"
	case "style", "regionalism":
		return "style", "consistency"
	default:
		return "grammar", "grammar"
	}
}
