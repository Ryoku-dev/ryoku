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
	Plate    bool    `json:"plate"`
	Created  int64   `json:"created"`
}
type sightHighlight struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Language string `json:"language"`
	Output   string `json:"output"`
	Created  int64  `json:"created"`
}
type sightFrame struct {
	Watching  bool            `json:"watching"`
	Reason    string          `json:"reason"`
	Hidden    bool            `json:"hidden"`
	Cards     []sightCard     `json:"cards"`
	Select    *sightSelection `json:"select"`
	Highlight *sightHighlight `json:"highlight"`
}
type sightOptions struct {
	Enabled      bool     `json:"enabled"`
	Highlight    bool     `json:"highlight"`
	TapKey       string   `json:"tapKey"`
	HighlightKey string   `json:"highlightKey"`
	Mode         string   `json:"mode"`
	OCRLanguages []string `json:"ocrLanguages"`
}

func sightOptionsFromFrame(data []byte) sightOptions {
	options := sightOptions{Enabled: true, Highlight: true, TapKey: "Alt_R", HighlightKey: "Alt_L", Mode: "explain", OCRLanguages: []string{"eng"}}
	var frame struct {
		Sight json.RawMessage `json:"sight"`
	}
	if json.Unmarshal(data, &frame) == nil && len(frame.Sight) != 0 {
		_ = json.Unmarshal(frame.Sight, &options)
	}
	options.TapKey = sightGestureKey(options.TapKey, "Alt_R").Name
	options.HighlightKey = sightGestureKey(options.HighlightKey, "Alt_L").Name
	return options
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
}

func (s *sightService) publishLocked() { body, _ := json.Marshal(s.frame); s.topic.publish(body) }
func (s *sightService) setWatching(watching bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frame.Watching = watching && s.enabled && s.rashin
	switch {
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
	return s.openLocked(card)
}
func (s *sightService) openLocked(card sightCard) (any, error) {
	switch card.Mode {
	case "explain", "text", "translate", "ask":
	default:
		card.Mode = s.options.Mode
	}
	switch card.Mode {
	case "explain", "text", "translate", "ask":
	default:
		card.Mode = "explain"
	}
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
	s := &sightService{daemon: d, topic: d.registerTopic("sight"), root: filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "ryoku", "sight"), frame: sightFrame{Reason: "rashin-off", Cards: []sightCard{}}}
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
			ID string `json:"id"`
			sightBox
		}
		if err := decodeSightArgs(raw, &args); err != nil {
			return nil, err
		}
		return s.commit(args.ID, args.sightBox)
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
	highlight := s.frame.Highlight
	if highlight == nil || highlight.ID != id {
		return nil, errors.New("stale-highlight")
	}
	if mode != "explain" && mode != "translate" && mode != "ask" || math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return nil, errors.New("bad-request")
	}
	s.highlightEpoch++
	s.frame.Highlight = nil
	return s.openLocked(sightCard{Kind: "text", Text: highlight.Text, Language: highlight.Language, Mode: mode, X: x, Y: y, Output: output})
}

func (s *sightService) watchConfiguration() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	settingsTopic := s.daemon.topic("settings")
	settingsSub := settingsTopic.subscribe()
	defer settingsTopic.unsubscribe(settingsSub)
	presenceTopic := s.daemon.topic("presence")
	presenceSub := presenceTopic.subscribe()
	defer presenceTopic.unsubscribe(presenceSub)
	changes := make(chan struct{}, 1)
	configPath := filepath.Join(ryokuConfigDir(), "rashin.json")
	go watchDirectoryChanges(ctx, filepath.Dir(configPath), changes)
	var stop context.CancelFunc
	var done chan struct{}
	presence := make(chan []byte, 1)
	var previous sightOptions
	var wasOn bool
	reconcile := func() {
		raw, _ := os.ReadFile(configPath)
		var config struct {
			Enabled bool `json:"enabled"`
		}
		_ = json.Unmarshal(raw, &config)
		s.mu.Lock()
		options := s.options
		s.enabled = options.Enabled || options.Highlight
		s.rashin = config.Enabled
		s.mu.Unlock()
		on := config.Enabled && (options.Enabled || options.Highlight)
		if stop != nil && (!on || previous.Enabled != options.Enabled || previous.Highlight != options.Highlight || previous.TapKey != options.TapKey || previous.HighlightKey != options.HighlightKey) {
			stop()
			<-done
			stop = nil
		}
		if on && stop == nil {
			workerCtx, workerCancel := context.WithCancel(ctx)
			stop = workerCancel
			done = make(chan struct{})
			go func() { defer close(done); s.watchGestures(workerCtx, options, presence) }()
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
	}()
	for {
		select {
		case <-s.daemon.quit:
			return
		case frame := <-settingsSub.frames:
			s.mu.Lock()
			s.options = sightOptionsFromFrame(frame)
			s.mu.Unlock()
			reconcile()
		case <-changes:
			reconcile()
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
			}
		}
	}
}
