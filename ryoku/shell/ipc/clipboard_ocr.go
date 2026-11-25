package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type clipReadOutput struct{ bytes.Buffer }

func (b *clipReadOutput) Write(data []byte) (int, error) {
	if b.Len()+len(data) > 4<<20 {
		return 0, errors.New("output-too-large")
	}
	return b.Buffer.Write(data)
}

func prepareClipRead(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
}

func parseClipReadLanguages(raw string) []string {
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

func clipReadLanguageIntersection(requested, installed []string) []string {
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

func decodeClipReadCodes(raw []byte) ([]string, error) {
	var document struct {
		Sources []struct {
			Images []struct {
				Symbols []struct {
					Data string `xml:"data"`
				} `xml:"symbol"`
			} `xml:"image"`
		} `xml:"source"`
	}
	if err := xml.Unmarshal(raw, &document); err != nil {
		return nil, err
	}
	codes := []string{}
	for _, source := range document.Sources {
		for _, img := range source.Images {
			for _, symbol := range img.Symbols {
				codes = append(codes, symbol.Data)
			}
		}
	}
	return codes, nil
}
