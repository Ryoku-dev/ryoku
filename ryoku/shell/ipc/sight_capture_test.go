package main

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func testSightCaptureService(t *testing.T, frames []sightCaptureFrame, preferred *string) *sightService {
	t.Helper()
	s := &sightService{root: t.TempDir(), topic: newStateTopic(), daemon: &daemon{quit: make(chan struct{})}, options: sightOptions{Mode: "translate"}, frame: sightFrame{Cards: []sightCard{}}, locked: func() bool { return false }}
	s.snapshot = func() ([]sightCaptureFrame, string) {
		return append([]sightCaptureFrame(nil), frames...), *preferred
	}
	s.capturer = func(ctx context.Context, frame sightCaptureFrame) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		pixels := image.NewNRGBA(image.Rect(0, 0, int(frame.Width*frame.Scale), int(frame.Height*frame.Scale)))
		for y := range pixels.Bounds().Dy() {
			for x := range pixels.Bounds().Dx() {
				pixels.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), A: 255})
			}
		}
		file, err := os.OpenFile(frame.Path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		err = png.Encode(file, pixels)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	t.Cleanup(func() { s.cancelCapture("") })
	return s
}

func TestSightTripleTapCaptureFireCommit(t *testing.T) {
	preferred := "DP-1"
	s := testSightCaptureService(t, []sightCaptureFrame{{Output: "DP-1", X: 100, Y: 50, Width: 100, Height: 80, Scale: 1}}, &preferred)
	taps := newSightTap(100)
	now := time.Unix(100, 0)
	taps.event(1, 100, 1, now)
	taps.event(1, 100, 0, now.Add(50*time.Millisecond))
	taps.event(1, 100, 1, now.Add(100*time.Millisecond))
	if action := taps.event(1, 100, 0, now.Add(150*time.Millisecond)); action != "prewarm" {
		t.Fatalf("second tap action = %q", action)
	}
	id := s.beginCapture(context.Background())
	if id == "" || s.frame.Select != nil {
		t.Fatal("capture was published before the third tap")
	}
	taps.event(1, 100, 1, now.Add(200*time.Millisecond))
	if action := taps.event(1, 100, 0, now.Add(250*time.Millisecond)); action != "fire" {
		t.Fatalf("third tap action = %q", action)
	}
	if !s.fireCapture(context.Background(), id) {
		t.Fatal("completed capture did not fire")
	}
	if s.frame.Select == nil || s.frame.Select.ID != id || s.frame.Select.Previous != nil {
		t.Fatalf("selection = %#v", s.frame.Select)
	}
	framePath := s.frame.Select.Frames[0].Path
	result, err := s.commit(id, sightBox{Output: "DP-1", X: 110, Y: 60, Width: 20, Height: 15})
	if err != nil {
		t.Fatal(err)
	}
	card := s.frame.Cards[0]
	if result.(map[string]any)["id"] != card.ID || !card.Plate || card.Mode != "translate" || card.Kind != "image" {
		t.Fatalf("card = %#v, result = %#v", card, result)
	}
	wire, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Language *string `json:"language"`
	}
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Language == nil || *decoded.Language != "" {
		t.Fatalf("image card language must be unknown before OCR: %s", wire)
	}
	if s.frame.Select != nil || s.capture != nil {
		t.Fatal("commit retained selection state")
	}
	if _, err := os.Stat(framePath); !os.IsNotExist(err) {
		t.Fatalf("commit retained frame: %v", err)
	}
	file, err := os.Open(card.Image)
	if err != nil {
		t.Fatal(err)
	}
	pixels, err := png.Decode(file)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if pixels.Bounds() != image.Rect(0, 0, 20, 15) || color.NRGBAModel.Convert(pixels.At(0, 0)).(color.NRGBA) != (color.NRGBA{R: 10, G: 10, A: 255}) {
		t.Fatalf("cropped image = %v, origin pixel = %v", pixels.Bounds(), pixels.At(0, 0))
	}
}

