package keyring

import (
	"os"
	"testing"

	"ryoku-cli/internal/testenv"
)

// TestMain runs the package's tests off this machine (see testenv.Run). The
// opt-in E2E brings its own dbus-run-session and keyring daemon, so it keeps them.
func TestMain(m *testing.M) {
	if os.Getenv("RYOKU_KEYRING_E2E") == "1" {
		os.Exit(m.Run())
	}
	os.Exit(testenv.Run(m))
}
