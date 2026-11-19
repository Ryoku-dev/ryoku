package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"

	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var okBase = "https://ok.ru"

type okru struct{}

func init() { media.Register(okru{}) }

func (okru) ID() string   { return "okru" }
func (okru) Name() string { return "OK.ru" }

func (okru) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "ok.ru", "odnoklassniki.ru") }

var (
	okIDRe      = regexp.MustCompile(`^/(?:video|videoembed|live)/(\d+(?:-\d+)?)`)
	okOptionsRe = regexp.MustCompile(`data-options="([^"]*?metadata[^"]*)"`)
)

// rendition names OK uses, by picture height
var okHeights = map[string]int{"mobile": 144, "lowest": 240, "low": 360, "sd": 480, "hd": 720, "full": 1080, "quad": 1440, "ultra": 2160}

func (s okru) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	m := okIDRe.FindStringSubmatch(u.Path)
	if m == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	id := m[1]
	r, err := fetch(ctx, env, request{url: okBase + "/videoembed/" + id, header: navigate()})
	if err != nil {
		return nil, err
	}
	om := okOptionsRe.FindSubmatch(r.body)
	if om == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This video was removed or is private."))
	}
	var opts struct {
		Flashvars struct {
			Metadata json.RawMessage `json:"metadata"`
		} `json:"flashvars"`
	}
	if err := decode([]byte(unescapeHTML(string(om[1]))), &opts); err != nil {
		return nil, err
	}
	raw := opts.Flashvars.Metadata
	var s2 string
	if json.Unmarshal(raw, &s2) == nil {
		raw = []byte(s2)
	}
	var md struct {
		Provider string `json:"provider"`
		Movie    struct {
			Title    string      `json:"title"`
			Poster   string      `json:"poster"`
			Duration json.Number `json:"duration"`
		} `json:"movie"`
		Author *struct {
			Name string `json:"name"`
		} `json:"author"`
		Videos []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"videos"`
	}
	if err := decode(raw, &md); err != nil {
		return nil, err
	}
	if md.Provider != "" && md.Provider != "UPLOADED_ODKL" && md.Provider != "LIVE_TV_APP" {
		ext := ""
		if len(md.Videos) > 0 {
			ext = md.Videos[0].URL
		}
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.Tf("This video plays from another site: %s", ext))
	}
	// the CDN refuses requests without a browser user agent
	out := &media.Media{Kind: plan.KindMedia, URL: okBase + "/video/" + id, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(md.Movie.Title, id), Thumb: md.Movie.Poster, Headers: map[string]string{"User-Agent": browserUA, "Referer": okBase + "/"}}
	if md.Author != nil {
		out.Author = md.Author.Name
	}
	if d, err := md.Movie.Duration.Float64(); err == nil {
		out.Duration = d
	}
	for _, v := range md.Videos {
		h, ok := okHeights[v.Name]
		if !ok || v.URL == "" {
			continue
		}
		out.Formats = append(out.Formats, fileVideo(v.Name, v.URL, "mp4", h, 0, 0, 0, "h264,aac", nil))
	}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This video has no stream OK.ru will share."))
	}
	out.Default = best(out.Formats)
	return out, nil
}
