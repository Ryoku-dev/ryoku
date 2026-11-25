package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

var errWritingGatewayProbe = errors.New("writing gateway reached")

type writingGatewayProbe struct {
	core.Gateway
	calls int
}

func (g *writingGatewayProbe) Stream(context.Context, llm.Request) (llm.Stream, error) {
	g.calls++
	return nil, errWritingGatewayProbe
}

func writingTestDaemon(t *testing.T, gateway *writingGatewayProbe) *Daemon {
	t.Helper()
	d := &Daemon{deps: Deps{Gateway: gateway}}
	t.Cleanup(func() {
		writingRequests.Lock()
		defer writingRequests.Unlock()
		for key, request := range writingRequests.active {
			if key.daemon == d {
				if request.cancel != nil {
					request.cancel()
				}
				delete(writingRequests.active, key)
			}
		}
	})
	return d
}

func TestWritingEnhanceRequiresFeatureBeforeRegistration(t *testing.T) {
	gateway := &writingGatewayProbe{}
	d := writingTestDaemon(t, gateway)
	_, err := d.writingTransform(context.Background(), nil, protocol.WritingTransformParams{
		Request: "enhance-off", Action: "enhance", Text: "Review this page.",
	})
	if err == nil || errors.Is(err, errWritingGatewayProbe) || gateway.calls != 0 {
		t.Fatalf("disabled enhance: error=%v, provider calls=%d", err, gateway.calls)
	}
	writingRequests.Lock()
	_, registered := writingRequests.active[writingRequestKey{daemon: d, id: "enhance-off"}]
	writingRequests.Unlock()
	if registered {
		t.Fatal("disabled enhance registered a writing request")
	}
	d.settings.Features = map[string]bool{"enhance": true}
	_, err = d.writingTransform(context.Background(), nil, protocol.WritingTransformParams{
		Request: "enhance-on", Action: "enhance", Text: "Review this page.",
	})
	if !errors.Is(err, errWritingGatewayProbe) || gateway.calls != 1 {
		t.Fatalf("enabled enhance: error=%v, provider calls=%d", err, gateway.calls)
	}
}

func TestWritingTheiaTransformsDoNotRequireAppFeatures(t *testing.T) {
	gateway := &writingGatewayProbe{}
	d := writingTestDaemon(t, gateway)
	for _, action := range []string{"translate", "proofread", "rewrite", "tone", "fluent", "humanize", "lines"} {
		t.Run(action, func(t *testing.T) {
			before := gateway.calls
			_, err := d.writingTransform(context.Background(), nil, protocol.WritingTransformParams{
				Request: "theia-" + action, Action: action, Text: "Review this page.",
				Target: "French", Options: map[string]string{"tone": "formal"}, Lines: []string{"Review this page."},
			})
			if !errors.Is(err, errWritingGatewayProbe) || gateway.calls != before+1 {
				t.Fatalf("Theia action %s was gated: error=%v, provider calls=%d", action, err, gateway.calls)
			}
		})
	}
}

func TestWritingCancelBeforeRegistrationPreventsAllEngines(t *testing.T) {
	gateway := &writingGatewayProbe{}
	d := writingTestDaemon(t, gateway)
	for _, method := range []string{"check", "transform", "voice"} {
		t.Run(method, func(t *testing.T) {
			request := "cancel-before-" + method
			if _, err := d.writingCancel(context.Background(), nil, protocol.WritingCancelParams{Request: request}); err != nil {
				t.Fatal(err)
			}
			var err error
			switch method {
			case "check":
				_, err = d.writingCheck(context.Background(), nil, protocol.WritingCheckParams{Request: request, Text: "This is an test.", Engine: "local"})
			case "transform":
				_, err = d.writingTransform(context.Background(), nil, protocol.WritingTransformParams{Request: request, Text: "Review this page.", Action: "rewrite"})
			case "voice":
				_, err = d.writingVoiceLearn(context.Background(), nil, protocol.WritingVoiceLearnParams{Request: request})
			}
			if !errors.Is(err, context.Canceled) || gateway.calls != 0 {
				t.Fatalf("early cancellation: error=%v, provider calls=%d", err, gateway.calls)
			}
			if _, _, err := d.writingContext(context.Background(), request); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled id started on replay: %v", err)
			}
		})
	}
}

