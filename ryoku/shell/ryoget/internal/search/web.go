package search

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
)

// The web source is one lane: Keenable and DuckDuckGo lite in parallel,
// merged by reciprocal rank fusion. Bing's RSS feed is asked only when both
// failed or are paused for this query (owner decision, contract section 2).
var webSource = &source{
	SourceInfo: SourceInfo{ID: "web", Name: "Web", Scope: "web", Glyph: "travel_explore"},
	limit:      5,
	gate:       gateLenient,
	lead:       6,
	modes:      modeFull,
	direct:     true,
	search:     searchWeb,
}

// once the first engine answered, the second gets this long to join the merge
const laneGrace = 1500 * time.Millisecond

var (
	laneOrder   = []string{"keenable", "ddg-lite", "bing-rss"}
	laneWeights = map[string]float64{"ddg-lite": 1, "keenable": 1, "bing-rss": 0.9}
)

type laneResult struct {
	engine string
	items  []Item
	err    error
}

// engineResults is one engine's list for a query, cached on its own so the
// lane can merge again without asking anyone.
func (e *Service) engineResults(ctx context.Context, engine string, q Query, fn func(context.Context, *Service, Query) ([]Item, error)) ([]Item, error) {
	key := "eng|" + engine + "|" + q.Lang + "|" + q.norm
	v, err := e.do(ctx, key, cachePolicy{ttl: resultTTL, disk: true}, waitingOf(ctx), func(fctx context.Context) (any, error) {
		items, err := fn(fctx, e, q)
		if err != nil {
			return nil, err
		}
		return cached{items: items}, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(cached).items, nil
}

func searchWeb(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	lctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := make(chan laneResult, 2)
	for _, eng := range []struct {
		id string
		fn func(context.Context, *Service, Query) ([]Item, error)
	}{{"ddg-lite", searchDDGLite}, {"keenable", searchKeenable}} {
		go func(id string, fn func(context.Context, *Service, Query) ([]Item, error)) {
			items, err := e.engineResults(lctx, id, q, fn)
			ch <- laneResult{engine: id, items: items, err: err}
		}(eng.id, eng.fn)
	}
	var got []laneResult
	var grace <-chan time.Time
collect:
	for len(got) < 2 {
		select {
		case r := <-ch:
			got = append(got, r)
			if r.err == nil && grace == nil {
				grace = time.After(laneGrace)
			}
		case <-grace:
			// the slower engine keeps running into the cache for next time
			break collect
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}
	}
	var ok []laneResult
	var errs []error
	for _, r := range got {
		if r.err == nil {
			ok = append(ok, r)
		} else {
			errs = append(errs, r.err)
		}
	}
	if len(ok) == 0 {
		items, err := e.engineResults(ctx, "bing-rss", q, searchBingRSS)
		if err != nil {
			errs = append(errs, err)
			if p := earliest(errs...); p != nil && allPaused(errs) {
				return nil, false, p
			}
			for _, err := range errs {
				if _, paused := isPaused(err); !paused {
					return nil, false, err
				}
			}
			return nil, false, errs[0]
		}
		ok = append(ok, laneResult{engine: "bing-rss", items: items})
	}
	merged := fuse(ok)
	return merged, len(merged) > 0, nil
}

func allPaused(errs []error) bool {
	for _, err := range errs {
		if _, ok := isPaused(err); !ok {
			return false
		}
	}
	return true
}

// fuse merges engine lists by reciprocal rank fusion: score = sum of
// w/(60+rank). Links found by two engines rise; the plain snippet of
// DuckDuckGo or Bing beats Keenable's markdown one.
func fuse(lists []laneResult) []Item {
	type acc struct {
		item    Item
		score   float64
		engines map[string]bool
		first   int
	}
	byURL := map[string]*acc{}
	var all []*acc
	sort.SliceStable(lists, func(a, b int) bool { return lists[a].engine == "ddg-lite" && lists[b].engine != "ddg-lite" })
	for _, l := range lists {
		w := laneWeights[l.engine]
		for i, it := range l.items {
			k := normURL(it.URL)
			if k == "" {
				continue
			}
			a, seen := byURL[k]
			if !seen {
				a = &acc{item: it, engines: map[string]bool{}, first: len(all)}
				byURL[k] = a
				all = append(all, a)
			} else if l.engine != "keenable" && it.Snippet != "" && a.engines["keenable"] && len(a.engines) == 1 {
				a.item.Snippet = it.Snippet
			}
			if a.item.Snippet == "" {
				a.item.Snippet = it.Snippet
			}
			if a.item.Date == "" {
				a.item.Date = it.Date
			}
			a.engines[l.engine] = true
			a.score += w / float64(60+i+1)
		}
	}
	sort.SliceStable(all, func(a, b int) bool { return all[a].score > all[b].score })
	out := make([]Item, 0, len(all))
	for _, a := range all {
		it := a.item
		var engines []string
		for _, id := range laneOrder {
			if a.engines[id] {
				engines = append(engines, id)
			}
		}
		it.Meta = map[string]any{"engines": engines}
		if h := hostOf(it.URL); h != "" {
			it.thumbURL = "https://" + h + "/favicon.ico"
			it.thumbPage = true
		}
		out = append(out, it)
	}
	return out
}

func searchKeenable(ctx context.Context, e *Service, q Query) ([]Item, error) {
	body, _ := json.Marshal(map[string]any{
		"query": q.Text, "mode": "realtime", "max_results": 10, "snippet_max_length": 300,
	})
	var res struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Snippet     string `json:"snippet"`
			Description string `json:"description"`
			PublishedAt string `json:"published_at"`
		} `json:"results"`
	}
	if _, err := e.getJSON(ctx, request{
		engine: "keenable", method: http.MethodPost, url: "https://api.keenable.ai/v1/search/public",
		body: body, ctype: "application/json", locale: q.Locale,
		headers: map[string]string{"X-Keenable-Title": "Ryoku"},
	}, &res); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(res.Results))
	for _, r := range res.Results {
		if !webURL(r.URL) {
			continue
		}
		snippet := r.Description
		if snippet == "" {
			snippet = r.Snippet
		}
		items = append(items, Item{
			Kind: KindPage, Title: plain(r.Title), URL: r.URL, Snippet: clip(mdText(snippet), 240),
			Site: hostOf(r.URL), Date: r.PublishedAt,
		})
	}
	return items, nil
}

func webURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

var (
	mdImage  = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	mdLink   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdMarks  = regexp.MustCompile("\\*\\*|__|`+|(^|\\s)#{1,6}\\s")
	mdBullet = regexp.MustCompile(`(^|\n)\s*[-*+]\s+`)
)

// mdText flattens Keenable's markdown snippets to one line of text.
func mdText(s string) string {
	s = mdImage.ReplaceAllString(s, "$1")
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdBullet.ReplaceAllString(s, "$1")
	s = mdMarks.ReplaceAllString(s, "$1")
	return plain(s)
}

var ddgRegions = map[string]string{
	"de": "de-de", "fr": "fr-fr", "es": "es-es", "it": "it-it", "ja": "jp-jp", "ru": "ru-ru",
	"zh": "cn-zh", "pt": "pt-pt", "nl": "nl-nl", "pl": "pl-pl", "ko": "kr-kr", "tr": "tr-tr",
	"sv": "se-sv", "uk": "ua-uk", "cs": "cz-cs", "da": "dk-da", "fi": "fi-fi", "el": "gr-el",
	"hu": "hu-hu", "id": "id-en", "no": "no-no", "nb": "no-no", "ro": "ro-ro", "vi": "vn-vi",
	"th": "th-th", "he": "il-he", "ar": "xa-ar",
	"pt-br": "br-pt", "en-gb": "uk-en", "en-us": "us-en", "es-mx": "mx-es", "zh-tw": "tw-tzh",
}

