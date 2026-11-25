package daemon

import (
	"context"
	"encoding/json"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestMemberConfigurationRequiresVerifiedAppNotCLIOrOwnerClaims(t *testing.T) {
	isolateDaemon(t)
	for _, method := range []string{protocol.MTeamsMemberConfigGet, protocol.MTeamsMemberConfigSave, protocol.MTeamsMemberConfigHistory, protocol.MTeamsMemberConfigRestore} {
		t.Run(method, func(t *testing.T) {
			crew := &gateTeams{}
			d := &Daemon{deps: Deps{Teams: crew}}
			raw := json.RawMessage(`{"team":"team-1","member":"member-1","expectedRevision":1,"revision":1,"yaml":"version: 1","app":true,"owner":true}`)
			for _, kind := range []string{protocol.ClientCLI, protocol.ClientApp, protocol.ClientHub, protocol.ClientBar} {
				if _, err := d.dispatch(context.Background(), &connection{kind: kind}, method, raw); !forbidden(err) {
					t.Fatalf("unverified %s accessed config: %v", kind, err)
				}
			}
			if len(crew.handled) != 0 {
				t.Fatal("refused requests reached the configuration controller")
			}
			if _, err := d.dispatch(context.Background(), &connection{app: true}, method, json.RawMessage(`{}`)); err != nil || len(crew.handled) != 1 {
				t.Fatal("verified app cannot access configuration")
			}
			if err := d.hostGate(protocol.SessionInfo{ID: "agent-owner", Team: "team-1"}, method, raw); !forbidden(err) {
				t.Fatal("owning agent model bypassed app authority")
			}
			if err := agentGate(method); !forbidden(err) {
				t.Fatal("agent socket bypassed app authority")
			}
		})
	}
}
