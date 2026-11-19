package main

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type sightVoice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Language string `json:"language"`
}

type sightVoices struct {
	Engine  string       `json:"engine"`
	Voices  []sightVoice `json:"voices"`
	Missing string       `json:"missing"`
}

type sightSpeech struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func sightPiperDirectory() string {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, _ := os.UserHomeDir()
		data = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(data, "piper", "voices")
}

func parseSightVoices(raw string) []sightVoice {
	voices := []sightVoice{}
	seen := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if _, err := strconv.Atoi(fields[0]); err != nil || seen[fields[1]] {
			continue
		}
		voices = append(voices, sightVoice{ID: fields[1], Name: strings.ReplaceAll(fields[3], "_", " "), Language: fields[1]})
		seen[fields[1]] = true
	}
	return voices
}

func (s *sightService) voices() sightVoices {
	result := sightVoices{Voices: []sightVoice{}}
	if _, err := sightOutputLookPath("espeak-ng"); err == nil {
		result.Engine = "espeak-ng"
		ctx, cancel := context.WithTimeout(s.outputContext(), 5*time.Second)
		raw, err := runSightInputCommand(ctx, "", "espeak-ng", "--voices")
		cancel()
		if err == nil {
			result.Voices = append(result.Voices, parseSightVoices(string(raw))...)
		}
	} else {
		result.Missing = "espeak-ng"
	}
	_, piperErr := sightOutputLookPath("piper")
	directory := sightPiperDirectory()
	entries, _ := os.ReadDir(directory)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".onnx") {
			continue
		}
		info, err := os.Stat(filepath.Join(directory, entry.Name()))
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".onnx")
		language, _, _ := strings.Cut(name, "-")
		result.Voices = append(result.Voices, sightVoice{ID: "piper:" + entry.Name(), Name: strings.ReplaceAll(name, "_", " "), Language: strings.ReplaceAll(language, "_", "-")})
		if result.Engine == "" && piperErr == nil {
			result.Engine = "piper"
		}
	}
	return result
}

func sightSpeechRate(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 {
		return 1
	}
	return math.Max(0.75, math.Min(1.5, rate))
}

func sightPiperModel(voice string) string {
	file, ok := strings.CutPrefix(voice, "piper:")
	if !ok || file == "" || filepath.Base(file) != file || !strings.HasSuffix(file, ".onnx") {
		return ""
	}
	path := filepath.Join(sightPiperDirectory(), file)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		return path
	}
	return ""
}

func (s *sightService) speak(id, card, text, voice string, rate float64) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("failed")
	}
	s.speechMu.Lock()
	defer s.speechMu.Unlock()
	s.stopSpeakingLocked()
	s.mu.Lock()
	allowed := s.options.feature("readaloud") && s.rashin
	if voice == "" {
		voice = s.options.ReadAloud.Voice
	}
	if rate == 0 {
		rate = s.options.ReadAloud.Rate
	}
	s.mu.Unlock()
	if !allowed || s.outputContext().Err() != nil {
		return errors.New("failed")
	}
	ctx, cancel := context.WithTimeout(s.outputContext(), 10*time.Minute)
	rate = sightSpeechRate(rate)
	name := "espeak-ng"
	args := []string{}
	wave := ""
	if model := sightPiperModel(voice); model != "" {
		if _, err := sightOutputLookPath("piper"); err == nil {
			if err := os.MkdirAll(s.root, 0700); err != nil {
				cancel()
				return errors.New("failed")
			}
			file, err := os.CreateTemp(s.root, "theia-speech-*.wav")
			if err != nil {
				cancel()
				return errors.New("failed")
			}
			wave = file.Name()
			file.Close()
			name = "piper"
			args = []string{"--model", model, "--output_file", wave, "--length_scale", strconv.FormatFloat(1/rate, 'f', 4, 64)}
		}
	}
	if wave == "" {
		if _, err := sightOutputLookPath("espeak-ng"); err != nil {
			cancel()
			return errors.New("no-engine")
		}
		if voice != "" && !strings.HasPrefix(voice, "piper:") {
			args = append(args, "-v", voice)
		}
		args = append(args, "-s", strconv.Itoa(int(math.Round(175*rate))), "--stdin")
	}
	cmd := sightOutputCommand(ctx, name, args...)
	sightPrepareCommand(cmd)
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Start(); err != nil {
		cancel()
		if wave != "" {
			os.Remove(wave)
		}
		return errors.New("failed")
	}
	job := &sightSpeech{cancel: cancel, done: make(chan struct{})}
	s.mu.Lock()
	s.speech = job
	s.frame.Speaking = &sightSpeaking{ID: id, Card: card, Started: time.Now().UnixMilli()}
	s.publishLocked()
	s.mu.Unlock()
	go s.waitSpeaking(ctx, job, cmd, wave)
	return nil
}

func (s *sightService) waitSpeaking(ctx context.Context, job *sightSpeech, cmd *exec.Cmd, wave string) {
	defer close(job.done)
	defer job.cancel()
	err := cmd.Wait()
	if wave != "" {
		if err == nil && ctx.Err() == nil {
			_, _ = runSightInputCommand(ctx, "", "pw-play", wave)
		}
		os.Remove(wave)
	}
	s.mu.Lock()
	if s.speech == job {
		s.speech = nil
		s.frame.Speaking = nil
		s.publishLocked()
	}
	s.mu.Unlock()
}

func (s *sightService) stopSpeakingLocked() {
	s.mu.Lock()
	job := s.speech
	s.mu.Unlock()
	if job != nil {
		job.cancel()
		<-job.done
	}
}

func (s *sightService) stopSpeaking() {
	s.speechMu.Lock()
	defer s.speechMu.Unlock()
	s.stopSpeakingLocked()
}
