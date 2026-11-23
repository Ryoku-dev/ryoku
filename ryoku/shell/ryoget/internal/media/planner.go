package media

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"ryoget/internal/mux"
	"ryoget/internal/plan"

	i18n "ryoku-i18n"
)

const nameLimit = 180

// PlanFor decides what one download of m fetches and how the parts become the
// final file. Torrents and pages have no fetch plan. A post of several items
// plans its formats (the first video) unless items are selected or it has no
// formats; then the items land as files in one folder.
func PlanFor(m *Media, opts Options) (*plan.Plan, error) {
	pl, err := planFor(m, opts)
	if err == nil && opts.CoverArt {
		addCover(pl, m)
	}
	return pl, err
}

// addCover fetches the thumbnail of an audio-only download as an optional part the finish step embeds.
func addCover(pl *plan.Plan, m *Media) {
	if (pl.Post != plan.PostAudio && pl.Post != plan.PostTranscode) || !mux.CoverBoxes(pl.Container) {
		return
	}
	u, err := url.Parse(m.Thumb)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return
	}
	var h http.Header
	if ua := m.Headers["User-Agent"]; ua != "" {
		h = http.Header{"User-Agent": {ua}}
	}
	// the extension is left unknown so ffmpeg reads the picture type from its bytes
	pl.Parts = append(pl.Parts, plan.Part{Kind: plan.PartHTTP, URL: u.String(), Ext: "cover", Size: -1, Role: "cover", Headers: h})
}

func planFor(m *Media, opts Options) (*plan.Plan, error) {
	if m == nil {
		return nil, ErrInvalid
	}
	switch m.Kind {
	case plan.KindFile:
		return filePlan(m), nil
	case plan.KindMedia:
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupported, i18n.T("This link has nothing to plan."))
	}
	if len(m.Items) > 0 && (len(opts.Items) > 0 || len(m.Formats) == 0) {
		if p, ok, err := itemsPlan(m, opts); ok {
			return p, err
		}
	}
	if len(m.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, i18n.T("No downloadable formats were found."))
	}
	mode := opts.Mode
	if mode == "" {
		mode = "auto"
	}
	set := classify(m.Formats)
	limit := heightLimit(opts.Quality)

	if opts.Format != "" {
		return explicitPlan(m, set, mode, opts)
	}
	switch mode {
	case "audio":
		return audioPlan(m, set, opts.AudioFormat)
	case "mute":
		if v := bestVideo(set.video, limit); v != nil {
			return build(m, []*Format{v}, plan.PostNone, "")
		}
		if p := bestVideo(set.both, limit); p != nil {
			return build(m, []*Format{p}, plan.PostMute, "")
		}
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, i18n.T("This link has no video."))
	default:
		return autoPlan(m, set, limit)
	}
}

type formatSet struct {
	video []*Format // video only
	audio []*Format // audio only
	both  []*Format // video with audio
}

func classify(fs []Format) formatSet {
	var s formatSet
	for i := range fs {
		f := &fs[i]
		if len(f.Parts) == 0 {
			continue
		}
		hv, ha := f.HasVideo, f.HasAudio
		if !hv && !ha {
			if f.Kind == "audio" {
				ha = true
			} else {
				hv, ha = true, true
			}
		}
		switch {
		case hv && ha:
			s.both = append(s.both, f)
		case hv:
			s.video = append(s.video, f)
		default:
			s.audio = append(s.audio, f)
		}
	}
	return s
}

func heightLimit(q string) int {
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(q), "p"))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func autoPlan(m *Media, s formatSet, limit int) (*plan.Plan, error) {
	v := bestVideo(s.video, limit)
	p := bestVideo(s.both, limit)
	split := v != nil && len(s.audio) > 0
	if split && p != nil {
		fits := func(f *Format) bool { return limit == 0 || f.Height <= limit }
		switch {
		case fits(v) != fits(p):
			split = fits(v)
		case !fits(v):
			split = v.Height < p.Height
		default:
			split = v.Height > p.Height || (v.Height == p.Height && v.FPS > p.FPS)
		}
	}
	if split {
		a := bestAudio(s.audio, family(v.Codec, true), "")
		return build(m, []*Format{v, a}, plan.PostMerge, "")
	}
	if p != nil {
		return build(m, []*Format{p}, plan.PostNone, "")
	}
	if v != nil {
		return build(m, []*Format{v}, plan.PostNone, "")
	}
	return audioPlan(m, s, "")
}

