package prompts

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

var storeMu sync.Mutex
var variable = regexp.MustCompile(`\{\{\s*([a-zA-Z][a-zA-Z0-9_]*)\s*\}\}`)

const maxStoreBytes = 8 << 20

func List() ([]protocol.Prompt, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	return listUnlocked()
}

func listUnlocked() ([]protocol.Prompt, error) {
	items, err := readUnlocked()
	if err != nil {
		return nil, err
	}
	return append(BuiltIns(), items...), nil
}

func readUnlocked() ([]protocol.Prompt, error) {
	file, err := os.Open(filepath.Join(paths.DataDir(), "prompts.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxStoreBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxStoreBytes {
		return nil, errors.New("The prompt library is too large")
	}
	var items []protocol.Prompt
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("Read the prompt library: %w", err)
	}
	for index := range items {
		items[index].BuiltIn = false
	}
	return items, nil
}

func Save(p protocol.PromptSaveParams) (protocol.Prompt, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || utf8.RuneCountInString(p.Name) > 100 {
		return protocol.Prompt{}, errors.New("Give the prompt a name of up to 100 characters")
	}
	if strings.TrimSpace(p.Text) == "" || len(p.Text) > 64<<10 {
		return protocol.Prompt{}, errors.New("Write a prompt of up to 64 KB")
	}
	if p.Kind != "prompt" && p.Kind != "template" && p.Kind != "snippet" {
		return protocol.Prompt{}, errors.New("Choose prompt, template or snippet")
	}
	if strings.HasPrefix(p.ID, "builtin:") {
		return protocol.Prompt{}, errors.New("Copy a built-in prompt before changing it")
	}
	items, err := readUnlocked()
	if err != nil {
		return protocol.Prompt{}, err
	}
	index := -1
	if p.ID == "" {
		var id [12]byte
		if _, err := rand.Read(id[:]); err != nil {
			return protocol.Prompt{}, err
		}
		p.ID = hex.EncodeToString(id[:])
	} else {
		for i, item := range items {
			if item.ID == p.ID {
				index = i
				break
			}
		}
		if index < 0 {
			return protocol.Prompt{}, errors.New("That prompt no longer exists")
		}
	}
	item := protocol.Prompt{ID: p.ID, Name: p.Name, Kind: p.Kind, Text: p.Text, Updated: time.Now().UTC().Format(time.RFC3339)}
	if index < 0 {
		items = append(items, item)
	} else {
		items[index] = item
	}
	return item, writeUnlocked(items)
}

func Delete(id string) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	if strings.HasPrefix(id, "builtin:") {
		return errors.New("Built-in prompts cannot be deleted")
	}
	items, err := readUnlocked()
	if err != nil {
		return err
	}
	for index, item := range items {
		if item.ID == id {
			return writeUnlocked(append(items[:index], items[index+1:]...))
		}
	}
	return errors.New("That prompt no longer exists")
}

func writeUnlocked(items []protocol.Prompt) error {
	data, err := json.MarshalIndent(items, "", "\t")
	if err != nil {
		return err
	}
	if len(data) > maxStoreBytes {
		return errors.New("The prompt library is full")
	}
	dir := paths.DataDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".prompts-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), filepath.Join(dir, "prompts.json"))
}

func Variables(text string) []string {
	var names []string
	seen := make(map[string]bool)
	for _, match := range variable.FindAllStringSubmatch(text, -1) {
		if !seen[match[1]] {
			seen[match[1]] = true
			names = append(names, match[1])
		}
	}
	return names
}

func Expand(item protocol.Prompt, vars map[string]string) (string, error) {
	matches := variable.FindAllStringSubmatchIndex(item.Text, -1)
	today := time.Now().Format("2006-01-02")
	size := len(item.Text)
	for _, match := range matches {
		name := item.Text[match[2]:match[3]]
		value, exists := vars[name]
		if !exists {
			if name != "date" {
				return "", fmt.Errorf("Fill in %s first", name)
			}
			value = today
		}
		size += len(value) - (match[1] - match[0])
		if size > 256<<10 {
			return "", errors.New("The expanded prompt is too large")
		}
	}
	var out strings.Builder
	out.Grow(size)
	cursor := 0
	for _, match := range matches {
		out.WriteString(item.Text[cursor:match[0]])
		name := item.Text[match[2]:match[3]]
		value, exists := vars[name]
		if !exists {
			value = today
		}
		out.WriteString(value)
		cursor = match[1]
	}
	out.WriteString(item.Text[cursor:])
	return out.String(), nil
}

func ExpandID(p protocol.PromptExpandParams) (string, error) {
	items, err := List()
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.ID == p.ID {
			return Expand(item, p.Vars)
		}
	}
	return "", errors.New("That prompt no longer exists")
}
