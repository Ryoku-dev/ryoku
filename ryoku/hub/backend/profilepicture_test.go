package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pictureMachine is a profile and badge store of the test's own; holder activates a
// badge the test owner key signed for this machine.
func pictureMachine(t *testing.T, holder bool) {
	t.Helper()
	owner, _, _ := badgeTestKeys(t)
	setCardMachine(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if !holder {
		return
	}
	device, err := badgeDeviceSeed()
	if err != nil {
		t.Fatal(err)
	}
	if err := badgeActivate(badgeTestCode(t, 1, 3, owner, device, "neur0map")); err != nil {
		t.Fatal(err)
	}
	if !badgeHolder() {
		t.Fatal("activated badge is not held")
	}
}

func testGIF(t *testing.T, frames int) []byte {
	t.Helper()
	palette := color.Palette{color.Black, color.White, color.RGBA{0xff, 0x80, 0, 0xff}}
	anim := &gif.GIF{}
	for i := range frames {
		frame := image.NewPaletted(image.Rect(0, 0, 32, 24), palette)
		for p := range frame.Pix {
			frame.Pix[p] = uint8((p + i) % len(palette))
		}
		anim.Image = append(anim.Image, frame)
		anim.Delay = append(anim.Delay, 10)
	}
	var out bytes.Buffer
	if err := gif.EncodeAll(&out, anim); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// padGIF grows a GIF past size with comment blocks, which every reader skips.
func padGIF(raw []byte, size int) []byte {
	body := append([]byte{}, raw[:len(raw)-1]...)
	for len(body) < size {
		body = append(body, 0x21, 0xfe)
		for i := 0; i < 64 && len(body) < size; i++ {
			body = append(body, 0xff)
			body = append(body, bytes.Repeat([]byte{'x'}, 255)...)
		}
		body = append(body, 0)
	}
	return append(body, 0x3b)
}

func writeChosen(t *testing.T, name string, raw []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func testVideo(t *testing.T, name string, seconds string) []byte {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe is not installed")
	}
	path := filepath.Join(t.TempDir(), name)
	out, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-y", "-f", "lavfi", "-i",
		"testsrc=size=64x48:rate=4:duration="+seconds, "-pix_fmt", "yuv420p", path).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestGIFFramesCountsWithoutDecoding(t *testing.T) {
	for frames := 1; frames <= 3; frames++ {
		if n, err := gifFrames(testGIF(t, frames)); err != nil || n != frames {
			t.Fatalf("frames = %d, %v; want %d", n, err, frames)
		}
	}
	if n, err := gifFrames(padGIF(testGIF(t, 2), 4096)); err != nil || n != 2 {
		t.Fatal("comment blocks were counted", n, err)
	}
	if _, err := gifFrames([]byte("GIF89a\x01\x00\x01\x00\x00\x00\x00\x99")); err == nil {
		t.Fatal("damaged GIF accepted")
	}
}

func TestSniffPictureReadsContentNotName(t *testing.T) {
	for _, tc := range []struct {
		head          []byte
		ext           string
		video, moving bool
	}{
		{sampleHero, "png", false, false},
		{testGIF(t, 1), "gif", false, false},
		{testGIF(t, 2), "gif", false, true},
		{[]byte("\x00\x00\x00\x18ftypisom\x00\x00\x02\x00isomiso2"), "mp4", true, true},
		{[]byte("\x00\x00\x00\x18ftypavif\x00\x00\x00\x00avifmif1"), "avif", false, false},
		{[]byte("\x1a\x45\xdf\xa3\x9f\x42\x86\x81\x01\x42\x82\x84webm"), "webm", true, true},
		{[]byte("\x1a\x45\xdf\xa3\xa3\x42\x86\x81\x01\x42\x82\x88matroska"), "mkv", true, true},
		{append([]byte("RIFF\x00\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x02\x00\x00\x00"), 9, 0, 0, 9, 0, 0), "webp", false, true},
	} {
		kind, err := sniffPicture(tc.head)
		if err != nil || kind.ext != tc.ext || kind.video != tc.video || kind.moving != tc.moving {
			t.Fatalf("%s: got %+v, %v", tc.ext, kind, err)
		}
	}
	for _, refused := range [][]byte{[]byte("<svg/>"), []byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00"), []byte("\x1a\x45\xdf\xa3 nothing")} {
		if _, err := sniffPicture(refused); err == nil {
			t.Fatalf("%q accepted", refused)
		}
	}
}

func TestMovingPictureNeedsBadge(t *testing.T) {
	pictureMachine(t, false)
	video := testVideo(t, "clip.mp4", "2")
	if _, err := savePicture(writeChosen(t, "clip.mp4", video)); err == nil || err != errMovingNeedsBadge {
		t.Fatal("a video was kept without a badge:", err)
	}
	if _, err := os.Stat(filepath.Join(profileHeroDir(), "hero.mp4")); !os.IsNotExist(err) {
		t.Fatal("a refused video left a file", err)
	}
	// a moving GIF is kept as its first frame, as a still
	saved, err := savePicture(writeChosen(t, "loop.gif", testGIF(t, 3)))
	if err != nil || saved.Source != "hero.png" || saved.Moving || !saved.Still {
		t.Fatal(saved, err)
	}
	raw, err := os.ReadFile(filepath.Join(profileHeroDir(), "hero.png"))
	if err != nil {
		t.Fatal(err)
	}
	if kind, _ := sniffPicture(raw); kind.ext != "png" {
		t.Fatal("first frame not kept as PNG", kind)
	}
	// stills are anyone's
	if saved, err = savePicture(writeChosen(t, "me.png", sampleHero)); err != nil || saved.Source != "hero.png" || saved.Moving {
		t.Fatal(saved, err)
	}
}

func TestMovingPictureSizeCap(t *testing.T) {
	pictureMachine(t, true)
	big := padGIF(testGIF(t, 2), maxMovingBytes+(1<<20))
	_, err := savePicture(writeChosen(t, "big.gif", big))
	if err == nil || !strings.Contains(err.Error(), sizeText(int64(len(big)))) || !strings.Contains(err.Error(), "10.0 MB") {
		t.Fatal("an oversized moving picture was not refused by its size:", err)
	}
	// a video's size is refused before any tool reads it
	mp4 := append([]byte("\x00\x00\x00\x18ftypisom\x00\x00\x02\x00isomiso2"), make([]byte, maxMovingBytes)...)
	if _, err = savePicture(writeChosen(t, "big.mp4", mp4)); err == nil || !strings.Contains(err.Error(), "10.0 MB") {
		t.Fatal("an oversized video was not refused by its size:", err)
	}
	// past even a still's limit, the size is named before reading it
	huge := writeChosen(t, "huge.png", nil)
	if err := os.Truncate(huge, maxPictureBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err = savePicture(huge); err == nil || !strings.Contains(err.Error(), "16.0 MB") {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(profileHeroDir()); len(entries) != 0 {
		t.Fatal("refused pictures left files", entries)
	}
	// within the cap a moving GIF is kept moving
	saved, err := savePicture(writeChosen(t, "loop.gif", padGIF(testGIF(t, 2), maxMovingBytes-4096)))
	if err != nil || saved.Source != "hero.gif" || !saved.Moving {
		t.Fatal(saved, err)
	}
}

func TestMovingPictureContentValidation(t *testing.T) {
	pictureMachine(t, true)
	if _, err := savePicture(writeChosen(t, "clip.mp4", sampleHero)); err == nil || !strings.Contains(err.Error(), "named as a video") {
		t.Fatal("a PNG renamed .mp4 was kept:", err)
	}
	video := testVideo(t, "clip.mp4", "2")
	if _, err := savePicture(writeChosen(t, "clip.png", video)); err == nil {
		t.Fatal("a video renamed .png was kept")
	}
	// a container header with nothing playable behind it
	fake := append([]byte("\x00\x00\x00\x18ftypisom\x00\x00\x02\x00isomiso2"), bytes.Repeat([]byte{7}, 4096)...)
	if _, err := savePicture(writeChosen(t, "fake.mp4", fake)); err == nil {
		t.Fatal("an unplayable mp4 was kept")
	}
	long := testVideo(t, "long.webm", "61")
	if _, err := savePicture(writeChosen(t, "long.webm", long)); err == nil || !strings.Contains(err.Error(), "60 seconds") {
		t.Fatal("an over-long video was kept:", err)
	}
	if entries, _ := os.ReadDir(profileHeroDir()); len(entries) != 0 {
		t.Fatal("refused videos left files", entries)
	}

	saved, err := savePicture(writeChosen(t, "clip.mp4", video))
	if err != nil || saved.Source != "hero.mp4" || !saved.Moving {
		t.Fatal(saved, err)
	}
	kept, _ := os.ReadFile(filepath.Join(profileHeroDir(), "hero.mp4"))
	if !bytes.Equal(kept, video) {
		t.Fatal("the kept video differs from the chosen one")
	}
	poster, err := readPictureFile(filepath.Join(profileHeroDir(), "hero-mp4.png"))
	if err != nil {
		t.Fatal(err)
	}
	if img, err := decodePicture(poster); err != nil || img.Bounds().Dx() != 64 {
		t.Fatal("poster", err)
	}
}

// A moving picture's design travels still: the .ryoprofile carries the poster as hero.png.
func TestMovingPictureExportsStill(t *testing.T) {
	pictureMachine(t, true)
	saved, err := savePicture(writeChosen(t, "clip.webm", testVideo(t, "clip.webm", "2")))
	if err != nil || saved.Source != "hero.webm" {
		t.Fatal(saved, err)
	}
	if err := atomicWrite(profileConfigPath(), []byte(`{"hero":{"kind":"custom","source":"hero.webm","style":"dither"},"text":{"name":"Nero"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "card.ryoprofile")
	if err := exportProfile(dst); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(dst)
	var env ryoEnvelope
	if err := json.Unmarshal(raw, &env); err != nil || env.Hero == nil || env.Hero.Name != "hero.png" {
		t.Fatal("export", env.Hero, err)
	}
	var model profileModel
	if err := json.Unmarshal(env.Profile, &model); err != nil || model.Hero.Source != "hero.png" {
		t.Fatal("exported source", string(env.Profile))
	}
	still, _ := base64.StdEncoding.DecodeString(env.Hero.B64)
	if kind, _ := sniffPicture(still); kind.ext != "png" {
		t.Fatal("exported hero is not a PNG still")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := importProfile(dst); err != nil {
		t.Fatal("import of a moving picture's export:", err)
	}

	// a moving GIF also travels as its first frame
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if saved, err = savePicture(writeChosen(t, "loop.gif", testGIF(t, 3))); err != nil || saved.Source != "hero.gif" {
		t.Fatal(saved, err)
	}
	if err := atomicWrite(profileConfigPath(), []byte(`{"hero":{"kind":"custom","source":"hero.gif"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exportProfile(dst); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(dst)
	env = ryoEnvelope{}
	if err := json.Unmarshal(raw, &env); err != nil || env.Hero == nil || env.Hero.Name != "hero.png" {
		t.Fatal("gif export", err)
	}
	still, _ = base64.StdEncoding.DecodeString(env.Hero.B64)
	if kind, _ := sniffPicture(still); kind.ext != "png" {
		t.Fatal("exported GIF is not a PNG still")
	}
}

func TestLiveWallpaperOffersOnlyVideo(t *testing.T) {
	pictureMachine(t, true)
	state := filepath.Join(os.Getenv("XDG_STATE_HOME"), "ryoku-wallpaper")
	still := writeChosen(t, "wall.png", sampleHero)
	if err := os.WriteFile(state, []byte(still+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := liveWallpaper(); got["video"] != false {
		t.Fatal("a still wallpaper offered", got)
	}
	clip := writeChosen(t, "wall.mp4", append([]byte("\x00\x00\x00\x18ftypisom\x00\x00\x02\x00isomiso2"), make([]byte, 2048)...))
	if err := os.WriteFile(state, []byte(clip+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := liveWallpaper()
	if got["video"] != true || got["path"] != clip || got["bytes"] != int64(2072) || got["limit"] != maxMovingBytes {
		t.Fatal(got)
	}
}
