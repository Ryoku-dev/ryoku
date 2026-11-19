package search

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ryoku-i18n"
)

var githubSource = &source{
	SourceInfo: SourceInfo{ID: "github", Name: "GitHub", Scope: "code", Glyph: "code"},
	limit:      3,
	gate:       gateMedium,
	modes:      modeFull,
	search:     searchGitHub,
	detail:     githubDetail,
}

var githubHeaders = map[string]string{"X-GitHub-Api-Version": "2022-11-28"}

type githubRepo struct {
	FullName    string   `json:"full_name"`
	HTMLURL     string   `json:"html_url"`
	Description string   `json:"description"`
	Stars       int64    `json:"stargazers_count"`
	Forks       int64    `json:"forks_count"`
	OpenIssues  int64    `json:"open_issues_count"`
	Language    string   `json:"language"`
	Topics      []string `json:"topics"`
	Homepage    string   `json:"homepage"`
	PushedAt    string   `json:"pushed_at"`
	Archived    bool     `json:"archived"`
	License     *struct {
		SPDX string `json:"spdx_id"`
		Name string `json:"name"`
	} `json:"license"`
	Owner struct {
		Avatar string `json:"avatar_url"`
	} `json:"owner"`
}

func (r githubRepo) license() string {
	if r.License == nil {
		return ""
	}
	if r.License.SPDX != "" && r.License.SPDX != "NOASSERTION" {
		return r.License.SPDX
	}
	return r.License.Name
}

func avatar(u string, size int) string {
	if u == "" {
		return ""
	}
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + "s=" + strconv.Itoa(size)
}

func (e *Service) githubGet(ctx context.Context, engine, u string, v any) error {
	resp, err := e.getJSON(ctx, request{engine: engine, url: u, accept: "application/vnd.github+json", headers: githubHeaders}, v)
	if resp != nil && resp.header.Get("X-RateLimit-Remaining") == "0" {
		// the answer spent the last of the anonymous budget: rest until it resets
		if reset, perr := strconv.ParseInt(resp.header.Get("X-RateLimit-Reset"), 10, 64); perr == nil {
			if d := time.Unix(reset, 0).Sub(e.now()); d > 0 {
				e.gov.hold(engine, d)
			}
		}
	}
	return err
}

func searchGitHub(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	u := "https://api.github.com/search/repositories?" + url.Values{"q": {q.Text}, "per_page": {"10"}}.Encode()
	var res struct {
		Total int64        `json:"total_count"`
		Items []githubRepo `json:"items"`
	}
	if err := e.githubGet(ctx, "github", u, &res); err != nil {
		return nil, false, err
	}
	items := make([]Item, 0, len(res.Items))
	for _, r := range res.Items {
		meta := map[string]any{"stars": r.Stars, "forks": r.Forks, "language": r.Language, "topics": nonNil(r.Topics), "license": r.license()}
		if r.Homepage != "" {
			meta["homepage"] = r.Homepage
		}
		items = append(items, Item{
			Key: r.FullName, Kind: KindRepo, Title: r.FullName, URL: r.HTMLURL, Snippet: plain(r.Description),
			Site: "github.com", Date: r.PushedAt, Meta: meta, thumbURL: avatar(r.Owner.Avatar, 64),
			pop: popularity(float64(r.Stars), 5),
		})
	}
	return items, res.Total > int64(len(res.Items)), nil
}

var repoKey = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func githubDetail(ctx context.Context, e *Service, key, _ string) (Detail, error) {
	if !repoKey.MatchString(key) {
		return Detail{}, errBadKey
	}
	var r githubRepo
	if err := e.githubGet(ctx, "github-api", "https://api.github.com/repos/"+key, &r); err != nil {
		return Detail{}, err
	}
	d := Detail{Title: r.FullName, URL: r.HTMLURL, Text: plain(r.Description), Site: "github.com", thumbURL: avatar(r.Owner.Avatar, 128)}
	d.fact(i18n.T("Stars"), compact(r.Stars))
	d.fact(i18n.T("Forks"), compact(r.Forks))
	d.fact(i18n.T("Open issues"), compact(r.OpenIssues))
	d.fact(i18n.T("Language"), r.Language)
	d.fact(i18n.T("License"), r.license())
	d.fact(i18n.T("Last push"), dateOnly(r.PushedAt))
	if len(r.Topics) > 0 {
		d.fact(i18n.T("Topics"), strings.Join(r.Topics, ", "))
	}
	d.fact(i18n.T("Homepage"), r.Homepage)
	if r.Archived {
		d.fact(i18n.T("Status"), i18n.T("Archived"))
	}
	return d, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// compact formats counts the way the bar shows them: 950, 12.4k, 3.1M.
func compact(n int64) string {
	trim := func(f float64, unit string) string {
		s := strconv.FormatFloat(f, 'f', 1, 64)
		return strings.TrimSuffix(s, ".0") + unit
	}
	switch {
	case n < 1000:
		return strconv.FormatInt(n, 10)
	case n < 1_000_000:
		return trim(float64(n)/1000, "k")
	default:
		return trim(float64(n)/1_000_000, "M")
	}
}

func unixDate(sec int64) string {
	if sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}
