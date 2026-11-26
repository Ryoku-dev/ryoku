package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type gifPart struct {
	image, control  []byte
	delay, disposal int
}
type gifLayout struct {
	prefix              []byte
	width, height, loop int
	frames              []gifPart
}

// Walk every sub-block before allocating pixels. Only structural/control fields
// survive; comments, application payloads and trailing/polyglot bytes do not.
func inspectGIF(raw []byte) (gifLayout, error) {
	var g gifLayout
	bad := func() (gifLayout, error) { return g, fmt.Errorf("GIF: invalid or truncated frame/control data") }
	if len(raw) < 13 {
		return bad()
	}
	g.width = int(binary.LittleEndian.Uint16(raw[6:8]))
	g.height = int(binary.LittleEndian.Uint16(raw[8:10]))
	g.loop = -1
	at := 13
	if raw[10]&128 != 0 {
		at += 3 << (raw[10]&7 + 1)
	}
	if at > len(raw) || !validPictureSize(g.width, g.height) {
		return g, errPictureTooLarge
	}
	g.prefix = raw[:at]
	blocks := func() bool {
		for at < len(raw) {
			n := int(raw[at])
			at++
			if n == 0 {
				return true
			}
			if n > len(raw)-at {
				return false
			}
			at += n
		}
		return false
	}
	var control []byte
	var total int
	for at < len(raw) {
		start := at
		switch raw[at] {
		case 0x3b:
			if at+1 != len(raw) || len(g.frames) == 0 {
				return bad()
			}
			return g, nil
		case 0x21:
			if at+2 >= len(raw) {
				return bad()
			}
			label := raw[at+1]
			at += 2
			if !blocks() {
				return bad()
			}
			switch label {
			case 0xf9:
				if at-start != 8 || raw[start+2] != 4 || raw[start+3]&0xe0 != 0 || (raw[start+3]>>2)&7 > 3 || control != nil {
					return bad()
				}
				control = raw[start:at]
			case 0xff:
				extension := raw[start:at]
				if len(extension) >= 19 && (string(extension[3:14]) == "NETSCAPE2.0" || string(extension[3:14]) == "ANIMEXTS1.0") {
					if len(extension) != 19 || extension[2] != 11 || extension[14] != 3 || extension[15] != 1 {
						return bad()
					}
					g.loop = int(binary.LittleEndian.Uint16(extension[16:18]))
				}
			case 0xfe: // comment: discard
			default:
				return g, fmt.Errorf("GIF: unsupported extension")
			}
		case 0x2c:
			if at+10 > len(raw) {
				return bad()
			}
			x, y := int(binary.LittleEndian.Uint16(raw[at+1:at+3])), int(binary.LittleEndian.Uint16(raw[at+3:at+5]))
			w, h := int(binary.LittleEndian.Uint16(raw[at+5:at+7])), int(binary.LittleEndian.Uint16(raw[at+7:at+9]))
			flags := raw[at+9]
			if flags&0x18 != 0 || w == 0 || h == 0 || x+w > g.width || y+h > g.height {
				return bad()
			}
			at += 10
			if flags&128 != 0 {
				at += 3 << (flags&7 + 1)
			}
			if at >= len(raw) || raw[at] < 2 || raw[at] > 8 {
				return bad()
			}
			at++
			if !blocks() {
				return bad()
			}
			p := gifPart{image: raw[start:at], control: control}
			if control != nil {
				p.delay = int(binary.LittleEndian.Uint16(control[4:6])) * 10
				p.disposal = int(control[3] >> 2 & 7)
			}
			total += p.delay
			g.frames = append(g.frames, p)
			control = nil
			if len(g.frames) > mediaFrameLimit || int64(g.width)*int64(g.height)*int64(len(g.frames)) > mediaPixelLimit || total > maxMovingSeconds*1000 {
				return g, fmt.Errorf("GIF: animation exceeds frame, composited-pixel or duration budget")
			}
		default:
			return bad()
		}
	}
	return bad()
}

func mediaPreflight(raw []byte, kind pictureKind) error {
	switch kind.ext {
	case "gif":
		_, e := inspectGIF(raw)
		return e
	case "webp":
		info, e := parseWebP(raw)
		if e != nil {
			return e
		}
		if int64(info.Width)*int64(info.Height)*int64(len(info.Frames)) > mediaPixelLimit {
			return fmt.Errorf("WebP: composited-pixel budget exceeded")
		}
		chunks, _ := webpChunks(raw[12:])
		total := 0
		for _, c := range chunks {
			if c.kind == "ANMF" {
				total += webp24(c.body[12:15])
			}
		}
		if total > maxMovingSeconds*1000 {
			return fmt.Errorf("WebP: animation duration exceeds 60 seconds")
		}
	case "png", "jpg":
		cfg, _, e := image.DecodeConfig(bytes.NewReader(raw))
		if e != nil {
			return e
		}
		if !validPictureSize(cfg.Width, cfg.Height) {
			return errPictureTooLarge
		}
	}
	return nil
}

