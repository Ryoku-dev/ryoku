package writing

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/grammar"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

type testClient struct {
	reply    string
	request  llm.Request
	calls    int
	deadline time.Time
	stream   *testStream
	started  chan struct{}
}

func (c *testClient) Stream(ctx context.Context, request llm.Request) (llm.Stream, error) {
	c.calls++
	c.request = request
	c.deadline, _ = ctx.Deadline()
	if c.stream == nil {
		c.stream = &testStream{reply: c.reply, closed: make(chan struct{})}
	}
	if c.started != nil {
		close(c.started)
	}
	return c.stream, nil
}

type testStream struct {
	reply  string
	read   bool
	block  bool
	closed chan struct{}
	once   sync.Once
}

func (s *testStream) Next() (llm.Delta, error) {
	if s.block {
		<-s.closed
		return llm.Delta{}, io.EOF
	}
	if s.read {
		return llm.Delta{}, io.EOF
	}
	s.read = true
	return llm.Delta{Text: s.reply}, nil
}

func (s *testStream) Final() llm.Final { return llm.Final{Model: "local/test"} }
func (s *testStream) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

func TestAutoUsesModelForFrench(t *testing.T) {
	client := &testClient{reply: `{"corrections":[{"original":"bonjor","replacement":"bonjour","reason":"Check spelling.","kind":"spelling"}]}`}
	out, err := checkWith(context.Background(), client, protocol.WritingCheckParams{Text: "🙂 中文 bonjor, comment allez-vous ?", Local: true}, func(context.Context, string, grammar.Options) ([]grammar.Issue, error) {
		t.Fatal("French text was checked locally despite a configured model")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Engine != "model" || client.calls != 1 || !client.request.LocalOnly {
		t.Fatalf("result=%#v, client=%#v", out, client)
	}
	if out.Issues[0].Start != 6 || out.Issues[0].End != 12 {
		t.Fatalf("model span=%#v", out.Issues[0])
	}
	if client.deadline.IsZero() || time.Until(client.deadline) > 2*time.Minute {
		t.Fatalf("missing request deadline: %v", client.deadline)
	}
}

func TestLocalFrenchDoesNotCallModel(t *testing.T) {
	client := &testClient{}
	checked := false
	out, err := checkWith(context.Background(), client, protocol.WritingCheckParams{Text: "Bonjour, comment allez-vous ?", Engine: "local"}, func(context.Context, string, grammar.Options) ([]grammar.Issue, error) {
		checked = true
		return nil, nil
	})
	if err != nil || !checked || out.Engine != "local" || client.calls != 0 {
		t.Fatalf("result=%#v, error=%v, calls=%d", out, err, client.calls)
	}
}

func TestModelFilterAndRepeatedOccurrences(t *testing.T) {
	client := &testClient{reply: "```json\n" + `{"corrections":[{"original":"teh","replacement":"the","reason":"Typo.","kind":"spelling"},{"original":"teh","replacement":"the","reason":"Typo.","kind":"spelling"},{"original":"is are","replacement":"is","reason":"Agreement.","kind":"grammar"}]}` + "\n```"}
	out, err := Check(context.Background(), client, protocol.WritingCheckParams{Text: "teh teh is are", Engine: "model", Off: []string{"spelling"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Issues) != 1 || out.Issues[0].Start != 8 || out.Issues[0].End != 14 {
		t.Fatalf("filtered issues=%#v", out.Issues)
	}
}

func TestTransformActions(t *testing.T) {
	for _, action := range []string{"translate", "proofread", "rewrite", "tone", "fluent", "humanize", "enhance"} {
		t.Run(action, func(t *testing.T) {
			client := &testClient{reply: "Changed text"}
			out, err := Transform(context.Background(), client, protocol.WritingTransformParams{
				Text: "Please review this text.", Action: action, Target: "French", Local: true,
				Options: map[string]string{"tone": "friendly"},
			})
			if err != nil || out.Text != "Changed text" || out.Model != "local/test" || client.calls != 1 || !client.request.LocalOnly {
				t.Fatalf("out=%#v, error=%v, calls=%d", out, err, client.calls)
			}
		})
	}
}

func TestTranslationFixInstruction(t *testing.T) {
	p := protocol.WritingTransformParams{Action: "translate", Target: "English"}
	base, err := transformInstruction(p)
	if err != nil || strings.Contains(base, "already in the target language") {
		t.Fatalf("plain translation=%q, error=%v", base, err)
	}
	p.Options = map[string]string{"fix": "on"}
	fixed, err := transformInstruction(p)
	if err != nil || !strings.Contains(fixed, "If the text is already in the target language, fix only its spelling, grammar and punctuation instead, keeping meaning, tone and layout.") {
		t.Fatalf("fix translation=%q, error=%v", fixed, err)
	}
	p.Options["fix"] = "off"
	if got, err := transformInstruction(p); err != nil || got != base {
		t.Fatalf("disabled fix=%q, error=%v", got, err)
	}
	for _, action := range []string{"translate", "lines", "proofread"} {
		p.Action = action
		p.Options["fix"] = "unknown"
		if _, err := transformInstruction(p); err == nil {
			t.Fatalf("%s accepted unknown fix option", action)
		}
	}
	for _, action := range []string{"lines", "proofread"} {
		p.Action = action
		p.Options["fix"] = "on"
		if _, err := transformInstruction(p); err == nil {
			t.Fatalf("%s accepted translation-only fix", action)
		}
	}
}

func TestLinesKeepsCountAndBlanks(t *testing.T) {
	client := &testClient{reply: `{"lines":["Bonjour","ignored","Au revoir"]}`}
	out, err := Transform(context.Background(), client, protocol.WritingTransformParams{Action: "lines", Target: "French", Lines: []string{"Hello", "  ", "Goodbye"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Lines) != 3 || out.Lines[1] != "  " || out.Text != "Bonjour\n  \nAu revoir" {
		t.Fatalf("translated lines=%#v", out)
	}
	client = &testClient{reply: `{"lines":["Bonjour"]}`}
	if _, err := Transform(context.Background(), client, protocol.WritingTransformParams{Action: "lines", Target: "French", Lines: []string{"Hello", "Goodbye"}}); err == nil {
		t.Fatal("accepted the wrong line count")
	}
}

func TestCancelledModelClosesStream(t *testing.T) {
	stream := &testStream{block: true, closed: make(chan struct{})}
	client := &testClient{stream: stream, started: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-client.started
		cancel()
	}()
	_, _, err := complete(ctx, client, "", false, "Rewrite.", "Text.")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled completion error=%v", err)
	}
	select {
	case <-stream.closed:
	default:
		t.Fatal("stream was not closed")
	}
}

func TestTransformInvalidActionDoesNotCallModel(t *testing.T) {
	client := &testClient{}
	if _, err := Transform(context.Background(), client, protocol.WritingTransformParams{Text: "Text.", Action: "unknown"}); err == nil || client.calls != 0 {
		t.Fatalf("error=%v, calls=%d", err, client.calls)
	}
}

type providerClient struct {
	testClient
	providers []protocol.ProviderInfo
}

func (c *providerClient) Providers(context.Context) ([]protocol.ProviderInfo, error) {
	return c.providers, nil
}

func TestAutoTypoEnglishChecksLocallyWithoutProviderCall(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	client := &testClient{}
	checked := false
	out, err := checkWith(context.Background(), client, protocol.WritingCheckParams{Text: "Our neighbours decided to recieve it."}, func(context.Context, string, grammar.Options) ([]grammar.Issue, error) {
		checked = true
		return []grammar.Issue{{Start: 26, End: 33, Kind: "Spelling", Replacements: []string{"receive"}}}, nil
	})
	if err != nil || !checked || out.Engine != "local" || len(out.Issues) != 1 || client.calls != 0 {
		t.Fatalf("result=%#v, checked=%v, provider calls=%d, error=%v", out, checked, client.calls, err)
	}
}

func TestAutoSpanishUsesConfiguredModel(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	client := &providerClient{testClient: testClient{reply: `{"corrections":[]}`}, providers: []protocol.ProviderInfo{{Enabled: true, Configured: true, Models: 1}}}
	out, err := checkWith(context.Background(), client, protocol.WritingCheckParams{Text: "Hola, ¿cómo estás?"}, func(context.Context, string, grammar.Options) ([]grammar.Issue, error) {
		t.Fatal("Spanish text checked locally")
		return nil, nil
	})
	if err != nil || out.Engine != "model" || client.calls != 1 {
		t.Fatalf("result=%#v, calls=%d, error=%v", out, client.calls, err)
	}
}

func TestAutoWithoutConfiguredProviderChecksLocally(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	client := &providerClient{}
	for _, gateway := range []llm.Client{nil, client} {
		checked := false
		out, err := checkWith(context.Background(), gateway, protocol.WritingCheckParams{Text: "Hola, ¿cómo estás?"}, func(context.Context, string, grammar.Options) ([]grammar.Issue, error) {
			checked = true
			return nil, nil
		})
		if err != nil || !checked || out.Engine != "local" || client.calls != 0 {
			t.Fatalf("result=%#v, calls=%d, error=%v", out, client.calls, err)
		}
	}
}
