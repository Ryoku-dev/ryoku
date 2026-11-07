package doctor

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestReconcileShellRequirements(t *testing.T) {
	original := shellRequirementCommand
	t.Cleanup(func() { shellRequirementCommand = original })

	var calls []string
	responses := [][]byte{
		[]byte(`{"missing":[{"selection":"barStyle","id":"python"}]}`),
	}
	shellRequirementCommand = func(name string, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		if len(responses) == 0 {
			return nil, fmt.Errorf("unexpected command")
		}
		output := responses[0]
		responses = responses[1:]
		return output, nil
	}
	if result := reconcileShellRequirements(true); result.status != recWouldFix {
		t.Fatalf("check status = %v: %s", result.status, result.detail)
	}
	if want := []string{"ryostore internal shells-require --json"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("check calls = %#v, want %#v", calls, want)
	}

	calls = nil
	responses = [][]byte{
		[]byte(`{"missing":[{"selection":"barStyle","id":"python"}]}`),
		[]byte(`{"installed":[{"category":"subshells","id":"python"}],"fellBack":[]}`),
		{},
	}
	if result := reconcileShellRequirements(false); result.status != recFixed {
		t.Fatalf("repair status = %v: %s", result.status, result.detail)
	}
	want := []string{
		"ryostore internal shells-require --json",
		"ryostore internal shells-require --install --fallback --json",
		"ryoku-shell shells commit",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("repair calls = %#v, want %#v", calls, want)
	}
}

func TestReconcileShellRequirementsHealthy(t *testing.T) {
	original := shellRequirementCommand
	t.Cleanup(func() { shellRequirementCommand = original })

	calls := 0
	shellRequirementCommand = func(name string, args ...string) ([]byte, error) {
		calls++
		return []byte(`{"missing":[]}`), nil
	}
	if result := reconcileShellRequirements(false); result.status != recOK {
		t.Fatalf("status = %v: %s", result.status, result.detail)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}
