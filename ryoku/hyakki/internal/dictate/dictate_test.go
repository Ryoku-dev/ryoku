package dictate

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func process(t *testing.T, text string, o Options) Result {
	t.Helper()
	res, err := Process(text, o)
	if err != nil {
		t.Fatalf("Process(%q): %v", text, err)
	}
	return res
}

func hasNote(res Result, code string) bool {
	for _, n := range res.Notes {
		if n.Code == code {
			return true
		}
	}
	return false
}

func TestExactIsVerbatim(t *testing.T) {
	personal := Personal{
		Dictionary: []Entry{{"kay eights", "k8s"}},
		Snippets:   []Snippet{{"my address", "12 Long Road"}},
	}
	for _, in := range []string{
		"um so  we ship it uh tomorrow ",
		"Send it Monday. Scratch that. Send it Tuesday.",
		"my address",
		"deploy to kay eights comma now",
		"line one\n\n\nline two\t",
		"",
	} {
		res := process(t, in, Options{Mode: Exact, Personal: personal})
		if res.Text != in || res.Mode != Exact || res.Fallback != FallbackNone || len(res.Notes) != 0 {
			t.Errorf("Exact(%q) = %+v, want the text unchanged with no notes", in, res)
		}
	}
}

func TestCleanTidiesDeterministically(t *testing.T) {
	cases := []struct{ in, want string }{
		{"um so we ship it uh tomorrow", "So we ship it tomorrow"},
		{"Um, I think, uh, it works.", "I think, it works."},
		{"hmm   okay   then", "Okay then"},
		{"ER visits went up", "ER visits went up"},
		{"hello ,world .  how are you ?", "Hello, world. How are you?"},
		{"i think i'm right and i'll go", "I think I'm right and I'll go"},
		{"hello comma world period how are you question mark", "Hello, world. How are you?"},
		{"Hello, comma, world, period.", "Hello, world."},
		{"that is great exclamation point", "That is great!"},
		{"the period ended", "The period ended"},
		{"a comma here", "A comma here"},
		{"a period of growth", "A period of growth"},
		{"comma at the start", "Comma at the start"},
		{"first line new line second line", "First line\nSecond line"},
		{"dear Anna new paragraph thanks for the notes", "Dear Anna\n\nThanks for the notes"},
		{"It ended. New paragraph. Then more", "It ended.\n\nThen more"},
		{"wait... what", "Wait... what"},
		{"see e.g. this one", "See e.g. this one"},
		{"either and/or works", "Either and/or works"},
		{"(see the notes) and \"quote\" it", "(See the notes) and \"quote\" it"},
		{"   ", ""},
		{"um uh", ""},
		{"we ship today, uh", "We ship today"},
	}
	for _, c := range cases {
		first := process(t, c.in, Options{Mode: Clean})
		if first.Text != c.want || first.Mode != Clean {
			t.Errorf("Clean(%q) = %q (%v), want %q", c.in, first.Text, first.Mode, c.want)
		}
		for range 3 {
			if again := process(t, c.in, Options{Mode: Clean}); again.Text != first.Text {
				t.Fatalf("Clean(%q) is not deterministic: %q then %q", c.in, first.Text, again.Text)
			}
		}
	}
	if res := process(t, "um, so, uh, yes", Options{Mode: Clean}); !hasNote(res, NoteFillers) {
		t.Errorf("filler removal left no note: %+v", res.Notes)
	}
	if res := process(t, "hello comma world", Options{Mode: Clean}); !hasNote(res, NotePunctuation) {
		t.Errorf("spoken punctuation left no note: %+v", res.Notes)
	}
}

// protectedSample holds one of each protected value.
const protectedSample = "um check https://example.com/a?b=1&c=D and mail Ann.Lee@example.org about ~/notes/todo.md, /usr/bin/env, src/app/main.go, `go test ./...` and 3.14 period"

var protectedValues = []string{
	"https://example.com/a?b=1&c=D", "Ann.Lee@example.org", "~/notes/todo.md",
	"/usr/bin/env", "src/app/main.go", "`go test ./...`", "3.14",
}

