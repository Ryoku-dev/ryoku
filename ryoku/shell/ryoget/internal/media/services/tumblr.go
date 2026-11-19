package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var tumblrBase = "https://www.tumblr.com"

type tumblr struct{}

func init() { media.Register(tumblr{}) }

func (tumblr) ID() string   { return "tumblr" }
func (tumblr) Name() string { return "Tumblr" }

func (tumblr) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "tumblr.com") }

var tumblrTokenRe = regexp.MustCompile(`"API_TOKEN":\s*"(\w+)"`)

type tumblrMedia struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Type   string `json:"type"`
}

type tumblrBlock struct {
	Type     string          `json:"type"`
	Provider string          `json:"provider"`
	URL      string          `json:"url"`
	Title    string          `json:"title"`
	Artist   string          `json:"artist"`
	Media    json.RawMessage `json:"media"`
	Poster   []tumblrMedia   `json:"poster"`
}

// media is one object for video and audio blocks and a list of sizes for images
func (b tumblrBlock) files() []tumblrMedia {
	var list []tumblrMedia
	if json.Unmarshal(b.Media, &list) == nil {
		return list
	}
	var one tumblrMedia
	if json.Unmarshal(b.Media, &one) == nil && one.URL != "" {
		return []tumblrMedia{one}
	}
	return nil
}

func (s tumblr) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	blog, id := tumblrPost(u)
	if blog == "" {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single post can be downloaded."))
	}
	login, err := getBody(ctx, env, tumblrBase+"/login", navigate())
	if err != nil {
		return nil, err
	}
	tok := tumblrTokenRe.FindSubmatch(login.body)
	if tok == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("Tumblr did not hand out an access token."))
	}
	var resp struct {
		Response struct {
			Timeline struct {
				Elements []struct {
					Summary  string        `json:"summary"`
					BlogName string        `json:"blog_name"`
					PostURL  string        `json:"post_url"`
					Content  []tumblrBlock `json:"content"`
					Trail    []struct {
						Content []tumblrBlock `json:"content"`
					} `json:"trail"`
				} `json:"elements"`
			} `json:"timeline"`
		} `json:"response"`
	}
	err = getJSON(ctx, env, tumblrBase+"/api/v2/blog/"+url.PathEscape(blog)+"/posts/"+id+"/permalink",
		hdr("Authorization", "Bearer "+string(tok[1])), &resp)
	if err != nil {
		return nil, err
	}
	if len(resp.Response.Timeline.Elements) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post does not exist."))
	}
	el := resp.Response.Timeline.Elements[0]
	var blocks []tumblrBlock
	for _, t := range el.Trail {
		blocks = append(blocks, t.Content...)
	}
	blocks = append(blocks, el.Content...)

	src := media.Source{ID: s.ID(), Name: s.Name()}
	title := oneLine(el.Summary, 90)
	if title == "" {
		title = i18n.Tf("Post by %s", first(el.BlogName, blog))
	}
	out := &media.Media{Kind: plan.KindMedia, URL: first(el.PostURL, "https://"+blog+".tumblr.com/post/"+id),
		Source: src, Title: title, Author: first(el.BlogName, blog)}
	var vids, pics, songs int
	for _, b := range blocks {
		files := b.files()
		switch b.Type {
		case "video":
			vids++
			thumb := ""
			if len(b.Poster) > 0 {
				thumb = b.Poster[0].URL
			}
			if len(files) == 0 || (b.Provider != "" && b.Provider != "tumblr") {
				// a video hosted elsewhere, which its own service resolves
				if b.URL != "" {
					out.Items = append(out.Items, media.Item{Title: i18n.Tf("Video %d", vids), Thumb: thumb, Kind: "video", URL: b.URL})
				}
				continue
			}
			f := files[0]
			out.Items = append(out.Items, media.Item{Title: i18n.Tf("Video %d", vids), Thumb: thumb, Kind: "video", URL: f.URL})
			if len(out.Formats) == 0 {
				out.Formats = []media.Format{fileVideo("mp4", f.URL, extOf(f.URL, "mp4"), f.Height, 0, 0, 0, "h264,aac", nil)}
				out.Thumb = thumb
			}
		case "audio":
			songs++
			if len(files) == 0 || (b.Provider != "" && b.Provider != "tumblr") {
				if b.URL != "" {
					out.Items = append(out.Items, media.Item{Title: first(b.Title, i18n.Tf("Audio %d", songs)), Kind: "audio", URL: b.URL})
				}
				continue
			}
			f := files[0]
			ext := extOf(f.URL, "mp3")
			out.Items = append(out.Items, media.Item{Title: first(b.Title, i18n.Tf("Audio %d", songs)), Kind: "audio", URL: f.URL})
			if len(out.Formats) == 0 {
				out.Formats = []media.Format{fileAudio("audio", f.URL, ext, 0, 0, ext, nil)}
				if b.Title != "" {
					out.Title = strings.TrimSpace(strings.Join([]string{b.Artist, b.Title}, " - "))
					out.Title = strings.TrimPrefix(out.Title, "- ")
				}
			}
		case "image":
			if len(files) == 0 {
				continue
			}
			pics++
			// the first size is the original upload
			out.Items = append(out.Items, media.Item{Title: i18n.Tf("Picture %d", pics), Thumb: tumblrSmall(files), Kind: "photo", URL: files[0].URL})
		}
	}
	if len(out.Items) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post has no video or picture."))
	}
	if len(out.Items) == 1 {
		it := out.Items[0]
		if it.Kind == "photo" {
			return photoFile(src, it.URL, title, out.Author, nil), nil
		}
		if len(out.Formats) == 0 {
			return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.Tf("This post shares a video from elsewhere: %s", it.URL))
		}
		out.Items = nil
	} else {
		out.Note = i18n.Tf("%d items in this post", len(out.Items))
	}
	if out.Thumb == "" && len(out.Items) > 0 {
		out.Thumb = out.Items[0].Thumb
	}
	out.Default = best(out.Formats)
	return out, nil
}

func tumblrSmall(files []tumblrMedia) string {
	for _, f := range files {
		if f.Width > 0 && f.Width <= 640 {
			return f.URL
		}
	}
	return files[0].URL
}

// tumblrPost reads blog name and post id from the URL shapes Tumblr links use.
func tumblrPost(u *url.URL) (blog, id string) {
	p := pathParts(u)
	host := strings.ToLower(u.Hostname())
	isDigits := func(s string) bool { return s != "" && strings.Trim(s, "0123456789") == "" }
	switch {
	case host == "www.tumblr.com" || host == "tumblr.com":
		if len(p) >= 4 && p[0] == "blog" && p[1] == "view" && isDigits(p[3]) {
			return p[2], p[3]
		}
		if len(p) >= 2 && isDigits(p[1]) {
			return p[0], p[1]
		}
	case strings.HasSuffix(host, ".tumblr.com"):
		if len(p) >= 2 && (p[0] == "post" || p[0] == "image") && isDigits(p[1]) {
			return strings.TrimSuffix(host, ".tumblr.com"), p[1]
		}
	}
	return "", ""
}