func audioPlan(m *Media, s formatSet, want string) (*plan.Plan, error) {
	want = normAudio(want)
	if a := bestAudio(s.audio, "", want); a != nil {
		return audioFrom(m, a, want)
	}
	if p := bestVideo(s.both, 0); p != nil {
		return audioFrom(m, p, want)
	}
	return nil, fmt.Errorf("%w: %s", ErrUnavailable, i18n.T("This link has no audio."))
}

func audioFrom(m *Media, f *Format, want string) (*plan.Plan, error) {
	have := audioContainer(f.Codec)
	if want != "" && want != have {
		return build(m, []*Format{f}, plan.PostTranscode, want)
	}
	if have == "" {
		have = "mka"
	}
	return build(m, []*Format{f}, plan.PostAudio, have)
}

func explicitPlan(m *Media, s formatSet, mode string, opts Options) (*plan.Plan, error) {
	ids := strings.Split(opts.Format, "+")
	var picked []*Format
	for _, id := range ids {
		f := findFormat(m.Formats, id)
		if f == nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalid, i18n.Tf("There is no format %s.", id))
		}
		picked = append(picked, f)
	}
	if len(picked) > 1 {
		return build(m, picked, plan.PostMerge, "")
	}
	f := picked[0]
	one := classify([]Format{*f})
	switch {
	case mode == "audio" || len(one.audio) == 1:
		return audioFrom(m, f, normAudio(opts.AudioFormat))
	case mode == "mute" && len(one.both) == 1:
		return build(m, picked, plan.PostMute, "")
	case mode == "mute" || len(one.both) == 1:
		return build(m, picked, plan.PostNone, "")
	}
	if len(s.audio) == 0 {
		return build(m, picked, plan.PostNone, "")
	}
	a := bestAudio(s.audio, family(f.Codec, true), "")
	return build(m, []*Format{f, a}, plan.PostMerge, "")
}

func findFormat(fs []Format, id string) *Format {
	for i := range fs {
		if fs[i].ID == id {
			return &fs[i]
		}
	}
	return nil
}

// bestVideo picks the tallest format within limit; when nothing fits it takes
// the shortest one above it rather than failing.
func bestVideo(fs []*Format, limit int) *Format {
	if len(fs) == 0 {
		return nil
	}
	in := make([]*Format, 0, len(fs))
	for _, f := range fs {
		if limit == 0 || f.Height <= limit {
			in = append(in, f)
		}
	}
	if len(in) == 0 {
		low := fs[0]
		for _, f := range fs[1:] {
			if f.Height < low.Height || (f.Height == low.Height && videoBetter(f, low)) {
				low = f
			}
		}
		return low
	}
	sort.SliceStable(in, func(i, j int) bool { return videoBetter(in[i], in[j]) })
	return in[0]
}

func videoBetter(a, b *Format) bool {
	if a.Height != b.Height {
		return a.Height > b.Height
	}
	if a.FPS != b.FPS {
		return a.FPS > b.FPS
	}
	if ra, rb := videoRank(a.Codec), videoRank(b.Codec); ra != rb {
		return ra > rb
	}
	return a.Bitrate > b.Bitrate
}

// h264 first at equal height: it plays everywhere and stays in mp4; vp9 and av1
// win only by being the only way to reach a taller picture.
func videoRank(codec string) int {
	switch videoFamily(codec) {
	case "h264":
		return 4
	case "vp9":
		return 3
	case "av1":
		return 2
	case "hevc":
		return 1
	}
	return 0
}

// bestAudio prefers the format whose codec already matches what is wanted:
// want (a target file type) first, then the video's container family, then
// bitrate.
func bestAudio(fs []*Format, fam, want string) *Format {
	if len(fs) == 0 {
		return nil
	}
	score := func(f *Format) (int, int) {
		s := 0
		if want != "" && audioContainer(f.Codec) == want {
			s += 2
		}
		if fam != "" && family(f.Codec, false) == fam {
			s++
		}
		return s, f.Bitrate
	}
	best := fs[0]
	bs, bb := score(best)
	for _, f := range fs[1:] {
		s, b := score(f)
		if s > bs || (s == bs && b > bb) {
			best, bs, bb = f, s, b
		}
	}
	return best
}

