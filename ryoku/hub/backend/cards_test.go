package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func sharedCard(t *testing.T, d codedDesign, format string) string {
	t.Helper()
	levels, err := encodeLevels(d)
	if err != nil {
		t.Fatal(err)
	}
	img := drawCard(levels, 1.45, color.RGBA{18, 12, 0, 255}, color.RGBA{226, 214, 196, 255})
	var raw bytes.Buffer
	switch format {
	case "png":
		err = png.Encode(&raw, img)
	case "gif":
		err = gif.Encode(&raw, img, nil)
	default:
		raw.Write(jpegOf(t, img, 80))
	}
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "shared."+format)
	if err := os.WriteFile(path, raw.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCardsAddStoresEntryAndLiftsOwnPicture(t *testing.T) {
	setCardMachine(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := atomicWrite(profileConfigPath(), []byte(sampleProfile), 0o644); err != nil {
		t.Fatal(err)
	}
	d := codedDesign{kind: 2, owner: [3]byte{0x3f, 0xa2, 0xc1}, name: "friend", tagline: "hello", epithet: "Ryoku",
		threads: 16, ramGb: 46, refresh: 165, width: 2560, gpuClass: 2, gpu: "NVIDIA GeForce RTX 4060 Laptop GPU", distro: 1, wm: 2}
	out := captureStdout(t, func() {
		if err := runCards([]string{"add", sharedCard(t, d, "jpg")}); err != nil {
			t.Fatal(err)
		}
	})
	var entry binderCard
	if err := json.Unmarshal([]byte(out), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.ID != "3fa2c1" || entry.Name != d.name || entry.Tagline != d.tagline || entry.Epithet != d.epithet || entry.Added.IsZero() {
		t.Fatalf("bad entry: %+v", entry)
	}
	wantHW := cardHardware{Threads: 16, RamGb: 46, Refresh: 164, Width: 2560, GPUClass: 2, GPU: "RTX 4060", Distro: "Arch Linux", WM: "Niri"}
	if entry.HW != wantHW {
		t.Fatalf("hardware = %+v, want %+v", entry.HW, wantHW)
	}
	if entry.Hero.Kind != "custom" || entry.Hero.Source != "" || !filepath.IsAbs(entry.Hero.File) || entry.Hero.File != filepath.Join(cardsDir(), entry.ID+".png") {
		t.Fatalf("bad lifted hero: %+v", entry.Hero)
	}
	f, err := os.Open(entry.Hero.File)
	if err != nil {
		t.Fatal(err)
	}
	pic, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	r, _, _, _ := pic.At(pic.Bounds().Dx()/2, pic.Bounds().Dy()/2).RGBA()
	if pic.Bounds().Dx() != 813 || pic.Bounds().Dy() != 566 || r>>8 < 150 {
		t.Fatalf("lifted %v, red %d: not the visible window", pic.Bounds(), r>>8)
	}
	stored, err := os.ReadFile(filepath.Join(cardsDir(), entry.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved binderCard
	if err := json.Unmarshal(stored, &saved); err != nil || !reflect.DeepEqual(saved, entry) {
		t.Fatalf("stored entry differs: %+v (%v)", saved, err)
	}
	d.refresh, d.gpu = 164, "RTX 4060"
	decoded, err := decodeLevels(entry.Levels)
	if err != nil || decoded != d {
		t.Fatalf("entry levels = %+v (%v), want %+v", decoded, err, d)
	}
	if profile, err := os.ReadFile(profileConfigPath()); err != nil || string(profile) != sampleProfile {
		t.Fatalf("adding a card changed my profile: %q (%v)", profile, err)
	}
}

func TestCardsSameOwnerReplacesKeepingAdded(t *testing.T) {
	setCardMachine(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	d := codedDesign{kind: 2, owner: [3]byte{1, 2, 3}, name: "first"}
	entry, err := addCard(sharedCard(t, d, "png"))
	if err != nil {
		t.Fatal(err)
	}
	entry.Added = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	old, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(filepath.Join(cardsDir(), entry.ID+".json"), old, 0o644); err != nil {
		t.Fatal(err)
	}
	d.kind, d.name, d.gallerySrc = 1, "second", "hawk"
	updated, err := addCard(sharedCard(t, d, "gif"))
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != entry.ID || !updated.Added.Equal(entry.Added) || updated.Name != "second" || updated.Hero != (cardHero{Kind: "gallery", Source: "hawk.png"}) {
		t.Fatalf("bad replacement: %+v", updated)
	}
	if _, err := os.Stat(entry.Hero.File); !os.IsNotExist(err) {
		t.Fatalf("replacement left the old own picture: %v", err)
	}
	cards, err := listCards()
	if err != nil || len(cards) != 1 || !reflect.DeepEqual(cards[0], updated) {
		t.Fatalf("replaced card list = %+v (%v)", cards, err)
	}
}

func TestCardsRefuseOwnCardAndGarbage(t *testing.T) {
	owner := setCardMachine(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if _, err := addCard(sharedCard(t, codedDesign{owner: owner}, "jpg")); err == nil || !strings.Contains(err.Error(), "your own card") {
		t.Fatalf("own card refusal = %v", err)
	}
	plain := filepath.Join(t.TempDir(), "plain.png")
	writeTinyPNG(t, plain)
	garbage := filepath.Join(t.TempDir(), "garbage.jpg")
	if err := os.WriteFile(garbage, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{plain, garbage} {
		if _, err := addCard(path); !errors.Is(err, errNoCardCode) {
			t.Fatalf("garbage refusal = %v", err)
		}
	}
	if _, err := os.Stat(cardsDir()); !os.IsNotExist(err) {
		t.Fatalf("refused card wrote to the binder: %v", err)
	}
}

func TestCardsListOrderAndRemove(t *testing.T) {
	setCardMachine(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	out := captureStdout(t, func() {
		if err := runCards([]string{"list"}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty binder = %q", out)
	}
	var entries []binderCard
	for i := range 3 {
		d := codedDesign{kind: 2, owner: [3]byte{byte(i + 1), 2, 3}, name: "friend"}
		entry, err := addCard(sharedCard(t, d, "png"))
		if err != nil {
			t.Fatal(err)
		}
		entry.Added = time.Date(2026, 11, 10+i, 20, 45, 0, 0, time.UTC)
		raw, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		if err := atomicWrite(filepath.Join(cardsDir(), entry.ID+".json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	cards, err := listCards()
	if err != nil || len(cards) != 3 || cards[0].ID != entries[2].ID || cards[1].ID != entries[1].ID || cards[2].ID != entries[0].ID {
		t.Fatalf("list order = %+v (%v)", cards, err)
	}
	if err := runCards([]string{"remove", entries[1].ID}); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".json", ".png"} {
		if _, err := os.Stat(filepath.Join(cardsDir(), entries[1].ID+ext)); !os.IsNotExist(err) {
			t.Fatalf("remove left %s: %v", ext, err)
		}
	}
	cards, err = listCards()
	if err != nil || len(cards) != 2 {
		t.Fatalf("list after remove = %+v (%v)", cards, err)
	}
	if err := removeCard("../escape"); err == nil {
		t.Fatal("remove accepted a path instead of an id")
	}
}

func TestCardsClipboardFallsBackToJPEG(t *testing.T) {
	setCardMachine(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	d := codedDesign{owner: [3]byte{4, 5, 6}, name: "clipboard"}
	path := sharedCard(t, d, "jpg")
	bin := t.TempDir()
	// A private wl-paste substitute leaves the real clipboard untouched.
	if err := os.WriteFile(filepath.Join(bin, "wl-paste"), []byte("#!/bin/sh\n[ \"$3\" = image/jpeg ] || exit 1\nexec /usr/bin/cat \"$CARD_CLIPBOARD_TEST\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("CARD_CLIPBOARD_TEST", path)
	entry, err := addCard("--clipboard")
	if err != nil || entry.ID != hex.EncodeToString(d.owner[:]) || entry.Hero != (cardHero{Kind: "default"}) {
		t.Fatalf("clipboard card = %+v (%v)", entry, err)
	}
}
