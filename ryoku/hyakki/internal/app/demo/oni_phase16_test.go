package demo

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// phase16Call makes one Phase 16 call on s and decodes its result into out
func phase16Call[T any](t *testing.T, s *server, method string, params any) T {
	t.Helper()
	raw, _ := json.Marshal(params)
	got, err := s.call(method, raw)
	if err != nil {
		t.Fatalf("%s %s: %v", method, raw, err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

var phase16Calls = []struct {
	method string
	params any
	id     string
}{
	{protocol.MOniRewrite, protocol.OniRewriteParams{Action: protocol.OniRewriteStart}, feature.Rewrite},
	{protocol.MOniRecall, protocol.OniRecallParams{Action: protocol.OniRecallKnow}, feature.Recall},
	{protocol.MOniRecallImport, protocol.OniRecallImportParams{Paths: []string{"/home/u/export.zip"}}, feature.Recall},
	{protocol.MOniFocus, protocol.OniFocusParams{Action: protocol.OniFocusStatus}, feature.FeedGuard},
	{protocol.MOniAssignment, protocol.OniAssignmentParams{Action: protocol.OniAssignmentStatus}, feature.Assignment},
	{protocol.MOniAssignmentLog, protocol.Empty{}, feature.Assignment},
}

func TestDemoPhase16CallsNeedTheirFeature(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOni)}
	for _, c := range phase16Calls {
		if refused := oniCallErr(t, s, c.method, c.params); refused.Code != protocol.CodeDisabled || refused.Reason != feature.ReasonFeatureOff {
			t.Errorf("%s with %s off: %+v", c.method, c.id, refused)
		}
	}
	s = &server{data: makeFixtures(FeatureOniPhase16)}
	s.data.snapshot.Settings.Oni.Master = false
	for _, c := range phase16Calls {
		if refused := oniCallErr(t, s, c.method, c.params); refused.Code != protocol.CodeDisabled || refused.Reason != feature.ReasonOniOff {
			t.Errorf("%s with Oni off: %+v", c.method, refused)
		}
	}
	s = &server{data: makeFixtures(FeatureOniPhase16)}
	for _, c := range phase16Calls {
		raw, _ := json.Marshal(c.params)
		if _, err := s.call(c.method, raw); err != nil {
			t.Errorf("%s with %s on: %v", c.method, c.id, err)
		}
	}
	for _, c := range [][2]string{
		{protocol.MOniRewrite, `{"action":"start","text":"x"}`},
		{protocol.MOniRecall, `{"action":"ask"}`},
		{protocol.MOniRecallImport, `{"paths":["relative.zip"]}`},
		{protocol.MOniFocus, `{"action":"arm","minutes":481}`},
		{protocol.MOniAssignment, `{"action":"next","style":"answers"}`},
		{protocol.MOniAssignmentLog, `{"day":"x"}`},
	} {
		var refused *protocol.Error
		if _, err := s.call(c[0], json.RawMessage(c[1])); !errors.As(err, &refused) || refused.Code != protocol.CodeBadRequest {
			t.Errorf("%s %s: %v", c[0], c[1], err)
		}
	}
}

// A rewrite shows the selection on its card, applies only that card, and
// undoes only what it applied.
func TestDemoRewriteWalksCardApplyAndUndo(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOniPhase16)}
	start := phase16Call[protocol.OniRewriteResult](t, s, protocol.MOniRewrite, protocol.OniRewriteParams{Action: protocol.OniRewriteStart})
	if start.Status != protocol.OniRewriteConsent || !strings.HasPrefix(start.ID, feature.RewriteCardPrefix) || start.Text == "" || start.Style != protocol.OniRewriteGrammar || start.Chars != len([]rune(start.Text)) {
		t.Fatalf("start = %+v", start)
	}
	if _, err := feature.EncodeFrame(feature.RewriteCard(start.ID, start.Destination, start.Route, "Rewrite?")); err != nil {
		t.Fatalf("the demo's card id does not make a valid card: %v", err)
	}
	if got := phase16Call[protocol.OniRewriteResult](t, s, protocol.MOniRewrite, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: "rewrite-other"}); got.Refused != protocol.OniRewriteRefusedUnknownID {
		t.Fatalf("applying another card = %+v", got)
	}
	applied := phase16Call[protocol.OniRewriteResult](t, s, protocol.MOniRewrite, protocol.OniRewriteParams{Action: protocol.OniRewriteApply, ID: start.ID})
	if applied.Status != protocol.OniRewriteApplied || applied.Landed == 0 || applied.Text != "" || applied.ExpiresMs <= 0 {
		t.Fatalf("apply = %+v", applied)
	}
	undone := phase16Call[protocol.OniRewriteResult](t, s, protocol.MOniRewrite, protocol.OniRewriteParams{Action: protocol.OniRewriteUndo, ID: applied.ID})
	if undone.Status != protocol.OniRewriteUndone {
		t.Fatalf("undo = %+v", undone)
	}
	if again := phase16Call[protocol.OniRewriteResult](t, s, protocol.MOniRewrite, protocol.OniRewriteParams{Action: protocol.OniRewriteUndo, ID: applied.ID}); again.Refused != protocol.OniRewriteRefusedNoUndo {
		t.Fatalf("a second undo = %+v", again)
	}
}

