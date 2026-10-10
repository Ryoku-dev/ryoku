package doctor

import "testing"

// planChannelPin is the #291 decision: an accidental stale release pin (older
// than the installed release, not matching the recorded intent) is repaired to
// the intended channel; a pin the user chose with `ryoku track v...` is left
// alone. Testing the pure planner catches the bug the reconciler exists for.
func TestPlanChannelPin(t *testing.T) {
	cases := []struct {
		name             string
		pin, inst, intnt string
		want             channelPinOutcome
		wantChan         string
	}{
		{
			name: "accidental stale pin repairs to stable when no intent",
			pin:  "v0.63.1-beta.19", inst: "v0.75.3-beta.20", intnt: "",
			want: channelPinStale, wantChan: "stable",
		},
		{
			name: "accidental stale pin repairs to the recorded intent",
			pin:  "v0.63.1-beta.19", inst: "v0.75.3-beta.20", intnt: "testing",
			want: channelPinStale, wantChan: "testing",
		},
		{
			name: "a deliberate ryoku track v... pin is left alone",
			pin:  "v0.63.1-beta.19", inst: "v0.75.3-beta.20", intnt: "v0.63.1-beta.19",
			want: channelPinDeliberate, wantChan: "v0.63.1-beta.19",
		},
		{
			name: "a stable pin is never stale",
			pin:  "stable", inst: "v0.75.3-beta.20", intnt: "",
			want: channelPinFine,
		},
		{
			name: "a pin equal to the installed release (a deliberate revert) is fine",
			pin:  "v0.63.1-beta.19", inst: "v0.63.1-beta.19", intnt: "",
			want: channelPinFine,
		},
		{
			name: "a pin ahead of the installed release is fine",
			pin:  "v0.75.3-beta.20", inst: "v0.63.1-beta.19", intnt: "",
			want: channelPinFine,
		},
		{
			name:  "an accidental unstable build pin repairs to unstable",
			pin:   "v0.94.1-beta.20.dev.11+gaaaaaaa",
			inst:  "v0.94.1-beta.20.dev.12+gbbbbbbb",
			intnt: "testing",
			want:  channelPinStale, wantChan: "testing",
		},
		{
			name:  "a deliberate unstable build pin is left alone",
			pin:   "v0.94.1-beta.20.dev.11+gaaaaaaa",
			inst:  "v0.94.1-beta.20.dev.12+gbbbbbbb",
			intnt: "v0.94.1-beta.20.dev.11+gaaaaaaa",
			want:  channelPinDeliberate, wantChan: "v0.94.1-beta.20.dev.11+gaaaaaaa",
		},
		{
			name:  "an unstable build pin equal to the installed build is fine",
			pin:   "v0.94.1-beta.20.dev.12+gbbbbbbb",
			inst:  "v0.94.1-beta.20.dev.12+gbbbbbbb",
			intnt: "testing",
			want:  channelPinFine,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, gotChan := planChannelPin(c.pin, c.inst, c.intnt)
			if got != c.want {
				t.Fatalf("outcome = %d, want %d", got, c.want)
			}
			if c.want != channelPinFine && gotChan != c.wantChan {
				t.Fatalf("channel = %q, want %q", gotChan, c.wantChan)
			}
		})
	}
}
