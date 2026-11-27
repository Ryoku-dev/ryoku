package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Extension sets mirror crates/ryogami/src/wall/mod.rs (IMAGE_EXTS/VIDEO_EXTS)
// rather than the leaner set sketched in the port brief: the daemon must key the
// cache identically to the Rust build it drops in for, so gif stays a static and
// the extra raster formats stay recognized.
var imageExts = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "webp": true, "bmp": true,
	"gif": true, "tiff": true, "tif": true, "avif": true,
}
var videoExts = map[string]bool{
	"mp4": true, "webm": true, "mkv": true, "avi": true, "mov": true,
}

// The Rust reference uses a 60s ceiling; the brief pins the Go port to 20s.
const cmdTimeout = 20 * time.Second

const (
	thumbW = 640
	thumbH = 360
	smallW = 240
	smallH = 135
)

// scanned is a discovered source file with its resolved thumbnail destinations.
type scanned struct {
	key         string
	name        string
	wpType      string
	src         string
	videoFile   string
	mtime       int64
	filesize    int64
	thumbPath   string
	thumbSmPath string
	// we marks a Steam Workshop source: Steam revalidation moves its
	// mtime at every login, so preview staleness keys on the size record
	// beside the clip instead of the file's timestamps.
	we bool
}

// ScanDirs walks the wallpaper and video dirs and returns fresh entries merged
// with prior state (favourites, tags, apply counts survive a rescan). onItem
// fires after each newly generated thumbnail (the cache.item event).
func ScanDirs(wallDir, videoDir, cacheDir string, prior map[string]Entry, onItem func(Entry)) (map[string]Entry, error) {
	// The Rust daemon kept thumbnails under cacheDir/wallpaper/, and the
	// machine's already-generated cache lives there; matching the layout keeps
	// that cache warm across the rewrite.
	thumbsDir := filepath.Join(cacheDir, "wallpaper", "thumbs")
	thumbsSmDir := filepath.Join(cacheDir, "wallpaper", "thumbs-sm")
	videoThumbsDir := filepath.Join(cacheDir, "wallpaper", "video-thumbs")
	animDir := filepath.Join(cacheDir, "wallpaper", "anim")
	for _, d := range []string{thumbsDir, thumbsSmDir, videoThumbsDir, animDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", d, err)
		}
	}

	// Mirror cache.rs rebuild(): statics from wallDir, videos from videoDir when
	// it differs, then videos from wallDir too (a mixed wallDir is supported).
	var items []scanned
	collectMedia(wallDir, thumbsDir, thumbsSmDir, "static", imageExts, "", cacheDir, &items)
	if videoDir != wallDir {
		collectMedia(videoDir, videoThumbsDir, thumbsSmDir, "video", videoExts, "vid-", cacheDir, &items)
	}
	collectMedia(wallDir, videoThumbsDir, thumbsSmDir, "video", videoExts, "vid-", cacheDir, &items)

	result := processItemsParallel(items, prior, onItem)
	// Workshop rows join the scan; a local file with the same key wins.
	if weScanHook != nil {
		for k, e := range weScanHook(cacheDir, prior, onItem) {
			if _, dup := result[k]; !dup {
				result[k] = e
			}
		}
	}
	return result, nil
}

var weScanHook func(cacheDir string, prior map[string]Entry, onItem func(Entry)) map[string]Entry

// collectMedia walks root recursively, skipping dotfiles and the cache subtree,
// and appends every file whose extension is in exts. name is the slash-joined
// path relative to root; key is that name with '/' replaced by '--', matching
// the thumbnail stem the Rust daemon derives via thumb::cache_key.
func collectMedia(root, thumbDir, thumbSmDir, wpType string, exts map[string]bool, smPrefix, cacheDir string, out *[]scanned) {
	absCache, _ := filepath.Abs(cacheDir)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		absPath, _ := filepath.Abs(path)
		if d.IsDir() {
			if absPath == absCache {
				return filepath.SkipDir
			}
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(rel), "."))
		if !exts[ext] {
			return nil
		}
		st, err := os.Stat(path)
		if err != nil {
			return nil
		}
		thumbName := strings.ReplaceAll(rel, "/", "--") + ".webp"
		videoFile := ""
		if wpType == "video" {
			videoFile = absPath
		}
		*out = append(*out, scanned{
			key:         strings.ReplaceAll(rel, "/", "--"),
			name:        rel,
			wpType:      wpType,
			src:         absPath,
			videoFile:   videoFile,
			mtime:       st.ModTime().Unix(),
			filesize:    st.Size(),
			thumbPath:   filepath.Join(thumbDir, thumbName),
			thumbSmPath: filepath.Join(thumbSmDir, smPrefix+thumbName),
		})
		return nil
	})
}

