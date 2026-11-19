package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type sightCard struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Image    string  `json:"image"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	Output   string  `json:"output"`
	Mode     string  `json:"mode"`
	Text     string  `json:"text"`
	Language string  `json:"language"`
	App      string  `json:"app"`
	Source   string  `json:"source"`
	Plate    bool    `json:"plate"`
	Created  int64   `json:"created"`
}
type sightHighlight struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Language string `json:"language"`
	Output   string `json:"output"`
	App      string `json:"app"`
	Created  int64  `json:"created"`
}
type sightFrame struct {
	Watching  bool            `json:"watching"`
	Reason    string          `json:"reason"`
	Hidden    bool            `json:"hidden"`
	Cards     []sightCard     `json:"cards"`
	Select    *sightSelection `json:"select"`
	Highlight *sightHighlight `json:"highlight"`
	InPlace   *sightInPlace   `json:"inplace"`
	Speaking  *sightSpeaking  `json:"speaking"`
}

type sightSpeaking struct {
	ID      string `json:"id"`
	Card    string `json:"card"`
	Started int64  `json:"started"`
}
type sightOptions struct {
	On       bool            `json:"on"`
	Features map[string]bool `json:"features"`
	InPlace  struct {
		Key    string `json:"key"`
		Action string `json:"action"`
		Paste  bool   `json:"paste"`
	} `json:"inplace"`
	Quiet struct {
		Apps   []string `json:"apps"`
		Snooze int      `json:"snooze"`
	} `json:"quiet"`
	ReadAloud struct {
		Voice string  `json:"voice"`
		Rate  float64 `json:"rate"`
	} `json:"readaloud"`
	Enabled      bool     `json:"enabled"`
	Highlight    bool     `json:"highlight"`
	TapKey       string   `json:"tapKey"`
	HighlightKey string   `json:"highlightKey"`
	Mode         string   `json:"mode"`
	OCRLanguages []string `json:"ocrLanguages"`
}

func sightOptionsFromFrame(data []byte) sightOptions {
	options := sightOptions{Enabled: true, Highlight: true, TapKey: "Alt_R", HighlightKey: "Alt_L", Mode: "explain", OCRLanguages: []string{"eng"}}
	options.On = true
	options.Features = make(map[string]bool)
	options.InPlace.Key, options.InPlace.Action, options.InPlace.Paste = "Control_R", "auto", true
	options.Quiet.Apps = []string{}
	options.ReadAloud.Rate = 1
	var frame struct {
		Sight json.RawMessage `json:"sight"`
	}
	if json.Unmarshal(data, &frame) == nil && len(frame.Sight) != 0 {
		_ = json.Unmarshal(frame.Sight, &options)
	}
	options.TapKey = sightGestureKey(options.TapKey, "Alt_R").Name
	options.HighlightKey = sightGestureKey(options.HighlightKey, "Alt_L").Name
	options.InPlace.Key = sightGestureKey(options.InPlace.Key, "Control_R").Name
	if !sightInPlaceAction(options.InPlace.Action) {
		options.InPlace.Action = "auto"
	}
	if math.IsNaN(options.ReadAloud.Rate) || math.IsInf(options.ReadAloud.Rate, 0) {
		options.ReadAloud.Rate = 1
	}
	options.ReadAloud.Rate = math.Max(0.75, math.Min(1.5, options.ReadAloud.Rate))
	options.ReadAloud.Voice = strings.TrimSpace(options.ReadAloud.Voice)
	if options.Quiet.Snooze != 15 && options.Quiet.Snooze != 60 {
		options.Quiet.Snooze = 0
	}
	apps := make([]string, 0, len(options.Quiet.Apps))
	seen := make(map[string]bool)
	for _, app := range options.Quiet.Apps {
		app = strings.TrimSpace(app)
		if app != "" && !seen[app] {
			apps = append(apps, app)
			seen[app] = true
		}
	}
	options.Quiet.Apps = apps
	return options
}

func (o sightOptions) feature(id string) bool { return o.On && o.Features[id] }

func (o sightOptions) gestures() bool {
	return o.On && (o.Enabled || o.Highlight || o.feature("inplace"))
}

func sightGestureOptionsChanged(a, b sightOptions) bool {
	return a.On != b.On || a.Enabled != b.Enabled || a.Highlight != b.Highlight ||
		a.TapKey != b.TapKey || a.HighlightKey != b.HighlightKey ||
		a.feature("inplace") != b.feature("inplace") || a.InPlace.Key != b.InPlace.Key
}

var sightModePattern = regexp.MustCompile(`^[a-z]{2,16}$`)

func sightMode(mode, fallback string) string {
	if sightModePattern.MatchString(mode) {
		return mode
	}
	if sightModePattern.MatchString(fallback) {
		return fallback
	}
	return "explain"
}

type sightService struct {
	mu             sync.Mutex
	daemon         *daemon
	topic          *stateTopic
	root           string
	frame          sightFrame
	options        sightOptions
	enabled        bool
	rashin         bool
	next           uint64
	highlightEpoch uint64
	captureMu      sync.Mutex
	capture        *sightCapture
	previous       map[string]sightBox
	snapshot       func() ([]sightCaptureFrame, string)
	capturer       func(context.Context, sightCaptureFrame) error
	locked         func() bool
	ctx            context.Context
	speechMu       sync.Mutex
	speech         *sightSpeech
	snoozedApp     string
	snoozedUntil   time.Time
	inplaceEpoch   uint64
	inputMu        sync.Mutex
	inputCancel    context.CancelFunc
	ocrRequests    map[string]*sightOCRRequest
}

func (s *sightService) publishLocked() { body, _ := json.Marshal(s.frame); s.topic.publish(body) }
func (s *sightService) setWatching(watching bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frame.Watching = watching && s.options.On && s.enabled && s.rashin
	switch {
	case !s.options.On:
		s.frame.Reason = "theia-off"
	case !s.rashin:
		s.frame.Reason = "rashin-off"
	case !s.enabled:
		s.frame.Reason = "disabled"
	case !watching:
		s.frame.Reason = "no-keyboard-access"
	default:
		s.frame.Reason = ""
	}
	s.publishLocked()
}
func validateSightImage(root, path string) (resolved string, err error) {
	defer func() {
		if err != nil {
			log.Printf("ryoku-shell: sight image: %v", err)
			err = errors.New("bad-image")
		}
	}()
	if !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return "", errors.New("invalid-image-path")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || relative == "." || relative == ".." || len(relative) >= 3 && relative[:3] == ".."+string(os.PathSeparator) {
		return "", errors.New("invalid-image-path")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("invalid-image-path")
	}
	return resolved, nil
}
func (s *sightService) open(card sightCard) (any, error) {
	if card.Kind == "image" {
		path, err := validateSightImage(s.root, card.Image)
		if err != nil {
			return nil, err
		}
		card.Image = path
	}
	if card.Kind == "text" {
		card.Language = detectSightLanguage(card.Text)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.options.On {
		return nil, errors.New("theia-off")
	}
	card.App = s.focusedApp()
	card.Source = "open"
	return s.openLocked(card)
}
func (s *sightService) openLocked(card sightCard) (any, error) {
	card.Mode = sightMode(card.Mode, s.options.Mode)
	s.next++
	card.ID = fmt.Sprintf("s%x-%x", time.Now().UnixNano(), s.next)
	card.Created = time.Now().UnixMilli()
	s.frame.Cards = append(s.frame.Cards, card)
	s.frame.Hidden = false
	s.publishLocked()
	return map[string]any{"id": card.ID}, nil
}
func (s *sightService) close(id string, all bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if all {
		s.cancelOCRLocked("")
	} else {
		s.cancelOCRLocked(id)
	}
	retained := make([]sightCard, 0, len(s.frame.Cards))
	var firstErr error
	for _, card := range s.frame.Cards {
		if !all && card.ID != id {
			retained = append(retained, card)
			continue
		}
		if card.Image != "" {
			path, err := validateSightImage(s.root, card.Image)
			if err == nil {
				err = os.Remove(path)
			}
			if err != nil && !os.IsNotExist(err) && firstErr == nil {
				log.Printf("ryoku-shell: sight close: %v", err)
				firstErr = errors.New("bad-image")
			}
		}
	}
	s.frame.Cards = retained
	s.publishLocked()
	return firstErr
}
func (s *sightService) trigger() error {
	s.mu.Lock()
	on := s.options.On
	s.mu.Unlock()
	if !on {
		return errors.New("theia-off")
	}
	if s.sessionLocked() {
		return errors.New("locked")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		select {
		case <-s.daemon.quit:
			cancel()
		case <-stopped:
		}
	}()
	if id := s.beginCapture(ctx); id != "" {
		s.fireCapture(ctx, id)
	}
	return nil
}
func (d *daemon) startSight() {
	ctx, cancel := context.WithCancel(context.Background())
	s := &sightService{daemon: d, topic: d.registerTopic("sight"), ctx: ctx, options: sightOptionsFromFrame(nil), root: filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "ryoku", "sight"), frame: sightFrame{Reason: "rashin-off", Cards: []sightCard{}}}
	go func() { <-d.quit; cancel() }()
	d.sight = s
	if info, err := os.Lstat(s.root); err == nil && info.IsDir() {
		entries, err := os.ReadDir(s.root)
		if err != nil {
			log.Printf("ryoku-shell: sight stale images: %v", err)
		}
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				log.Printf("ryoku-shell: sight stale image: %v", err)
				continue
			}
			if !info.Mode().IsRegular() {
				continue
			}
			if err := os.Remove(filepath.Join(s.root, entry.Name())); err != nil {
				log.Printf("ryoku-shell: sight stale image: %v", err)
			}
		}
	}
	s.mu.Lock()
	s.publishLocked()
	s.mu.Unlock()
	d.setCallLane("sight.ocr", "ocr")
	d.setCallLane("sight.ocrBoxes", "ocr")
	d.setCallLane("sight.replace", "input")
	d.setCallLane("sight.undo", "input")
	d.registerCall("sight.open", func(raw json.RawMessage) (any, error) {
		var card sightCard
		if err := decodeSightArgs(raw, &card); err != nil {
			return nil, err
		}
		card.Kind = "text"
		card.Image = ""
		card.Plate = false
		return s.open(card)
	})
	d.registerCall("sight.highlightOpen", func(raw json.RawMessage) (any, error) {
		var args struct {
			ID     string  `json:"id"`
			Mode   string  `json:"mode"`
			X      float64 `json:"x"`
			Y      float64 `json:"y"`
			Output string  `json:"output"`
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		return s.openHighlight(args.ID, args.Mode, args.X, args.Y, args.Output)
	})
	d.registerCall("sight.highlightDismiss", func(raw json.RawMessage) (any, error) {
		var args struct {
			ID string `json:"id"`
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		s.dismissHighlight(args.ID)
		return nil, nil
	})
	d.registerCall("sight.ocr", func(raw json.RawMessage) (any, error) {
		var args struct {
			ID string `json:"id"`
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		return s.ocr(args.ID)
	})
	d.registerCall("sight.detect", func(raw json.RawMessage) (any, error) {
		var args struct {
			Text string `json:"text"`
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		return map[string]string{"language": detectSightLanguage(args.Text)}, nil
	})
	d.registerCall("sight.languages", func(json.RawMessage) (any, error) {
		installed, err := sightInstalledLanguages()
		return map[string]any{"installed": installed}, err
	})
	d.registerCall("sight.close", func(raw json.RawMessage) (any, error) {
		var args struct {
			ID string `json:"id"`
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		return nil, s.close(args.ID, false)
	})
	d.registerCall("sight.closeAll", func(json.RawMessage) (any, error) { return nil, s.close("", true) })
	d.registerCall("sight.hide", func(raw json.RawMessage) (any, error) {
		var args struct {
			Hidden bool `json:"hidden"`
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.frame.Hidden = args.Hidden
		s.publishLocked()
		s.mu.Unlock()
		return nil, nil
	})
	d.registerCall("sight.trigger", func(json.RawMessage) (any, error) { return nil, s.trigger() })
	d.registerCall("sight.commit", func(raw json.RawMessage) (any, error) {
		var args struct {
			ID   string `json:"id"`
			Mode string `json:"mode"`
			sightBox
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		return s.commit(args.ID, args.sightBox, args.Mode)
	})
	d.registerCall("sight.cancel", func(raw json.RawMessage) (any, error) {
		var args struct {
			ID string `json:"id"`
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		if args.ID == "" {
			return nil, errors.New("bad-request")
		}
		s.cancelCapture(args.ID)
		return nil, nil
	})
	d.registerCall("sight.replace", func(raw json.RawMessage) (any, error) {
		var args struct {
			Text string `json:"text"`
			Mode string `json:"mode"`
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		mode := args.Mode
		if s.focusedTerminal() {
			mode = "copy"
		}
		if err := s.replace(args.Text, mode); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true, "copied": mode == "copy"}, nil
	})
	d.registerCall("sight.undo", func(json.RawMessage) (any, error) {
		if err := s.undoReplace(); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})
	d.registerCall("sight.speak", func(raw json.RawMessage) (any, error) {
		var args struct {
			ID    string  `json:"id"`
			Card  string  `json:"card"`
			Text  string  `json:"text"`
			Voice string  `json:"voice"`
			Rate  float64 `json:"rate"`
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		if err := s.speak(args.ID, args.Card, args.Text, args.Voice, args.Rate); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})
	d.registerCall("sight.stopSpeaking", func(json.RawMessage) (any, error) {
		s.stopSpeaking()
		return map[string]any{}, nil
	})
	d.registerCall("sight.voices", func(json.RawMessage) (any, error) { return s.voices(), nil })
	d.registerCall("sight.ocrBoxes", func(raw json.RawMessage) (any, error) {
		var args struct {
			ID string `json:"id"`
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		return s.ocrBoxes(args.ID)
	})
	d.registerCall("sight.quietNow", func(raw json.RawMessage) (any, error) {
		var args struct {
			App string `json:"app"`
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		return map[string]string{"app": s.quietNow(args.App)}, nil
	})
	d.registerCall("sight.inplaceDone", func(raw json.RawMessage) (any, error) {
		var args struct {
			ID string `json:"id"`
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		s.inplaceDone(args.ID)
		return map[string]any{}, nil
	})
	go s.watchConfiguration()
}

func decodeSightArgs(raw json.RawMessage, target any) error {
	if err := json.Unmarshal(raw, target); err != nil {
		log.Printf("ryoku-shell: sight arguments: %v", err)
		return errors.New("bad-request")
	}
	return nil
}
func (s *sightService) clearHighlight() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.highlightEpoch++
	if s.frame.Highlight != nil {
		s.frame.Highlight = nil
		s.publishLocked()
	}
}

func (s *sightService) dismissHighlight(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.frame.Highlight != nil && s.frame.Highlight.ID == id {
		s.highlightEpoch++
		s.frame.Highlight = nil
		s.publishLocked()
	}
}

func (s *sightService) openHighlight(id, mode string, x, y float64, output string) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.options.On {
		return nil, errors.New("theia-off")
	}
	highlight := s.frame.Highlight
	if highlight == nil || highlight.ID != id {
		return nil, errors.New("stale-highlight")
	}
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return nil, errors.New("bad-request")
	}
	s.highlightEpoch++
	s.frame.Highlight = nil
	return s.openLocked(sightCard{Kind: "text", Text: highlight.Text, Language: highlight.Language, App: s.focusedApp(), Source: "highlight", Mode: mode, X: x, Y: y, Output: output})
}

func (s *sightService) stopOutputs() {
	s.mu.Lock()
	s.cancelInputLocked()
	s.cancelOCRLocked("")
	s.mu.Unlock()
	s.stopSpeaking()
}

func (s *sightService) watchConfiguration() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes := make(chan struct{}, 1)
	configPath := filepath.Join(ryokuConfigDir(), "rashin.json")
	go watchDirectoryChanges(ctx, filepath.Dir(configPath), changes)
	s.watchConfigurationLoop(ctx, configPath, changes, s.watchGestures)
}

func (s *sightService) watchConfigurationLoop(ctx context.Context, configPath string, changes <-chan struct{}, runGestures func(context.Context, sightOptions, <-chan []byte)) {
	settingsTopic := s.daemon.topic("settings")
	settingsSub := settingsTopic.subscribe()
	defer settingsTopic.unsubscribe(settingsSub)
	presenceTopic := s.daemon.topic("presence")
	presenceSub := presenceTopic.subscribe()
	defer presenceTopic.unsubscribe(presenceSub)
	wmTopic := s.daemon.topic("wm")
	var wmSub *topicSubscriber
	var wmFrames <-chan []byte
	defer func() {
		if wmSub != nil {
			wmTopic.unsubscribe(wmSub)
		}
	}()
	var stop context.CancelFunc
	var done chan struct{}
	presence := make(chan []byte, 1)
	var previous sightOptions
	var wasOn bool
	var receivedSettings bool
	reconcile := func() {
		if !receivedSettings {
			return
		}
		raw, _ := os.ReadFile(configPath)
		var config struct {
			Enabled bool `json:"enabled"`
		}
		_ = json.Unmarshal(raw, &config)
		s.mu.Lock()
		options := s.options
		s.enabled = options.gestures()
		s.rashin = config.Enabled
		if !options.feature("inplace") || !config.Enabled {
			s.cancelInputLocked()
		}
		if !options.On || !config.Enabled {
			s.cancelOCRLocked("")
		}
		if !options.On {
			s.clearTheiaLocked()
		}
		s.mu.Unlock()
		on := config.Enabled && options.gestures()
		if wmSub == nil {
			wmTopic = s.daemon.topic("wm")
		}
		if options.feature("quiet") && wmSub == nil && wmTopic != nil {
			wmSub = wmTopic.subscribe()
			wmFrames = wmSub.frames
		} else if !options.feature("quiet") && wmSub != nil {
			wmTopic.unsubscribe(wmSub)
			wmSub, wmFrames = nil, nil
		}
		if !options.feature("readaloud") || !config.Enabled {
			s.stopSpeaking()
		}
		if stop != nil && (!on || sightGestureOptionsChanged(previous, options)) {
			stop()
			<-done
			stop = nil
		}
		if !options.On {
			s.cancelCapture("")
		}
		if on && stop == nil {
			workerCtx, workerCancel := context.WithCancel(ctx)
			stop = workerCancel
			done = make(chan struct{})
			go func() { defer close(done); runGestures(workerCtx, options, presence) }()
		}
		if !on || !wasOn {
			s.setWatching(false)
		}
		previous = options
		wasOn = on
	}
	defer func() {
		if stop != nil {
			stop()
			<-done
		}
		s.cancelCapture("")
		s.stopOutputs()
	}()
	for {
		select {
		case <-s.daemon.quit:
			return
		case frame := <-settingsSub.frames:
			s.mu.Lock()
			s.options = sightOptionsFromFrame(frame)
			s.mu.Unlock()
			receivedSettings = true
			reconcile()
		case <-changes:
			reconcile()
		case <-wmFrames:
			s.mu.Lock()
			if s.options.feature("quiet") {
				s.quietBlockedLocked(s.focusedApp(), time.Now())
			}
			s.mu.Unlock()
		case frame := <-presenceSub.frames:
			select {
			case presence <- frame:
			default:
				select {
				case <-presence:
				default:
				}
				presence <- frame
			}
			var state presenceFrame
			if json.Unmarshal(frame, &state) == nil && state.Locked {
				s.cancelCapture("")
				s.clearHighlight()
				s.inplaceDone("")
				s.stopOutputs()
			}
		}
	}
}
