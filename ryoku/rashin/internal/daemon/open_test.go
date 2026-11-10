package daemon

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

// Asked to show a session by its id alone, the daemon says which tab it is in:
// an agent session used to open as an empty ask.
func TestAnOpenRequestForASessionNamesItsTab(t *testing.T) {
	isolateDaemon(t)
	d := New(Deps{Asker: fakeAsker{runner: &fakeRunner{}}, Harnesses: &fakeHarnesses{runner: &fakeRunner{}}, Version: "test"})
	ask, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAsk})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent})
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		asked protocol.UIOpenParams
		tab   string
	}{
		{protocol.UIOpenParams{Session: ask.ID}, protocol.TabAsk},
		{protocol.UIOpenParams{Session: agent.ID}, protocol.TabAgent},
		{protocol.UIOpenParams{Session: agent.ID, Tab: protocol.TabProjects}, protocol.TabProjects},
		{protocol.UIOpenParams{Session: "gone"}, ""},
		{protocol.UIOpenParams{Tab: protocol.TabIndex}, protocol.TabIndex},
	} {
		if got := d.openTarget(check.asked).Tab; got != check.tab {
			t.Errorf("%+v opens in tab %q, want %q", check.asked, got, check.tab)
		}
	}
}