// processItem resolves one scanned file against prior state: an unchanged mtime
// reuses the prior entry at the file's current location; otherwise thumbs and
// colors are (re)built while favourites and apply counts carry over.
func processItem(it scanned, prior map[string]Entry, onItem func(Entry)) Entry {
	if p, ok := prior[it.key]; ok && p.Mtime == it.mtime {
		// Location follows the file, so a moved library or home keeps applying.
		p.Path = it.src
		// An animated still's VideoFile is its cached clip, not a location.
		if it.videoFile != "" {
			p.VideoFile = it.videoFile
		}
		// A warm entry may predate newer fields: backfill them without rebuilding thumbnails.
		if it.wpType == "video" && (p.VideoPrev == "" || !fileExists(p.VideoPrev)) {
			p.VideoPrev = ensureVideoPreview(it)
		}
		if p.Width == 0 && p.Height == 0 {
			fillDims(&p, it)
		}
		if p.Tags == "" {
			p.Tags = buildTags(it, p.Hue)
		}
		return p
	}

	e := Entry{
		Key:       it.key,
		Name:      it.name,
		Type:      it.wpType,
		Path:      it.src,
		VideoFile: it.videoFile,
		Mtime:     it.mtime,
		Filesize:  it.filesize,
	}
	if p, ok := prior[it.key]; ok {
		e.Favourite = p.Favourite
		e.ApplyCount = p.ApplyCount
	}

	// Warm-cache reuse: an existing thumbnail newer than its source is kept as
	// is so an already-built cache survives a fresh daemon start without work.
	regen := true
	if fi, err := os.Stat(it.thumbPath); err == nil && fi.ModTime().Unix() >= it.mtime {
		regen = false
	}
	if regen {
		if err := genFullThumb(it); err != nil {
			fmt.Fprintf(os.Stderr, "ryogami: thumb failed for %s: %v\n", it.name, err)
			return e
		}
	}
	e.Thumb = it.thumbPath

	if smi, err := os.Stat(it.thumbSmPath); err != nil || smi.ModTime().Unix() < fileMtime(it.thumbPath) {
		if err := genSmallThumb(it.thumbPath, it.thumbSmPath); err != nil {
			fmt.Fprintf(os.Stderr, "ryogami: small thumb failed for %s: %v\n", it.name, err)
		} else {
			e.ThumbSm = it.thumbSmPath
		}
	} else {
		e.ThumbSm = it.thumbSmPath
	}

	if it.wpType == "static" {
		if isAnimatedImage(it.src) {
			if animPath := ensureAnimatedMp4(it); animPath != "" {
				e.VideoFile = animPath
				e.Type = "video"
			}
		}
	}

	if it.wpType == "video" {
		e.VideoPrev = ensureVideoPreview(it)
	}
	hue, sat, richness := extractColors(it.thumbPath)
	e.Hue = int(hueBucket(hue, sat))
	e.Sat = int(sat)
	e.Richness = int(richness)
	fillDims(&e, it)
	e.Tags = buildTags(it, e.Hue)

	if regen {
		onItem(e)
	}
	return e
}

func fileMtime(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.ModTime().Unix()
}

// tmpPath is a unique ".tmp" sibling of the destination: a failed conversion
// must never leave a half-written file at the real path, and two overlapping
// generators of the same output (a forced rescan while another still runs)
// must never interleave into each other's writer and publish a corrupt file.
func tmpPath(dest string) string {
	ext := filepath.Ext(dest)
	return fmt.Sprintf("%s.tmp.%d-%d%s", strings.TrimSuffix(dest, ext),
		os.Getpid(), time.Now().UnixNano(), ext)
}

