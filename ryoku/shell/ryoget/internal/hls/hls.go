// Package hls fetches HTTP Live Streaming playlists into one file: the best
// (or the named) variant, segments fetched four at a time and written in
// order, AES-128 segments decrypted on the way.
package hls

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Eyevinn/hls-m3u8/m3u8"

	"ryoget/internal/plan"
)

const (
	workers      = 4
	window       = 8
	attempts     = 5
	chunk        = 16 << 10
	playlistMax  = 8 << 20
	segmentMax   = 256 << 20
	stateSuffix  = ".hls"
	userAgent    = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0"
	stateVersion = 1
)

// retryDelay is the first pause between attempts; it doubles each time.
var retryDelay = 500 * time.Millisecond

var (
	ErrDRM  = errors.New("hls: the stream is DRM protected")
	ErrLive = errors.New("hls: the stream is live")
)

// Variant is one choice a playlist offers. A plain media playlist yields a
// single Variant carrying only its URL.
type Variant struct {
	URL        string
	Name       string
	Bandwidth  int
	Width      int
	Height     int
	FPS        float64
	Codecs     string
	VideoCodec string
	AudioCodec string
	// separate audio rendition this variant plays with
	AudioURL  string
	AudioOnly bool
	NoVideo   bool
	NoAudio   bool
}

// Variants lists what the playlist at rawURL offers, best first.
func Variants(ctx context.Context, env *plan.Env, rawURL string, headers http.Header) ([]Variant, error) {
	base, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	pl, typ, err := loadPlaylist(ctx, env, base, headers)
	if err != nil {
		return nil, err
	}
	if typ == m3u8.MEDIA {
		return []Variant{{URL: rawURL}}, nil
	}
	return masterVariants(pl.(*m3u8.MasterPlaylist), base), nil
}

func masterVariants(master *m3u8.MasterPlaylist, base *url.URL) []Variant {
	var out []Variant
	audio := map[string]bool{}
	for _, v := range master.Variants {
		if v == nil || v.Iframe || v.URI == "" {
			continue
		}
		x := Variant{URL: resolve(base, v.URI), Name: v.Name, Bandwidth: int(v.Bandwidth), FPS: v.FrameRate, Codecs: v.Codecs}
		if v.AverageBandwidth > 0 {
			x.Bandwidth = int(v.AverageBandwidth)
		}
		if w, h, ok := strings.Cut(v.Resolution, "x"); ok {
			x.Width, _ = strconv.Atoi(w)
			x.Height, _ = strconv.Atoi(h)
		}
		x.VideoCodec, x.AudioCodec = splitCodecs(v.Codecs)
		if v.Codecs != "" {
			x.NoVideo = x.VideoCodec == ""
			x.NoAudio = x.AudioCodec == ""
		}
		var rendition *m3u8.Alternative
		for _, alt := range v.Alternatives {
			if alt == nil || !strings.EqualFold(alt.Type, "AUDIO") || alt.URI == "" || (v.Audio != "" && alt.GroupId != v.Audio) {
				continue
			}
			if rendition == nil || (alt.Default && !rendition.Default) {
				rendition = alt
			}
		}
		if rendition != nil {
			x.AudioURL = resolve(base, rendition.URI)
			x.NoAudio = false
			if !audio[x.AudioURL] {
				audio[x.AudioURL] = true
				out = append(out, Variant{URL: x.AudioURL, Name: rendition.Name, AudioOnly: true, NoVideo: true, AudioCodec: x.AudioCodec})
			}
		}
		if x.NoVideo && x.AudioURL == "" {
			x.AudioOnly = true
		}
		out = append(out, x)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.AudioOnly != b.AudioOnly {
			return !a.AudioOnly
		}
		if a.Height != b.Height {
			return a.Height > b.Height
		}
		return a.Bandwidth > b.Bandwidth
	})
	return out
}

