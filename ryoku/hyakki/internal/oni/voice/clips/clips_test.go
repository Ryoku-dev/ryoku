package clips

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/profile"
)

const fixtureRate = 16000

// wav16 encodes mono 16-bit PCM.
func wav16(rate int, s []int16) []byte {
	return wavBytes(rate, 1, 1, 16, func(b []byte) []byte {
		for _, v := range s {
			b = binary.LittleEndian.AppendUint16(b, uint16(v))
		}
		return b
	}, len(s)*2)
}

func wavBytes(rate, format, channels, bits int, body func([]byte) []byte, size int) []byte {
	b := []byte("RIFF")
	b = binary.LittleEndian.AppendUint32(b, uint32(36+size))
	b = append(b, "WAVEfmt "...)
	b = binary.LittleEndian.AppendUint32(b, 16)
	b = binary.LittleEndian.AppendUint16(b, uint16(format))
	b = binary.LittleEndian.AppendUint16(b, uint16(channels))
	b = binary.LittleEndian.AppendUint32(b, uint32(rate))
	b = binary.LittleEndian.AppendUint32(b, uint32(rate*channels*bits/8))
	b = binary.LittleEndian.AppendUint16(b, uint16(channels*bits/8))
	b = binary.LittleEndian.AppendUint16(b, uint16(bits))
	b = append(b, "data"...)
	b = binary.LittleEndian.AppendUint32(b, uint32(size))
	return body(b)
}

// level is the constant sample value a fixture clip file holds, so joined
// PCM shows which file played where.
func level(n int) int16 { return int16(100 * (n + 1)) }

// fixture is a clip pack generated at test time: every clip the composer
// needs plus a few lines, each a 100 ms constant level, with comma, final
// and fast variants on some.
type fixture struct {
	clips  map[string]fixClip
	levels map[string]int16 // file -> level
	skip   []string         // ids left out
}

// fixClip is one manifest clip as the fixture writes it.
type fixClip struct {
	File, Text, Comma, Final, Fast string
}

func newFixture() *fixture {
	f := &fixture{clips: map[string]fixClip{}, levels: map[string]int16{}}
	var ids []string
	ids = append(ids, ones...)
	for _, t := range tens {
		if t != "" {
			ids = append(ids, t)
		}
	}
	for _, t := range nths {
		if t != "" {
			ids = append(ids, t)
		}
	}
	ids = append(ids, days...)
	for _, m := range months {
		if m != "" {
			ids = append(ids, m)
		}
	}
	ids = append(ids, Hundred, Past, OClock, "minute", "minutes", "its", "on-it", "on")
	for _, id := range ids {
		f.clips[id] = fixClip{File: id + ".wav"}
	}
	f.clips["its"] = fixClip{File: "its.wav", Text: "it's", Comma: "its-comma.wav"}
	f.clips[OClock] = fixClip{File: "oclock.wav", Text: "o'clock", Final: "oclock-final.wav"}
	f.clips["three"] = fixClip{File: "three.wav", Final: "three-final.wav"}
	f.clips["first"] = fixClip{File: "first.wav", Final: "first-final.wav"}
	f.clips["monday"] = fixClip{File: "monday.wav", Comma: "monday-comma.wav"}
	f.clips["forty"] = fixClip{File: "forty.wav", Fast: "forty-fast.wav"}
	return f
}

func (f *fixture) write(t *testing.T, dir string, extra map[string][]byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := map[string]any{"version": 1}
	out := map[string]any{}
	var files []string
	for id, e := range f.clips {
		if slices.Contains(f.skip, id) {
			continue
		}
		out[id] = e
		files = append(files, e.File, e.Comma, e.Final, e.Fast)
	}
	m["clips"] = out
	slices.Sort(files)
	for _, file := range slices.Compact(files) {
		if file == "" {
			continue
		}
		lv := level(len(f.levels))
		f.levels[file] = lv
		pcm := make([]int16, fixtureRate/10)
		for i := range pcm {
			pcm[i] = lv
		}
		writeFile(t, filepath.Join(dir, file), wav16(fixtureRate, pcm))
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, Manifest), data)
	writeFile(t, filepath.Join(dir, "LICENSE"), []byte("CC0-1.0\n"))
	writeFile(t, filepath.Join(dir, "PROVENANCE.md"), []byte("Generated tones for this test.\n"))
	for name, data := range extra {
		p := filepath.Join(dir, name)
		if data == nil {
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			continue
		}
		writeFile(t, p, data)
	}
}

