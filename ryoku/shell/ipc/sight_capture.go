package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type sightBox struct {
	Output string  `json:"output"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type sightCaptureFrame struct {
	Output string  `json:"output"`
	Path   string  `json:"path"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Scale  float64 `json:"scale"`
}

type sightSelection struct {
	ID       string              `json:"id"`
	Frames   []sightCaptureFrame `json:"frames"`
	Previous *sightBox           `json:"previous"`
}

type sightCapture struct {
	id        string
	frames    []sightCaptureFrame
	preferred string
	cancel    context.CancelFunc
	done      chan struct{}
	err       error
}

func captureSightFrame(ctx context.Context, frame sightCaptureFrame) error {
	ctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	// grim excludes the pointer by default; hiding it would make quick taps blink.
	command := exec.CommandContext(ctx, "grim", "-o", frame.Output, "-l", "0", frame.Path)
	if err := command.Run(); err != nil {
		return err
	}
	return ctx.Err()
}

func (s *sightService) captureSnapshot() ([]sightCaptureFrame, string) {
	if s.snapshot != nil {
		return s.snapshot()
	}
	s.daemon.wmMu.Lock()
	defer s.daemon.wmMu.Unlock()
	frames := make([]sightCaptureFrame, 0, len(s.daemon.wmOutputs))
	preferred := s.daemon.activeMon
	for _, output := range s.daemon.wmOutputs {
		if output.Disabled || output.Name == "" || output.Width <= 0 || output.Height <= 0 {
			continue
		}
		scale := output.Scale
		if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
			scale = 1
		}
		frames = append(frames, sightCaptureFrame{Output: output.Name, X: float64(output.X), Y: float64(output.Y), Width: float64(output.Width), Height: float64(output.Height), Scale: scale})
		if preferred == "" && output.Focused {
			preferred = output.Name
		}
	}
	if preferred == "" && len(frames) > 0 {
		preferred = frames[0].Output
	}
	return frames, preferred
}

func (s *sightService) sessionLocked() bool {
	if s.locked != nil {
		return s.locked()
	}
	return isLocked()
}

func (s *sightService) beginCapture(ctx context.Context) string {
	s.captureMu.Lock()
	defer s.captureMu.Unlock()
	s.cancelCapture("")
	s.mu.Lock()
	on := s.options.On
	s.mu.Unlock()
	if !on {
		return ""
	}
	if s.sessionLocked() {
		return ""
	}
	frames, preferred := s.captureSnapshot()
	if len(frames) == 0 {
		log.Printf("ryoku-shell: sight capture: no output geometry")
		return ""
	}
	if !filepath.IsAbs(s.root) {
		log.Printf("ryoku-shell: sight capture: invalid runtime directory")
		return ""
	}
	if err := os.MkdirAll(s.root, 0700); err != nil {
		log.Printf("ryoku-shell: sight capture: %v", err)
		return ""
	}
	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	if !s.options.On || ctx.Err() != nil {
		s.mu.Unlock()
		cancel()
		return ""
	}
	s.next++
	id := fmt.Sprintf("%x-%x", time.Now().UnixNano(), s.next)
	for i := range frames {
		frames[i].Path = filepath.Join(s.root, "frame-"+id+"-"+url.PathEscape(frames[i].Output)+".png")
	}
	capture := &sightCapture{id: id, frames: frames, preferred: preferred, cancel: cancel, done: make(chan struct{})}
	s.capture = capture
	capturer := s.capturer
	if capturer == nil {
		capturer = captureSightFrame
	}
	s.mu.Unlock()
	go func() {
		defer close(capture.done)
		var workers sync.WaitGroup
		errorsFound := make(chan error, len(frames))
		for _, frame := range frames {
			workers.Add(1)
			go func(frame sightCaptureFrame) {
				defer workers.Done()
				if err := capturer(ctx, frame); err != nil {
					errorsFound <- err
				}
			}(frame)
		}
		workers.Wait()
		close(errorsFound)
		for err := range errorsFound {
			if capture.err == nil {
				capture.err = err
			}
		}
	}()
	return id
}

func removeSightFrames(capture *sightCapture) {
	for _, frame := range capture.frames {
		if err := os.Remove(frame.Path); err != nil && !os.IsNotExist(err) {
			log.Printf("ryoku-shell: sight frame cleanup: %v", err)
		}
	}
}

func (s *sightService) cancelCapture(id string) {
	s.mu.Lock()
	capture := s.capture
	if capture == nil || id != "" && capture.id != id {
		s.mu.Unlock()
		return
	}
	s.capture = nil
	if s.frame.Select != nil && s.frame.Select.ID == capture.id {
		s.frame.Select = nil
		s.publishLocked()
	}
	s.mu.Unlock()
	capture.cancel()
	<-capture.done
	removeSightFrames(capture)
}

