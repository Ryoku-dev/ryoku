package decisions

// Derived from oh-my-pi (MIT), packages/coding-agent/src/sharpshooter/{consolidate,scheduler}.ts@c7fff67
//
// omp's consolidator rewrites the three files whole. A 2B model cannot
// reliably reproduce 360 lines, so here the model answers with changes
// against numbered notes (add, replace, remove) and the code applies them,
// which also means a bad answer cannot wipe a bank.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/memory/scan"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
)

const (
	consolidateOut = 2048
	// Retry after failures: the quiet time doubled per failure in a row,
	// at least a minute, at most six hours.
	minBackoff = time.Minute
	maxBackoff = 6 * time.Hour
	// The most of the project's own docs a consolidation reads, and the
	// least room worth sending them in.
	docsBytes    = 24000
	minDocsBytes = 2000
	// Allowance for the tool schema and message framing the estimate leaves
	// out.
	framingTokens = 1024
	maxSection    = 60
)

// schedule (re)arms a bank's consolidation after delay. Every new decision
// pushes it back, so the bank consolidates after a quiet spell.
func (s *Service) schedule(id, project string, delay time.Duration) {
	if !s.enabled() {
		return
	}
	bank := s.bank(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	bank.checked = true
	if bank.timer != nil {
		bank.timer.Stop()
	}
	bank.timer = time.AfterFunc(delay, func() { s.fire(id, project) })
}

// arm schedules a bank that has decisions waiting from an earlier run, once
// per bank and process.
func (s *Service) arm(id, project string) {
	bank := s.bank(id)
	s.mu.Lock()
	checked := bank.checked
	bank.checked = true
	s.mu.Unlock()
	if !checked && s.queueDepth(id) > 0 {
		s.schedule(id, project, s.quiet)
	}
}

func (s *Service) fire(id, project string) {
	bank := s.bank(id)
	s.mu.Lock()
	bank.timer = nil
	s.mu.Unlock()
	if !s.enabled() {
		// Turning the feature back on and the next turn in the project arm
		// the bank again.
		s.mu.Lock()
		bank.checked = false
		s.mu.Unlock()
		return
	}
	if !s.begin() {
		return
	}
	defer s.end()
	err := s.consolidate(s.ctx, id, project)
	if err == nil || s.ctx.Err() != nil {
		return
	}
	failures := min(max(s.readState(id).failures()-1, 0), 16)
	s.schedule(id, project, min(max(minBackoff, s.quiet)<<failures, maxBackoff))
}

func (st state) failures() int {
	if st.Error == nil {
		return 0
	}
	return max(st.Error.Failures, 1)
}

// consolidate folds a bank's queue into its notes. The deltas that fit the
// light model's window go in one call; the rest wait for the next pass.
func (s *Service) consolidate(ctx context.Context, id, project string) error {
	bank := s.bank(id)
	bank.mu.Lock()
	items := s.listQueue(id)
	if len(items) == 0 {
		bank.mu.Unlock()
		return nil
	}
	areas := map[string][]note{}
	original := map[string]string{}
	for _, area := range ordered() {
		original[area] = s.readArea(id, area)
		areas[area] = parseNotes(original[area])
	}
	bank.mu.Unlock()
	notes := notesInput(areas)
	probe := request(consolidateSystem, "", consolidateTool, consolidateSchema, consolidateOut)
	room := (s.window(ctx, probe)-consolidateOut-framingTokens-llm.EstimateText(consolidateSystem))*4*9/10 - len(notes)
	var used []queued
	var fresh strings.Builder
	for _, item := range items {
		line := fmt.Sprintf("- %s: %s (%s)\n", item.Area, item.Rule, item.Friction)
		if len(used) > 0 && fresh.Len()+len(line) > room {
			break
		}
		fresh.WriteString(line)
		used = append(used, item)
	}
	room -= fresh.Len()
	if room < 0 {
		bank.mu.Lock()
		defer bank.mu.Unlock()
		return s.failed(id, project, errors.New(i18n.T("The decision notes are too long for the light model's context window.")))
	}
	var input strings.Builder
	if room >= minDocsBytes {
		if docs := projectDocs(project, min(room, docsBytes)); docs != "" {
			input.WriteString("<docs>\n" + docs + "\n</docs>\n")
		}
	}
	input.WriteString("<notes>\n" + notes + "</notes>\n<new>\n" + fresh.String() + "</new>")

	req := request(consolidateSystem, input.String(), consolidateTool, consolidateSchema, consolidateOut)
	args, final, err := s.call(ctx, req)
	if err != nil {
		if ctx.Err() != nil {
			return err
		}
		bank.mu.Lock()
		defer bank.mu.Unlock()
		return s.failed(id, project, err)
	}
	changes, err := list(args, "changes")
	if err != nil {
		bank.mu.Lock()
		defer bank.mu.Unlock()
		return s.failed(id, project, errNoCall)
	}
	next, err := apply(areas, changes, redact.Snapshot())
	if err != nil {
		bank.mu.Lock()
		defer bank.mu.Unlock()
		return s.failed(id, project, err)
	}
	bank.mu.Lock()
	defer bank.mu.Unlock()
	for _, area := range ordered() {
		if s.readArea(id, area) != original[area] {
			s.schedule(id, project, minBackoff)
			return nil
		}
	}
	for _, area := range ordered() {
		if err := s.writeArea(id, area, renderNotes(next[area])); err != nil {
			return s.failed(id, project, err)
		}
	}
	consume(used)
	now := s.now().UTC()
	if err := s.writeState(id, state{Project: project, Consolidated: &now, Model: final.Model}); err != nil {
		return err
	}
	s.broadcast(project)
	if len(used) < len(items) {
		s.schedule(id, project, minBackoff)
	}
	return nil
}

// failed records a failed pass, which the Learning page shows, and returns
// err.
func (s *Service) failed(id, project string, err error) error {
	st := s.readState(id)
	st.Project = project
	st.Error = &failure{At: s.now().UTC(), Message: failureText(err), Failures: st.failures() + 1}
	s.writeState(id, st)
	s.broadcast(project)
	return err
}

// failureText is what the Learning page shows for a failed pass.
func failureText(err error) string {
	switch {
	case errors.Is(err, errNoCall):
		return i18n.T("The light model did not answer in the expected form.")
	case errors.Is(err, errTruncated):
		return i18n.T("The light model's answer was cut off.")
	}
	return err.Error()
}

// ordered is the areas in the order a bank lists and injects them.
func ordered() []string { return protocol.DecisionAreas }

// areaLetter prefixes a note's id: A1, P2, S3.
func areaLetter(area string) string { return strings.ToUpper(area[:1]) }

// notesInput lists every note with its id, area and topic, "(none)" for an
// empty bank.
func notesInput(areas map[string][]note) string {
	var b strings.Builder
	for _, area := range ordered() {
		for i, n := range areas[area] {
			topic := area
			if n.Section != "" {
				topic += " / " + n.Section
			}
			fmt.Fprintf(&b, "%s%d [%s] %s\n", areaLetter(area), i+1, topic, n.Text)
		}
	}
	if b.Len() == 0 {
		return "(none)\n"
	}
	return b.String()
}

// apply carries out a consolidation's changes. Removes and replaces name
// notes by their ids in the listing the model saw; adds join their topic's
// section, or open it at the end. An add that would take a file past
// MaxLines, repeats a note, or lacks a valid area is dropped; a change
// naming an unknown id is ignored. Removing every note of a bank that held
// some is refused.
func apply(areas map[string][]note, changes []map[string]any, values redact.Values) (map[string][]note, error) {
	next := map[string][]note{}
	removed := map[string]map[int]bool{}
	replaced := map[string]map[int]string{}
	var adds []map[string]any
	before := 0
	for _, area := range ordered() {
		removed[area], replaced[area] = map[int]bool{}, map[int]string{}
		before += len(areas[area])
	}
	for _, change := range changes {
		action := strings.ToLower(strings.TrimSpace(field(change, "action", "op")))
		if action == "add" {
			adds = append(adds, change)
			continue
		}
		area, index, ok := noteID(field(change, "id"), areas)
		if !ok {
			continue
		}
		switch action {
		case "remove", "delete":
			removed[area][index] = true
		case "replace", "update":
			if text := clean(field(change, "text", "rule"), values); text != "" {
				replaced[area][index] = text
			}
		}
	}
	after := 0
	for _, area := range ordered() {
		for i, n := range areas[area] {
			if removed[area][i] {
				continue
			}
			if text, ok := replaced[area][i]; ok {
				n.Text = text
			}
			next[area] = append(next[area], n)
		}
		after += len(next[area])
	}
	for _, change := range adds {
		area := areaOf(field(change, "area"))
		text := clean(field(change, "text", "rule"), values)
		if area == "" || text == "" || holds(next[area], text) {
			continue
		}
		section := clean(field(change, "section", "topic"), values)
		section = strings.TrimSpace(strings.TrimLeft(section, "#"))
		if utf8.RuneCountInString(section) > maxSection {
			section = strings.TrimSpace(string([]rune(section)[:maxSection]))
		}
		if section == "" {
			section = "General"
		}
		candidate := insert(next[area], note{Section: section, Text: text})
		if lineCount(renderNotes(candidate)) > MaxLines {
			continue
		}
		next[area] = candidate
		after++
	}
	if before > 0 && after == 0 {
		return nil, errors.New(i18n.T("The light model tried to remove every decision; nothing was changed."))
	}
	for _, area := range ordered() {
		for lineCount(renderNotes(next[area])) > MaxLines && len(next[area]) > 0 {
			next[area] = next[area][:len(next[area])-1]
		}
	}
	return next, nil
}

// noteID reads an id such as "P2" against the listing.
func noteID(id string, areas map[string][]note) (string, int, bool) {
	id = strings.ToUpper(strings.TrimSpace(id))
	if len(id) < 2 {
		return "", 0, false
	}
	index, err := strconv.Atoi(id[1:])
	if err != nil {
		return "", 0, false
	}
	for _, area := range ordered() {
		if areaLetter(area) == id[:1] && index >= 1 && index <= len(areas[area]) {
			return area, index - 1, true
		}
	}
	return "", 0, false
}

func holds(notes []note, text string) bool {
	for _, n := range notes {
		if strings.EqualFold(n.Text, text) {
			return true
		}
	}
	return false
}

// insert puts n after the last note of its section, or opens the section
// at the end.
func insert(notes []note, n note) []note {
	at := -1
	for i, existing := range notes {
		if strings.EqualFold(existing.Section, n.Section) {
			at, n.Section = i, existing.Section
		}
	}
	out := make([]note, 0, len(notes)+1)
	if at < 0 {
		return append(append(out, notes...), n)
	}
	out = append(out, notes[:at+1]...)
	out = append(out, n)
	return append(out, notes[at+1:]...)
}

// clean makes model text one redacted line, cut to maxRuleRunes.
func clean(text string, values redact.Values) string {
	if _, unsafe := scan.Strict(text); unsafe {
		return ""
	}
	text = oneLine(text)
	if utf8.RuneCountInString(text) > maxRuleRunes {
		text = strings.TrimSpace(string([]rune(text)[:maxRuleRunes]))
	}
	if text == "" {
		return ""
	}
	return values.Strict(text)
}

// projectDocs is the project's AGENTS.md and CLAUDE.md, which the notes
// must not repeat, cut at a line within max bytes.
func projectDocs(project string, max int) string {
	root, err := os.OpenRoot(project)
	if err != nil {
		return ""
	}
	defer root.Close()
	var parts []string
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		f, err := root.Open(name)
		if err != nil {
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(f, int64(max)+1))
		closeErr := f.Close()
		if readErr != nil || closeErr != nil {
			continue
		}
		if text := strings.TrimSpace(redact.Strict(cutLines(string(data), max))); text != "" {
			parts = append(parts, "--- "+name+" ---\n"+text)
		}
	}
	return cutLines(strings.Join(parts, "\n\n"), max)
}

// cutLines cuts text at the last line break within max bytes.
func cutLines(text string, max int) string {
	if len(text) <= max {
		return text
	}
	text = text[:max]
	if at := strings.LastIndexByte(text, '\n'); at > 0 {
		text = text[:at]
	}
	return strings.ToValidUTF8(text, "")
}
