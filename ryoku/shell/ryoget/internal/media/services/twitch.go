package services

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"ryoget/internal/hls"
	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var (
	twitchGQL   = "https://gql.twitch.tv/gql"
	twitchUsher = "https://usher.ttvnw.net"
)

// the public Client-ID the twitch.tv web player sends
const twitchClientID = "kimne78kx3ncx6brgo4mv6wki5h1ko"

type twitch struct{}

func init() { media.Register(twitch{}) }

func (twitch) ID() string   { return "twitch" }
func (twitch) Name() string { return "Twitch" }

func (twitch) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "twitch.tv") }

var (
	twitchVODRe = regexp.MustCompile(`^/(?:videos|[^/]+/v(?:ideo)?)/(\d+)`)
)

type twitchToken struct {
	Value     string `json:"value"`
	Signature string `json:"signature"`
}

func (s twitch) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	q := u.Query()
	if v := strings.TrimPrefix(q.Get("video"), "v"); v != "" {
		return s.vod(ctx, env, v)
	}
	if m := twitchVODRe.FindStringSubmatch(u.Path); m != nil {
		return s.vod(ctx, env, m[1])
	}
	slug := q.Get("clip")
	parts := pathParts(u)
	for i, p := range parts {
		if p == "clip" && i+1 < len(parts) {
			slug = parts[i+1]
		}
	}
	if slug == "" && media.HostIs(u.Hostname(), "clips.twitch.tv") && len(parts) == 1 {
		slug = parts[0]
	}
	if slug == "" || slug == "embed" {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only clips and past broadcasts can be downloaded; live streams cannot."))
	}
	return s.clip(ctx, env, slug)
}

func (twitch) gql(ctx context.Context, env *plan.Env, query string, out any) error {
	return postJSON(ctx, env, twitchGQL, hdr("Client-ID", twitchClientID), map[string]string{"query": query}, out)
}

func (s twitch) clip(ctx context.Context, env *plan.Env, slug string) (*media.Media, error) {
	var resp struct {
		Data struct {
			Clip *struct {
				Title           string  `json:"title"`
				DurationSeconds float64 `json:"durationSeconds"`
				ThumbnailURL    string  `json:"thumbnailURL"`
				Broadcaster     *struct {
					DisplayName string `json:"displayName"`
				} `json:"broadcaster"`
				VideoQualities []struct {
					Quality   string  `json:"quality"`
					FrameRate float64 `json:"frameRate"`
					SourceURL string  `json:"sourceURL"`
				} `json:"videoQualities"`
				Token twitchToken `json:"playbackAccessToken"`
			} `json:"clip"`
		} `json:"data"`
	}
	query := `{clip(slug:` + strconv.Quote(slug) + `){title durationSeconds thumbnailURL broadcaster{displayName} ` +
		`videoQualities{quality frameRate sourceURL} ` +
		`playbackAccessToken(params:{platform:"web",playerBackend:"mediaplayer",playerType:"site"}){value signature}}}`
	if err := s.gql(ctx, env, query, &resp); err != nil {
		return nil, err
	}
	c := resp.Data.Clip
	if c == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This clip does not exist."))
	}
	out := &media.Media{Kind: plan.KindMedia, URL: "https://clips.twitch.tv/" + slug, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(c.Title, slug), Thumb: c.ThumbnailURL, Duration: c.DurationSeconds}
	if c.Broadcaster != nil {
		out.Author = c.Broadcaster.DisplayName
	}
	sign := url.Values{"sig": {c.Token.Signature}, "token": {c.Token.Value}}.Encode()
	for _, v := range c.VideoQualities {
		if v.SourceURL == "" {
			continue
		}
		h, _ := strconv.Atoi(v.Quality)
		out.Formats = append(out.Formats, fileVideo("clip-"+v.Quality, v.SourceURL+"?"+sign, "mp4", h, int(v.FrameRate+0.5), 0, 0, "h264,aac", nil))
	}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This clip has no playable video."))
	}
	out.Default = best(out.Formats)
	return out, nil
}

func (s twitch) vod(ctx context.Context, env *plan.Env, id string) (*media.Media, error) {
	var resp struct {
		Data struct {
			Video *struct {
				Title         string  `json:"title"`
				LengthSeconds float64 `json:"lengthSeconds"`
				Owner         *struct {
					DisplayName string `json:"displayName"`
				} `json:"owner"`
				Thumb string `json:"previewThumbnailURL"`
			} `json:"video"`
			Token *twitchToken `json:"videoPlaybackAccessToken"`
		} `json:"data"`
	}
	query := `{video(id:` + strconv.Quote(id) + `){title lengthSeconds owner{displayName} previewThumbnailURL(width:640,height:360)} ` +
		`videoPlaybackAccessToken(id:` + strconv.Quote(id) + `,params:{platform:"web",playerBackend:"mediaplayer",playerType:"site"}){value signature}}`
	if err := s.gql(ctx, env, query, &resp); err != nil {
		return nil, err
	}
	v, tok := resp.Data.Video, resp.Data.Token
	if v == nil || tok == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This broadcast does not exist."))
	}
	if strings.Contains(tok.Value, `"forbidden":true`) {
		return nil, fmt.Errorf("%w: %s", media.ErrPrivate, i18n.T("This broadcast is for subscribers only."))
	}
	q := url.Values{
		"sig": {tok.Signature}, "token": {tok.Value}, "allow_source": {"true"}, "allow_audio_only": {"true"},
		"player": {"twitchweb"}, "playlist_include_framerate": {"true"},
	}
	vs, err := hls.Variants(ctx, env, twitchUsher+"/vod/"+id+".m3u8?"+q.Encode(), nil)
	if err != nil {
		if strings.Contains(err.Error(), "403") {
			return nil, fmt.Errorf("%w: %s", media.ErrPrivate, i18n.T("This broadcast is for subscribers only."))
		}
		return nil, fmt.Errorf("%w: %v", media.ErrUnavailable, err)
	}
	out := &media.Media{Kind: plan.KindMedia, URL: "https://www.twitch.tv/videos/" + id, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(v.Title, id), Thumb: v.Thumb, Duration: v.LengthSeconds, Formats: media.HLSFormats(vs, nil)}
	if v.Owner != nil {
		out.Author = v.Owner.DisplayName
	}
	out.Default = best(out.Formats)
	return out, nil
}