func TestDemoRecallKeepsTheVaultBehindItsSwitch(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOniPhase16)}
	ask := protocol.OniRecallParams{Action: protocol.OniRecallAsk, Text: "what did I ask Claude about eigenvalues"}
	hasVault := func(r protocol.OniRecallResult) bool {
		for _, src := range r.Sources {
			if src.Kind == protocol.OniRecallVault {
				return true
			}
		}
		return false
	}
	if got := phase16Call[protocol.OniRecallResult](t, s, protocol.MOniRecall, ask); got.Status != protocol.OniRecallStatusAnswered || !hasVault(got) {
		t.Fatalf("recall with the vault on = %+v", got)
	}
	s.data.snapshot.Settings.Oni.Recall.Vault = false
	if got := phase16Call[protocol.OniRecallResult](t, s, protocol.MOniRecall, ask); hasVault(got) || len(got.Sources) == 0 {
		t.Fatalf("recall with the vault off = %+v", got)
	}
}

func TestDemoFocusArmsAndLifts(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOniPhase16)}
	armed := phase16Call[protocol.OniFocusResult](t, s, protocol.MOniFocus, protocol.OniFocusParams{Action: protocol.OniFocusArm, Minutes: 20})
	if !armed.Armed || armed.LeftMs != 20*60*1000 || len(armed.Words) == 0 {
		t.Fatalf("arm = %+v", armed)
	}
	if got := phase16Call[protocol.OniFocusResult](t, s, protocol.MOniFocus, protocol.OniFocusParams{Action: protocol.OniFocusStatus}); !got.Armed {
		t.Fatalf("status after arm = %+v", got)
	}
	if got := phase16Call[protocol.OniFocusResult](t, s, protocol.MOniFocus, protocol.OniFocusParams{Action: protocol.OniFocusBack}); got.Armed || got.Reopen != 0 {
		t.Fatalf("back = %+v", got)
	}
}

func TestDemoAssignmentStepsThroughCachedViews(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOniPhase16)}
	if got := phase16Call[protocol.OniAssignmentResult](t, s, protocol.MOniAssignment, protocol.OniAssignmentParams{Action: protocol.OniAssignmentStart}); got.Status != protocol.OniAssignmentWatching || got.Day.AttemptsCap == 0 {
		t.Fatalf("start = %+v", got)
	}
	next := phase16Call[protocol.OniAssignmentResult](t, s, protocol.MOniAssignment, protocol.OniAssignmentParams{Action: protocol.OniAssignmentNext})
	if next.Status != protocol.OniAssignmentAnswered || next.Answer == "" || len(next.Steps) == 0 || next.Step != 1 || !next.Cached {
		t.Fatalf("next = %+v", next)
	}
	if got := phase16Call[protocol.OniAssignmentResult](t, s, protocol.MOniAssignment, protocol.OniAssignmentParams{Action: protocol.OniAssignmentDone}); got.Status != protocol.OniAssignmentStopped {
		t.Fatalf("done = %+v", got)
	}
	log := phase16Call[protocol.OniAssignmentLogResult](t, s, protocol.MOniAssignmentLog, protocol.Empty{})
	if len(log.Rows) == 0 || log.Day.Day == "" {
		t.Fatalf("log = %+v", log)
	}
	lanes := map[string]bool{}
	for _, row := range log.Rows {
		lanes[row.Lane] = true
	}
	if !lanes[protocol.OniAssignmentLaneFast] || !lanes[protocol.OniAssignmentLaneChecker] {
		t.Fatalf("the ledger shows lanes %v", lanes)
	}
}