// Encoder output is bounded while streaming, not only after a full allocation.
type mediaBuffer struct {
	bytes.Buffer
	err error
}

func (b *mediaBuffer) Write(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	if len(p) > mediaOutputLimit-b.Len() {
		b.err = errPictureTooLarge
		return 0, b.err
	}
	return b.Buffer.Write(p)
}
func (b *mediaBuffer) WriteString(s string) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	if len(s) > mediaOutputLimit-b.Len() {
		b.err = errPictureTooLarge
		return 0, b.err
	}
	return b.Buffer.WriteString(s)
}
func (b *mediaBuffer) WriteByte(v byte) error {
	if b.err != nil {
		return b.err
	}
	if b.Len() >= mediaOutputLimit {
		b.err = errPictureTooLarge
		return b.err
	}
	return b.Buffer.WriteByte(v)
}

func pngBytes(img image.Image) ([]byte, error) {
	var b mediaBuffer
	e := png.Encode(&b, img)
	return b.Bytes(), e
}

func canonicalGIF(raw []byte) (inspectedMedia, error) {
	var out inspectedMedia
	g, err := inspectGIF(raw)
	if err != nil {
		return out, err
	}
	out = inspectedMedia{Extension: "gif", Width: g.width, Height: g.height, Frames: len(g.frames)}
	var encoded mediaBuffer
	encoded.WriteString("GIF89a")
	encoded.Write(g.prefix[6:])
	if g.loop >= 0 {
		encoded.Write([]byte{0x21, 0xff, 11})
		encoded.WriteString("NETSCAPE2.0")
		encoded.Write([]byte{3, 1, byte(g.loop), byte(g.loop >> 8), 0})
	}
	for i, p := range g.frames {
		// The decoder sees one frame at a time; at most one palette frame and one
		// composited poster exist, not an attacker-selected DecodeAll allocation.
		frame := make([]byte, 0, len(g.prefix)+len(p.control)+len(p.image)+1)
		frame = append(frame, g.prefix...)
		frame = append(frame, p.control...)
		frame = append(frame, p.image...)
		frame = append(frame, 0x3b)
		img, e := gif.Decode(bytes.NewReader(frame))
		if e != nil {
			return out, fmt.Errorf("GIF frame %d: %w", i+1, e)
		}
		var clean mediaBuffer
		paletted, ok := img.(*image.Paletted)
		if !ok {
			return out, fmt.Errorf("GIF frame did not decode to a palette")
		}
		normalized := *paletted
		normalized.Rect = image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy())
		if e = gif.Encode(&clean, &normalized, nil); e != nil {
			return out, e
		}
		single, e := inspectGIF(clean.Bytes())
		if e != nil {
			return out, e
		}
		data := append([]byte(nil), single.frames[0].image...)
		// Go's single-frame encoder uses a global palette. Give this frame that
		// palette locally, retaining the source rect, transparency and disposal.
		if data[9]&128 == 0 {
			if len(single.prefix) == 13 {
				return out, fmt.Errorf("GIF frame missing palette")
			}
			data[9] = (data[9] & 0x78) | 128 | (single.prefix[10] & 7)
			data = append(append(append([]byte{}, data[:10]...), single.prefix[13:]...), data[10:]...)
		}
		copy(data[1:9], p.image[1:9])
		// Palette indices remain stable for Paletted images; take the encoder's
		// transparent index, not the source's (which its encoder may normalize).
		control := []byte{0x21, 0xf9, 4, byte(p.disposal << 2), byte(p.delay / 10), byte(p.delay / 10 >> 8), 0, 0}
		if c := single.frames[0].control; len(c) == 8 && c[3]&1 != 0 {
			control[3] |= 1
			control[6] = c[6]
		}
		encoded.Write(control)
		encoded.Write(data)
		out.Durations = append(out.Durations, p.delay)
		if encoded.err != nil {
			return out, encoded.err
		}
		if i == 0 {
			canvas := image.NewNRGBA(image.Rect(0, 0, g.width, g.height))
			draw.Draw(canvas, img.Bounds(), img, img.Bounds().Min, draw.Src)
			out.Poster, e = pngBytes(canvas)
			if e != nil {
				return out, e
			}
		}
		if encoded.Len()+len(out.Poster) > mediaOutputLimit {
			return out, errPictureTooLarge
		}
	}
	if err := encoded.WriteByte(0x3b); err != nil {
		return out, err
	}
	out.Data = encoded.Bytes()
	return out, nil
}

