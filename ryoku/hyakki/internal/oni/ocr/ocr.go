// Package ocr reads the words in a picture of a window with tesseract, so a
// screen-ask box can be checked against text that is really there. The
// picture goes to the reader on stdin and the words come back on stdout:
// nothing is written to disk, and nothing but one allowlisted argv is run.
package ocr

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	xdraw "golang.org/x/image/draw"

	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/screenask"
)

// Tool is the one program this package may start.
const Tool = "tesseract"

// Bounds on what a read keeps.
const (
	// MinConfidence is the lowest tesseract word confidence (0..100) kept.
	MinConfidence = 60
	// MaxWords caps the words one picture yields; the rest are dropped.
	MaxWords = 2000
	// MaxWordRunes caps one word's text.
	MaxWordRunes = 64
	// maxLang bounds a language selector such as "eng+deu".
	maxLang = 64
)

var (
	// ErrUnavailable: tesseract or the asked language is not installed.
	ErrUnavailable = errors.New("ocr: no text reader installed")
	// ErrNotAllowed is returned for any argv outside the allowlist.
	ErrNotAllowed = errors.New("ocr: command not in the text reader allowlist")
	// ErrBadImage: the picture is not a PNG this package will hand on.
	ErrBadImage = errors.New("ocr: the picture is not a readable PNG")
	// ErrBadOutput: the reader answered something that is not its TSV.
	ErrBadOutput = errors.New("ocr: the text reader's answer could not be read")
	// ErrTooLarge: the reader wrote more than MaxOutput.
	ErrTooLarge = errors.New("ocr: the text reader's answer was too large")
)

// Runner starts name with args, writes stdin to it and returns its stdout.
// Exec is the real one; tests pass a fake, so nothing is started.
type Runner func(ctx context.Context, name string, args []string, stdin []byte) ([]byte, error)

// Reader reads the words in one PNG picture. Each word's box is in
// fractions of the picture.
type Reader interface {
	Words(ctx context.Context, png []byte) ([]screenask.Word, error)
}

// New returns a Reader that runs tesseract through run for lang, such as
// "eng". A nil run reads nothing and answers ErrUnavailable.
func New(run Runner, lang string) Reader { return &reader{run: run, lang: lang} }

type reader struct {
	run  Runner
	lang string
}

// Args is the only argv Words starts: the PNG on stdin, sparse-text page
// segmentation (window text is scattered labels, not a page), word boxes
// as TSV on stdout.
func Args(lang string) []string {
	return []string{"stdin", "stdout", "-l", lang, "--psm", "11", "tsv"}
}

// Allowed reports whether argv is exactly Args for a well-formed language.
func Allowed(name string, args ...string) bool {
	if name != Tool || len(args) != 7 || !validLang(args[3]) {
		return false
	}
	want := Args(args[3])
	for i := range want {
		if args[i] != want[i] {
			return false
		}
	}
	return true
}

// validLang accepts tesseract language names, such as "eng" or "chi_sim",
// joined by "+", and nothing the reader could take as a flag or a path.
func validLang(lang string) bool {
	if lang == "" || len(lang) > maxLang {
		return false
	}
	for part := range strings.SplitSeq(lang, "+") {
		if part == "" {
			return false
		}
		for i := range len(part) {
			c := part[i]
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
				return false
			}
		}
	}
	return true
}