func ddgRegion(lang string) string {
	if r, ok := ddgRegions[lang]; ok {
		return r
	}
	if r, ok := ddgRegions[primaryLang(lang)]; ok {
		return r
	}
	return "wt-wt"
}

func searchDDGLite(ctx context.Context, e *Service, q Query) ([]Item, error) {
	u := "https://lite.duckduckgo.com/lite/?" + url.Values{"q": {q.Text}, "kl": {ddgRegion(q.Lang)}}.Encode()
	resp, err := e.fetch(ctx, request{engine: "ddg-lite", url: u, accept: "text/html", locale: q.Locale})
	if err != nil {
		return nil, err
	}
	if err := statusErr(resp.status); err != nil {
		return nil, err
	}
	return parseDDGLite(resp.body)
}

// parseDDGLite reads lite's table: each `a.result-link` row is followed by a
// `td.result-snippet` row; sponsored rows are skipped.
func parseDDGLite(body []byte) ([]Item, error) {
	doc, err := xhtml.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var items []Item
	var cur *Item
	sponsored := false
	walk(doc, func(n *xhtml.Node) bool {
		if n.Type != xhtml.ElementNode {
			return true
		}
		switch {
		case n.Data == "tr":
			if hasClass(n, "result-sponsored") {
				sponsored = true
			} else if find(n, func(c *xhtml.Node) bool { return c.Data == "a" && hasClass(c, "result-link") }) != nil {
				sponsored = false
			}
		case n.Data == "a" && hasClass(n, "result-link"):
			cur = nil
			if sponsored {
				return false
			}
			link := ddgTarget(attr(n, "href"))
			if link == "" {
				return false
			}
			items = append(items, Item{Kind: KindPage, Title: plain(textOf(n)), URL: link, Site: hostOf(link)})
			cur = &items[len(items)-1]
			return false
		case n.Data == "td" && hasClass(n, "result-snippet"):
			if cur != nil && cur.Snippet == "" {
				cur.Snippet = clip(plain(textOf(n)), 240)
			}
			return false
		}
		return true
	})
	return items, nil
}

// ddgTarget unwraps the //duckduckgo.com/l/?uddg= redirect; ad redirects are dropped.
func ddgTarget(href string) string {
	u, err := url.Parse(absURL(href))
	if err != nil {
		return ""
	}
	if strings.HasSuffix(u.Hostname(), "duckduckgo.com") {
		if u.Path == "/y.js" {
			return ""
		}
		if t := u.Query().Get("uddg"); webURL(t) {
			return t
		}
		return ""
	}
	if !webURL(u.String()) {
		return ""
	}
	return u.String()
}

func absURL(s string) string {
	if strings.HasPrefix(s, "//") {
		return "https:" + s
	}
	return s
}

func searchBingRSS(ctx context.Context, e *Service, q Query) ([]Item, error) {
	vals := url.Values{"q": {q.Text}, "format": {"rss"}, "setlang": {q.Locale}}
	if _, region, ok := strings.Cut(q.Locale, "-"); ok {
		vals.Set("cc", region)
	}
	resp, err := e.fetch(ctx, request{engine: "bing-rss", url: "https://www.bing.com/search?" + vals.Encode(), accept: "application/rss+xml, text/xml", locale: q.Locale})
	if err != nil {
		return nil, err
	}
	if err := statusErr(resp.status); err != nil {
		return nil, err
	}
	return parseBingRSS(resp.body)
}

func parseBingRSS(body []byte) ([]Item, error) {
	var feed struct {
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
		} `xml:"channel>item"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, errors.New("bing: unreadable feed")
	}
	items := make([]Item, 0, len(feed.Items))
	for _, r := range feed.Items {
		if !webURL(r.Link) {
			continue
		}
		// pubDate is when Bing built the feed, not when the page was written
		items = append(items, Item{
			Kind: KindPage, Title: plain(r.Title), URL: r.Link, Snippet: clip(plain(r.Description), 240),
			Site: hostOf(r.Link),
		})
	}
	return items, nil
}
