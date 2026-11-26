package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Real compressed pixels, not header-only WebP fixtures. The structural parser
// tests retain their malformed/header-only constructors separately.
func realTestWebP(t *testing.T, w, h, count int) []byte {
	t.Helper()
	tool, err := exec.LookPath("cwebp")
	if err != nil {
		t.Fatal("media fixtures require libwebp cwebp:", err)
	}
	dir := t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			if x >= w/3 {
				img.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), 120, 200})
			}
		}
	}
	pngRaw, err := pngBytes(img)
	if err != nil {
		t.Fatal(err)
	}
	input, output := filepath.Join(dir, "input.png"), filepath.Join(dir, "output.webp")
	if err = os.WriteFile(input, pngRaw, 0600); err != nil {
		t.Fatal(err)
	}
	if raw, e := exec.Command(tool, "-quiet", "-lossless", "-exact", input, "-o", output).CombinedOutput(); e != nil {
		t.Fatal(e, string(raw))
	}
	still, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if count == 1 {
		return still
	}
	chunks, err := webpChunks(still[12:])
	if err != nil {
		t.Fatal(err)
	}
	var pixels []byte
	for _, c := range chunks {
		if c.kind == "VP8L" || c.kind == "VP8 " || c.kind == "ALPH" {
			pixels = append(pixels, webpChunkBytes(c.kind, c.body)...)
		}
	}
	result := webpExtended(w, h, 0x12)
	result = append(result, webpChunkBytes("ANIM", []byte{0, 0, 0, 0, 3, 0})...)
	for i := range count {
		header := make([]byte, 16)
		putWebp24(header[6:9], w-1)
		putWebp24(header[9:12], h-1)
		putWebp24(header[12:15], (i+1)*70)
		header[15] = byte(i%2) | 2
		result = append(result, webpChunkBytes("ANMF", append(header, pixels...))...)
	}
	return webpContainer(result)
}

