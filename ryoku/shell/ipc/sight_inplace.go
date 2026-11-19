package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type sightInPlace struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Language string `json:"language"`
	App      string `json:"app"`
	Action   string `json:"action"`
	Output   string `json:"output"`
	Created  int64  `json:"created"`
}

func sightInPlaceAction(action string) bool {
	switch action {
	case "auto", "translate", "proofread", "rewrite", "fluent":
		return true
	}
	if !strings.HasPrefix(action, "tone:") {
		return false
	}
	switch strings.TrimPrefix(action, "tone:") {
	case "formal", "confident", "friendly", "diplomatic", "direct", "empathetic", "assertive", "constructive", "engaging", "exciting", "informative", "inspirational", "objective", "persuasive", "academic", "casual", "compassionate", "cooperative", "fluent", "professional", "witty", "personable", "warm":
		return true
	}
	return false
}

func (s *sightService) publishInPlace(text, app, output string, epoch uint64) {
	if strings.TrimSpace(text) == "" {
		return
	}
	language := detectSightLanguage(text)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.options.feature("inplace") || s.inplaceEpoch != epoch || s.quietBlockedLocked(s.focusedApp(), time.Now()) {
		return
	}
	s.next++
	s.frame.InPlace = &sightInPlace{ID: fmt.Sprintf("p%x-%x", time.Now().UnixNano(), s.next), Text: text, Language: language, App: app, Action: s.options.InPlace.Action, Output: output, Created: time.Now().UnixMilli()}
	s.frame.Hidden = false
	s.publishLocked()
}

func (s *sightService) inplaceDone(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" || s.frame.InPlace != nil && s.frame.InPlace.ID == id {
		s.inplaceEpoch++
		if s.frame.InPlace != nil {
			s.frame.InPlace = nil
			s.publishLocked()
		}
	}
}

func (s *sightService) clearTheiaLocked() {
	s.highlightEpoch++
	s.inplaceEpoch++
	for _, card := range s.frame.Cards {
		if card.Image != "" {
			if path, err := validateSightImage(s.root, card.Image); err == nil {
				_ = os.Remove(path)
			}
		}
	}
	s.frame.Cards = []sightCard{}
	s.frame.Select, s.frame.Highlight, s.frame.InPlace = nil, nil, nil
	s.frame.Hidden, s.frame.Watching = false, false
	s.frame.Reason = "theia-off"
	s.publishLocked()
}
