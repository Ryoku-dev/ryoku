package search

import (
	"strings"
	"testing"
)

func TestNormURL(t *testing.T) {
	same := [][2]string{
		{"https://www.example.com/a/", "http://example.com/a"},
		{"https://en.m.wikipedia.org/wiki/Wayland", "https://en.wikipedia.org/wiki/Wayland#History"},
		{"https://example.com/p?utm_source=x&id=3", "https://EXAMPLE.com/p?id=3"},
	}
	for _, p := range same {
		if normURL(p[0]) != normURL(p[1]) {
			t.Errorf("%q and %q should dedupe (%q vs %q)", p[0], p[1], normURL(p[0]), normURL(p[1]))
		}
	}
	if normURL("https://example.com/a?id=1") == normURL("https://example.com/a?id=2") {
		t.Error("different query values are different pages")
	}
}

func TestRankDedupesAndBoostsMatches(t *testing.T) {
	q := newQuery("wayland", "en")
	items := rank([]Item{
		{Title: "Something else", URL: "https://a.example/1"},
		{Title: "Weston", URL: "https://a.example/2", Snippet: "a wayland compositor"},
		{Title: "wayland", URL: "https://a.example/3"},
		{Title: "dup", URL: "http://www.a.example/3/"},
	}, q)
	if len(items) != 3 {
		t.Fatalf("duplicate kept: %+v", items)
	}
	if items[0].URL != "https://a.example/3" {
		t.Errorf("exact title should lead, got %q", items[0].Title)
	}
	if items[len(items)-1].Title != "Something else" {
		t.Errorf("non-matching item should sink, got order %v", []string{items[0].Title, items[1].Title, items[2].Title})
	}
	for _, it := range items {
		if it.Meta == nil {
			t.Error("meta must serialise as {}")
		}
	}
}

func TestRankKeepsSourceOrderOtherwise(t *testing.T) {
	q := newQuery("rust async", "en")
	items := rank([]Item{
		{Title: "rust async book", URL: "u1"},
		{Title: "rust async guide", URL: "u2"},
		{Title: "rust async tour", URL: "u3"},
	}, q)
	if items[0].URL != "u1" || items[2].URL != "u3" {
		t.Errorf("equal matches must keep the source's order: %v %v %v", items[0].URL, items[1].URL, items[2].URL)
	}
}

func TestQueryHelpers(t *testing.T) {
	q := newQuery("  NVIDIA   wayland flicker ", "pt_BR.UTF-8")
	if q.Text != "NVIDIA wayland flicker" || q.Lang != "pt-br" || q.longest() != "wayland" {
		t.Errorf("query = %+v longest=%q", q, q.longest())
	}
	if !q.hasAll("Fixes NVIDIA flicker under Wayland") || q.hasAll("nvidia only") {
		t.Error("hasAll")
	}
	if normLang("../etc") != "en-us" || normLang("") != "en-us" || normLang("zh-TW") != "zh-tw" {
		t.Error("normLang must only pass language tags")
	}
	for in, want := range map[string]string{"pt_BR.UTF-8": "pt-BR", "de": "de", "en-us": "en-US", "zh-hant": "zh-hant", "": "en-US"} {
		if got := normLocale(in); got != want {
			t.Errorf("normLocale(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"en-US": "en-US,en;q=0.9", "de": "de,en;q=0.8", "pt-BR": "pt-BR,pt;q=0.9,en;q=0.8", "en": "en"} {
		if got := acceptLanguage(in); got != want {
			t.Errorf("acceptLanguage(%q) = %q, want %q", in, got, want)
		}
	}
	if wikiLang("pt-br") != "pt" || wikiLang("en") != "en" {
		t.Error("wikiLang")
	}
	if w := words("C++ and c# vs Go"); strings.Join(w, ",") != "c++,and,c#,vs,go" {
		t.Errorf("words = %v", w)
	}
}

func TestTextHelpers(t *testing.T) {
	if got := clean(`using the <span class="searchmatch">Wayland</span> protocol &amp; more`); got != "using the Wayland protocol & more" {
		t.Errorf("clean = %q", got)
	}
	if got := plain("Vec<T>   helpers"); got != "Vec<T> helpers" {
		t.Errorf("plain = %q", got)
	}
	if got := clip("one two three four five six", 12); got != "one two…" {
		t.Errorf("clip = %q", got)
	}
	got := htmlText("<p>Run this:</p><pre><code>sudo pacman -S foo\nreboot\n</code></pre><ul><li>first</li><li>second &amp; last</li></ul>", 500)
	want := "Run this:\n\nsudo pacman -S foo\nreboot\n\n- first\n- second & last"
	if got != want {
		t.Errorf("htmlText =\n%q\nwant\n%q", got, want)
	}
	if compact(950) != "950" || compact(12400) != "12.4k" || compact(3_000_000) != "3M" {
		t.Error("compact")
	}
}