func TestCleanKeepsProtectedValues(t *testing.T) {
	personal := Personal{Dictionary: []Entry{{"example", "EXAMPLE"}, {"go", "Go"}, {"test", "TEST"}}}
	res := process(t, protectedSample, Options{Mode: Clean, Personal: personal})
	want := "Check https://example.com/a?b=1&c=D and mail Ann.Lee@example.org about ~/notes/todo.md, /usr/bin/env, src/app/main.go, `go test ./...` and 3.14."
	if res.Text != want {
		t.Fatalf("Clean = %q\nwant    %q", res.Text, want)
	}
	cases := []struct{ in, want string }{
		{"https://x.org is up", "https://x.org is up"},
		{"www.example.com um has it", "www.example.com has it"},
		{"run `um uh` now", "Run `um uh` now"},
		{"```\nscratch that\n```", "```\nscratch that\n```"},
		{"v1.2.3 shipped on 2026-11-22 at 3:45pm for $5.99", "v1.2.3 shipped on 2026-11-22 at 3:45pm for $5.99"},
		{"C:\\Users\\ann\\file.txt is there", "C:\\Users\\ann\\file.txt is there"},
	}
	for _, c := range cases {
		if got := process(t, c.in, Options{Mode: Clean}).Text; got != c.want {
			t.Errorf("Clean(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRepairsStayInsideTheUtterance(t *testing.T) {
	cases := []struct {
		in, want string
		repaired bool
	}{
		{"Send it Monday. Scratch that. Send it Tuesday.", "Send it Tuesday.", true},
		{"Buy milk, scratch that, buy bread", "Buy bread", true},
		{"Hello. Buy milk scratch that.", "Hello.", true},
		{"Scratch that, call Bob", "Call Bob", true},
		{"Scratch that.", "", true},
		{"I need to scratch that itch", "I need to scratch that itch", false},
		{"Meet at 3, make that 4.", "Meet at 4.", true},
		{"Meet at 3. Make that 4.", "Meet at 4.", true},
		{"Book a table for two, make that three, please.", "Book a table for three, please.", true},
		{"Send it to Anna, no, I mean Bob.", "Send it to Bob.", true},
		{"Call Anna Smith, no I mean Anna Jones.", "Call Anna Jones.", true},
		{"Ship it Monday, actually, Tuesday.", "Ship it Tuesday.", true},
		{"Ship it Monday — actually — Tuesday.", "Ship it Tuesday.", true},
		{"First point. Second point, make that third point.", "First point. Third point.", true},
		{"Meet at 3, make that 4, scratch that, meet at 5", "Meet at 5", true},
		{"It's actually fine.", "It's actually fine.", false},
		{"Actually, it is fine.", "Actually, it is fine.", false},
		{"We could meet Thursday. Actually, let's do Friday.", "We could meet Thursday. Actually, let's do Friday.", false},
		{"No, I mean it.", "No, I mean it.", false},
		{"I'm serious, no, I mean it.", "I'm serious, no, I mean it.", false},
		{"I can make that work.", "I can make that work.", false},
		{"Make that the default.", "Make that the default.", false},
	}
	for _, c := range cases {
		res := process(t, c.in, Options{Mode: Clean})
		if res.Text != c.want {
			t.Errorf("Clean(%q) = %q, want %q", c.in, res.Text, c.want)
		}
		if hasNote(res, NoteRepair) != c.repaired {
			t.Errorf("Clean(%q) repair note = %v, want %v", c.in, hasNote(res, NoteRepair), c.repaired)
		}
	}
}

func TestSnippetsOnlyWhenAskedFor(t *testing.T) {
	body := "12 Long Road\nLeeds LS1 4AB"
	personal := Personal{Snippets: []Snippet{{"my address", body}}}
	cases := []struct {
		in       string
		expanded bool
	}{
		{"my address", true},
		{"My address.", true},
		{"insert my address", true},
		{"Type my address.", true},
		{"um, my address", true},
		{"send it to my address", false},
		{"my address is old", false},
		{"insert the address", false},
	}
	for _, c := range cases {
		res := process(t, c.in, Options{Mode: Clean, Personal: personal})
		if got := res.Text == body; got != c.expanded {
			t.Errorf("Clean(%q) = %q, expanded %v, want %v", c.in, res.Text, got, c.expanded)
		}
		if hasNote(res, NoteSnippet) != c.expanded {
			t.Errorf("Clean(%q) snippet note = %v, want %v", c.in, hasNote(res, NoteSnippet), c.expanded)
		}
	}
	calls := 0
	res := process(t, "insert my address", Options{Mode: Polished, Personal: personal, Polisher: polishFunc(func(context.Context, string) (string, error) {
		calls++
		return "rewritten", nil
	})})
	if res.Text != body || calls != 0 || res.Mode != Clean || res.Fallback != FallbackNone {
		t.Errorf("Polished snippet = %+v after %d polish calls, want the snippet as stored and no polish", res, calls)
	}
}

func TestDictionarySpellings(t *testing.T) {
	personal := Personal{Dictionary: []Entry{
		{"kay eights", "k8s"},
		{"eye phone", "iPhone"},
		{"open ai", "OpenAI"},
		{"open", "OPEN"},
	}}
	cases := []struct{ in, want string }{
		{"deploy to kay eights now", "Deploy to k8s now"},
		{"eye phone sales are up. eye phone again", "iPhone sales are up. iPhone again"},
		{"ask open ai to open it", "Ask OpenAI to OPEN it"},
		{"kay, eights", "Kay, eights"},
	}
	for _, c := range cases {
		res := process(t, c.in, Options{Mode: Clean, Personal: personal})
		if res.Text != c.want {
			t.Errorf("Clean(%q) = %q, want %q", c.in, res.Text, c.want)
		}
	}
	if res := process(t, "kay eights", Options{Mode: Clean, Personal: personal}); !hasNote(res, NoteDictionary) {
		t.Errorf("dictionary use left no note: %+v", res.Notes)
	}
}

func TestPerAppStyles(t *testing.T) {
	personal := Personal{Styles: []Style{
		{App: "discord", Casing: CasingLower, End: EndNone},
		{App: "org.gnome.Evolution", End: EndPeriod},
		{App: "kitty", Mode: "exact"},
		{App: "notes", Casing: CasingKeep},
	}}
	cases := []struct {
		app, in, want string
		mode          Mode
	}{
		{"discord", "Sounds good. See you then.", "sounds good. see you then", Clean},
		{"Discord", "NASA called. I said hi.", "NASA called. I said hi", Clean},
		{"org.gnome.Evolution", "thanks for the update", "Thanks for the update.", Clean},
		{"org.gnome.Evolution", "is it done?", "Is it done?", Clean},
		{"kitty", "um ls dash la", "um ls dash la", Exact},
		{"notes", "lower start. and more", "lower start. and more", Clean},
		{"firefox", "um hello", "Hello", Clean},
	}
	for _, c := range cases {
		res := process(t, c.in, Options{Mode: Clean, App: c.app, Personal: personal})
		if res.Text != c.want || res.Mode != c.mode {
			t.Errorf("Clean(%q) in %s = %q (%v), want %q (%v)", c.in, c.app, res.Text, res.Mode, c.want, c.mode)
		}
		if hasNote(res, NoteStyle) != (c.app != "firefox") {
			t.Errorf("Clean(%q) in %s style note = %v", c.in, c.app, hasNote(res, NoteStyle))
		}
	}
	polisher := polishFunc(func(_ context.Context, s string) (string, error) { return "Sounds good, see you then.", nil })
	res := process(t, "sounds good see you then", Options{Mode: Polished, App: "discord", Personal: personal, Polisher: polisher})
	if res.Mode != Polished || res.Text != "Sounds good, see you then" {
		t.Errorf("Polished in discord = %+v, want the rewrite without its final full stop", res)
	}
}

type polishFunc func(context.Context, string) (string, error)

func (f polishFunc) Polish(ctx context.Context, s string) (string, error) { return f(ctx, s) }

func TestPolishedKeepsProtectedValues(t *testing.T) {
	clean := process(t, protectedSample, Options{Mode: Clean}).Text
	keeper := polishFunc(func(_ context.Context, s string) (string, error) {
		return "Please " + strings.ToLower(s[:1]) + s[1:], nil
	})
	res := process(t, protectedSample, Options{Mode: Polished, Polisher: keeper})
	if res.Mode != Polished || res.Fallback != FallbackNone || !hasNote(res, NotePolished) {
		t.Fatalf("Polished = %+v, want the rewrite", res)
	}
	for _, v := range protectedValues {
		if !strings.Contains(res.Text, v) {
			t.Errorf("Polished lost %q: %q", v, res.Text)
		}
	}
	for _, v := range protectedValues {
		dropper := polishFunc(func(_ context.Context, s string) (string, error) {
			return strings.Replace(s, v, "it", 1), nil
		})
		res := process(t, protectedSample, Options{Mode: Polished, Polisher: dropper})
		if res.Text != clean || res.Mode != Clean || res.Fallback != FallbackProtected || !hasNote(res, NoteFallback) {
			t.Errorf("rewrite dropping %q = %+v, want the clean text with a protected fallback", v, res)
		}
	}
	changer := polishFunc(func(_ context.Context, s string) (string, error) {
		return strings.Replace(s, "3.14", "3.140", 1), nil
	})
	if res := process(t, protectedSample, Options{Mode: Polished, Polisher: changer}); res.Fallback != FallbackProtected {
		t.Errorf("rewrite changing a number = %+v, want a protected fallback", res)
	}
	twice := "send 5 then 5 more"
	halver := polishFunc(func(_ context.Context, s string) (string, error) { return "Send 5 more.", nil })
	if res := process(t, twice, Options{Mode: Polished, Polisher: halver}); res.Fallback != FallbackProtected {
		t.Errorf("rewrite dropping a repeated number = %+v, want a protected fallback", res)
	}
}

func TestPolishedFallsBackToClean(t *testing.T) {
	const in = "um so we should send the report by friday i think"
	clean := process(t, in, Options{Mode: Clean}).Text
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	cases := []struct {
		name     string
		polisher Polisher
		want     Fallback
	}{
		{"none", nil, FallbackUnavailable},
		{"ignores its deadline", polishFunc(func(context.Context, string) (string, error) {
			<-release
			return "too late", nil
		}), FallbackTimeout},
		{"honours its deadline", polishFunc(func(ctx context.Context, _ string) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		}), FallbackTimeout},
		{"errors", polishFunc(func(context.Context, string) (string, error) {
			return "", errors.New("model crashed")
		}), FallbackFailed},
		{"empty", polishFunc(func(context.Context, string) (string, error) { return "  \n ", nil }), FallbackInvalid},
		{"control", polishFunc(func(context.Context, string) (string, error) { return "We should\x1b[2J send it.", nil }), FallbackInvalid},
		{"not utf-8", polishFunc(func(context.Context, string) (string, error) { return "We should \xff send it.", nil }), FallbackInvalid},
		{"expands", polishFunc(func(_ context.Context, s string) (string, error) {
			return s + " " + s + " " + s, nil
		}), FallbackExpansion},
	}
	for _, c := range cases {
		start := time.Now()
		res := process(t, in, Options{Mode: Polished, Polisher: c.polisher, Timeout: 40 * time.Millisecond})
		if res.Text != clean || res.Mode != Clean || res.Fallback != c.want || !hasNote(res, NoteFallback) {
			t.Errorf("%s: %+v, want the clean text %q with fallback %q", c.name, res, clean, c.want)
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Errorf("%s: took %v, want the polish timeout to bound it", c.name, elapsed)
		}
	}
}

func TestPolishedExpansionLimit(t *testing.T) {
	in := strings.Repeat("ab ", 13) + "ab" // Clean: 41 characters
	clean := process(t, in, Options{Mode: Clean}).Text
	n := len([]rune(clean))
	limit := max((3*n+1)/2, n+ExpansionFloor) // ceil(1.5 n)
	grow := func(size int) Polisher {
		return polishFunc(func(_ context.Context, s string) (string, error) {
			return s + strings.Repeat("x", size-len([]rune(s))), nil
		})
	}
	if res := process(t, in, Options{Mode: Polished, Polisher: grow(limit)}); res.Mode != Polished {
		t.Errorf("rewrite at the limit (%d of %d) fell back: %+v", limit, n, res)
	}
	if res := process(t, in, Options{Mode: Polished, Polisher: grow(limit + 1)}); res.Fallback != FallbackExpansion {
		t.Errorf("rewrite past the limit (%d of %d) = %+v, want an expansion fallback", limit+1, n, res)
	}
	short := "ok"
	if res := process(t, short, Options{Mode: Polished, Polisher: grow(2 + ExpansionFloor)}); res.Mode != Polished {
		t.Errorf("a short text may grow by the floor: %+v", res)
	}
	if res := process(t, in, Options{Mode: Polished, MaxExpansion: 1, Polisher: grow(n + ExpansionFloor + 1)}); res.Fallback != FallbackExpansion {
		t.Errorf("custom ratio 1 = %+v, want an expansion fallback", res)
	}
}

type recorder struct {
	mu    sync.Mutex
	texts []string
}

func (r *recorder) Polish(_ context.Context, s string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.texts = append(r.texts, s)
	return s, nil
}

func (r *recorder) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.texts)
}

