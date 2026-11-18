package main

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestValidateSightImage(t *testing.T) {
	runtime := t.TempDir()
	root := filepath.Join(runtime, "sight")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, "capture.png")
	outside := filepath.Join(runtime, "secret.png")
	for _, path := range []string{inside, outside} {
		if err := os.WriteFile(path, []byte("image"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "escape.png")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if got, err := validateSightImage(root, inside); err != nil || got != inside {
		t.Fatalf("inside = %q, %v", got, err)
	}
	for _, path := range []string{outside, link, root, "capture.png", filepath.Join(root, "missing.png"), filepath.Join(root, "..", "secret.png")} {
		if _, err := validateSightImage(root, path); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
}

func TestSightLanguageIntersection(t *testing.T) {
	installed := parseSightLanguages("List of available languages in /usr/share/tessdata/ (3):\neng\njpn\nscript/Latin\n")
	if !reflect.DeepEqual(installed, []string{"eng", "jpn", "script/Latin"}) {
		t.Fatalf("installed = %v", installed)
	}
	for _, test := range []struct{ requested, want []string }{
		{[]string{"jpn", "fra", "jpn"}, []string{"jpn"}}, {nil, []string{"eng"}}, {[]string{"fra"}, []string{"eng"}},
	} {
		if got := sightLanguageIntersection(test.requested, installed); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("intersection = %v, want %v", got, test.want)
		}
	}
}

func TestSightOCRUsesInstalledLanguagesAndUpscalesSmallImage(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "capture.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 20, 30))); err != nil {
		t.Fatal(err)
	}
	file.Close()
	original := runSightCommand
	defer func() { runSightCommand = original }()
	var calls []string
	runSightCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("unbounded OCR command")
		}
		calls = append(calls, name+" "+strings.Join(args, " "))
		switch name {
		case "tesseract":
			if len(args) == 1 {
				return []byte("eng\njpn\n"), nil
			}
			return []byte("The text is here and you can read it.\n"), nil
		case "magick":
			return nil, nil
		case "zbarimg":
			return []byte(`<barcodes xmlns="http://zbar.sourceforge.net/2008/barcode"><source><image><symbol type="QR-Code"><data>https://example.org</data></symbol></image></source></barcodes>`), nil
		}
		return nil, errors.New("unexpected command")
	}
	s := &sightService{root: root, options: sightOptions{OCRLanguages: []string{"jpn", "fra"}}, frame: sightFrame{Cards: []sightCard{{ID: "s1", Kind: "image", Image: path}}}}
	result, err := s.ocr("s1")
	if err != nil {
		t.Fatal(err)
	}
	got := result.(sightOCRResult)
	if got.Text != "The text is here and you can read it.\n" || got.Language != "en" || !reflect.DeepEqual(got.Codes, []sightCode{{Format: "QR-Code", Text: "https://example.org"}}) {
		t.Fatalf("OCR = %#v", got)
	}
	wire, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Language *string `json:"language"`
	}
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Language == nil || *decoded.Language != "en" {
		t.Fatalf("OCR language missing from JSON: %s", wire)
	}
	if len(calls) != 4 || !strings.HasPrefix(calls[1], "magick ") || !strings.Contains(calls[2], " stdout -l jpn --psm 3") {
		t.Fatalf("commands = %v", calls)
	}
}

