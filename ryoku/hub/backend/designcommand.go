package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func designSHA(s string) ([32]byte, error) {
	var sum [32]byte
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 || hex.EncodeToString(b) != s {
		return sum, fmt.Errorf("sha must be 64 lowercase hex characters")
	}
	copy(sum[:], b)
	return sum, nil
}

func designFetch(sha string, pending bool, issuer, keyFile string) (designResult, error) {
	var out designResult
	sum, err := designSHA(sha)
	if err != nil {
		return out, err
	}
	header := make(http.Header)
	path := "/v1/packs/" + sha + ".zip"
	if pending {
		path = "/v1/pending/" + sha + ".zip"
		if issuer != "owner" && issuer != "mika" {
			return out, fmt.Errorf("pending fetch requires --issuer owner|mika and its signing key")
		}
		seed, e := loadIssuerSeed(issuer, keyFile)
		if e != nil {
			return out, e
		}
		defer clear(seed)
		id := 1
		if issuer == "mika" {
			id = 2
		}
		seconds := strconv.FormatInt(time.Now().Unix(), 10)
		payload := []byte(http.MethodGet + "\n" + path + "\n" + seconds + "\n" + sha)
		header.Set("X-Ryoku-Upload", strconv.Itoa(id)+"."+seconds+"."+base64.StdEncoding.EncodeToString(signDomain(seed, "ryoku-upload-v1", payload)))
	}
	raw, status, err := teamSend(http.MethodGet, path, "", nil, header, designZipLimit)
	if err != nil {
		return out, err
	}
	if status != http.StatusOK {
		return out, fmt.Errorf("design fetch HTTP %d", status)
	}
	if sha256.Sum256(raw) != sum {
		return out, fmt.Errorf("design sha256 mismatch")
	}
	cache, err := teamCacheDir()
	if err != nil {
		return out, err
	}
	for _, dir := range []string{cache, filepath.Dir(cache)} {
		info, e := os.Lstat(dir)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return out, fmt.Errorf("unsafe team cache directory")
		}
	}
	root := filepath.Join(cache, "designs")
	if err = os.MkdirAll(root, 0700); err != nil {
		return out, err
	}
	if info, e := os.Lstat(root); e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return out, fmt.Errorf("unsafe design cache directory")
	}
	target := filepath.Join(root, sha)
	p, err := designReadZip(raw, target)
	if err != nil {
		return out, err
	}
	p, err = canonicalDesignPack(p, false)
	if err != nil {
		return out, err
	}
	spec, err := designValidate(p)
	if err != nil {
		return out, err
	}
	temp, err := os.MkdirTemp(root, ".design-")
	if err != nil {
		return out, err
	}
	defer os.RemoveAll(temp)
	for _, name := range p.names {
		if err = os.WriteFile(filepath.Join(temp, name), p.files[name], 0600); err != nil {
			return out, err
		}
	}
	normalized, err := json.Marshal(spec)
	if err != nil {
		return out, err
	}
	if err = os.WriteFile(filepath.Join(temp, "normalized.json"), normalized, 0600); err != nil {
		return out, err
	}
	if info, e := os.Lstat(target); e == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return out, fmt.Errorf("unsafe cached design")
		}
		// A concurrent fetch may already have installed the same content. Verify every byte before reuse.
		entries, e := os.ReadDir(target)
		if e != nil {
			return out, e
		}
		if len(entries) != len(p.names)+1 {
			return out, fmt.Errorf("cached design contains unexpected files")
		}
		for _, entry := range entries {
			if !entry.Type().IsRegular() {
				return out, fmt.Errorf("unsafe cached design entry")
			}
			expected, ok := p.files[entry.Name()]
			if entry.Name() == "normalized.json" {
				expected = normalized
				ok = true
			}
			if !ok {
				return out, fmt.Errorf("unexpected cached file")
			}
			actual, e := readBoundedFile(filepath.Join(target, entry.Name()), int64(len(expected)))
			if e != nil || sha256.Sum256(actual) != sha256.Sum256(expected) {
				return out, fmt.Errorf("cached design differs")
			}
		}
	} else if !os.IsNotExist(e) {
		return out, e
	} else if err = os.Rename(temp, target); err != nil {
		return out, err
	}
	return designResult{Dir: target, Design: spec}, nil
}

func designCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: design check|pack|wear|unwear|upload-pending|fetch|approve")
	}
	emit := func(v any) error { return json.NewEncoder(os.Stdout).Encode(v) }
	switch args[0] {
	case "check":
		if len(args) != 2 {
			return fmt.Errorf("usage: design check <dir|zip>")
		}
		r, err := designCheck(args[1])
		if e := emit(r); e != nil {
			return e
		}
		return err
	case "pack":
		if len(args) != 3 {
			return fmt.Errorf("usage: design pack <dir> <out.zip>")
		}
		st, err := os.Lstat(args[1])
		if err != nil {
			return err
		}
		if !st.IsDir() {
			return fmt.Errorf("pack input must be a directory")
		}
		p, err := designReadPack(args[1])
		if err != nil {
			return err
		}
		p, err = canonicalDesignPack(p, false)
		if err != nil {
			return err
		}
		if _, err = designValidate(p); err != nil {
			return err
		}
		raw, err := designZip(p)
		if err != nil {
			return err
		}
		if err = os.WriteFile(args[2], raw, 0600); err != nil {
			return err
		}
		return emit(map[string]bool{"ok": true})
	case "wear", "unwear":
		var err error
		if args[0] == "wear" {
			if len(args) != 2 {
				return fmt.Errorf("usage: design wear <dir|zip>")
			}
			err = designWear(args[1])
		} else if len(args) != 1 {
			return fmt.Errorf("usage: design unwear")
		} else {
			err = designUnwear()
		}
		if err != nil {
			return err
		}
		status, err := badgeStatus(false)
		if err != nil {
			return err
		}
		return emit(status)
	case "fetch":
		fs := flag.NewFlagSet("design fetch", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		pending := fs.Bool("pending", false, "")
		issuer := fs.String("issuer", "", "")
		key := fs.String("key-file", "", "")
		// The documented positional SHA precedes --pending.
		rest := args[1:]
		if len(rest) > 0 && rest[0] != "--pending" {
			rest = append(append([]string{}, rest[1:]...), rest[0])
		}
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: design fetch <sha> [--pending]")
		}
		if !*pending && (*issuer != "" || *key != "") {
			return fmt.Errorf("issuer proof is only used with --pending")
		}
		r, err := designFetch(fs.Arg(0), *pending, *issuer, *key)
		if err != nil {
			return err
		}
		return emit(r)
	case "upload-pending", "approve":
		fs := flag.NewFlagSet("design "+args[0], flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		issuer := fs.String("issuer", "", "")
		key := fs.String("key-file", "", "")
		login := fs.String("login", "", "")
		sha := fs.String("sha", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *issuer != "owner" && *issuer != "mika" {
			return fmt.Errorf("issuer must be owner or mika")
		}
		id := byte(1)
		if *issuer == "mika" {
			id = 2
		}
		if args[0] == "upload-pending" {
			if fs.NArg() != 1 || *login != "" || *sha != "" {
				return fmt.Errorf("usage: design upload-pending --issuer owner|mika [--key-file PATH] <zip>")
			}
			raw, err := readBoundedFile(fs.Arg(0), designZipLimit)
			if err != nil {
				return err
			}
			p, err := designReadZip(raw, "")
			if err != nil {
				return err
			}
			if _, err = canonicalDesignPack(p, false); err != nil {
				return err
			}
			if _, err = designValidate(p); err != nil {
				return err
			}
			seed, err := loadIssuerSeed(*issuer, *key)
			if err != nil {
				return err
			}
			defer clear(seed)
			sum := sha256.Sum256(raw)
			digest := hex.EncodeToString(sum[:])
			path := "/v1/pending/" + digest
			seconds := strconv.FormatInt(time.Now().Unix(), 10)
			payload := []byte(http.MethodPut + "\n" + path + "\n" + seconds + "\n" + digest)
			sig := signDomain(seed, "ryoku-upload-v1", payload)
			header := make(http.Header)
			header.Set("X-Ryoku-Upload", strconv.Itoa(int(id))+"."+seconds+"."+base64.StdEncoding.EncodeToString(sig))
			_, status, err := teamSend(http.MethodPut, path, "application/zip", raw, header, 64<<10)
			if err != nil {
				return err
			}
			if status != 200 {
				return fmt.Errorf("design upload HTTP %d", status)
			}
			return emit(map[string]string{"sha": digest})
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected approval arguments")
		}
		sum, err := designSHA(*sha)
		if err != nil {
			return err
		}
		seed, err := loadIssuerSeed(*issuer, *key)
		if err != nil {
			return err
		}
		defer clear(seed)
		code, err := rydSign(id, seed, sum, *login, uint32(time.Now().Unix()))
		if err != nil {
			return err
		}
		_, status, err := teamSend(http.MethodPost, "/v1/designs", "text/plain", []byte(code), nil, 64<<10)
		if err != nil {
			return err
		}
		if status != 200 {
			return fmt.Errorf("design approval HTTP %d", status)
		}
		return emit(map[string]string{"code": code})
	}
	return fmt.Errorf("unknown design command %q", args[0])
}
