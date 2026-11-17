package apprelease

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCheckoutReleaseAvoidsLegacyPackageOwnership(t *testing.T) {
	for _, scenario := range []string{"legacy", "foreign", "bad checksum"} {
		t.Run(scenario, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local/share"))
			var payload bytes.Buffer
			tarball := tar.NewWriter(&payload)
			body := []byte("#!/bin/sh\nprintf 'native fleet dashboard\\n'\n")
			if err := tarball.WriteHeader(&tar.Header{Name: "usr/bin/ryoport", Mode: 0o755, Size: int64(len(body))}); err != nil {
				t.Fatal(err)
			}
			if _, err := tarball.Write(body); err != nil {
				t.Fatal(err)
			}
			if err := tarball.Close(); err != nil {
				t.Fatal(err)
			}
			f := &fixture{repo: Ryoport.Repo, tag: "v1.0.0", pkgAsset: "ryoport-1.0.0-1-x86_64.pkg.tar.zst", pkgBytes: payload.Bytes()}
			if scenario == "bad checksum" {
				f.shaOverride = strings.Repeat("0", 64)
			}
			server := f.server()
			defer server.Close()
			var rec recorder
			c := newClient(t, server, f, "", pkgMeta{Name: "ryoport", Version: "1.0.0-1", Arch: wantArch}, nil, &rec)
			c.app = Ryoport
			c.listFiles = func(context.Context, string) ([]string, error) {
				return []string{"/usr/bin/ryoport", "/usr/bin/ryovm"}, nil
			}
			bin := filepath.Join(home, "fakebin")
			if err := os.MkdirAll(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			reader := "/usr/bin/bsdtar"
			if _, err := os.Stat(reader); err != nil {
				reader = "/usr/bin/tar"
			}
			if err := os.Symlink(reader, filepath.Join(bin, "bsdtar")); err != nil {
				t.Fatal(err)
			}
			owner := "ryoku-desktop"
			if scenario == "foreign" {
				owner = "other-package"
			}
			if err := os.WriteFile(filepath.Join(bin, "pacman"), []byte("#!/bin/sh\n[ \"$1\" = -Qoq ] || exit 2\nprintf '%s\\n' "+owner+"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			c.fileOwners = defaultFileOwners
			c.legacyInstall = func(ctx context.Context, path, version string) error {
				return installCheckoutRyoport(ctx, home, path, version)
			}
			st, err := c.Ensure(context.Background())
			if atomic.LoadInt32(&rec.installs) != 0 {
				t.Fatal("attempted a system package transaction")
			}
			if scenario != "legacy" {
				if err == nil || checkoutRyoportVersion(home) != "" {
					t.Fatalf("untrusted/conflicting package deployed: %+v, %v", st, err)
				}
				return
			}
			if err != nil || !st.Updated || checkoutRyoportVersion(home) != "1.0.0-1" {
				t.Fatalf("Ensure = %+v, %v", st, err)
			}
			for _, name := range checkoutCommands {
				out, err := exec.Command(filepath.Join(home, ".local/bin", name)).CombinedOutput()
				if err != nil || string(out) != "native fleet dashboard\n" {
					t.Fatalf("%s: %v: %s", name, err, out)
				}
			}
			desktop := filepath.Join(home, ".local/share/applications/ryovm.desktop")
			data, err := os.ReadFile(desktop)
			if err != nil || !strings.Contains(string(data), "Exec=ryoport\n") || strings.Contains(string(data), "qs -c") {
				t.Fatalf("launcher = %q, %v", data, err)
			}
			custom := filepath.Join(home, ".local/bin/ryossh")
			if err := os.Remove(custom); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(custom, []byte("custom"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := removeCheckoutRyoport(home, filepath.Join(home, ".local/share")); err != nil {
				t.Fatal(err)
			}
			if checkoutRyoportVersion(home) != "" {
				t.Fatal("retirement left the user release installed")
			}
			if data, err := os.ReadFile(custom); err != nil || string(data) != "custom" {
				t.Fatal("retirement removed the user's replacement command")
			}
			if _, err := os.Stat(desktop); !os.IsNotExist(err) {
				t.Fatal("retirement left the old launcher override")
			}
		})
	}
}
