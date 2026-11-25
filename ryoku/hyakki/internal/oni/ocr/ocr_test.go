package ocr

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/screenask"
)

func picture(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range m.Pix {
		m.Pix[i] = 255
	}
	m.SetRGBA(1, 1, color.RGBA{0, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type call struct {
	name  string
	args  []string
	stdin []byte
}

type fake struct {
	calls []call
	out   string
	err   error
}

func (f *fake) run(_ context.Context, name string, args []string, stdin []byte) ([]byte, error) {
	f.calls = append(f.calls, call{name, slices.Clone(args), slices.Clone(stdin)})
	if f.err != nil {
		return nil, f.err
	}
	return []byte(f.out), nil
}

func tsv(rows ...string) string {
	return tsvHeader + "\n" + strings.Join(rows, "\n") + "\n"
}

func TestAllowlistAdmitsNothingElse(t *testing.T) {
	ok := Args("eng")
	with := func(i int, v string) []string { a := slices.Clone(ok); a[i] = v; return a }
	for _, lang := range []string{"eng", "chi_sim", "eng+deu", "osd"} {
		if !Allowed("tesseract", Args(lang)...) {
			t.Errorf("language %q refused", lang)
		}
	}
	bad := map[string]struct {
		name string
		args []string
	}{
		"other program":       {"grim", ok},
		"path to the program": {"/usr/bin/tesseract", ok},
		"input file":          {"tesseract", with(0, "/tmp/shot.png")},
		"output base":         {"tesseract", with(1, "/tmp/out")},
		"flag as language":    {"tesseract", with(3, "--tessdata-dir")},
		"path as language":    {"tesseract", with(3, "../eng")},
		"empty language":      {"tesseract", with(3, "")},
		"dangling plus":       {"tesseract", with(3, "eng+")},
		"shell in language":   {"tesseract", with(3, "eng;rm")},
		"long language":       {"tesseract", with(3, strings.Repeat("e", 65))},
		"other segmentation":  {"tesseract", with(5, "3")},
		"other config":        {"tesseract", with(6, "hocr")},
		"text config":         {"tesseract", with(6, "txt")},
		"extra argument":      {"tesseract", append(slices.Clone(ok), "pdf")},
		"missing argument":    {"tesseract", ok[:6]},
		"user words":          {"tesseract", []string{"stdin", "stdout", "--user-words", "/x", "--psm", "11", "tsv"}},
	}
	for name, c := range bad {
		if Allowed(c.name, c.args...) {
			t.Errorf("%s: %s %q admitted", name, c.name, c.args)
		}
	}
	f := &fake{out: tsv()}
	if _, err := New(f.run, "../eng").Words(context.Background(), picture(t, 4, 4)); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("a path as language gave %v", err)
	}
	if len(f.calls) != 0 {
		t.Fatal("a refused argv still ran")
	}
}

func TestWordsAreFractionsOfThePicture(t *testing.T) {
	f := &fake{out: tsv(
		"1\t1\t0\t0\t0\t0\t0\t0\t200\t100\t-1\t",
		"2\t1\t1\t0\t0\t0\t10\t20\t150\t30\t-1\t",
		"5\t1\t1\t1\t1\t1\t10\t20\t40\t10\t95.5\tSave",
		"5\t1\t1\t1\t1\t2\t60\t20\t30\t10\t91\tAs…",
		"5\t1\t1\t1\t1\t3\t100\t20\t30\t10\t59.9\tblur",
		"5\t1\t1\t1\t1\t4\t140\t20\t2\t30\t97\t|",
		"5\t1\t1\t1\t1\t5\t180\t90\t40\t20\t88\tEdge",
		"5\t1\t1\t1\t1\t6\t0\t0\t10\t10\t90\tA\u200bB\x07",
		"5\t1\t1\t1\t1\t7\t0\t0\t10\t10\t-1\t",
	)}
	got, err := parse([]byte(f.out), 200, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []screenask.Word{
		{Text: "Save", Box: feature.Box{X: 0.05, Y: 0.2, W: 0.2, H: 0.1}},
		{Text: "As…", Box: feature.Box{X: 0.3, Y: 0.2, W: 0.15, H: 0.1}},
		{Text: "Edge", Box: feature.Box{X: 0.9, Y: 0.9, W: 0.1, H: 0.1}},
		{Text: "AB", Box: feature.Box{X: 0, Y: 0, W: 0.05, H: 0.1}},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("words\n got %+v\nwant %+v", got, want)
	}
}

func TestRecognitionBorderNeverMovesWordsIntoTheSource(t *testing.T) {
	rows := tsv(
		"5\t1\t1\t1\t1\t1\t0\t0\t10\t10\t95\tBorder",
		"5\t1\t1\t1\t1\t2\t40\t70\t120\t30\t95\tSave",
		"5\t1\t1\t1\t1\t3\t5\t5\t35\t35\t95\tEdge",
	)
	got, err := parse([]byte(rows), 600, 300, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []screenask.Word{
		{Text: "Save", Box: feature.Box{X: 0.05, Y: 0.2, W: 0.2, H: 0.1}},
		{Text: "Edge", Box: feature.Box{X: 0, Y: 0, W: 0.05, H: 0.1}},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("recognition border leaked into original coordinates: got %+v, want %+v", got, want)
	}
}

func TestWordsAreCapped(t *testing.T) {
	rows := make([]string, 0, MaxWords+5)
	for range MaxWords + 5 {
		rows = append(rows, "5\t1\t1\t1\t1\t1\t0\t0\t1\t1\t90\t"+strings.Repeat("x", MaxWordRunes+9))
	}
	got, err := parse([]byte(tsv(rows...)), 10, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != MaxWords || len([]rune(got[0].Text)) != MaxWordRunes {
		t.Fatalf("%d words of %d runes, want %d of %d", len(got), len([]rune(got[0].Text)), MaxWords, MaxWordRunes)
	}
}

func TestMalformedAnswersAndPicturesAreRefused(t *testing.T) {
	for name, out := range map[string]string{
		"no header":       "5\t1\t1\t1\t1\t1\t0\t0\t1\t1\t90\tSave\n",
		"empty":           "",
		"short row":       tsv("5\t1\t1\t1\t1\t1\t0\t0\t1\t90\tSave"),
		"word box text":   tsv("5\t1\t1\t1\t1\t1\tten\t0\t1\t1\t90\tSave"),
		"confidence text": tsv("5\t1\t1\t1\t1\t1\t0\t0\t1\t1\thigh\tSave"),
	} {
		if _, err := New((&fake{out: out}).run, "eng").Words(context.Background(), picture(t, 10, 10)); !errors.Is(err, ErrBadOutput) {
			t.Errorf("%s: %v", name, err)
		}
	}
	f := &fake{out: tsv()}
	for name, pic := range map[string][]byte{"not a png": []byte("GIF89a"), "empty": nil} {
		if _, err := New(f.run, "eng").Words(context.Background(), pic); !errors.Is(err, ErrBadImage) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(f.calls) != 0 {
		t.Fatal("a picture that is not a PNG reached the reader")
	}
}

func TestMissingReaderIsUnavailable(t *testing.T) {
	pic := picture(t, 10, 10)
	if _, err := New(nil, "eng").Words(context.Background(), pic); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no runner gave %v", err)
	}
	if _, err := New(Exec([]string{"PATH=" + t.TempDir()}), "eng").Words(context.Background(), pic); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("an empty PATH gave %v", err)
	}
	// a tesseract in the working directory is never found through a relative PATH entry
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tesseract"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if _, err := New(Exec([]string{"PATH=."}), "eng").Words(context.Background(), pic); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("a relative PATH gave %v", err)
	}
}

func TestDistributionReaderUsesOnlyItsOwnEnvironment(t *testing.T) {
	dir := t.TempDir()
	reader := filepath.Join(dir, "tesseract-ocr")
	body := "#!/bin/sh\nprintf '" + strings.ReplaceAll(tsvHeader, "\t", "\\t") + "\\n5\\t1\\t1\\t1\\t1\\t1\\t0\\t0\\t1\\t1\\t90\\tSave\\n'\n"
	if err := os.WriteFile(reader, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{"PATH=" + dir}
	out, err := Exec(env)(context.Background(), Tool, Args("eng"), picture(t, 8, 8))
	if err != nil {
		t.Fatal(err)
	}
	words, err := parse(out, 8, 8, 0)
	if err != nil || len(words) != 1 || words[0].Text != "Save" {
		t.Fatalf("distribution reader: %v, %v", words, err)
	}
	if err := os.WriteFile(filepath.Join(dir, Tool), []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Exec(env), "eng").Words(context.Background(), picture(t, 8, 8)); err == nil {
		t.Fatal("a failing preferred reader was replaced by another program")
	}
	t.Setenv("PATH", dir)
	if !Available() {
		t.Fatal("installed distribution reader was reported missing")
	}
	if _, err := New(Exec([]string{"PATH="}), "eng").Words(context.Background(), picture(t, 8, 8)); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("reader escaped its explicit PATH: %v", err)
	}
}

// stand puts a shell script named tesseract in a fresh directory and returns
// an environment whose PATH finds it first.
func stand(t *testing.T, body string) []string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tesseract"), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{"PATH=" + dir + ":/usr/bin:/bin", "LANG=C"}
}

func TestExecBoundsAndNamesFailures(t *testing.T) {
	pic := picture(t, 8, 8)
	cases := map[string]struct {
		body string
		want error
	}{
		"flood":            {"cat >/dev/null; head -c 5000000 /dev/zero\n", ErrTooLarge},
		"missing language": {"cat >/dev/null; echo \"Failed loading language 'eng'\" >&2; exit 1\n", ErrUnavailable},
	}
	for name, c := range cases {
		if _, err := New(Exec(stand(t, c.body)), "eng").Words(context.Background(), pic); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
	_, err := New(Exec(stand(t, "cat >/dev/null; echo broken >&2; exit 3\n")), "eng").Words(context.Background(), pic)
	if err == nil || errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("a failing reader gave %v", err)
	}
	if _, err := Exec(nil)(context.Background(), "sh", []string{"-c", "true"}, nil); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("Exec ran an argv off the allowlist: %v", err)
	}
}

func tree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != root {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