func genFullThumb(it scanned) error {
	if err := os.MkdirAll(filepath.Dir(it.thumbPath), 0o755); err != nil {
		return err
	}
	tmp := tmpPath(it.thumbPath)
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	var cmd *exec.Cmd
	switch it.wpType {
	case "static":
		cmd = exec.CommandContext(ctx, "magick",
			fmt.Sprintf("%s[0]", it.src),
			"-resize", fmt.Sprintf("%dx%d^", thumbW, thumbH),
			"-gravity", "center",
			"-extent", fmt.Sprintf("%dx%d", thumbW, thumbH),
			"-quality", "85", tmp)
	case "video":
		// Mirrors thumb::generate_video with cache.rs's seek of 0; the brief's
		// -q:v flag is absent from the reference so it is omitted here too.
		cmd = exec.CommandContext(ctx, "ffmpeg", "-y", "-ss", "0", "-i", it.src,
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", thumbW, thumbH, thumbW, thumbH),
			"-frames:v", "1", "-update", "1", tmp)
	default:
		return fmt.Errorf("unknown type %q", it.wpType)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("%s: %w: %s", cmd.Args[0], err, strings.TrimSpace(string(out)))
	}
	return os.Rename(tmp, it.thumbPath)
}

func genSmallThumb(thumbPath, thumbSmPath string) error {
	if err := os.MkdirAll(filepath.Dir(thumbSmPath), 0o755); err != nil {
		return err
	}
	tmp := tmpPath(thumbSmPath)
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "magick", thumbPath,
		"-resize", fmt.Sprintf("%dx%d^", smallW, smallH),
		"-gravity", "center",
		"-extent", fmt.Sprintf("%dx%d", smallW, smallH),
		"-quality", "85", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("magick: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return os.Rename(tmp, thumbSmPath)
}

// extractColors reproduces thumb.rs's route: the Rust build decodes the full
// 640x360 thumbnail in-process (image::open -> to_rgba8) and runs extract_hue_sat
// over every pixel. Go's stdlib cannot decode webp, so the pixels are pulled out
// with `magick <thumb> -depth 8 RGB:-` (full resolution, no resampling) and the
// identical histogram algorithm is applied to the raw bytes.
func extractColors(thumbPath string) (uint16, uint16, uint16) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "magick", thumbPath, "-depth", "8", "RGB:-").Output()
	if err != nil {
		return 0, 0, 0
	}
	return extractHueSat(out)
}

// extractHueSat is a faithful port of thumb.rs extract_hue_sat over interleaved
// 8-bit RGB pixels.
func extractHueSat(rgb []byte) (uint16, uint16, uint16) {
	var counts [13]uint64
	var meaningful uint64

	for i := 0; i+2 < len(rgb); i += 3 {
		r := float64(rgb[i]) / 255.0
		g := float64(rgb[i+1]) / 255.0
		b := float64(rgb[i+2]) / 255.0
		max := math.Max(r, math.Max(g, b))
		min := math.Min(r, math.Min(g, b))
		delta := max - min
		lightness := (max + min) / 2.0

		if lightness < 0.06 || lightness > 0.94 {
			continue
		}

		var sat float64
		if delta >= 1e-6 {
			sat = delta / (1.0 - math.Abs(2.0*lightness-1.0))
		}
		if sat < 0.18 {
			counts[12]++
			meaningful++
			continue
		}

		var hue float64
		switch {
		case math.Abs(max-r) < 1e-6:
			hue = 60.0 * math.Mod((g-b)/delta, 6.0)
		case math.Abs(max-g) < 1e-6:
			hue = 60.0*((b-r)/delta) + 120.0
		default:
			hue = 60.0*((r-g)/delta) + 240.0
		}
		if hue < 0.0 {
			hue += 360.0
		}
		hueU := uint16(int(math.Round(hue)) % 360)
		counts[hueToBucketIdx(hueU)]++
		meaningful++
	}

	if meaningful == 0 {
		return 0, 0, 0
	}

	var bestIdx int
	var bestCount uint64
	for i := range 12 {
		if counts[i] > bestCount {
			bestCount = counts[i]
			bestIdx = i
		}
	}

	var chromaticMass uint64
	for i := range 12 {
		chromaticMass += counts[i]
	}

	var richness uint16
	if chromaticMass != 0 {
		total := float64(chromaticMass)
		var sumsq float64
		for i := range 12 {
			if counts[i] == 0 {
				continue
			}
			p := float64(counts[i]) / total
			sumsq += p * p
		}
		if sumsq > 0.0 {
			richness = uint16(math.Min(math.Max(math.Round(100.0/sumsq), 0.0), 1500.0))
		}
	}

	if chromaticMass*100 < meaningful*5 {
		return 0, 0, richness
	}

	coverage := uint16(math.Round(float64(bestCount) / float64(meaningful) * 100.0))
	if coverage < 10 {
		coverage = 10
	} else if coverage > 100 {
		coverage = 100
	}

	var hueForBucket uint16
	switch bestIdx {
	case 0:
		hueForBucket = 10
	case 10:
		hueForBucket = 307
	case 11:
		hueForBucket = 337
	default:
		hueForBucket = 25 + uint16(bestIdx-1)*30 + 15
	}
	return hueForBucket, coverage, richness
}