func TestPrimaryOfferRetentionDoesNotReceive(t *testing.T) {
	client, server := clipSocketPair(t)
	defer client.Close()
	defer server.Close()
	ingested := false
	watcher := &clipWatcher{conn: client, offers: map[uint32]*clipOffer{10: {types: []string{"text/plain"}}, 11: {types: []string{"text/plain;charset=utf-8"}}}, ingest: func(string, []byte) { ingested = true }}
	primary := &primarySelectionState{}
	primary.set(watcher)
	if err := watcher.handleDeviceEvent(wlMessage{opcode: dataDevicePrimaryEvent, payload: wlUint(10)}); err != nil {
		t.Fatal(err)
	}
	if watcher.primaryID != 10 || primaryTextMime(watcher.offers[watcher.primaryID]) == "" {
		t.Fatal("latest offer not retained")
	}
	if err := watcher.handleDeviceEvent(wlMessage{opcode: dataDevicePrimaryEvent, payload: wlUint(11)}); err != nil {
		t.Fatal(err)
	}
	fakeExpectDestroy(t, server, 10)
	if watcher.offers[10] != nil || watcher.primaryID != 11 || ingested {
		t.Fatal("primary offer leaked into history or was not replaced")
	}
	_ = server.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
	if _, err := readWLMessage(server); err == nil {
		t.Fatal("primary selection sent an unsolicited receive")
	}
	_ = server.SetReadDeadline(time.Time{})
	watcher.offers[11].types = append(watcher.offers[11].types, passwordManagerHint)
	if primaryTextMime(watcher.offers[watcher.primaryID]) != "" {
		t.Fatal("password manager selection exposed")
	}
	if _, err := primary.read(); err == nil {
		t.Fatal("password manager selection read")
	}
	if err := watcher.handleDeviceEvent(wlMessage{opcode: dataDevicePrimaryEvent, payload: wlUint(0)}); err != nil {
		t.Fatal(err)
	}
	fakeExpectDestroy(t, server, 11)
	if primaryTextMime(watcher.offers[watcher.primaryID]) != "" {
		t.Fatal("cleared selection still available")
	}
}

