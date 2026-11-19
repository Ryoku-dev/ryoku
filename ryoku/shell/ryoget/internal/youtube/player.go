package youtube

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"ryoget/internal/media"
	"ryoget/internal/plan"

	i18n "ryoku-i18n"
)

var (
	// ErrBotGate is YouTube asking the anonymous visitor to prove it is human.
	ErrBotGate = errors.New("youtube: bot check")
	// ErrGone means the video does not exist or was removed; nothing else can
	// fetch it either.
	ErrGone = errors.New("youtube: video gone")
	// ErrNoFormats means the player answered without any plain stream URL.
	ErrNoFormats = errors.New("youtube: no direct formats")
)

// PlayabilityError carries YouTube's own status and reason.
type PlayabilityError struct {
	Status string
	Reason string
	kinds  []error
}

func (e *PlayabilityError) Error() string {
	if e.Reason != "" {
		return e.Reason
	}
	return "YouTube: " + e.Status
}

func (e *PlayabilityError) Unwrap() []error { return e.kinds }

type textRuns struct {
	SimpleText string `json:"simpleText"`
	Runs       []struct {
		Text string `json:"text"`
	} `json:"runs"`
}

func (t textRuns) String() string {
	if t.SimpleText != "" {
		return t.SimpleText
	}
	var b strings.Builder
	for _, r := range t.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

type thumbnails struct {
	Thumbnails []struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"thumbnails"`
}

func (t thumbnails) best() string {
	best, w := "", -1
	for _, th := range t.Thumbnails {
		if th.Width > w {
			best, w = th.URL, th.Width
		}
	}
	if strings.HasPrefix(best, "//") {
		best = "https:" + best
	}
	return best
}

type streamFormat struct {
	Itag             int    `json:"itag"`
	URL              string `json:"url"`
	SignatureCipher  string `json:"signatureCipher"`
	MimeType         string `json:"mimeType"`
	Bitrate          int    `json:"bitrate"`
	AverageBitrate   int    `json:"averageBitrate"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	ContentLength    string `json:"contentLength"`
	QualityLabel     string `json:"qualityLabel"`
	FPS              int    `json:"fps"`
	AudioQuality     string `json:"audioQuality"`
	ApproxDurationMs string `json:"approxDurationMs"`
	IsDrc            bool   `json:"isDrc"`
	Type             string `json:"type"`
	TargetDuration   int    `json:"targetDurationSec"`
	DRMFamilies      []any  `json:"drmFamilies"`
	AudioTrack       *struct {
		DisplayName    string `json:"displayName"`
		ID             string `json:"id"`
		AudioIsDefault bool   `json:"audioIsDefault"`
	} `json:"audioTrack"`
}

// Player is the part of a player response we use.
type Player struct {
	PlayabilityStatus struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"playabilityStatus"`
	StreamingData struct {
		Formats         []streamFormat `json:"formats"`
		AdaptiveFormats []streamFormat `json:"adaptiveFormats"`
		HLSManifestURL  string         `json:"hlsManifestUrl"`
	} `json:"streamingData"`
	VideoDetails struct {
		VideoID       string     `json:"videoId"`
		Title         string     `json:"title"`
		LengthSeconds string     `json:"lengthSeconds"`
		ChannelID     string     `json:"channelId"`
		Author        string     `json:"author"`
		Thumbnail     thumbnails `json:"thumbnail"`
		IsLive        bool       `json:"isLive"`
		IsPrivate     bool       `json:"isPrivate"`
	} `json:"videoDetails"`
}

// Player asks the visionOS client for id. A bot gate refreshes the visitor
// identity and tries once more.
func (c *Client) Player(ctx context.Context, id string) (*Player, error) {
	var p *Player
	for attempt := range 2 {
		p = &Player{}
		body := map[string]any{
			"videoId":        id,
			"contentCheckOk": true,
			"racyCheckOk":    true,
			"playbackContext": map[string]any{
				"contentPlaybackContext": map[string]any{"html5Preference": "HTML5_PREF_WANTS"},
			},
		}
		if err := c.call(ctx, visionOS, "player", body, "", p); err != nil {
			return nil, classifyCall(err)
		}
		err := p.check()
		if err == nil {
			return p, nil
		}
		if attempt == 0 && errors.Is(err, ErrBotGate) && c.RefreshVisitor(ctx) {
			continue
		}
		return p, err
	}
	return p, nil
}

func classifyCall(err error) error {
	var ae apiError
	if errors.As(err, &ae) {
		switch {
		case ae.status == http.StatusTooManyRequests:
			return fmt.Errorf("%w: %v", media.ErrRateLimited, err)
		case ae.status == http.StatusNotFound:
			return fmt.Errorf("%w: %v", media.ErrUnavailable, err)
		}
		return fmt.Errorf("%w: %v", media.ErrUnavailable, err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %v", media.ErrNetwork, err)
}

func (p *Player) check() error {
	st := p.PlayabilityStatus
	if st.Status == "OK" {
		return nil
	}
	reason := strings.TrimSpace(st.Reason)
	low := strings.ToLower(reason)
	e := &PlayabilityError{Status: st.Status, Reason: reason}
	switch {
	case strings.Contains(low, "not a bot"):
		e.kinds = []error{media.ErrRateLimited, ErrBotGate}
	case st.Status == "LOGIN_REQUIRED" && strings.Contains(low, "private"), p.VideoDetails.IsPrivate:
		e.kinds = []error{media.ErrPrivate}
	case st.Status == "ERROR":
		e.kinds = []error{media.ErrUnavailable, ErrGone}
	default:
		// age gates, made-for-kids, region and premiere states: another client
		// (yt-dlp's) may still play them
		e.kinds = []error{media.ErrUnavailable}
	}
	return e
}

// Media resolves id to its metadata and directly fetchable formats.
func (c *Client) Media(ctx context.Context, id string) (*media.Media, error) {
	p, err := c.Player(ctx, id)
	if err != nil {
		return nil, err
	}
	return p.Media()
}

// Media converts a playable response. Formats that need the JS player, a PO
// token or DRM are left out; none left is ErrNoFormats.
func (p *Player) Media() (*media.Media, error) {
	if err := p.check(); err != nil {
		return nil, err
	}
	d := p.VideoDetails
	if d.IsLive {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Live streams cannot be downloaded while they are live."))
	}
	m := &media.Media{
		Kind:     plan.KindMedia,
		URL:      WatchURL(d.VideoID),
		Source:   media.Source{ID: "youtube", Name: "YouTube"},
		Title:    d.Title,
		Author:   strings.TrimSuffix(d.Author, " - Topic"),
		Thumb:    d.Thumbnail.best(),
		Duration: atof(d.LengthSeconds),
	}
	all := append(append([]streamFormat(nil), p.StreamingData.Formats...), p.StreamingData.AdaptiveFormats...)
	defaultTrack := defaultAudioTrack(all)
	seen := map[int]bool{}
	for _, f := range all {
		if f.URL == "" || f.IsDrc || len(f.DRMFamilies) > 0 || f.Type == "FORMAT_STREAM_TYPE_OTF" || f.TargetDuration > 0 || seen[f.Itag] {
			continue
		}
		if f.AudioTrack != nil && defaultTrack != "" && f.AudioTrack.ID != defaultTrack {
			continue
		}
		mf, ok := toFormat(f)
		if !ok {
			continue
		}
		seen[f.Itag] = true
		m.Formats = append(m.Formats, mf)
	}
	if len(m.Formats) == 0 {
		return nil, fmt.Errorf("%w: %w", media.ErrUnavailable, ErrNoFormats)
	}
	sort.SliceStable(m.Formats, func(i, j int) bool {
		a, b := m.Formats[i], m.Formats[j]
		if a.HasVideo != b.HasVideo {
			return a.HasVideo
		}
		if a.Height != b.Height {
			return a.Height > b.Height
		}
		return a.Bitrate > b.Bitrate
	})
	return m, nil
}

// defaultAudioTrack picks the original-language track of a dubbed video.
func defaultAudioTrack(fs []streamFormat) string {
	id := ""
	for _, f := range fs {
		if f.AudioTrack == nil || f.URL == "" {
			continue
		}
		if strings.Contains(strings.ToLower(f.AudioTrack.DisplayName), "original") {
			return f.AudioTrack.ID
		}
		if f.AudioTrack.AudioIsDefault && id == "" {
			id = f.AudioTrack.ID
		}
	}
	return id
}

// streamHeaders go with every googlevideo request: the URLs are bound to the
// client that asked for them.
func streamHeaders() http.Header {
	return http.Header{
		"User-Agent": {visionOS.ua},
		"Origin":     {origin},
		"Referer":    {origin + "/"},
	}
}

func toFormat(f streamFormat) (media.Format, bool) {
	mt, params, err := mime.ParseMediaType(f.MimeType)
	if err != nil {
		return media.Format{}, false
	}
	kind, sub, _ := strings.Cut(mt, "/")
	codecs := strings.ReplaceAll(params["codecs"], " ", "")
	var ac string
	for _, c := range strings.Split(codecs, ",") {
		if isAudioCodec(c) {
			ac = c
		}
	}
	mf := media.Format{
		ID:      strconv.Itoa(f.Itag),
		Codec:   codecs,
		Height:  f.Height,
		FPS:     f.FPS,
		Bitrate: firstInt(f.AverageBitrate, f.Bitrate),
		Size:    -1,
	}
	if n, err := strconv.ParseInt(f.ContentLength, 10, 64); err == nil && n > 0 {
		mf.Size = n
	}
	switch kind {
	case "video":
		mf.Kind = "video"
		mf.HasVideo = true
		mf.HasAudio = ac != ""
		mf.Ext = sub
		mf.Label = firstOf(f.QualityLabel, strconv.Itoa(f.Height)+"p")
	case "audio":
		if !knownAudio(ac) {
			return media.Format{}, false
		}
		mf.Kind = "audio"
		mf.HasAudio = true
		mf.Ext = sub
		if sub == "mp4" {
			mf.Ext = "m4a"
		}
		mf.Label = strconv.Itoa((mf.Bitrate+500)/1000) + " kbps"
	default:
		return media.Format{}, false
	}
	mf.Parts = []plan.Part{{Kind: plan.PartHTTP, URL: f.URL, Ext: mf.Ext, Size: mf.Size, Headers: streamHeaders()}}
	return mf, true
}

func isAudioCodec(c string) bool {
	c = strings.ToLower(c)
	return strings.HasPrefix(c, "mp4a") || c == "opus" || c == "vorbis" || strings.HasPrefix(c, "ac-3") ||
		strings.HasPrefix(c, "ec-3") || strings.HasPrefix(c, "iamf") || c == "flac"
}

// iamf and other spatial codecs play almost nowhere; stereo stays the choice.
func knownAudio(c string) bool {
	c = strings.ToLower(c)
	return strings.HasPrefix(c, "mp4a") || c == "opus" || c == "vorbis"
}

func firstInt(vals ...int) int {
	for _, v := range vals {
		if v > 0 {
			return v
		}
	}
	return 0
}

func atof(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