func webpChunkBytes(kind string, body []byte) []byte {
	out := make([]byte, 8+len(body)+(len(body)&1))
	copy(out, kind)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(body)))
	copy(out[8:], body)
	return out
}
func webpContainer(chunks []byte) []byte {
	out := make([]byte, 12, len(chunks)+12)
	copy(out, "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(chunks)+4))
	copy(out[8:], "WEBP")
	return append(out, chunks...)
}
func putWebp24(b []byte, n int) { b[0] = byte(n); b[1] = byte(n >> 8); b[2] = byte(n >> 16) }
func webpExtended(w, h int, flags byte) []byte {
	b := make([]byte, 10)
	b[0] = flags
	putWebp24(b[4:7], w-1)
	putWebp24(b[7:10], h-1)
	return webpChunkBytes("VP8X", b)
}

func workerTool(name string, args ...string) error {
	path, err := filepath.EvalSymlinks("/tools/" + name)
	if err != nil {
		return fmt.Errorf("missing isolated media tool %s", name)
	}
	cmd := exec.Command(path, args...)
	cmd.Env = []string{"PATH=/tools", "HOME=/nonexistent", "LANG=C"}
	cmd.Stderr = &mediaDiagnostic{}
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s rejected the media or exceeded a resource limit: %w: %s", name, err, cmd.Stderr)
	}
	return nil
}
func decodeWebpFrame(raw []byte, w, h int) ([]byte, []byte, error) {
	if err := os.WriteFile("/tmp/frame.webp", raw, 0600); err != nil {
		return nil, nil, err
	}
	if err := workerTool("dwebp", "/tmp/frame.webp", "-o", "/tmp/frame.png", "-quiet"); err != nil {
		return nil, nil, err
	}
	poster, err := readBoundedFile("/tmp/frame.png", mediaOutputLimit)
	if err != nil {
		return nil, nil, err
	}
	img, err := decodePictureLocal(poster)
	if err != nil {
		return nil, nil, err
	}
	if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		return nil, nil, fmt.Errorf("WebP decoded dimensions changed")
	}
	clean, err := pngBytes(img)
	if err != nil {
		return nil, nil, err
	}
	if err = os.WriteFile("/tmp/frame.png", clean, 0600); err != nil {
		return nil, nil, err
	}
	if err = workerTool("cwebp", "-quiet", "-lossless", "-exact", "-metadata", "none", "/tmp/frame.png", "-o", "/tmp/clean.webp"); err != nil {
		return nil, nil, err
	}
	canonical, err := readBoundedFile("/tmp/clean.webp", maxPictureBytes)
	if err != nil {
		return nil, nil, err
	}
	info, err := parseWebP(canonical)
	if err != nil {
		return nil, nil, err
	}
	if info.Width != w || info.Height != h || len(info.Frames) != 1 {
		return nil, nil, fmt.Errorf("invalid canonical WebP frame")
	}
	chunks, err := webpChunks(canonical[12:])
	if err != nil {
		return nil, nil, err
	}
	var pixels []byte
	for _, c := range chunks {
		if c.kind == "VP8L" || c.kind == "VP8 " || c.kind == "ALPH" {
			pixels = append(pixels, webpChunkBytes(c.kind, c.body)...)
		} else if c.kind != "VP8X" {
			return nil, nil, fmt.Errorf("unexpected canonical WebP chunk")
		}
	}
	return pixels, clean, nil
}

func canonicalWebP(raw []byte) (inspectedMedia, error) {
	var out inspectedMedia
	info, err := parseWebP(raw)
	if err != nil {
		return out, err
	}
	out = inspectedMedia{Extension: "webp", Width: info.Width, Height: info.Height, Frames: len(info.Frames)}
	chunks, _ := webpChunks(raw[12:])
	animated := false
	if chunks[0].kind == "VP8X" {
		animated = chunks[0].body[0]&2 != 0
	}
	if !animated {
		pixels, poster, e := decodeWebpFrame(raw, info.Width, info.Height)
		if e != nil {
			return out, e
		}
		out.Data = webpContainer(append(webpExtended(info.Width, info.Height, 0x10), pixels...))
		out.Poster = poster
		out.Durations = []int{0}
		return out, nil
	}
	result := webpExtended(info.Width, info.Height, 0x12)
	frameIndex := 0
	for _, c := range chunks {
		switch c.kind {
		case "ANIM":
			result = append(result, webpChunkBytes("ANIM", c.body)...)
		case "ANMF":
			rect := info.Frames[frameIndex]
			frameIndex++
			sub := append(webpExtended(rect.Dx(), rect.Dy(), 0x10), c.body[16:]...)
			pixels, poster, e := decodeWebpFrame(webpContainer(sub), rect.Dx(), rect.Dy())
			if e != nil {
				return out, e
			}
			result = append(result, webpChunkBytes("ANMF", append(append([]byte{}, c.body[:16]...), pixels...))...)
			out.Durations = append(out.Durations, webp24(c.body[12:15]))
			if frameIndex == 1 {
				img, e := decodePictureLocal(poster)
				if e != nil {
					return out, e
				}
				canvas := image.NewNRGBA(image.Rect(0, 0, info.Width, info.Height))
				draw.Draw(canvas, rect, img, img.Bounds().Min, draw.Src)
				out.Poster, e = pngBytes(canvas)
				if e != nil {
					return out, e
				}
			}
			if len(result)+len(out.Poster) > mediaOutputLimit {
				return out, errPictureTooLarge
			}
		}
	}
	out.Data = webpContainer(result)
	return out, nil
}

