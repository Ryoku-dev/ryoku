package settings

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestPrivacyPartialActivityKeepsHealthySelectedClearAvailable(t *testing.T) {
	b := privacyBlock{loaded: true, result: protocol.OniActivityResult{
		Counts: map[string]int{protocol.OniActivityReminders: 2},
		Errors: map[string]string{protocol.OniActivityRoutines: "The routine record cannot be read."},
	}}
	if !b.canClear(protocol.OniActivityReminders) {
		t.Fatal("unrelated routine damage disabled healthy reminder removal")
	}
	for _, unavailable := range []string{protocol.OniActivityRoutines, protocol.OniActivityHandsLog, protocol.OniActivityAll} {
		if b.canClear(unavailable) {
			t.Fatalf("failed, unknown or aggregate owner %s was clearable", unavailable)
		}
	}
	b.busy = true
	if b.canClear(protocol.OniActivityReminders) {
		t.Fatal("a second clear was available while busy")
	}
	b.busy, b.err = false, "The daemon is disconnected."
	if b.canClear(protocol.OniActivityReminders) {
		t.Fatal("a transport failure left destructive controls available")
	}
}
