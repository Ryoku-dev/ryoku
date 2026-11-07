package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"
)

func stageShellBundle(ctx context.Context, stage string, cache *Cache, entry ProductEntry, manifest ProductManifest, local *localProductSource, downloadTotal int64) ([]ReceiptFile, error) {
	files := append([]ProductFile(nil), manifest.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Destination < files[j].Destination })
	bundlePath := filepath.Join(stage, shellBundleName)
	handle, err := os.OpenFile(bundlePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	gzipWriter := gzip.NewWriter(io.MultiWriter(handle, hash))
	gzipWriter.Header.ModTime = time.Unix(0, 0)
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	members := make([]ReceiptFile, 0, len(files)+1)
	downloadDone := int64(0)
	reportProgressNow(ctx, "download", 0, downloadTotal)
	for index, file := range files {
		if !file.Install {
			continue
		}
		var data []byte
		if local != nil {
			data, err = readLocalProductFile(local.root, file.Source, file.Size, file.SHA256)
		} else {
			data, err = fetchProductFileProgress(ctx, cache, path.Join(entry.Path, file.Source), file.Size, file.SHA256, downloadDone, downloadTotal)
		}
		if err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			_ = handle.Close()
			return nil, fmt.Errorf("%s/%s: files[%d] %s: %w", manifest.Category, entry.ID, index, file.Source, err)
		}
		downloadDone += int64(len(data))
		reportProgress(ctx, "download", downloadDone, downloadTotal)
		mode := int64(0o644)
		if file.Mode == "0755" {
			mode = 0o755
		}
		header := &tar.Header{
			Name: file.Destination, Mode: mode, Size: file.Size,
			Typeflag: tar.TypeReg, ModTime: time.Unix(0, 0),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			_ = handle.Close()
			return nil, err
		}
		if _, err := tarWriter.Write(data); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			_ = handle.Close()
			return nil, err
		}
		members = append(members, ReceiptFile{
			Source: file.Source, Destination: file.Destination, SHA256: file.SHA256,
			Mode: file.Mode, Size: file.Size, ArchiveMember: true,
		})
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		_ = handle.Close()
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		_ = handle.Close()
		return nil, err
	}
	if err := handle.Close(); err != nil {
		return nil, err
	}
	if err := os.Chmod(bundlePath, 0o644); err != nil {
		return nil, err
	}
	info, err := os.Stat(bundlePath)
	if err != nil {
		return nil, err
	}
	bundle := ReceiptFile{
		Source: shellBundleName, Destination: shellBundleName,
		SHA256: fmt.Sprintf("%x", hash.Sum(nil)), Mode: "0644", Size: info.Size(),
	}
	reportProgressNow(ctx, "download", downloadDone, downloadTotal)
	reportProgressNow(ctx, "extract", int64(len(members)), int64(len(members)))
	return append([]ReceiptFile{bundle}, members...), nil
}

func receiptPhysicalFiles(receipt Receipt) []ReceiptFile {
	files := make([]ReceiptFile, 0, len(receipt.Files))
	for _, file := range receipt.Files {
		if !file.ArchiveMember {
			files = append(files, file)
		}
	}
	return files
}

func shellBundleMembers(receipt Receipt) map[string]ReceiptFile {
	members := map[string]ReceiptFile{}
	for _, file := range receipt.Files {
		if file.ArchiveMember {
			members[file.Destination] = file
		}
	}
	return members
}

func verifyShellBundle(bundlePath string, receipt Receipt, extractRoot string) error {
	bundle, ok := shellBundleReceiptFile(receipt)
	if !ok {
		return fmt.Errorf("%s/%s receipt has no bundle", receipt.Category, receipt.ID)
	}
	if err := verifyReceiptFile(bundlePath, bundle); err != nil {
		return err
	}
	handle, err := os.Open(bundlePath)
	if err != nil {
		return err
	}
	defer handle.Close()
	gzipReader, err := gzip.NewReader(handle)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	archive := tar.NewReader(gzipReader)
	expected := shellBundleMembers(receipt)
	seen := map[string]bool{}
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return fmt.Errorf("bundle member %q is not a regular file", header.Name)
		}
		if !validProductPath(header.Name) || path.Clean(header.Name) != header.Name {
			return fmt.Errorf("bundle member %q escapes the item", header.Name)
		}
		member, ok := expected[header.Name]
		if !ok || seen[header.Name] {
			return fmt.Errorf("unexpected bundle member %q", header.Name)
		}
		seen[header.Name] = true
		wantMode := int64(0o644)
		if member.Mode == "0755" {
			wantMode = 0o755
		}
		if header.Mode&0o777 != wantMode || header.Size != member.Size {
			return fmt.Errorf("bundle member %q metadata mismatch", header.Name)
		}
		target := ""
		if extractRoot != "" {
			target = filepath.Join(extractRoot, filepath.FromSlash(header.Name))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
		}
		hash := sha256.New()
		var writer io.Writer = hash
		var output *os.File
		if target != "" {
			output, err = os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(wantMode))
			if err != nil {
				return err
			}
			writer = io.MultiWriter(hash, output)
		}
		written, copyErr := io.CopyN(writer, archive, member.Size)
		if output != nil {
			closeErr := output.Close()
			if copyErr == nil {
				copyErr = closeErr
			}
			if copyErr == nil {
				copyErr = os.Chmod(target, os.FileMode(wantMode))
			}
		}
		if copyErr != nil || written != member.Size || fmt.Sprintf("%x", hash.Sum(nil)) != member.SHA256 {
			return fmt.Errorf("bundle member %q failed verification", header.Name)
		}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("bundle has %d of %d receipt members", len(seen), len(expected))
	}
	return nil
}

func shellBundleReceiptFile(receipt Receipt) (ReceiptFile, bool) {
	for _, file := range receipt.Files {
		if !file.ArchiveMember && file.Destination == shellBundleName {
			return file, true
		}
	}
	return ReceiptFile{}, false
}

func verifyReceiptFile(filePath string, file ReceiptFile) error {
	info, err := os.Lstat(filePath)
	if err != nil {
		return err
	}
	// Modes are not compared: installs from before receipts recorded them, and
	// a user's chmod, must still verify.
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() != file.Size {
		return fmt.Errorf("installed file %s does not match receipt", filePath)
	}
	handle, err := os.Open(filePath)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, handle)
	closeErr := handle.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != file.SHA256 {
		return fmt.Errorf("installed file %s failed verification", filePath)
	}
	return nil
}
