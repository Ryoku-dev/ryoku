package writing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const maxWords = 5000

var wordsCache struct {
	sync.Mutex
	path     string
	modified time.Time
	size     int64
	words    []string
	keys     []string
}

func wordsPath() string { return filepath.Join(paths.DataDir(), "words.txt") }

func WordsGet() (protocol.WritingWords, error) {
	wordsCache.Lock()
	defer wordsCache.Unlock()
	words, err := readWords(wordsPath())
	return protocol.WritingWords{Words: slices.Clone(words)}, err
}

func dictionaryWords() ([]string, error) {
	wordsCache.Lock()
	defer wordsCache.Unlock()
	return readWords(wordsPath())
}

func readWords(path string) ([]string, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		wordsCache.path, wordsCache.keys = "", nil
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if wordsCache.path == path && wordsCache.modified.Equal(info.ModTime()) && wordsCache.size == info.Size() {
		return wordsCache.words, nil
	}
	if info.Size() > maxWords*257 {
		return nil, badRequest("Keep your dictionary under 5,000 words.")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]string)
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		word, err := normalizeWord(line)
		if err != nil {
			return nil, err
		}
		byKey[strings.ToLower(word)] = word
	}
	if len(byKey) > maxWords {
		return nil, badRequest("Keep your dictionary under 5,000 words.")
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	words := make([]string, len(keys))
	for index, key := range keys {
		words[index] = byKey[key]
	}
	wordsCache.path, wordsCache.modified, wordsCache.size, wordsCache.words, wordsCache.keys = path, info.ModTime(), info.Size(), words, keys
	return words, nil
}

func normalizeWord(word string) (string, error) {
	if !utf8.ValidString(word) {
		return "", badRequest("Enter one word up to 256 bytes.")
	}
	word = strings.TrimSpace(word)
	if word == "" || len(word) > 256 {
		return "", badRequest("Enter one word up to 256 bytes.")
	}
	for _, r := range word {
		if !unicode.IsLetter(r) && !unicode.IsMark(r) && r != '\'' && r != '’' && r != '-' {
			return "", badRequest("Enter one word, without spaces or numbers.")
		}
	}
	return word, nil
}

func WordsAdd(ctx context.Context, word string) (protocol.WritingWords, error) {
	return changeWord(ctx, word, true)
}
func WordsRemove(ctx context.Context, word string) (protocol.WritingWords, error) {
	return changeWord(ctx, word, false)
}

func changeWord(ctx context.Context, word string, add bool) (protocol.WritingWords, error) {
	word, err := normalizeWord(word)
	if err != nil {
		return protocol.WritingWords{}, err
	}
	wordsCache.Lock()
	defer wordsCache.Unlock()
	path := wordsPath()
	current, err := readWords(path)
	if err != nil {
		return protocol.WritingWords{}, err
	}
	words := slices.Clone(current)
	index, exists := slices.BinarySearch(wordsCache.keys, strings.ToLower(word))
	if add && !exists {
		if len(words) >= maxWords {
			return protocol.WritingWords{}, badRequest("Your dictionary can hold up to 5,000 words.")
		}
		words = slices.Insert(words, index, word)
	} else if add && exists && words[index] != word {
		words[index] = word
	} else if !add && exists {
		words = slices.Delete(words, index, index+1)
	} else {
		return protocol.WritingWords{Words: words}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return protocol.WritingWords{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return protocol.WritingWords{}, err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".words-*")
	if err != nil {
		return protocol.WritingWords{}, err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return protocol.WritingWords{}, err
	}
	raw := strings.Join(words, "\n")
	if raw != "" {
		raw += "\n"
	}
	if _, err := file.WriteString(raw); err != nil {
		file.Close()
		return protocol.WritingWords{}, err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return protocol.WritingWords{}, err
	}
	if err := file.Close(); err != nil {
		return protocol.WritingWords{}, err
	}
	if err := ctx.Err(); err != nil {
		return protocol.WritingWords{}, err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return protocol.WritingWords{}, err
	}
	wordsCache.path = ""
	return protocol.WritingWords{Words: words}, nil
}
