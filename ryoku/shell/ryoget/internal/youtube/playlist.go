package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ryoget/internal/media"

	i18n "ryoku-i18n"
)

// PlaylistCap bounds how many entries a playlist lists.
const PlaylistCap = 200

// Entry is one video of a playlist.
type Entry struct {
	ID       string
	Title    string
	Thumb    string
	Duration float64
}

// Playlist is a playlist's title, size and first entries.
type Playlist struct {
	ID      string
	Title   string
	Count   int // as YouTube reports it; may exceed len(Entries)
	First   string
	Entries []Entry
}

// Playlist reads list through the browse endpoint, following continuations
// up to PlaylistCap entries. Lists browse cannot open (mixes) fall back to
// the watch-next panel.
func (c *Client) Playlist(ctx context.Context, list string) (*Playlist, error) {
	pl := &Playlist{ID: list}
	seen := map[string]bool{}
	var root map[string]any
	if err := c.call(ctx, webClient, "browse", map[string]any{"browseId": "VL" + list}, "", &root); err == nil {
		pl.Title, pl.Count = playlistHeader(root)
		token := pl.add(root, seen)
		for token != "" && len(pl.Entries) < PlaylistCap && ctx.Err() == nil {
			var next map[string]any
			if err := c.call(ctx, webClient, "browse", map[string]any{"continuation": token}, "", &next); err != nil {
				break
			}
			token = pl.add(next["onResponseReceivedActions"], seen)
		}
	}
	if len(pl.Entries) == 0 {
		var root map[string]any
		if err := c.call(ctx, webClient, "next", map[string]any{"playlistId": list}, "", &root); err != nil {
			return nil, classifyCall(err)
		}
		panel, _ := dig(root, "contents", "twoColumnWatchNextResults", "playlist", "playlist").(map[string]any)
		if panel != nil {
			pl.Title, _ = panel["title"].(string)
			if n, ok := panel["totalVideos"].(float64); ok {
				pl.Count = int(n)
			}
			pl.add(panel["contents"], seen)
		}
	}
	if len(pl.Entries) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("The playlist is empty or private."))
	}
	pl.First = pl.Entries[0].ID
	if pl.Count < len(pl.Entries) {
		pl.Count = len(pl.Entries)
	}
	return pl, nil
}

// add collects the video entries under v in document order and returns the
// continuation token found there, if any.
func (pl *Playlist) add(v any, seen map[string]bool) string {
	token := ""
	var walk func(v any)
	walk = func(v any) {
		if len(pl.Entries) >= PlaylistCap {
			return
		}
		switch x := v.(type) {
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			for _, k := range []string{"lockupViewModel", "playlistVideoRenderer", "playlistPanelVideoRenderer"} {
				if r, ok := x[k].(map[string]any); ok {
					if e, ok := entryFrom(k, r); ok && !seen[e.ID] {
						seen[e.ID] = true
						pl.Entries = append(pl.Entries, e)
					}
					return
				}
			}
			if t, ok := dig(x, "continuationCommand", "token").(string); ok {
				if token == "" {
					token = t
				}
				return
			}
			for _, k := range orderedKeys(x) {
				walk(x[k])
			}
		}
	}
	walk(v)
	return token
}

func entryFrom(kind string, r map[string]any) (Entry, bool) {
	var e Entry
	switch kind {
	case "lockupViewModel":
		if t, _ := r["contentType"].(string); t != "" && t != "LOCKUP_CONTENT_TYPE_VIDEO" {
			return e, false
		}
		e.ID, _ = r["contentId"].(string)
		e.Title, _ = dig(r, "metadata", "lockupMetadataViewModel", "title", "content").(string)
		e.Thumb = bestSource(dig(r, "contentImage", "thumbnailViewModel", "image", "sources"))
		if badges, ok := dig(r, "contentImage", "thumbnailViewModel", "overlays").([]any); ok {
			for _, o := range badges {
				if text, ok := dig(o, "thumbnailBottomOverlayViewModel", "badges").([]any); ok && len(text) > 0 {
					s, _ := dig(text[0], "thumbnailBadgeViewModel", "text").(string)
					e.Duration = parseClock(s)
				}
			}
		}
	default:
		e.ID, _ = r["videoId"].(string)
		e.Title = runsText(r["title"])
		e.Thumb = bestSource(dig(r, "thumbnail", "thumbnails"))
		if s, ok := r["lengthSeconds"].(string); ok {
			e.Duration = atof(s)
		} else {
			e.Duration = parseClock(runsText(r["lengthText"]))
		}
	}
	return e, idRe.MatchString(e.ID)
}

func playlistHeader(root map[string]any) (string, int) {
	title, _ := dig(root, "metadata", "playlistMetadataRenderer", "title").(string)
	count := 0
	for _, path := range [][]string{
		{"header", "playlistHeaderRenderer", "numVideosText"},
		{"header", "playlistHeaderRenderer", "stats"},
	} {
		if n := parseViews(runsText(dig(root, path...))); n > 0 {
			count = int(n)
			break
		}
	}
	if count == 0 {
		// newer page headers only say "1,596 videos" somewhere in their metadata rows
		if rows, ok := dig(root, "header", "pageHeaderRenderer", "content", "pageHeaderViewModel", "metadata").(map[string]any); ok {
			data, _ := json.Marshal(rows)
			for _, part := range strings.Split(string(data), `"content":"`) {
				if i := strings.Index(part, " videos"); i > 0 && i < 16 {
					if n := parseViews(part[:i]); n > 0 {
						count = int(n)
						break
					}
				}
			}
		}
	}
	return title, count
}

func dig(v any, path ...string) any {
	for _, k := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

func runsText(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	if s, ok := m["simpleText"].(string); ok {
		return s
	}
	var b strings.Builder
	if runs, ok := m["runs"].([]any); ok {
		for _, r := range runs {
			if s, ok := dig(r, "text").(string); ok {
				b.WriteString(s)
			}
		}
	}
	return b.String()
}

func bestSource(v any) string {
	list, _ := v.([]any)
	best, w := "", -1.0
	for _, s := range list {
		u, _ := dig(s, "url").(string)
		width, _ := dig(s, "width").(float64)
		if u != "" && width > w {
			best, w = u, width
		}
	}
	if strings.HasPrefix(best, "//") {
		best = "https:" + best
	}
	return best
}
