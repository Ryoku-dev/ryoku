package attachment

import (
	"bytes"
	"io"
	"os"
)

// IsAudio recognizes recordings by their container bytes, not their filename.
func IsAudio(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return false, err
	}
	head, err := io.ReadAll(io.LimitReader(file, 64<<10))
	if err != nil {
		return false, err
	}
	return audioBytes(head), nil
}

func audioBytes(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	if bytes.HasPrefix(b, []byte("fLaC")) || bytes.HasPrefix(b, []byte("ID3")) {
		return true
	}
	if len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WAVE" {
		return true
	}
	if bytes.HasPrefix(b, []byte("OggS")) {
		return bytes.Contains(b, []byte("OpusHead")) || bytes.Contains(b, []byte("\x01vorbis")) || bytes.Contains(b, []byte("fLaC")) || bytes.Contains(b, []byte("Speex   "))
	}
	if len(b) >= 12 && string(b[4:8]) == "ftyp" {
		for i := 8; i+4 <= len(b) && i < 64; i += 4 {
			switch string(b[i : i+4]) {
			case "M4A ", "M4B ", "M4P ", "F4A ", "F4B ":
				return true
			}
		}
	}
	if bytes.HasPrefix(b, []byte{0x1a, 0x45, 0xdf, 0xa3}) && bytes.Contains(b, []byte("webm")) {
		return !bytes.Contains(b, []byte("V_")) && (bytes.Contains(b, []byte("A_OPUS")) || bytes.Contains(b, []byte("A_VORBIS")))
	}
	// reserved version, layer, bitrate and sample-rate values are not MPEG audio
	return b[0] == 0xff && b[1]&0xe0 == 0xe0 && b[1]&0x18 != 0x08 && b[1]&0x06 != 0 && b[2]&0xf0 != 0 && b[2]&0xf0 != 0xf0 && b[2]&0x0c != 0x0c
}
