package engine

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"net/url"
	"os"
	"strings"

	i18n "ryoku-i18n"

	"ryoget/internal/proto"
)

// hex digits per algorithm a checksum may name
var sumLen = map[string]int{"sha256": 64, "sha1": 40, "md5": 32}

// normalSum checks "algo:hex" (or "algo=hex", the fragment form) and returns it as "algo:hex" in lower case.
func normalSum(s string) (string, bool) {
	algo, digest, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok {
		algo, digest, ok = strings.Cut(strings.TrimSpace(s), "=")
	}
	algo, digest = strings.ToLower(strings.TrimSpace(algo)), strings.ToLower(strings.TrimSpace(digest))
	n, known := sumLen[algo]
	if !ok || !known || len(digest) != n {
		return "", false
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return "", false
	}
	return algo + ":" + digest, true
}

// takeChecksum strips a "#sha256=<hex>" style fragment off a web link; a given checksum wins over it
func takeChecksum(link, given string) (string, string, error) {
	sum := ""
	if u, err := url.Parse(link); err == nil && u.Fragment != "" {
		if s, ok := normalSum(u.Fragment); ok {
			u.Fragment, u.RawFragment = "", ""
			link, sum = u.String(), s
		}
	}
	if strings.TrimSpace(given) != "" {
		s, ok := normalSum(given)
		if !ok {
			return "", "", perr(proto.CodeInvalid, i18n.T("That checksum is not a SHA-256, SHA-1 or MD5 digest."))
		}
		sum = s
	}
	return link, sum, nil
}

// hashFile is ported from prism-download-manager crates/prism-engine/src/checksum.rs (MIT)
func hashFile(path, algo string) (string, error) {
	var h hash.Hash
	switch algo {
	case "sha256":
		h = sha256.New()
	case "sha1":
		h = sha1.New()
	case "md5":
		h = md5.New()
	default:
		return "", perr(proto.CodeInternal, "unknown checksum "+algo)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.CopyBuffer(h, f, make([]byte, 1<<20)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// verify is ported from prism-download-manager crates/prism-engine/src/transfer.rs (MIT); it hashes outside the lock
func (e *Engine) verify(t *task) error {
	e.mu.Lock()
	sum, path, kind := t.Checksum, t.Path, t.Kind
	e.mu.Unlock()
	if sum == "" || path == "" || kind != "file" {
		return nil
	}
	algo, want, _ := strings.Cut(sum, ":")
	got, err := hashFile(path, algo)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if got != want {
		removeFinal(t)
		t.Done, t.Pieces = 0, emptyPieces
		return perr(proto.CodeChecksum, i18n.T("The file did not match its checksum, so it was deleted."))
	}
	t.Verified = algo
	return nil
}
