package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunVoxtypeMutationWaitsForOperationLock(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))

	entered := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- withVoxtypeMutationLock(func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered

	secondStarted := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondStarted)
		secondDone <- runVoxtype([]string{"set", "not-json"})
	}()
	<-secondStarted
	select {
	case err := <-secondDone:
		close(release)
		<-firstDone
		t.Fatalf("second mutation bypassed the operation lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err == nil || !strings.Contains(err.Error(), "bad JSON") {
		t.Fatalf("second mutation did not proceed after unlock: %v", err)
	}
}

func TestSnapshotRestoresPackagedVoxtypeUnitState(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	bin := filepath.Join(root, "bin")
	state := filepath.Join(root, "service-state")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFakeVoxtypeSystemdHost(t, bin, state, false)
	if err := os.WriteFile(filepath.Join(bin, "voxtype"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(state, "known"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "known", "voxtype"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, stateName := range []string{"enabled", "active"} {
		path := filepath.Join(state, stateName, "voxtype")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	snapshot, err := snapshotVoxtypeSetState()
	if err != nil {
		t.Fatal(err)
	}
	var previous *voxtypeUserServiceSnapshot
	for i := range snapshot.services {
		if snapshot.services[i].name == "voxtype" {
			previous = &snapshot.services[i]
			break
		}
	}
	if previous == nil || !previous.tracked || !previous.servicePresent ||
		!previous.enabled || !previous.active || previous.definitionPresent {
		t.Fatalf("packaged service state was not captured: %#v", previous)
	}

	if err := os.MkdirAll(filepath.Dir(voxtypeUnitPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(voxtypeUnitPath(), []byte("generated user override"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := userService("disable", "--now", "voxtype"); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.restore(); err != nil {
		t.Fatal(err)
	}
	if fileExists(voxtypeUnitPath()) {
		t.Fatal("rollback left the newly generated user unit in place")
	}
	for _, stateName := range []string{"enabled", "active"} {
		if !fileExists(filepath.Join(state, stateName, "voxtype")) {
			t.Errorf("rollback did not restore packaged Voxtype %s state", stateName)
		}
	}
}
