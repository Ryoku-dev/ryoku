package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

type sightCode struct {
	Format string `json:"format"`
	Text   string `json:"text"`
}
type sightOCRResult struct {
	Text     string      `json:"text"`
	Language string      `json:"language"`
	Codes    []sightCode `json:"codes"`
}
type sightOutput struct{ bytes.Buffer }

func (b *sightOutput) Write(data []byte) (int, error) {
	if b.Len()+len(data) > 4<<20 {
		return 0, errors.New("output-too-large")
	}
	return b.Buffer.Write(data)
}

var runSightCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	var output sightOutput
	command.Stdout = &output
	err := command.Run()
	if ctx.Err() != nil {
		return output.Bytes(), ctx.Err()
	}
	return output.Bytes(), err
}

func parseSightLanguages(raw string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		code := strings.TrimSpace(line)
		if code == "" || strings.ContainsAny(code, " :()\t") || seen[code] {
			continue
		}
		valid := true
		for _, r := range code {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '/' || r == '-') {
				valid = false
			}
		}
		if valid {
			result = append(result, code)
			seen[code] = true
		}
	}
	return result
}
func sightLanguageIntersection(requested, installed []string) []string {
	available := map[string]bool{}
	for _, code := range installed {
		available[code] = true
	}
	result := []string{}
	seen := map[string]bool{}
	for _, code := range requested {
		if available[code] && !seen[code] {
			result = append(result, code)
			seen[code] = true
		}
	}
	if len(result) == 0 {
		return []string{"eng"}
	}
	return result
}
func sightInstalledLanguages() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := runSightCommand(ctx, "tesseract", "--list-langs")
	if err != nil {
		return []string{}, sightOCRError(err)
	}
	return parseSightLanguages(string(raw)), nil
}
func decodeSightCodes(raw []byte) ([]sightCode, error) {
	var document struct {
		Sources []struct {
			Images []struct {
				Symbols []struct {
					Type string `xml:"type,attr"`
					Data string `xml:"data"`
				} `xml:"symbol"`
			} `xml:"image"`
		} `xml:"source"`
	}
	if err := xml.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	codes := []sightCode{}
	for _, source := range document.Sources {
		for _, img := range source.Images {
			for _, symbol := range img.Symbols {
				codes = append(codes, sightCode{Format: symbol.Type, Text: symbol.Data})
			}
		}
	}
	return codes, nil
}
func sightOCRError(err error) error {
	if err == nil {
		return nil
	}
	log.Printf("ryoku-shell: sight OCR: %v", err)
	if errors.Is(err, context.DeadlineExceeded) || err.Error() == "timeout" {
		return errors.New("timeout")
	}
	if err.Error() == "bad-image" {
		return err
	}
	return errors.New("ocr-failed")
}
func (s *sightService) ocr(id string) (result any, err error) {
	defer func() { err = sightOCRError(err) }()
	s.mu.Lock()
	path := ""
	for _, card := range s.frame.Cards {
		if card.ID == id && card.Kind == "image" {
			path = card.Image
			break
		}
	}
	requested := append([]string(nil), s.options.OCRLanguages...)
	s.mu.Unlock()
	if path == "" {
		return nil, errors.New("bad-image")
	}
	path, err = validateSightImage(s.root, path)
	if err != nil {
		return nil, err
	}
	installed, err := sightInstalledLanguages()
	if err != nil {
		return nil, err
	}
	languages := sightLanguageIntersection(requested, installed)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	dimensions, _, err := image.DecodeConfig(file)
	file.Close()
	if err != nil {
		return nil, errors.New("bad-image")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	input := path
	if dimensions.Height < 400 {
		temporary, err := os.CreateTemp(s.root, "ocr-*.png")
		if err != nil {
			return nil, err
		}
		input = temporary.Name()
		temporary.Close()
		defer os.Remove(input)
		if _, err := runSightCommand(ctx, "magick", path, "-resize", "200%", input); err != nil {
			return nil, err
		}
	}
	text, err := runSightCommand(ctx, "tesseract", input, "stdout", "-l", strings.Join(languages, "+"), "--psm", "3")
	if err != nil {
		return nil, err
	}
	raw, err := runSightCommand(ctx, "zbarimg", "--quiet", "--xml", path)
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 4 {
			return nil, err
		}
	}
	codes := []sightCode{}
	if len(bytes.TrimSpace(raw)) != 0 {
		codes, err = decodeSightCodes(raw)
		if err != nil {
			return nil, err
		}
	}
	transcript := string(text)
	return sightOCRResult{Text: transcript, Language: detectSightLanguage(transcript), Codes: codes}, nil
}