func TestSecretsNeverReachAHostedPolisher(t *testing.T) {
	secrets := []struct{ text, context, app string }{
		{text: "my password is swordfish"},
		{text: "the PIN is four four two one"},
		{text: "cvv is one two three"},
		{text: "here is the one-time code"},
		{text: "my card number is coming"},
		{text: "card 4111 1111 1111 1111 expires soon"},
		{text: "the verification code is ready"},
		{text: "um, scratch that, the passcode is swordfish"},
		{text: "type it in", context: "Enter your PIN"},
		{text: "type it in", context: "Bank — Two-factor authentication"},
		{text: "type it in", app: "org.keepassxc.KeePassXC"},
		{text: "type it in", app: "com.bitwarden.desktop"},
	}
	for _, s := range secrets {
		rec := &recorder{}
		res := process(t, s.text, Options{Mode: Polished, Polisher: rec, Hosted: true, Context: s.context, App: s.app})
		if rec.calls() != 0 || res.Fallback != FallbackSecret || res.Mode != Clean {
			t.Errorf("hosted polish of %+v: %d calls, result %+v; want no call and a secret fallback", s, rec.calls(), res)
		}
		for _, n := range res.Notes {
			if strings.Contains(n.Text, "swordfish") {
				t.Errorf("a note repeats the dictated secret: %q", n.Text)
			}
		}
		local := &recorder{}
		res = process(t, s.text, Options{Mode: Polished, Polisher: local, Context: s.context, App: s.app})
		if local.calls() != 1 || res.Mode != Polished {
			t.Errorf("local polish of %+v: %d calls, result %+v; want the local polisher used", s, local.calls(), res)
		}
	}
	for _, plain := range []string{
		"spin the wheel", "a pinecone fell", "call me at 0113 496 0000",
		"card 4111 1111 1111 1112 is a typo", "ship the release notes",
	} {
		rec := &recorder{}
		res := process(t, plain, Options{Mode: Polished, Polisher: rec, Hosted: true})
		if rec.calls() != 1 || res.Mode != Polished {
			t.Errorf("hosted polish of %q: %d calls, result %+v; want it polished", plain, rec.calls(), res)
		}
	}
}

func TestCancelTypesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	polisher := polishFunc(func(ctx context.Context, _ string) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	go func() {
		<-started
		cancel()
	}()
	res, err := ProcessContext(ctx, "send it", Options{Mode: Polished, Polisher: polisher, Timeout: time.Minute})
	if !errors.Is(err, context.Canceled) || res.Text != "" {
		t.Fatalf("cancelled polish = %+v, %v; want no text and the cancel error", res, err)
	}
	if res, err := ProcessContext(ctx, "send it", Options{Mode: Exact}); err == nil || res.Text != "" {
		t.Fatalf("cancelled context = %+v, %v; want no text and an error", res, err)
	}
}

func TestProcessRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		text string
		o    Options
	}{
		{"not utf-8", "bad \xff text", Options{Mode: Clean}},
		{"too long", strings.Repeat("a", MaxText+1), Options{Mode: Clean}},
		{"unknown mode", "hi", Options{Mode: Mode(9)}},
		{"negative timeout", "hi", Options{Mode: Polished, Timeout: -time.Second}},
		{"shrinking ratio", "hi", Options{Mode: Polished, MaxExpansion: 0.5}},
	}
	for _, c := range cases {
		_, err := Process(c.text, c.o)
		var pe *protocol.Error
		if !errors.As(err, &pe) || pe.Code != protocol.CodeBadRequest {
			t.Errorf("%s: err = %v, want a bad request", c.name, err)
		}
	}
	if _, err := Process(strings.Repeat("a", MaxText), Options{Mode: Exact}); err != nil {
		t.Errorf("text at MaxText: %v", err)
	}
}

