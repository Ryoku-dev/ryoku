package main

import (
	"archive/tar"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellInstallStoresOnlyAVerifiedBundle(t *testing.T) {
	f := newShellFixture(t)
	shell := testShellManifest()
	shell.Lifecycle.Activate = "bin/activate"
	files := append(defaultShellFiles(), shellTestFile{path: "bin/activate", body: "#!/bin/sh\nexit 0\n", exec: true})
	entry := f.addCatalogueItem("barstyles", "demo", shell, files, nil)
	if err := installProduct(context.Background(), f.cache(), "barstyles", entry); err != nil {
		t.Fatal(err)
	}

	destination, _, _ := productDestination("barstyles", "demo")
	listing, err := os.ReadDir(destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing) != 1 || listing[0].Name() != shellBundleName {
		t.Fatalf("installed tree = %v, want only %s", listing, shellBundleName)
	}
	receipt, err := readReceipt("barstyles", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Shell == nil || receipt.Shell.Lifecycle.Activate != "bin/activate" || receipt.Name != entry.Name {
		t.Fatalf("receipt shell metadata = %#v", receipt)
	}
	members := shellBundleMembers(receipt)
	if len(members) != len(files) || members["bin/activate"].Mode != "0755" {
		t.Fatalf("receipt members = %#v", members)
	}

	extract := filepath.Join(t.TempDir(), "unpacked")
	if err := os.MkdirAll(extract, 0o755); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(destination, shellBundleName)
	if err := verifyShellBundle(bundle, receipt, extract); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		raw, err := os.ReadFile(filepath.Join(extract, filepath.FromSlash(file.path)))
		if err != nil || string(raw) != file.body {
			t.Fatalf("unpacked %s = %q, %v", file.path, raw, err)
		}
	}
	if info, err := os.Stat(filepath.Join(extract, "bin", "activate")); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("unpacked script mode = %v, %v", info, err)
	}

	tampered := receipt
	tampered.Files = append([]ReceiptFile(nil), receipt.Files...)
	for index := range tampered.Files {
		if tampered.Files[index].ArchiveMember && tampered.Files[index].Destination == "qml/Scene.qml" {
			tampered.Files[index].SHA256 = strings.Repeat("0", 64)
		}
	}
	if err := verifyShellBundle(bundle, tampered, ""); err == nil || !strings.Contains(err.Error(), "failed verification") {
		t.Fatalf("tampered member hash verified: %v", err)
	}

	missing := receipt
	missing.Files = append(append([]ReceiptFile(nil), receipt.Files...), ReceiptFile{
		Source: "qml/Extra.qml", Destination: "qml/Extra.qml", SHA256: strings.Repeat("1", 64),
		Mode: "0644", Size: 1, ArchiveMember: true,
	})
	if err := verifyShellBundle(bundle, missing, ""); err == nil {
		t.Fatal("bundle missing a receipt member verified")
	}
}

func TestShellBundleRejectsLinksEscapesAndStrangers(t *testing.T) {
	dir := t.TempDir()
	body := "x"
	member := ReceiptFile{Source: "qml/a.qml", Destination: "qml/a.qml", SHA256: testSHA([]byte(body)), Mode: "0644", Size: 1, ArchiveMember: true}
	cases := []struct {
		name   string
		header *tar.Header
		body   string
		want   string
	}{
		{"link", &tar.Header{Name: "qml/a.qml", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0o644}, "", "not a regular file"},
		{"escape", &tar.Header{Name: "../a.qml", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1}, body, "escapes"},
		{"stranger", &tar.Header{Name: "qml/b.qml", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1}, body, "unexpected"},
		{"mode", &tar.Header{Name: "qml/a.qml", Typeflag: tar.TypeReg, Mode: 0o755, Size: 1}, body, "metadata"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle := filepath.Join(dir, tc.name+".tar.gz")
			row := writeRawBundle(t, bundle, []*tar.Header{tc.header}, []string{tc.body})
			receipt := Receipt{Category: "barstyles", ID: "demo", Version: "1.0.0", Files: []ReceiptFile{row, member}}
			extract := filepath.Join(dir, tc.name)
			if err := os.MkdirAll(extract, 0o755); err != nil {
				t.Fatal(err)
			}
			err := verifyShellBundle(bundle, receipt, extract)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("verifyShellBundle = %v, want %q", err, tc.want)
			}
			if _, err := os.Lstat(filepath.Join(dir, "a.qml")); !os.IsNotExist(err) {
				t.Fatal("an escaping member was written outside the unpack root")
			}
		})
	}
}
