package services

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var (
	bilibiliBase = "https://www.bilibili.com"
	bilibiliAPI  = "https://api.bilibili.com"
)

type bilibili struct{}

func init() { media.Register(bilibili{}) }

func (bilibili) ID() string   { return "bilibili" }
func (bilibili) Name() string { return "Bilibili" }

func (bilibili) Match(u *url.URL) bool {
	host := u.Hostname()
	return (media.HostIs(host, "bilibili.com") && !media.HostIs(host, "live.bilibili.com")) || media.HostIs(host, "b23.tv")
}

var biliIDRe = regexp.MustCompile(`/video/((?:BV|bv)[0-9A-Za-z]{10}|av\d+)`)

type biliStream struct {
	ID        int      `json:"id"`
	BaseURL   string   `json:"baseUrl"`
	BaseURL2  string   `json:"base_url"`
	Backup    []string `json:"backupUrl"`
	Bandwidth int      `json:"bandwidth"`
	Codecs    string   `json:"codecs"`
	Height    int      `json:"height"`
	FrameRate string   `json:"frameRate"`
}

func (b biliStream) url() string { return first(b.BaseURL, b.BaseURL2) }

func (s bilibili) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	if media.HostIs(u.Hostname(), "b23.tv") {
		to, err := follow(ctx, env, u.String(), "bilibili.com")
		if err != nil {
			return nil, err
		}
		u = to
	}
	m := biliIDRe.FindStringSubmatch(u.Path)
	if m == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	id := m[1]
	page, _ := strconv.Atoi(u.Query().Get("p"))
	if page < 1 {
		page = 1
	}
	pageURL := bilibiliBase + "/video/" + id + "/"
	if page > 1 {
		pageURL += "?p=" + strconv.Itoa(page)
	}
	r, err := fetch(ctx, env, request{url: pageURL, header: navigate()})
	if err != nil {
		return nil, err
	}
	var state struct {
		VideoData *struct {
			BVID     string  `json:"bvid"`
			CID      int64   `json:"cid"`
			Title    string  `json:"title"`
			Pic      string  `json:"pic"`
			Duration float64 `json:"duration"`
			Owner    struct {
				Name string `json:"name"`
			} `json:"owner"`
			Pages []struct {
				CID      int64   `json:"cid"`
				Page     int     `json:"page"`
				Part     string  `json:"part"`
				Duration float64 `json:"duration"`
				Pic      string  `json:"first_frame"`
			} `json:"pages"`
		} `json:"videoData"`
	}
	if raw := scriptJSON(r.body, "window.__INITIAL_STATE__="); raw != nil {
		_ = decode(raw, &state)
	}
	if state.VideoData == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This video is unavailable or needs a signed-in account."))
	}
	vd := state.VideoData
	cid := vd.CID
	for _, p := range vd.Pages {
		if p.Page == page {
			cid = p.CID
		}
	}
	var info struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Timelength float64 `json:"timelength"`
			Dash       *struct {
				Video []biliStream `json:"video"`
				Audio []biliStream `json:"audio"`
			} `json:"dash"`
			Durl []struct {
				URL  string `json:"url"`
				Size int64  `json:"size"`
			} `json:"durl"`
		} `json:"data"`
	}
	// fnval 4048 asks for every DASH rendition the anonymous player may have
	q := url.Values{"bvid": {first(vd.BVID, id)}, "cid": {strconv.FormatInt(cid, 10)}, "fnval": {"4048"}, "fourk": {"1"}, "qn": {"127"}}
	if err := getJSON(ctx, env, bilibiliAPI+"/x/player/playurl?"+q.Encode(), hdr("Referer", bilibiliBase+"/"), &info); err != nil {
		return nil, err
	}
	switch {
	case info.Code == -404 || info.Code == 62002 || info.Code == 62004:
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, first(info.Message, i18n.T("This video does not exist.")))
	case info.Code == -403 || info.Code == -10403 || info.Code == 87008:
		return nil, fmt.Errorf("%w: %s", media.ErrPrivate, first(info.Message, i18n.T("This video is unavailable or needs a signed-in account.")))
	case info.Code == -412 || info.Code == -509 || info.Code == -799:
		return nil, fmt.Errorf("%w: %s", media.ErrRateLimited, first(info.Message, i18n.T("Bilibili is limiting requests; try again later.")))
	case info.Code != 0:
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, info.Message)
	}
	cdn := map[string]string{"Referer": bilibiliBase + "/"}
	out := &media.Media{Kind: plan.KindMedia, URL: pageURL, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: vd.Title, Author: vd.Owner.Name, Thumb: biliHTTPS(vd.Pic), Duration: vd.Duration, Headers: cdn}
	if len(vd.Pages) > 1 {
		for _, p := range vd.Pages {
			out.Items = append(out.Items, media.Item{Title: first(p.Part, i18n.Tf("Part %d", p.Page)), Thumb: biliHTTPS(p.Pic),
				Kind: "video", URL: bilibiliBase + "/video/" + id + "/?p=" + strconv.Itoa(p.Page)})
			if p.Page == page {
				out.Title = strings.TrimSpace(vd.Title + " - " + p.Part)
				out.Duration = p.Duration
			}
		}
		out.Note = i18n.Tf("%d parts in this video", len(vd.Pages))
	}
	if info.Data.Timelength > 0 {
		out.Duration = info.Data.Timelength / 1000
	}

	if d := info.Data.Dash; d != nil && len(d.Video) > 0 {
		var audio *biliStream
		for i := range d.Audio {
			if audio == nil || d.Audio[i].Bandwidth > audio.Bandwidth {
				audio = &d.Audio[i]
			}
		}
		for _, v := range d.Video {
			fps := int(frameRate(v.FrameRate) + 0.5)
			f := media.Format{
				ID: "dash-" + strconv.Itoa(v.ID) + "-" + biliCodec(v.Codecs), Kind: "video",
				Label: heightLabel(v.Height, fps) + " " + strings.ToUpper(biliCodec(v.Codecs)), Ext: "mp4",
				Codec: v.Codecs, Height: v.Height, FPS: fps, Bitrate: v.Bandwidth, HasVideo: true,
				Parts: []plan.Part{{Kind: plan.PartHTTP, URL: v.url(), Ext: "m4v", Size: -1, Role: "video"}},
			}
			if audio != nil {
				f.HasAudio = true
				f.Codec += "," + audio.Codecs
				f.Bitrate += audio.Bandwidth
				f.Parts = append(f.Parts, plan.Part{Kind: plan.PartHTTP, URL: audio.url(), Ext: "m4a", Size: -1, Role: "audio"})
			}
			out.Formats = append(out.Formats, f)
		}
		if audio != nil {
			out.Formats = append(out.Formats, fileAudio("audio-"+strconv.Itoa(audio.ID), audio.url(), "m4a", audio.Bandwidth, 0, audio.Codecs, nil))
		}
	} else if len(info.Data.Durl) == 1 {
		out.Formats = []media.Format{fileVideo("durl", info.Data.Durl[0].URL, "flv", 0, 0, 0, info.Data.Durl[0].Size, "", nil)}
	}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This video has no stream Bilibili will share."))
	}
	out.Default = biliDefault(out.Formats)
	return out, nil
}

// biliDefault takes h264 at the best height: the av1 and hevc renditions are
// smaller but not every player decodes them.
func biliDefault(fs []media.Format) string {
	var avc []media.Format
	for _, f := range fs {
		if strings.HasSuffix(f.ID, "-avc") {
			avc = append(avc, f)
		}
	}
	if len(avc) > 0 {
		return best(avc)
	}
	return best(fs)
}

func biliCodec(c string) string {
	switch {
	case strings.HasPrefix(c, "avc"):
		return "avc"
	case strings.HasPrefix(c, "hev"), strings.HasPrefix(c, "hvc"):
		return "hevc"
	case strings.HasPrefix(c, "av01"):
		return "av1"
	}
	return "v"
}

func biliHTTPS(u string) string {
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	return strings.Replace(u, "http://", "https://", 1)
}