func hueToBucketIdx(hue uint16) int {
	if hue < 25 || hue >= 355 {
		return 0
	}
	if hue >= 320 {
		return 11
	}
	if hue >= 295 {
		return 10
	}
	return int((hue-25)/30 + 1)
}

// hueBucket collapses a representative hue+saturation into the stored bucket:
// 99 for desaturated, else the 0-11 index. This is the value the Rust daemon
// writes to the hue column, so the Go port stores it identically.
func hueBucket(hue, sat uint16) uint16 {
	if sat < 10 {
		return 99
	}
	return uint16(hueToBucketIdx(hue))
}

// isAnimatedImage reports whether the source needs the mp4 transcode path so
// the in-shell player advances frames. Every animated-capable format is gated
// on its real frame count: a single-frame webp/png stays a still image and
// paints once, rather than being transcoded to a looping clip the player keeps
// decoding on the GPU.
func isAnimatedImage(src string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(src), "."))
	if ext != "gif" && ext != "png" && ext != "avif" && ext != "webp" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-count_frames",
		"-show_entries", "stream=nb_read_frames",
		"-of", "csv=p=0", src).Output()
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return false
	}
	return n > 1
}

func animatedH264Args(encoders codecSet) ([]string, error) {
	encoder, err := pickCPUH264Encoder(encoders)
	if err != nil {
		return nil, err
	}
	return h264CPUArgs(encoder, 1920, 24,
		"-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p")
}

// ensureAnimatedMp4 transcodes an animated image to a cached mp4 the player
// can play frame-by-frame. "" on failure (the entry stays static). Regenerated
// when missing or older than the source.
func ensureAnimatedMp4(it scanned) string {
	cacheRoot := filepath.Dir(filepath.Dir(it.thumbPath))
	animDir := filepath.Join(cacheRoot, "anim")
	if err := os.MkdirAll(animDir, 0o755); err != nil {
		return ""
	}
	animPath := filepath.Join(animDir, it.key+".mp4")
	if fi, err := os.Stat(animPath); err == nil && fi.ModTime().Unix() >= it.mtime {
		return animPath
	}
	tmp := tmpPath(animPath)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	codecArgs, err := animatedH264Args(ffmpegCodecs().encoders)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ryogami: animated transcode unavailable for %s: %v\n", it.name, err)
		return ""
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(it.src), "."))
	var args []string
	if ext == "gif" {
		// GIF needs a palette for clean h264 colors; the standard ffmpeg
		// recipe splits, generates a palette from one stream, applies it to
		// the other.
		args = []string{"-y", "-v", "error",
			"-i", it.src,
			"-filter_complex",
			"fps=24,scale='min(1920,iw)':-2:flags=lanczos,split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=5",
			"-map", "[b]"}
	} else {
		args = []string{"-y", "-v", "error",
			"-i", it.src,
			"-vf", "fps=24,scale='min(1920,iw)':-2"}
	}
	args = append(args, codecArgs...)
	args = append(args, "-an", tmp)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		fmt.Fprintf(os.Stderr, "ryogami: animated transcode failed for %s: %v: %s\n",
			it.name, err, strings.TrimSpace(string(out)))
		return ""
	}
	if os.Rename(tmp, animPath) != nil {
		os.Remove(tmp)
		return ""
	}
	return animPath
}