func probeStill(path string) error {
	raw, err := mediaTool(context.Background(), "ffprobe", "-v", "error", "-max_alloc", "268435456", "-protocol_whitelist", "file", "-show_entries", "stream=codec_type,width,height", "-of", "json", path)
	if err != nil {
		return err
	}
	var probe struct {
		Streams []struct {
			Type   string `json:"codec_type"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"streams"`
	}
	if err = json.Unmarshal(raw, &probe); err != nil {
		return fmt.Errorf("image dimensions could not be read")
	}
	if len(probe.Streams) != 1 || probe.Streams[0].Type != "video" {
		return fmt.Errorf("still image requires exactly one image stream")
	}
	if !validPictureSize(probe.Streams[0].Width, probe.Streams[0].Height) {
		return errPictureTooLarge
	}
	return nil
}
func canonicalMedia(raw []byte, allowVideo bool) (inspectedMedia, error) {
	var out inspectedMedia
	kind, err := sniffPicture(raw)
	if err != nil {
		return out, err
	}
	if err = mediaPreflight(raw, kind); err != nil {
		return out, err
	}
	switch kind.ext {
	case "gif":
		return canonicalGIF(raw)
	case "webp":
		return canonicalWebP(raw)
	case "png", "jpg":
		img, e := decodePictureLocal(raw)
		if e != nil {
			return out, e
		}
		out = inspectedMedia{Extension: kind.ext, Width: img.Bounds().Dx(), Height: img.Bounds().Dy(), Frames: 1, Durations: []int{0}}
		out.Poster, e = pngBytes(img)
		if e != nil {
			return out, e
		}
		if kind.ext == "jpg" {
			var b mediaBuffer
			e = jpeg.Encode(&b, img, &jpeg.Options{Quality: 95})
			out.Data = b.Bytes()
		} else {
			out.Data = out.Poster
		}
		return out, e
	default:
		if kind.video && !allowVideo {
			return out, fmt.Errorf("video is not supported in image imports")
		}
		if err = os.WriteFile("/tmp/input", raw, 0600); err != nil {
			return out, err
		}
		if kind.video {
			if err = probeVideo(context.Background(), "/tmp/input", kind); err != nil {
				return out, err
			}
			// Existing badge-only video remains local, but no original container is
			// handed to Qt: strip audio/attachments/metadata and encode one bounded stream.
			if err = workerTool("ffmpeg", "-nostdin", "-v", "error", "-y", "-threads", "1", "-filter_threads", "1", "-protocol_whitelist", "file", "-i", "/tmp/input", "-map", "0:v:0", "-an", "-sn", "-dn", "-map_metadata", "-1", "-t", strconv.Itoa(maxMovingSeconds), "-vf", "scale=w='min(1600,iw)':h='min(1600,ih)':force_original_aspect_ratio=decrease", "-c:v", "libvpx-vp9", "-threads", "1", "-b:v", "1M", "-frames:v", "1800", "-f", "webm", "/tmp/video.webm"); err != nil {
				return out, err
			}
			out.Data, err = readBoundedFile("/tmp/video.webm", maxMovingBytes)
			if err != nil {
				return out, err
			}
			out.Extension = "webm"
		} else {
			if err = probeStill("/tmp/input"); err != nil {
				return out, err
			}
			out.Extension = "png"
		}
		if err = workerTool("ffmpeg", "-nostdin", "-v", "error", "-y", "-threads", "1", "-filter_threads", "1", "-protocol_whitelist", "file", "-i", "/tmp/input", "-frames:v", "1", "-an", "-sn", "-dn", "-map_metadata", "-1", "-f", "image2", "-c:v", "png", "-threads", "1", "/tmp/poster.png"); err != nil {
			return out, err
		}
		poster, e := readBoundedFile("/tmp/poster.png", maxPictureBytes)
		if e != nil {
			return out, e
		}
		img, e := decodePictureLocal(poster)
		if e != nil {
			return out, e
		}
		out.Width = img.Bounds().Dx()
		out.Height = img.Bounds().Dy()
		out.Frames = 1
		out.Durations = []int{0}
		out.Poster, e = pngBytes(img)
		if !kind.video {
			out.Data = out.Poster
		}
		return out, e
	}
}
