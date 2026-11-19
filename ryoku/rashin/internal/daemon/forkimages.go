package daemon

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// keepRuntimeImages gives a kept conversation its own copies before a capture
// card or the login session removes the originals.
func keepRuntimeImages(events []protocol.SessionEvent, id string) (kept []protocol.SessionEvent, err error) {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" || !filepath.IsAbs(runtime) {
		return events, nil
	}
	dir := filepath.Join(paths.AttachmentsDir(), id)
	made := false
	defer func() {
		if err != nil && made {
			_ = os.RemoveAll(dir)
		}
	}()
	copied := make(map[string]string)
	kept = append([]protocol.SessionEvent(nil), events...)
	for index, event := range kept {
		if event.Kind != protocol.EvUser {
			continue
		}
		var user protocol.UserData
		if err = json.Unmarshal(event.Data, &user); err != nil {
			return nil, err
		}
		changed := false
		for i, attached := range user.Images {
			rel, relErr := filepath.Rel(runtime, attached.Path)
			if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			path := copied[attached.Path]
			if path == "" {
				if err = paths.EnsureDir(dir); err != nil {
					return nil, err
				}
				made = true
				path, err = copyKeptImage(attached.Path, dir)
				if err != nil {
					return nil, err
				}
				if path == "" {
					continue
				}
				copied[attached.Path] = path
			}
			user.Images[i].Path = path
			changed = true
		}
		if changed {
			kept[index].Data, err = json.Marshal(user)
			if err != nil {
				return nil, err
			}
		}
	}
	return kept, nil
}

func copyKeptImage(source, dir string) (string, error) {
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	var head [512]byte
	n, err := io.ReadFull(in, head[:])
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", err
	}
	switch http.DetectContentType(head[:n]) {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return "", nil
	}
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	out, err := os.CreateTemp(dir, "capture-*"+filepath.Ext(source))
	if err != nil {
		return "", err
	}
	_, err = io.Copy(out, in)
	closed := out.Close()
	if err == nil {
		err = closed
	}
	if err != nil {
		_ = os.Remove(out.Name())
		return "", err
	}
	return out.Name(), nil
}
