// Package ytdlp hands links our own resolvers cannot read to an installed
// yt-dlp: Resolve asks it for the formats (direct ones go through our engine),
// Fetch lets it download what only it can.
package ytdlp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	"ryoget/internal/media"
	"ryoget/internal/plan"

	i18n "ryoku-i18n"
)

func init() {
	media.RegisterFallback(func(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
		return Resolve(ctx, env, u.String())
	})
}

const resolveTimeout = 90 * time.Second

// Binary returns the yt-dlp to run, or an error wrapping media.ErrUnsupported
// when none is installed.
func Binary(env *plan.Env) (string, error) {
	name := "yt-dlp"
	if env != nil && env.YTDLP != "" {
		name = env.YTDLP
	}
	bin, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Install yt-dlp to download from this site."))
	}
	return bin, nil
}

type info struct {
	Type         string            `json:"_type"`
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Uploader     string            `json:"uploader"`
	Channel      string            `json:"channel"`
	Duration     float64           `json:"duration"`
	Thumbnail    string            `json:"thumbnail"`
	WebpageURL   string            `json:"webpage_url"`
	Extractor    string            `json:"extractor_key"`
	URL          string            `json:"url"`
	Ext          string            `json:"ext"`
	Protocol     string            `json:"protocol"`
	VCodec       string            `json:"vcodec"`
	ACodec       string            `json:"acodec"`
	Height       float64           `json:"height"`
	FPS          float64           `json:"fps"`
	TBR          float64           `json:"tbr"`
	Filesize     float64           `json:"filesize"`
	FilesizeApx  float64           `json:"filesize_approx"`
	Headers      map[string]string `json:"http_headers"`
	Formats      []format          `json:"formats"`
	Entries      []info            `json:"entries"`
	PlaylistSize int               `json:"playlist_count"`
	LiveStatus   string            `json:"live_status"`
}

type format struct {
	ID          string            `json:"format_id"`
	URL         string            `json:"url"`
	ManifestURL string            `json:"manifest_url"`
	Ext         string            `json:"ext"`
	Protocol    string            `json:"protocol"`
	VCodec      string            `json:"vcodec"`
	ACodec      string            `json:"acodec"`
	Height      float64           `json:"height"`
	FPS         float64           `json:"fps"`
	TBR         float64           `json:"tbr"`
	ABR         float64           `json:"abr"`
	VBR         float64           `json:"vbr"`
	Filesize    float64           `json:"filesize"`
	FilesizeApx float64           `json:"filesize_approx"`
	Note        string            `json:"format_note"`
	Headers     map[string]string `json:"http_headers"`
	HasDRM      any               `json:"has_drm"`
	Language    string            `json:"language"`
	LangPref    float64           `json:"language_preference"`
}

// Resolve runs `yt-dlp -J` on rawURL and turns its answer into Media. Formats
// with a plain URL become our own HTTP or HLS parts; the rest stay with yt-dlp.
func Resolve(ctx context.Context, env *plan.Env, rawURL string) (*media.Media, error) {
	bin, err := Binary(env)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--ignore-config", "-J", "--no-warnings", "--no-playlist", "--playlist-items", "1", "--", rawURL)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 3 * time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("%w: %s", media.ErrNetwork, i18n.T("yt-dlp took too long to answer."))
			}
			return nil, ctx.Err()
		}
		return nil, classify(stderr.String())
	}
	var in info
	if err := json.Unmarshal(stdout.Bytes(), &in); err != nil {
		return nil, fmt.Errorf("yt-dlp: %v", err)
	}
	return toMedia(&in, rawURL)
}

