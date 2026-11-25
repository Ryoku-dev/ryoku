// Package grammar checks plain English locally using Harper 3.2.0's rule core.
package grammar

import (
	"context"
	_ "embed"
	"encoding/json"
	"hash/maphash"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"sync"

	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/rules"
)

type Options struct {
	Dialect  string
	Disabled []string
	Words    []string
}
type Issue struct {
	Start, End          int
	Rule, Kind, Message string
	Replacements        []string
	Priority            int
}
type RuleInfo struct {
	Name, Kind, Description string
	DefaultOn               bool
}

// A borrowed group owns all its mutable rule caches. Neither its configuration
// nor its child linters are shared with another simultaneous Check.
type dialectState struct{ pool sync.Pool }
type sharedState struct {
	dictionary  dict.Dictionary
	dialects    sync.Map
	wordsMu     sync.Mutex
	words       []string
	fingerprint uint64
	withWords   *sharedState
}

var shared = sync.OnceValue(func() *sharedState { return &sharedState{dictionary: dict.Curated()} })

var releaseInitializationGarbage sync.Once

var wordHashSeed = maphash.MakeSeed()

func (s *sharedState) forWords(words []string) *sharedState {
	if len(words) == 0 {
		s.wordsMu.Lock()
		s.words, s.withWords = nil, nil
		s.fingerprint = 0
		s.wordsMu.Unlock()
		return s
	}
	var hash maphash.Hash
	hash.SetSeed(wordHashSeed)
	for _, word := range words {
		hash.WriteString(word)
		hash.WriteByte(0)
	}
	fingerprint := hash.Sum64()
	s.wordsMu.Lock()
	defer s.wordsMu.Unlock()
	if s.withWords != nil && s.fingerprint == fingerprint && slices.Equal(s.words, words) {
		return s.withWords
	}
	user := dict.NewMutableDictionary()
	allDialects := dict.AMERICAN | dict.BRITISH | dict.CANADIAN | dict.AUSTRALIAN | dict.INDIAN
	for _, word := range words {
		user.AppendWordStr(word, dict.DictWordMetadata{Dialects: allDialects})
		if lower := strings.ToLower(word); lower != word {
			user.AppendWordStr(lower, dict.DictWordMetadata{Dialects: allDialects})
		}
	}
	merged := dict.NewMergedDictionary()
	merged.AddDictionary(s.dictionary)
	merged.AddDictionary(user)
	s.words, s.fingerprint = slices.Clone(words), fingerprint
	s.withWords = &sharedState{dictionary: merged}
	return s.withWords
}

// LooksEnglish is a routing heuristic, never a prerequisite for local checking.
func LooksEnglish(source string, words []string) bool {
	if strings.TrimSpace(source) == "" {
		return true
	}
	dictionary := shared().forWords(words).dictionary
	document := doc.NewPlainEnglish(source, dictionary)
	chars := document.GetSource()
	total, valid, unlintable := 0, 0, 0
	for _, token := range document.Tokens() {
		switch token.Kind.Kind {
		case doc.Word:
			total++
			if dictionary.ContainsWord(token.GetCh(chars)) {
				valid++
			}
		case doc.Unlintable:
			unlintable++
		}
	}
	if total > 7 {
		return doc.IsDocLikelyEnglish(document, dictionary)
	}
	if total == 0 {
		return unlintable == 0
	}
	return total-valid <= max(1, total/3) && unlintable <= valid
}

func (s *sharedState) state(dialect dict.Dialect) *dialectState {
	if value, ok := s.dialects.Load(dialect); ok {
		return value.(*dialectState)
	}
	candidate := &dialectState{}
	candidate.pool.New = func() any { return rules.NewCurated(s.dictionary, dialect) }
	value, _ := s.dialects.LoadOrStore(dialect, candidate)
	return value.(*dialectState)
}
func parseDialect(value string) dict.Dialect {
	switch strings.ToLower(value) {
	case "uk":
		return dict.British
	case "ca":
		return dict.Canadian
	case "au":
		return dict.Australian
	case "in":
		return dict.Indian
	default:
		return dict.American
	}
}

func Check(ctx context.Context, source string, opts Options) ([]Issue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	state := shared().forWords(opts.Words)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dictionary := state.dictionary
	document := doc.NewPlainEnglish(source, dictionary)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dialect := parseDialect(opts.Dialect)
	ds := state.state(dialect)
	group := ds.pool.Get().(*lint.LintGroup)
	defer ds.pool.Put(group)
	// Models and the first curated group are now initialized. Reclaim their
	// one-off construction buffers once, not on subsequent checks or a timer.
	releaseInitializationGarbage.Do(debug.FreeOSMemory)
	// Restore only altered options on return, preserving warm group caches.
	previous := make(map[string]*bool, len(opts.Disabled))
	present := make(map[string]bool, len(opts.Disabled))
	for _, name := range opts.Disabled {
		if _, seen := present[name]; seen {
			continue
		}
		v, ok := group.Config[name]
		previous[name] = v
		present[name] = ok
		group.Config.SetRuleEnabled(name, false)
	}
	defer func() {
		for name, exists := range present {
			if exists {
				group.Config[name] = previous[name]
			} else {
				delete(group.Config, name)
			}
		}
	}()
	organized, err := group.OrganizedLintsContext(ctx, document)
	if err != nil {
		return nil, err
	}
	lint.RemoveOverlapsMap(organized)
	keys := make([]string, 0, len(organized))
	for key := range organized {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var issues []Issue
	chars := document.GetSource()
	for _, name := range keys {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, entry := range organized[name] {
			issues = append(issues, issueFromLint(name, entry, chars))
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		if a.End != b.End {
			return a.End > b.End
		}
		return a.Priority < b.Priority
	})
	return issues, nil
}

func issueFromLint(name string, entry lint.Lint, source []rune) Issue {
	issue := Issue{Start: entry.Span.Start, End: entry.Span.End, Rule: name, Kind: entry.LintKind.String(), Message: entry.Message, Priority: int(entry.Priority)}
	for _, suggestion := range entry.Suggestions {
		replacement := string(suggestion.Replacement)
		switch suggestion.Kind {
		case lint.InsertAfterKind:
			replacement = string(source[entry.Span.Start:entry.Span.End]) + replacement
		case lint.RemoveKind:
			replacement = ""
		}
		duplicate := false
		for _, existing := range issue.Replacements {
			if existing == replacement {
				duplicate = true
				break
			}
		}
		if !duplicate {
			issue.Replacements = append(issue.Replacements, replacement)
		}
	}
	return issue
}

// Generated from Harper 3.2.0's `config --no-color`, excluding BoringWords.
// This keeps opening rule settings from initializing grammar models.
//
//go:embed rule_metadata.json
var ruleMetadataJSON []byte

var ruleMetadata = sync.OnceValue(func() []RuleInfo {
	var out []RuleInfo
	if err := json.Unmarshal(ruleMetadataJSON, &out); err != nil {
		panic(err)
	}
	return out
})

func Rules() []RuleInfo { return append([]RuleInfo(nil), ruleMetadata()...) }
