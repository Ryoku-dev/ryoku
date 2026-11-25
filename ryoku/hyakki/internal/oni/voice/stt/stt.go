// Package stt turns 16 kHz signed 16-bit mono speech into text through a
// speech engine that runs as a child process: never cgo, never a device.
// The caller hands it PCM it already captured; nothing here opens a
// microphone or writes audio anywhere but a private, short-lived file the
// file-only engine needs.
//
// Two adapters implement Transcriber: Whisper drives a resident
// whisper-server on loopback, and Dictate drives ryoku-dictate's file
// self-test when that program is installed. Every decode is bounded by the
// spec's voice.budgets (partial and final), and a resident engine unloads
// after voice.engines.sttIdleUnloadS without a decode.
package stt

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/oni/spec"
)

// SampleRate is the only rate the adapters accept.
const SampleRate = 16000

// MaxVocab caps the vocabulary hint: more names make a decoder hallucinate
// them into speech that never held them.
const MaxVocab = 14

// maxVocabRunes drops a hint entry longer than a name.
const maxVocabRunes = 48

var (
	// ErrMissingEngine: the engine program is not installed.
	ErrMissingEngine = errors.New("the speech engine is not installed")
	// ErrBudget: a decode ran past its spec budget and was abandoned.
	ErrBudget = errors.New("the speech engine ran past its time budget")
	// ErrClosed: the transcriber was closed.
	ErrClosed = errors.New("the speech engine was closed")
	// ErrModel: the model file is missing, unpinned or not the pinned file.
	ErrModel = errors.New("the speech model is not the pinned file")
	// ErrNoAudio: there is no PCM to decode.
	ErrNoAudio = errors.New("there is no audio to transcribe")
	// ErrNoConsent: a model download was asked for without the user's
	// agreement.
	ErrNoConsent = errors.New("a speech model is downloaded only after the user agrees")
)

// Info names exactly what decodes: the engine program, the model file, the
// model's licence and its pinned SHA-256, and the VAD model file the engine
// runs before decoding ("" when it runs none).
type Info struct {
	Engine, Model, License, SHA256 string
	VAD                            string
}

// Pass is which budget a decode runs under.
type Pass uint8

const (
	// Final is the decode of a finished segment (voice.budgets.finalS).
	Final Pass = iota
	// Partial is a decode of a segment still growing (voice.budgets.partialMs).
	Partial
)

func (p Pass) String() string {
	if p == Partial {
		return "partial"
	}
	return "final"
}

// Hint steers a decode. Vocab is at most MaxVocab proper names the user
// is likely to say; the adapters drop the character's own name from it.
type Hint struct {
	Language string
	Vocab    []string
	Pass     Pass
}

// Result is one decode.
type Result struct {
	Text string
	Pass Pass
	// Took is the wall time from the call to the text.
	Took time.Duration
	// Reused is set when a final reused the partial of the same audio.
	Reused bool
}

// Transcriber decodes speech. Transcribe never keeps pcm after it returns.
// Close unloads a resident engine; a closed Transcriber returns ErrClosed.
type Transcriber interface {
	Info() Info
	Transcribe(ctx context.Context, pcm []int16, h Hint) (Result, error)
	Close() error
}

// Limits are the spec numbers every adapter enforces.
type Limits struct {
	Partial    time.Duration // voice.budgets.partialMs
	Final      time.Duration // voice.budgets.finalS
	IdleUnload time.Duration // voice.engines.sttIdleUnloadS
	Window     time.Duration // longest audio a partial re-decodes: voice.vad.maxSegmentS
}

// LimitsFromSpec reads Limits from the spec's voice section.
func LimitsFromSpec(sp *spec.OniSpec) Limits {
	v := sp.Voice
	return Limits{
		Partial:    time.Duration(v.Budgets.PartialMs * float64(time.Millisecond)),
		Final:      time.Duration(v.Budgets.FinalS * float64(time.Second)),
		IdleUnload: time.Duration(v.Engines.SttIdleUnloadS * float64(time.Second)),
		Window:     time.Duration(v.Vad.MaxSegmentS * float64(time.Second)),
	}
}

func (l Limits) check() error {
	if l.Partial <= 0 || l.Final <= 0 || l.IdleUnload <= 0 || l.Window <= 0 {
		return errors.New("stt: limits must all be positive")
	}
	return nil
}

func (l Limits) budget(p Pass) time.Duration {
	if p == Partial {
		return l.Partial
	}
	return l.Final
}