func TestSightCardLifetime(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.png")
	second := filepath.Join(root, "second.png")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("capture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s := &sightService{root: root, topic: newStateTopic(), frame: sightFrame{Cards: []sightCard{}}}
	if _, err := s.open(sightCard{Kind: "image", Image: first}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.open(sightCard{Kind: "image", Image: second}); err != nil {
		t.Fatal(err)
	}
	firstID, secondID := s.frame.Cards[0].ID, s.frame.Cards[1].ID
	if firstID == secondID || firstID == "" {
		t.Fatal("card ids are not unique")
	}
	if err := s.close(firstID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("closed image remains: %v", err)
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("other card image removed: %v", err)
	}
	if len(s.frame.Cards) != 1 || s.frame.Cards[0].ID != secondID {
		t.Fatal("closing one card removed another")
	}
	if err := s.close("", true); err != nil {
		t.Fatal(err)
	}
	if len(s.frame.Cards) != 0 {
		t.Fatal("closeAll retained cards")
	}
	if _, err := os.Stat(second); !os.IsNotExist(err) {
		t.Fatalf("closeAll retained image: %v", err)
	}
}

func TestSightGateReasonsAndDefaults(t *testing.T) {
	s := &sightService{topic: newStateTopic()}
	s.setWatching(true)
	if s.frame.Watching || s.frame.Reason != "rashin-off" {
		t.Fatalf("off state = %#v", s.frame)
	}
	s.rashin = true
	s.setWatching(true)
	if s.frame.Watching || s.frame.Reason != "disabled" {
		t.Fatalf("disabled state = %#v", s.frame)
	}
	s.enabled = true
	s.setWatching(false)
	if s.frame.Watching || s.frame.Reason != "no-keyboard-access" {
		t.Fatalf("inaccessible state = %#v", s.frame)
	}
	s.setWatching(true)
	if !s.frame.Watching || s.frame.Reason != "" {
		t.Fatalf("watching state = %#v", s.frame)
	}
	defaults := sightOptionsFromFrame([]byte(`{}`))
	if !defaults.Enabled || !defaults.Highlight || defaults.TapKey != "Alt_R" || defaults.HighlightKey != "Alt_L" {
		t.Fatalf("defaults = %#v", defaults)
	}
	options := sightOptionsFromFrame([]byte(`{"sight":{"enabled":false,"highlight":false,"tapKey":"bad","highlightKey":"Super_L","ocrLanguages":[]}}`))
	if options.Enabled || options.Highlight || options.TapKey != "Alt_R" || options.HighlightKey != "Alt_L" || len(options.OCRLanguages) != 0 {
		t.Fatalf("explicit settings = %#v", options)
	}
}

func TestSightStartupRemovesStaleImages(t *testing.T) {
	runtime := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := filepath.Join(runtime, "ryoku", "sight")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"stale.png", "frame-stale-DP-1.png"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("capture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	d := &daemon{quit: make(chan struct{})}
	settings := d.registerTopic("settings")
	d.registerTopic("presence")
	stopped := make(chan struct{})
	settings.onDemand(func(active bool) {
		if !active {
			close(stopped)
		}
	})
	close(d.quit)
	d.startSight()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Sight configuration watcher did not stop")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("startup retained %d stale entries", len(entries))
	}
	if len(d.sight.frame.Cards) != 0 {
		t.Fatalf("startup restored stale cards: %#v", d.sight.frame.Cards)
	}
}

func TestSightLanguageCalls(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	d := &daemon{quit: make(chan struct{}), clip: &clipState{}}
	settings := d.registerTopic("settings")
	d.registerTopic("presence")
	stopped := make(chan struct{})
	settings.onDemand(func(active bool) {
		if !active {
			close(stopped)
		}
	})
	close(d.quit)
	d.startSight()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Sight configuration watcher did not stop")
	}

	for _, test := range []struct{ args, want string }{
		{`{"text":"The text is here and you can read it."}`, `{"language":"en"}`},
		{`{"text":"我們會在學校門口見面。"}`, `{"language":"zh_TW"}`},
		{`{"text":""}`, `{"language":""}`},
		{`{}`, `{"language":""}`},
	} {
		result, err := d.callHandler("sight.detect")(json.RawMessage(test.args))
		if err != nil {
			t.Fatal(err)
		}
		wire, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		if string(wire) != test.want {
			t.Errorf("sight.detect(%s) = %s, want %s", test.args, wire, test.want)
		}
	}
	if _, err := d.callHandler("sight.detect")(json.RawMessage(`{"text":12}`)); err == nil {
		t.Fatal("sight.detect accepted non-text input")
	}

	assertCardLanguage := func(text, language string) {
		t.Helper()
		var frame struct {
			Cards []struct {
				Kind     string  `json:"kind"`
				Text     string  `json:"text"`
				Language *string `json:"language"`
			} `json:"cards"`
		}
		d.sight.topic.mu.Lock()
		err := json.Unmarshal(d.sight.topic.last, &frame)
		d.sight.topic.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		if len(frame.Cards) == 0 {
			t.Fatal("text card was not published")
		}
		card := frame.Cards[len(frame.Cards)-1]
		if card.Kind != "text" || card.Text != text || card.Language == nil || *card.Language != language {
			t.Fatalf("text card JSON = %#v, want text %q and language %q", card, text, language)
		}
	}
	for _, test := range []struct{ text, language string }{
		{"The text is here and you can read it.", "en"},
		{"Hello world", ""},
	} {
		args, err := json.Marshal(map[string]string{"text": test.text, "language": "wrong"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.callHandler("sight.open")(args); err != nil {
			t.Fatal(err)
		}
		assertCardLanguage(test.text, test.language)
	}

	const selected = "Цей текст українською мовою пояснює, як працює система."
	d.sight.frame.Highlight = &sightHighlight{ID: "highlight", Text: selected, Language: detectSightLanguage(selected)}
	if _, err := d.callHandler("sight.highlightOpen")(json.RawMessage(`{"id":"highlight","mode":"translate","x":12,"y":34,"output":"DP-1"}`)); err != nil {
		t.Fatal(err)
	}
	assertCardLanguage(selected, "uk")
}