func toMedia(in *info, rawURL string) (*media.Media, error) {
	note := ""
	if in.Type == "playlist" || in.Type == "multi_video" {
		if len(in.Entries) == 0 {
			return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("The playlist is empty."))
		}
		count := in.PlaylistSize
		first := in.Entries[0]
		in = &first
		if count > 1 {
			note = i18n.Tf("Playlist of %d: only the first item is downloaded.", count)
		} else {
			note = i18n.T("Playlist: only the first item is downloaded.")
		}
	}
	page := firstOf(in.WebpageURL, rawURL)
	m := &media.Media{
		Kind:     plan.KindMedia,
		URL:      page,
		Source:   media.Source{ID: "ytdlp", Name: firstOf(in.Extractor, "yt-dlp")},
		Title:    firstOf(in.Title, in.ID),
		Author:   firstOf(in.Uploader, in.Channel),
		Duration: in.Duration,
		Thumb:    in.Thumbnail,
		Note:     note,
	}
	fs := in.Formats
	if len(fs) == 0 && in.URL != "" {
		fs = []format{{ID: "0", URL: in.URL, Ext: in.Ext, Protocol: in.Protocol, VCodec: in.VCodec, ACodec: in.ACodec,
			Height: in.Height, FPS: in.FPS, TBR: in.TBR, Filesize: in.Filesize, FilesizeApx: in.FilesizeApx, Headers: in.Headers}}
	}
	for _, f := range fs {
		if mf, ok := toFormat(f, page); ok {
			m.Formats = append(m.Formats, mf)
		}
	}
	if len(m.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("No downloadable formats were found."))
	}
	return m, nil
}

func drm(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x != "" && x != "maybe"
	}
	return false
}

func toFormat(f format, page string) (media.Format, bool) {
	note := strings.ToLower(f.Note)
	// dynamic-range-compressed audio duplicates the normal track, quieter
	if drm(f.HasDRM) || f.ID == "" || f.Ext == "mhtml" || strings.Contains(note, "storyboard") ||
		strings.HasSuffix(f.ID, "-drc") || strings.Contains(note, "drc") {
		return media.Format{}, false
	}
	vc, ac := codec(f.VCodec), codec(f.ACodec)
	hasV, hasA := vc != "" || f.VCodec == "", ac != "" || f.ACodec == ""
	if f.VCodec == "none" && f.ACodec == "none" {
		return media.Format{}, false
	}
	mf := media.Format{
		ID: f.ID, Ext: f.Ext, Codec: strings.Trim(vc+","+ac, ","),
		Height: int(f.Height), FPS: int(math.Round(f.FPS)), Bitrate: int(f.TBR * 1000),
		HasVideo: hasV, HasAudio: hasA,
		Size: int64(firstNum(f.Filesize, f.FilesizeApx, -1)),
	}
	mf.Kind = "video"
	if !hasV {
		mf.Kind = "audio"
		if f.ABR > 0 {
			mf.Bitrate = int(f.ABR * 1000)
		}
	}
	mf.Label = label(f, hasV)
	headers := http.Header{}
	for k, v := range f.Headers {
		headers.Set(k, v)
	}
	switch {
	case f.URL != "" && (f.Protocol == "https" || f.Protocol == "http"):
		mf.Parts = []plan.Part{{Kind: plan.PartHTTP, URL: f.URL, Ext: f.Ext, Size: mf.Size, Headers: headers}}
	case f.URL != "" && (f.Protocol == "m3u8_native" || f.Protocol == "m3u8"):
		mf.Parts = []plan.Part{{Kind: plan.PartHLS, URL: f.URL, Ext: "ts", Size: -1, Headers: headers}}
	default:
		mf.Parts = []plan.Part{{Kind: plan.PartYTDLP, URL: page, Ext: f.Ext, Size: mf.Size, Args: []string{"-f", f.ID}}}
	}
	return mf, true
}

func codec(c string) string {
	if c == "none" {
		return ""
	}
	return c
}

func label(f format, video bool) string {
	if video && f.Height > 0 {
		s := strconv.Itoa(int(f.Height)) + "p"
		if f.FPS > 30 {
			s += strconv.Itoa(int(math.Round(f.FPS)))
		}
		return s
	}
	if !video {
		br := f.ABR
		if br == 0 {
			br = f.TBR
		}
		if br > 0 {
			return strconv.Itoa(int(math.Round(br))) + " kbps"
		}
	}
	return firstOf(f.Note, f.ID)
}

