package search

import (
	"context"
	"strings"
	"testing"
)

func TestSuggestionsFromDuckDuckGo(t *testing.T) {
	f := newFixtures(t).standard()
	e := testService(t, f, "")
	items, _, err := searchSuggestions(context.Background(), e, newQuery("what does tb", "en-US"))
	if err != nil || len(items) != 8 {
		t.Fatalf("err=%v items=%d", err, len(items))
	}
	if items[0].Kind != KindSuggestion || items[0].Title != "what does tba mean" || items[0].URL != "" || items[0].Meta["engine"] != "ddg-ac" {
		t.Errorf("first = %+v", items[0])
	}
	if f.count("api.bing.com/osjson.aspx") != 0 {
		t.Error("Bing asked although DuckDuckGo answered")
	}
}

func TestSuggestionsFallBackToBing(t *testing.T) {
	f := newFixtures(t).standard()
	f.set("duckduckgo.com/ac/", route{status: 503})
	e := testService(t, f, "")
	items, _, err := searchSuggestions(context.Background(), e, newQuery("what does tb", "en-US"))
	if err != nil || len(items) < 10 || items[0].Meta["engine"] != "bing-suggest" {
		t.Fatalf("err=%v items=%d first=%+v", err, len(items), items)
	}
	if got := f.last("api.bing.com/osjson.aspx").URL.Query().Get("mkt"); got != "en-US" {
		t.Errorf("mkt = %q", got)
	}
	// DuckDuckGo is now paused: the next keystroke goes straight to Bing
	before := f.count("duckduckgo.com/ac/")
	if _, _, err := searchSuggestions(context.Background(), e, newQuery("what does tbh", "en-US")); err != nil {
		t.Fatal(err)
	}
	if f.count("duckduckgo.com/ac/") != before {
		t.Error("a paused engine was asked")
	}
}

func TestOpenSearchShapes(t *testing.T) {
	list, err := openSearchList([]byte(`["tb",["tbh","tba"]]`))
	if err != nil || strings.Join(list, ",") != "tbh,tba" {
		t.Errorf("opensearch = %v %v", list, err)
	}
	list, err = openSearchList([]byte(`[{"phrase":"tbh"},{"phrase":"tba"}]`))
	if err != nil || strings.Join(list, ",") != "tbh,tba" {
		t.Errorf("phrase list = %v %v", list, err)
	}
}

func TestWikipediaTitleSearchWhileTyping(t *testing.T) {
	f := newFixtures(t).standard()
	e := testService(t, f, "")
	q := newQuery("wayl", "en-US")
	q.suggest = true
	items, _, err := searchWikipedia(context.Background(), e, q)
	if err != nil || len(items) != 5 {
		t.Fatalf("err=%v items=%d", err, len(items))
	}
	if items[1].Title != "Wayland (protocol)" || items[1].Snippet != "Display system intended to replace X11" {
		t.Errorf("second = %+v", items[1])
	}
	if f.count("en.wikipedia.org/w/rest.php/v1/search/page") != 0 {
		t.Error("typing used the page search")
	}
}

func TestLookupTerm(t *testing.T) {
	for in, want := range map[string]string{
		"what does tbh mean in texting": "tbh",
		"What does “ennui” mean?":       "ennui",
		"define ennui":                  "ennui",
		"definition of ice cream":       "ice cream",
		"ennui meaning":                 "ennui",
		"tbh":                           "tbh",
		"ice cream":                     "ice cream",
		"nvidia wayland flicker":        "",
		"x":                             "",
		"c++":                           "",
		"what does 2fa mean":            "",
	} {
		if got := lookupTerm(in); got != want {
			t.Errorf("lookupTerm(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDictionaryCardWithAttribution(t *testing.T) {
	f := newFixtures(t).standard()
	e := testService(t, f, "")
	items, _, err := searchDictionary(context.Background(), e, newQuery("what does tbh mean in texting", "en-US"))
	if err != nil || len(items) != 1 {
		t.Fatalf("err=%v items=%+v", err, items)
	}
	it := items[0]
	if it.Kind != KindDefinition || it.Title != "tbh" || !strings.Contains(it.Snippet, "to be honest") {
		t.Errorf("item = %+v", it)
	}
	if it.Meta["partOfSpeech"] != "preposition phrase" || it.Meta["attributionUrl"] != "https://en.wiktionary.org/wiki/tbh" ||
		!strings.Contains(it.Meta["attribution"].(string), "CC BY-SA 4.0") {
		t.Errorf("meta = %+v", it.Meta)
	}
	if f.count("en.wiktionary.org/api/rest_v1/page/definition/tbh") != 0 {
		t.Error("Wiktionary asked although FreeDictionaryAPI answered")
	}
}

func TestDictionaryFallsBackToWiktionary(t *testing.T) {
	f := newFixtures(t).standard()
	f.set("freedictionaryapi.com/api/v1/entries/en/tbh", route{file: "fdict404.json"})
	e := testService(t, f, "")
	items, _, err := searchDictionary(context.Background(), e, newQuery("tbh", "en-US"))
	if err != nil || len(items) != 2 {
		t.Fatalf("err=%v items=%+v", err, items)
	}
	if !strings.Contains(items[0].Snippet, "to be honest") || items[0].Meta["partOfSpeech"] != "Prepositional phrase" {
		t.Errorf("the English sense must lead the Translingual one: %+v", items[0])
	}
	if strings.Contains(items[0].Snippet, "<") || items[0].Meta["attribution"] != wiktionaryAttribution {
		t.Errorf("first = %+v", items[0])
	}
}

func TestSuggestModeNeverTouchesAWebEngine(t *testing.T) {
	f := newFixtures(t).standard()
	e := testService(t, f, "")
	got := collectMode(e, "what does tbh mean in texting", "all", "suggest")
	for src := range got {
		if s := e.byID[src]; s.runsIn()&modeSuggest == 0 {
			t.Errorf("%s ran while typing", src)
		}
	}
	if got["dictionary"].Rank != 7 || len(got["dictionary"].Items) == 0 || len(got["suggest"].Items) == 0 {
		t.Errorf("suggest mode = %+v", got)
	}
	for _, host := range []string{"api.keenable.ai/v1/search/public", "lite.duckduckgo.com/lite/", "www.bing.com/search", "api.github.com/search/repositories", "en.wikipedia.org/w/rest.php/v1/search/page"} {
		if n := f.count(host); n != 0 {
			t.Errorf("%s asked %d times in suggest mode", host, n)
		}
	}

	full := collectMode(e, "nvidia wayland flicker", "all", "full")
	if _, ok := full["suggest"]; ok {
		t.Error("suggestions ran in full mode")
	}
	if _, ok := full["dictionary"]; ok {
		t.Error("the dictionary ran for a query that is not a lookup")
	}
	if _, ok := full["web"]; !ok {
		t.Error("full mode skipped the web lane")
	}
}
