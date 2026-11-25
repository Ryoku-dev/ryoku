package input

import (
	"context"
	"fmt"
	"time"
	"unicode"
	"unicode/utf8"
)

// DefaultChunkTimeout bounds one Tool call.
const DefaultChunkTimeout = 5 * time.Second

// Keyboard types and presses keys through Run on one display.
type Keyboard struct {
	Run Runner
	// ChunkTimeout bounds each Tool call; 0 means DefaultChunkTimeout.
	ChunkTimeout time.Duration
}

// Typed is what a typing call got done.
type Typed struct {
	// Landed is the exact number of characters (runes) in chunks the tool
	// finished.
	Landed int
	// Rest is the text not confirmed typed, byte for byte: text minus its
	// first Landed characters.
	Rest string
	// Unsure is the size, in characters, of the chunk the tool failed on:
	// some of it may have reached the window. It is 0 unless the error is
	// ErrTool. Rest still starts at that chunk.
	Unsure int
}

// Verify is a caller's check that the target is still the one it chose,
// still focused and still its own; any error stops typing.
type Verify func(ctx context.Context) error

// Type types text in chunks of at most MaxChunk characters, with no check
// around them. It is for a display Hyakki owns, where nothing else has the
// keyboard. Text that fails Check (the function) types nothing.
func (k Keyboard) Type(ctx context.Context, text string) (Typed, error) {
	return k.typeText(ctx, text, nil)
}

// TypeChecked types text into a live window: verify runs before every chunk
// and again after it, and the first failure stops typing. A failure after a
// chunk leaves that chunk counted in Landed, since the tool finished it.
// The error is verify's own, unwrapped.
func (k Keyboard) TypeChecked(ctx context.Context, text string, verify Verify) (Typed, error) {
	if verify == nil {
		return Typed{Rest: text}, ErrNoCheck
	}
	return k.typeText(ctx, text, verify)
}

func (k Keyboard) typeText(ctx context.Context, text string, verify Verify) (Typed, error) {
	res := Typed{Rest: text}
	if text == "" {
		return res, nil
	}
	if err := Check(text); err != nil {
		return res, err
	}
	if k.Run == nil {
		return res, ErrNoRunner
	}
	off := 0
	for _, c := range Chunks(text, MaxChunk) {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if verify != nil {
			if err := verify(ctx); err != nil {
				return res, err
			}
		}
		n := utf8.RuneCountInString(c)
		if err := k.run(ctx, "-s", Settle, "--", c); err != nil {
			res.Unsure = n
			return res, err
		}
		off += len(c)
		res.Landed += n
		res.Rest = text[off:]
		if verify != nil {
			if err := verify(ctx); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

// Chord presses one chord, such as ctrl+l or Return (see KeyArgs).
func (k Keyboard) Chord(ctx context.Context, chord string) error {
	args, err := KeyArgs(chord)
	if err != nil {
		return err
	}
	if k.Run == nil {
		return ErrNoRunner
	}
	return k.run(ctx, append([]string{"-s", Settle}, args...)...)
}

func (k Keyboard) run(ctx context.Context, args ...string) error {
	if !Allowed(Tool, args...) {
		return ErrNotAllowed
	}
	timeout := k.ChunkTimeout
	if timeout <= 0 {
		timeout = DefaultChunkTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := k.Run(cctx, Tool, args...); err != nil {
		return fmt.Errorf("%w: %w", ErrTool, err)
	}
	return nil
}

// Chunks splits text into pieces of at most max runes whose concatenation
// is text. A piece never ends inside a rune and, where the cluster fits,
// never inside a user-perceived character: combining marks, variation
// selectors, emoji modifiers, tag sequences, zero-width-joiner sequences and
// flag pairs stay with the character before them.
func Chunks(text string, max int) []string {
	if max < 1 {
		max = 1
	}
	var out []string
	start, n := 0, 0
	for i := 0; i < len(text); {
		end, runes := cluster(text, i)
		if n > 0 && n+runes > max {
			out = append(out, text[start:i])
			start, n = i, 0
		}
		if runes > max {
			// A cluster longer than a chunk is split on rune boundaries; the
			// chunk is empty here, since the check above flushed it.
			k := 0
			for j := i; j < end; {
				if k == max {
					out = append(out, text[start:j])
					start, k = j, 0
				}
				_, size := utf8.DecodeRuneInString(text[j:])
				j += size
				k++
			}
			n, i = k, end
			continue
		}
		n += runes
		i = end
	}
	if start < len(text) {
		out = append(out, text[start:])
	}
	return out
}

const zwj = '\u200d'

// cluster returns the end of the cluster starting at byte i and its length
// in runes.
func cluster(s string, i int) (int, int) {
	r, size := utf8.DecodeRuneInString(s[i:])
	end, n := i+size, 1
	prev := r
	flags := 0
	if isRegional(r) {
		flags = 1
	}
	for end < len(s) {
		next, size := utf8.DecodeRuneInString(s[end:])
		switch {
		case prev == zwj, extends(next):
		case isRegional(next) && flags == 1:
			flags = 2
		default:
			return end, n
		}
		end += size
		n++
		prev = next
	}
	return end, n
}

func extends(r rune) bool {
	switch {
	case r == zwj,
		unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc),
		r >= 0xfe00 && r <= 0xfe0f,
		r >= 0xe0100 && r <= 0xe01ef,
		r >= 0x1f3fb && r <= 0x1f3ff,
		r >= 0xe0020 && r <= 0xe007f:
		return true
	}
	return false
}

func isRegional(r rune) bool { return r >= 0x1f1e6 && r <= 0x1f1ff }
