package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHostileCardTextThroughJPEG(t *testing.T) {
	for _, text := range []string{"a\x00b\x1fc", "a\u0085b", "a\u061cb", "a\u200eb", "a\u200fb", "a\u202ab", "a\u202eb", "a\u2066b", "a\u2069b", "a\u200bb", "a\u200cb", "a\u200db", "a\u2060b", "a\ufeffb", "<img src=//x>", "<a href=//x>x"} {
		d := codedDesign{name: text, tagline: text, epithet: text, gpu: text}
		levels, err := encodeLevels(d)
		if err != nil {
			t.Fatal(err)
		}
		raw := jpegOf(t, drawCard(levels, 1, color.RGBA{18, 12, 0, 255}, color.RGBA{226, 214, 196, 255}), 80)
		img, err := decodePicture(raw)
		if err != nil {
			t.Fatal(err)
		}
		read, err := readLevels(img)
		if err != nil {
			t.Fatal(err)
		}
		got, err := decodeLevels(read)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{cleanCardText(clip(text, 14)), cleanCardText(clip(text, 20)), cleanCardText(clip(text, 7)), cleanCardText(shortGPU(text))}
		for i, value := range []string{got.name, got.tagline, got.epithet, got.gpu} {
			if value != want[i] {
				t.Fatalf("%q field %d = %q, want %q", text, i, value, want[i])
			}
		}
	}
	bad := (codedDesign{name: "\xff"}).bytes()
	if _, err := parseDesign(bad); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func bombPNG(w, h uint32) []byte {
	raw := append([]byte(nil), []byte("\x89PNG\r\n\x1a\n")...)
	body := make([]byte, 13)
	binary.BigEndian.PutUint32(body, w)
	binary.BigEndian.PutUint32(body[4:], h)
	body[8], body[9] = 8, 2
	chunk := append([]byte("IHDR"), body...)
	raw = binary.BigEndian.AppendUint32(raw, uint32(len(body)))
	raw = append(raw, chunk...)
	return binary.BigEndian.AppendUint32(raw, crc32.ChecksumIEEE(chunk))
}

func TestPictureRejectsDecompressionBombBeforeDecode(t *testing.T) {
	for _, size := range [][2]uint32{{100000, 100000}, {8192, 8192}, {8193, 1}, {1, 8193}} {
		start := time.Now()
		if _, err := decodePicture(bombPNG(size[0], size[1])); !errors.Is(err, errPictureTooLarge) {
			t.Fatalf("size %v: %v", size, err)
		}
		if time.Since(start) > time.Second {
			t.Fatal("declared dimensions were not rejected promptly")
		}
	}
}

func TestPictureFileLimitBeforeRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxPictureBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := cardPicture(path); !errors.Is(err, errPictureTooLarge) {
		t.Fatalf("oversized file: %v", err)
	}
}

func TestClipboardLimitKillsWriter(t *testing.T) {
	bin := t.TempDir()
	// An endless real child exercises the kill-and-reap path without the live clipboard.
	if err := os.WriteFile(filepath.Join(bin, "wl-paste"), []byte("#!/bin/sh\nexec /usr/bin/yes x\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	start := time.Now()
	if _, err := cardPicture("--clipboard"); !errors.Is(err, errPictureTooLarge) {
		t.Fatalf("oversized clipboard: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("oversized clipboard child was not killed promptly")
	}
}

func TestGalleryAndIDsRejectTraversal(t *testing.T) {
	for _, name := range []string{"../hawk", "a/b", "a\\b", "a.png", "http:x", "a%2fb", "A", "a\u200bb", ""} {
		if validGalleryName(name) {
			t.Fatalf("gallery name accepted: %q", name)
		}
		levels, err := encodeLevels(codedDesign{kind: 1, gallerySrc: name})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decodeLevels(levels); err == nil {
			t.Fatalf("gallery code accepted: %q", name)
		}
	}
	for _, id := range []string{"../aaa", "AABBCCDDEEFF0011", "aabbcc", "aabbccddeeff001", "aabbccddeeff00112", "gg00000000000000", "aa/bbb0000000000", "000000000000000\x00"} {
		if validCardID(id) || removeCard(id) == nil {
			t.Fatalf("id accepted: %q", id)
		}
	}
}

func TestStoredCardCannotOverridePictureOrWords(t *testing.T) {
	setCardMachine(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	entry, err := addCard(sharedCard(t, codedDesign{kind: 2, cardID: [8]byte{1, 2, 3}, name: "friend"}, "png"))
	if err != nil {
		t.Fatal(err)
	}
	entry.Name = "\u202eevil"
	raw, _ := json.Marshal(entry)
	path := filepath.Join(cardsDir(), entry.ID+".json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cards, err := listCards()
	if err != nil || len(cards) != 1 || cards[0].Name != "friend" {
		t.Fatalf("unsafe stored words: %+v (%v)", cards, err)
	}
	entry.Hero.File = "/etc/passwd"
	raw, _ = json.Marshal(entry)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := listCards(); err == nil {
		t.Fatal("outside-store picture accepted")
	}
	entry.Hero.File = filepath.Join(cardsDir(), entry.ID+".png")
	raw, _ = json.Marshal(entry)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(entry.Hero.File); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", entry.Hero.File); err != nil {
		t.Fatal(err)
	}
	if _, err := listCards(); err == nil {
		t.Fatal("symlinked picture accepted")
	}
}

func TestDecodeRejectsMalformedLevelShape(t *testing.T) {
	for _, levels := range [][][]int{nil, {nil}, {{1}}, {{1, 2}, {2}, {3}}} {
		if _, err := decodeLevels(levels); err == nil {
			t.Fatal("malformed levels accepted")
		}
	}
	for _, p := range []int{-1, 0, 1, 20, 256} {
		if _, err := rsDecode(nil, p); err == nil {
			t.Fatal("empty RS codeword accepted")
		}
	}
	if _, err := rsDecode(bytes.Repeat([]byte{0}, 256), 20); err == nil {
		t.Fatal("oversize RS codeword accepted")
	}
	if _, err := parseDesign([]byte(strings.Repeat("x", 10000))); err == nil {
		t.Fatal("oversize body accepted")
	}
}
