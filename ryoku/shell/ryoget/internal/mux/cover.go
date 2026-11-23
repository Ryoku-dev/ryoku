package mux

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
)

// ffmpeg cannot give Ogg a picture stream, so opus and ogg take a FLAC picture block comment
var coverStyle = map[string]string{
	"mp3": "stream", "m4a": "stream", "flac": "stream",
	"opus": "comment", "ogg": "comment", "oga": "comment",
}

// CoverBoxes reports whether an audio file of type box can carry a cover.
func CoverBoxes(box string) bool { return coverStyle[strings.ToLower(box)] != "" }

func coverFile(out string) string {
	return filepath.Join(filepath.Dir(out), "."+filepath.Base(out)+".ryoget-cover.jpg")
}

func coverMeta(out string) string {
	return filepath.Join(filepath.Dir(out), "."+filepath.Base(out)+".ryoget-cover.txt")
}

// withCover maps input 0's audio and picture as a cover, or the audio alone when picture is unusable.
func withCover(ctx context.Context, bin, box, picture, out string) []string {
	plain := []string{"-map", "0:a:0", "-vn"}
	style := coverStyle[box]
	if picture == "" || style == "" {
		return plain
	}
	// thumbnails come as webp, png or jpeg, and one JPEG suits every container
	jpg := coverFile(out)
	// -update 1 writes a single image, so a % in the folder name is not read as a frame pattern
	if err := run(ctx, bin, []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-i", picture, "-frames:v", "1", "-c:v", "mjpeg", "-q:v", "2", "-update", "1", "-f", "image2", jpg}); err != nil {
		return plain
	}
	if style == "stream" {
		// without the comment mp3 and flac label the picture type Other instead of the front cover
		return []string{"-i", jpg, "-map", "0:a:0", "-map", "1:v:0", "-c:v", "copy", "-disposition:v:0", "attached_pic",
			"-metadata:s:v:0", "comment=Cover (front)"}
	}
	block, err := pictureBlock(jpg)
	if err != nil {
		return plain
	}
	meta := coverMeta(out)
	b64 := base64.StdEncoding.EncodeToString(block)
	if err := os.WriteFile(meta, []byte(";FFMETADATA1\nMETADATA_BLOCK_PICTURE="+strings.ReplaceAll(b64, "=", `\=`)+"\n"), 0o600); err != nil {
		return plain
	}
	// the source keeps its tags and only the audio stream gains the picture comment
	return []string{"-i", meta, "-map", "0:a:0", "-vn", "-map_metadata", "0", "-map_metadata:s:a:0", "1:g"}
}

// pictureBlock builds a FLAC METADATA_BLOCK_PICTURE for a front cover.
func pictureBlock(jpg string) ([]byte, error) {
	data, err := os.ReadFile(jpg)
	if err != nil {
		return nil, err
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	const mimeType = "image/jpeg"
	var b bytes.Buffer
	for _, v := range []uint32{3, uint32(len(mimeType))} {
		binary.Write(&b, binary.BigEndian, v)
	}
	b.WriteString(mimeType)
	for _, v := range []uint32{0, uint32(cfg.Width), uint32(cfg.Height), 24, 0, uint32(len(data))} {
		binary.Write(&b, binary.BigEndian, v)
	}
	b.Write(data)
	return b.Bytes(), nil
}
