package main

import (
	"bytes"
	"fmt"
	"image/png"
	"os/exec"
)

// writeCard shares only the rendered pixels. The original picture and private
// profile settings must never travel with a card.
func writeCard(rendered, dst string) error {
	raw, err := readPictureFile(rendered)
	if err != nil {
		return err
	}
	img, err := decodePicture(raw)
	if err != nil {
		return fmt.Errorf("card picture: %w", err)
	}
	// A render file is still an input boundary: carrying its original bytes
	// would also share text chunks, EXIF, profiles and any trailing payload.
	var clean bytes.Buffer
	if err := png.Encode(&clean, img); err != nil {
		return fmt.Errorf("card PNG: %w", err)
	}
	card := clean.Bytes()
	if len(card) > maxPictureBytes {
		return fmt.Errorf("card PNG is too large")
	}
	if dst != "--copy" {
		return atomicWrite(dst, card, 0o644)
	}
	cmd := exec.Command("wl-copy", "--type", "image/png")
	cmd.Stdin = bytes.NewReader(card)
	if out, err := readLimitedCommand(cmd, 64<<10); err != nil {
		return fmt.Errorf("wl-copy: %v: %s", err, bytes.TrimSpace(out))
	}
	return nil
}
