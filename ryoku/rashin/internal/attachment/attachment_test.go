package attachment

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// the eight bytes every PNG starts with, and enough after them to be a file
var png = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)

func write(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A screenshot saved without an extension is still a picture, and a log
// named like one is not: what the bytes say is what the model would get.
func TestAPictureIsToldByItsBytes(t *testing.T) {
	image, ok, err := ReadImage(write(t, "screenshot", png))
	if err != nil || !ok || image.Mime != "image/png" || !bytes.Equal(image.Data, png) {
		t.Fatalf("a PNG without an extension read as %q, %v, %v with %d bytes", image.Mime, ok, err, len(image.Data))
	}
	if _, ok, err := ReadImage(write(t, "build.png", []byte("error: linker failed\n"))); ok || err != nil {
		t.Fatalf("a text file named .png read as a picture: %v, %v", ok, err)
	}
}

func TestWhatIsNotAPictureIsNotAnError(t *testing.T) {
	for name, path := range map[string]string{
		"a folder":      t.TempDir(),
		"an empty file": write(t, "empty", nil),
		"a drawing":     write(t, "mark.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)),
	} {
		if _, ok, err := ReadImage(path); ok || err != nil {
			t.Fatalf("%s read as a picture: %v, %v", name, ok, err)
		}
	}
	if _, _, err := ReadImage(filepath.Join(t.TempDir(), "gone.png")); err == nil {
		t.Fatal("a file that is not there read without an error")
	}
}

func TestAPictureOverTheLimitIsRefusedWhole(t *testing.T) {
	large := append(append([]byte(nil), png...), make([]byte, MaxImage)...)
	image, ok, err := ReadImage(write(t, "large.png", large))
	if !ok || !errors.Is(err, ErrTooLarge) || len(image.Data) != 0 {
		t.Fatalf("a picture over the limit read as %v, %v with %d bytes", ok, err, len(image.Data))
	}
}

func TestTextIsReadUpToALimitThatNeverSplitsALetter(t *testing.T) {
	text, total, ok, err := ReadText(write(t, "build.log", []byte("step 1 ok\nstep 2 failed\n")), 1024)
	if err != nil || !ok || text != "step 1 ok\nstep 2 failed\n" || total != 24 {
		t.Fatalf("a short log read as %q of %d, %v, %v", text, total, ok, err)
	}
	// each of these letters is three bytes, and the limit falls inside the third
	text, total, ok, err = ReadText(write(t, "notes.txt", []byte("日本語のメモ")), 8)
	if err != nil || !ok || text != "日本" || total != 18 {
		t.Fatalf("text cut inside a letter read as %q of %d, %v, %v", text, total, ok, err)
	}
}

func TestWhatIsNotTextIsNotReadAsText(t *testing.T) {
	for name, path := range map[string]string{
		"a picture":   write(t, "shot.png", png),
		"a program":   write(t, "tool", []byte("\x7fELF\x02\x01\x01\x00\x00\x00")),
		"a folder":    t.TempDir(),
		"other bytes": write(t, "latin1.txt", []byte("caf\xe9 au lait, cr\xe8me br\xfbl\xe9e")),
	} {
		if text, _, ok, err := ReadText(path, 1024); ok || err != nil {
			t.Fatalf("%s read as text %q: %v, %v", name, text, ok, err)
		}
	}
}
