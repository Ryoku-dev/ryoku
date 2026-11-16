package main

// The shareable system card: a PNG of the card Settings draws. Its design travels
// twice: in its pixels as the card code (cardcode.go), which survives a chat app
// re-encoding the picture, and whole (the .ryoprofile envelope, own picture at full
// size) in a tEXt chunk, which a file sent as it is keeps. Opening a card prefers
// the chunk and falls back to the code. The chunk is ASCII (base64 of the
// envelope's JSON), so tEXt's Latin-1 rule never mangles a Japanese tagline.

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

const cardKeyword = "ryoprofile"

var errNoCardDesign = errors.New("this picture is not a Ryoku card, or its design was stripped when it was shared")

// profileEnvelope packs the current profile the way export writes it
func profileEnvelope() ([]byte, error) {
	tmp, err := os.CreateTemp("", "ryoprofile-*")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)
	if err := exportProfile(path); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// embedCardDesign returns the PNG with the envelope in a tEXt chunk just before
// IEND, replacing any design chunk the picture already had
func embedCardDesign(png, envelope []byte) ([]byte, error) {
	chunks, err := pngChunks(png)
	if err != nil {
		return nil, err
	}
	data := append([]byte(cardKeyword+"\x00"), base64.StdEncoding.EncodeToString(envelope)...)
	var out bytes.Buffer
	out.Write(pngSignature)
	for _, c := range chunks {
		if c.kind == "tEXt" && bytes.HasPrefix(c.data, []byte(cardKeyword+"\x00")) {
			continue
		}
		if c.kind == "IEND" {
			writeChunk(&out, "tEXt", data)
		}
		writeChunk(&out, c.kind, c.data)
	}
	return out.Bytes(), nil
}

// extractCardDesign finds the envelope a card PNG carries
func extractCardDesign(png []byte) ([]byte, error) {
	chunks, err := pngChunks(png)
	if err != nil {
		return nil, err
	}
	for _, c := range chunks {
		if c.kind != "tEXt" || !bytes.HasPrefix(c.data, []byte(cardKeyword+"\x00")) {
			continue
		}
		env, err := base64.StdEncoding.DecodeString(string(c.data[len(cardKeyword)+1:]))
		if err != nil {
			return nil, errNoCardDesign
		}
		return env, nil
	}
	return nil, errNoCardDesign
}

type pngChunk struct {
	kind string
	data []byte
}

func pngChunks(b []byte) ([]pngChunk, error) {
	if !bytes.HasPrefix(b, pngSignature) {
		return nil, fmt.Errorf("not a PNG picture")
	}
	var out []pngChunk
	for p := len(pngSignature); p+12 <= len(b); {
		n := int(binary.BigEndian.Uint32(b[p:]))
		if n < 0 || p+12+n > len(b) {
			return nil, fmt.Errorf("the PNG picture is cut short")
		}
		c := pngChunk{kind: string(b[p+4 : p+8]), data: b[p+8 : p+8+n]}
		out = append(out, c)
		p += 12 + n
		if c.kind == "IEND" {
			return out, nil
		}
	}
	return nil, fmt.Errorf("the PNG picture has no end")
}

func writeChunk(w *bytes.Buffer, kind string, data []byte) {
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(data)))
	w.Write(n[:])
	crc := crc32.NewIEEE()
	crc.Write([]byte(kind))
	crc.Write(data)
	w.WriteString(kind)
	w.Write(data)
	binary.BigEndian.PutUint32(n[:], crc.Sum32())
	w.Write(n[:])
}

// writeCard puts the design into the rendered card and writes it to dst, or
// onto the clipboard when dst is "--copy"
func writeCard(rendered, dst string) error {
	png, err := os.ReadFile(rendered)
	if err != nil {
		return err
	}
	env, err := profileEnvelope()
	if err != nil {
		return err
	}
	card, err := embedCardDesign(png, env)
	if err != nil {
		return err
	}
	if dst != "--copy" {
		return atomicWrite(dst, card, 0o644)
	}
	cmd := exec.Command("wl-copy", "--type", "image/png")
	cmd.Stdin = bytes.NewReader(card)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("wl-copy: %v: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

// cardEnvelopeFrom reads a design from a .ryoprofile, a card PNG, or (path
// "--clipboard") a card picture on the clipboard
func cardEnvelopeFrom(path string) ([]byte, error) {
	var raw []byte
	var err error
	if path == "--clipboard" {
		for _, kind := range []string{"image/png", "image/jpeg"} {
			if raw, err = exec.Command("wl-paste", "--no-newline", "--type", kind).Output(); err == nil && len(raw) > 0 {
				break
			}
		}
		if err != nil || len(raw) == 0 {
			return nil, fmt.Errorf("the clipboard holds no picture: %w", errNoCardDesign)
		}
	} else if raw, err = os.ReadFile(path); err != nil {
		return nil, err
	}
	if bytes.HasPrefix(raw, []byte("{")) {
		return raw, nil
	}
	// a card saved straight from Settings still carries its whole design; one that went
	// through a chat app has only its pixels, and the card code in them
	if bytes.HasPrefix(raw, pngSignature) {
		if env, err := extractCardDesign(raw); err == nil {
			return env, nil
		}
	}
	return envelopeFromPixels(raw)
}