func (e fixClip) MarshalJSON() ([]byte, error) {
	type plain struct {
		File  string `json:"file"`
		Text  string `json:"text,omitempty"`
		Comma string `json:"comma,omitempty"`
		Final string `json:"final,omitempty"`
		Fast  string `json:"fast,omitempty"`
	}
	if e.Text == "" && e.Comma == "" && e.Final == "" && e.Fast == "" {
		return json.Marshal(e.File)
	}
	return json.Marshal(plain(e))
}

func writeFile(t *testing.T, p string, data []byte) {
	t.Helper()
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func openFixture(t *testing.T, f *fixture) *Pack {
	t.Helper()
	dir := t.TempDir()
	f.write(t, filepath.Join(dir, "voice"), nil)
	p, err := Open(os.DirFS(dir), "voice")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestComposeTimes(t *testing.T) {
	p := openFixture(t, newFixture())
	cases := []struct {
		name string
		u    Utterance
		want []string
	}{
		{"quarter to four as minutes past", Utterance{LeadIn: "its", Phrases: []Phrase{Time{15, 45}}},
			[]string{"its.comma", "forty", "five", "past", "three.final"}},
		{"on the hour", Utterance{Phrases: []Phrase{Time{9, 0}}}, []string{"nine", "oclock.final"}},
		{"midnight hour", Utterance{Phrases: []Phrase{Time{0, 7}}}, []string{"seven", "past", "twelve"}},
		{"noon hour", Utterance{Phrases: []Phrase{Time{12, 30}}}, []string{"thirty", "past", "twelve"}},
		{"teen minutes", Utterance{Phrases: []Phrase{Time{23, 13}}}, []string{"thirteen", "past", "eleven"}},
		{"fast run", Utterance{Fast: true, Phrases: []Phrase{Time{15, 45}}},
			[]string{"forty.fast", "five", "past", "three.final"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := p.Compose(c.u)
			if err != nil || !slices.Equal(got, c.want) {
				t.Fatalf("Compose = %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

func TestComposeDates(t *testing.T) {
	p := openFixture(t, newFixture())
	got, err := p.Compose(Utterance{Phrases: []Phrase{Day(time.Monday), Date{time.March, 21}}})
	if want := []string{"monday.comma", "march", "twenty", "first.final"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("Monday, March 21 = %q, %v; want %q", got, err, want)
	}
	got, err = p.Compose(Utterance{Phrases: []Phrase{Date{time.December, 2}}})
	if want := []string{"december", "second"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("December 2 = %q, %v; want %q", got, err, want)
	}
	for d := time.Sunday; d <= time.Saturday; d++ {
		got, err := p.Compose(Utterance{Phrases: []Phrase{Day(d), Word("on")}})
		want := strings.ToLower(d.String())
		if d == time.Monday {
			want += ".comma"
		}
		if err != nil || len(got) != 2 || got[0] != want {
			t.Fatalf("%v = %q, %v", d, got, err)
		}
	}
	for m := time.January; m <= time.December; m++ {
		got, err := p.Compose(Utterance{Phrases: []Phrase{Date{m, 10}}})
		if err != nil || !slices.Equal(got, []string{strings.ToLower(m.String()), "tenth"}) {
			t.Fatalf("%v 10 = %q, %v", m, got, err)
		}
	}
}

func TestComposeCounts(t *testing.T) {
	p := openFixture(t, newFixture())
	spot := map[int][]string{
		0: {"zero"}, 7: {"seven"}, 13: {"thirteen"}, 19: {"nineteen"}, 20: {"twenty"},
		21: {"twenty", "one"}, 45: {"forty.fast", "five"}, 70: {"seventy"}, 99: {"ninety", "nine"},
		100: {"one", "hundred"},
	}
	for n := 0; n <= 100; n++ {
		got, err := p.Compose(Utterance{Fast: true, Phrases: []Phrase{Number(n), Word("on")}})
		if err != nil {
			t.Fatalf("%d: %v", n, err)
		}
		got = got[:len(got)-1]
		wantLen := 2
		if n < 20 || (n < 100 && n%10 == 0) {
			wantLen = 1
		}
		if len(got) != wantLen {
			t.Fatalf("%d = %q: want %d clips", n, got, wantLen)
		}
		if want, ok := spot[n]; ok && !slices.Equal(got, want) {
			t.Fatalf("%d = %q, want %q", n, got, want)
		}
	}
	ordinals := map[int][]string{
		1: {"first.final"}, 3: {"third"}, 12: {"twelfth"}, 20: {"twentieth"},
		22: {"twenty", "second"}, 30: {"thirtieth"}, 31: {"thirty", "first.final"},
	}
	for n := 1; n <= 31; n++ {
		got, err := p.Compose(Utterance{Phrases: []Phrase{Ordinal(n)}})
		if err != nil {
			t.Fatalf("ordinal %d: %v", n, err)
		}
		if want, ok := ordinals[n]; ok && !slices.Equal(got, want) {
			t.Fatalf("ordinal %d = %q, want %q", n, got, want)
		}
	}
	for _, c := range []struct {
		n    int
		want []string
	}{{1, []string{"one", "minute"}}, {5, []string{"five", "minutes"}}, {0, []string{"zero", "minutes"}}} {
		got, err := p.Compose(Utterance{Phrases: []Phrase{Count{N: c.n, One: "minute", Many: "minutes"}}})
		if err != nil || !slices.Equal(got, c.want) {
			t.Fatalf("count %d = %q, %v; want %q", c.n, got, err, c.want)
		}
	}
	for _, ph := range []Phrase{Number(-1), Number(101), Ordinal(0), Ordinal(32), Time{24, 0}, Time{1, 60},
		Date{13, 1}, Date{time.May, 32}, Day(7), Count{N: 101}} {
		if got, err := p.Compose(Utterance{Phrases: []Phrase{ph}}); err == nil {
			t.Fatalf("%#v composed %q", ph, got)
		}
	}
}

func TestComposeFailsOnAMissingClip(t *testing.T) {
	f := newFixture()
	f.skip = []string{"fifty"}
	p := openFixture(t, f)
	if _, err := p.Compose(Utterance{Phrases: []Phrase{Number(49)}}); err != nil {
		t.Fatalf("49 needs no fifty: %v", err)
	}
	for _, u := range []Utterance{
		{Phrases: []Phrase{Number(52)}},
		{Phrases: []Phrase{Time{10, 50}}},
		{LeadIn: "the-time-is", Phrases: []Phrase{Time{10, 0}}},
		{Phrases: []Phrase{Word("firefox")}},
	} {
		got, err := p.Compose(u)
		if !errors.Is(err, ErrMissingClip) || got != nil {
			t.Fatalf("%+v = %q, %v; want ErrMissingClip", u, got, err)
		}
	}
	_, err := p.Compose(Utterance{Phrases: []Phrase{Number(52)}})
	if !strings.Contains(err.Error(), `"fifty"`) {
		t.Fatalf("the error does not name the clip: %v", err)
	}
}

func TestSpeakReadsTimesDatesAndLines(t *testing.T) {
	p := openFixture(t, newFixture())
	cases := map[string][]string{
		"It's 3:45.":            {"its", "forty", "five", "past", "three.final"},
		"Monday, March 21st.":   {"monday.comma", "march", "twenty", "first.final"},
		"On it.":                {"on-it"},
		"It’s 9:00":             {"its", "nine", "oclock.final"},
		"It's, 15 minutes":      {"its.comma", "fifteen", "minutes"},
		"forty five past three": {"forty", "five", "past", "three.final"},
	}
	for line, want := range cases {
		got, err := p.Speak(line)
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("Speak(%q) = %q, %v; want %q", line, got, err, want)
		}
	}
	for _, line := range []string{"Open Firefox.", "It's 3:45 in Paris", "", "...", "It's 101"} {
		if got, err := p.Speak(line); err == nil {
			t.Fatalf("Speak(%q) = %q, want an error", line, got)
		}
	}
	if _, err := p.Speak("Open Firefox."); !errors.Is(err, ErrMissingClip) {
		t.Fatalf("a missing word is not ErrMissingClip: %v", err)
	}
}

func TestPCMJoinsTheChosenFiles(t *testing.T) {
	f := newFixture()
	p := openFixture(t, f)
	refs, err := p.Compose(Utterance{LeadIn: "its", Phrases: []Phrase{Time{15, 45}}})
	if err != nil {
		t.Fatal(err)
	}
	const rate = 24000
	pcm, err := p.PCM(refs, rate)
	if err != nil {
		t.Fatal(err)
	}
	per := rate / 10
	if len(pcm) != per*len(refs) {
		t.Fatalf("%d samples for %d clips, want %d each", len(pcm), len(refs), per)
	}
	files := []string{"its-comma.wav", "forty.wav", "five.wav", "past.wav", "three-final.wav"}
	for i, file := range files {
		want := float32(f.levels[file]) / 32768
		for _, v := range pcm[i*per : (i+1)*per] {
			if math.Abs(float64(v-want)) > 1e-6 {
				t.Fatalf("clip %d (%s) holds %v, want %v", i, file, v, want)
			}
		}
	}
	if _, err := p.PCM([]string{"three.comma"}, rate); !errors.Is(err, ErrMissingClip) {
		t.Fatalf("an absent variant: %v", err)
	}
}

func TestOpenRefusesWhatTheProfileLoaderRefuses(t *testing.T) {
	link := func(t *testing.T, dir string) {
		if err := os.Symlink(filepath.Join(dir, "LICENSE"), filepath.Join(dir, "extra.wav")); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name  string
		extra map[string][]byte
		after func(*testing.T, string)
		ok    bool
	}{
		{name: "complete", ok: true},
		{name: "COPYING license", extra: map[string][]byte{"LICENSE": nil, "COPYING": []byte("GPL\n")}, ok: true},
		{name: "no license", extra: map[string][]byte{"LICENSE": nil}},
		{name: "empty license", extra: map[string][]byte{"LICENSE": {}}},
		{name: "no provenance", extra: map[string][]byte{"PROVENANCE.md": nil}},
		{name: "empty provenance", extra: map[string][]byte{"PROVENANCE.md": {}}},
		{name: "symlink inside", after: link},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			moth := copyMoth(t, root)
			pack := filepath.Join(moth, "voice")
			newFixture().write(t, pack, c.extra)
			if c.after != nil {
				c.after(t, pack)
			}
			_, err := ForProfile(os.DirFS(root), "moth", profile.Voice{Engine: profile.VoiceClips, ClipPack: "voice/"})
			loaded, report := profile.Load(os.DirFS(root), "moth")
			if (err == nil) != c.ok || loaded.Valid != c.ok {
				t.Fatalf("Open err=%v, profile valid=%v report=%+v; want ok=%v", err, loaded.Valid, report, c.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalid) {
				t.Fatalf("not ErrInvalid: %v", err)
			}
		})
	}
}

// copyMoth copies the profile loader's test character into root/moth set to
// the clips engine with its pack in voice/.
func copyMoth(t *testing.T, root string) string {
	t.Helper()
	src := os.DirFS(filepath.Join("..", "..", "profile", "testdata", "testmoth"))
	dst := filepath.Join(root, "moth")
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, p), 0o755)
		}
		data, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		if p == "profile.json" {
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				return err
			}
			m["id"] = "moth"
			m["voice"] = map[string]any{"engine": profile.VoiceClips, "clipPack": "voice/", "speed": 1, "honorific": "none"}
			if data, err = json.Marshal(m); err != nil {
				return err
			}
		}
		return os.WriteFile(filepath.Join(dst, p), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestOpenChecksTheManifest(t *testing.T) {
	bad := func(m string) map[string][]byte { return map[string][]byte{Manifest: []byte(m)} }
	cases := map[string]func(*testing.T, string){
		"escaping file": func(t *testing.T, d string) { newFixture().write(t, d, bad(`{"version":1,"clips":{"a":"../a.wav"}}`)) },
		"missing file":  func(t *testing.T, d string) { newFixture().write(t, d, bad(`{"version":1,"clips":{"a":"gone.wav"}}`)) },
		"not wav":       func(t *testing.T, d string) { newFixture().write(t, d, bad(`{"version":1,"clips":{"a":"LICENSE"}}`)) },
		"unknown field": func(t *testing.T, d string) {
			newFixture().write(t, d, bad(`{"version":1,"clips":{"a":{"file":"on.wav","loud":true}}}`))
		},
		"version": func(t *testing.T, d string) { newFixture().write(t, d, bad(`{"version":2,"clips":{}}`)) },
		"bad id": func(t *testing.T, d string) {
			newFixture().write(t, d, bad(`{"version":1,"clips":{"On It":"on.wav"}}`))
		},
		"duplicate text": func(t *testing.T, d string) {
			newFixture().write(t, d, bad(`{"version":1,"clips":{"a":{"file":"on.wav","text":"on"},"on":"on.wav"}}`))
		},
		"no manifest": func(t *testing.T, d string) { newFixture().write(t, d, map[string][]byte{Manifest: nil}) },
		"symlinked pack": func(t *testing.T, d string) {
			real := d + "-real"
			newFixture().write(t, real, nil)
			if err := os.Symlink(real, d); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			setup(t, filepath.Join(root, "voice"))
			if p, err := Open(os.DirFS(root), "voice"); err == nil || !errors.Is(err, ErrInvalid) {
				t.Fatalf("Open = %v, %v; want ErrInvalid", p, err)
			}
		})
	}
}

func TestDecodeWAV(t *testing.T) {
	pcm, rate, err := DecodeWAV(wav16(22050, []int16{0, 16384, -16384, 32767}))
	if err != nil || rate != 22050 || !slices.Equal(pcm, []float32{0, 0.5, -0.5, 32767.0 / 32768}) {
		t.Fatalf("16-bit = %v %d %v", pcm, rate, err)
	}
	stereo := wavBytes(48000, wavFloat, 2, 32, func(b []byte) []byte {
		for _, v := range []float32{0.5, 0.25, -1, 1} {
			b = binary.LittleEndian.AppendUint32(b, math.Float32bits(v))
		}
		return b
	}, 16)
	if pcm, rate, err := DecodeWAV(stereo); err != nil || rate != 48000 || !slices.Equal(pcm, []float32{0.375, 0}) {
		t.Fatalf("float stereo = %v %d %v", pcm, rate, err)
	}
	pcm24 := wavBytes(16000, wavPCM, 1, 24, func(b []byte) []byte { return append(b, 0, 0, 0x40, 0, 0, 0xc0) }, 6)
	if pcm, _, err := DecodeWAV(pcm24); err != nil || !slices.Equal(pcm, []float32{0.5, -0.5}) {
		t.Fatalf("24-bit = %v %v", pcm, err)
	}
	// A streamed header claims more data than follows.
	streamed := wav16(22050, []int16{1000, 2000, 3000})
	binary.LittleEndian.PutUint32(streamed[40:], 0x7ffff000)
	if pcm, _, err := DecodeWAV(streamed); err != nil || len(pcm) != 3 {
		t.Fatalf("streamed = %v %v", pcm, err)
	}
	for name, b := range map[string][]byte{
		"empty":      nil,
		"not riff":   []byte("OggS0000WAVEfmt "),
		"8 channel+": wavBytes(16000, wavPCM, 9, 16, func(b []byte) []byte { return append(b, make([]byte, 18)...) }, 18),
		"mu-law":     wavBytes(8000, 7, 1, 8, func(b []byte) []byte { return append(b, 1, 2) }, 2),
		"no data":    wav16(16000, nil)[:36],
	} {
		if _, _, err := DecodeWAV(b); err == nil {
			t.Fatalf("%s decoded", name)
		}
	}
}

func TestResampleKeepsPitchAndLength(t *testing.T) {
	const from, to, hz = 22050, 24000, 440.0
	in := make([]float32, from)
	for i := range in {
		in[i] = float32(math.Sin(2 * math.Pi * hz * float64(i) / from))
	}
	for _, c := range []struct{ from, to int }{{from, to}, {48000, to}} {
		src := in
		if c.from == 48000 {
			src = make([]float32, 48000)
			for i := range src {
				src[i] = float32(math.Sin(2 * math.Pi * hz * float64(i) / 48000))
			}
		}
		out := Resample(src, c.from, c.to)
		if len(out) != c.to {
			t.Fatalf("%d->%d: %d samples for one second", c.from, c.to, len(out))
		}
		crossings := 0
		for i := 1; i < len(out); i++ {
			if (out[i-1] < 0) != (out[i] < 0) {
				crossings++
			}
		}
		if crossings < 2*hz-4 || crossings > 2*hz+4 {
			t.Fatalf("%d->%d: %d zero crossings, want about %v", c.from, c.to, crossings, 2*hz)
		}
	}
	if out := Resample(in[:10], 24000, 24000); &out[0] != &in[0] {
		t.Fatal("equal rates copied")
	}
}