func normAudio(a string) string {
	switch strings.ToLower(a) {
	case "mp3", "opus", "m4a":
		return strings.ToLower(a)
	}
	return ""
}

func codecs(c string) []string {
	var out []string
	for _, p := range strings.Split(strings.ToLower(c), ",") {
		if p = strings.TrimSpace(p); p != "" && p != "none" {
			out = append(out, p)
		}
	}
	return out
}

func videoFamily(c string) string {
	for _, p := range codecs(c) {
		switch {
		case strings.HasPrefix(p, "avc") || p == "h264":
			return "h264"
		case strings.HasPrefix(p, "vp09") || p == "vp9":
			return "vp9"
		case strings.HasPrefix(p, "vp8"):
			return "vp8"
		case strings.HasPrefix(p, "av01") || p == "av1":
			return "av1"
		case strings.HasPrefix(p, "hvc1") || strings.HasPrefix(p, "hev1") || p == "hevc" || p == "h265":
			return "hevc"
		}
	}
	return ""
}

func audioFamily(c string) string {
	for _, p := range codecs(c) {
		switch {
		case strings.HasPrefix(p, "mp4a") || p == "aac":
			return "aac"
		case p == "opus":
			return "opus"
		case p == "vorbis":
			return "vorbis"
		case p == "mp3" || p == "mp4a.40.34" || p == "mp4a.6b":
			return "mp3"
		case p == "flac":
			return "flac"
		case strings.HasPrefix(p, "ac-3") || strings.HasPrefix(p, "ec-3"):
			return "ac3"
		}
	}
	return ""
}

// family maps a codec to the container it stream-copies into without fuss:
// "mp4" or "webm" ("" when unknown).
func family(c string, video bool) string {
	if video {
		switch videoFamily(c) {
		case "h264", "hevc":
			return "mp4"
		case "vp9", "vp8":
			return "webm"
		case "av1":
			return "mp4"
		}
		return ""
	}
	switch audioFamily(c) {
	case "aac", "mp3", "ac3":
		return "mp4"
	case "opus", "vorbis":
		return "webm"
	}
	return ""
}

func audioContainer(c string) string {
	switch audioFamily(c) {
	case "aac":
		return "m4a"
	case "opus":
		return "opus"
	case "vorbis":
		return "ogg"
	case "mp3":
		return "mp3"
	case "flac":
		return "flac"
	}
	return ""
}

// container chooses a box every stream fits by copy: mp4 for h264/av1 + aac,
// webm for vp9/av1 + opus, mkv for anything mixed.
func container(fs []*Format) string {
	mp4, webm := true, true
	known := false
	for _, f := range fs {
		if v := videoFamily(f.Codec); v != "" {
			known = true
			mp4 = mp4 && (v == "h264" || v == "hevc" || v == "av1")
			webm = webm && (v == "vp9" || v == "vp8" || v == "av1")
		}
		if a := audioFamily(f.Codec); a != "" {
			known = true
			mp4 = mp4 && (a == "aac" || a == "mp3" || a == "ac3")
			webm = webm && (a == "opus" || a == "vorbis")
		}
	}
	if !known {
		for _, f := range fs {
			e := strings.ToLower(f.Ext)
			if e == "webm" || e == "mkv" {
				return e
			}
		}
		return "mp4"
	}
	switch {
	case mp4:
		return "mp4"
	case webm:
		return "webm"
	}
	return "mkv"
}

