// Package backendtest is the contract every memory engine keeps: the
// built-in store, Hindsight and Honcho run the same suite, the external ones
// against fakes of their servers.
package backendtest

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/protocol"
)

// Canary is the key the fakes ask for. It must never show in an error or a
// status.
const Canary = "sk-test-CANARY4f1c9e2a7b"

// Open makes a fresh, empty engine for one test. A fake server behind it
// asks for Canary as its key and learns each sentence the user says in a
// transcript as one fact, at once.
type Open func(t *testing.T) backend.Backend

const project = protocol.MemoryProjectPrefix + "3f2a9c1d0b7e"

// Run checks an engine against the contract.
func Run(t *testing.T, open Open) {
	t.Run("RememberListRecall", func(t *testing.T) { rememberListRecall(t, open(t)) })
	t.Run("Edit", func(t *testing.T) { edit(t, open(t)) })
	t.Run("Forget", func(t *testing.T) { forget(t, open(t)) })
	t.Run("ObserveForgetSession", func(t *testing.T) { observe(t, open(t)) })
	t.Run("ForgetScope", func(t *testing.T) { forgetScope(t, open(t)) })
	t.Run("Groups", func(t *testing.T) { groups(t, open(t)) })
	t.Run("CardStatus", func(t *testing.T) { cardStatus(t, open(t)) })
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return c
}

// no fails the test on an error, and on one that shows the key.
func no(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		clean(t, err.Error())
		t.Fatal(err)
	}
}

func clean(t *testing.T, text string) {
	t.Helper()
	if strings.Contains(text, Canary) {
		t.Fatalf("the key shows in %q", text)
	}
}

func remember(t *testing.T, b backend.Backend, scope string, texts ...string) []protocol.Memory {
	t.Helper()
	facts := make([]protocol.Memory, len(texts))
	for i, text := range texts {
		facts[i] = protocol.Memory{Scope: scope, Text: text, Kind: protocol.MemoryDurable, Source: protocol.MemorySource{Kind: protocol.MemoryFromYou}}
	}
	kept, err := b.Remember(ctx(t), facts)
	no(t, err)
	if len(kept) != len(texts) {
		t.Fatalf("Remember returned %d facts for %d", len(kept), len(texts))
	}
	for i, k := range kept {
		if k.ID == "" || k.Text != texts[i] || k.Scope != scope {
			t.Fatalf("Remember returned %+v for %q in %s", k, texts[i], scope)
		}
	}
	return kept
}

func list(t *testing.T, b backend.Backend, scope string) []protocol.Memory {
	t.Helper()
	all, total, err := b.List(ctx(t), scope, 0, 100)
	no(t, err)
	if total != len(all) {
		t.Fatalf("List(%q) gave %d facts and a total of %d", scope, len(all), total)
	}
	return all
}

func texts(facts []protocol.Memory) []string {
	out := make([]string, len(facts))
	for i, f := range facts {
		out[i] = f.Text
	}
	slices.Sort(out)
	return out
}

func ids(facts []protocol.Memory) []string {
	out := make([]string, len(facts))
	for i, f := range facts {
		out[i] = f.ID
	}
	return out
}

func rememberListRecall(t *testing.T, b backend.Backend) {
	you := remember(t, b, protocol.MemoryYou, "The user prefers the Helix editor.", "The user lives in Lisbon.")
	remember(t, b, project, "The project builds with meson.")
	remember(t, b, protocol.MemoryTeamPrefix+"team_research", "The team writes reports in Markdown.")

	if got := texts(list(t, b, protocol.MemoryYou)); !slices.Equal(got, texts(you)) {
		t.Fatalf("List(you) = %q", got)
	}
	if got := list(t, b, ""); len(got) != 4 {
		t.Fatalf("List(all) = %d facts, want 4", len(got))
	}
	if got := texts(list(t, b, protocol.MemoryProjects)); !slices.Equal(got, []string{"The project builds with meson."}) {
		t.Fatalf("List(projects) = %q", got)
	}
	if got := texts(list(t, b, protocol.MemoryTeams)); !slices.Equal(got, []string{"The team writes reports in Markdown."}) {
		t.Fatalf("List(teams) = %q", got)
	}
	first, total, err := b.List(ctx(t), protocol.MemoryYou, 0, 1)
	no(t, err)
	second, _, err := b.List(ctx(t), protocol.MemoryYou, 1, 1)
	no(t, err)
	if total != 2 || len(first) != 1 || len(second) != 1 || first[0].ID == second[0].ID {
		t.Fatalf("paging gave %v then %v of %d", ids(first), ids(second), total)
	}
	none, total, err := b.List(ctx(t), protocol.MemoryYou, 0, 0)
	no(t, err)
	if len(none) != 0 || total != 2 {
		t.Fatalf("a count gave %d facts and a total of %d", len(none), total)
	}

	found, err := b.Recall(ctx(t), backend.Query{Text: "Which editor does the user prefer?", Scopes: []string{protocol.MemoryYou}, Limit: 5})
	no(t, err)
	if len(found) == 0 || found[0].Text != "The user prefers the Helix editor." {
		t.Fatalf("Recall found %q", texts(found))
	}
	for _, f := range found {
		if f.Scope != protocol.MemoryYou {
			t.Fatalf("Recall in you found %+v", f)
		}
	}
}

