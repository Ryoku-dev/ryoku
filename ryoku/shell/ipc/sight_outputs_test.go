package main

import (
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	wm "ryoku-wm"
)

func TestSightOCRBoxesGroupAndNormalize(t *testing.T) {
	raw := "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
		"1\t1\t0\t0\t0\t0\t0\t0\t200\t100\t-1\t\n" +
		"5\t1\t1\t1\t1\t1\t20\t10\t40\t10\t95\tHello\n" +
		"5\t1\t1\t1\t1\t2\t70\t8\t30\t14\t95\tworld\n" +
		"5\t1\t1\t1\t2\t1\t0\t40\t50\t20\t95\tNext\n" +
		"5\t1\t2\t1\t1\t1\t190\t90\t30\t30\t95\tEdge\n" +
		"5\t1\t2\t1\t2\t1\t0\t0\t10\t10\t95\t \n"
	lines, err := parseSightOCRBoxes([]byte(raw), 200, 100)
	if err != nil {
		t.Fatal(err)
	}
	want := []sightOCRLine{
		{Text: "Hello world", X: 0.1, Y: 0.08, Width: 0.4, Height: 0.14},
		{Text: "Next", X: 0, Y: 0.4, Width: 0.25, Height: 0.2},
		{Text: "Edge", X: 0.95, Y: 0.9, Width: 0.05, Height: 0.1},
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines = %#v, want %#v", lines, want)
	}
	if _, err := parseSightOCRBoxes(nil, 0, 100); err == nil {
		t.Fatal("accepted zero image width")
	}
}

func TestSightOCRBoxesUsesCardAndLanguages(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "card.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 200, 100))); err != nil {
		t.Fatal(err)
	}
	file.Close()
	original := runSightCommand
	t.Cleanup(func() { runSightCommand = original })
	var calls [][]string
	runSightCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if _, bounded := ctx.Deadline(); !bounded {
			t.Error("OCR command has no deadline")
		}
		calls = append(calls, append([]string{name}, args...))
		if reflect.DeepEqual(args, []string{"--list-langs"}) {
			return []byte("List of available languages (2):\neng\nfra\n"), nil
		}
		return []byte("5\t1\t1\t1\t1\t1\t20\t10\t40\t10\t95\tBonjour\n"), nil
	}
	s := &sightService{root: root, rashin: true, options: sightOptions{On: true, OCRLanguages: []string{"fra", "jpn"}}, frame: sightFrame{Cards: []sightCard{{ID: "c1", Kind: "image", Image: path}}}}
	result, err := s.ocrBoxes("c1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Width != 200 || result.Height != 100 || len(result.Lines) != 1 {
		t.Fatalf("boxes = %#v", result)
	}
	want := []string{"tesseract", path, "stdout", "-l", "fra", "--psm", "3", "tsv"}
	if len(calls) != 2 || !reflect.DeepEqual(calls[1], want) {
		t.Fatalf("commands = %#v", calls)
	}
	if _, err := s.ocrBoxes("missing"); err == nil || err.Error() != "bad-image" {
		t.Fatalf("missing image error = %v", err)
	}
}

