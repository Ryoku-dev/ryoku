package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"syscall"
)

const (
	maxPictureBytes     = 16 << 20
	maxPictureDimension = 8192
	maxPicturePixels    = 24_000_000
)

var errPictureTooLarge = errors.New("picture exceeds the size limit")

func readPictureFile(path string) ([]byte, error) {
	return readBoundedFile(path, maxPictureBytes)
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	if info.Size() > limit {
		return nil, errPictureTooLarge
	}
	// The limit still applies if the file grows after Stat.
	return readBounded(f, limit)
}

func readBounded(r io.Reader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errPictureTooLarge
	}
	return raw, nil
}

func readLimitedCommand(cmd *exec.Cmd, limit int64) ([]byte, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		stdout.Close()
		return nil, err
	}
	raw, readErr := readBounded(stdout, limit)
	if readErr != nil {
		// Waiting before killing lets a clipboard writer block forever on its pipe.
		_ = cmd.Process.Kill()
	}
	stdout.Close()
	waitErr := cmd.Wait()
	if readErr != nil {
		return nil, readErr
	}
	return raw, waitErr
}

func validPictureSize(w, h int) bool {
	return w > 0 && h > 0 && w <= maxPictureDimension && h <= maxPictureDimension && int64(w)*int64(h) <= maxPicturePixels
}

func decodePicture(raw []byte) (image.Image, error) {
	if len(raw) > maxPictureBytes {
		return nil, errPictureTooLarge
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if format != "png" && format != "jpeg" && format != "gif" {
		return nil, fmt.Errorf("unsupported picture format")
	}
	if !validPictureSize(config.Width, config.Height) {
		return nil, errPictureTooLarge
	}
	img, decodedFormat, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if decodedFormat != format || img.Bounds().Dx() != config.Width || img.Bounds().Dy() != config.Height {
		return nil, fmt.Errorf("picture dimensions changed while decoding")
	}
	return img, nil
}
