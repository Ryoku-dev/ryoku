package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var tiktokBase = "https://www.tiktok.com"

type tiktok struct{}

func init() { media.Register(tiktok{}) }

func (tiktok) ID() string   { return "tiktok" }
func (tiktok) Name() string { return "TikTok" }

func (tiktok) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "tiktok.com") }

var ttPostRe = regexp.MustCompile(`/(?:video|photo|v|embed/v2|embed)/(\d{8,})`)

type ttItem struct {
	ID     string `json:"id"`
	Desc   string `json:"desc"`
	Author struct {
		Nickname string `json:"nickname"`
		UniqueID string `json:"uniqueId"`
	} `json:"author"`
	Video struct {
		Duration    float64 `json:"duration"`
		Cover       string  `json:"cover"`
		OriginCover string  `json:"originCover"`
		PlayAddr    string  `json:"playAddr"`
		Height      int     `json:"height"`
		Bitrate     int     `json:"bitrate"`
		CodecType   string  `json:"codecType"`
		BitrateInfo []struct {
			GearName  string `json:"GearName"`
			Bitrate   int    `json:"Bitrate"`
			CodecType string `json:"CodecType"`
			PlayAddr  struct {
				URLList  []string    `json:"UrlList"`
				Height   int         `json:"Height"`
				Width    int         `json:"Width"`
				DataSize json.Number `json:"DataSize"`
			} `json:"PlayAddr"`
		} `json:"bitrateInfo"`
	} `json:"video"`
	Music struct {
		Title    string  `json:"title"`
		PlayURL  string  `json:"playUrl"`
		Duration float64 `json:"duration"`
	} `json:"music"`
	ImagePost *struct {
		Images []struct {
			ImageURL struct {
				URLList []string `json:"urlList"`
			} `json:"imageURL"`
		} `json:"images"`
	} `json:"imagePost"`
}

func (s tiktok) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	m := ttPostRe.FindStringSubmatch(u.Path)
	if m == nil {
		// vm.tiktok.com/x, vt.tiktok.com/x and tiktok.com/t/x are short links
		to, err := follow(ctx, env, u.String(), "www.tiktok.com", "m.tiktok.com")
		if err != nil {
			return nil, err
		}
		if m = ttPostRe.FindStringSubmatch(to.Path); m == nil {
			return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single post can be downloaded."))
		}
	}
	id := m[1]
	r, err := fetch(ctx, env, request{url: tiktokBase + "/@i/video/" + id, header: navigate()})
	if err != nil {
		return nil, err
	}
	raw := scriptByID(r.body, "__UNIVERSAL_DATA_FOR_REHYDRATION__")
	if raw == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrRateLimited, i18n.T("TikTok did not send the post; try again later."))
	}
	var page struct {
		Scope struct {
			Detail struct {
				StatusCode int    `json:"statusCode"`
				StatusMsg  string `json:"statusMsg"`
				ItemInfo   *struct {
					Item ttItem `json:"itemStruct"`
				} `json:"itemInfo"`
			} `json:"webapp.video-detail"`
		} `json:"__DEFAULT_SCOPE__"`
	}
	if err := decode(raw, &page); err != nil {
		return nil, err
	}
	d := page.Scope.Detail
	if d.ItemInfo == nil || d.StatusCode != 0 {
		return nil, ttStatus(d.StatusCode, d.StatusMsg)
	}
	it := &d.ItemInfo.Item

	// the CDN only serves a page's own session
	cdn := map[string]string{"Referer": tiktokBase + "/"}
	if ck := cookieHeader(r.cookies); ck != "" {
		cdn["Cookie"] = ck
	}
	src := media.Source{ID: s.ID(), Name: s.Name()}
	author := first(it.Author.Nickname, it.Author.UniqueID)
	title := oneLine(it.Desc, 90)
	if title == "" {
		title = i18n.Tf("Post by %s", first(author, src.Name))
	}
	out := &media.Media{
		Kind: plan.KindMedia, URL: tiktokBase + "/@" + first(it.Author.UniqueID, "i") + "/video/" + id,
		Source: src, Title: title, Author: author, Thumb: first(it.Video.OriginCover, it.Video.Cover),
		Duration: it.Video.Duration, Headers: cdn,
	}

	if it.ImagePost != nil && len(it.ImagePost.Images) > 0 {
		for i, img := range it.ImagePost.Images {
			if len(img.ImageURL.URLList) == 0 {
				continue
			}
			out.Items = append(out.Items, media.Item{
				Title: i18n.Tf("Picture %d", i+1), Thumb: img.ImageURL.URLList[0], Kind: "photo", URL: img.ImageURL.URLList[0],
			})
		}
		if out.Thumb == "" && len(out.Items) > 0 {
			out.Thumb = out.Items[0].Thumb
		}
		if it.Music.PlayURL != "" {
			out.Formats = []media.Format{fileAudio("music", it.Music.PlayURL, "mp3", 0, 0, "mp3", nil)}
			out.Duration = it.Music.Duration
		}
		out.Note = i18n.Tf("%d items in this post", len(out.Items))
		out.Default = best(out.Formats)
		return out, nil
	}

	seen := map[string]bool{}
	for _, b := range it.Video.BitrateInfo {
		if len(b.PlayAddr.URLList) == 0 {
			continue
		}
		u := b.PlayAddr.URLList[0]
		if seen[u] {
			continue
		}
		seen[u] = true
		size, _ := b.PlayAddr.DataSize.Int64()
		codec := ttCodec(b.CodecType)
		f := fileVideo(first(b.GearName, "v"+strconv.Itoa(len(out.Formats))), u, "mp4", b.PlayAddr.Height, 0, b.Bitrate, size, codec+",aac", nil)
		f.Label = strings.TrimSpace(f.Label + " " + strings.ToUpper(codec))
		out.Formats = append(out.Formats, f)
	}
	if len(out.Formats) == 0 && it.Video.PlayAddr != "" {
		out.Formats = append(out.Formats, fileVideo("play", it.Video.PlayAddr, "mp4", it.Video.Height, 0, it.Video.Bitrate, 0, ttCodec(it.Video.CodecType)+",aac", nil))
	}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post has no video or picture."))
	}
	out.Default = ttDefault(out.Formats)
	return out, nil
}

// ttDefault prefers h264 at the best height: TikTok's h265 gears are smaller
// but do not play everywhere.
func ttDefault(fs []media.Format) string {
	var h264 []media.Format
	for _, f := range fs {
		if strings.HasPrefix(f.Codec, "h264") {
			h264 = append(h264, f)
		}
	}
	if len(h264) > 0 {
		return best(h264)
	}
	return best(fs)
}

func ttCodec(c string) string {
	switch strings.ToLower(c) {
	case "h265", "bytevc1", "hevc":
		return "h265"
	}
	return "h264"
}

func ttStatus(code int, msg string) error {
	switch {
	case code == 10216 || code == 10222 || strings.Contains(msg, "private") || strings.Contains(msg, "self_see"):
		return fmt.Errorf("%w: %s", media.ErrPrivate, i18n.T("This post is private."))
	case code == 10204 || strings.Contains(msg, "deleted") || strings.Contains(msg, "not_exist"):
		return fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post was removed."))
	}
	return fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.Tf("TikTok says the post is unavailable (%d).", code))
}
