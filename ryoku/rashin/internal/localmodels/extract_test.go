package localmodels

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

type tarPart struct {
	name, link string
	kind       byte
	data       string
}

func fixtureArchive(t *testing.T, parts []tarPart) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, part := range parts {
		h := &tar.Header{Name: part.name, Typeflag: part.kind, Linkname: part.link, Mode: 0o755, Size: int64(len(part.data))}
		if part.kind != tar.TypeReg {
			h.Size = 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if part.data != "" {
			if _, err := tw.Write([]byte(part.data)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "runtime.tgz")
	if err := os.WriteFile(archive, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return archive
}

func TestExtractTarGzAllowsInTreeLibraryLink(t *testing.T) {
	archive := fixtureArchive(t, []tarPart{{name: "llama-b12521/", kind: tar.TypeDir},
		{name: "llama-b12521/libggml.so.1.0.0", kind: tar.TypeReg, data: "library"},
		{name: "llama-b12521/libggml.so.1", kind: tar.TypeSymlink, link: "libggml.so.1.0.0"}})
	root := filepath.Join(t.TempDir(), "install")
	if err := extractTarGz(archive, root, "llama-b12521"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "libggml.so.1"))
	if err != nil || string(data) != "library" {
		t.Fatalf("linked library = %q, %v", data, err)
	}
}

func TestExtractTarGzRejectsEscapeAndForeignTop(t *testing.T) {
	for _, part := range []tarPart{
		{name: "llama-b12521/../../outside", kind: tar.TypeReg, data: "unsafe"},
		{name: "llama-b12521/lib.so", kind: tar.TypeSymlink, link: "../outside"},
		{name: "another-root/lib.so", kind: tar.TypeReg, data: "unsafe"},
		{name: "/absolute/lib.so", kind: tar.TypeReg, data: "unsafe"},
	} {
		t.Run(part.name, func(t *testing.T) {
			archive := fixtureArchive(t, []tarPart{part})
			root := filepath.Join(t.TempDir(), "install")
			if err := extractTarGz(archive, root, "llama-b12521"); err == nil {
				t.Fatal("unsafe archive accepted")
			}
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatalf("failed extraction left files: %v", err)
			}
		})
	}
}