func TestMediaGIFPreservesFramesTimingAlpha(t *testing.T) {
	palette := color.Palette{color.NRGBA{}, color.NRGBA{R: 255, A: 255}, color.NRGBA{G: 255, A: 255}}
	original := &gif.GIF{LoopCount: 3, Config: image.Config{ColorModel: palette, Width: 12, Height: 10}, BackgroundIndex: 0}
	for i := range 3 {
		frame := image.NewPaletted(image.Rect(2, 2, 10, 8), palette)
		for j := range frame.Pix {
			frame.Pix[j] = uint8((j + i) % 3)
		}
		original.Image = append(original.Image, frame)
		original.Delay = append(original.Delay, []int{3, 11, 7}[i])
		original.Disposal = append(original.Disposal, []byte{gif.DisposalNone, gif.DisposalBackground, gif.DisposalPrevious}[i])
	}
	var raw bytes.Buffer
	if err := gif.EncodeAll(&raw, original); err != nil {
		t.Fatal(err)
	}
	result, err := inspectMedia(context.Background(), raw.Bytes(), false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := gif.DecodeAll(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Image) != 3 || !reflect.DeepEqual(got.Delay, original.Delay) || !reflect.DeepEqual(got.Disposal, original.Disposal) || got.LoopCount != 3 || !reflect.DeepEqual(result.Durations, []int{30, 110, 70}) {
		t.Fatalf("frame/timing/disposal loss: %+v", result)
	}
	for i, frame := range got.Image {
		if frame.Bounds() != original.Image[i].Bounds() {
			t.Fatal("frame rectangle changed")
		}
		for y := frame.Rect.Min.Y; y < frame.Rect.Max.Y; y++ {
			for x := frame.Rect.Min.X; x < frame.Rect.Max.X; x++ {
				if color.NRGBAModel.Convert(frame.At(x, y)) != color.NRGBAModel.Convert(original.Image[i].At(x, y)) {
					t.Fatal("frame pixels/alpha changed", i, x, y)
				}
			}
		}
	}
}

func TestMediaWebPPreservesFramesTimingAlpha(t *testing.T) {
	raw := realTestWebP(t, 24, 18, 3)
	result, err := inspectMedia(context.Background(), raw, false)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := webpChunks(raw[12:])
	after, err := webpChunks(result.Data[12:])
	if err != nil {
		t.Fatal(err)
	}
	var controlsBefore, controlsAfter [][]byte
	for _, c := range before {
		if c.kind == "ANIM" {
			controlsBefore = append(controlsBefore, c.body)
		}
		if c.kind == "ANMF" {
			controlsBefore = append(controlsBefore, c.body[:16])
		}
	}
	for _, c := range after {
		if c.kind == "ANIM" {
			controlsAfter = append(controlsAfter, c.body)
		}
		if c.kind == "ANMF" {
			controlsAfter = append(controlsAfter, c.body[:16])
		}
	}
	if !reflect.DeepEqual(controlsBefore, controlsAfter) || result.Frames != 3 || !reflect.DeepEqual(result.Durations, []int{70, 140, 210}) {
		t.Fatal("WebP frame/control data changed", result.Durations)
	}
	// Exact lossless reencoding must preserve decoded pixels, not merely headers.
	again, err := inspectMedia(context.Background(), result.Data, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result.Poster, again.Poster) {
		t.Fatal("WebP poster pixels/alpha changed on canonical roundtrip")
	}
	poster, err := decodePictureLocal(result.Poster)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, a := poster.At(0, 0).RGBA(); a != 0 {
		t.Fatal("transparent pixels flattened")
	}
	if _, _, _, a := poster.At(20, 10).RGBA(); a == 0 || a == 65535 {
		t.Fatal("partial alpha was lost")
	}
}

func TestMediaRejectsMalformedBudgets(t *testing.T) {
	raw := testGIF(t, 2)
	oversized := append([]byte{}, raw...)
	binary.LittleEndian.PutUint16(oversized[6:8], 65535)
	longGIF := append([]byte{}, raw...)
	layout, e := inspectGIF(longGIF)
	if e != nil {
		t.Fatal(e)
	}
	binary.LittleEndian.PutUint16(layout.frames[0].control[4:6], 6001)
	compositedGIF := testGIF(t, 24)
	binary.LittleEndian.PutUint16(compositedGIF[6:8], 2048)
	binary.LittleEndian.PutUint16(compositedGIF[8:10], 2048)
	longWebP := designTestAnimated(16, 16, 2, 0)
	chunks, _ := webpChunks(longWebP[12:])
	for _, c := range chunks {
		if c.kind == "ANMF" {
			putWebp24(c.body[12:15], 60001)
		}
	}
	tests := map[string][]byte{"gif-truncated": raw[:len(raw)-1], "gif-canvas": oversized, "gif-frames": testGIF(t, 97), "webp-frames": designTestAnimated(1, 1, 97, 0), "webp-composited": designTestAnimated(2048, 2048, 24, 0), "webp-truncated": []byte("RIFF\x10\x00\x00\x00WEBPVP8X"), "png-size": bombPNG(100000, 100000)}
	tests["gif-duration"] = longGIF
	tests["gif-composited"] = compositedGIF
	tests["webp-duration"] = longWebP
	for name, fixture := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := inspectMedia(context.Background(), fixture, false); err == nil {
				t.Fatal("accepted malformed/budget fixture")
			}
		})
	}
	// Structurally plausible compressed data must still be decoded, not passed to Qt.
	if _, err := inspectMedia(context.Background(), designTestRIFF(designTestVP8(256, 256)), false); err == nil {
		t.Fatal("header-only fake pixels accepted")
	}
}

