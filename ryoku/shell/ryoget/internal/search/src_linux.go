package search

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var archwikiSource = &source{
	SourceInfo: SourceInfo{ID: "archwiki", Name: "ArchWiki", Scope: "linux", Glyph: "auto_stories"},
	limit:      3,
	gate:       gateLenient,
	search:     searchArchWiki,
}

// ArchWiki keeps translations as separate pages titled "Page (Language)".
var archLangs = map[string]string{
	"العربية": "ar", "Български": "bg", "Bosanski": "bs", "Català": "ca", "Čeština": "cs", "Dansk": "da",
	"Deutsch": "de", "Ελληνικά": "el", "English": "en", "Esperanto": "eo", "Español": "es", "فارسی": "fa",
	"Suomi": "fi", "Français": "fr", "עברית": "he", "Hrvatski": "hr", "Magyar": "hu", "Indonesia": "id",
	"Italiano": "it", "日本語": "ja", "한국어": "ko", "Lietuvių": "lt", "Latviešu": "lv", "Norsk Bokmål": "nb",
	"Nederlands": "nl", "Polski": "pl", "Português": "pt", "Română": "ro", "Русский": "ru", "Slovenčina": "sk",
	"Српски": "sr", "Svenska": "sv", "ไทย": "th", "Türkçe": "tr", "Українська": "uk", "Tiếng Việt": "vi",
	"简体中文": "zh", "正體中文": "zh",
}

var parenSuffix = regexp.MustCompile(`\s\(([^()]+)\)$`)
var (
	wikiTemplate = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	wikiLabelled = regexp.MustCompile(`\[\[[^\]|]*\|([^\]]*)\]\]`)
	wikiLangLink = regexp.MustCompile(`\[\[[a-z]{2,3}(-[a-z]+)?:[^\]]*\]\]`)
	wikiLink     = regexp.MustCompile(`\[\[([^\]]*)\]\]`)
	wikiMarks    = regexp.MustCompile(`'{2,}|={2,}|(^|\s)[*#]+\s`)
)

// wikiText turns a MediaWiki search snippet (raw wikitext with highlight
// spans) into readable text.
func wikiText(s string) string {
	s = clean(s)
	s = wikiTemplate.ReplaceAllString(s, "")
	s = wikiLabelled.ReplaceAllString(s, "$1")
	s = wikiLangLink.ReplaceAllString(s, "")
	s = wikiLink.ReplaceAllString(s, "$1")
	s = wikiMarks.ReplaceAllString(s, " ")
	return plain(s)
}

func searchArchWiki(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	u := "https://wiki.archlinux.org/api.php?" + url.Values{
		"action": {"query"}, "list": {"search"}, "srsearch": {q.Text}, "format": {"json"}, "formatversion": {"2"},
		"srlimit": {"20"}, "srwhat": {"text"}, "srprop": {"snippet|timestamp"},
	}.Encode()
	var res struct {
		Continue *json.RawMessage `json:"continue"`
		Query    struct {
			Search []struct {
				Title     string `json:"title"`
				Snippet   string `json:"snippet"`
				Timestamp string `json:"timestamp"`
			} `json:"search"`
		} `json:"query"`
	}
	if _, err := e.getJSON(ctx, request{engine: "archwiki", url: u, locale: q.Locale}, &res); err != nil {
		return nil, false, err
	}
	lang := primaryLang(q.Lang)
	var mine, english []Item
	for _, s := range res.Query.Search {
		it := Item{
			Kind: KindArticle, Title: s.Title,
			URL:     "https://wiki.archlinux.org/title/" + url.PathEscape(strings.ReplaceAll(s.Title, " ", "_")),
			Snippet: clip(wikiText(s.Snippet), 240), Site: "wiki.archlinux.org",
			Date: s.Timestamp, Meta: map[string]any{},
		}
		if m := parenSuffix.FindStringSubmatch(s.Title); m != nil {
			if code, ok := archLangs[m[1]]; ok {
				if code == lang && lang != "en" {
					mine = append(mine, it)
				}
				continue
			}
		}
		english = append(english, it)
	}
	items := append(mine, english...)
	if len(items) > 10 {
		items = items[:10]
	}
	return items, res.Continue != nil, nil
}

// nameTier orders package names: exact, prefix, contains, elsewhere.
func nameTier(name string, q Query) int {
	n := strings.ToLower(name)
	joined := strings.Join(q.words, "-")
	switch {
	case n == q.norm || n == joined:
		return 3
	case strings.HasPrefix(n, q.longest()):
		return 2
	case strings.Contains(n, q.longest()):
		return 1
	}
	return 0
}

var archpkgSource = &source{
	SourceInfo: SourceInfo{ID: "archpkg", Name: "Arch packages", Scope: "linux", Glyph: "package_2"},
	limit:      3,
	gate:       gateStrict,
	archOnly:   true,
	search:     searchArchPkg,
}

