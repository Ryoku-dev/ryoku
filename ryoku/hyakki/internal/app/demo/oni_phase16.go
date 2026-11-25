package demo

import (
	"encoding/json"
	"slices"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// FeatureOniPhase16 is no feature: it starts the demo as FeatureOni with
// notes, recall (vault included), rewrite, assignment mode and the focus
// guard on, for the Settings pages of Phase 16.
const FeatureOniPhase16 = "oni-phase16"

// the demo's selection a rewrite reads and the result it types back
const (
	demoRewriteSelection = "teh meeting moved to thursday, can you make it"
	demoRewriteResult    = "The meeting moved to Thursday. Can you make it?"
	demoRewriteCard      = feature.RewriteCardPrefix + "00000000000000e1"
	demoRewriteRecord    = feature.RewriteCardPrefix + "00000000000000e2"
)

// phase16Fixtures is what the demo's Phase 16 calls answer from: a rewrite
// card and its applied record, a guard armed or not, and the assignment
// views stepped through, all told from Epoch so every run answers the same.
// Nothing is read from or typed into any window: the demo has none.
type phase16Fixtures struct {
	rows        []protocol.OniAssignmentRow
	rewriteCard string
	applied     string
	guard       *protocol.OniFocusResult
	watching    bool
	view        int
}

// applyPhase16 turns the Phase 16 switches on for FeatureOniPhase16
func applyPhase16(settings *protocol.OniSettings) {
	settings.Voice, settings.Reflex = true, true
	settings.ReflexSettings.Notes = true
	settings.Recall = protocol.OniRecallSettings{Enabled: true, Vault: true}
	settings.Rewrite.Enabled = true
	settings.Assignment.Enabled = true
	settings.Feeds = protocol.OniFeedSettings{Guard: true, Words: []string{"reels", "shorts"}}
}

// the demo's recalled sources: titles and places only, never a passage
var demoRecallSources = []protocol.OniRecallSource{
	{Kind: protocol.OniRecallChat, Title: "Eigenvalues for the exam", From: protocol.OniImportClaude, At: Epoch.Add(-72 * time.Hour)},
	{Kind: protocol.OniRecallVault, Title: "Linear algebra notes", At: Epoch.Add(-240 * time.Hour)},
}

// the demo's solved views: the answer first, then the steps that lead to it
var demoAssignmentViews = []struct {
	answer string
	steps  []string
}{
	{"x = 4", []string{"Subtract 3 from both sides: 2x = 8.", "Divide both sides by 2.", "So x = 4."}},
	{"y = -2", []string{"Add 5 to both sides: 3y = -6.", "Divide both sides by 3.", "So y = -2."}},
}

// demoAssignmentDay is the demo's spend today: two solves, measured
func demoAssignmentDay() protocol.OniAssignmentDay {
	sp, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return protocol.OniAssignmentDay{
		Day: Epoch.Format(time.DateOnly), Attempts: 2, Tokens: 3120,
		AttemptsCap: sp.Assignment.AttemptsPerDay, TokensCap: sp.Assignment.TokensPerDay,
	}
}

// the demo's assignment ledger, newest first: lanes, routes and counts only
var demoAssignmentRows = []protocol.OniAssignmentRow{
	{At: Epoch.Add(-time.Minute), Lane: protocol.OniAssignmentLaneJudge, Route: "local/small", Input: 220, Output: 8, Outcome: "same"},
	{At: Epoch.Add(-time.Minute - 3*time.Second), Lane: protocol.OniAssignmentLaneChecker, Route: "local/small", Input: 900, Output: 420, Outcome: "done"},
	{At: Epoch.Add(-time.Minute - 5*time.Second), Lane: protocol.OniAssignmentLaneFast, Route: "local/small", Input: 900, Output: 40, Outcome: "done"},
}

// callOniPhase16 answers the Phase 16 calls as the daemon will: each
// decodes strictly and needs its feature, and only the app, which is all
// the demo serves, sends them.
func (s *server) callOniPhase16(method string, raw json.RawMessage) (any, error) {
	params, err := feature.DecodeCall(method, raw)
	if err != nil {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni call is invalid: %v.", err))
	}
	oni := protocol.EffectiveOni(s.data.snapshot.Settings)
	p16 := &s.data.oni.phase16
	switch p := params.(type) {
	case *protocol.OniRewriteParams:
		if err := demoRefusal(oni, feature.Rewrite); err != nil {
			return nil, err
		}
		return p16.rewrite(p, oni), nil
	case *protocol.OniRecallParams:
		if err := demoRefusal(oni, feature.Recall); err != nil {
			return nil, err
		}
		sources := slices.Clone(demoRecallSources)
		if !oni.Recall.Vault {
			sources = slices.DeleteFunc(sources, func(src protocol.OniRecallSource) bool { return src.Kind == protocol.OniRecallVault })
		}
		answer := i18n.T("You asked Claude how to find eigenvalues: solve det(A - λI) = 0 for λ.")
		if p.Action == protocol.OniRecallKnow {
			answer = i18n.T("You live in Lisbon and you are studying for a linear algebra exam.")
			sources = []protocol.OniRecallSource{{Kind: protocol.OniRecallFact, Title: i18n.T("Where you live")}}
		}
		return protocol.OniRecallResult{Status: protocol.OniRecallStatusAnswered, Answer: answer, Sources: sources, Route: "local/small", Destination: feature.EgressLocal}, nil
	case *protocol.OniRecallImportParams:
		if err := demoRefusal(oni, feature.Recall); err != nil {
			return nil, err
		}
		s.data.oni.phase17.counts[protocol.OniActivityRecallImports] += 3 * len(p.Paths)
		return protocol.OniRecallImportResult{Chats: 3 * len(p.Paths), Sources: []protocol.OniRecallImported{{From: protocol.OniImportClaude, Chats: 3 * len(p.Paths)}}, Root: "imports/chats"}, nil
	case *protocol.OniFocusParams:
		if err := demoRefusal(oni, feature.FeedGuard); err != nil {
			return nil, err
		}
		return p16.focus(p, oni), nil
	case *protocol.OniAssignmentParams:
		if err := demoRefusal(oni, feature.Assignment); err != nil {
			return nil, err
		}
		return p16.assignment(p.Action), nil
	}
	if err := demoRefusal(oni, feature.Assignment); err != nil {
		return nil, err
	}
	return protocol.OniAssignmentLogResult{Day: demoAssignmentDay(), Rows: slices.Clone(p16.rows)}, nil
}

// rewrite walks the demo's one selection through its card, the typed
// result and undo
func (f *phase16Fixtures) rewrite(p *protocol.OniRewriteParams, oni protocol.OniSettings) protocol.OniRewriteResult {
	unknown := protocol.OniRewriteResult{Status: protocol.OniRewriteRefused, Refused: protocol.OniRewriteRefusedUnknownID}
	switch p.Action {
	case protocol.OniRewriteStart:
		style := p.Style
		if style == "" {
			style = oni.Rewrite.Style
		}
		f.rewriteCard = demoRewriteCard
		return protocol.OniRewriteResult{Status: protocol.OniRewriteConsent, ID: f.rewriteCard, Style: style,
			Chars: len([]rune(demoRewriteSelection)), Destination: feature.EgressLocal, Text: demoRewriteSelection,
			ExpiresMs: feature.ConsentTTL.Milliseconds()}
	case protocol.OniRewriteApply:
		if p.ID != f.rewriteCard || f.rewriteCard == "" {
			return unknown
		}
		f.rewriteCard, f.applied = "", demoRewriteRecord
		sp, err := spec.Load()
		if err != nil {
			panic(err)
		}
		return protocol.OniRewriteResult{Status: protocol.OniRewriteApplied, ID: f.applied, Destination: feature.EgressLocal,
			Landed: len([]rune(demoRewriteResult)), UndoVerified: true, ExpiresMs: sp.Rewrite.Undo().Milliseconds()}
	case protocol.OniRewriteUndo:
		if p.ID != f.applied || f.applied == "" {
			return protocol.OniRewriteResult{Status: protocol.OniRewriteRefused, Refused: protocol.OniRewriteRefusedNoUndo}
		}
		f.applied = ""
		return protocol.OniRewriteResult{Status: protocol.OniRewriteUndone, ID: p.ID}
	}
	if p.ID != f.rewriteCard || f.rewriteCard == "" {
		return unknown
	}
	f.rewriteCard = ""
	return protocol.OniRewriteResult{Status: protocol.OniRewriteDeclined, ID: p.ID}
}

// focus arms, reports and lifts the demo's guard; the demo has no windows,
// so it closes and reopens nothing
func (f *phase16Fixtures) focus(p *protocol.OniFocusParams, oni protocol.OniSettings) protocol.OniFocusResult {
	switch p.Action {
	case protocol.OniFocusArm:
		words := slices.Clone(oni.Feeds.Words)
		if len(p.Words) > 0 {
			words, _ = protocol.CleanOniFeedWords(p.Words)
		}
		if len(words) == 0 {
			return protocol.OniFocusResult{Refused: protocol.OniFocusRefusedWords}
		}
		guard := protocol.OniFocusResult{Armed: true, Words: words, UntilApp: p.UntilApp}
		if p.Minutes > 0 {
			guard.LeftMs = (time.Duration(p.Minutes) * time.Minute).Milliseconds()
		}
		f.guard = &guard
	case protocol.OniFocusStop, protocol.OniFocusBack:
		f.guard = nil
	}
	if f.guard == nil {
		return protocol.OniFocusResult{}
	}
	return *f.guard
}

// assignment steps through the demo's solved views; it sends nothing, so
// every view comes from the cache
func (f *phase16Fixtures) assignment(action string) protocol.OniAssignmentResult {
	day := demoAssignmentDay()
	switch action {
	case protocol.OniAssignmentStart:
		f.watching, f.view = true, 0
		return protocol.OniAssignmentResult{Status: protocol.OniAssignmentWatching, Day: day}
	case protocol.OniAssignmentStop, protocol.OniAssignmentDone:
		f.watching, f.view = false, 0
		return protocol.OniAssignmentResult{Status: protocol.OniAssignmentStopped, Day: day}
	case protocol.OniAssignmentNext:
		if f.watching && f.view < len(demoAssignmentViews) {
			f.view++
		}
	case protocol.OniAssignmentBack:
		if f.watching && f.view > 1 {
			f.view--
		}
	}
	if !f.watching {
		return protocol.OniAssignmentResult{Status: protocol.OniAssignmentIdle, Day: day}
	}
	if f.view == 0 {
		return protocol.OniAssignmentResult{Status: protocol.OniAssignmentWatching, Day: day}
	}
	v := demoAssignmentViews[f.view-1]
	return protocol.OniAssignmentResult{Status: protocol.OniAssignmentAnswered, Answer: v.answer, Steps: slices.Clone(v.steps), Step: f.view, Cached: true, Day: day}
}