func TestWritingCancelAfterRegistrationAndCleanup(t *testing.T) {
	d := writingTestDaemon(t, &writingGatewayProbe{})
	ctx, finish, err := d.writingContext(context.Background(), "running")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.writingCancel(context.Background(), nil, protocol.WritingCancelParams{Request: "running"}); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("running request was not cancelled: %v", ctx.Err())
	}
	finish()
	writingRequests.Lock()
	_, exists := writingRequests.active[writingRequestKey{daemon: d, id: "running"}]
	writingRequests.Unlock()
	if exists {
		t.Fatal("finished request remained registered")
	}
}

func TestWritingCancellationTombstonesExpireWithoutExpiringWork(t *testing.T) {
	d := writingTestDaemon(t, &writingGatewayProbe{})
	if _, err := d.writingCancel(context.Background(), nil, protocol.WritingCancelParams{Request: "expires"}); err != nil {
		t.Fatal(err)
	}
	ctx, finish, err := d.writingContext(context.Background(), "still-running")
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	writingRequests.Lock()
	expireWritingRequests(time.Now().Add(writingCancelLifetime + time.Second))
	_, expired := writingRequests.active[writingRequestKey{daemon: d, id: "expires"}]
	_, running := writingRequests.active[writingRequestKey{daemon: d, id: "still-running"}]
	writingRequests.Unlock()
	if expired || !running || ctx.Err() != nil {
		t.Fatalf("expiry removed active work or kept tombstone: expired=%v, running=%v, error=%v", expired, running, ctx.Err())
	}
	_, afterExpiry, err := d.writingContext(context.Background(), "expires")
	if err != nil {
		t.Fatal(err)
	}
	afterExpiry()
}

func TestWritingCancellationRegistryIsBounded(t *testing.T) {
	d := writingTestDaemon(t, &writingGatewayProbe{})
	for i := range writingRequestLimit {
		if _, err := d.writingCancel(context.Background(), nil, protocol.WritingCancelParams{Request: fmt.Sprintf("bounded-%d", i)}); err != nil {
			t.Fatalf("tombstone %d: %v", i, err)
		}
	}
	if _, err := d.writingCancel(context.Background(), nil, protocol.WritingCancelParams{Request: "overflow"}); err == nil {
		t.Fatal("unbounded cancellation tombstones")
	}
	if _, finish, err := d.writingContext(context.Background(), "overflow"); err == nil {
		finish()
		t.Fatal("registered new work beyond the registry limit")
	}
}

func TestWritingWordsPersistAcrossDaemonRestart(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	ctx := context.Background()
	first := writingTestDaemon(t, &writingGatewayProbe{})
	value, err := first.dispatch(ctx, nil, protocol.MWritingWordsAdd, []byte(`{"word":"  RYOKU  "}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := value.(protocol.WritingWords).Words; !reflect.DeepEqual(got, []string{"RYOKU"}) {
		t.Fatalf("added words=%v", got)
	}
	path := filepath.Join(paths.DataDir(), "words.txt")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("dictionary permissions=%v, error=%v", info, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "RYOKU\n" {
		t.Fatalf("dictionary=%q, error=%v", raw, err)
	}
	second := writingTestDaemon(t, &writingGatewayProbe{})
	value, err = second.dispatch(ctx, nil, protocol.MWritingWords, []byte(`{}`))
	if err != nil || !reflect.DeepEqual(value.(protocol.WritingWords).Words, []string{"RYOKU"}) {
		t.Fatalf("restarted dictionary=%#v, error=%v", value, err)
	}
	value, err = second.dispatch(ctx, nil, protocol.MWritingWordsRemove, []byte(`{"word":"Ryoku"}`))
	if err != nil || len(value.(protocol.WritingWords).Words) != 0 {
		t.Fatalf("removed dictionary=%#v, error=%v", value, err)
	}
}
