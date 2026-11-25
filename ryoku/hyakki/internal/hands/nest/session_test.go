package nest

import (
	"os"
	"strings"
	"testing"
)

const nestedProviderName = "niri"

func sessionHandles(t *testing.T) (socket, instance string) {
	t.Helper()
	data, err := os.ReadFile("../../../../wm/oniwindow/testdata/session-handles.txt")
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(data))
	if len(fields) != 2 {
		t.Fatalf("session fixture must name both handles: %q", fields)
	}
	return fields[0], fields[1]
}
