package search

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"ryoku-i18n"
)

var wikipediaSource = &source{
	SourceInfo: SourceInfo{ID: "wikipedia", Name: "Wikipedia", Scope: "knowledge", Glyph: "menu_book"},
	limit:      3,
	gate:       gateLenient,
	modes:      modeBoth,
	search:     searchWikipedia,
	detail:     wikipediaDetail,
}

var wikiLangPattern = regexp.MustCompile(`^[a-z]{2,3}$`)

func wikiLang(lang string) string {
	p := primaryLang(lang)
	if !wikiLangPattern.MatchString(p) {
		return "en"
	}
	return p
}

func wikiPageURL(lang, key string) string {
	return "https://" + lang + ".wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(key, " ", "_"))
}

// While typing, the REST title search answers prefixes ("wayl"); a full
// search uses the page search with excerpts.
func searchWikipedia(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	wl := wikiLang(q.Lang)
	u := "https://" + wl + ".wikipedia.org/w/rest.php/v1/search/page?" + url.Values{"q": {q.Text}, "limit": {"10"}}.Encode()
	want := 10
	if q.suggest {
		u = "https://" + wl + ".wikipedia.org/w/rest.php/v1/search/title?" + url.Values{"q": {q.Text}, "limit": {"5"}}.Encode()
		want = 5
	}
	var res struct {
		Pages []struct {
			Key         string `json:"key"`
			Title       string `json:"title"`
			Excerpt     string `json:"excerpt"`
			Description string `json:"description"`
			Thumbnail   *struct {
				URL string `json:"url"`
			} `json:"thumbnail"`
		} `json:"pages"`
	}
	if _, err := e.getJSON(ctx, request{engine: "wikipedia", url: u, locale: q.Locale}, &res); err != nil {
		return nil, false, err
	}
	items := make([]Item, 0, len(res.Pages))
	for _, p := range res.Pages {
		excerpt := clean(p.Excerpt)
		snippet := plain(p.Description)
		if snippet == "" {
			snippet = excerpt
		}
		it := Item{
			Key: wl + ":" + p.Key, Kind: KindArticle, Title: p.Title, URL: wikiPageURL(wl, p.Key),
			Snippet: snippet, Site: wl + ".wikipedia.org",
			Meta: map[string]any{"description": plain(p.Description), "excerpt": excerpt},
		}
		if p.Thumbnail != nil && p.Thumbnail.URL != "" {
			// the search API hands out 60 px; 120 px is a standard Wikimedia step and stays sharp at 2x
			it.thumbURL = strings.Replace(absURL(p.Thumbnail.URL), "/60px-", "/120px-", 1)
		}
		items = append(items, it)
	}
	return items, len(res.Pages) >= want, nil
}

func wikipediaDetail(ctx context.Context, e *Service, key, lang string) (Detail, error) {
	wl, title, ok := strings.Cut(key, ":")
	if !ok || !wikiLangPattern.MatchString(wl) || title == "" {
		return Detail{}, errBadKey
	}
	u := "https://" + wl + ".wikipedia.org/api/rest_v1/page/summary/" + url.PathEscape(strings.ReplaceAll(title, " ", "_"))
	var s struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Extract     string `json:"extract"`
		Timestamp   string `json:"timestamp"`
		Thumbnail   *struct {
			Source string `json:"source"`
		} `json:"thumbnail"`
		ContentURLs struct {
			Desktop struct {
				Page string `json:"page"`
			} `json:"desktop"`
		} `json:"content_urls"`
	}
	if _, err := e.getJSON(ctx, request{engine: "wikipedia", url: u, locale: lang}, &s); err != nil {
		return Detail{}, err
	}
	d := Detail{Title: s.Title, URL: s.ContentURLs.Desktop.Page, Text: strings.TrimSpace(s.Extract), Site: wl + ".wikipedia.org"}
	if d.URL == "" {
		d.URL = wikiPageURL(wl, title)
	}
	if s.Thumbnail != nil {
		d.thumbURL = s.Thumbnail.Source
	}
	d.fact(i18n.T("Description"), plain(s.Description))
	d.fact(i18n.T("Last edited"), dateOnly(s.Timestamp))
	return d, nil
}
