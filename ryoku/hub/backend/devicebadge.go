package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func badgeDataDir() (string, error) {
	base, e := os.UserHomeDir()
	if e != nil {
		return "", e
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		base = x
	} else {
		base = filepath.Join(base, ".local", "share")
	}
	dir := filepath.Join(base, "ryoku", "badge")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	st, e := os.Lstat(dir)
	if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("invalid badge directory")
	}
	if e = os.Chmod(dir, 0700); e != nil {
		return "", e
	}
	return dir, nil
}
func badgeDeviceSeed() ([]byte, error) {
	dir, e := badgeDataDir()
	if e != nil {
		return nil, e
	}
	path := filepath.Join(dir, "device.key")
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e == nil {
		seed := make([]byte, 32)
		if _, e = rand.Read(seed); e == nil {
			encoded := base64.StdEncoding.AppendEncode(nil, seed)
			encoded = append(encoded, '\n')
			_, e = f.Write(encoded)
			clear(encoded)
		}
		closeErr := f.Close()
		if e != nil {
			clear(seed)
			os.Remove(path)
			return nil, e
		}
		if closeErr != nil {
			clear(seed)
			return nil, closeErr
		}
		return seed, nil
	}
	if !os.IsExist(e) {
		return nil, e
	}
	return badgeReadDeviceSeed(path)
}

func badgeExistingDeviceSeed() ([]byte, error) {
	dir, e := badgeDataDir()
	if e != nil {
		return nil, e
	}
	return badgeReadDeviceSeed(filepath.Join(dir, "device.key"))
}

func badgeReadDeviceSeed(path string) ([]byte, error) {
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return nil, errors.New("device key must be a private regular file")
	}
	raw, e := readBounded(f, 256)
	defer clear(raw)
	if e != nil {
		return nil, e
	}
	encoded := bytes.TrimSpace(raw)
	seed := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	n, e := base64.StdEncoding.Decode(seed, encoded)
	if e != nil || n != ed25519.SeedSize {
		clear(seed)
		return nil, errors.New("invalid device key")
	}
	return seed[:n], nil
}
func badgeDevicePublic(seed []byte) []byte {
	key := ed25519.NewKeyFromSeed(seed)
	defer clear(key)
	return key.Public().(ed25519.PublicKey)
}

func cardID(pub []byte) [8]byte {
	sum := sha256.Sum256(append([]byte("ryoku-card-id-v1\x00"), pub...))
	return [8]byte(sum[:8])
}

func cardIDText(pub []byte) string {
	id := cardID(pub)
	return hex.EncodeToString(id[:])
}

func ownCardID() ([8]byte, error) {
	seed, e := badgeDeviceSeed()
	if e != nil {
		return [8]byte{}, e
	}
	defer clear(seed)
	return cardID(badgeDevicePublic(seed)), nil
}
func badgeActive() (badgeInfo, error) {
	dir, e := badgeDataDir()
	if e != nil {
		return badgeInfo{}, e
	}
	raw, e := readBoundedFile(filepath.Join(dir, "badge.txt"), 600)
	if e != nil {
		return badgeInfo{}, e
	}
	info, e := badgeParse(string(raw))
	if e != nil {
		return badgeInfo{}, e
	}
	seed, e := badgeDeviceSeed()
	if e != nil {
		return badgeInfo{}, e
	}
	defer clear(seed)
	pub := badgeDevicePublic(seed)
	if info.CardID != cardIDText(pub) || !bytes.Equal(info.device, pub) {
		return badgeInfo{}, errors.New("badge belongs to another machine or device")
	}
	return info, nil
}
func badgeActivate(text string) error {
	info, e := badgeParse(text)
	if e != nil {
		return e
	}
	seed, e := badgeDeviceSeed()
	if e != nil {
		return e
	}
	defer clear(seed)
	pub := badgeDevicePublic(seed)
	if info.CardID != cardIDText(pub) || !bytes.Equal(info.device, pub) {
		return errors.New("badge belongs to another machine or device")
	}
	dir, e := badgeDataDir()
	if e != nil {
		return e
	}
	return atomicWrite(filepath.Join(dir, "badge.txt"), []byte(info.Code), 0600)
}
