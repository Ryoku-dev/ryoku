package dictate

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "state", "hyakki", "dictation.json"))
}

func wantCode(t *testing.T, err error, code, reason string) {
	t.Helper()
	var pe *protocol.Error
	if !errors.As(err, &pe) || pe.Code != code || pe.Reason != reason {
		t.Fatalf("err = %#v, want code %q reason %q", err, code, reason)
	}
}

func TestStoreDefaultsToHyakkiState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	if got, want := NewStore("").Path(), filepath.Join(dir, "hyakki", "dictation.json"); got != want {
		t.Fatalf("default store = %s, want %s", got, want)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	s := newStore(t)
	empty, err := s.Load()
	if err != nil || len(empty.Dictionary)+len(empty.Snippets)+len(empty.Styles) != 0 {
		t.Fatalf("missing store = %+v, %v; want empty", empty, err)
	}
	if _, err := os.Stat(s.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load created the file: %v", err)
	}

	steps := []func() (Personal, error){
		func() (Personal, error) { return s.SetWord("  Kay   Eights ", " k8s ") },
		func() (Personal, error) { return s.SetWord("eye phone", "iPhone") },
		func() (Personal, error) { return s.SetWord("kay eights", "K8s") }, // same phrase: replaces
		func() (Personal, error) { return s.SetSnippet("my address", "12 Long Road\nLeeds\n") },
		func() (Personal, error) { return s.SetSnippet("sign off", "Thanks,\n\tAnn") },
		func() (Personal, error) { return s.SetStyle(Style{App: "Discord", Casing: "Lower", End: "none"}) },
		func() (Personal, error) { return s.SetStyle(Style{App: "kitty", Mode: "exact"}) },
	}
	var last Personal
	for k, step := range steps {
		if last, err = step(); err != nil {
			t.Fatalf("step %d: %v", k, err)
		}
	}
	want := Personal{
		Dictionary: []Entry{{"eye phone", "iPhone"}, {"kay eights", "K8s"}},
		Snippets:   []Snippet{{"my address", "12 Long Road\nLeeds\n"}, {"sign off", "Thanks,\n\tAnn"}},
		Styles:     []Style{{App: "Discord", Casing: CasingLower, End: EndNone}, {App: "kitty", Mode: "exact"}},
	}
	if !reflect.DeepEqual(last, want) {
		t.Fatalf("after edits = %+v\nwant %+v", last, want)
	}
	loaded, err := NewStore(s.Path()).Load()
	if err != nil || !reflect.DeepEqual(loaded, want) {
		t.Fatalf("reloaded = %+v, %v\nwant %+v", loaded, err, want)
	}

	info, err := os.Stat(s.Path())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("store file mode = %v, %v; want 0600", info.Mode(), err)
	}
	dir, err := os.Stat(filepath.Dir(s.Path()))
	if err != nil || dir.Mode().Perm() != 0o700 {
		t.Fatalf("store dir mode = %v, %v; want 0700", dir.Mode(), err)
	}
	entries, _ := os.ReadDir(filepath.Dir(s.Path()))
	if len(entries) != 1 {
		t.Fatalf("store dir holds %d files, want only the store: %v", len(entries), entries)
	}

	// The loaded store drives dictation.
	res := process(t, "um deploy to kay eights", Options{Mode: Clean, App: "discord", Personal: loaded})
	if res.Text != "deploy to K8s" {
		t.Fatalf("dictation with the loaded store = %q", res.Text)
	}
	if res := process(t, "insert sign off", Options{Mode: Clean, Personal: loaded}); res.Text != "Thanks,\n\tAnn" {
		t.Fatalf("snippet from the loaded store = %q", res.Text)
	}

	if last, err = s.RemoveWord("KAY EIGHTS"); err != nil || len(last.Dictionary) != 1 {
		t.Fatalf("RemoveWord = %+v, %v", last, err)
	}
	if last, err = s.RemoveSnippet("my address"); err != nil || len(last.Snippets) != 1 {
		t.Fatalf("RemoveSnippet = %+v, %v", last, err)
	}
	if last, err = s.RemoveStyle("DISCORD"); err != nil || len(last.Styles) != 1 {
		t.Fatalf("RemoveStyle = %+v, %v", last, err)
	}
	before, _ := os.ReadFile(s.Path())
	if _, err := s.RemoveWord("never added"); err != nil {
		t.Fatalf("removing an absent word: %v", err)
	}
	if after, _ := os.ReadFile(s.Path()); !bytes.Equal(before, after) {
		t.Fatal("removing an absent word rewrote the store")
	}

	saved, err := s.Save(Personal{Dictionary: []Entry{{"b", "B"}, {"a", "A"}, {"b", "Bee"}}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []Entry{{"a", "A"}, {"b", "Bee"}}; !reflect.DeepEqual(saved.Dictionary, want) || len(saved.Snippets) != 0 || len(saved.Styles) != 0 {
		t.Fatalf("Save = %+v, want sorted, last of each key, nothing else", saved)
	}
}

func TestStoreRejectsInvalidEntries(t *testing.T) {
	s := newStore(t)
	if _, err := s.SetWord("keep", "Keep"); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(s.Path())
	long := strings.Repeat("a", MaxPhraseBytes+1)
	cases := []struct {
		name string
		do   func() (Personal, error)
	}{
		{"empty spoken", func() (Personal, error) { return s.SetWord("  ", "x") }},
		{"spoken punctuation", func() (Personal, error) { return s.SetWord("c++", "C++") }},
		{"spoken too many words", func() (Personal, error) { return s.SetWord("red tan sky ink jet oak elm", "x") }},
		{"spoken too long", func() (Personal, error) { return s.SetWord(long, "x") }},
		{"spoken not utf-8", func() (Personal, error) { return s.SetWord("a\xff", "x") }},
		{"empty written", func() (Personal, error) { return s.SetWord("word", " ") }},
		{"written two lines", func() (Personal, error) { return s.SetWord("word", "a\nb") }},
		{"written too long", func() (Personal, error) { return s.SetWord("word", strings.Repeat("w", MaxWrittenBytes+1)) }},
		{"trigger starts with insert", func() (Personal, error) { return s.SetSnippet("insert date", "x") }},
		{"trigger starts with type", func() (Personal, error) { return s.SetSnippet("type date", "x") }},
		{"blank snippet", func() (Personal, error) { return s.SetSnippet("date", " \n ") }},
		{"snippet control", func() (Personal, error) { return s.SetSnippet("date", "a\x00b") }},
		{"snippet too long", func() (Personal, error) { return s.SetSnippet("date", strings.Repeat("s", MaxSnippetBytes+1)) }},
		{"app with space", func() (Personal, error) { return s.SetStyle(Style{App: "my app"}) }},
		{"app empty", func() (Personal, error) { return s.SetStyle(Style{App: ""}) }},
		{"bad mode", func() (Personal, error) { return s.SetStyle(Style{App: "x", Mode: "fancy"}) }},
		{"bad casing", func() (Personal, error) { return s.SetStyle(Style{App: "x", Casing: "title"}) }},
		{"bad end", func() (Personal, error) { return s.SetStyle(Style{App: "x", End: "bang"}) }},
		{"remove invalid phrase", func() (Personal, error) { return s.RemoveWord("?!") }},
	}
	for _, c := range cases {
		_, err := c.do()
		var pe *protocol.Error
		if !errors.As(err, &pe) || pe.Code != protocol.CodeBadRequest {
			t.Errorf("%s: err = %v, want a bad request", c.name, err)
		}
		if now, _ := os.ReadFile(s.Path()); !bytes.Equal(now, original) {
			t.Fatalf("%s changed the store", c.name)
		}
	}
}

func TestStoreBounds(t *testing.T) {
	s := newStore(t)
	// Every field at its bound, filled with characters JSON escapes.
	full := Personal{}
	phrase := func(k int) string {
		return "w" + strings.Repeat("x", 4) + string(rune('a'+k%26)) + " " + strings.Repeat("y", 3) + itoa(k)
	}
	for k := range MaxEntries {
		full.Dictionary = append(full.Dictionary, Entry{phrase(k), strings.Repeat(`"\`, MaxWrittenBytes/2)})
	}
	for k := range MaxSnippets {
		full.Snippets = append(full.Snippets, Snippet{phrase(k), strings.Repeat("\"\n", MaxSnippetBytes/2)})
	}
	for k := range MaxStyles {
		full.Styles = append(full.Styles, Style{App: itoa(k) + strings.Repeat(`\`, MaxAppBytes-len(itoa(k))), Mode: "polished", Casing: CasingSentence, End: EndPeriod})
	}
	saved, err := s.Save(full)
	if err != nil {
		t.Fatalf("saving a store at every bound: %v", err)
	}
	if info, _ := os.Stat(s.Path()); info.Size() > MaxStoreBytes {
		t.Fatalf("store at every bound is %d bytes, over MaxStoreBytes", info.Size())
	}
	loaded, err := s.Load()
	if err != nil || !reflect.DeepEqual(loaded, saved) {
		t.Fatalf("reloading the full store: %v", err)
	}
	for name, do := range map[string]func() (Personal, error){
		"word":    func() (Personal, error) { return s.SetWord("one more", "x") },
		"snippet": func() (Personal, error) { return s.SetSnippet("one more", "x") },
		"style":   func() (Personal, error) { return s.SetStyle(Style{App: "one.more"}) },
	} {
		_, err := do()
		wantCode(t, err, protocol.CodeBadRequest, "")
		if after, _ := s.Load(); !reflect.DeepEqual(after, saved) {
			t.Fatalf("adding a %s past the bound changed the store", name)
		}
	}
	// Replacing an existing key at the bound is allowed.
	if _, err := s.SetWord(phrase(0), "replaced"); err != nil {
		t.Fatalf("replacing at the bound: %v", err)
	}
}

func itoa(k int) string {
	if k == 0 {
		return "0"
	}
	var b []byte
	for ; k > 0; k /= 10 {
		b = append([]byte{byte('0' + k%10)}, b...)
	}
	return string(b)
}

func TestStoreDamagedFileIsKept(t *testing.T) {
	cases := map[string][]byte{
		"not json":      []byte("{dictionary: nope"),
		"trailing data": []byte(`{"v":1,"dictionary":[],"snippets":[],"styles":[]} extra`),
		"wrong version": []byte(`{"v":2,"dictionary":[],"snippets":[],"styles":[]}`),
		"no version":    []byte(`{"dictionary":[]}`),
		"bad entry":     []byte(`{"v":1,"dictionary":[{"spoken":"c++","written":"C++"}],"snippets":[],"styles":[]}`),
		"too many":      tooMany(),
		"too large":     append([]byte(`{"v":1,"dictionary":[],"snippets":[],"styles":[]}`), bytes.Repeat([]byte(" "), MaxStoreBytes)...),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			if err := os.MkdirAll(filepath.Dir(s.Path()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(s.Path(), content, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := s.Load()
			wantCode(t, err, protocol.CodeConflict, ReasonDamaged)
			for _, do := range []func() (Personal, error){
				func() (Personal, error) { return s.SetWord("new", "New") },
				func() (Personal, error) { return s.RemoveWord("new") },
				func() (Personal, error) { return s.SetSnippet("new", "New") },
				func() (Personal, error) { return s.SetStyle(Style{App: "new"}) },
				func() (Personal, error) { return s.Save(Personal{}) },
			} {
				_, err := do()
				wantCode(t, err, protocol.CodeConflict, ReasonDamaged)
				if now, _ := os.ReadFile(s.Path()); !bytes.Equal(now, content) {
					t.Fatal("a damaged store was overwritten")
				}
			}
			if err := s.Reset(); err != nil {
				t.Fatal(err)
			}
			if kept, _ := os.ReadFile(s.DamagedPath()); !bytes.Equal(kept, content) {
				t.Fatal("Reset did not keep the damaged file beside the store")
			}
			p, err := s.Load()
			if err != nil || len(p.Dictionary) != 0 {
				t.Fatalf("after Reset = %+v, %v; want empty", p, err)
			}
			if p, err := s.SetWord("new", "New"); err != nil || len(p.Dictionary) != 1 {
				t.Fatalf("SetWord after Reset = %+v, %v", p, err)
			}
		})
	}
	if err := newStore(t).Reset(); err != nil {
		t.Fatalf("Reset with no file: %v", err)
	}
}

func tooMany() []byte {
	var b strings.Builder
	b.WriteString(`{"v":1,"dictionary":[],"snippets":[],"styles":[`)
	for k := range MaxStyles + 1 {
		if k > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"app":"a` + itoa(k) + `"}`)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}