func TestMediaArchiveSafetyBudgets(t *testing.T) {
	var symlink bytes.Buffer
	writer := zip.NewWriter(&symlink)
	h := &zip.FileHeader{Name: "art.webp", Method: zip.Store}
	h.SetMode(os.ModeSymlink | 0777)
	f, _ := writer.CreateHeader(h)
	f.Write([]byte("/outside"))
	writer.Close()
	names := make([]string, 25)
	for i := range names {
		names[i] = fmt.Sprintf("image%d.png", i)
	}
	payload := make([]byte, 13<<20)
	state := uint32(7)
	for i := 0; i < len(payload); i += 8 {
		state = state*1664525 + 1013904223
		payload[i] = byte(state >> 24)
	}
	fixtures := map[string][]byte{"traversal": designTestZip(t, []string{"../art.webp"}, zip.Store, nil), "symlink": symlink.Bytes(), "case-duplicate": designTestZip(t, []string{"art.webp", "ART.webp"}, zip.Store, nil), "unicode": designTestZip(t, []string{"a\u0301.png"}, zip.Store, nil), "count": designTestZip(t, names, zip.Store, nil), "ratio": designTestZip(t, []string{"art.webp"}, zip.Deflate, make([]byte, 1<<20)), "total": designTestZip(t, []string{"a.png", "b.png"}, zip.Deflate, payload)}
	for name, raw := range fixtures {
		t.Run(name, func(t *testing.T) {
			if _, err := designReadZip(raw, ""); err == nil {
				t.Fatal("accepted hostile archive")
			}
		})
	}
}

// Test-only executable branch: the production worker has no probe/sleep verbs.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "__card-media-test-probe" {
		if len(os.Args) > 2 && os.Args[2] == "sleep" {
			time.Sleep(time.Minute)
			os.Exit(0)
		}
		report := map[string]bool{}
		for _, path := range os.Args[2:5] {
			_, e := os.Stat(path)
			report[path] = os.IsNotExist(e)
		}
		report["environment"] = os.Getenv("RYOKU_MEDIA_SECRET") == "" && os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" && os.Getenv("DBUS_SYSTEM_BUS_ADDRESS") == ""
		c, e := net.DialTimeout("tcp", os.Args[5], 100*time.Millisecond)
		report["network"] = e != nil
		if c != nil {
			c.Close()
		}
		c, e = net.DialTimeout("unix", os.Args[6], 100*time.Millisecond)
		report["abstract"] = e != nil
		if c != nil {
			c.Close()
		}
		json.NewEncoder(os.Stdout).Encode(report)
		os.Exit(0)
	}
}

func TestMediaSandboxContainmentAndTimeout(t *testing.T) {
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		t.Fatal("containment requires bubblewrap:", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "signing-material-marker")
	os.WriteFile(marker, []byte("not-a-real-key"), 0600)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	abstract := fmt.Sprintf("@ryoku-media-test-%d", os.Getpid())
	socket, err := net.Listen("unix", abstract)
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	args, err := mediaSandboxArgs(executable, nil)
	if err != nil {
		t.Fatal(err)
	}
	args[len(args)-1] = "__card-media-test-probe"
	args = append(args, marker, "/run/dbus/system_bus_socket", "/proc/1/root", listener.Addr().String(), abstract)
	t.Setenv("RYOKU_MEDIA_SECRET", "fixture-secret")
	cmd := mediaCommand(context.Background(), bwrap, args)
	raw, err := readLimitedCommand(cmd, 16<<10)
	if err != nil {
		t.Fatalf("sandbox containment unavailable: %v: %s", err, cmd.Stderr)
	}
	var report map[string]bool
	if err = json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err, string(raw))
	}
	if len(report) != 6 {
		t.Fatal("incomplete containment report", report)
	}
	for name, blocked := range report {
		if !blocked {
			t.Fatal("sandbox escape", name)
		}
	}
	args, err = mediaSandboxArgs(executable, nil)
	if err != nil {
		t.Fatal(err)
	}
	args[len(args)-1] = "__card-media-test-probe"
	args = append(args, "sleep")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cmd = mediaCommand(ctx, bwrap, args)
	start := time.Now()
	_, err = readLimitedCommand(cmd, 1024)
	if err == nil || ctx.Err() == nil || time.Since(start) > 3*time.Second {
		t.Fatal("wall timeout failed", err, ctx.Err())
	}
	if cmd.Process != nil {
		if err = syscall.Kill(cmd.Process.Pid, 0); err != syscall.ESRCH {
			t.Fatal("worker survived timeout", err)
		}
	}
}