func classify(stderr string) error {
	msg := lastError(stderr)
	l := strings.ToLower(msg)
	var kind error
	switch {
	case strings.Contains(l, "unsupported url"):
		kind = media.ErrUnsupported
	case strings.Contains(l, "private video") || strings.Contains(l, "members-only") || strings.Contains(l, "login required") ||
		strings.Contains(l, "confirm your age") || strings.Contains(l, "requires authentication"):
		kind = media.ErrPrivate
	case strings.Contains(l, "not a bot") || strings.Contains(l, "429") || strings.Contains(l, "rate-limit") || strings.Contains(l, "rate limit"):
		kind = media.ErrRateLimited
	case strings.Contains(l, "unavailable") || strings.Contains(l, "not available") || strings.Contains(l, "404") ||
		strings.Contains(l, "removed") || strings.Contains(l, "does not exist"):
		kind = media.ErrUnavailable
	case strings.Contains(l, "timed out") || strings.Contains(l, "unable to download") || strings.Contains(l, "connection") ||
		strings.Contains(l, "name resolution") || strings.Contains(l, "network"):
		kind = media.ErrNetwork
	default:
		kind = media.ErrUnavailable
	}
	if msg == "" {
		msg = i18n.T("yt-dlp could not read this link.")
	}
	return fmt.Errorf("%w: %s", kind, msg)
}

func lastError(stderr string) string {
	var last string
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ERROR:") {
			last = strings.TrimSpace(strings.TrimPrefix(line, "ERROR:"))
		}
	}
	if last == "" {
		lines := strings.Split(strings.TrimSpace(stderr), "\n")
		last = strings.TrimSpace(lines[len(lines)-1])
	}
	if i := strings.Index(last, "] "); i > 0 && strings.HasPrefix(last, "[") {
		last = last[i+2:]
	}
	return last
}

type progressLine struct {
	Status     string  `json:"status"`
	Downloaded float64 `json:"downloaded_bytes"`
	Total      float64 `json:"total_bytes"`
	Estimate   float64 `json:"total_bytes_estimate"`
	Filename   string  `json:"filename"`
	TmpName    string  `json:"tmpfilename"`
	FragIndex  float64 `json:"fragment_index"`
	FragCount  float64 `json:"fragment_count"`
}

const (
	progressTag = "RYOGET-PROGRESS "
	pathTag     = "RYOGET-PATH "
	buckets     = 64
)

type resumer interface{ Resumed(bytes int64) }

// Fetch downloads what part selects with yt-dlp into dst, reporting progress
// from its --progress-template lines. yt-dlp keeps its own .part files, so a
// cancelled fetch resumes when run again.
func Fetch(ctx context.Context, env *plan.Env, part plan.Part, dst string, p plan.Progress) error {
	bin, err := Binary(env)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	args := []string{
		"--ignore-config", "--no-playlist", "--newline", "--progress", "--no-colors", "--no-mtime",
		"--progress-template", "download:" + progressTag + "%(progress)j",
		"--print", "after_move:" + pathTag + "%(filepath)s",
		"-N", "4", "--continue",
		"-o", strings.ReplaceAll(dst, "%", "%%"),
	}
	if env != nil && env.FFmpeg != "" {
		args = append(args, "--ffmpeg-location", env.FFmpeg)
	}
	if lim := limitRate(env); lim > 0 {
		args = append(args, "--limit-rate", strconv.FormatInt(lim, 10))
	}
	for k, v := range part.Headers {
		if len(v) > 0 && !strings.EqualFold(k, "Range") {
			args = append(args, "--add-headers", k+":"+v[0])
		}
	}
	args = append(args, part.Args...)
	args = append(args, "--", part.URL)

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// SIGINT lets yt-dlp stop its ffmpeg child and keep the .part files
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT) }
	cmd.WaitDelay = 10 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &limitedBuffer{buf: &stderr, max: 64 << 10}
	if err := cmd.Start(); err != nil {
		return err
	}
	syscall.Setpriority(syscall.PRIO_PROCESS, cmd.Process.Pid, 10)
	p.Stage("fetch")
	final := track(stdout, p)
	err = cmd.Wait()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return classify(stderr.String())
	}
	if final != "" && final != dst {
		if err := os.Rename(final, dst); err != nil {
			return err
		}
	}
	if _, err := os.Stat(dst); err != nil {
		return fmt.Errorf("yt-dlp: %s", i18n.T("yt-dlp finished without writing the file."))
	}
	return nil
}

