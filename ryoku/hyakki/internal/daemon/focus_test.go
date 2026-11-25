package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestFocusTokensReachOnlyVerifiedAppPeers(t *testing.T) {
	d := &Daemon{clients: make(map[*connection]struct{})}
	var unverified []*connection
	for _, kind := range []string{protocol.ClientApp, protocol.ClientBar, protocol.ClientCLI, protocol.ClientHub} {
		c := &connection{kind: kind, subscribed: true, outbound: make(chan []byte, 1)}
		d.clients[c] = struct{}{}
		unverified = append(unverified, c)
	}
	verified := &connection{kind: protocol.ClientApp, app: true, subscribed: true, outbound: make(chan []byte, 1)}
	d.clients[verified] = struct{}{}
	request := protocol.UIOpenParams{Tab: protocol.TabSettings, View: "features", ActivationToken: "private-one-shot-activation"}
	if !d.focusApps(request) {
		t.Fatal("the verified application did not receive its focus request")
	}
	for _, c := range unverified {
		select {
		case frame := <-c.outbound:
			t.Fatalf("unverified %s learned the activation request: %s", c.kind, frame)
		default:
		}
	}
	var event protocol.Event
	if err := json.Unmarshal(<-verified.outbound, &event); err != nil {
		t.Fatal(err)
	}
	var delivered protocol.UIOpenParams
	if err := json.Unmarshal(event.Data, &delivered); err != nil {
		t.Fatal(err)
	}
	if event.Event != protocol.EvtUIFocus || delivered != request {
		t.Fatalf("verified app received %#v, %s", delivered, event.Event)
	}
}

func TestActivationTokensCannotOverflowOrTruncateTheNativeWireString(t *testing.T) {
	d := &Daemon{}
	for _, token := range []string{strings.Repeat("a", 4097), "valid-prefix\x00hidden-suffix"} {
		raw, err := json.Marshal(protocol.UIOpenParams{Tab: protocol.TabSettings, ActivationToken: token})
		if err != nil {
			t.Fatal(err)
		}
		_, err = d.dispatch(context.Background(), &connection{}, protocol.MUIOpen, raw)
		var refused *protocol.Error
		if !errors.As(err, &refused) || refused.Code != protocol.CodeBadRequest {
			t.Fatalf("unsafe native token was accepted: %v", err)
		}
	}
	raw, err := json.Marshal(protocol.UIOpenParams{Tab: protocol.TabSettings, ActivationToken: strings.Repeat("a", 4096)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.dispatch(context.Background(), &connection{}, protocol.MUIOpen, raw); err != nil {
		t.Fatalf("a bounded native token was refused: %v", err)
	}
}
