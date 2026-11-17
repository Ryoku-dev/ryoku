package main

// The card's own picture. Settings hands over the file the reader chose and this
// decides, by its bytes and not its name, what it is and how it is kept beside
// profile.json as hero.<ext>. A badge holder's picture may move: an animated GIF
// or WebP, or a short video, up to maxMovingBytes. Everyone else keeps a still,
// so a moving GIF or WebP is kept as its first frame and a video is refused.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	maxMovingBytes   = 10 << 20
	maxMovingSeconds = 60
)

var errMovingNeedsBadge = errors.New("moving pictures come with a team badge")

// pictureKind is what a picture's bytes say it is.
type pictureKind struct {
	ext    string
	video  bool
	moving bool
}

// sniffPicture reads the container from the file's first bytes.
func sniffPicture(raw []byte) (pictureKind, error) {
	switch {
	case bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")):
		return pictureKind{ext: "png"}, nil
	case bytes.HasPrefix(raw, []byte{0xff, 0xd8, 0xff}):
		return pictureKind{ext: "jpg"}, nil
	case bytes.HasPrefix(raw, []byte("GIF87a")) || bytes.HasPrefix(raw, []byte("GIF89a")):
		frames, err := gifFrames(raw)
		if err != nil {
			return pictureKind{}, err
		}
		return pictureKind{ext: "gif", moving: frames > 1}, nil
	case len(raw) >= 12 && string(raw[:4]) == "RIFF" && string(raw[8:12]) == "WEBP":
		moving, err := webpAnimated(raw)
		if err != nil {
			return pictureKind{}, err
		}
		return pictureKind{ext: "webp", moving: moving}, nil
	case bytes.HasPrefix(raw, []byte("BM")):
		return pictureKind{ext: "bmp"}, nil
	case len(raw) >= 12 && string(raw[4:8]) == "ftyp":
		switch string(raw[8:12]) {
		case "avif":
			return pictureKind{ext: "avif"}, nil
		case "avis", "heic", "heix", "hevc", "hevx", "mif1", "msf1":
			return pictureKind{}, fmt.Errorf("this picture's format is not one Ryoku can show")
		}
		return pictureKind{ext: "mp4", video: true, moving: true}, nil
	case bytes.HasPrefix(raw, []byte{0x1a, 0x45, 0xdf, 0xa3}):
		// the EBML header names its DocType within its first few dozen bytes
		head := raw[:min(len(raw), 64)]
		if bytes.Contains(head, []byte("webm")) {
			return pictureKind{ext: "webm", video: true, moving: true}, nil
		}
		if bytes.Contains(head, []byte("matroska")) {
			return pictureKind{ext: "mkv", video: true, moving: true}, nil
		}
	}
	return pictureKind{}, fmt.Errorf("this file is not a picture or video Ryoku can show")
}

// gifFrames counts a GIF's frames by walking its blocks, without decoding any:
// a small file can unpack into far more pixels than it holds.
func gifFrames(raw []byte) (int, error) {
	bad := fmt.Errorf("this GIF is damaged")
	if len(raw) < 13 {
		return 0, bad
	}
	at := 13
	if flags := raw[10]; flags&0x80 != 0 {
		at += 3 << (flags&7 + 1)
	}
	skipBlocks := func() bool {
		for at < len(raw) {
			n := int(raw[at])
			at++
			if n == 0 {
				return true
			}
			at += n
		}
		return false
	}
	frames := 0
	for at < len(raw) {
		switch raw[at] {
		case 0x3b:
			return frames, nil
		case 0x21:
			at += 2
			if !skipBlocks() {
				return 0, bad
			}
		case 0x2c:
			if at+10 > len(raw) {
				return 0, bad
			}
			flags := raw[at+9]
			at += 10
			if flags&0x80 != 0 {
				at += 3 << (flags&7 + 1)
			}
			at++ // LZW minimum code size
			if !skipBlocks() {
				return 0, bad
			}
			frames++
		default:
			return 0, bad
		}
	}
	// a file cut short after whole frames still shows them
	if frames == 0 {
		return 0, bad
	}
	return frames, nil
}

// webpAnimated reads the extended header's animation flag and checks its canvas size.
func webpAnimated(raw []byte) (bool, error) {
	if len(raw) < 20 {
		return false, fmt.Errorf("this WebP is damaged")
	}
	if string(raw[12:16]) != "VP8X" {
		return false, nil
	}
	if len(raw) < 30 {
		return false, fmt.Errorf("this WebP is damaged")
	}
	w := int(raw[24]) | int(raw[25])<<8 | int(raw[26])<<16 + 1
	h := int(raw[27]) | int(raw[28])<<8 | int(raw[29])<<16 + 1
	if !validPictureSize(w, h) {
		return false, errPictureTooLarge
	}
	return raw[20]&0x02 != 0, nil
}

