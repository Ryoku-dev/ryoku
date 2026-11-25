package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// the daemon reads an attachment whole, so a pasted image stays a reasonable size
const pastedLimit = 32 << 20

var imageTypes = []struct{ mime, extension string }{
	{"image/png", ".png"},
	{"image/jpeg", ".jpg"},
	{"image/webp", ".webp"},
	{"image/gif", ".gif"},
}

// false when the clipboard holds text or no image at all
func ClipboardImage(ctx context.Context, dir string) (string, bool) {
	offered, err := exec.CommandContext(ctx, "wl-paste", "--list-types").Output()
	if err != nil {
		return "", false
	}
	mime, extension := pastedType(string(offered))
	if mime == "" {
		return "", false
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false
	}
	name := make([]byte, 6)
	if _, err := rand.Read(name); err != nil {
		return "", false
	}
	path := filepath.Join(dir, "pasted-"+hex.EncodeToString(name)+extension)
	if !savePasted(ctx, mime, path) {
		_ = os.Remove(path)
		return "", false
	}
	return path, true
}

// a copy that offers text along with a picture stays text, the field already pasted it
func pastedType(offered string) (mime, extension string) {
	types := make(map[string]bool)
	for _, line := range strings.Split(offered, "\n") {
		types[strings.TrimSpace(line)] = true
	}
	if types["text/plain"] || types["text/plain;charset=utf-8"] || types["UTF8_STRING"] {
		return "", ""
	}
	for _, image := range imageTypes {
		if types[image.mime] {
			return image.mime, image.extension
		}
	}
	return "", ""
}

func savePasted(ctx context.Context, mime, path string) bool {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return false
	}
	defer file.Close()
	paste := exec.CommandContext(ctx, "wl-paste", "--type", mime)
	output, err := paste.StdoutPipe()
	if err != nil || paste.Start() != nil {
		return false
	}
	written, copyErr := io.Copy(file, io.LimitReader(output, pastedLimit+1))
	if written > pastedLimit {
		_ = paste.Process.Kill()
	}
	waitErr := paste.Wait()
	return copyErr == nil && waitErr == nil && written > 0 && written <= pastedLimit
}