func build(m *Media, fs []*Format, post plan.Post, box string) (*plan.Plan, error) {
	if box == "" {
		box = boxFor(fs, post)
	}
	if post == plan.PostNone && len(fs) == 1 && needsRemux(fs[0]) {
		post = plan.PostRemux
	}
	pl := &plan.Plan{Name: SafeName(firstOf(m.Title, strings.TrimSuffix(m.Name, filepath.Ext(m.Name)), m.Source.Name), box), Container: box, Post: post}

	if usesYTDLP(fs) {
		ids := make([]string, len(fs))
		for i, f := range fs {
			ids[i] = f.ID
		}
		args := []string{"-f", strings.Join(ids, "+")}
		ext := box
		if len(fs) > 1 {
			args = append(args, "--merge-output-format", box)
			pl.Post = plan.PostNone
		} else {
			ext = firstOf(strings.ToLower(fs[0].Ext), box)
		}
		page := m.URL
		for _, p := range fs[0].Parts {
			if p.Kind == plan.PartYTDLP && p.URL != "" {
				page = p.URL
				break
			}
		}
		pl.Parts = []plan.Part{{Kind: plan.PartYTDLP, URL: page, Ext: ext, Size: totalSize(fs), Args: args,
			Headers: mergeHeaders(m.Headers, nil)}}
		return pl, nil
	}

	for _, f := range fs {
		for _, p := range onlyNeeded(f.Parts, post) {
			p.Headers = mergeHeaders(m.Headers, p.Headers)
			if p.Role == "" && len(fs) > 1 {
				if f.HasVideo || (!f.HasAudio && f.Kind != "audio") {
					p.Role = "video"
				} else {
					p.Role = "audio"
				}
			}
			if p.Ext == "" {
				p.Ext = firstOf(strings.ToLower(f.Ext), "bin")
			}
			pl.Parts = append(pl.Parts, p)
		}
	}
	if len(pl.Parts) > 1 && (pl.Post == plan.PostNone || pl.Post == plan.PostRemux) {
		pl.Post = plan.PostMerge
	}
	return pl, nil
}

// onlyNeeded drops a format's separate rendition that the post step would
// throw away anyway (the audio track of a muted download, the picture of an
// audio one).
func onlyNeeded(parts []plan.Part, post plan.Post) []plan.Part {
	want := ""
	switch post {
	case plan.PostAudio, plan.PostTranscode:
		want = "audio"
	case plan.PostMute:
		want = "video"
	}
	if want == "" || len(parts) < 2 {
		return parts
	}
	for _, p := range parts {
		if p.Role == want {
			return []plan.Part{p}
		}
	}
	return parts
}

func boxFor(fs []*Format, post plan.Post) string {
	if post == plan.PostNone && len(fs) == 1 && fs[0].Ext != "" && !hasHLS(fs) {
		return strings.ToLower(fs[0].Ext)
	}
	return container(fs)
}

// HLS lands as MPEG-TS or fragmented MP4 pieces; a copy into a real container
// makes it seekable and playable everywhere.
func needsRemux(f *Format) bool { return hasHLS([]*Format{f}) }

func hasHLS(fs []*Format) bool {
	for _, f := range fs {
		for _, p := range f.Parts {
			if p.Kind == plan.PartHLS {
				return true
			}
		}
	}
	return false
}

func usesYTDLP(fs []*Format) bool {
	for _, f := range fs {
		for _, p := range f.Parts {
			if p.Kind == plan.PartYTDLP {
				return true
			}
		}
	}
	return false
}

func totalSize(fs []*Format) int64 {
	var n int64
	for _, f := range fs {
		if f.Size <= 0 {
			return -1
		}
		n += f.Size
	}
	return n
}

func mergeHeaders(base map[string]string, own http.Header) http.Header {
	if len(base) == 0 && len(own) == 0 {
		return nil
	}
	h := http.Header{}
	for k, v := range base {
		h.Set(k, v)
	}
	for k, v := range own {
		h[http.CanonicalHeaderKey(k)] = append([]string(nil), v...)
	}
	return h
}

func filePlan(m *Media) *plan.Plan {
	name := SafeName(firstOf(m.Name, m.Title), "")
	if name == "" {
		name = "download"
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	return &plan.Plan{
		Name:      name,
		Container: ext,
		Parts: []plan.Part{{Kind: plan.PartHTTP, URL: m.URL, Ext: ext, Size: m.Size, Role: "file",
			Headers: mergeHeaders(m.Headers, nil)}},
	}
}

// SafeName makes a single path element from title: no separators or control
// characters, no leading dots, at most nameLimit bytes, then ext (without dot)
// appended when given.
func SafeName(title, ext string) string {
	var b strings.Builder
	space := false
	for _, r := range title {
		switch {
		case r == '/' || r == '\\' || r == 0:
			r = '-'
		case unicode.IsControl(r) || r == utf8.RuneError:
			r = ' '
		}
		if unicode.IsSpace(r) {
			if !space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = true
			continue
		}
		space = false
		b.WriteRune(r)
	}
	s := strings.Trim(b.String(), " .")
	limit := nameLimit
	if ext != "" {
		limit -= len(ext) + 1
	}
	if len(s) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = strings.TrimRight(s[:cut], " .")
	}
	if ext == "" {
		return s
	}
	if s == "" {
		s = "download"
	}
	return s + "." + ext
}