// previewRenderLocks serializes the render and publication of one clip at a
// time per preview path: a forced reset can overlap a running scan, and two
// writers for the same clip would otherwise publish clip and identity record
// in crossed order, leaving an old clip described by a new identity. Entries
// are refcounted and released when idle, so a library that churns does not
// grow the registry.
type previewLockEntry struct {
	mu   sync.Mutex
	refs int
}

var previewLocksMu sync.Mutex
var previewLocks = map[string]*previewLockEntry{}

// lockPreviewRender holds the render lock for one clip path; the returned
// func releases it and drops the registry entry when nothing waits on it.
func lockPreviewRender(prev string) func() {
	previewLocksMu.Lock()
	e, ok := previewLocks[prev]
	if !ok {
		e = &previewLockEntry{}
		previewLocks[prev] = e
	}
	e.refs++
	previewLocksMu.Unlock()
	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		previewLocksMu.Lock()
		e.refs--
		if e.refs == 0 && previewLocks[prev] == e {
			delete(previewLocks, prev)
		}
		previewLocksMu.Unlock()
	}
}

// previewSourceIdentity is the content marker a workshop clip records beside
// itself: the source's byte size and a hex prefix of its sha256 digest. The
// whole file is streamed through the hash, so a same-size update that changes
// bytes anywhere in the source moves the identity. Mtimes are deliberately
// not part of it: Steam revalidation rewrites them at every login, and a
// redownload can restore an old one, while the source bytes are the contract.
func previewSourceIdentity(src string, size int64) string {
	f, err := os.Open(src)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return strconv.FormatInt(size, 10) + ":" + hex.EncodeToString(h.Sum(nil)[:8])
}

// cachedPreviewSourceIdentity reuses the full-content identity computed on an
// earlier scan when the source's cheap stat signature (size and mtime) is
// unchanged. Hashing every workshop clip on every warm rescan is wasted work
// when the bytes were not touched. Any size or timestamp change drops the
// memo and forces a fresh full-content hash, so a same-size rewrite is still
// detected by previewSourceIdentity.
type previewIdentityMemo struct {
	size     int64
	mtime    int64
	identity string
}

var (
	previewIdentityMu    sync.Mutex
	previewIdentityMemos = map[string]previewIdentityMemo{}
)

func cachedPreviewSourceIdentity(it scanned) string {
	previewIdentityMu.Lock()
	memo, ok := previewIdentityMemos[it.src]
	previewIdentityMu.Unlock()
	if ok && memo.identity != "" && memo.size == it.filesize && memo.mtime == it.mtime {
		return memo.identity
	}
	identity := previewSourceIdentity(it.src, it.filesize)
	if identity != "" {
		previewIdentityMu.Lock()
		previewIdentityMemos[it.src] = previewIdentityMemo{
			size: it.filesize, mtime: it.mtime, identity: identity,
		}
		previewIdentityMu.Unlock()
	}
	return identity
}

