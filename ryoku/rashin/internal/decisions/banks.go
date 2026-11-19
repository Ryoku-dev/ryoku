package decisions

// Derived from oh-my-pi (MIT), packages/coding-agent/src/sharpshooter/backend.ts@c7fff67

import (
	"context"
	"os"
	"sort"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine"
	"ryoku-rashin/internal/protocol"
)

// Budgets of the system-prompt block: about 15,000 tokens for the full
// profile, 2,000 for the compact one, at four bytes a token.
const (
	BlockBytes        = 60000
	CompactBlockBytes = 8000
)

// Blocks adds the project's decisions to the system prompt. The text only
// changes when a consolidation or a delete rewrote the files, so the prompt
// prefix stays byte-stable between turns.
func (s *Service) Blocks(ctx context.Context, tc engine.TurnContext) []engine.Block {
	if !informs(tc.Profile.Mode) || !s.enabled() {
		return nil
	}
	project := projectRoot(tc)
	if project == "" {
		return nil
	}
	id := BankID(project)
	s.arm(id, project)
	budget := BlockBytes
	if tc.Profile.Compact() {
		budget = CompactBlockBytes
	}
	text := s.blockText(id, budget)
	if text == "" {
		return nil
	}
	return []engine.Block{{Name: engine.BlockDecisions, Text: text, Budget: budget}}
}

// blockText renders a bank for the prompt: the intro, then each non-empty
// area under its heading with its topics below, cut at a line within
// budget bytes.
func (s *Service) blockText(id string, budget int) string {
	var parts []string
	for _, area := range ordered() {
		notes := parseNotes(s.readArea(id, area))
		if len(notes) == 0 {
			continue
		}
		var b strings.Builder
		b.WriteString("### " + strings.ToUpper(area[:1]) + area[1:] + "\n")
		for i, n := range notes {
			if n.Section != "" && (i == 0 || n.Section != notes[i-1].Section) {
				b.WriteString("#### " + n.Section + "\n")
			}
			b.WriteString("- " + n.Text + "\n")
		}
		parts = append(parts, strings.TrimRight(b.String(), "\n"))
	}
	if len(parts) == 0 {
		return ""
	}
	return cutLines(blockIntro+"\n\n"+strings.Join(parts, "\n\n"), budget)
}

// Banks lists decision banks, one project's or every one, most recently
// consolidated first. It works with the feature off, so what was learned
// stays visible.
func (s *Service) Banks(ctx context.Context, project string) (protocol.LearningDecisionsResult, error) {
	result := protocol.LearningDecisionsResult{Banks: []protocol.DecisionBank{}}
	if strings.TrimSpace(project) != "" {
		root := cleanProject(project)
		if root == "" {
			return result, nil
		}
		if bank, ok := s.read(BankID(root)); ok {
			result.Banks = append(result.Banks, bank)
		}
		return result, nil
	}
	entries, err := os.ReadDir(s.root())
	if err != nil {
		return result, nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if bank, ok := s.read(entry.Name()); ok {
			result.Banks = append(result.Banks, bank)
		}
	}
	sort.SliceStable(result.Banks, func(i, j int) bool {
		a, b := result.Banks[i].Consolidated, result.Banks[j].Consolidated
		switch {
		case a != nil && b != nil && !a.Equal(*b):
			return a.After(*b)
		case (a == nil) != (b == nil):
			return a != nil
		}
		return result.Banks[i].Project < result.Banks[j].Project
	})
	return result, nil
}

// read loads one bank; a folder without a project in its state is not one.
func (s *Service) read(id string) (protocol.DecisionBank, bool) {
	st := s.readState(id)
	if st.Project == "" {
		return protocol.DecisionBank{}, false
	}
	bank := protocol.DecisionBank{
		Project:      st.Project,
		Bank:         id,
		Architecture: s.readArea(id, protocol.DecisionArchitecture),
		Product:      s.readArea(id, protocol.DecisionProduct),
		Style:        s.readArea(id, protocol.DecisionStyle),
		Queued:       s.queueDepth(id),
		Consolidated: st.Consolidated,
	}
	if st.Error != nil {
		bank.Error = st.Error.Message
	}
	return bank, true
}

// Delete removes one decision: the first note of the area whose text,
// trimmed and without its list marker, equals the given line. A topic left
// empty goes with it.
func (s *Service) Delete(ctx context.Context, p protocol.DecisionDeleteParams) (protocol.DecisionBank, error) {
	if !validArea(p.Area) {
		return protocol.DecisionBank{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("That decision area is not known."))
	}
	line := bulletText(strings.TrimSpace(p.Line))
	root := cleanProject(p.Project)
	if root == "" || line == "" {
		return protocol.DecisionBank{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That decision is no longer there."))
	}
	id := BankID(root)
	bank := s.bank(id)
	bank.mu.Lock()
	notes := parseNotes(s.readArea(id, p.Area))
	at := -1
	for i, n := range notes {
		if n.Text == line {
			at = i
			break
		}
	}
	if at < 0 {
		bank.mu.Unlock()
		return protocol.DecisionBank{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That decision is no longer there."))
	}
	notes = append(notes[:at], notes[at+1:]...)
	err := s.writeArea(id, p.Area, renderNotes(notes))
	bank.mu.Unlock()
	if err != nil {
		return protocol.DecisionBank{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The decision could not be removed: %v.", err))
	}
	result, ok := s.read(id)
	if !ok {
		return protocol.DecisionBank{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That decision is no longer there."))
	}
	return result, nil
}