func searchArchPkg(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	term := q.longest()
	if term == "" {
		return nil, false, nil
	}
	var res struct {
		Results []struct {
			Name       string `json:"pkgname"`
			Repo       string `json:"repo"`
			Arch       string `json:"arch"`
			Version    string `json:"pkgver"`
			Release    string `json:"pkgrel"`
			Epoch      int    `json:"epoch"`
			Desc       string `json:"pkgdesc"`
			LastUpdate string `json:"last_update"`
		} `json:"results"`
	}
	u := "https://archlinux.org/packages/search/json/?" + url.Values{"q": {term}}.Encode()
	if _, err := e.getJSON(ctx, request{engine: "archlinux", url: u}, &res); err != nil {
		return nil, false, err
	}
	var items []Item
	tiers := map[string]int{}
	for _, p := range res.Results {
		// the site matches one phrase; the other words filter here
		if !q.hasAll(p.Name + " " + p.Desc) {
			continue
		}
		version := p.Version + "-" + p.Release
		if p.Epoch > 0 {
			version = strconv.Itoa(p.Epoch) + ":" + version
		}
		link := "https://archlinux.org/packages/" + p.Repo + "/" + p.Arch + "/" + p.Name + "/"
		tiers[link] = nameTier(p.Name, q)
		items = append(items, Item{
			Kind: KindPackage, Title: p.Name, URL: link, Snippet: clip(plain(p.Desc), 240), Site: "archlinux.org",
			Date: p.LastUpdate, Meta: map[string]any{"version": version, "repo": p.Repo, "arch": p.Arch},
		})
	}
	sort.SliceStable(items, func(a, b int) bool { return tiers[items[a].URL] > tiers[items[b].URL] })
	more := len(items) > 10
	if more {
		items = items[:10]
	}
	return items, more, nil
}

var aurSource = &source{
	SourceInfo: SourceInfo{ID: "aur", Name: "AUR", Scope: "linux", Glyph: "archive"},
	limit:      3,
	gate:       gateStrict,
	archOnly:   true,
	search:     searchAUR,
}

func searchAUR(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	term := q.longest()
	if len([]rune(term)) < 2 {
		return nil, false, nil
	}
	var res struct {
		Error   string `json:"error"`
		Results []struct {
			Name         string  `json:"Name"`
			Description  string  `json:"Description"`
			Version      string  `json:"Version"`
			NumVotes     int64   `json:"NumVotes"`
			Popularity   float64 `json:"Popularity"`
			OutOfDate    *int64  `json:"OutOfDate"`
			Maintainer   *string `json:"Maintainer"`
			LastModified int64   `json:"LastModified"`
		} `json:"results"`
	}
	u := "https://aur.archlinux.org/rpc/v5/search/" + url.PathEscape(term) + "?by=name-desc"
	if _, err := e.getJSON(ctx, request{engine: "aur", url: u}, &res); err != nil {
		return nil, false, err
	}
	// "Too many package results." and "Query arg too small." mean nothing to show
	type ranked struct {
		item Item
		tier int
		pop  float64
	}
	var all []ranked
	for _, p := range res.Results {
		if !q.hasAll(p.Name + " " + p.Description) {
			continue
		}
		maint := ""
		if p.Maintainer != nil {
			maint = *p.Maintainer
		}
		link := "https://aur.archlinux.org/packages/" + url.PathEscape(p.Name)
		all = append(all, ranked{
			item: Item{
				Kind: KindPackage, Title: p.Name, URL: link, Snippet: clip(plain(p.Description), 240),
				Site: "aur.archlinux.org", Date: unixDate(p.LastModified),
				Meta: map[string]any{
					"version": p.Version, "votes": p.NumVotes, "popularity": p.Popularity,
					"outOfDate": p.OutOfDate != nil, "maintainer": maint,
				},
				pop: popularity(p.Popularity*100, 4),
			},
			tier: nameTier(p.Name, q), pop: p.Popularity,
		})
	}
	sort.SliceStable(all, func(a, b int) bool {
		if all[a].tier != all[b].tier {
			return all[a].tier > all[b].tier
		}
		return all[a].pop > all[b].pop
	})
	more := len(all) > 10
	if more {
		all = all[:10]
	}
	items := make([]Item, len(all))
	for i, r := range all {
		items[i] = r.item
	}
	return items, more, nil
}

var flathubSource = &source{
	SourceInfo: SourceInfo{ID: "flathub", Name: "Flathub", Scope: "linux", Glyph: "apps"},
	limit:      3,
	gate:       gateMedium,
	search:     searchFlathub,
}

func searchFlathub(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	body, _ := json.Marshal(map[string]string{"query": q.Text, "locale": primaryLang(q.Lang)})
	var res struct {
		TotalHits int `json:"totalHits"`
		Hits      []struct {
			Name      string `json:"name"`
			Summary   string `json:"summary"`
			AppID     string `json:"app_id"`
			Icon      string `json:"icon"`
			Developer string `json:"developer_name"`
			Verified  bool   `json:"verification_verified"`
			License   string `json:"project_license"`
			UpdatedAt int64  `json:"updated_at"`
		} `json:"hits"`
	}
	if _, err := e.getJSON(ctx, request{engine: "flathub", method: "POST", url: "https://flathub.org/api/v2/search", body: body, ctype: "application/json", locale: q.Locale}, &res); err != nil {
		return nil, false, err
	}
	hits := res.Hits
	if len(hits) > 10 {
		hits = hits[:10]
	}
	items := make([]Item, 0, len(hits))
	for _, h := range hits {
		link := "https://flathub.org/apps/" + url.PathEscape(h.AppID)
		items = append(items, Item{
			Key: link, Kind: KindPackage, Title: h.Name, URL: link, Snippet: clip(plain(h.Summary), 240),
			Site: "flathub.org", Date: unixDate(h.UpdatedAt), thumbURL: h.Icon,
			Meta: map[string]any{"appId": h.AppID, "developer": h.Developer, "verified": h.Verified, "license": h.License},
		})
	}
	return items, res.TotalHits > len(items), nil
}
