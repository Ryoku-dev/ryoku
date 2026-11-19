package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"ryoget/internal/hls"
	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var pinterestBase = "https://www.pinterest.com"

type pinterest struct{}

func init() { media.Register(pinterest{}) }

func (pinterest) ID() string   { return "pinterest" }
func (pinterest) Name() string { return "Pinterest" }

// pinterest.com plus its country domains (pinterest.ca, pinterest.co.uk, ...)
func (pinterest) Match(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	if media.HostIs(host, "pin.it") {
		return true
	}
	for _, label := range strings.Split(host, ".") {
		if label == "pinterest" {
			return true
		}
	}
	return false
}

var (
	pinIDRe   = regexp.MustCompile(`/pin/(?:[\w-]+--)?(\d+)`)
	pinTierRe = regexp.MustCompile(`_t\d+\.mp4$`)
)

type pinVideo struct {
	URL      string  `json:"url"`
	Height   int     `json:"height"`
	Duration float64 `json:"duration"`
}

func (s pinterest) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	if media.HostIs(u.Hostname(), "pin.it") {
		to, err := follow(ctx, env, u.String())
		if err != nil {
			return nil, err
		}
		u = to
	}
	m := pinIDRe.FindStringSubmatch(u.Path)
	if m == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single pin can be downloaded."))
	}
	id := m[1]
	opts, _ := json.Marshal(map[string]any{"options": map[string]string{"field_set_key": "unauth_react_main_pin", "id": id}})
	var resp struct {
		ResourceResponse struct {
			Data *struct {
				Title     string `json:"title"`
				GridTitle string `json:"grid_title"`
				SEOTitle  string `json:"seo_title"`
				Domain    string `json:"domain"`
				Images    map[string]struct {
					URL string `json:"url"`
				} `json:"images"`
				Attribution *struct {
					FullName string `json:"full_name"`
				} `json:"closeup_attribution"`
				Pinner *struct {
					FullName string `json:"full_name"`
				} `json:"pinner"`
				Videos *struct {
					List map[string]pinVideo `json:"video_list"`
				} `json:"videos"`
				Story *struct {
					Pages []struct {
						Blocks []struct {
							Video *struct {
								List map[string]pinVideo `json:"video_list"`
							} `json:"video"`
						} `json:"blocks"`
					} `json:"pages"`
				} `json:"story_pin_data"`
				Embed *struct {
					Src string `json:"src"`
				} `json:"embed"`
			} `json:"data"`
		} `json:"resource_response"`
	}
	q := url.Values{"data": {string(opts)}}
	if err := getJSON(ctx, env, pinterestBase+"/resource/PinResource/get/?"+q.Encode(),
		hdr("X-Pinterest-PWS-Handler", "www/[username].js"), &resp); err != nil {
		return nil, err
	}
	d := resp.ResourceResponse.Data
	if d == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This pin does not exist."))
	}
	src := media.Source{ID: s.ID(), Name: s.Name()}
	title := oneLine(first(d.Title, d.GridTitle, d.SEOTitle), 90)
	if title == "" {
		title = i18n.Tf("Pin %s", id)
	}
	author := ""
	if d.Attribution != nil {
		author = d.Attribution.FullName
	} else if d.Pinner != nil {
		author = d.Pinner.FullName
	}
	thumb := first(d.Images["736x"].URL, d.Images["orig"].URL)

	list := map[string]pinVideo{}
	if d.Videos != nil {
		list = d.Videos.List
	}
	if len(list) == 0 && d.Story != nil {
		for _, p := range d.Story.Pages {
			for _, b := range p.Blocks {
				if b.Video != nil && len(list) == 0 {
					list = b.Video.List
				}
			}
		}
	}
	if len(list) == 0 {
		if d.Embed != nil && d.Embed.Src != "" && !strings.EqualFold(d.Domain, "Uploaded by user") {
			return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.Tf("This pin plays a video from %s; download it from there.", d.Domain))
		}
		img := d.Images["orig"].URL
		if img == "" {
			return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This pin has no video or picture."))
		}
		return photoFile(src, img, title, author, nil), nil
	}

	out := &media.Media{Kind: plan.KindMedia, URL: pinterestBase + "/pin/" + id + "/", Source: src, Title: title, Author: author, Thumb: thumb}
	seen := map[string]bool{}
	master := ""
	keys := make([]string, 0, len(list))
	for k := range list {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		v := list[key]
		if v.URL == "" || seen[v.URL] {
			continue
		}
		seen[v.URL] = true
		out.Duration = v.Duration / 1000
		if strings.Contains(strings.ToLower(key), "hls") || extOf(v.URL, "") == "m3u8" {
			if master == "" || strings.Contains(key, "HLSV4") {
				master = v.URL
			}
			continue
		}
		if extOf(v.URL, "") != "mp4" {
			continue
		}
		// story pins add lower-bitrate copies (name_t1.mp4 ...) at the same size
		if pinTierRe.MatchString(v.URL) {
			continue
		}
		out.Formats = append(out.Formats, fileVideo(strings.ToLower(key), v.URL, "mp4", v.Height, 0, 0, 0, "h264,aac", nil))
	}
	if master != "" {
		if vs, err := hls.Variants(ctx, env, master, nil); err == nil {
			out.Formats = append(out.Formats, media.HLSFormats(vs, nil)...)
		}
	}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This pin has no video or picture."))
	}
	out.Default = pinDefault(out.Formats)
	return out, nil
}

// pinDefault prefers a whole mp4 file over a playlist at the same height.
func pinDefault(fs []media.Format) string {
	var files []media.Format
	for _, f := range fs {
		if f.Parts[0].Kind == plan.PartHTTP {
			files = append(files, f)
		}
	}
	if len(files) > 0 {
		b := best(files)
		if hb := best(fs); findHeight(fs, hb) > findHeight(fs, b) {
			return hb
		}
		return b
	}
	return best(fs)
}

func findHeight(fs []media.Format, id string) int {
	for _, f := range fs {
		if f.ID == id {
			return f.Height
		}
	}
	return 0
}
