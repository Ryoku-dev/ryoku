package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Video is one search result.
type Video struct {
	ID           string
	Title        string
	URL          string
	Channel      string
	Duration     float64 // seconds; 0 when live or unknown
	DurationText string  // "4:12"
	Views        int64   // -1 when unknown
	Published    string  // "5 years ago"
	Thumb        string  // remote i.ytimg.com URL
	Live         bool
	Snippet      string
}

// videos only: the search filter YouTube's own UI sends for "Type: Video"
const videoFilter = "EgIQAQ%3D%3D"

// Search runs a YouTube search through client (nil: a shared default) in
// language lang and returns up to limit videos.
func Search(ctx context.Context, client *http.Client, query, lang string, limit int) ([]Video, error) {
	return For(client, "").Search(ctx, query, lang, limit)
}

// Search runs a video search on c.
func (c *Client) Search(ctx context.Context, query, lang string, limit int) ([]Video, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	var resp json.RawMessage
	body := map[string]any{"query": query, "params": videoFilter}
	if err := c.call(ctx, webClient, "search", body, lang, &resp); err != nil {
		return nil, classifyCall(err)
	}
	return parseSearch(resp, limit)
}

type videoRenderer struct {
	VideoID           string     `json:"videoId"`
	Title             textRuns   `json:"title"`
	OwnerText         textRuns   `json:"ownerText"`
	LongBylineText    textRuns   `json:"longBylineText"`
	LengthText        textRuns   `json:"lengthText"`
	ViewCountText     textRuns   `json:"viewCountText"`
	PublishedTimeText textRuns   `json:"publishedTimeText"`
	Thumbnail         thumbnails `json:"thumbnail"`
	Snippets          []struct {
		SnippetText textRuns `json:"snippetText"`
	} `json:"detailedMetadataSnippets"`
	DescriptionSnippet textRuns `json:"descriptionSnippet"`
	Badges             []struct {
		Badge struct {
			Style string `json:"style"`
			Label string `json:"label"`
		} `json:"metadataBadgeRenderer"`
	} `json:"badges"`
}

func parseSearch(data []byte, limit int) ([]Video, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	var raws []json.RawMessage
	collect(root, &raws)
	var out []Video
	for _, raw := range raws {
		if limit > 0 && len(out) >= limit {
			break
		}
		var r videoRenderer
		if json.Unmarshal(raw, &r) != nil || !idRe.MatchString(r.VideoID) {
			continue
		}
		v := Video{
			ID:           r.VideoID,
			Title:        r.Title.String(),
			URL:          WatchURL(r.VideoID),
			Channel:      firstOf(r.OwnerText.String(), r.LongBylineText.String()),
			DurationText: r.LengthText.String(),
			Published:    r.PublishedTimeText.String(),
			Thumb:        r.Thumbnail.best(),
			Views:        parseViews(r.ViewCountText.String()),
		}
		v.Duration = parseClock(v.DurationText)
		for _, b := range r.Badges {
			if b.Badge.Style == "BADGE_STYLE_TYPE_LIVE_NOW" {
				v.Live = true
			}
		}
		if len(r.Snippets) > 0 {
			v.Snippet = r.Snippets[0].SnippetText.String()
		} else {
			v.Snippet = r.DescriptionSnippet.String()
		}
		out = append(out, v)
	}
	return out, nil
}

// collect gathers every videoRenderer in document order, wherever YouTube
// nests it this month.
func collect(v any, out *[]json.RawMessage) {
	switch x := v.(type) {
	case map[string]any:
		if r, ok := x["videoRenderer"]; ok {
			if b, err := json.Marshal(r); err == nil {
				*out = append(*out, b)
			}
			return
		}
		for _, k := range orderedKeys(x) {
			collect(x[k], out)
		}
	case []any:
		for _, e := range x {
			collect(e, out)
		}
	}
}

// orderedKeys keeps "contents" before side panels so results stay in rank order.
func orderedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	if _, ok := m["contents"]; ok {
		keys = append(keys, "contents")
	}
	for k := range m {
		if k != "contents" && k != "secondaryContents" {
			keys = append(keys, k)
		}
	}
	return keys
}

func parseClock(s string) float64 {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	total := 0
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0
		}
		total = total*60 + n
	}
	return float64(total)
}

func parseViews(s string) int64 {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if digits == "" {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "no views") {
			return 0
		}
		return -1
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return -1
	}
	return n
}
