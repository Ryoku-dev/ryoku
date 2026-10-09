package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ryoku-i18n"
)

type sourceAsset struct {
	name     string
	url      string
	sha256   string
	format   string
	rawName  string
	fontPath func(string) bool
	cursors  map[string]string
}

// Arch packages the whole material-design-icons commit, whose generated archive
// is several gigabytes. The three fonts are fetched directly from that same
// immutable commit so a source install does not download the unrelated assets.
var sourceAssets = []sourceAsset{
	{
		name: "Material Symbols Outlined", format: "raw", rawName: "MaterialSymbolsOutlined[FILL,GRAD,opsz,wght].ttf",
		url:    "https://raw.githubusercontent.com/google/material-design-icons/bb04090f930e272697f2a1f0d7b352d92dfeee43/variablefont/MaterialSymbolsOutlined%5BFILL%2CGRAD%2Copsz%2Cwght%5D.ttf",
		sha256: "fb0d00bfa03507fe6712348604aea33741b41f97643e109ad4e582b1a15865b9",
	},
	{
		name: "Material Symbols Rounded", format: "raw", rawName: "MaterialSymbolsRounded[FILL,GRAD,opsz,wght].ttf",
		url:    "https://raw.githubusercontent.com/google/material-design-icons/bb04090f930e272697f2a1f0d7b352d92dfeee43/variablefont/MaterialSymbolsRounded%5BFILL%2CGRAD%2Copsz%2Cwght%5D.ttf",
		sha256: "d719f22fdee27e344b07e46e6fa8b50b1fce3cfcb03d4a84f03fafbf0812fc22",
	},
	{
		name: "Material Symbols Sharp", format: "raw", rawName: "MaterialSymbolsSharp[FILL,GRAD,opsz,wght].ttf",
		url:    "https://raw.githubusercontent.com/google/material-design-icons/bb04090f930e272697f2a1f0d7b352d92dfeee43/variablefont/MaterialSymbolsSharp%5BFILL%2CGRAD%2Copsz%2Cwght%5D.ttf",
		sha256: "e76edbb72b8cbca2380c507cd17ea5cb4023cf3e01a9dcfeddd670da8c495f37",
	},
	{
		name: "Readex Pro", format: "tar.gz",
		url:    "https://github.com/ThomasJockin/readexpro/archive/1a5aaa4c15edb043c37113a8cddf020235917050.tar.gz",
		sha256: "128848eba1ae8fe0454f279a367f5ef0e932949e131382f7b3e26b9373c9a7e4",
		fontPath: func(path string) bool {
			return strings.HasSuffix(path, "/fonts/variable/Readexpro[HEXP,wght].ttf")
		},
	},
	{
		name: "Rubik", format: "raw", rawName: "Rubik[wght].ttf",
		url:    "https://github.com/googlefonts/rubik/raw/e337a5f69a9bea30e58d05bd40184d79cc099628/fonts/variable/Rubik%5Bwght%5D.ttf",
		sha256: "1b3a7437ba2af80e465e773ed60c5036d1ba6ace492d89046dbcf18fb31e4e88",
	},
	{
		name: "Rubik Italic", format: "raw", rawName: "Rubik-Italic[wght].ttf",
		url:    "https://github.com/googlefonts/rubik/raw/e337a5f69a9bea30e58d05bd40184d79cc099628/fonts/variable/Rubik-Italic%5Bwght%5D.ttf",
		sha256: "08c6c4018a5ada8b517407b46897e46cf6ebb106853fbd3e89addb51d3b59c62",
	},
	{
		name: "Maple Mono NF", format: "zip",
		url:    "https://github.com/subframe7536/maple-font/releases/download/v7.9/MapleMono-NF.zip",
		sha256: "59098b87c895d871635d37680e88000ae2b2b25b55428195b228ec589e35fb89",
		fontPath: func(path string) bool {
			return strings.HasSuffix(strings.ToLower(path), ".ttf")
		},
	},
	{
		name: "Space Grotesk", format: "zip",
		url:    "https://github.com/floriankarsten/space-grotesk/releases/download/2.0.0/SpaceGrotesk-2.0.0.zip",
		sha256: "53b415577d4139248555300710bea0d268c7a5be67b93de53b716a9736cabffd",
		fontPath: func(path string) bool {
			return strings.Contains(path, "/otf/") && strings.HasSuffix(strings.ToLower(path), ".otf")
		},
	},
	{
		name: "Vimix cursors", format: "tar.gz",
		url:    "https://github.com/vinceliuice/Vimix-cursors/archive/2020-02-24.tar.gz",
		sha256: "69298d02264b5b15239c340f8fa899f91574c0eac49ad5745e8e588315423618",
		cursors: map[string]string{
			"dist":       "Vimix",
			"dist-white": "Vimix-white-cursors",
		},
	},
}

