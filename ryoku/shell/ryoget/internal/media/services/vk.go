package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var vkBase = "https://vk.com"

type vk struct{}

func init() { media.Register(vk{}) }

func (vk) ID() string   { return "vk" }
func (vk) Name() string { return "VK" }

func (vk) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "vk.com", "vk.ru", "vkvideo.ru") }

var vkIDRe = regexp.MustCompile(`(?:video|clip)(-?\d+_\d+)`)

func (s vk) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	id := ""
	if m := vkIDRe.FindStringSubmatch(u.Query().Get("z")); m != nil {
		id = m[1]
	} else if m := vkIDRe.FindStringSubmatch(u.Path); m != nil {
		id = m[1]
	} else if q := u.Query(); strings.HasSuffix(u.Path, "video_ext.php") && q.Get("oid") != "" && q.Get("id") != "" {
		id = q.Get("oid") + "_" + q.Get("id")
	}
	if id == "" {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	form := url.Values{"act": {"show"}, "video": {id}, "al": {"1"}}
	if list := u.Query().Get("list"); list != "" {
		form.Set("list", list)
	}
	r, err := fetch(ctx, env, request{method: http.MethodPost, url: vkBase + "/al_video.php",
		header: hdr("Content-Type", "application/x-www-form-urlencoded", "X-Requested-With", "XMLHttpRequest", "Referer", vkBase+"/al_video.php"),
		body:   []byte(form.Encode())})
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(r.final.Path, "/challenge.html") {
		return nil, fmt.Errorf("%w: %s", media.ErrRateLimited, i18n.T("VK asked for a browser check; try again later."))
	}
	body := r.body
	if !utf8.Valid(body) || strings.Contains(strings.ToLower(r.header.Get("Content-Type")), "1251") {
		body = cp1251(body)
	}
	var resp struct {
		Payload []json.RawMessage `json:"payload"`
	}
	if err := decode(body, &resp); err != nil || len(resp.Payload) < 2 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("VK did not send the video."))
	}
	code := strings.Trim(string(resp.Payload[0]), `"`)
	var parts []json.RawMessage
	_ = json.Unmarshal(resp.Payload[1], &parts)
	switch code {
	case "0":
	case "3":
		return nil, fmt.Errorf("%w: %s", media.ErrPrivate, i18n.T("This video needs a signed-in account."))
	default:
		msg := ""
		if len(parts) > 0 {
			_ = json.Unmarshal(parts[0], &msg)
		}
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, first(htmlText(msg), i18n.T("This video is unavailable.")))
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("VK did not send the video."))
	}
	var opts struct {
		Player *struct {
			Params []map[string]json.RawMessage `json:"params"`
		} `json:"player"`
	}
	if err := json.Unmarshal(parts[len(parts)-1], &opts); err != nil || opts.Player == nil || len(opts.Player.Params) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This video plays from another site or is unavailable."))
	}
	p := opts.Player.Params[0]
	str := func(k string) string {
		var v string
		_ = json.Unmarshal(p[k], &v)
		return v
	}
	out := &media.Media{Kind: plan.KindMedia, URL: vkBase + "/video" + id, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: first(htmlText(str("md_title")), id), Author: htmlText(str("md_author")), Thumb: str("jpg"),
		Headers: map[string]string{"User-Agent": browserUA, "Referer": vkBase + "/"}}
	var dur float64
	if json.Unmarshal(p["duration"], &dur) == nil {
		out.Duration = dur
	}
	for _, h := range []int{2160, 1440, 1080, 720, 480, 360, 240, 144} {
		if v := str("url" + strconv.Itoa(h)); v != "" {
			out.Formats = append(out.Formats, fileVideo("mp4-"+strconv.Itoa(h), v, "mp4", h, 0, 0, 0, "h264,aac", nil))
		}
	}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This video has no stream VK will share."))
	}
	out.Default = best(out.Formats)
	return out, nil
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

func htmlText(s string) string { return strings.TrimSpace(unescapeHTML(tagRe.ReplaceAllString(s, ""))) }

// cp1251Upper maps bytes 0x80-0xBF of Windows-1251; 0xC0-0xFF are А-я in order.
var cp1251Upper = [64]rune{
	'Ђ', 'Ѓ', '‚', 'ѓ', '„', '…', '†', '‡', '€', '‰', 'Љ', '‹', 'Њ', 'Ќ', 'Ћ', 'Џ',
	'ђ', '‘', '’', '“', '”', '•', '–', '—', '\uFFFD', '™', 'љ', '›', 'њ', 'ќ', 'ћ', 'џ',
	'\u00A0', 'Ў', 'ў', 'Ј', '¤', 'Ґ', '¦', '§', 'Ё', '©', 'Є', '«', '¬', '\u00AD', '®', 'Ї',
	'°', '±', 'І', 'і', 'ґ', 'µ', '¶', '·', 'ё', '№', 'є', '»', 'ј', 'Ѕ', 'ѕ', 'ї',
}

func cp1251(b []byte) []byte {
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		switch {
		case c < 0x80:
			out = append(out, c)
		case c < 0xC0:
			out = utf8.AppendRune(out, cp1251Upper[c-0x80])
		default:
			out = utf8.AppendRune(out, rune(0x410+int(c)-0xC0))
		}
	}
	return out
}
