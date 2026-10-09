package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCommandStub(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestStopHostUserServiceExitHandling(t *testing.T) {
	tests := []struct {
		name    string
		exit    string
		wantErr bool
	}{
		{name: "service absent", exit: "4"},
		{name: "stop failed", exit: "1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := t.TempDir()
			writeCommandStub(t, bin, "ryoku-host", "exit "+tt.exit)
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

			err := stopHostUserService("ryoku-qylock-unlock-guard-test.service")
			if (err != nil) != tt.wantErr {
				t.Fatalf("stopHostUserService() error = %v, want error %v", err, tt.wantErr)
			}
		})
	}
}