// bounded runs a decode under its pass budget. A decode that hits the
// budget reports ErrBudget; one the caller cancelled reports the caller's
// error.
func bounded(ctx context.Context, l Limits, p Pass, decode func(context.Context) (string, error)) (string, error) {
	dctx, cancel := context.WithTimeout(ctx, l.budget(p))
	defer cancel()
	text, err := decode(dctx)
	if err == nil {
		return text, nil
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if errors.Is(dctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("%w (%s %s)", ErrBudget, p, l.budget(p))
	}
	return "", err
}

var wordSplit = regexp.MustCompile(`[\p{L}\p{N}']+`)

// Vocabulary is the hint a decoder gets: names trimmed, control characters
// removed, duplicates and overlong entries dropped, any entry that holds
// the character's name as a word dropped (the decoder would otherwise
// bend ordinary words toward it), at most MaxVocab kept in order.
func Vocabulary(names []string, character string) []string {
	char := strings.ToLower(strings.TrimSpace(character))
	seen := map[string]bool{}
	var out []string
	for _, n := range names {
		n = strings.Join(strings.FieldsFunc(n, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }), " ")
		if n == "" || utf8.RuneCountInString(n) > maxVocabRunes {
			continue
		}
		key := strings.ToLower(n)
		if seen[key] || char != "" && holdsWord(key, char) {
			continue
		}
		seen[key] = true
		out = append(out, n)
		if len(out) == MaxVocab {
			break
		}
	}
	return out
}

func holdsWord(text, name string) bool {
	nameWords := wordSplit.FindAllString(name, -1)
	if len(nameWords) == 0 {
		return false
	}
	words := wordSplit.FindAllString(text, -1)
	for i := 0; i+len(nameWords) <= len(words); i++ {
		match := true
		for j, w := range nameWords {
			if words[i+j] != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// AudioCtx is the encoder context whisper decodes a clip of d with: about
// 50 frames a second plus a margin, rounded up to 64, never below 256
// (shorter contexts loop on a word cut mid-way) nor above the model's 1500.
func AudioCtx(d time.Duration) int {
	ms := float64(d) / float64(time.Millisecond)
	n := int(math.Ceil((ms/1000*50+48)/64)) * 64
	return max(256, min(1500, n))
}

func duration(samples int) time.Duration {
	return time.Duration(samples) * time.Second / SampleRate
}

// writeWAV writes pcm as a 16 kHz mono s16 RIFF file.
func writeWAV(w io.Writer, pcm []int16) error {
	data := uint32(len(pcm) * 2)
	var h [44]byte
	copy(h[0:], "RIFF")
	binary.LittleEndian.PutUint32(h[4:], 36+data)
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], 1)
	binary.LittleEndian.PutUint32(h[24:], SampleRate)
	binary.LittleEndian.PutUint32(h[28:], SampleRate*2)
	binary.LittleEndian.PutUint16(h[32:], 2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], data)
	if _, err := w.Write(h[:]); err != nil {
		return err
	}
	buf := make([]byte, 2*len(pcm))
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(buf[2*i:], uint16(s))
	}
	_, err := w.Write(buf)
	return err
}

// pinned verifies a model file against its SHA-256 and remembers the
// file's identity, so an unchanged file is not hashed again.
type pinned struct {
	mu    sync.Mutex
	path  string
	want  string
	stamp string
}

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

func newPinned(path, sum string) (*pinned, error) {
	if path == "" || !sha256Hex.MatchString(sum) {
		return nil, fmt.Errorf("%w: a model needs a path and a pinned sha256", ErrModel)
	}
	return &pinned{path: path, want: sum}, nil
}

func (p *pinned) verify() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	info, err := os.Stat(p.path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not on disk", ErrModel, p.path)
	}
	stamp := fmt.Sprintf("%d/%d", info.Size(), info.ModTime().UnixNano())
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		stamp += fmt.Sprintf("/%d/%d/%d.%d", st.Dev, st.Ino, st.Ctim.Sec, st.Ctim.Nsec)
	}
	if stamp == p.stamp {
		return nil
	}
	f, err := os.Open(p.path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrModel, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("%w: %v", ErrModel, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != p.want {
		p.stamp = ""
		return fmt.Errorf("%w: %s hashes to %s, pinned %s", ErrModel, p.path, got, p.want)
	}
	p.stamp = stamp
	return nil
}

// Clock is the time an adapter reads and the idle timer it arms; tests
// inject one.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

// Timer is an armed idle timer.
type Timer interface{ Stop() bool }

type wallClock struct{}

func (wallClock) Now() time.Time                            { return time.Now() }
func (wallClock) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }
