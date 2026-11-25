package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// the palette wakes the reader while open and as it closes, so a closed clipboard runs nothing once this passes
var clipReadWindow = 2 * time.Minute

const (
	clipReadTimeout = 20 * time.Second
	clipReadMaxText = 3000
	clipReadMinConf = 60.0
	clipReadFlat    = 0.45
	clipReadSmall   = 400
)

type clipReadJob struct {
	hash    uint64
	thumb   string
	payload string
	height  int
}

// idle priority and one thread, so reading a picture never competes with the desktop
var runClipRead = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "nice", append([]string{"-n", "19", name}, args...)...)
	prepareClipRead(cmd)
	cmd.Env = append(os.Environ(), "OMP_THREAD_LIMIT=1")
	var out clipReadOutput
	cmd.Stdout = &out
	err := cmd.Run()
	if ctx.Err() != nil {
		return out.Bytes(), ctx.Err()
	}
	return out.Bytes(), err
}

var clipReadLookPath = exec.LookPath

func (s *clipState) readingOn() bool {
	return s.readOn != nil && s.readOn()
}

func (s *clipState) wake() {
	s.mu.Lock()
	if !s.readingOn() {
		forgot := s.forgetTextLocked()
		if forgot {
			s.persistLocked()
		}
		s.mu.Unlock()
		if forgot {
			s.publish()
		}
		return
	}
	if s.readGone {
		s.mu.Unlock()
		return
	}
	s.readUntil = time.Now().Add(clipReadWindow)
	if s.readKick == nil {
		s.readKick = make(chan struct{}, 1)
	}
	kick := s.readKick
	start := !s.reading
	s.reading = true
	s.mu.Unlock()
	if start {
		go s.readLoop()
	} else {
		poke(kick)
	}
	s.publish()
}

func poke(kick chan struct{}) {
	select {
	case kick <- struct{}{}:
	default:
	}
}

func (s *clipState) readLoop() {
	languages := ""
	for {
		s.mu.Lock()
		if !s.readingOn() || !time.Now().Before(s.readUntil) {
			s.reading = false
			s.mu.Unlock()
			s.publish()
			return
		}
		job, ok := s.nextUnreadLocked()
		if !ok {
			wait := time.Until(s.readUntil)
			kick := s.readKick
			s.mu.Unlock()
			timer := time.NewTimer(wait)
			select {
			case <-kick:
			case <-timer.C:
			}
			timer.Stop()
			continue
		}
		s.mu.Unlock()
		if languages == "" {
			if _, err := clipReadLookPath("tesseract"); err != nil {
				log.Printf("ryoku-shell: clipboard pictures are not read: %v", err)
				s.mu.Lock()
				s.reading, s.readGone = false, true
				s.mu.Unlock()
				s.publish()
				return
			}
			languages = s.readLanguages()
		}
		text := s.readPicture(job, languages)
		s.mu.Lock()
		if e := s.entryByHashLocked(job.hash); e != nil && !e.Read && s.readingOn() {
			e.Read, e.text = true, text
			s.persistLocked()
		}
		s.mu.Unlock()
		s.publish()
	}
}

// newest first, so the picture just copied is the first one searchable
func (s *clipState) nextUnreadLocked() (clipReadJob, bool) {
	for _, e := range s.entries {
		if e.Kind != "image" || e.Read || e.ThumbPath == "" {
			continue
		}
		job := clipReadJob{hash: e.hash, thumb: e.ThumbPath, height: e.H}
		if s.dataDir != "" {
			job.payload = s.dataPath(e.hash)
		}
		return job, true
	}
	return clipReadJob{}, false
}

func (s *clipState) unreadLocked() int {
	n := 0
	for _, e := range s.entries {
		if e.Kind == "image" && !e.Read && e.ThumbPath != "" {
			n++
		}
	}
	return n
}

func (s *clipState) entryByHashLocked(hash uint64) *clipEntry {
	for _, e := range s.entries {
		if e.hash == hash {
			return e
		}
	}
	return nil
}

// the caller persists, so one write covers the forget and whatever else it changed
func (s *clipState) forgetTextLocked() bool {
	forgot := false
	for _, e := range s.entries {
		if e.Read || e.text != "" {
			e.Read, e.text = false, ""
			forgot = true
		}
	}
	return forgot
}

func (s *clipState) words() map[string]string {
	out := map[string]string{}
	s.mu.Lock()
	if !s.readingOn() {
		forgot := s.forgetTextLocked()
		if forgot {
			s.persistLocked()
		}
		s.mu.Unlock()
		if forgot {
			s.publish()
		}
		return out
	}
	for _, e := range s.entries {
		if e.Read && e.text != "" {
			out[strconv.FormatUint(e.ID, 10)] = e.text
		}
	}
	s.mu.Unlock()
	return out
}