func TestSightLateThirdTapCancelsCaptureWithoutPublishing(t *testing.T) {
	preferred := "DP-1"
	s := testSightCaptureService(t, []sightCaptureFrame{{Output: "DP-1", Width: 100, Height: 80, Scale: 1}}, &preferred)
	started := make(chan string, 1)
	cancelled := make(chan struct{})
	s.capturer = func(ctx context.Context, frame sightCaptureFrame) error {
		if err := os.WriteFile(frame.Path, []byte("partial capture"), 0600); err != nil {
			return err
		}
		started <- frame.Path
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	}
	taps := newSightTap(100)
	now := time.Unix(100, 0)
	taps.event(1, 100, 1, now)
	taps.event(1, 100, 0, now.Add(50*time.Millisecond))
	taps.event(1, 100, 1, now.Add(100*time.Millisecond))
	if action := taps.event(1, 100, 0, now.Add(150*time.Millisecond)); action != "prewarm" {
		t.Fatalf("second tap action = %q", action)
	}
	id := s.beginCapture(context.Background())
	var path string
	select {
	case path = <-started:
	case <-time.After(time.Second):
		t.Fatal("capture did not start")
	}
	if action := taps.tick(now.Add(501 * time.Millisecond)); action != "cancel" {
		t.Fatalf("tap action = %q", action)
	}
	s.cancelCapture(id)
	select {
	case <-cancelled:
	default:
		t.Fatal("tap did not cancel the capturer")
	}
	if s.frame.Select != nil || s.capture != nil {
		t.Fatal("tap published or retained a selection")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("tap retained frame: %v", err)
	}
	if len(s.topic.last) != 0 {
		t.Fatalf("tap published a frame: %s", s.topic.last)
	}
	if action := taps.tick(now.Add(time.Second)); action != "" {
		t.Fatalf("tap later fired: %q", action)
	}
}

