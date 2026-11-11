package teams

import (
	"path/filepath"
	"slices"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestATeamsAccessIsCheckedAndGivenToItsSandbox(t *testing.T) {
	h := newHarness(t, 0)
	shared := t.TempDir()
	for _, bad := range []string{"/", paths.Home(), "relative/folder", filepath.Join(shared, "missing"), filepath.Join(paths.TeamsHome(), "someone-else")} {
		refused(t, h, protocol.MTeamsCreate, protocol.TeamCreateParams{
			Name: "Bad", Folder: t.TempDir(), Members: []protocol.Member{{Name: "Ana", Lead: true, Harness: "fake"}},
			Access: protocol.TeamAccess{Writable: []string{bad}},
		}, protocol.CodeBadRequest)
	}
	team := h.newTeam(t, "Builder", func(p *protocol.TeamCreateParams) {
		p.Access = protocol.TeamAccess{Writable: []string{shared, shared}, Hidden: []string{"~/.aws"}}
	})
	if !slices.Equal(team.Access.Writable, []string{shared}) || !slices.Equal(team.Access.Hidden, []string{"~/.aws"}) {
		t.Fatalf("the team keeps the access %+v", team.Access)
	}
	given := h.s.Access(protocol.SessionInfo{Team: team.ID})
	if !slices.Equal(given.Writable, []string{shared}) || !slices.Equal(given.Hidden, []string{filepath.Join(paths.Home(), ".aws")}) {
		t.Fatalf("the sandbox is given %+v, want full paths", given)
	}
	if other := h.s.Access(protocol.SessionInfo{}); other.Writable != nil || other.Hidden != nil {
		t.Fatalf("a session of no team is given %+v", other)
	}
}