func TestMediaNoUnsandboxedFallback(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := inspectMedia(context.Background(), sampleHero, false); err == nil || !strings.Contains(err.Error(), "bubblewrap") {
		t.Fatal("missing sandbox did not fail closed", err)
	}
}

func TestMediaPreviewCanonicalDigestAndReplacement(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	first, err := mediaPreview(testGIF(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	second, err := mediaPreview(testGIF(t, 3))
	if err != nil {
		t.Fatal(err)
	}
	if first["frames"] != 2 || second["frames"] != 3 || first["sourceSha"] == second["sourceSha"] {
		t.Fatal("poster confused whole-animation identity", first, second)
	}
	path, ok := second["path"].(string)
	if !ok {
		t.Fatal(second)
	}
	raw, err := readPictureFile(path)
	if err != nil {
		t.Fatal(err)
	}
	kind, err := sniffPicture(raw)
	if err != nil || kind.ext != "png" {
		t.Fatal("preview is not canonical PNG", err)
	}
	if _, err = mediaPreview([]byte("hostile replacement")); err == nil {
		t.Fatal("invalid replacement reused a cached clean claim")
	}
	chosen := writeChosen(t, "source.png", sampleHero)
	link := filepath.Join(t.TempDir(), "link.png")
	if err = os.Symlink(chosen, link); err != nil {
		t.Fatal(err)
	}
	if _, err = readChosenFile(link, maxPictureBytes, "a picture"); err == nil {
		t.Fatal("foreign symlink followed")
	}
}

func TestMediaDesignCanonicalizesUnusedAssetsAndReferences(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	p := designTestPack(t)
	clean, err := canonicalDesignPack(p, true)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := designValidate(clean)
	if err != nil {
		t.Fatal(err)
	}
	if clean.root == p.root || !strings.HasPrefix(spec.Badge.Emblem, clean.root+string(filepath.Separator)) {
		t.Fatal("preview still references original folder")
	}
	p.files["unused.webp"] = designTestRIFF(designTestVP8(256, 256))
	p.names = append(p.names, "unused.webp")
	if _, err = canonicalDesignPack(p, false); err == nil {
		t.Fatal("unused undecodable asset bypassed worker")
	}
}

func TestMediaTradeRulersSurviveWebPRecompression(t *testing.T) {
	levels, err := encodeLevels(codedDesign{name: "Friend", tagline: "Shared pixels", epithet: "Ryoku"})
	if err != nil {
		t.Fatal(err)
	}
	card := drawCard(levels, 1, color.RGBA{18, 12, 0, 255}, color.RGBA{226, 214, 196, 255})
	raw, err := pngBytes(card)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	input, output := filepath.Join(dir, "card.png"), filepath.Join(dir, "chat.webp")
	if err = os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	tool, err := exec.LookPath("cwebp")
	if err != nil {
		t.Fatal(err)
	}
	if out, e := exec.Command(tool, "-quiet", "-q", "90", "-resize", "504", "704", input, "-o", output).CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	foreign, err := readPictureFile(output)
	if err != nil {
		t.Fatal(err)
	}
	img, err := decodePicture(foreign)
	if err != nil {
		t.Fatal(err)
	}
	_, decoded, err := readCard(img)
	if err != nil {
		t.Fatal("visible rulers lost after isolated WebP canonicalization", err)
	}
	if decoded.name != "Friend" || decoded.tagline != "Shared pixels" {
		t.Fatal("image-only trade changed decoded data", decoded.name, decoded.tagline)
	}
}

func TestMediaParentRejectsForgedOutputFacts(t *testing.T) {
	result, err := inspectMedia(context.Background(), testGIF(t, 2), false)
	if err != nil {
		t.Fatal(err)
	}
	result.Width++
	if err = validateMediaResult(result, false); err == nil {
		t.Fatal("worker-provided dimensions were authoritative")
	}
	result.Width--
	result.Durations = append([]int(nil), result.Durations...)
	result.Durations[1]++
	if err = validateMediaResult(result, false); err == nil {
		t.Fatal("worker-provided timings were authoritative")
	}
	result.Durations = nil
	if err = validateMediaResult(result, false); err == nil {
		t.Fatal("missing frame facts were accepted")
	}
}