func TestSightCapturesOutputsInParallelAndCancelsVisibleSelection(t *testing.T) {
	preferred := "DP-2"
	frames := []sightCaptureFrame{{Output: "DP-1", Width: 100, Height: 80, Scale: 1}, {Output: "DP-2", X: 100, Width: 100, Height: 80, Scale: 1}}
	s := testSightCaptureService(t, frames, &preferred)
	capture := s.capturer
	started := make(chan struct{}, len(frames))
	release := make(chan struct{})
	s.capturer = func(ctx context.Context, frame sightCaptureFrame) error {
		started <- struct{}{}
		select {
		case <-release:
			return capture(ctx, frame)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	id := s.beginCapture(context.Background())
	for range frames {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("outputs were not captured concurrently")
		}
	}
	close(release)
	if !s.fireCapture(context.Background(), id) {
		t.Fatal("parallel capture did not fire")
	}
	paths := append([]sightCaptureFrame(nil), s.frame.Select.Frames...)
	s.cancelCapture("stale-id")
	if s.frame.Select == nil {
		t.Fatal("a stale cancel removed the current selection")
	}
	s.cancelCapture(id)
	if s.frame.Select != nil || len(s.frame.Cards) != 0 {
		t.Fatal("cancel retained selection or created a card")
	}
	for _, frame := range paths {
		if _, err := os.Stat(frame.Path); !os.IsNotExist(err) {
			t.Fatalf("cancel retained frame: %v", err)
		}
	}
}

func TestSightCaptureFailureAndLockStayHidden(t *testing.T) {
	for _, locked := range []bool{false, true} {
		preferred := "DP-1"
		s := testSightCaptureService(t, []sightCaptureFrame{{Output: "DP-1", Width: 100, Height: 80, Scale: 1}}, &preferred)
		if !locked {
			s.capturer = func(context.Context, sightCaptureFrame) error { return errors.New("capture failed") }
		}
		id := s.beginCapture(context.Background())
		s.locked = func() bool { return locked }
		if s.fireCapture(context.Background(), id) || s.frame.Select != nil {
			t.Fatal("failed or locked capture became visible")
		}
		entries, err := os.ReadDir(s.root)
		if err != nil || len(entries) != 0 {
			t.Fatalf("failed capture files = %v, %v", entries, err)
		}
	}
}

func TestSightCropMath(t *testing.T) {
	for _, test := range []struct {
		name   string
		scale  float64
		box    sightBox
		bounds image.Rectangle
		want   image.Rectangle
	}{
		{"scale one", 1, sightBox{X: 110, Y: 70, Width: 20, Height: 15}, image.Rect(0, 0, 100, 80), image.Rect(10, 20, 30, 35)},
		{"fractional outward", 1.333, sightBox{X: 110.25, Y: 70.25, Width: 20, Height: 15}, image.Rect(0, 0, 134, 107), image.Rect(13, 26, 41, 47)},
		{"clamped", 1, sightBox{X: 90, Y: 40, Width: 30, Height: 30}, image.Rect(0, 0, 100, 80), image.Rect(0, 0, 20, 20)},
	} {
		t.Run(test.name, func(t *testing.T) {
			frame := sightCaptureFrame{Output: "DP-1", X: 100, Y: 50, Width: 100, Height: 80, Scale: test.scale}
			got, _, err := sightCropRect(frame, test.box, test.bounds)
			if err != nil || got != test.want {
				t.Fatalf("crop = %v, %v; want %v", got, err, test.want)
			}
		})
	}
}

func TestSightCommitTooSmallRetainsSelection(t *testing.T) {
	preferred := "DP-1"
	s := testSightCaptureService(t, []sightCaptureFrame{{Output: "DP-1", Width: 100, Height: 80, Scale: 1}}, &preferred)
	id := s.beginCapture(context.Background())
	if !s.fireCapture(context.Background(), id) {
		t.Fatal("capture did not fire")
	}
	for _, box := range []sightBox{{Output: "DP-1", Width: 7.99, Height: 20}, {Output: "DP-1", Width: 20, Height: 7}, {Output: "DP-1", X: 95, Width: 20, Height: 20}} {
		if _, err := s.commit(id, box); err == nil || err.Error() != "too-small" {
			t.Fatalf("small commit = %v", err)
		}
	}
	if s.frame.Select == nil || len(s.frame.Cards) != 0 {
		t.Fatal("too-small changed the selection")
	}
}

func TestSightPreviousBoxPerOutput(t *testing.T) {
	preferred := "DP-1"
	frames := []sightCaptureFrame{{Output: "DP-1", Width: 100, Height: 80, Scale: 1}, {Output: "DP-2", X: 100, Width: 100, Height: 80, Scale: 1}}
	s := testSightCaptureService(t, frames, &preferred)
	boxes := []sightBox{{Output: "DP-1", X: 10, Y: 20, Width: 20, Height: 20}, {Output: "DP-2", X: 120, Y: 30, Width: 30, Height: 20}}
	for _, box := range boxes {
		preferred = box.Output
		id := s.beginCapture(context.Background())
		if !s.fireCapture(context.Background(), id) {
			t.Fatal("capture did not fire")
		}
		if s.frame.Select.Previous != nil {
			t.Fatal("previous leaked from another output")
		}
		if _, err := s.commit(id, box); err != nil {
			t.Fatal(err)
		}
	}
	for _, box := range boxes {
		preferred = box.Output
		id := s.beginCapture(context.Background())
		if !s.fireCapture(context.Background(), id) {
			t.Fatal("capture did not fire")
		}
		if !reflect.DeepEqual(s.frame.Select.Previous, &box) {
			t.Fatalf("previous = %#v, want %#v", s.frame.Select.Previous, box)
		}
		s.cancelCapture(id)
	}
	entries, err := filepath.Glob(filepath.Join(s.root, "frame-*"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("frames retained: %v, %v", entries, err)
	}
}

func TestSightTriggerCapturesAndFiresImmediately(t *testing.T) {
	preferred := "DP-1"
	s := testSightCaptureService(t, []sightCaptureFrame{{Output: "DP-1", Width: 100, Height: 80, Scale: 1}}, &preferred)
	if err := s.trigger(); err != nil {
		t.Fatal(err)
	}
	if s.frame.Select == nil || len(s.frame.Select.Frames) != 1 || len(s.frame.Cards) != 0 {
		t.Fatalf("trigger state = %#v", s.frame)
	}
}

func TestSightCancelledFireCleansCompletedFrames(t *testing.T) {
	preferred := "DP-1"
	s := testSightCaptureService(t, []sightCaptureFrame{{Output: "DP-1", Width: 100, Height: 80, Scale: 1}}, &preferred)
	id := s.beginCapture(context.Background())
	<-s.capture.done
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if s.fireCapture(ctx, id) || s.frame.Select != nil || s.capture != nil {
		t.Fatal("cancelled fire retained or published a selection")
	}
	entries, err := os.ReadDir(s.root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cancelled fire files = %v, %v", entries, err)
	}
}