func TestSightVoicesParseEspeak(t *testing.T) {
	raw := "Pty Language       Age/Gender VoiceName          File                 Other Languages\n" +
		" 5  en-gb          --/M      English_(Great_Britain) gmw/en           (en 2)\n" +
		" 5  en-us          --/M      English_(America)   gmw/en-US            (en 3)\n" +
		" 5  fr-fr          --/M      French_(France)     roa/fr               (fr 5)\n" +
		" 7  en-us          --/F      Duplicate           other\n" +
		"malformed\n"
	got := parseSightVoices(raw)
	want := []sightVoice{
		{ID: "en-gb", Name: "English (Great Britain)", Language: "en-gb"},
		{ID: "en-us", Name: "English (America)", Language: "en-us"},
		{ID: "fr-fr", Name: "French (France)", Language: "fr-fr"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("voices = %#v", got)
	}
}

func TestSightVoicesIncludesPiperAndMissingEngine(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	dir := sightPiperDirectory()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "en_US-lessac-medium.onnx"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.json"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	original := sightOutputLookPath
	t.Cleanup(func() { sightOutputLookPath = original })
	sightOutputLookPath = func(name string) (string, error) {
		if name == "piper" {
			return name, nil
		}
		return "", exec.ErrNotFound
	}
	result := (&sightService{}).voices()
	if result.Engine != "piper" || result.Missing != "espeak-ng" || len(result.Voices) != 1 {
		t.Fatalf("voices = %#v", result)
	}
	if result.Voices[0].ID != "piper:en_US-lessac-medium.onnx" || result.Voices[0].Language != "en-US" {
		t.Fatalf("piper voice = %#v", result.Voices[0])
	}
}

type sightOutputCall struct {
	name  string
	args  []string
	input string
}

func TestSightReplaceTypeAndUndoArguments(t *testing.T) {
	originalRun, originalPath := runSightInputCommand, sightOutputLookPath
	t.Cleanup(func() { runSightInputCommand, sightOutputLookPath = originalRun, originalPath })
	sightOutputLookPath = func(name string) (string, error) { return name, nil }
	calls := []sightOutputCall{}
	runSightInputCommand = func(ctx context.Context, input, name string, args ...string) ([]byte, error) {
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 5*time.Second {
			t.Error("missing five second deadline")
		}
		calls = append(calls, sightOutputCall{name: name, args: args, input: input})
		return nil, nil
	}
	s := &sightService{rashin: true, options: sightOptions{On: true, Features: map[string]bool{"inplace": true}}}
	if err := s.replace("-hello\nworld", "type"); err != nil {
		t.Fatal(err)
	}
	if err := s.undoReplace(); err != nil {
		t.Fatal(err)
	}
	want := []sightOutputCall{
		{name: "wtype", args: []string{"-d", "3", "--", "-hello\nworld"}},
		{name: "wtype", args: []string{"-M", "ctrl", "-k", "z", "-m", "ctrl"}},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v", calls)
	}
	if err := s.replace(" \n", "type"); err == nil || err.Error() != "no-text" {
		t.Fatalf("empty error = %v", err)
	}
	sightOutputLookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	if err := s.replace("hello", "type"); err == nil || err.Error() != "no-wtype" {
		t.Fatalf("missing wtype = %v", err)
	}
}

func TestSightTerminalSelectionDecision(t *testing.T) {
	for _, app := range []string{"kitty", "KiTtY", "foot", "Alacritty", "org.wezfurlong.wezterm", "com.mitchellh.ghostty", "org.kde.konsole", "org.gnome.Console", "org.gnome.Terminal.desktop", "org.gnome.Ptyxis", "XTerm", "UXTerm"} {
		if !sightTerminalApp(app, "") {
			t.Errorf("%q was not recognized as a terminal", app)
		}
	}
	for _, app := range []string{"", "firefox", "org.kde.kate", "footnote", "kitty-browser"} {
		if sightTerminalApp(app, "") {
			t.Errorf("%q was treated as a terminal", app)
		}
	}
	for _, app := range []string{"custom-term", "ORG.EXAMPLE.CUSTOM-TERM.desktop"} {
		if !sightTerminalApp(app, "/opt/bin/custom-term --new-window") {
			t.Errorf("configured terminal %q was not recognized", app)
		}
	}
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "ryoku"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "ryoku", "desktop.json"), []byte(`{"desktop":{"apps":{"terminal":"/opt/bin/custom-term --new-window"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	s := &sightService{daemon: &daemon{wmWindows: []wm.Window{{AppID: "firefox", FocusOrder: 1}, {AppID: "org.example.custom-term", FocusOrder: 0}}}}
	if !s.focusedTerminal() {
		t.Fatal("the focused configured terminal was missed")
	}
	s.daemon.wmWindows[1].AppID = "org.kde.kate"
	if s.focusedTerminal() {
		t.Fatal("the focused editor was treated as a terminal")
	}
}

func TestSightTerminalResultStaysOnClipboardWithoutInputOrRestore(t *testing.T) {
	originalRun, originalPath := runSightInputCommand, sightOutputLookPath
	t.Cleanup(func() { runSightInputCommand, sightOutputLookPath = originalRun, originalPath })
	sightOutputLookPath = func(string) (string, error) {
		t.Fatal("copy-only terminal result checked for wtype")
		return "", exec.ErrNotFound
	}
	var calls []sightOutputCall
	runSightInputCommand = func(_ context.Context, input, name string, args ...string) ([]byte, error) {
		calls = append(calls, sightOutputCall{name: name, input: input, args: args})
		return nil, nil
	}
	s := &sightService{rashin: true, options: sightOptions{On: true, Features: map[string]bool{"inplace": true}}}
	if err := s.replace("corrected terminal selection", "copy"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []sightOutputCall{{name: "wl-copy", input: "corrected terminal selection"}}) {
		t.Fatalf("terminal output ran input or restored the clipboard: %#v", calls)
	}
}

func TestSightReplacePasteRestoresOnlyOurClipboard(t *testing.T) {
	for _, newer := range []bool{false, true} {
		t.Run(map[bool]string{false: "restore", true: "leave-new-copy"}[newer], func(t *testing.T) {
			originalRun, originalPath := runSightInputCommand, sightOutputLookPath
			t.Cleanup(func() { runSightInputCommand, sightOutputLookPath = originalRun, originalPath })
			sightOutputLookPath = func(name string) (string, error) { return name, nil }
			var mu sync.Mutex
			clipboard := "before\n"
			var calls []sightOutputCall
			checked := make(chan struct{}, 1)
			restored := make(chan struct{}, 1)
			reads := 0
			runSightInputCommand = func(ctx context.Context, input, name string, args ...string) ([]byte, error) {
				mu.Lock()
				defer mu.Unlock()
				calls = append(calls, sightOutputCall{name: name, args: args, input: input})
				switch name {
				case "wl-paste":
					reads++
					if reads == 2 {
						checked <- struct{}{}
					}
					return []byte(clipboard), nil
				case "wl-copy":
					clipboard = input
					if input == "before\n" {
						restored <- struct{}{}
					}
				case "wtype":
					if newer {
						clipboard = "a newer copy"
					}
				}
				return nil, nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &sightService{ctx: ctx, rashin: true, options: sightOptions{On: true, Features: map[string]bool{"inplace": true}}}
			if err := s.replace("replacement", "paste"); err != nil {
				t.Fatal(err)
			}
			select {
			case <-checked:
			case <-time.After(3 * time.Second):
				t.Fatal("clipboard was not checked")
			}
			if !newer {
				select {
				case <-restored:
				case <-time.After(time.Second):
					t.Fatal("clipboard was not restored")
				}
			}
			mu.Lock()
			defer mu.Unlock()
			want := []sightOutputCall{
				{name: "wl-paste", args: []string{"-n", "-t", "text"}},
				{name: "wl-copy", input: "replacement"},
				{name: "wtype", args: []string{"-M", "ctrl", "-k", "v", "-m", "ctrl"}},
				{name: "wl-paste", args: []string{"-n", "-t", "text"}},
			}
			if !newer {
				want = append(want, sightOutputCall{name: "wl-copy", input: "before\n"})
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls = %#v", calls)
			}
		})
	}
}

func TestSightOutputHelperProcess(t *testing.T) {
	if os.Getenv("RYOKU_SIGHT_OUTPUT_HELPER") != "1" {
		return
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	if os.Getenv("RYOKU_SIGHT_OUTPUT_WAIT") == "1" {
		time.Sleep(time.Minute)
	}
	os.Exit(0)
}

func fakeSightSpeechCommand(t *testing.T, wait bool) *[]sightOutputCall {
	t.Helper()
	originalCommand, originalPath := sightOutputCommand, sightOutputLookPath
	t.Cleanup(func() { sightOutputCommand, sightOutputLookPath = originalCommand, originalPath })
	calls := &[]sightOutputCall{}
	sightOutputLookPath = func(name string) (string, error) { return name, nil }
	sightOutputCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		*calls = append(*calls, sightOutputCall{name: name, args: append([]string(nil), args...)})
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSightOutputHelperProcess$")
		cmd.Env = append(os.Environ(), "RYOKU_SIGHT_OUTPUT_HELPER=1")
		if wait {
			cmd.Env = append(cmd.Env, "RYOKU_SIGHT_OUTPUT_WAIT=1")
		}
		return cmd
	}
	return calls
}

func TestSightSpeakArgumentsAndRate(t *testing.T) {
	calls := fakeSightSpeechCommand(t, false)
	s := &sightService{topic: newStateTopic(), rashin: true, options: sightOptions{On: true, Features: map[string]bool{"readaloud": true}}}
	for _, rate := range []float64{0.75, 1, 1.5} {
		if err := s.speak("read", "c1", "Read this", "en-gb", rate); err != nil {
			t.Fatal(err)
		}
		s.mu.Lock()
		job := s.speech
		s.mu.Unlock()
		if job != nil {
			<-job.done
		}
	}
	want := []sightOutputCall{
		{name: "espeak-ng", args: []string{"-v", "en-gb", "-s", "131", "--stdin"}},
		{name: "espeak-ng", args: []string{"-v", "en-gb", "-s", "175", "--stdin"}},
		{name: "espeak-ng", args: []string{"-v", "en-gb", "-s", "263", "--stdin"}},
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("commands = %#v", *calls)
	}
	if sightSpeechRate(math.NaN()) != 1 || sightSpeechRate(4) != 1.5 || sightSpeechRate(0.5) != 0.75 {
		t.Fatal("invalid speech rate mapping")
	}
}

func TestSightSpeakStopAndLifetimeCancel(t *testing.T) {
	fakeSightSpeechCommand(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &sightService{ctx: ctx, topic: newStateTopic(), rashin: true, options: sightOptions{On: true, Features: map[string]bool{"readaloud": true}}}
	if err := s.speak("read", "c1", "Read this", "", 1); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	if s.frame.Speaking == nil || s.frame.Speaking.ID != "read" || s.frame.Speaking.Card != "c1" || s.frame.Speaking.Started == 0 {
		t.Errorf("speaking frame = %#v", s.frame.Speaking)
	}
	s.mu.Unlock()
	s.stopSpeaking()
	s.mu.Lock()
	if s.frame.Speaking != nil || s.speech != nil {
		t.Error("stop left a speaking frame")
	}
	s.mu.Unlock()
	if err := s.speak("next", "c2", "Read again", "", 1); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	job := s.speech
	s.mu.Unlock()
	cancel()
	select {
	case <-job.done:
	case <-time.After(3 * time.Second):
		t.Fatal("speech survived lifetime cancellation")
	}
}

func TestSightSpeakPiperAndFallback(t *testing.T) {
	calls := fakeSightSpeechCommand(t, false)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := os.MkdirAll(sightPiperDirectory(), 0700); err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(sightPiperDirectory(), "en_US-test.onnx")
	if err := os.WriteFile(model, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s := &sightService{root: filepath.Join(t.TempDir(), "fresh", "sight"), topic: newStateTopic(), rashin: true, options: sightOptions{On: true, Features: map[string]bool{"readaloud": true}}}
	if err := s.speak("read", "c1", "Read this", "piper:en_US-test.onnx", 1.25); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(s.root); err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		t.Fatalf("fresh speech root not created privately: %v, %v", info, err)
	}
	s.mu.Lock()
	job := s.speech
	s.mu.Unlock()
	if job != nil {
		<-job.done
	}
	if len(*calls) != 2 || (*calls)[0].name != "piper" || (*calls)[1].name != "pw-play" {
		t.Fatalf("piper commands = %#v", *calls)
	}
	args := (*calls)[0].args
	if len(args) != 6 || !reflect.DeepEqual(args[:3], []string{"--model", model, "--output_file"}) || !reflect.DeepEqual(args[4:], []string{"--length_scale", "0.8000"}) {
		t.Fatalf("piper args = %#v", args)
	}
	if _, err := os.Stat(args[3]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("speech temporary file remains: %v", err)
	}
	if err := s.speak("read", "c1", "Read this", "piper:missing.onnx", 1); err != nil {
		t.Fatal(err)
	}
	s.stopSpeaking()
	last := (*calls)[len(*calls)-1]
	if last.name != "espeak-ng" || strings.Contains(strings.Join(last.args, " "), "piper:") {
		t.Fatalf("fallback = %#v", last)
	}
}

func TestSightSpeakDisabledAndMissingEngine(t *testing.T) {
	calls := fakeSightSpeechCommand(t, false)
	s := &sightService{topic: newStateTopic(), rashin: true}
	if err := s.speak("read", "c1", "Read this", "", 1); err == nil || err.Error() != "failed" {
		t.Fatalf("disabled speech error = %v", err)
	}
	if len(*calls) != 0 {
		t.Fatal("disabled speech started an engine")
	}
	s.options.On = true
	s.options.Features = map[string]bool{"readaloud": true}
	sightOutputLookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	if err := s.speak("read", "c1", "Read this", "", 1); err == nil || err.Error() != "no-engine" {
		t.Fatalf("missing engine error = %v", err)
	}
}

func TestSightSpeakUsesConfiguredVoiceAndRate(t *testing.T) {
	calls := fakeSightSpeechCommand(t, false)
	s := &sightService{topic: newStateTopic(), rashin: true, options: sightOptions{On: true, Features: map[string]bool{"readaloud": true}}}
	s.options.ReadAloud.Voice, s.options.ReadAloud.Rate = "fr-fr", 1.25
	if err := s.speak("read", "c1", "Bonjour", "", 0); err != nil {
		t.Fatal(err)
	}
	s.stopSpeaking()
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0].args, []string{"-v", "fr-fr", "-s", "219", "--stdin"}) {
		t.Fatalf("configured speech = %#v", *calls)
	}
}

func TestSightInputRejectsDisabledGates(t *testing.T) {
	originalRun := runSightInputCommand
	t.Cleanup(func() { runSightInputCommand = originalRun })
	runSightInputCommand = func(context.Context, string, string, ...string) ([]byte, error) {
		t.Error("disabled input ran a command")
		return nil, nil
	}
	for _, gate := range []string{"master", "inplace", "rashin"} {
		t.Run(gate, func(t *testing.T) {
			s := &sightService{rashin: true, options: sightOptions{On: true, Features: map[string]bool{"inplace": true}}}
			switch gate {
			case "master":
				s.options.On = false
			case "inplace":
				s.options.Features["inplace"] = false
			case "rashin":
				s.rashin = false
			}
			if err := s.replace("text", "type"); err == nil || err.Error() != "failed" {
				t.Fatalf("disabled replace error = %v", err)
			}
			if err := s.undoReplace(); err == nil || err.Error() != "failed" {
				t.Fatalf("disabled undo error = %v", err)
			}
		})
	}
}

func TestSightInputCancellationRestoresClipboardAndRejectsQueuedInput(t *testing.T) {
	for _, gate := range []string{"master", "inplace"} {
		t.Run(gate, func(t *testing.T) {
			originalRun, originalPath := runSightInputCommand, sightOutputLookPath
			t.Cleanup(func() { runSightInputCommand, sightOutputLookPath = originalRun, originalPath })
			sightOutputLookPath = func(name string) (string, error) { return name, nil }
			var mu sync.Mutex
			clipboard := "the user's clipboard"
			typed := 0
			started := make(chan struct{})
			runSightInputCommand = func(ctx context.Context, input, name string, args ...string) ([]byte, error) {
				switch name {
				case "wl-paste":
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					mu.Lock()
					defer mu.Unlock()
					return []byte(clipboard), nil
				case "wl-copy":
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					mu.Lock()
					clipboard = input
					mu.Unlock()
				case "wtype":
					mu.Lock()
					typed++
					mu.Unlock()
					close(started)
					<-ctx.Done()
					return nil, ctx.Err()
				}
				return nil, nil
			}
			s := &sightService{rashin: true, options: sightOptions{On: true, Features: map[string]bool{"inplace": true}}}
			first := make(chan error, 1)
			go func() { first <- s.replace("temporary", "paste") }()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("paste never started")
			}
			queued := make(chan error, 1)
			go func() { queued <- s.undoReplace() }()
			s.mu.Lock()
			if gate == "master" {
				s.options.On = false
			} else {
				s.options.Features["inplace"] = false
			}
			s.cancelInputLocked()
			s.mu.Unlock()
			for _, result := range []<-chan error{first, queued} {
				select {
				case err := <-result:
					if err == nil || err.Error() != "failed" {
						t.Fatalf("cancelled input error = %v", err)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("cancelled input did not stop")
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if clipboard != "the user's clipboard" || typed != 1 {
				t.Fatalf("clipboard = %q, input commands = %d", clipboard, typed)
			}
		})
	}
}

func TestSightOverlappingPastesKeepOriginalClipboard(t *testing.T) {
	originalRun, originalPath := runSightInputCommand, sightOutputLookPath
	t.Cleanup(func() { runSightInputCommand, sightOutputLookPath = originalRun, originalPath })
	sightOutputLookPath = func(name string) (string, error) { return name, nil }
	var mu sync.Mutex
	clipboard := "original"
	var copies, typed []string
	firstPasted := make(chan struct{})
	runSightInputCommand = func(ctx context.Context, input, name string, args ...string) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		switch name {
		case "wl-paste":
			return []byte(clipboard), nil
		case "wl-copy":
			clipboard = input
			copies = append(copies, input)
		case "wtype":
			typed = append(typed, clipboard)
			if len(typed) == 1 {
				close(firstPasted)
			}
		}
		return nil, nil
	}
	s := &sightService{rashin: true, options: sightOptions{On: true, Features: map[string]bool{"inplace": true}}}
	first, second := make(chan error, 1), make(chan error, 1)
	go func() { first <- s.replace("A", "paste") }()
	select {
	case <-firstPasted:
	case <-time.After(2 * time.Second):
		t.Fatal("first paste did not start")
	}
	go func() { second <- s.replace("B", "paste") }()
	for _, result := range []<-chan error{first, second} {
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(4 * time.Second):
			t.Fatal("serialized paste did not finish")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if clipboard != "original" || !reflect.DeepEqual(copies, []string{"A", "original", "B", "original"}) || !reflect.DeepEqual(typed, []string{"A", "B"}) {
		t.Fatalf("clipboard = %q, copies = %v, typed = %v", clipboard, copies, typed)
	}
}

func TestSightOCRBoxesCancelsOnCloseMasterOffAndSupersedingRequest(t *testing.T) {
	for _, action := range []string{"close", "master-off", "supersede"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "card.png")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 100, 100))); err != nil {
				t.Fatal(err)
			}
			file.Close()
			original := runSightCommand
			t.Cleanup(func() { runSightCommand = original })
			var mu sync.Mutex
			requests := 0
			started := make(chan struct{})
			runSightCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if reflect.DeepEqual(args, []string{"--list-langs"}) {
					return []byte("eng\n"), nil
				}
				mu.Lock()
				requests++
				first := requests == 1
				mu.Unlock()
				if first {
					close(started)
					<-ctx.Done()
					return nil, ctx.Err()
				}
				return []byte("5\t1\t1\t1\t1\t1\t0\t0\t10\t10\t95\tText\n"), nil
			}
			s := &sightService{root: root, topic: newStateTopic(), rashin: true, options: sightOptions{On: true}, frame: sightFrame{Cards: []sightCard{{ID: "c1", Kind: "image", Image: path}}}}
			first := make(chan error, 1)
			go func() { _, err := s.ocrBoxes("c1"); first <- err }()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("OCR did not start")
			}
			switch action {
			case "close":
				if err := s.close("c1", false); err != nil {
					t.Fatal(err)
				}
			case "master-off":
				s.mu.Lock()
				s.options.On = false
				s.cancelOCRLocked("")
				s.mu.Unlock()
			case "supersede":
				if result, err := s.ocrBoxes("c1"); err != nil || len(result.Lines) != 1 {
					t.Fatalf("superseding OCR result = %#v, %v", result, err)
				}
			}
			select {
			case err := <-first:
				if err == nil {
					t.Fatal("cancelled OCR returned success")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("OCR survived cancellation")
			}
		})
	}
}
