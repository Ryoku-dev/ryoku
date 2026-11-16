package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type cardHero struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
	File   string `json:"file"`
}

type cardHardware struct {
	Threads  int    `json:"threads"`
	RamGb    int    `json:"ramGb"`
	Refresh  int    `json:"refresh"`
	Width    int    `json:"width"`
	GPUClass int    `json:"gpuClass"`
	GPU      string `json:"gpu"`
	Distro   string `json:"distro"`
	WM       string `json:"wm"`
}

type binderCard struct {
	ID      string       `json:"id"`
	Added   time.Time    `json:"added"`
	Name    string       `json:"name"`
	Tagline string       `json:"tagline"`
	Epithet string       `json:"epithet"`
	Hero    cardHero     `json:"hero"`
	HW      cardHardware `json:"hw"`
	Levels  [][]int      `json:"levels"`
}

func cardsDir() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	return filepath.Join(base, "ryoku", "cards")
}

func validCardID(id string) bool {
	if len(id) != 6 || id != strings.ToLower(id) {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func cardPicture(path string) ([]byte, error) {
	if path != "--clipboard" {
		return readPictureFile(path)
	}
	for _, kind := range []string{"image/png", "image/jpeg"} {
		raw, err := readLimitedCommand(exec.Command("wl-paste", "--no-newline", "--type", kind), maxPictureBytes)
		if err == nil && len(raw) > 0 {
			return raw, nil
		}
		if errors.Is(err, errPictureTooLarge) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("the clipboard holds no picture: %w", errNoCardCode)
}

func addCard(path string) (binderCard, error) {
	var entry binderCard
	raw, err := cardPicture(path)
	if err != nil {
		return entry, err
	}
	img, err := decodePicture(raw)
	if err != nil {
		return entry, errors.Join(errNoCardCode, err)
	}
	levels, err := readLevels(img)
	if err != nil {
		return entry, err
	}
	d, err := decodeLevels(levels)
	if err != nil {
		return entry, err
	}
	owner, err := ownerFingerprint()
	if err != nil {
		return entry, err
	}
	if d.owner == owner {
		return entry, fmt.Errorf("you cannot add your own card to the binder")
	}
	levels, err = encodeLevels(d)
	if err != nil {
		return entry, err
	}
	entry = entryForDesign(d, levels)
	entry.Added = time.Now().UTC().Truncate(time.Second)
	dir, err := filepath.Abs(cardsDir())
	if err != nil {
		return binderCard{}, err
	}
	jsonPath := filepath.Join(dir, entry.ID+".json")
	imagePath := filepath.Join(dir, entry.ID+".png")
	if old, err := readBoundedFile(jsonPath, 64<<10); err == nil {
		var previous binderCard
		if err := json.Unmarshal(old, &previous); err != nil {
			return binderCard{}, err
		}
		entry.Added = previous.Added
	} else if !os.IsNotExist(err) {
		return binderCard{}, err
	}
	var picture bytes.Buffer
	switch d.kind {
	case 1:
		entry.Hero = cardHero{Kind: "gallery", Source: d.gallerySrc + ".png"}
	case 2:
		// Only the visible window travels with the card, never the original photo.
		s := float64(img.Bounds().Dx()) / cardW
		r := image.Rect(int(float64(windowRect.Min.X)*s), int(float64(windowRect.Min.Y)*s),
			int(float64(windowRect.Max.X)*s), int(float64(windowRect.Max.Y)*s)).Add(img.Bounds().Min)
		sub, ok := img.(interface {
			SubImage(image.Rectangle) image.Image
		})
		if !ok {
			return binderCard{}, errNoCardCode
		}
		if err := png.Encode(&picture, sub.SubImage(r)); err != nil {
			return binderCard{}, err
		}
		entry.Hero = cardHero{Kind: "custom", File: imagePath}
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return binderCard{}, err
	}
	before, err := snapshotFiles(jsonPath, imagePath)
	if err != nil {
		return binderCard{}, err
	}
	if entry.Hero.Kind == "custom" {
		err = atomicWrite(imagePath, picture.Bytes(), 0o644)
	} else {
		err = os.Remove(imagePath)
		if os.IsNotExist(err) {
			err = nil
		}
	}
	if err == nil {
		err = atomicWrite(jsonPath, encoded, 0o644)
	}
	if err != nil {
		return binderCard{}, errors.Join(err, before.restore())
	}
	return entry, nil
}

func entryForDesign(d codedDesign, levels [][]int) binderCard {
	return binderCard{
		ID:   hex.EncodeToString(d.owner[:]),
		Name: d.name, Tagline: d.tagline, Epithet: d.epithet,
		Hero: cardHero{Kind: "default"},
		HW: cardHardware{Threads: d.threads, RamGb: d.ramGb, Refresh: d.refresh, Width: d.width,
			GPUClass: d.gpuClass, GPU: d.gpu,
			Distro: []string{"", "Arch Linux", "CachyOS", "Fedora", "Void", "NixOS"}[d.distro],
			WM:     []string{"", "Hyprland", "Niri", "Wayfire"}[d.wm]},
		Levels: levels,
	}
}

func listCards() ([]binderCard, error) {
	cards := []binderCard{}
	files, err := os.ReadDir(cardsDir())
	if os.IsNotExist(err) {
		return cards, nil
	}
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if !file.Type().IsRegular() || !strings.HasSuffix(file.Name(), ".json") || !validCardID(strings.TrimSuffix(file.Name(), ".json")) {
			continue
		}
		raw, err := readBoundedFile(filepath.Join(cardsDir(), file.Name()), 64<<10)
		if err != nil {
			return nil, err
		}
		var entry binderCard
		if err := json.Unmarshal(raw, &entry); err != nil {
			return nil, err
		}
		d, err := decodeLevels(entry.Levels)
		if err != nil || entry.ID != strings.TrimSuffix(file.Name(), ".json") || entry.ID != hex.EncodeToString(d.owner[:]) {
			return nil, fmt.Errorf("invalid stored card")
		}
		safe := entryForDesign(d, entry.Levels)
		safe.Added = entry.Added
		if d.kind == 1 {
			safe.Hero = cardHero{Kind: "gallery", Source: d.gallerySrc + ".png"}
		}
		if d.kind == 2 {
			expected, err := filepath.Abs(filepath.Join(cardsDir(), entry.ID+".png"))
			if err != nil || entry.Hero.File != expected {
				return nil, fmt.Errorf("invalid stored card picture")
			}
			info, err := os.Lstat(expected)
			if err != nil || !info.Mode().IsRegular() {
				return nil, fmt.Errorf("invalid stored card picture")
			}
			safe.Hero = cardHero{Kind: "custom", File: expected}
		}
		entry = safe
		cards = append(cards, entry)
	}
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Added.Equal(cards[j].Added) {
			return cards[i].ID < cards[j].ID
		}
		return cards[i].Added.After(cards[j].Added)
	})
	return cards, nil
}

func removeCard(id string) error {
	if !validCardID(id) {
		return fmt.Errorf("invalid card id: %s", id)
	}
	for _, ext := range []string{".json", ".png"} {
		if err := os.Remove(filepath.Join(cardsDir(), id+ext)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func runCards(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("cards needs add|list|remove")
	}
	switch args[0] {
	case "add":
		if len(args) != 2 {
			return fmt.Errorf("cards add needs a picture path or --clipboard")
		}
		entry, err := addCard(args[1])
		if err != nil {
			return err
		}
		return printJSON(entry)
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("cards list takes no arguments")
		}
		cards, err := listCards()
		if err != nil {
			return err
		}
		return printJSON(cards)
	case "remove":
		if len(args) != 2 {
			return fmt.Errorf("cards remove needs a card id")
		}
		return removeCard(args[1])
	default:
		return fmt.Errorf("unknown cards subcommand: %s", args[0])
	}
}