func (r *reader) Words(ctx context.Context, pic []byte) ([]screenask.Word, error) {
	if r.run == nil {
		return nil, ErrUnavailable
	}
	args := Args(r.lang)
	if !Allowed(Tool, args...) {
		return nil, ErrNotAllowed
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(pic))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > capture.MaxPixels/cfg.Height {
		return nil, ErrBadImage
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pic, scale, margin, err := recognitionPNG(pic, cfg.Width, cfg.Height)
	if err != nil {
		return nil, err
	}
	if margin != 0 {
		defer clear(pic[:cap(pic)])
	}
	out, err := r.run(ctx, Tool, args, pic)
	defer clear(out)
	if err != nil {
		return nil, err
	}
	return parse(out, cfg.Width*scale, cfg.Height*scale, margin)
}

// give small edge text room to read without lowering confidence
func recognitionPNG(pic []byte, w, h int) ([]byte, int, int, error) {
	const margin = 10
	scale := 2
	for scale > 0 && w*scale+2*margin > capture.MaxPixels/(h*scale+2*margin) {
		scale--
	}
	if scale == 0 {
		return pic, 1, 0, nil
	}
	src, err := png.Decode(bytes.NewReader(pic))
	if err != nil {
		return nil, 0, 0, ErrBadImage
	}
	defer clearImage(src)
	raster := image.NewRGBA(image.Rect(0, 0, w*scale+2*margin, h*scale+2*margin))
	defer clear(raster.Pix)
	background := image.NewUniform(src.At(0, 0))
	right, bottom := raster.Rect.Max.X, raster.Rect.Max.Y
	for _, edge := range [...]image.Rectangle{
		image.Rect(0, 0, right, margin),
		image.Rect(0, bottom-margin, right, bottom),
		image.Rect(0, margin, margin, bottom-margin),
		image.Rect(right-margin, margin, right, bottom-margin),
	} {
		draw.Draw(raster, edge, background, image.Point{}, draw.Src)
	}
	inside := image.Rect(margin, margin, margin+w*scale, margin+h*scale)
	xdraw.ApproxBiLinear.Scale(raster, inside, src, src.Bounds(), draw.Src, nil)
	out := &bounded{max: capture.MaxStdout, stop: func() {}}
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(out, raster); err != nil {
		out.clear()
		return nil, 0, 0, err
	}
	return out.buf, scale, margin, nil
}

func clearImage(pic image.Image) {
	switch m := pic.(type) {
	case *image.RGBA:
		clear(m.Pix)
	case *image.NRGBA:
		clear(m.Pix)
	case *image.RGBA64:
		clear(m.Pix)
	case *image.NRGBA64:
		clear(m.Pix)
	case *image.Gray:
		clear(m.Pix)
	case *image.Gray16:
		clear(m.Pix)
	case *image.Paletted:
		clear(m.Pix)
	}
}

// tsvHeader is the first line tesseract's tsv output starts with.
const tsvHeader = "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext"

// parse turns tesseract TSV into words. Only word rows (level 5) at or
// above MinConfidence with a letter or digit are kept; a box is clamped to
// the original w×h content and given in fractions of it; margin is the
// recognition-only border around that content.
func parse(tsv []byte, w, h, margin int) ([]screenask.Word, error) {
	header, rest, _ := bytes.Cut(tsv, []byte("\n"))
	if string(bytes.TrimRight(header, "\r")) != tsvHeader {
		return nil, ErrBadOutput
	}
	var words []screenask.Word
	for line := range bytes.SplitSeq(rest, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		if len(line) == 0 {
			continue
		}
		f := bytes.SplitN(line, []byte("\t"), 12)
		if len(f) != 12 {
			return nil, ErrBadOutput
		}
		if string(f[0]) != "5" {
			continue
		}
		var n [4]int
		for i := range n {
			v, err := strconv.Atoi(string(f[6+i]))
			if err != nil {
				return nil, ErrBadOutput
			}
			n[i] = v
		}
		conf, err := strconv.ParseFloat(string(f[10]), 64)
		if err != nil {
			return nil, ErrBadOutput
		}
		if !(conf >= MinConfidence) {
			continue
		}
		word := cleanWord(string(f[11]))
		if word == "" {
			continue
		}
		x0, y0 := min(max(n[0]-margin, 0), w), min(max(n[1]-margin, 0), h)
		x1, y1 := min(max(n[0]+n[2]-margin, 0), w), min(max(n[1]+n[3]-margin, 0), h)
		if x1 <= x0 || y1 <= y0 {
			continue
		}
		fw, fh := float64(w), float64(h)
		words = append(words, screenask.Word{Text: word, Box: feature.Box{
			X: float64(x0) / fw, Y: float64(y0) / fh,
			W: float64(x1-x0) / fw, H: float64(y1-y0) / fh,
		}})
		if len(words) == MaxWords {
			break
		}
	}
	return words, nil
}

// cleanWord drops invalid UTF-8, control and format runes, caps the length
// and keeps the word only when it holds a letter or a digit: a lone bar or
// dash is a border the reader mistook for text.
func cleanWord(s string) string {
	s = strings.ToValidUTF8(s, "")
	var b strings.Builder
	readable := false
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
			continue
		}
		if unicode.IsSpace(r) {
			r = ' '
		}
		readable = readable || unicode.IsLetter(r) || unicode.IsDigit(r)
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if !readable {
		return ""
	}
	if utf8.RuneCountInString(out) > MaxWordRunes {
		out = string([]rune(out)[:MaxWordRunes])
	}
	return out
}
