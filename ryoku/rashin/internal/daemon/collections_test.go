package daemon

import (
	"context"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestTheiaSourceCannotBypassCollectionsGate(t *testing.T) {
	isolateDaemon(t)
	d := New(Deps{Asker: fakeAsker{runner: &fakeRunner{}}, Agents: &fakeAgents{runner: &fakeRunner{}}, Version: "test"})
	if d.feature("collections") {
		t.Fatal("collections must start off")
	}
	for _, source := range []string{"theia", "", "app"} {
		_, err := d.collectionsAdd(context.Background(), &connection{}, protocol.CollectionAddParams{Kind: "answer", Text: "saved text", Source: source})
		if err == nil {
			t.Fatalf("source %q bypassed the feature gate", source)
		}
	}
}