func (s *clipState) readLanguages() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var installed []string
	if raw, err := runClipRead(ctx, "tesseract", "--list-langs"); err == nil {
		installed = parseClipReadLanguages(string(raw))
	}
	var requested []string
	if s.readLangs != nil {
		requested = s.readLangs()
	}
	return strings.Join(clipReadLanguageIntersection(requested, installed), "+")
}

func (s *clipState) readPicture(job clipReadJob, languages string) string {
	ctx, cancel := context.WithTimeout(context.Background(), clipReadTimeout)
	defer cancel()
	input := job.thumb
	// a photo is only read from its thumbnail, where big text still reads and grain costs nothing
	if job.payload != "" && thumbFlatness(job.thumb) >= clipReadFlat {
		input = job.payload
		if job.height > 0 && job.height < clipReadSmall {
			if big, ok := s.enlarge(ctx, job); ok {
				defer os.Remove(big)
				input = big
			}
		}
	}
	lines := []string{}
	if raw, err := runClipRead(ctx, "tesseract", input, "stdout", "-l", languages, "--psm", "3", "tsv"); err == nil {
		if text := clipReadWords(raw); text != "" {
			lines = append(lines, text)
		}
	}
	if job.payload != "" {
		if codes := clipReadCodes(ctx, job.payload); codes != "" {
			lines = append(lines, codes)
		}
	}
	return truncateBytesRunes([]byte(strings.Join(lines, "\n")), clipReadMaxText)
}

// Small type reads far better at twice the size.
func (s *clipState) enlarge(ctx context.Context, job clipReadJob) (string, bool) {
	if s.cacheDir == "" {
		return "", false
	}
	if _, err := clipReadLookPath("magick"); err != nil {
		return "", false
	}
	out := filepath.Join(s.cacheDir, fmt.Sprintf("ocr-%016x.png", job.hash))
	if _, err := runClipRead(ctx, "magick", job.payload, "-resize", "200%", out); err != nil {
		_ = os.Remove(out)
		return "", false
	}
	return out, true
}

func clipReadCodes(ctx context.Context, path string) string {
	if _, err := clipReadLookPath("zbarimg"); err != nil {
		return ""
	}
	raw, err := runClipRead(ctx, "zbarimg", "--quiet", "--xml", path)
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 4 {
			return ""
		}
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	codes, err := decodeClipReadCodes(raw)
	if err != nil {
		return ""
	}
	texts := make([]string, 0, len(codes))
	for _, code := range codes {
		if t := strings.TrimSpace(code); t != "" {
			texts = append(texts, t)
		}
	}
	return strings.Join(texts, "\n")
}

// only words tesseract is sure of, so a photo's grain never turns into searchable junk
func clipReadWords(tsv []byte) string {
	var out strings.Builder
	line := []string{}
	last := ""
	flush := func() {
		if len(line) == 0 {
			return
		}
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(strings.Join(line, " "))
		line = line[:0]
	}
	scanner := bufio.NewScanner(bytes.NewReader(tsv))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 12)
		if len(fields) != 12 || fields[0] != "5" {
			continue
		}
		conf, err := strconv.ParseFloat(fields[10], 64)
		word := strings.TrimSpace(fields[11])
		if err != nil || conf < clipReadMinConf || !clipWordish(word) {
			continue
		}
		if key := strings.Join(fields[1:5], "/"); key != last {
			flush()
			last = key
		}
		line = append(line, word)
	}
	flush()
	return out.String()
}

// two letters or digits make a word, or one from a script that writes words in a single character
func clipWordish(word string) bool {
	n := 0
	for _, r := range word {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return true
		}
		n++
	}
	return n >= 2
}

// a screen is mostly runs of identical neighbouring pixels, a photo almost never is
func thumbFlatness(thumb string) float64 {
	f, err := os.Open(thumb)
	if err != nil {
		return 0
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		return 0
	}
	b := img.Bounds()
	var pix []byte
	stride := 0
	switch m := img.(type) {
	case *image.RGBA:
		pix, stride = m.Pix, m.Stride
	case *image.NRGBA:
		pix, stride = m.Pix, m.Stride
	}
	same, pairs := 0, 0
	for y := range b.Dy() {
		for x := 1; x < b.Dx(); x++ {
			pairs++
			if pix != nil {
				at := y*stride + x*4
				if bytes.Equal(pix[at-4:at], pix[at:at+4]) {
					same++
				}
				continue
			}
			r0, g0, b0, a0 := img.At(b.Min.X+x-1, b.Min.Y+y).RGBA()
			r1, g1, b1, a1 := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if r0 == r1 && g0 == g1 && b0 == b1 && a0 == a1 {
				same++
			}
		}
	}
	if pairs == 0 {
		return 0
	}
	return float64(same) / float64(pairs)
}