// limitRate turns the engine's shared limiter into yt-dlp's --limit-rate when
// it is an x/time/rate limiter with a finite rate.
func limitRate(env *plan.Env) int64 {
	if env == nil || env.Limit == nil {
		return 0
	}
	l, ok := env.Limit.(interface{ Limit() rate.Limit })
	if !ok {
		return 0
	}
	r := l.Limit()
	if r == rate.Inf || r <= 0 {
		return 0
	}
	return int64(r)
}

type limitedBuffer struct {
	buf *bytes.Buffer
	max int
}

func (l *limitedBuffer) Write(b []byte) (int, error) {
	if over := l.buf.Len() + len(b) - l.max; over > 0 {
		rest := l.buf.Bytes()
		if over >= len(rest) {
			l.buf.Reset()
		} else {
			kept := append([]byte(nil), rest[over:]...)
			l.buf.Reset()
			l.buf.Write(kept)
		}
	}
	return l.buf.Write(b)
}

// track follows yt-dlp's stdout. Each stream yt-dlp downloads (video, then
// audio) is a file of its own; the piece map spreads over their sum.
func track(r io.Reader, p plan.Progress) string {
	type stream struct{ done, total int64 }
	streams := map[string]*stream{}
	var order []string
	var reported int64
	final := ""
	first := true
	marked := make([]float64, buckets)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, pathTag) {
			final = strings.TrimSpace(strings.TrimPrefix(line, pathTag))
			continue
		}
		if !strings.HasPrefix(line, progressTag) {
			continue
		}
		var pl progressLine
		if json.Unmarshal([]byte(strings.TrimPrefix(line, progressTag)), &pl) != nil {
			continue
		}
		key := pl.Filename
		s := streams[key]
		if s == nil {
			s = &stream{}
			streams[key] = s
			order = append(order, key)
		}
		if t := int64(firstNum(pl.Total, pl.Estimate, 0)); t > 0 {
			s.total = t
		}
		s.done = int64(pl.Downloaded)
		if pl.Status == "finished" && s.total < s.done {
			s.total = s.done
		}
		var done, total int64
		for _, k := range order {
			done += streams[k].done
			total += max(streams[k].total, streams[k].done)
		}
		if total > 0 {
			p.SetTotal(total)
		}
		if delta := done - reported; delta != 0 {
			if first && delta > 0 {
				// the first frame of a resumed run carries what is already on disk
				if rs, ok := p.(resumer); ok && pl.Status == "downloading" && delta > 1<<20 {
					rs.Resumed(delta)
				} else {
					p.Add(delta)
				}
			} else {
				p.Add(delta)
			}
			reported = done
		}
		first = false
		if total > 0 {
			markBuckets(p, marked, float64(done)/float64(total))
		}
	}
	return final
}

func markBuckets(p plan.Progress, marked []float64, frac float64) {
	frac = math.Max(0, math.Min(1, frac))
	pos := frac * float64(len(marked))
	for i := range marked {
		fill := math.Max(0, math.Min(1, pos-float64(i)))
		if fill != marked[i] && (fill == 1 || fill-marked[i] >= 0.05 || fill < marked[i]) {
			marked[i] = fill
			p.Mark(i, len(marked), fill)
		}
	}
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNum(vals ...float64) float64 {
	for _, v := range vals[:len(vals)-1] {
		if v > 0 {
			return v
		}
	}
	return vals[len(vals)-1]
}
