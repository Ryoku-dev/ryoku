package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name  string
		reply string
		want  string
	}{
		{
			name:  "running",
			reply: "VM status: running\n(qemu) ",
			want:  "running",
		},
		{
			name:  "paused prelaunch",
			reply: "VM status: paused (prelaunch)\n(qemu) ",
			want:  "paused",
		},
		{
			name:  "suspended",
			reply: "VM status: suspended\r\n(qemu) ",
			want:  "suspended",
		},
		{
			name:  "device not found",
			reply: "Device not found\n(qemu) ",
			want:  "unknown",
		},
		{
			name:  "empty",
			reply: "",
			want:  "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseStatus(tt.reply); got != tt.want {
				t.Fatalf("parseStatus(%q) = %q, want %q", tt.reply, got, tt.want)
			}
		})
	}
}

func TestParseBalloon(t *testing.T) {
	tests := []struct {
		name   string
		reply  string
		wantMB int
		wantOK bool
	}{
		{
			name:   "actual size",
			reply:  "balloon: actual=4096\n(qemu) ",
			wantMB: 4096,
			wantOK: true,
		},
		{
			name:   "actual with max mem",
			reply:  "balloon: actual=2048, max_mem=4096",
			wantMB: 2048,
			wantOK: true,
		},
		{
			name:   "device inactive",
			reply:  "Device 'balloon' has not been activated",
			wantMB: 0,
			wantOK: false,
		},
		{
			name:   "empty",
			reply:  "",
			wantMB: 0,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMB, gotOK := parseBalloon(tt.reply)
			if gotMB != tt.wantMB || gotOK != tt.wantOK {
				t.Fatalf("parseBalloon(%q) = (%d, %v), want (%d, %v)", tt.reply, gotMB, gotOK, tt.wantMB, tt.wantOK)
			}
		})
	}
}

func TestParseCpus(t *testing.T) {
	tests := []struct {
		name  string
		reply string
		want  []int
	}{
		{
			name:  "single cpu",
			reply: "* CPU #0: pc=0x000000000009fb00 (halted) thread_id=1234\n(qemu) ",
			want:  []int{1234},
		},
		{
			name: "three cpus in order",
			reply: "* CPU #0: pc=0x123 thread_id=101\n" +
				"  CPU #1: pc=0x456 thread_id=102\n" +
				"  CPU #2: pc=0x789 thread_id=103\n(qemu) ",
			want: []int{101, 102, 103},
		},
		{
			name:  "no matches",
			reply: "error: no cpus\n(qemu) ",
			want:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCpus(tt.reply)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseCpus(%q) = %#v, want %#v", tt.reply, got, tt.want)
			}
		})
	}
}

func TestReadPid(t *testing.T) {
	dir := t.TempDir()

	validFile := filepath.Join(dir, "valid.pid")
	if err := os.WriteFile(validFile, []byte("  54321 \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	abcFile := filepath.Join(dir, "abc.pid")
	if err := os.WriteFile(abcFile, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}

	emptyFile := filepath.Join(dir, "empty.pid")
	if err := os.WriteFile(emptyFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		wantPID int
		wantErr bool
	}{
		{
			name:    "valid pid with whitespace",
			path:    validFile,
			wantPID: 54321,
			wantErr: false,
		},
		{
			name:    "missing file",
			path:    filepath.Join(dir, "missing.pid"),
			wantPID: 0,
			wantErr: true,
		},
		{
			name:    "non numeric content",
			path:    abcFile,
			wantPID: 0,
			wantErr: true,
		},
		{
			name:    "empty file",
			path:    emptyFile,
			wantPID: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPID, err := readPid(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("readPid(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
			if gotPID != tt.wantPID {
				t.Fatalf("readPid(%q) = %d, want %d", tt.path, gotPID, tt.wantPID)
			}
		})
	}
}
