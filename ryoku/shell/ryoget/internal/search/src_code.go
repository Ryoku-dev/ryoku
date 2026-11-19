package search

import (
	"context"
	"net/url"
	"strings"
)

var mdnSource = &source{
	SourceInfo: SourceInfo{ID: "mdn", Name: "MDN Web Docs", Scope: "code", Glyph: "description"},
	limit:      3,
	gate:       gateStrict,
	search:     searchMDN,
}

var mdnLocales = map[string]string{
	"de": "de", "es": "es", "fr": "fr", "ja": "ja", "ko": "ko", "ru": "ru",
	"pt": "pt-BR", "pt-br": "pt-BR", "zh": "zh-CN", "zh-cn": "zh-CN", "zh-tw": "zh-TW",
}

func mdnLocale(lang string) string {
	if l, ok := mdnLocales[lang]; ok {
		return l
	}
	if l, ok := mdnLocales[primaryLang(lang)]; ok {
		return l
	}
	return "en-US"
}

func searchMDN(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	locale := mdnLocale(q.Lang)
	u := "https://developer.mozilla.org/api/v1/search?" + url.Values{"q": {q.Text}, "locale": {locale}, "size": {"10"}}.Encode()
	var res struct {
		Documents []struct {
			URL        string  `json:"mdn_url"`
			Title      string  `json:"title"`
			Summary    string  `json:"summary"`
			Popularity float64 `json:"popularity"`
			Locale     string  `json:"locale"`
		} `json:"documents"`
		Metadata struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
		} `json:"metadata"`
	}
	if _, err := e.getJSON(ctx, request{engine: "mdn", url: u, locale: q.Locale}, &res); err != nil {
		return nil, false, err
	}
	items := make([]Item, 0, len(res.Documents))
	for _, d := range res.Documents {
		link := "https://developer.mozilla.org" + d.URL
		items = append(items, Item{
			Key: link, Kind: KindDoc, Title: plain(d.Title), URL: link, Snippet: clip(plain(d.Summary), 240),
			Site: "developer.mozilla.org", Meta: map[string]any{"locale": d.Locale},
			// MDN popularity is a share of all page views; the top pages sit near 0.1
			pop: clamp01(d.Popularity * 10),
		})
	}
	return items, res.Metadata.Total.Value > len(items), nil
}

var npmSource = &source{
	SourceInfo: SourceInfo{ID: "npm", Name: "npm", Scope: "code", Glyph: "deployed_code"},
	limit:      3,
	gate:       gateStrict,
	search:     searchNpm,
}

func searchNpm(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	u := "https://registry.npmjs.org/-/v1/search?" + url.Values{"text": {q.Text}, "size": {"10"}}.Encode()
	var res struct {
		Total   int `json:"total"`
		Objects []struct {
			Downloads struct {
				Weekly int64 `json:"weekly"`
			} `json:"downloads"`
			Package struct {
				Name        string `json:"name"`
				Version     string `json:"version"`
				Description string `json:"description"`
				Date        string `json:"date"`
				License     string `json:"license"`
				Publisher   struct {
					Username string `json:"username"`
				} `json:"publisher"`
				Links struct {
					Npm        string `json:"npm"`
					Homepage   string `json:"homepage"`
					Repository string `json:"repository"`
				} `json:"links"`
			} `json:"package"`
		} `json:"objects"`
	}
	if _, err := e.getJSON(ctx, request{engine: "npm", url: u}, &res); err != nil {
		return nil, false, err
	}
	items := make([]Item, 0, len(res.Objects))
	for _, o := range res.Objects {
		p := o.Package
		link := p.Links.Npm
		if link == "" {
			link = "https://www.npmjs.com/package/" + p.Name
		}
		meta := map[string]any{"version": p.Version, "weeklyDownloads": o.Downloads.Weekly, "license": p.License, "publisher": p.Publisher.Username}
		if p.Links.Repository != "" {
			meta["repository"] = strings.TrimPrefix(p.Links.Repository, "git+")
		}
		if p.Links.Homepage != "" {
			meta["homepage"] = p.Links.Homepage
		}
		items = append(items, Item{
			Key: link, Kind: KindPackage, Title: p.Name, URL: link, Snippet: clip(plain(p.Description), 240),
			Site: "npmjs.com", Date: p.Date, Meta: meta, pop: popularity(float64(o.Downloads.Weekly), 7),
		})
	}
	return items, res.Total > len(items), nil
}

var cratesSource = &source{
	SourceInfo: SourceInfo{ID: "crates", Name: "crates.io", Scope: "code", Glyph: "inventory_2"},
	limit:      3,
	gate:       gateStrict,
	search:     searchCrates,
}

func searchCrates(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	u := "https://crates.io/api/v1/crates?" + url.Values{"q": {q.Text}, "per_page": {"10"}}.Encode()
	var res struct {
		Crates []struct {
			Name             string `json:"name"`
			Description      string `json:"description"`
			MaxStableVersion string `json:"max_stable_version"`
			MaxVersion       string `json:"max_version"`
			Downloads        int64  `json:"downloads"`
			RecentDownloads  int64  `json:"recent_downloads"`
			UpdatedAt        string `json:"updated_at"`
			Repository       string `json:"repository"`
			Homepage         string `json:"homepage"`
			Documentation    string `json:"documentation"`
			ExactMatch       bool   `json:"exact_match"`
		} `json:"crates"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if _, err := e.getJSON(ctx, request{engine: "crates", url: u}, &res); err != nil {
		return nil, false, err
	}
	items := make([]Item, 0, len(res.Crates))
	for _, c := range res.Crates {
		version := c.MaxStableVersion
		if version == "" {
			version = c.MaxVersion
		}
		meta := map[string]any{"version": version, "downloads": c.Downloads, "recentDownloads": c.RecentDownloads}
		for k, v := range map[string]string{"repository": c.Repository, "homepage": c.Homepage, "documentation": c.Documentation} {
			if v != "" {
				meta[k] = v
			}
		}
		link := "https://crates.io/crates/" + url.PathEscape(c.Name)
		items = append(items, Item{
			Key: link, Kind: KindPackage, Title: c.Name, URL: link, Snippet: clip(plain(c.Description), 240),
			Site: "crates.io", Date: c.UpdatedAt, Meta: meta, pop: popularity(float64(c.RecentDownloads), 7),
		})
	}
	return items, res.Meta.Total > len(items), nil
}