func splitCodecs(c string) (video, audio string) {
	for _, p := range strings.Split(c, ",") {
		p = strings.TrimSpace(p)
		l := strings.ToLower(p)
		switch {
		case l == "":
		case strings.HasPrefix(l, "mp4a") || strings.HasPrefix(l, "ac-3") || strings.HasPrefix(l, "ec-3") || l == "opus" || l == "flac":
			if audio == "" {
				audio = p
			}
		case strings.HasPrefix(l, "stpp") || strings.HasPrefix(l, "wvtt"):
		default:
			if video == "" {
				video = p
			}
		}
	}
	return video, audio
}

func resolve(base *url.URL, ref string) string {
	r, err := base.Parse(strings.TrimSpace(ref))
	if err != nil {
		return ref
	}
	return r.String()
}

func client(env *plan.Env) *http.Client {
	if env != nil && env.HTTP != nil {
		return env.HTTP
	}
	return http.DefaultClient
}

func request(ctx context.Context, rawURL string, headers http.Header) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header[k] = append([]string(nil), v...)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}
	return req, nil
}

type statusError struct{ code int }

func (e statusError) Error() string {
	return "hls: " + http.StatusText(e.code) + " (" + strconv.Itoa(e.code) + ")"
}

func fatalStatus(err error) bool {
	var se statusError
	if !errors.As(err, &se) {
		return false
	}
	return se.code == http.StatusNotFound || se.code == http.StatusGone || se.code == http.StatusUnauthorized
}

func get(ctx context.Context, env *plan.Env, rawURL string, headers http.Header, rng string, max int64) ([]byte, error) {
	req, err := request(ctx, rawURL, headers)
	if err != nil {
		return nil, err
	}
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	resp, err := client(env).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, statusError{resp.StatusCode}
	}
	return io.ReadAll(io.LimitReader(resp.Body, max))
}

