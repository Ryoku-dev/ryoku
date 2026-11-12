package securitykey

import (
	"os"
	"testing"

	"ryoku-cli/internal/testenv"
)

// TestMain runs the package's tests off this machine (see testenv.Run).
func TestMain(m *testing.M) {
	os.Exit(testenv.Run(m))
}
