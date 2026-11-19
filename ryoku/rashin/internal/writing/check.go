package writing

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"ryoku-rashin/internal/grammar"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

const maxText = 256 * 1024

var categories = map[string]bool{
	"spelling": true, "grammar": true, "punctuation": true, "capitalization": true,
	"repetition": true, "wordiness": true, "passive": true, "weasel": true,
	"adverbs": true, "cliches": true, "readability": true, "consistency": true,
	"formatting": true, "inclusive": true,
}

func badRequest(message string) error {
	return protocol.Errorf(protocol.CodeBadRequest, "%s", message)
}

func validateText(text string) error {
	if strings.TrimSpace(text) == "" {
		return badRequest("Add some text first.")
	}
	if len(text) > maxText || !utf8.ValidString(text) {
		return badRequest("Use valid text up to 256 KiB.")
	}
	return nil
}

func Check(ctx context.Context, client llm.Client, p protocol.WritingCheckParams) (protocol.WritingCheckResult, error) {
	return checkWith(ctx, client, p, grammar.Check)
}

func checkWith(ctx context.Context, client llm.Client, p protocol.WritingCheckParams, check checker) (protocol.WritingCheckResult, error) {
	out := protocol.WritingCheckResult{Engine: "local", Issues: []protocol.WritingIssue{}}
	if err := validateText(p.Text); err != nil {
		return out, err
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	engine := p.Engine
	if engine == "" {
		engine = "auto"
	}
	if engine != "auto" && engine != "local" && engine != "model" {
		return out, badRequest("Choose auto, local or model for the writing engine.")
	}
	kinds := p.Kinds
	if len(kinds) == 0 {
		kinds = []string{"grammar"}
	}
	wanted := make(map[string]bool, len(kinds))
	for _, kind := range kinds {
		switch kind {
		case "grammar", "clarity", "inclusive", "readability", "aitell":
			wanted[kind] = true
		default:
			return out, badRequest("Choose grammar, clarity, inclusive, readability or aitell.")
		}
	}
	out.Stats = Stats(p.Text)
	if wanted["grammar"] {
		var issues []protocol.WritingIssue
		var err error
		var words []string
		if engine != "model" {
			words, err = dictionaryWords()
			if err != nil {
				return out, err
			}
			if engine == "auto" && !grammar.LooksEnglish(p.Text, words) {
				configured, err := modelConfigured(ctx, client, p.Local)
				if err != nil {
					return out, err
				}
				if configured {
					engine = "model"
				}
			}
		}
		if engine != "model" {
			issues, err = localCheck(ctx, p.Text, p.Dialect, words, check)
			if err != nil {
				return out, err
			}
		}
		if engine == "model" {
			issues, err = modelCheck(ctx, client, p)
			out.Engine = "model"
			if err != nil {
				return out, err
			}
		}
		out.Issues = append(out.Issues, issues...)
	}
	if wanted["clarity"] {
		out.Issues = append(out.Issues, clarityIssues(p.Text)...)
	}
	if wanted["inclusive"] {
		out.Issues = append(out.Issues, inclusiveIssues(p.Text)...)
	}
	if wanted["aitell"] {
		result := AITell(p.Text)
		out.AITell = &result
	}
	off := make(map[string]bool, len(p.Off))
	for _, category := range p.Off {
		off[category] = true
	}
	out.Issues = filterIssues(out.Issues, off)
	if out.AITell != nil {
		out.AITell.Phrases = filterIssues(out.AITell.Phrases, off)
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	return out, nil
}

func modelConfigured(ctx context.Context, client llm.Client, localOnly bool) (bool, error) {
	if client == nil {
		return false, nil
	}
	providers, ok := client.(interface {
		Providers(context.Context) ([]protocol.ProviderInfo, error)
	})
	if !ok {
		return true, nil
	}
	list, err := providers.Providers(ctx)
	if err != nil {
		return false, err
	}
	for _, provider := range list {
		if provider.Enabled && provider.Configured && provider.Models > 0 &&
			(!localOnly || provider.Local) && (!provider.Local || provider.Running) {
			return true, nil
		}
	}
	return false, nil
}

func filterIssues(issues []protocol.WritingIssue, off map[string]bool) []protocol.WritingIssue {
	out := make([]protocol.WritingIssue, 0, len(issues))
	seen := make(map[string]bool)
	for _, issue := range issues {
		key := fmt.Sprintf("%d:%d", issue.Start, issue.End)
		if off[issue.Category] || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, issue)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].End < out[j].End
	})
	for i := range out {
		out[i].ID = fmt.Sprintf("%s-%d-%d-%d", out[i].Category, out[i].Start, out[i].End, i)
	}
	return out
}

func utf16Len(text string) int {
	n := 0
	for _, r := range text {
		n++
		if r > 0xffff {
			n++
		}
	}
	return n
}

func byteIssue(text string, start, end int, kind, category, message string, replacements ...string) protocol.WritingIssue {
	return protocol.WritingIssue{
		Start: utf16Len(text[:start]), End: utf16Len(text[:end]), Kind: kind,
		Category: category, Message: message, Replacements: replacements, Severity: "hint",
	}
}