func loadPlaylist(ctx context.Context, env *plan.Env, u *url.URL, headers http.Header) (m3u8.Playlist, m3u8.ListType, error) {
	var data []byte
	err := retry(ctx, func() error {
		var err error
		data, err = get(ctx, env, u.String(), headers, "", playlistMax)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	if !bytes.Contains(data[:min(len(data), 1024)], []byte("#EXTM3U")) {
		return nil, 0, errors.New("hls: not a playlist")
	}
	pl, typ, err := m3u8.DecodeFrom(bufio.NewReader(bytes.NewReader(data)), false)
	if err != nil {
		return nil, 0, fmt.Errorf("hls: %w", err)
	}
	return pl, typ, nil
}

func retry(ctx context.Context, f func() error) error {
	var err error
	delay := retryDelay
	for i := range attempts {
		if err = f(); err == nil || ctx.Err() != nil || fatalStatus(err) {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if i == attempts-1 {
			break
		}
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
		delay *= 2
	}
	return err
}

// pick resolves a master playlist to the media playlist the part wants: the
// variant whose URL matches, else the one closest to Args height=N or
// bandwidth=N, else the best; Args audio picks its audio rendition.
func pick(ctx context.Context, env *plan.Env, part plan.Part) (*m3u8.MediaPlaylist, *url.URL, int, error) {
	u, err := url.Parse(part.URL)
	if err != nil {
		return nil, nil, 0, err
	}
	pl, typ, err := loadPlaylist(ctx, env, u, part.Headers)
	if err != nil {
		return nil, nil, 0, err
	}
	if typ == m3u8.MEDIA {
		return pl.(*m3u8.MediaPlaylist), u, 0, nil
	}
	vs := masterVariants(pl.(*m3u8.MasterPlaylist), u)
	var av []Variant
	for _, v := range vs {
		if !v.AudioOnly {
			av = append(av, v)
		}
	}
	if len(av) == 0 {
		av = vs
	}
	if len(av) == 0 {
		return nil, nil, 0, errors.New("hls: the playlist has no variants")
	}
	choice := av[0]
	opts := partArgs(part.Args)
	if h, ok := opts["height"]; ok {
		choice = closest(av, h, func(v Variant) int { return v.Height })
	} else if b, ok := opts["bandwidth"]; ok {
		choice = closest(av, b, func(v Variant) int { return v.Bandwidth })
	}
	target := choice.URL
	if _, ok := opts["audio"]; ok && choice.AudioURL != "" {
		target = choice.AudioURL
	}
	tu, _ := url.Parse(target)
	pl, typ, err = loadPlaylist(ctx, env, tu, part.Headers)
	if err != nil {
		return nil, nil, 0, err
	}
	if typ != m3u8.MEDIA {
		return nil, nil, 0, errors.New("hls: nested master playlist")
	}
	return pl.(*m3u8.MediaPlaylist), tu, choice.Bandwidth, nil
}

func partArgs(args []string) map[string]int {
	out := map[string]int{}
	for _, a := range args {
		k, v, _ := strings.Cut(a, "=")
		n, _ := strconv.Atoi(v)
		out[strings.ToLower(k)] = n
	}
	return out
}

func closest(vs []Variant, want int, val func(Variant) int) Variant {
	best := vs[0]
	bd := -1
	for _, v := range vs {
		d := val(v) - want
		over := d > 0
		if d < 0 {
			d = -d
		}
		// a variant above the wanted value only wins when nothing fits below
		if over {
			d += 1 << 30
		}
		if bd < 0 || d < bd {
			best, bd = v, d
		}
	}
	return best
}

type segment struct {
	index  int
	url    string
	rng    string
	key    *m3u8.Key
	keyURL string
	iv     []byte
	init   string // init section URL to write before this segment
	irng   string
	dur    float64
}

func segments(pl *m3u8.MediaPlaylist, base *url.URL) ([]segment, error) {
	var out []segment
	var key *m3u8.Key
	if len(pl.Keys) > 0 {
		k := pl.Keys[0]
		key = &k
	}
	var curMap *m3u8.Map = pl.Map
	var wroteMap *m3u8.Map
	seq := pl.SeqNo
	var lastURI string
	var lastEnd int64
	for i, s := range pl.GetAllSegments() {
		if s == nil {
			continue
		}
		if len(s.Keys) > 0 {
			k := s.Keys[0]
			key = &k
		}
		if s.Map != nil {
			curMap = s.Map
		}
		sg := segment{index: len(out), url: resolve(base, s.URI), dur: s.Duration}
		if s.Limit > 0 {
			off := s.Offset
			if off == 0 && s.URI == lastURI {
				off = lastEnd
			}
			sg.rng = fmt.Sprintf("bytes=%d-%d", off, off+s.Limit-1)
			lastEnd = off + s.Limit
		}
		lastURI = s.URI
		if key != nil && !strings.EqualFold(key.Method, "NONE") && key.Method != "" {
			if !strings.EqualFold(key.Method, "AES-128") {
				return nil, ErrDRM
			}
			if key.Keyformat != "" && !strings.EqualFold(key.Keyformat, "identity") {
				return nil, ErrDRM
			}
			sg.key = key
			sg.keyURL = resolve(base, key.URI)
			if key.IV != "" {
				iv, err := hex.DecodeString(strings.TrimPrefix(strings.TrimPrefix(key.IV, "0x"), "0X"))
				if err != nil || len(iv) != aes.BlockSize {
					return nil, fmt.Errorf("hls: bad IV %q", key.IV)
				}
				sg.iv = iv
			} else {
				sg.iv = make([]byte, aes.BlockSize)
				binary.BigEndian.PutUint64(sg.iv[8:], seq+uint64(i))
			}
		}
		if curMap != nil && (wroteMap == nil || !curMap.Equal(wroteMap)) {
			sg.init = resolve(base, curMap.URI)
			if curMap.Limit > 0 {
				sg.irng = fmt.Sprintf("bytes=%d-%d", curMap.Offset, curMap.Offset+curMap.Limit-1)
			}
			wroteMap = curMap
		}
		out = append(out, sg)
	}
	return out, nil
}

type state struct {
	Version  int    `json:"version"`
	Playlist string `json:"playlist"`
	Count    int    `json:"count"`
	Done     int    `json:"done"`
	Offset   int64  `json:"offset"`
}

func identity(segs []segment) string {
	if len(segs) == 0 {
		return ""
	}
	u, _ := url.Parse(segs[0].url)
	if u != nil {
		u.RawQuery = ""
		return u.String() + "|" + strconv.Itoa(len(segs))
	}
	return strconv.Itoa(len(segs))
}

func readState(dst string) state {
	var s state
	data, err := os.ReadFile(dst + stateSuffix)
	if err != nil || json.Unmarshal(data, &s) != nil || s.Version != stateVersion {
		return state{}
	}
	return s
}

func writeState(dst string, s state) {
	data, _ := json.Marshal(s)
	tmp := dst + stateSuffix + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		os.Rename(tmp, dst+stateSuffix)
	}
}

type resumer interface{ Resumed(bytes int64) }

// Fetch writes the stream part names into dst. An interrupted fetch of the
// same playlist continues after the last whole segment written.
func Fetch(ctx context.Context, env *plan.Env, part plan.Part, dst string, p plan.Progress) error {
	pl, base, bandwidth, err := pick(ctx, env, part)
	if err != nil {
		return err
	}
	if !pl.Closed && pl.MediaType != m3u8.VOD {
		return ErrLive
	}
	segs, err := segments(pl, base)
	if err != nil {
		return err
	}
	if len(segs) == 0 {
		return errors.New("hls: the playlist has no segments")
	}
	p.Stage("fetch")
	var duration float64
	for _, s := range segs {
		duration += s.dur
	}

	id := identity(segs)
	st := readState(dst)
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	start := 0
	var offset int64
	if st.Playlist == id && st.Count == len(segs) && st.Done > 0 && st.Done <= len(segs) {
		if fi, err := f.Stat(); err == nil && fi.Size() >= st.Offset {
			start, offset = st.Done, st.Offset
		}
	}
	if err := f.Truncate(offset); err != nil {
		return err
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	if start > 0 && offset > 0 {
		if r, ok := p.(resumer); ok {
			r.Resumed(offset)
		} else {
			p.Add(offset)
		}
		for i := range start {
			p.Mark(i, len(segs), 1)
		}
	}
	if bandwidth > 0 && duration > 0 {
		p.SetTotal(int64(float64(bandwidth) / 8 * duration))
	}
	// an init section belongs before the first segment fetched now as well
	if start > 0 {
		for i := start; i >= 0; i-- {
			if segs[i].init != "" {
				segs[start].init, segs[start].irng = segs[i].init, segs[i].irng
				break
			}
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	keys := &keyCache{m: map[string][]byte{}}
	results := make([]chan []byte, len(segs))
	for i := range results {
		results[i] = make(chan []byte, 1)
	}
	var (
		errOnce  sync.Once
		fetchErr error
	)
	fail := func(err error) {
		errOnce.Do(func() { fetchErr = err; cancel() })
	}
	ahead := make(chan struct{}, window)
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := start; i < len(segs); i++ {
			select {
			case ahead <- struct{}{}:
			case <-ctx.Done():
				return
			}
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			wg.Add(1)
			go func(s segment) {
				defer wg.Done()
				defer func() { <-sem }()
				data, err := fetchSegment(ctx, env, part.Headers, s, keys, p)
				if err != nil {
					fail(err)
					return
				}
				results[s.index] <- data
			}(segs[i])
		}
	}()

	written := offset
	var landed int64
	for i := start; i < len(segs); i++ {
		var data []byte
		select {
		case data = <-results[i]:
		case <-ctx.Done():
			wg.Wait()
			if fetchErr != nil {
				return fetchErr
			}
			return ctx.Err()
		}
		if _, err := f.Write(data); err != nil {
			fail(err)
			wg.Wait()
			return err
		}
		<-ahead
		written += int64(len(data))
		landed += int64(len(data))
		p.Mark(i, len(segs), 1)
		if done := i + 1 - start; done > 0 {
			p.SetTotal(offset + landed/int64(done)*int64(len(segs)-start))
		}
		if (i+1)%8 == 0 || i == len(segs)-1 {
			writeState(dst, state{Version: stateVersion, Playlist: id, Count: len(segs), Done: i + 1, Offset: written})
		}
	}
	wg.Wait()
	p.SetTotal(written)
	if err := f.Sync(); err != nil {
		return err
	}
	os.Remove(dst + stateSuffix)
	return nil
}

type keyCache struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (k *keyCache) get(ctx context.Context, env *plan.Env, headers http.Header, rawURL string) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if key, ok := k.m[rawURL]; ok {
		return key, nil
	}
	var key []byte
	err := retry(ctx, func() error {
		var err error
		key, err = get(ctx, env, rawURL, headers, "", 64)
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(key) != 16 {
		return nil, fmt.Errorf("hls: key is %d bytes, want 16", len(key))
	}
	k.m[rawURL] = key
	return key, nil
}

func fetchSegment(ctx context.Context, env *plan.Env, headers http.Header, s segment, keys *keyCache, p plan.Progress) ([]byte, error) {
	var out []byte
	if s.init != "" {
		err := retry(ctx, func() error {
			var err error
			out, err = get(ctx, env, s.init, headers, s.irng, segmentMax)
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	var body []byte
	err := retry(ctx, func() error {
		var err error
		body, err = download(ctx, env, s.url, headers, s.rng, p)
		return err
	})
	if err != nil {
		return nil, err
	}
	if s.key != nil {
		key, err := keys.get(ctx, env, headers, s.keyURL)
		if err != nil {
			return nil, err
		}
		plain, err := decrypt(body, key, s.iv)
		if err != nil {
			return nil, err
		}
		// count what lands in the file, not the cipher padding
		p.Add(int64(len(plain) - len(body)))
		body = plain
	}
	p.Add(int64(len(out)))
	return append(out, body...), nil
}

// download reads one segment through the shared limiter, reporting bytes as
// they arrive; a retried attempt takes back what the failed one reported.
func download(ctx context.Context, env *plan.Env, rawURL string, headers http.Header, rng string, p plan.Progress) ([]byte, error) {
	req, err := request(ctx, rawURL, headers)
	if err != nil {
		return nil, err
	}
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	resp, err := client(env).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, statusError{resp.StatusCode}
	}
	var buf bytes.Buffer
	if resp.ContentLength > 0 && resp.ContentLength < segmentMax {
		buf.Grow(int(resp.ContentLength))
	}
	tmp := make([]byte, chunk)
	var got int64
	var shared plan.Limiter
	if env != nil {
		shared = env.Limit
	}
	for {
		if err := plan.Wait(ctx, shared, chunk); err != nil {
			p.Add(-got)
			return nil, err
		}
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			got += int64(n)
			p.Add(int64(n))
			if buf.Len() > segmentMax {
				p.Add(-got)
				return nil, errors.New("hls: segment too large")
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			p.Add(-got)
			return nil, err
		}
	}
	if resp.ContentLength > 0 && got != resp.ContentLength {
		p.Add(-got)
		return nil, io.ErrUnexpectedEOF
	}
	return buf.Bytes(), nil
}

func decrypt(data, key, iv []byte) ([]byte, error) {
	if len(data)%aes.BlockSize != 0 || len(data) == 0 {
		return nil, errors.New("hls: encrypted segment is not block aligned")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, data)
	pad := int(out[len(out)-1])
	if pad == 0 || pad > aes.BlockSize || pad > len(out) {
		return nil, errors.New("hls: bad padding (wrong key?)")
	}
	for _, b := range out[len(out)-pad:] {
		if int(b) != pad {
			return nil, errors.New("hls: bad padding (wrong key?)")
		}
	}
	return out[:len(out)-pad], nil
}
