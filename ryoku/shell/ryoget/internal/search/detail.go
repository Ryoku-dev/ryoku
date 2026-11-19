package search

import (
	"bytes"
	"context"
	"mime"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"

	"ryoget/internal/thumbs"
	"ryoku-i18n"
)

// Detail is the side pane for one selected result.
type Detail struct {
	Source string `json:"source"`
	Key    string `json:"key"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Text   string `json:"text"`
	Site   string `json:"site"`
	Thumb  string `json:"thumb"`
	Facts  []Fact `json:"facts"`
	Error  string `json:"error,omitempty"`

	thumbURL string
}

type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type detailMsg struct {
	ID     int64  `json:"id"`
	Detail Detail `json:"detail"`
}

func (d *Detail) fact(label, value string) {
	if value != "" {
		d.Facts = append(d.Facts, Fact{Label: label, Value: value})
	}
}

// detail answers one detail request; false means the caller went away.
func (e *Service) detail(ctx context.Context, sourceID, key, locale string) (Detail, bool) {
	locale = normLocale(locale)
	s := e.byID[sourceID]
	ck := "detail|" + sourceID + "|" + locale + "|" + key
	v, err := e.do(ctx, ck, cachePolicy{ttl: detailTTL}, nil, func(fctx context.Context) (any, error) {
		var (
			d   Detail
			err error
		)
		switch {
		case s == nil:
			err = errBadKey
		case s.detail != nil:
			d, err = s.detail(fctx, e, key, locale)
		default:
			d, err = e.openGraph(fctx, key, locale)
		}
		if err != nil {
			return nil, err
		}
		if d.thumbURL != "" && e.thumbDir != "" {
			tctx, cancel := context.WithTimeout(fctx, e.thumbTimeout)
			client := e.client
			if s.detail == nil {
				client = e.pageClient
			}
			if p, err := thumbs.Fetch(tctx, client, e.thumbDir, d.thumbURL); err == nil {
				d.Thumb = p
			}
			cancel()
		}
		return d, nil
	})
	if ctx.Err() != nil {
		return Detail{}, false
	}
	var d Detail
	if err != nil {
		d.Error = errorText(err)
		if p, ok := isPaused(err); ok {
			d.Error = i18n.Tf("Paused until %s", p.until.Local().Format("15:04"))
		}
	} else {
		d = v.(Detail)
	}
	d.Source, d.Key = sourceID, key
	if d.Facts == nil {
		d.Facts = []Fact{}
	}
	return d, true
}

// openGraph reads the head of a page for its title, description and image.
func (e *Service) openGraph(ctx context.Context, raw, locale string) (Detail, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Detail{}, errBadKey
	}
	resp, err := e.fetch(ctx, request{
		url: u.String(), accept: "text/html,application/xhtml+xml;q=0.9", locale: locale,
		client: e.pageClient, limit: maxPageBody, truncate: true,
	})
	if err != nil {
		return Detail{}, err
	}
	if err := statusErr(resp.status); err != nil {
		return Detail{}, err
	}
	mt, _, _ := mime.ParseMediaType(resp.header.Get("Content-Type"))
	if mt != "text/html" && mt != "application/xhtml+xml" {
		return Detail{URL: u.String(), Site: hostOf(u.String()), Title: u.String()}, nil
	}
	return parseOpenGraph(resp.body, resp.url), nil
}

func parseOpenGraph(body []byte, base *url.URL) Detail {
	meta := map[string]string{}
	var title strings.Builder
	inTitle := false
	z := xhtml.NewTokenizer(bytes.NewReader(body))
loop:
	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			break loop
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch string(name) {
			case "body":
				break loop
			case "title":
				inTitle = true
			case "meta":
				var key, content string
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					switch string(k) {
					case "property", "name", "itemprop":
						if key == "" {
							key = strings.ToLower(string(v))
						}
					case "content":
						content = string(v)
					}
				}
				if key != "" && content != "" {
					if _, ok := meta[key]; !ok {
						meta[key] = content
					}
				}
			}
		case xhtml.EndTagToken:
			if name, _ := z.TagName(); string(name) == "head" {
				break loop
			} else if string(name) == "title" {
				inTitle = false
			}
		case xhtml.TextToken:
			if inTitle {
				title.Write(z.Text())
			}
		}
	}
	first := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(meta[k]); v != "" {
				return v
			}
		}
		return ""
	}
	d := Detail{URL: base.String()}
	d.Title = plain(first("og:title", "twitter:title"))
	if d.Title == "" {
		d.Title = plain(title.String())
	}
	d.Text = clip(plain(first("og:description", "twitter:description", "description")), 600)
	d.Site = plain(first("og:site_name", "application-name"))
	if d.Site == "" {
		d.Site = hostOf(base.String())
	}
	if img := first("og:image:secure_url", "og:image", "twitter:image", "twitter:image:src"); img != "" {
		if ref, err := url.Parse(img); err == nil {
			d.thumbURL = base.ResolveReference(ref).String()
		}
	}
	if t := first("og:type"); t != "" && t != "website" {
		d.fact(i18n.T("Type"), t)
	}
	if a := first("author", "article:author"); a != "" && !strings.HasPrefix(a, "http") {
		d.fact(i18n.T("Author"), plain(a))
	}
	if p := first("article:published_time"); p != "" {
		d.fact(i18n.T("Published"), dateOnly(p))
	}
	// Sites that refuse programs (Reddit among them) answer every page with
	// their front-page head; showing it would be worse than the row's own
	// title, so the detail leaves title, text and image empty instead.
	if slug := slugWords(base.Path); len(slug) >= 2 && !mentions(d.Title, slug) {
		d.Title, d.thumbURL = "", ""
		if !mentions(d.Text, slug) {
			d.Text = ""
		}
	}
	return d
}

// slugWords are the meaningful words of a URL path ("/r/linux/comments/1b/
// nvidia_wayland_flicker" gives linux, comments, nvidia, wayland, flicker).
func slugWords(path string) []string {
	var out []string
	for _, w := range words(path) {
		if utf8.RuneCountInString(w) < 4 || strings.IndexFunc(w, unicode.IsLetter) < 0 {
			continue
		}
		out = append(out, w)
	}
	return out
}

// mentions reports whether text shares a term with the slug, with the same
// forgiving match the ranking uses.
func mentions(text string, slug []string) bool {
	toks := words(text)
	for _, w := range slug {
		t := newTerm(w)
		for _, tok := range toks {
			if t.matches(tok) {
				return true
			}
		}
	}
	return false
}

func dateOnly(s string) string {
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	return s
}