func edit(t *testing.T, b backend.Backend) {
	kept := remember(t, b, protocol.MemoryYou, "The user drinks green tea.")
	edited, err := b.Edit(ctx(t), kept[0].ID, "The user drinks black coffee.")
	no(t, err)
	if edited.Text != "The user drinks black coffee." || edited.ID == "" {
		t.Fatalf("Edit returned %+v", edited)
	}
	if got := texts(list(t, b, protocol.MemoryYou)); !slices.Equal(got, []string{"The user drinks black coffee."}) {
		t.Fatalf("after Edit, List(you) = %q", got)
	}
}

func forget(t *testing.T, b backend.Backend) {
	kept := remember(t, b, protocol.MemoryYou, "The user plays the cello.", "The user owns a bicycle.")
	no(t, b.Forget(ctx(t), []string{kept[0].ID}))
	if got := texts(list(t, b, protocol.MemoryYou)); !slices.Equal(got, []string{"The user owns a bicycle."}) {
		t.Fatalf("after Forget, List(you) = %q", got)
	}
	found, err := b.Recall(ctx(t), backend.Query{Text: "cello", Scopes: []string{protocol.MemoryYou}, Limit: 5})
	no(t, err)
	if slices.Contains(texts(found), "The user plays the cello.") {
		t.Fatalf("Recall still finds a forgotten fact")
	}
	// forgetting what is gone is not an error
	no(t, b.Forget(ctx(t), []string{kept[0].ID}))
}

func observe(t *testing.T, b backend.Backend) {
	if !b.Caps().Learns {
		t.Skip("the engine does not learn from Observe; Rashin's learner does")
	}
	remember(t, b, protocol.MemoryYou, "The user likes short answers.")
	transcript := backend.Transcript{
		Session: "s-0123", Kind: protocol.MemoryFromAsk, Title: "Editors", Scope: protocol.MemoryYou,
		Turns: []backend.Turn{{Seq: 3, At: time.Date(2026, 11, 11, 22, 0, 0, 0, time.UTC), User: "I write Go every day.", Reply: "Noted."}},
	}
	no(t, b.Observe(ctx(t), transcript))
	learned := list(t, b, protocol.MemoryYou)
	if !slices.Contains(texts(learned), "I write Go every day.") {
		t.Fatalf("after Observe, List(you) = %q", texts(learned))
	}
	var fromSession []string
	for _, f := range learned {
		if f.Text == "I write Go every day." && f.Source.Session != "s-0123" {
			t.Fatalf("a learned fact names session %q", f.Source.Session)
		}
		if f.Source.Session == "s-0123" {
			fromSession = append(fromSession, f.ID)
		}
	}
	no(t, b.Forget(ctx(t), fromSession))
	if got := texts(list(t, b, protocol.MemoryYou)); !slices.Equal(got, []string{"The user likes short answers."}) {
		t.Fatalf("after forgetting what the conversation taught, List(you) = %q", got)
	}
}

// groups checks that projects and teams name every scope of their kind in
// recall and forgetting.
func groups(t *testing.T, b backend.Backend) {
	other := protocol.MemoryProjectPrefix + "9e8d7c6b5a41"
	team := protocol.MemoryTeamPrefix + "team_research"
	remember(t, b, protocol.MemoryYou, "The user builds keyboards.")
	remember(t, b, project, "The project builds with meson.")
	remember(t, b, other, "The other project builds with cargo.")
	remember(t, b, team, "The team builds weekly reports.")
	found, err := b.Recall(ctx(t), backend.Query{Text: "builds", Scopes: []string{protocol.MemoryProjects}, Limit: 10})
	no(t, err)
	if got := texts(found); !slices.Equal(got, []string{"The other project builds with cargo.", "The project builds with meson."}) {
		t.Fatalf("Recall in projects found %q", got)
	}
	no(t, b.ForgetScope(ctx(t), protocol.MemoryProjects))
	if got := texts(list(t, b, "")); !slices.Equal(got, []string{"The team builds weekly reports.", "The user builds keyboards."}) {
		t.Fatalf("after ForgetScope(projects), List = %q", got)
	}
	no(t, b.ForgetScope(ctx(t), protocol.MemoryTeams))
	if got := texts(list(t, b, "")); !slices.Equal(got, []string{"The user builds keyboards."}) {
		t.Fatalf("after ForgetScope(teams), List = %q", got)
	}
}

func forgetScope(t *testing.T, b backend.Backend) {
	remember(t, b, protocol.MemoryYou, "The user is left-handed.")
	remember(t, b, project, "The project targets Linux.")
	no(t, b.ForgetScope(ctx(t), project))
	if got := list(t, b, protocol.MemoryProjects); len(got) != 0 {
		t.Fatalf("after ForgetScope(project), List(projects) = %q", texts(got))
	}
	if got := list(t, b, protocol.MemoryYou); len(got) != 1 {
		t.Fatalf("ForgetScope(project) touched you: %q", texts(got))
	}
	no(t, b.ForgetScope(ctx(t), ""))
	if got := list(t, b, ""); len(got) != 0 {
		t.Fatalf("after ForgetScope(all), List = %q", texts(got))
	}
}

func cardStatus(t *testing.T, b backend.Backend) {
	remember(t, b, protocol.MemoryYou, "The user is a violinist.")
	if b.Caps().Card {
		_, err := b.Card(ctx(t))
		no(t, err)
	}
	status := b.Status(ctx(t))
	clean(t, status.Detail)
	if status.State != protocol.EngineReady {
		t.Fatalf("Status = %+v", status)
	}
	if b.Busy() {
		t.Fatal("Busy with no call in flight")
	}
	no(t, b.Close())
}
