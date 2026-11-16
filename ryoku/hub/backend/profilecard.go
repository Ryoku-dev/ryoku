package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// writeCard shares only the rendered pixels. The original picture and private
// profile settings must never travel with a card.
func writeCard(rendered, dst string) error {
	card, err := os.ReadFile(rendered)
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
