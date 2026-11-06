package host

import (
	"strings"
	"testing"

	"ryoku-cli/internal/profile"
)

func TestRefuseChangedPlan(t *testing.T) {
	t.Parallel()
	plan := &profile.Plan{Digest: strings.Repeat("a", 64)}
	if err := refuseChangedPlan(plan, plan.Digest); err != nil {
		t.Fatalf("matching digest rejected: %v", err)
	}
	if err := refuseChangedPlan(plan, strings.Repeat("b", 64)); err == nil || !strings.Contains(err.Error(), "plan changed") {
		t.Fatalf("changed plan error = %v", err)
	}
}

func TestRemoveProfilePackagesRefusesResolverExpansion(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{results: []Result{{Output: "nmap\nseclists\nshared-runtime\n"}}}
	app, _, _ := testApp(runner, nil)
	plan := &profile.RemovalPlan{Packages: []profile.RemovalPackage{{Name: "nmap"}, {Name: "seclists"}}}
	if err := app.removeProfilePackages(plan); err == nil || !strings.Contains(err.Error(), "shared-runtime") {
		t.Fatalf("unsafe removal error = %v", err)
	}
	if len(runner.commands) != 1 || !strings.Contains(argv(runner.commands[0]), "-Rsp") {
		t.Fatalf("commands = %#v", runner.commands)
	}
}

func TestRemoveProfilePackagesUsesPacmanSafeSetThenRemove(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{results: []Result{{Output: "nmap\nseclists\n"}, {}}}
	app, _, _ := testApp(runner, nil)
	plan := &profile.RemovalPlan{Packages: []profile.RemovalPackage{{Name: "nmap"}, {Name: "seclists"}}}
	if err := app.removeProfilePackages(plan); err != nil {
		t.Fatal(err)
	}
	if len(runner.commands) != 2 || !strings.Contains(argv(runner.commands[0]), "-Rsp") || !strings.Contains(argv(runner.commands[1]), "-Rns --noconfirm") {
		t.Fatalf("commands = %#v", runner.commands)
	}
}
