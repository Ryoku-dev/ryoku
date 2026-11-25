package writing

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

var voiceMu sync.Mutex

func VoiceGet() (protocol.WritingVoice, error) {
	voiceMu.Lock()
	defer voiceMu.Unlock()
	return readVoice(filepath.Join(paths.DataDir(), "voice.json"))
}

func VoiceSet(p protocol.WritingVoiceSetParams) (protocol.WritingVoice, error) {
	voiceMu.Lock()
	defer voiceMu.Unlock()
	return setVoice(context.Background(), filepath.Join(paths.DataDir(), "voice.json"), p)
}

func readVoice(path string) (protocol.WritingVoice, error) {
	var voice protocol.WritingVoice
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return voice, nil
	}
	if err != nil {
		return voice, err
	}
	if err := json.Unmarshal(raw, &voice); err != nil {
		return protocol.WritingVoice{}, err
	}
	return voice, nil
}

func setVoice(ctx context.Context, path string, p protocol.WritingVoiceSetParams) (protocol.WritingVoice, error) {
	if err := ctx.Err(); err != nil {
		return protocol.WritingVoice{}, err
	}
	voice, err := readVoice(path)
	if err != nil {
		return voice, err
	}
	if p.Samples != nil {
		if len(*p.Samples) > 64*1024 || !utf8.ValidString(*p.Samples) {
			return voice, badRequest("Keep writing samples under 64 KiB of valid text.")
		}
		voice.Samples = *p.Samples
	}
	if p.Notes != nil {
		if !utf8.ValidString(*p.Notes) || utf8.RuneCountInString(*p.Notes) > 600 {
			return voice, badRequest("Keep voice notes under 600 characters.")
		}
		voice.Notes = strings.TrimSpace(*p.Notes)
	}
	voice.Updated = time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := json.Marshal(voice)
	if err != nil {
		return voice, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return voice, err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".voice-*")
	if err != nil {
		return voice, err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return voice, err
	}
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return voice, err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return voice, err
	}
	if err := file.Close(); err != nil {
		return voice, err
	}
	if err := ctx.Err(); err != nil {
		return voice, err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return voice, err
	}
	return voice, nil
}

func VoiceLearn(ctx context.Context, client llm.Client, p protocol.WritingVoiceLearnParams) (protocol.WritingVoice, error) {
	voice, err := VoiceGet()
	if err != nil {
		return voice, err
	}
	if strings.TrimSpace(voice.Samples) == "" {
		return voice, badRequest("Add a few samples of your writing first.")
	}
	notes, _, err := complete(ctx, client, p.Model, p.Local,
		"Describe only the writing style shown in these samples in at most 600 characters. Note sentence length, tone, vocabulary, punctuation and formatting that can guide a rewrite. Do not infer personal traits, identity or facts about the author. Return only short style notes, no heading or code fence.", voice.Samples)
	if err != nil {
		return voice, err
	}
	if strings.TrimSpace(notes) == "" {
		return voice, errors.New("The model returned no voice notes.")
	}
	if utf8.RuneCountInString(notes) > 600 {
		notes = string([]rune(notes)[:600])
	}
	voiceMu.Lock()
	defer voiceMu.Unlock()
	if err := ctx.Err(); err != nil {
		return voice, err
	}
	path := filepath.Join(paths.DataDir(), "voice.json")
	current, err := readVoice(path)
	if err != nil {
		return voice, err
	}
	if current.Updated != voice.Updated {
		return current, badRequest("Your voice changed while learning, try again with the saved samples.")
	}
	return setVoice(ctx, path, protocol.WritingVoiceSetParams{Notes: &notes})
}

func VoiceNotes() string {
	voice, err := VoiceGet()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(voice.Notes)
}