// ensureVideoPreview builds the small clip the picker's hover/selection
// players decode, beside the video's thumbnail (<stem>-prev.mp4). Preview
// players used to decode the full source: a 4K60 HEVC-10 clip per hovered
// card, which froze the whole selector. 640w/24fps H.264 decodes for free.
// Regenerated when missing or older than the source; workshop clips
// instead reuse purely by recorded source identity (size plus a full-content
// hash), because Steam revalidation moves their mtimes at every login, a
// redownload can restore an old one, and a same-size update must not wear
// the previous clip forever; "" on failure, which callers treat as "no
// preview" (the delegate falls back to the still thumb).
func ensureVideoPreview(it scanned) string {
	prev := strings.TrimSuffix(it.thumbPath, ".webp") + "-prev.mp4"
	unlock := lockPreviewRender(prev)
	defer unlock()
	// A workshop clip is reusable only when both the source identity and the
	// record beside the clip are readable; two empty strings are not a match.
	var identity string
	if it.we {
		identity = cachedPreviewSourceIdentity(it)
		if identity == "" {
			return ""
		}
	}
	if fi, err := os.Stat(prev); err == nil {
		if it.we {
			// Workshop sources sit under Steam's control: validation
			// moves their mtimes at every login, and a redownload can
			// carry the manifest's old one back, so the recorded
			// identity decides whether the clip is current, not the
			// timestamps.
			if prevSourceIdentity(prev) == identity {
				return prev
			}
		} else if fi.ModTime().Unix() >= it.mtime {
			return prev
		}
	}
	if err := os.MkdirAll(filepath.Dir(prev), 0o755); err != nil {
		return ""
	}
	tmp := tmpPath(prev)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := transcodeH264(ctx, transcodeSpec{src: it.src, dst: tmp, fps: 24, maxWidth: 640, qp: 27, crf: 27}); err != nil {
		os.Remove(tmp)
		fmt.Fprintf(os.Stderr, "ryogami: preview failed for %s: %v\n", it.name, err)
		return ""
	}
	if it.we {
		// Drop the old record before the rename: a daemon killed between
		// the rename and the record write must leave either the new record
		// or none, never a stale record describing a clip that is gone.
		// An unremovable record makes publication unsafe, so skip it.
		if err := os.Remove(prev + ".meta"); err != nil && !errors.Is(err, os.ErrNotExist) {
			os.Remove(tmp)
			fmt.Fprintf(os.Stderr, "ryogami: cannot drop stale preview record for %s: %v\n", it.name, err)
			return ""
		}
	}
	if os.Rename(tmp, prev) != nil {
		os.Remove(tmp)
		return ""
	}
	if it.we {
		if err := writePrevSourceIdentity(prev, identity); err != nil {
			// The clip is published but unrecorded; clear any partial
			// record so the next scan re-renders instead of reusing it.
			_ = os.Remove(prev + ".meta")
			fmt.Fprintf(os.Stderr, "ryogami: preview record failed for %s: %v\n", it.name, err)
		}
	}
	return prev
}

// prevSourceIdentity reads the identity recorded when the clip beside it was
// rendered; "" when no record exists.
func prevSourceIdentity(prev string) string {
	b, err := os.ReadFile(prev + ".meta")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// writePrevSourceIdentity records the identity through a temp sibling and an
// atomic rename, so a killed writer never leaves a partial record that could
// mark the wrong clip as current.
func writePrevSourceIdentity(prev, identity string) error {
	meta := prev + ".meta"
	tmp := tmpPath(meta)
	if err := os.WriteFile(tmp, []byte(identity+"\n"), 0o644); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, meta); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// isTmpArtifact reports whether name is a temp sibling produced by tmpPath:
// a ".tmp.<pid>-<unixnano>" infix immediately before the destination's
// extension. A finished artifact whose stem merely contains ".tmp." (e.g.
// "photo.tmp.png.webp") is not a temp and must survive the startup sweep.
func isTmpArtifact(name string) bool {
	i := strings.LastIndex(name, ".tmp.")
	if i < 0 {
		return false
	}
	stamp := name[i+len(".tmp."):]
	dash := strings.IndexByte(stamp, '-')
	if dash < 1 {
		return false
	}
	for _, c := range stamp[:dash] {
		if c < '0' || c > '9' {
			return false
		}
	}
	tail := stamp[dash+1:]
	digits := strings.TrimSuffix(tail, filepath.Ext(tail))
	if digits == "" {
		return false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// sweepTmpFiles removes the temp artifacts a previous daemon lost mid-run: a
// killed transcode or thumb job never cleans up after itself, and a login's
// double start makes that routine. Callers must run it at startup, before any
// watcher, restore or rescan can create a new temp -- a broad ".tmp." match
// here would delete a live job's output.
func sweepTmpFiles(cacheDir string) {
	for _, sub := range []string{"wallpaper/thumbs", "wallpaper/thumbs-sm", "wallpaper/video-thumbs", "wallpaper/anim"} {
		dir := filepath.Join(cacheDir, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !isTmpArtifact(e.Name()) {
				continue
			}
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
