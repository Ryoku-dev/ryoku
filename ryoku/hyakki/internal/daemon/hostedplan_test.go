package daemon

import (
	"encoding/json"
	"testing"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// A planned route on Ryoku Ember spends the user's credits, so only the app
// may send to it; a route on this computer or a free model may come from
// anyone the gate lets in.
func TestHostedPlanStillNeedsTheApp(t *testing.T) {
	hosted := llm.Plan{Provider: protocol.ProviderEmber, Model: "rashin-fast", Paid: true}
	local := llm.Plan{Provider: protocol.ProviderOllamaLocal, Model: "eyes:latest", Local: true, Vision: true}
	if e := protocol.AsError(hostedNeedsApp(&connection{}, hosted.Route())); e == nil || e.Code != protocol.CodeForbidden {
		t.Fatalf("a hosted plan from the socket got %v", e)
	}
	if err := hostedNeedsApp(&connection{app: true}, hosted.Route()); err != nil {
		t.Fatalf("the app was refused its hosted plan: %v", err)
	}
	if err := hostedNeedsApp(&connection{}, local.Route()); err != nil {
		t.Fatalf("a local plan was refused: %v", err)
	}
	// a glance that names the hosted route on the socket is refused at the gate
	d := &Daemon{}
	raw, _ := json.Marshal(protocol.GlanceParams{Mode: "ask", Text: "look", Model: hosted.Route()})
	if e := protocol.AsError(d.mainGate(&connection{}, protocol.MGlance, raw)); e == nil || e.Code != protocol.CodeForbidden {
		t.Fatalf("a hosted glance from the socket got %v", e)
	}
	if err := d.mainGate(&connection{app: true}, protocol.MGlance, raw); err != nil {
		t.Fatalf("the app's hosted glance was refused: %v", err)
	}
}
