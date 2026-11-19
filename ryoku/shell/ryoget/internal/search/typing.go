package search

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Suggestions are the only network traffic while typing besides Wikipedia
// titles and the dictionary card; no web engine runs in suggest mode.
var suggestSource = &source{
	SourceInfo: SourceInfo{ID: "suggest", Name: "Suggestions", Scope: "web", Glyph: "search"},
	limit:      8,
	gate:       gateNone,
	lead:       6,
	modes:      modeSuggest,
	search:     searchSuggestions,
}

func searchSuggestions(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	list, engine, err := ddgSuggest(ctx, e, q)
	if err != nil {
		var berr error
		list, berr = bingSuggest(ctx, e, q)
		if berr != nil {
			if allPaused([]error{err, berr}) {
				return nil, false, earliest(err, berr)
			}
			return nil, false, berr
		}
		engine = "bing-suggest"
	}
	seen := map[string]bool{q.norm: true}
	items := make([]Item, 0, len(list))
	for _, s := range list {
		s = plain(s)
		k := strings.ToLower(s)
		if s == "" || seen[k] {
			continue
		}
		seen[k] = true
		items = append(items, Item{Key: "suggest:" + s, Kind: KindSuggestion, Title: s, Meta: map[string]any{"engine": engine}})
	}
	return items, false, nil
}

func ddgSuggest(ctx context.Context, e *Service, q Query) ([]string, string, error) {
	u := "https://duckduckgo.com/ac/?" + url.Values{"q": {q.Text}, "type": {"list"}, "kl": {ddgRegion(q.Lang)}}.Encode()
	resp, err := e.fetch(ctx, request{engine: "ddg-ac", url: u, accept: "application/json", locale: q.Locale})
	if err != nil {
		return nil, "", err
	}
	if err := statusErr(resp.status); err != nil {
		return nil, "", err
	}
	list, err := openSearchList(resp.body)
	return list, "ddg-ac", err
}

func bingSuggest(ctx context.Context, e *Service, q Query) ([]string, error) {
	u := "https://api.bing.com/osjson.aspx?" + url.Values{"query": {q.Text}, "mkt": {q.Locale}}.Encode()
	resp, err := e.fetch(ctx, request{engine: "bing-suggest", url: u, accept: "application/json", locale: q.Locale})
	if err != nil {
		return nil, err
	}
	if err := statusErr(resp.status); err != nil {
		return nil, err
	}
	return openSearchList(resp.body)
}

// openSearchList reads the OpenSearch suggestion shape ["q", ["a", "b"]], and
// DuckDuckGo's older [{"phrase": "a"}] form.
func openSearchList(body []byte) ([]string, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if len(raw) >= 2 {
		var list []string
		if json.Unmarshal(raw[1], &list) == nil {
			return list, nil
		}
	}
	var out []string
	for _, r := range raw {
		var p struct {
			Phrase string `json:"phrase"`
		}
		if json.Unmarshal(r, &p) == nil && p.Phrase != "" {
			out = append(out, p.Phrase)
		}
	}
	return out, nil
}

var dictionarySource = &source{
	SourceInfo: SourceInfo{ID: "dictionary", Name: "Dictionary", Scope: "knowledge", Glyph: "dictionary"},
	limit:      3,
	gate:       gateNone,
	lead:       7,
	modes:      modeBoth,
	applies:    func(q Query) bool { return lookupTerm(q.Text) != "" },
	search:     searchDictionary,
}

var lookupPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^what\s+does\s+(.+?)\s+mean(?:\s+in\s+.+)?$`),
	regexp.MustCompile(`^what\s+is\s+the\s+(?:meaning|definition)\s+of\s+(.+)$`),
	regexp.MustCompile(`^(?:define|definition\s+of|meaning\s+of)\s+(.+)$`),
	regexp.MustCompile(`^(.+?)\s+(?:meaning|definition)$`),
}

// lookupTerm finds the word a query asks about: "what does tbh mean in
// texting", "define ennui", "ennui meaning", or a bare one or two words.
func lookupTerm(text string) string {
	s := strings.ToLower(strings.TrimSpace(text))
	s = strings.TrimRight(s, "?!. ")
	for _, p := range lookupPatterns {
		if m := p.FindStringSubmatch(s); m != nil {
			if t := cleanTerm(m[1]); t != "" && len(strings.Fields(t)) <= 3 {
				return t
			}
			return ""
		}
	}
	if len(strings.Fields(s)) > 2 {
		return ""
	}
	return cleanTerm(s)
}

func cleanTerm(s string) string {
	s = strings.Trim(strings.TrimSpace(s), `"'“”‘’`)
	if utf8.RuneCountInString(s) < 2 {
		return ""
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && r != ' ' && r != '-' && r != '\'' {
			return ""
		}
	}
	return strings.Join(strings.Fields(s), " ")
}