func TestModeNames(t *testing.T) {
	for _, m := range []Mode{Exact, Clean, Polished} {
		b, err := m.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var back Mode
		if err := back.UnmarshalText(b); err != nil || back != m {
			t.Errorf("round trip of %v = %v, %v", m, back, err)
		}
	}
	if _, err := ParseMode("fancy"); err == nil {
		t.Error("ParseMode accepted an unknown name")
	}
	if m, err := ParseMode(" Polished "); err != nil || m != Polished {
		t.Errorf("ParseMode(\" Polished \") = %v, %v", m, err)
	}
}

func TestNotesCarryNoDictatedWords(t *testing.T) {
	personal := Personal{
		Dictionary: []Entry{{"zebra", "Zebrafish"}},
		Snippets:   []Snippet{{"giraffe", "Okapi"}},
		Styles:     []Style{{App: "app", End: EndPeriod}},
	}
	for _, in := range []string{"um zebra comma make that, scratch that, platypus", "insert giraffe"} {
		res := process(t, in, Options{Mode: Polished, App: "app", Personal: personal})
		for _, n := range res.Notes {
			for _, w := range []string{"zebra", "Zebrafish", "giraffe", "Okapi", "platypus"} {
				if strings.Contains(n.Text, w) {
					t.Errorf("note %+v repeats %q", n, w)
				}
			}
		}
	}
}
