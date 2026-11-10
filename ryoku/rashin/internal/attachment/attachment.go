// Package attachment tells what a file sent with a prompt is. A model is
// shown a picture only when the picture rides on the message, and can read
// any other file itself, so each runner needs to know which it was handed.
package attachment

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"unicode/utf8"
)

// MaxImage is the size of the largest picture sent to a model.
const MaxImage = 8 << 20

// ErrTooLarge reports a picture over MaxImage.
var ErrTooLarge = errors.New("picture too large to send")

// Image is a picture as a model takes it.
type Image struct {
	Mime string
	Data []byte
}

// ReadImage reads the file at path when it is a picture models are shown:
// PNG, JPEG, GIF or WebP, told by its first bytes and not by its name. ok is
// false for anything else, a folder included, and that is not an error. A
// picture over MaxImage is ok with ErrTooLarge and no data.
func ReadImage(path string) (image Image, ok bool, err error) {
	file, err := os.Open(path)
	if err != nil {
		return Image{}, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Image{}, false, err
	}
	if !info.Mode().IsRegular() {
		return Image{}, false, nil
	}
	head := make([]byte, 512)
	read, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return Image{}, false, err
	}
	mime := http.DetectContentType(head[:read])
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return Image{}, false, nil
	}
	if info.Size() > MaxImage {
		return Image{}, true, ErrTooLarge
	}
	rest, err := io.ReadAll(io.LimitReader(file, MaxImage))
	if err != nil {
		return Image{}, true, err
	}
	return Image{Mime: mime, Data: append(head[:read], rest...)}, true, nil
}

// ReadText reads up to limit bytes from the start of the file at path when
// it is text. total is the file's whole size, so a caller can say how much
// was left out. ok is false for anything that is not text, a folder
// included, and that is not an error.
func ReadText(path string, limit int) (text string, total int64, ok bool, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", 0, false, err
	}
	if !info.Mode().IsRegular() {
		return "", 0, false, nil
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(limit)))
	if err != nil {
		return "", 0, false, err
	}
	// a cut can land inside a rune, which says nothing about the file
	for cut := 0; cut < utf8.UTFMax && len(data) > 0 && int64(len(data)) < info.Size() && !utf8.Valid(data); cut++ {
		data = data[:len(data)-1]
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return "", info.Size(), false, nil
	}
	return string(data), info.Size(), true, nil
}