const wiktionaryAttribution = "Wiktionary, CC BY-SA 4.0"

func searchDictionary(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	term := lookupTerm(q.Text)
	if term == "" {
		return nil, false, nil
	}
	items, err := freeDictionary(ctx, e, term, q)
	if err == nil && len(items) > 0 {
		return items, false, nil
	}
	witems, werr := wiktionary(ctx, e, term, q)
	if werr != nil {
		if err != nil {
			if allPaused([]error{err, werr}) {
				return nil, false, earliest(err, werr)
			}
		}
		return nil, false, werr
	}
	return witems, false, nil
}

type dictExample struct{ text string }

func (d *dictExample) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		d.text = s
		return nil
	}
	var o struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return err
	}
	d.text = o.Text
	return nil
}

func freeDictionary(ctx context.Context, e *Service, term string, q Query) ([]Item, error) {
	lang := primaryLang(q.Lang)
	u := "https://freedictionaryapi.com/api/v1/entries/" + url.PathEscape(lang) + "/" + url.PathEscape(term)
	var res struct {
		Word    string `json:"word"`
		Entries []struct {
			PartOfSpeech string `json:"partOfSpeech"`
			Senses       []struct {
				Definition string        `json:"definition"`
				Tags       []string      `json:"tags"`
				Examples   []dictExample `json:"examples"`
			} `json:"senses"`
		} `json:"entries"`
		Source struct {
			URL     string `json:"url"`
			License struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"license"`
		} `json:"source"`
	}
	if _, err := e.getJSON(ctx, request{engine: "freedictionary", url: u, locale: q.Locale}, &res); err != nil {
		return nil, err
	}
	page := res.Source.URL
	if page == "" {
		page = wiktionaryPage(term)
	}
	license := res.Source.License.Name
	if license == "" {
		license = "CC BY-SA 4.0"
	}
	word := res.Word
	if word == "" {
		word = term
	}
	var items []Item
	for _, en := range res.Entries {
		for _, s := range en.Senses {
			def := plain(s.Definition)
			if def == "" {
				continue
			}
			var examples []string
			for _, ex := range s.Examples {
				if t := plain(ex.text); t != "" && len(examples) < 2 {
					examples = append(examples, t)
				}
			}
			items = append(items, definitionItem(word, def, en.PartOfSpeech, s.Tags, examples, "Wiktionary, "+license, page, len(items)))
		}
	}
	return items, nil
}

func definitionItem(word, def, pos string, tags, examples []string, attribution, page string, n int) Item {
	return Item{
		Key: "def:" + word + ":" + strconv.Itoa(n), Kind: KindDefinition, Title: word, Snippet: def,
		URL: page, Site: hostOf(page),
		Meta: map[string]any{
			"partOfSpeech": pos, "tags": nonNil(tags), "examples": nonNil(examples),
			"attribution": attribution, "attributionUrl": page,
		},
	}
}

func wiktionaryPage(term string) string {
	return "https://en.wiktionary.org/wiki/" + url.PathEscape(strings.ReplaceAll(term, " ", "_"))
}

// wiktionary is the fallback: its REST definitions group senses by the
// word's language and carry HTML.
func wiktionary(ctx context.Context, e *Service, term string, q Query) ([]Item, error) {
	u := "https://en.wiktionary.org/api/rest_v1/page/definition/" + url.PathEscape(strings.ReplaceAll(term, " ", "_"))
	resp, err := e.fetch(ctx, request{engine: "wiktionary", url: u, accept: "application/json", locale: q.Locale})
	if err != nil {
		return nil, err
	}
	if resp.status == 404 {
		return nil, nil
	}
	if err := statusErr(resp.status); err != nil {
		return nil, err
	}
	var res map[string][]struct {
		PartOfSpeech string `json:"partOfSpeech"`
		Language     string `json:"language"`
		Definitions  []struct {
			Definition string   `json:"definition"`
			Examples   []string `json:"examples"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(resp.body, &res); err != nil {
		return nil, err
	}
	entries := res[primaryLang(q.Lang)]
	if len(entries) == 0 {
		entries = res["en"]
	}
	// a word's own language before the Translingual symbol entry
	sort.SliceStable(entries, func(a, b int) bool {
		return entries[a].Language != "Translingual" && entries[b].Language == "Translingual"
	})
	page := wiktionaryPage(term)
	var items []Item
	for _, en := range entries {
		for _, d := range en.Definitions {
			def := clean(d.Definition)
			if def == "" {
				continue
			}
			var examples []string
			for _, ex := range d.Examples {
				if t := clean(ex); t != "" && len(examples) < 2 {
					examples = append(examples, t)
				}
			}
			items = append(items, definitionItem(term, def, en.PartOfSpeech, nil, examples, wiktionaryAttribution, page, len(items)))
		}
	}
	return items, nil
}