func sizeText(n int64) string {
	return strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64) + " MB"
}

func movingTooLarge(n int64) error {
	return fmt.Errorf("this moving picture is %s; moving pictures can be up to %s", sizeText(n), sizeText(maxMovingBytes))
}

// heroPoster names the still kept beside a moving picture that Go cannot decode itself.
func heroPoster(source string) string {
	ext := strings.TrimPrefix(filepath.Ext(source), ".")
	switch ext {
	case "webp", "mp4", "webm", "mkv":
		return "hero-" + ext + ".png"
	}
	return ""
}

// badgeHolder asks the rule Settings shows as Badge.ownBadge.
func badgeHolder() bool {
	_, held, _, _ := ownBadge(false)
	return held
}

func mediaTool(ctx context.Context, name string, args ...string) ([]byte, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("moving pictures need %s, which Ryoku installs; reinstall it and try again", name)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := readLimitedCommand(cmd, 64<<10)
	if err != nil {
		return nil, fmt.Errorf("%s: %v %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// probeVideo checks what the container holds: one picture stream, sound at most
// beside it (never played), and no longer than maxMovingSeconds.
func probeVideo(ctx context.Context, path string, kind pictureKind) error {
	out, err := mediaTool(ctx, "ffprobe", "-v", "error", "-show_entries",
		"format=format_name,duration:stream=codec_type,width,height", "-of", "json", "file:"+path)
	if err != nil {
		return err
	}
	var probe struct {
		Format struct {
			Name     string `json:"format_name"`
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Type   string `json:"codec_type"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		return fmt.Errorf("this video could not be read")
	}
	family := map[string]string{"mp4": "mp4", "webm": "webm", "mkv": "matroska"}[kind.ext]
	if !strings.Contains(probe.Format.Name, family) {
		return fmt.Errorf("this video's container is not what its first bytes say")
	}
	videos := 0
	for _, s := range probe.Streams {
		switch s.Type {
		case "video":
			videos++
			if !validPictureSize(s.Width, s.Height) {
				return errPictureTooLarge
			}
		case "audio":
		default:
			return fmt.Errorf("a video for the card holds only pictures and sound")
		}
	}
	if videos != 1 {
		return fmt.Errorf("a video for the card holds exactly one picture stream")
	}
	seconds, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil || seconds <= 0 {
		return fmt.Errorf("this video's length could not be read")
	}
	if seconds > maxMovingSeconds {
		return fmt.Errorf("this video is %.0f seconds long; a moving picture can be up to %d seconds", seconds, maxMovingSeconds)
	}
	return nil
}

// firstFrame has ffmpeg draw a moving picture's first frame as a PNG.
func firstFrame(ctx context.Context, src string) ([]byte, error) {
	f, err := os.CreateTemp(filepath.Dir(src), ".poster-*.png")
	if err != nil {
		return nil, err
	}
	dst := f.Name()
	f.Close()
	defer os.Remove(dst)
	if _, err := mediaTool(ctx, "ffmpeg", "-nostdin", "-v", "error", "-y", "-i", "file:"+src,
		"-frames:v", "1", "-an", "-f", "image2", "-c:v", "png", "file:"+dst); err != nil {
		return nil, err
	}
	raw, err := readPictureFile(dst)
	if err != nil {
		return nil, err
	}
	img, err := decodePicture(raw)
	if err != nil {
		return nil, fmt.Errorf("the first frame: %w", err)
	}
	var clean bytes.Buffer
	if err := png.Encode(&clean, img); err != nil {
		return nil, err
	}
	return clean.Bytes(), nil
}

// readPictureSource reads the chosen file whole, naming its size when it is too large.
func readPictureSource(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	if info.Size() > maxPictureBytes {
		return nil, fmt.Errorf("this file is %s; a picture can be up to %s", sizeText(info.Size()), sizeText(maxPictureBytes))
	}
	return readBounded(f, maxPictureBytes)
}

type savedPicture struct {
	Source string `json:"source"`
	Moving bool   `json:"moving"`
	// Still is set when a moving picture was kept as its first frame
	Still bool `json:"still,omitempty"`
}

// savePicture validates the file at src and keeps it as the card's picture.
func savePicture(src string) (savedPicture, error) {
	raw, err := readPictureSource(src)
	if err != nil {
		return savedPicture{}, err
	}
	kind, err := sniffPicture(raw)
	if err != nil {
		return savedPicture{}, err
	}
	// the bytes decide what is kept; a name claiming a video for a picture, or the other way
	// round, is a file disguised as something else
	switch strings.ToLower(filepath.Ext(src)) {
	case ".mp4", ".m4v", ".mov", ".webm", ".mkv":
		if !kind.video {
			return savedPicture{}, fmt.Errorf("this file is named as a video but holds a %s picture", strings.ToUpper(kind.ext))
		}
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".avif":
		if kind.video {
			return savedPicture{}, fmt.Errorf("this file is named as a picture but holds a video")
		}
	}
	holder := kind.moving && badgeHolder()
	if kind.moving && holder && len(raw) > maxMovingBytes {
		return savedPicture{}, movingTooLarge(int64(len(raw)))
	}
	if kind.video && !holder {
		return savedPicture{}, errMovingNeedsBadge
	}
	dir := profileHeroDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return savedPicture{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// tools read the very bytes that are kept, never the chosen file again
	f, err := os.CreateTemp(dir, ".hero-*")
	if err != nil {
		return savedPicture{}, err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, err = f.Write(raw)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, 0o644)
	}
	if err != nil {
		return savedPicture{}, err
	}

	switch {
	case kind.moving && !holder:
		// the first frame, as a still; a GIF Go reads itself, a WebP needs ffmpeg
		var still []byte
		if kind.ext == "gif" {
			img, err := decodePicture(raw)
			if err != nil {
				return savedPicture{}, err
			}
			var clean bytes.Buffer
			if err := png.Encode(&clean, img); err != nil {
				return savedPicture{}, err
			}
			still = clean.Bytes()
		} else if still, err = firstFrame(ctx, tmp); err != nil {
			return savedPicture{}, err
		}
		if err := atomicWrite(filepath.Join(dir, "hero.png"), still, 0o644); err != nil {
			return savedPicture{}, err
		}
		return savedPicture{Source: "hero.png", Still: true}, nil
	case kind.video:
		if err := probeVideo(ctx, tmp, kind); err != nil {
			return savedPicture{}, err
		}
	case kind.ext == "png" || kind.ext == "jpg" || kind.ext == "gif":
		if _, err := decodePicture(raw); err != nil {
			return savedPicture{}, err
		}
	}
	name := "hero." + kind.ext
	if poster := heroPoster(name); poster != "" {
		still, err := firstFrame(ctx, tmp)
		if err == nil {
			err = atomicWrite(filepath.Join(dir, poster), still, 0o644)
		}
		// a still WebP needs its poster only to travel in a .ryoprofile, so an ffmpeg that
		// cannot read it costs that and no more; a stale poster must not stand in for it
		if err != nil && (kind.moving || removeIfPresent(filepath.Join(dir, poster)) != nil) {
			return savedPicture{}, err
		}
	}
	if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
		return savedPicture{}, err
	}
	return savedPicture{Source: name, Moving: kind.moving}, nil
}

// heroStill is the custom picture as a PNG of one frame, for a .ryoprofile: the
// poster beside a moving picture, or the picture's own first frame.
func heroStill(source string) ([]byte, error) {
	path := filepath.Join(profileHeroDir(), source)
	if poster := heroPoster(source); poster != "" {
		path = filepath.Join(profileHeroDir(), poster)
	}
	raw, err := readPictureFile(path)
	if err != nil {
		return nil, err
	}
	img, err := decodePicture(raw)
	if err != nil {
		return nil, err
	}
	var clean bytes.Buffer
	if err := png.Encode(&clean, img); err != nil {
		return nil, err
	}
	return clean.Bytes(), nil
}

// liveWallpaper reports Ryogami's current wallpaper when it is a video, for the
// studio's "Use my live wallpaper".
func liveWallpaper() map[string]any {
	path := currentWallpaper()
	out := map[string]any{"video": false, "limit": maxMovingBytes}
	if path == "" {
		return out
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return out
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return out
	}
	head := make([]byte, 64)
	n, _ := f.Read(head)
	if kind, err := sniffPicture(head[:n]); err == nil && kind.video {
		out["video"] = true
		out["path"] = path
		out["bytes"] = info.Size()
		out["size"] = sizeText(info.Size())
	}
	return out
}
