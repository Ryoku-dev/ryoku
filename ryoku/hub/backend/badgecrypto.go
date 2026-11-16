package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode"
)

const badgeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func crockfordEncode(b []byte) string {
	var out strings.Builder
	var acc uint32
	var bits uint
	for _, v := range b {
		acc = acc<<8 | uint32(v)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out.WriteByte(badgeAlphabet[(acc>>bits)&31])
		}
	}
	if bits > 0 {
		out.WriteByte(badgeAlphabet[(acc<<(5-bits))&31])
	}
	return out.String()
}
func crockfordDecode(s string) ([]byte, error) {
	if len(s) > 600 {
		return nil, errors.New("code exceeds 600 characters")
	}
	var out []byte
	var acc uint32
	var bits uint
	for _, r := range strings.ToUpper(s) {
		if r == '-' || unicode.IsSpace(r) {
			continue
		}
		if r == 'I' || r == 'L' {
			r = '1'
		}
		if r == 'O' {
			r = '0'
		}
		n := strings.IndexRune(badgeAlphabet, r)
		if n < 0 {
			return nil, errors.New("invalid code character")
		}
		acc = acc<<5 | uint32(n)
		bits += 5
		if bits >= 8 {
			bits -= 8
			out = append(out, byte(acc>>bits))
		}
	}
	if bits >= 5 || (bits > 0 && acc&((1<<bits)-1) != 0) {
		return nil, errors.New("noncanonical code padding")
	}
	return out, nil
}
func codeText(prefix string, b []byte) string {
	s := crockfordEncode(b)
	var out strings.Builder
	out.WriteString(prefix)
	for len(s) > 0 {
		n := 5
		if len(s) < n {
			n = len(s)
		}
		out.WriteByte('-')
		out.WriteString(s[:n])
		s = s[n:]
	}
	return out.String()
}
func parseCodeText(prefix, text string) ([]byte, error) {
	if len(text) > 600 {
		return nil, errors.New("code exceeds 600 characters")
	}
	s := strings.Map(func(r rune) rune {
		if r == '-' || unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToUpper(r)
	}, text)
	if !strings.HasPrefix(s, prefix) {
		return nil, errors.New("wrong code prefix")
	}
	return crockfordDecode(s[len(prefix):])
}
func signDomain(seed []byte, domain string, payload []byte) []byte {
	if len(seed) != ed25519.SeedSize {
		return nil
	}
	key := ed25519.NewKeyFromSeed(seed)
	defer clear(key)
	msg := append(append([]byte(domain), 0), payload...)
	return ed25519.Sign(key, msg)
}
func verifyDomain(pub []byte, domain string, payload, sig []byte) bool {
	if len(pub) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(pub, append(append([]byte(domain), 0), payload...), sig)
}
func issuerPublic(issuer byte) ([]byte, bool) {
	p, ok := badgeIssuerKeys[issuer]
	return p, ok && len(p) == ed25519.PublicKeySize
}
func loadIssuerSeed(issuer, keyFile string) ([]byte, error) {
	if issuer != "owner" && issuer != "mika" {
		return nil, errors.New("unknown issuer")
	}
	var b []byte
	defer func() { clear(b) }()
	if keyFile != "" {
		f, e := os.OpenFile(keyFile, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		st, e := f.Stat()
		if e != nil {
			return nil, e
		}
		stat, owned := st.Sys().(*syscall.Stat_t)
		if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 || !owned || stat.Uid != uint32(os.Getuid()) {
			return nil, errors.New("issuer key must be a private regular file owned by this user (0600)")
		}
		b, e = io.ReadAll(io.LimitReader(f, 257))
		if e != nil {
			return nil, e
		}
	} else {
		if issuer != "owner" {
			return nil, errors.New("Mika requires --key-file")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "secret-tool", "lookup", "service", "ryoku-badge", "key", "owner")
		cmd.WaitDelay = time.Second
		out := badgeBoundedOutput{Buffer: *bytes.NewBuffer(make([]byte, 0, 256))}
		defer func() { clear(out.Bytes()) }()
		cmd.Stdout = &out
		if e := cmd.Run(); e != nil {
			return nil, errors.New("owner key unavailable")
		}
		b = out.Bytes()
	}
	if len(b) > 256 {
		return nil, errors.New("issuer key too large")
	}
	encoded := bytes.TrimSpace(b)
	seed := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	n, e := base64.StdEncoding.Decode(seed, encoded)
	if e != nil || n != ed25519.SeedSize {
		clear(seed)
		return nil, errors.New("invalid issuer seed")
	}
	seed = seed[:n]
	id := byte(1)
	if issuer == "mika" {
		id = 2
	}
	pub, _ := issuerPublic(id)
	key := ed25519.NewKeyFromSeed(seed)
	defer clear(key)
	if !bytes.Equal(key.Public().(ed25519.PublicKey), pub) {
		clear(seed)
		return nil, errors.New("issuer key does not match public key")
	}
	return seed, nil
}

type badgeBoundedOutput struct{ bytes.Buffer }

func (b *badgeBoundedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 256 {
		return 0, errors.New("keyring output too large")
	}
	return b.Buffer.Write(p)
}

var badgeEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func badgeDay(t time.Time) uint16 {
	d := int64(t.Sub(badgeEpoch) / (24 * time.Hour))
	if d < 0 {
		return 0
	}
	if d > 65535 {
		return 65535
	}
	return uint16(d)
}
func badgeDate(day uint16) string { return badgeEpoch.AddDate(0, 0, int(day)).Format("2006-01-02") }