func (s *sightService) fireCapture(ctx context.Context, id string) bool {
	s.mu.Lock()
	capture := s.capture
	s.mu.Unlock()
	if capture == nil || capture.id != id {
		return false
	}
	wait := time.NewTimer(1500 * time.Millisecond)
	defer wait.Stop()
	select {
	case <-ctx.Done():
		s.cancelCapture(id)
		return false
	case <-wait.C:
		log.Printf("ryoku-shell: sight capture: timed out")
		s.cancelCapture(id)
		return false
	case <-capture.done:
	}
	if capture.err != nil || s.sessionLocked() {
		if capture.err != nil {
			log.Printf("ryoku-shell: sight capture: %v", capture.err)
		}
		s.cancelCapture(id)
		return false
	}
	s.mu.Lock()
	if s.capture != capture {
		s.mu.Unlock()
		return false
	}
	if ctx.Err() != nil || !s.options.On || s.quietBlockedLocked(s.focusedApp(), time.Now()) {
		s.mu.Unlock()
		s.cancelCapture(id)
		return false
	}
	defer s.mu.Unlock()
	var previous *sightBox
	if box, found := s.previous[capture.preferred]; found {
		copy := box
		previous = &copy
	}
	s.frame.Select = &sightSelection{ID: id, Frames: capture.frames, Previous: previous}
	s.frame.Hidden = false
	s.publishLocked()
	return true
}

func sightCropRect(frame sightCaptureFrame, box sightBox, bounds image.Rectangle) (image.Rectangle, sightBox, error) {
	for _, value := range []float64{box.X, box.Y, box.Width, box.Height, frame.Scale} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return image.Rectangle{}, sightBox{}, errors.New("bad-request")
		}
	}
	if frame.Scale <= 0 {
		return image.Rectangle{}, sightBox{}, errors.New("bad-image")
	}
	left := math.Max(box.X, frame.X)
	top := math.Max(box.Y, frame.Y)
	right := math.Min(box.X+box.Width, frame.X+frame.Width)
	bottom := math.Min(box.Y+box.Height, frame.Y+frame.Height)
	if right-left < 8 || bottom-top < 8 {
		return image.Rectangle{}, sightBox{}, errors.New("too-small")
	}
	pixels := image.Rect(int(math.Floor((left-frame.X)*frame.Scale)), int(math.Floor((top-frame.Y)*frame.Scale)), int(math.Ceil((right-frame.X)*frame.Scale)), int(math.Ceil((bottom-frame.Y)*frame.Scale))).Intersect(bounds)
	if pixels.Empty() {
		return image.Rectangle{}, sightBox{}, errors.New("too-small")
	}
	return pixels, sightBox{Output: frame.Output, X: left, Y: top, Width: right - left, Height: bottom - top}, nil
}

func (s *sightService) commit(id string, box sightBox, modes ...string) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.options.On {
		return nil, errors.New("theia-off")
	}
	capture := s.capture
	if capture == nil || capture.id != id || s.frame.Select == nil || s.frame.Select.ID != id {
		return nil, errors.New("stale-selection")
	}
	var frame *sightCaptureFrame
	for i := range capture.frames {
		if capture.frames[i].Output == box.Output {
			frame = &capture.frames[i]
			break
		}
	}
	if frame == nil {
		return nil, errors.New("bad-request")
	}
	file, err := os.Open(frame.Path)
	if err != nil {
		return nil, errors.New("bad-image")
	}
	plate, err := png.Decode(file)
	file.Close()
	if err != nil {
		return nil, errors.New("bad-image")
	}
	pixels, box, err := sightCropRect(*frame, box, plate.Bounds())
	if err != nil {
		return nil, err
	}
	cropped := image.NewNRGBA(image.Rect(0, 0, pixels.Dx(), pixels.Dy()))
	draw.Draw(cropped, cropped.Bounds(), plate, pixels.Min, draw.Src)
	s.next++
	cardID := fmt.Sprintf("s%x-%x", time.Now().UnixNano(), s.next)
	path := filepath.Join(s.root, "card-"+cardID+".png")
	file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, errors.New("bad-image")
	}
	encodeErr := png.Encode(file, cropped)
	closeErr := file.Close()
	if encodeErr != nil || closeErr != nil {
		os.Remove(path)
		return nil, errors.New("bad-image")
	}
	mode := ""
	if len(modes) > 0 {
		mode = modes[0]
	}
	mode = sightMode(mode, s.options.Mode)
	card := sightCard{ID: cardID, Kind: "image", Image: path, X: box.X, Y: box.Y, Width: box.Width, Height: box.Height, Output: box.Output, Mode: mode, App: s.focusedApp(), Source: "look", Plate: true, Created: time.Now().UnixMilli()}
	if s.previous == nil {
		s.previous = make(map[string]sightBox)
	}
	s.previous[box.Output] = box
	s.frame.Cards = append(s.frame.Cards, card)
	s.frame.Select = nil
	s.frame.Hidden = false
	s.capture = nil
	capture.cancel()
	removeSightFrames(capture)
	s.publishLocked()
	return map[string]any{"id": cardID}, nil
}
