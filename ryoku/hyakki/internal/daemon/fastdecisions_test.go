package daemon

import (
	"net"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func TestLinkedAccountDecisionAuthorityCannotBeClaimedOnTheMainSocket(t *testing.T) {
	isolateDaemon(t)
	d := New(Deps{Version: "decision-authority-wire-test"})
	for _, clientKind := range []string{protocol.ClientCLI, protocol.ClientApp} {
		t.Run(clientKind, func(t *testing.T) {
			server, client := net.Pipe()
			c := newConnection(d, server)
			stopped := make(chan struct{})
			go func() { c.serve(); close(stopped) }()
			defer func() {
				client.Close()
				c.close()
				select {
				case <-stopped:
				case <-time.After(time.Second):
					t.Error("connection did not close")
				}
			}()
			if err := client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			encoder, decoder := protocol.NewEncoder(client), protocol.NewDecoder(client)
			var sequence int64
			call := func(method string, params any) protocol.Frame {
				t.Helper()
				sequence++
				id := sequence
				if err := encoder.Encode(protocol.Frame{ID: &id, Method: method, Params: mustMarshal(t, params)}); err != nil {
					t.Fatal(err)
				}
				response, err := decoder.Next()
				if err != nil {
					t.Fatal(err)
				}
				if response.ID == nil || *response.ID != id {
					t.Fatalf("wrong response identity for %s", method)
				}
				return response
			}
			hello := call(protocol.MHello, protocol.HelloParams{Client: clientKind, Version: protocol.Version})
			if hello.Error != nil {
				t.Fatalf("hello failed: %v", hello.Error)
			}
			for _, route := range []string{protocol.DecisionRules, protocol.DecisionLocal, protocol.DecisionAuto, protocol.DecisionEmber} {
				response := call(protocol.MFastDecide, map[string]any{"route": route, "state": "selected fixture", "questions": map[string]any{"proceed": map[string]string{"type": "noul"}}})
				if response.Error == nil || response.Error.Code != protocol.CodeForbidden {
					t.Fatalf("unverified %s client reached %s decision authority: %v", clientKind, route, response.Error)
				}
			}
			// looking for the local decision server is the app's alone too
			if response := call(protocol.MFastDecideLocal, protocol.Empty{}); response.Error == nil || response.Error.Code != protocol.CodeForbidden {
				t.Fatalf("unverified %s client looked for the local decision server: %v", clientKind, response.Error)
			}
		})
	}
}
