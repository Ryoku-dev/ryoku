package services

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var instagramBase = "https://www.instagram.com"

const (
	igAppID = "936619743392459"
	// the logged-out post query the web app runs
	igDocID = "27130156389949648"
)

type instagram struct{}

func init() { media.Register(instagram{}) }

func (instagram) ID() string   { return "instagram" }
func (instagram) Name() string { return "Instagram" }

func (instagram) Match(u *url.URL) bool {
	return media.HostIs(u.Hostname(), "instagram.com", "instagr.am")
}

var igPostRe = regexp.MustCompile(`^/(?:[^/]+/)?(?:p|reels?|tv)/([A-Za-z0-9_-]+)`)

type igMedia struct {
	Typename     string  `json:"__typename"`
	Code         string  `json:"code"`
	MediaType    int     `json:"media_type"`
	ProductType  string  `json:"product_type"`
	TakenAt      int64   `json:"taken_at"`
	DisplayURI   string  `json:"display_uri"`
	OriginalW    int     `json:"original_width"`
	OriginalH    int     `json:"original_height"`
	HasAudio     *bool   `json:"has_audio"`
	VideoDur     float64 `json:"video_duration"`
	DashManifest string  `json:"video_dash_manifest"`
	Caption      *struct {
		Text string `json:"text"`
	} `json:"caption"`
	User *struct {
		Username string `json:"username"`
		FullName string `json:"full_name"`
	} `json:"user"`
	ImageVersions struct {
		Candidates []struct {
			URL    string `json:"url"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"candidates"`
	} `json:"image_versions2"`
	VideoVersions []struct {
		Type   int    `json:"type"`
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"video_versions"`
	Carousel []igMedia `json:"carousel_media"`
}

func (s instagram) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	parts := pathParts(u)
	if len(parts) > 0 && parts[0] == "stories" {
		return nil, fmt.Errorf("%w: %s", media.ErrPrivate, i18n.T("Instagram stories need a signed-in account."))
	}
	m := igPostRe.FindStringSubmatch(u.Path)
	if m == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single post can be downloaded."))
	}
	code := m[1]
	pk, ok := igPK(code)
	if !ok {
		return nil, fmt.Errorf("%w: %s", media.ErrInvalid, i18n.T("This is not a link to a post."))
	}
	pick, _ := strconv.Atoi(u.Query().Get("img_index"))

	home, err := fetch(ctx, env, request{url: instagramBase + "/", header: navigate()})
	if err != nil {
		return nil, err
	}
	lsd := igLSD(home.body)
	csrf := ""
	for _, c := range home.cookies {
		if c.Name == "csrftoken" {
			csrf = c.Value
		}
	}
	form := url.Values{
		"lsd":                      {lsd},
		"fb_api_caller_class":      {"RelayModern"},
		"fb_api_req_friendly_name": {"PolarisLoggedOutDesktopWWWPostRootContentQuery"},
		"server_timestamps":        {"true"},
		"variables":                {`{"media_id":"` + pk + `"}`},
		"doc_id":                   {igDocID},
	}
	h := igHeaders()
	h.Set("Content-Type", "application/x-www-form-urlencoded")
	h.Set("X-FB-Friendly-Name", "PolarisLoggedOutDesktopWWWPostRootContentQuery")
	h.Set("X-FB-LSD", lsd)
	h.Set("X-Requested-With", "XMLHttpRequest")
	h.Set("Referer", instagramBase+"/p/"+code+"/")
	if csrf != "" {
		h.Set("X-CSRFToken", csrf)
	}
	if ck := cookieHeader(home.cookies); ck != "" {
		h.Set("Cookie", ck)
	}
	r, err := fetch(ctx, env, request{method: http.MethodPost, url: instagramBase + "/api/graphql", header: h, body: []byte(form.Encode())})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data struct {
			Media *struct {
				Product *igMedia `json:"if_not_gated_logged_out"`
			} `json:"xig_polaris_media"`
		} `json:"data"`
	}
	if err := decode(r.body, &resp); err != nil {
		return nil, fmt.Errorf("%w: %s", media.ErrRateLimited, i18n.T("Instagram did not answer the post query; try again later."))
	}
	if resp.Data.Media == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("This post does not exist."))
	}
	p := resp.Data.Media.Product
	if p == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrPrivate, i18n.T("This post is only visible to signed-in accounts."))
	}
	return s.build(p, code, pick)
}

func (s instagram) build(p *igMedia, code string, pick int) (*media.Media, error) {
	src := media.Source{ID: s.ID(), Name: s.Name()}
	author := ""
	if p.User != nil {
		author = first(p.User.FullName, p.User.Username)
	}
	title := ""
	if p.Caption != nil {
		title = oneLine(p.Caption.Text, 90)
	}
	if title == "" {
		title = i18n.Tf("Post by %s", first(author, src.Name))
	}
	page := instagramBase + "/p/" + code + "/"
	cdn := map[string]string{"Referer": instagramBase + "/"}

	nodes := []igMedia{*p}
	if len(p.Carousel) > 0 {
		nodes = p.Carousel
	}
	if pick > 0 && pick <= len(nodes) {
		nodes = nodes[pick-1 : pick]
	}
	if len(nodes) == 1 && len(nodes[0].VideoVersions) == 0 {
		return photoFile(src, igImage(&nodes[0]), title, author, cdn), nil
	}

	out := &media.Media{Kind: plan.KindMedia, URL: page, Source: src, Title: title, Author: author,
		Thumb: first(p.DisplayURI, igImage(p)), Headers: cdn}
	var vids, pics int
	for i := range nodes {
		n := &nodes[i]
		isVideo := len(n.VideoVersions) > 0
		if len(nodes) > 1 {
			item := media.Item{Thumb: first(n.DisplayURI, igImage(n)), Kind: "photo", URL: igImage(n)}
			if isVideo {
				vids++
				item.Kind, item.Title = "video", i18n.Tf("Video %d", vids)
				item.URL = page + "?img_index=" + strconv.Itoa(i+1)
			} else {
				pics++
				item.Title = i18n.Tf("Picture %d", pics)
			}
			out.Items = append(out.Items, item)
		}
		if isVideo && len(out.Formats) == 0 {
			out.Formats, out.Duration = igFormats(n)
			out.Thumb = first(n.DisplayURI, igImage(n), out.Thumb)
		}
	}
	if len(out.Items) > 1 {
		out.Note = i18n.Tf("%d items in this post", len(out.Items))
	}
	out.Default = best(out.Formats)
	return out, nil
}

func igImage(n *igMedia) string {
	best, area := "", -1
	for _, c := range n.ImageVersions.Candidates {
		if c.Width*c.Height > area {
			best, area = c.URL, c.Width*c.Height
		}
	}
	return first(best, n.DisplayURI)
}

// igFormats offers the progressive file the web player streams first, then the
// DASH renditions behind it (separate picture and sound, joined after).
func igFormats(n *igMedia) ([]media.Format, float64) {
	var fs []media.Format
	hasAudio := n.HasAudio == nil || *n.HasAudio
	seen := map[string]bool{}
	for _, v := range n.VideoVersions {
		key := v.URL
		if i := strings.IndexByte(key, '?'); i >= 0 {
			key = key[:i]
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		// the progressive file is often smaller than the upload, so no height is claimed for it
		f := fileVideo("mp4-"+strconv.Itoa(len(fs)), v.URL, "mp4", v.Height, 0, 0, 0, "h264,aac", nil)
		f.HasAudio = hasAudio
		fs = append(fs, f)
	}
	dur := n.VideoDur
	if n.DashManifest == "" {
		return fs, dur
	}
	mpd, err := parseMPD([]byte(n.DashManifest))
	if err != nil {
		return fs, dur
	}
	if dur == 0 {
		dur = mpd.duration
	}
	var audio *mpdRep
	for i := range mpd.audio {
		if audio == nil || mpd.audio[i].Bandwidth > audio.Bandwidth {
			audio = &mpd.audio[i]
		}
	}
	for _, v := range mpd.video {
		f := media.Format{
			ID: "dash-" + v.ID, Kind: "video", Label: heightLabel(v.Height, int(v.fps+0.5)), Ext: "mp4",
			Codec: v.Codecs, Height: v.Height, FPS: int(v.fps + 0.5), Bitrate: v.Bandwidth, HasVideo: true,
			Parts: []plan.Part{{Kind: plan.PartHTTP, URL: v.BaseURL, Ext: "mp4", Size: -1, Role: "video"}},
		}
		if audio != nil {
			f.HasAudio = true
			f.Codec += "," + audio.Codecs
			f.Bitrate += audio.Bandwidth
			f.Parts = append(f.Parts, plan.Part{Kind: plan.PartHTTP, URL: audio.BaseURL, Ext: "m4a", Size: -1, Role: "audio"})
		}
		fs = append(fs, f)
	}
	if audio != nil {
		fs = append(fs, fileAudio("dash-"+audio.ID, audio.BaseURL, "m4a", audio.Bandwidth, 0, audio.Codecs, nil))
	}
	return fs, dur
}

func igHeaders() http.Header {
	return hdr(
		"X-IG-App-ID", igAppID,
		"X-ASBD-ID", "359341",
		"X-IG-WWW-Claim", "0",
		"Origin", instagramBase,
		"Accept", "*/*",
		"Accept-Language", "en-US,en;q=0.5",
		"Sec-Fetch-Dest", "empty",
		"Sec-Fetch-Mode", "cors",
		"Sec-Fetch-Site", "same-origin",
	)
}

var igLSDRe = []*regexp.Regexp{
	regexp.MustCompile(`\["LSD",\[\],\{"token":"([^"]+)"`),
	regexp.MustCompile(`"lsd":"([^"]+)"`),
}

func igLSD(page []byte) string {
	if eq := scriptByID(page, "__eqmc"); eq != nil {
		var v struct {
			L string `json:"l"`
		}
		if json.Unmarshal(eq, &v) == nil && v.L != "" {
			return v.L
		}
	}
	for _, re := range igLSDRe {
		if m := re.FindSubmatch(page); m != nil {
			return string(m[1])
		}
	}
	return ""
}

// igPK turns a post shortcode into the numeric media id it encodes.
func igPK(code string) (string, bool) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	if len(code) > 28 {
		code = code[:len(code)-28]
	}
	n := new(big.Int)
	for _, c := range code {
		i := strings.IndexRune(alphabet, c)
		if i < 0 {
			return "", false
		}
		n.Lsh(n, 6)
		n.Or(n, big.NewInt(int64(i)))
	}
	return n.String(), code != ""
}

type mpdRep struct {
	ID        string  `xml:"id,attr"`
	Bandwidth int     `xml:"bandwidth,attr"`
	Width     int     `xml:"width,attr"`
	Height    int     `xml:"height,attr"`
	Codecs    string  `xml:"codecs,attr"`
	FrameRate string  `xml:"frameRate,attr"`
	MimeType  string  `xml:"mimeType,attr"`
	BaseURL   string  `xml:"BaseURL"`
	fps       float64 `xml:"-"`
}

type mpdInfo struct {
	video, audio []mpdRep
	duration     float64
}

// parseMPD reads a single-period manifest whose representations are whole
// files (BaseURL + SegmentBase), the shape Instagram and Facebook serve.
func parseMPD(b []byte) (*mpdInfo, error) {
	var doc struct {
		Duration string `xml:"mediaPresentationDuration,attr"`
		Periods  []struct {
			Sets []struct {
				ContentType string   `xml:"contentType,attr"`
				MimeType    string   `xml:"mimeType,attr"`
				Codecs      string   `xml:"codecs,attr"`
				Reps        []mpdRep `xml:"Representation"`
			} `xml:"AdaptationSet"`
		} `xml:"Period"`
	}
	if err := xml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	out := &mpdInfo{duration: isoDuration(doc.Duration)}
	for _, p := range doc.Periods {
		for _, set := range p.Sets {
			for _, r := range set.Reps {
				r.BaseURL = strings.TrimSpace(r.BaseURL)
				if r.BaseURL == "" {
					continue
				}
				if r.Codecs == "" {
					r.Codecs = set.Codecs
				}
				r.fps = frameRate(r.FrameRate)
				kind := first(set.ContentType, set.MimeType, r.MimeType)
				switch {
				case strings.HasPrefix(kind, "video"):
					out.video = append(out.video, r)
				case strings.HasPrefix(kind, "audio"):
					out.audio = append(out.audio, r)
				}
			}
		}
	}
	return out, nil
}

func frameRate(s string) float64 {
	if a, b, ok := strings.Cut(s, "/"); ok {
		n, _ := strconv.ParseFloat(a, 64)
		d, _ := strconv.ParseFloat(b, 64)
		if d > 0 {
			return n / d
		}
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// isoDuration reads the PT#H#M#S form manifests use.
func isoDuration(s string) float64 {
	s = strings.TrimPrefix(strings.TrimSpace(s), "P")
	_, t, ok := strings.Cut(s, "T")
	if !ok {
		return 0
	}
	d, err := time.ParseDuration(strings.ToLower(t))
	if err != nil {
		return 0
	}
	return d.Seconds()
}
