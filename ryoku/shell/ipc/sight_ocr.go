package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math"
	"os"
	"os/exec"
	"strconv"
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
	command := sightOutputCommand(ctx, name, args...)
	sightPrepareCommand(command)
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
	return sightInstalledLanguagesContext(context.Background())
}
func sightInstalledLanguagesContext(parent context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
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
	installed, err := sightInstalledLanguagesContext(s.outputContext())
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
	ctx, cancel := context.WithTimeout(s.outputContext(), 30*time.Second)
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

type sightOCRBoxes struct {
	Width  int            `json:"width"`
	Height int            `json:"height"`
	Lines  []sightOCRLine `json:"lines"`
}

type sightOCRLine struct {
	Text   string  `json:"text"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func parseSightOCRBoxes(raw []byte, width, height int) ([]sightOCRLine, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("bad-image")
	}
	type lineBox struct {
		text                  []string
		left, top, right, end float64
	}
	boxes := []lineBox{}
	indices := map[string]int{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 12)
		if len(fields) != 12 || fields[0] != "5" {
			continue
		}
		text := strings.TrimSpace(fields[11])
		if text == "" {
			continue
		}
		var coords [4]float64
		for i := range coords {
			n, err := strconv.Atoi(fields[6+i])
			if err != nil {
				return nil, err
			}
			coords[i] = float64(n)
		}
		if coords[2] <= 0 || coords[3] <= 0 {
			continue
		}
		left := math.Max(0, math.Min(float64(width), coords[0]))
		top := math.Max(0, math.Min(float64(height), coords[1]))
		right := math.Max(left, math.Min(float64(width), coords[0]+coords[2]))
		end := math.Max(top, math.Min(float64(height), coords[1]+coords[3]))
		if right == left || end == top {
			continue
		}
		key := strings.Join(fields[1:5], "/")
		index, exists := indices[key]
		if !exists {
			index = len(boxes)
			indices[key] = index
			boxes = append(boxes, lineBox{left: left, top: top, right: right, end: end})
		}
		box := &boxes[index]
		box.text = append(box.text, text)
		box.left = math.Min(box.left, left)
		box.top = math.Min(box.top, top)
		box.right = math.Max(box.right, right)
		box.end = math.Max(box.end, end)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	lines := make([]sightOCRLine, 0, len(boxes))
	for _, box := range boxes {
		lines = append(lines, sightOCRLine{
			Text: strings.Join(box.text, " "),
			X:    box.left / float64(width), Y: box.top / float64(height),
			Width: (box.right - box.left) / float64(width), Height: (box.end - box.top) / float64(height),
		})
	}
	return lines, nil
}

type sightOCRRequest struct {
	cancel context.CancelFunc
}

func (s *sightService) cancelOCRLocked(id string) {
	for card, request := range s.ocrRequests {
		if id == "" || card == id {
			request.cancel()
			delete(s.ocrRequests, card)
		}
	}
}

func (s *sightService) ocrBoxes(id string) (result sightOCRBoxes, err error) {
	defer func() { err = sightOCRError(err) }()
	s.mu.Lock()
	if !s.options.On || !s.rashin || s.outputContext().Err() != nil {
		s.mu.Unlock()
		return result, errors.New("ocr-failed")
	}
	path := ""
	for _, card := range s.frame.Cards {
		if card.ID == id && card.Kind == "image" {
			path = card.Image
			break
		}
	}
	requested := append([]string(nil), s.options.OCRLanguages...)
	if path == "" {
		s.mu.Unlock()
		return result, errors.New("bad-image")
	}
	s.cancelOCRLocked(id)
	ctx, cancel := context.WithTimeout(s.outputContext(), 30*time.Second)
	request := &sightOCRRequest{cancel: cancel}
	if s.ocrRequests == nil {
		s.ocrRequests = make(map[string]*sightOCRRequest)
	}
	s.ocrRequests[id] = request
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		if s.ocrRequests[id] == request {
			delete(s.ocrRequests, id)
		}
		s.mu.Unlock()
	}()
	path, err = validateSightImage(s.root, path)
	if err != nil {
		return result, err
	}
	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	dimensions, _, err := image.DecodeConfig(file)
	file.Close()
	if err != nil || dimensions.Width <= 0 || dimensions.Height <= 0 {
		return result, errors.New("bad-image")
	}
	installed, err := sightInstalledLanguagesContext(ctx)
	if err != nil {
		return result, err
	}
	languages := sightLanguageIntersection(requested, installed)
	raw, err := runSightCommand(ctx, "tesseract", path, "stdout", "-l", strings.Join(languages, "+"), "--psm", "3", "tsv")
	if err != nil {
		return result, err
	}
	lines, err := parseSightOCRBoxes(raw, dimensions.Width, dimensions.Height)
	if err != nil {
		return result, err
	}
	return sightOCRBoxes{Width: dimensions.Width, Height: dimensions.Height, Lines: lines}, nil
}
