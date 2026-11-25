//go:build !linux

package keys

import (
	"errors"
	"io"
)

// supported: only Linux has evdev.
const supported = false

func watchDir(string, func(string)) (io.Closer, error) {
	return nil, errors.New("keys: no evdev here")
}
