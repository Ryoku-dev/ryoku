package search

import (
	"context"
	"errors"
	"net/url"

	"ryoget/internal/media"
	"ryoget/internal/youtube"
)

var hackernewsSource = &source{
	SourceInfo: SourceInfo{ID: "hackernews", Name: "Hacker News", Scope: "discuss", Glyph: "forum"},
	limit:      3,
	gate:       gateMedium,
	search:     searchHackerNews,
}

func searchHackerNews(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	u := "https://hn.algolia.com/api/v1/search?" + url.Values{"query": {q.Text}, "tags": {"story"}, "hitsPerPage": {"10"}}.Encode()
	var res struct {
		NbHits int `json:"nbHits"`
		Hits   []struct {
			ObjectID    string `json:"objectID"`
			Title       string `json:"title"`
			URL         string `json:"url"`
			Author      string `json:"author"`
			Points      int64  `json:"points"`
			NumComments int64  `json:"num_comments"`
			CreatedAt   string `json:"created_at"`
			StoryText   string `json:"story_text"`
		} `json:"hits"`
	}
	if _, err := e.getJSON(ctx, request{engine: "hackernews", url: u}, &res); err != nil {
		return nil, false, err
	}
	items := make([]Item, 0, len(res.Hits))
	for _, h := range res.Hits {
		thread := "https://news.ycombinator.com/item?id=" + url.QueryEscape(h.ObjectID)
		link := h.URL
		if link == "" {
			link = thread
		}
		items = append(items, Item{
			Key: link, Kind: KindThread, Title: plain(h.Title), URL: link, Snippet: clip(clean(h.StoryText), 240),
			Site: hostOf(link), Date: h.CreatedAt,
			Meta: map[string]any{"points": h.Points, "comments": h.NumComments, "author": h.Author, "discussion": thread},
			pop:  popularity(float64(h.Points), 3),
		})
	}
	return items, res.NbHits > len(items), nil
}

var youtubeSource = &source{
	SourceInfo: SourceInfo{ID: "youtube", Name: "YouTube", Scope: "video", Glyph: "smart_display"},
	limit:      3,
	gate:       gateMedium,
	search:     searchYouTube,
}

// youtubeSearch is swapped by tests; the real one speaks innertube.
var youtubeSearch = youtube.Search

// The innertube client sends its own requests, so the budget and health are
// applied around the whole call.
func searchYouTube(ctx context.Context, e *Service, q Query) ([]Item, bool, error) {
	release, err := e.gov.acquire(ctx, "youtube", flightOf(ctx).notifyWaiting)
	if err != nil {
		return nil, false, err
	}
	flightOf(ctx).wire()
	wctx, cancel := context.WithTimeout(ctx, defaultWireTimeout)
	// innertube rejects a region-qualified hl such as "en-us"
	videos, err := youtubeSearch(wctx, e.rawClient, q.Text, primaryLang(q.Lang), 12)
	cancel()
	release()
	if err != nil {
		if errors.Is(err, media.ErrRateLimited) {
			return nil, false, &pausedError{engine: "youtube", until: e.gov.strike("youtube", 0)}
		}
		return nil, false, err
	}
	e.gov.ok("youtube")
	items := make([]Item, 0, len(videos))
	for _, v := range videos {
		items = append(items, Item{
			Key: v.URL, Kind: KindVideo, Title: v.Title, URL: v.URL, Snippet: clip(plain(v.Snippet), 240),
			Site: "youtube.com", thumbURL: v.Thumb,
			Meta: map[string]any{
				"videoId": v.ID, "channel": v.Channel, "duration": v.Duration, "durationText": v.DurationText,
				"views": v.Views, "published": v.Published, "live": v.Live,
			},
			pop: popularity(float64(v.Views), 8),
		})
	}
	return items, len(videos) >= 10, nil
}