var sourceHTTPClient = &http.Client{Timeout: 3 * time.Minute}

func stepFonts(e *engine) error {
	if e.dry {
		for _, asset := range sourceAssets {
			e.say(i18n.Tf("DRYRUN: fetch and verify %s from %s", asset.name, asset.url))
		}
		e.say(i18n.T("DRYRUN: install fonts in /usr/local/share/fonts/ryoku and cursors in /usr/share/icons"))
		e.say(i18n.T("DRYRUN: fc-cache -f"))
		return nil
	}

	root, err := os.MkdirTemp("", "ryoku-fonts-")
	if err != nil {
		e.say(i18n.Tf("warning: could not prepare font downloads: %s", err))
		return nil
	}
	defer os.RemoveAll(root)

	fonts := make([]string, 0, 32)
	for i, asset := range sourceAssets {
		dir := filepath.Join(root, fmt.Sprintf("%02d", i))
		if err := fetchSourceAsset(asset, dir); err != nil {
			e.say(i18n.Tf("warning: could not install %s: %s", asset.name, err))
			continue
		}
		if len(asset.cursors) > 0 {
			installCursorAsset(e, asset, dir)
			continue
		}
		if asset.format == "raw" {
			fonts = append(fonts, filepath.Join(dir, asset.rawName))
			continue
		}
		_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr == nil && !entry.IsDir() && asset.fontPath(filepath.ToSlash(path)) {
				fonts = append(fonts, path)
			}
			return nil
		})
	}
	if len(fonts) > 0 {
		args := []string{"install", "-Dm644", "-t", "/usr/local/share/fonts/ryoku"}
		if err := e.sudo(append(args, fonts...)...); err != nil {
			e.say(i18n.Tf("warning: could not install downloaded fonts: %s", err))
		}
	}
	if err := e.sudo("fc-cache", "-f"); err != nil {
		e.say(i18n.Tf("warning: could not rebuild the font cache: %s", err))
	}
	return nil
}

func fetchSourceAsset(asset sourceAsset, dir string) error {
	resp, err := sourceHTTPClient.Get(asset.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download returned %s", resp.Status)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	download := filepath.Join(dir, "source.download")
	file, err := os.Create(download)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, 512<<20+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > 512<<20 {
		return fmt.Errorf("download exceeds 512 MiB")
	}
	if hex.EncodeToString(hash.Sum(nil)) != asset.sha256 {
		return fmt.Errorf("checksum mismatch")
	}
	switch asset.format {
	case "raw":
		return os.Rename(download, filepath.Join(dir, asset.rawName))
	case "zip":
		return unpackZip(download, dir)
	case "tar.gz":
		return unpackTarGzip(download, dir)
	default:
		return fmt.Errorf("unsupported archive format %q", asset.format)
	}
}

func archivePath(root, name string) (string, error) {
	rel := filepath.Clean(filepath.FromSlash(name))
	if rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return filepath.Join(root, rel), nil
}

func unpackZip(archive, root string) error {
	rd, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer rd.Close()
	for _, file := range rd.File {
		path, err := archivePath(root, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err == nil {
			_, err = io.Copy(dst, src)
		}
		dstErr := error(nil)
		if dst != nil {
			dstErr = dst.Close()
		}
		srcErr := src.Close()
		if err != nil {
			return err
		}
		if dstErr != nil {
			return dstErr
		}
		if srcErr != nil {
			return srcErr
		}
	}
	return nil
}

func unpackTarGzip(archive, root string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	rd := tar.NewReader(gz)
	for {
		hdr, err := rd.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		path, err := archivePath(root, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			dst, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(dst, rd)
			closeErr := dst.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			target := filepath.Clean(filepath.FromSlash(hdr.Linkname))
			resolved := filepath.Clean(filepath.Join(filepath.Dir(hdr.Name), target))
			if filepath.IsAbs(target) || resolved == ".." || strings.HasPrefix(resolved, ".."+string(filepath.Separator)) {
				return fmt.Errorf("unsafe archive symlink %q", hdr.Name)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(target, path); err != nil {
				return err
			}
		}
	}
}

func findDir(root, base string) string {
	found := ""
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && found == "" && entry.IsDir() && entry.Name() == base {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func installCursorAsset(e *engine, asset sourceAsset, root string) {
	for sourceName, destName := range asset.cursors {
		source := findDir(root, sourceName)
		if source == "" {
			e.say(i18n.Tf("warning: %s archive is missing %s", asset.name, sourceName))
			continue
		}
		dest := filepath.Join("/usr/share/icons", destName)
		if err := e.sudo("rm", "-rf", dest); err != nil {
			e.say(i18n.Tf("warning: could not replace cursor theme %s", destName))
			continue
		}
		if err := e.sudo("cp", "-a", source, dest); err != nil {
			e.say(i18n.Tf("warning: could not install cursor theme %s", destName))
		}
	}
}
