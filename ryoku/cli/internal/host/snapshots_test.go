package host

import "testing"

func TestSnapshotsCapabilityAndCommand(t *testing.T) {
	tests := []struct {
		name       string
		manager    string
		supported  bool
		exit       int
		wantOutput string
	}{
		{name: "pacman", manager: "pacman", supported: true, exit: ExitOK},
		{name: "xbps", manager: "xbps", supported: true, exit: ExitOK},
		{name: "unknown", manager: "other", supported: false, exit: ExitNotProvided, wantOutput: snapshotsUnavailableReason + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, stdout, stderr := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": tt.manager})
			supported, reason := app.Snapshots()
			if supported != tt.supported {
				t.Fatalf("Snapshots supported = %v, want %v", supported, tt.supported)
			}
			if tt.supported && reason != "" {
				t.Fatalf("supported host reason = %q, want empty", reason)
			}
			if !tt.supported && reason != snapshotsUnavailableReason {
				t.Fatalf("unsupported host reason = %q", reason)
			}
			if code := app.Execute([]string{"snapshots"}); code != tt.exit {
				t.Fatalf("snapshots exit = %d, want %d", code, tt.exit)
			}
			if got := stdout.String(); got != tt.wantOutput {
				t.Fatalf("snapshots stdout = %q, want %q", got, tt.wantOutput)
			}
			if stderr.Len() != 0 {
				t.Fatalf("snapshots stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestSnapshotsCommandRejectsArguments(t *testing.T) {
	app, stdout, stderr := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})
	if code := app.Execute([]string{"snapshots", "extra"}); code != ExitUsage {
		t.Fatalf("snapshots with arguments exit = %d, want %d", code, ExitUsage)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("usage-only rejection wrote stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
