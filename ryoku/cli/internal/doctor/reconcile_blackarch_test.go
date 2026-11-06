package doctor

import (
	"errors"
	"testing"

	"ryoku-cli/internal/profile"
)

func blackArchReceipt() profile.SystemReceipt {
	var receipt profile.SystemReceipt
	receipt.Repos = []profile.RepoClaim{{ID: "blackarch", EnabledByProfile: true}}
	return receipt
}

func TestPacmanConfigHasRepo(t *testing.T) {
	contents := []byte("[options]\n# [blackarch]\n [blackarch] \nInclude = /etc/pacman.d/blackarch-mirrorlist\n")
	if !pacmanConfigHasRepo(contents, "blackarch") {
		t.Fatal("active [blackarch] section was not found")
	}
	if pacmanConfigHasRepo([]byte("# [blackarch]\n"), "blackarch") {
		t.Fatal("commented [blackarch] section was treated as active")
	}
}

func TestReconcileBlackArchRepo(t *testing.T) {
	failure := errors.New("failed")
	for _, tc := range []struct {
		name        string
		pacman      bool
		receipts    []profile.SystemReceipt
		receiptsErr error
		conf        string
		confErr     error
		checkOnly   bool
		enableErr   error
		wantStatus  recStatus
		wantEnable  bool
	}{
		{name: "not pacman", wantStatus: recNote},
		{name: "no profile claim", pacman: true, wantStatus: recOK},
		{name: "receipt read failure", pacman: true, receiptsErr: failure, wantStatus: recWarn},
		{name: "repository present", pacman: true, receipts: []profile.SystemReceipt{blackArchReceipt()}, conf: "[blackarch]\n", wantStatus: recOK},
		{name: "pacman config unreadable", pacman: true, receipts: []profile.SystemReceipt{blackArchReceipt()}, confErr: failure, wantStatus: recWarn},
		{name: "check reports missing repository", pacman: true, receipts: []profile.SystemReceipt{blackArchReceipt()}, conf: "[core]\n", checkOnly: true, wantStatus: recWouldFix},
		{name: "repair restores repository", pacman: true, receipts: []profile.SystemReceipt{blackArchReceipt()}, conf: "[core]\n", wantStatus: recFixed, wantEnable: true},
		{name: "repair failure", pacman: true, receipts: []profile.SystemReceipt{blackArchReceipt()}, conf: "[core]\n", enableErr: failure, wantStatus: recWarn, wantEnable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldHasPacman := hasPacman
			oldReceipts := readBlackArchReceipts
			oldConf := readBlackArchPacmanConf
			oldEnable := enableBlackArchRepo
			t.Cleanup(func() {
				hasPacman = oldHasPacman
				readBlackArchReceipts = oldReceipts
				readBlackArchPacmanConf = oldConf
				enableBlackArchRepo = oldEnable
			})

			hasPacman = func() bool { return tc.pacman }
			readBlackArchReceipts = func() ([]profile.SystemReceipt, error) {
				return tc.receipts, tc.receiptsErr
			}
			readBlackArchPacmanConf = func() ([]byte, error) {
				return []byte(tc.conf), tc.confErr
			}
			enabled := false
			enableBlackArchRepo = func() error {
				enabled = true
				return tc.enableErr
			}

			got := reconcileBlackArchRepo(tc.checkOnly)
			if got.status != tc.wantStatus {
				t.Fatalf("status = %v, want %v (%+v)", got.status, tc.wantStatus, got)
			}
			if enabled != tc.wantEnable {
				t.Fatalf("enable called = %v, want %v", enabled, tc.wantEnable)
			}
			if (got.status == recWarn || got.status == recWouldFix) && got.remedy == "" {
				t.Fatalf("finding has no fix: %+v", got)
			}
		})
	}
}
