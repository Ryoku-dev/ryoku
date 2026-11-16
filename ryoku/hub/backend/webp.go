package main

import (
	"encoding/binary"
	"fmt"
	"image"
)

type webpInfo struct {
	Width, Height int
	Frames        []image.Rectangle
}

type webpChunk struct {
	kind string
	body []byte
}

func webpChunks(raw []byte) ([]webpChunk, error) {
	var chunks []webpChunk
	for len(raw) > 0 {
		if len(raw) < 8 {
			return nil, fmt.Errorf("truncated WebP chunk header")
		}
		n := uint64(binary.LittleEndian.Uint32(raw[4:8]))
		padded := n + (n & 1)
		if padded > uint64(len(raw)-8) {
			return nil, fmt.Errorf("WebP chunk exceeds file")
		}
		if n&1 != 0 && raw[8+int(n)] != 0 {
			return nil, fmt.Errorf("invalid WebP padding")
		}
		chunks = append(chunks, webpChunk{string(raw[:4]), raw[8 : 8+int(n)]})
		raw = raw[8+int(padded):]
	}
	return chunks, nil
}

func webp24(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }

func webpImageSize(c webpChunk) (int, int, error) {
	b := c.body
	switch c.kind {
	case "VP8 ":
		if len(b) < 10 || b[0]&1 != 0 || string(b[3:6]) != "\x9d\x01\x2a" {
			return 0, 0, fmt.Errorf("invalid VP8 header")
		}
		w, h := int(binary.LittleEndian.Uint16(b[6:8])&0x3fff), int(binary.LittleEndian.Uint16(b[8:10])&0x3fff)
		if w == 0 || h == 0 {
			return 0, 0, fmt.Errorf("empty VP8 canvas")
		}
		return w, h, nil
	case "VP8L":
		if len(b) < 5 || b[0] != 0x2f || b[4]&0xe0 != 0 {
			return 0, 0, fmt.Errorf("invalid VP8L header")
		}
		bits := binary.LittleEndian.Uint32(b[1:5])
		return int(bits&0x3fff) + 1, int((bits>>14)&0x3fff) + 1, nil
	}
	return 0, 0, fmt.Errorf("missing WebP image")
}

func webpFrame(chunks []webpChunk) (int, int, error) {
	var w, h int
	alpha := false
	for i, c := range chunks {
		switch c.kind {
		case "ALPH":
			if alpha || w != 0 || len(c.body) < 1 || c.body[0]&0xc0 != 0 || c.body[0]&3 > 1 || c.body[0]>>4&3 > 1 {
				return 0, 0, fmt.Errorf("invalid ALPH chunk")
			}
			alpha = true
		case "VP8 ", "VP8L":
			if w != 0 || (alpha && c.kind != "VP8 ") || i != len(chunks)-1 {
				return 0, 0, fmt.Errorf("invalid frame image order")
			}
			var err error
			w, h, err = webpImageSize(c)
			if err != nil {
				return 0, 0, err
			}
		default:
			return 0, 0, fmt.Errorf("invalid frame chunk %q", c.kind)
		}
	}
	if w == 0 {
		return 0, 0, fmt.Errorf("missing frame image")
	}
	return w, h, nil
}

func parseWebP(raw []byte) (webpInfo, error) {
	fail := func(s string) (webpInfo, error) { return webpInfo{}, fmt.Errorf("WebP: %s", s) }
	if len(raw) < 12 || string(raw[:4]) != "RIFF" || string(raw[8:12]) != "WEBP" || uint64(binary.LittleEndian.Uint32(raw[4:8]))+8 != uint64(len(raw)) {
		return fail("invalid RIFF length or signature")
	}
	chunks, err := webpChunks(raw[12:])
	if err != nil {
		return webpInfo{}, err
	}
	if len(chunks) == 0 {
		return fail("missing image header")
	}
	var out webpInfo
	extended := chunks[0].kind == "VP8X"
	animated := false
	animSeen := false
	imageSeen := false
	alphaSeen := false
	if extended {
		b := chunks[0].body
		if len(b) != 10 || b[0]&0xc1 != 0 || b[1] != 0 || b[2] != 0 || b[3] != 0 {
			return fail("invalid VP8X header")
		}
		out.Width, out.Height = webp24(b[4:7])+1, webp24(b[7:10])+1
		animated = b[0]&2 != 0
	} else {
		if len(chunks) != 1 {
			return fail("simple WebP must contain one image")
		}
		out.Width, out.Height, err = webpImageSize(chunks[0])
		if err != nil {
			return webpInfo{}, err
		}
		imageSeen = true
	}
	if out.Width > 2048 || out.Height > 2048 {
		return fail("canvas exceeds 2048x2048")
	}
	var pixels int64
	if extended {
		for i, c := range chunks[1:] {
			switch c.kind {
			case "VP8X":
				return fail("duplicate VP8X")
			case "ANIM":
				if !animated || animSeen || len(out.Frames) > 0 || len(c.body) != 6 {
					return fail("invalid ANIM")
				}
				animSeen = true
			case "ANMF":
				if !animated || !animSeen || len(c.body) < 16 || c.body[15]&0xfc != 0 {
					return fail("invalid ANMF")
				}
				x, y := webp24(c.body[:3])*2, webp24(c.body[3:6])*2
				w, h := webp24(c.body[6:9])+1, webp24(c.body[9:12])+1
				if x+w > out.Width || y+h > out.Height {
					return fail("frame outside canvas")
				}
				sub, e := webpChunks(c.body[16:])
				if e != nil {
					return webpInfo{}, e
				}
				iw, ih, e := webpFrame(sub)
				if e != nil {
					return webpInfo{}, e
				}
				if iw != w || ih != h {
					return fail("frame dimensions disagree")
				}
				out.Frames = append(out.Frames, image.Rect(x, y, x+w, y+h))
				pixels += int64(w) * int64(h)
				if len(out.Frames) > 96 || pixels > 96_000_000 {
					return fail("animation exceeds frame budget")
				}
			case "ALPH":
				if animated || alphaSeen || imageSeen || len(c.body) == 0 || c.body[0]&0xc0 != 0 || c.body[0]&3 > 1 || c.body[0]>>4&3 > 1 {
					return fail("invalid ALPH")
				}
				alphaSeen = true
			case "VP8 ", "VP8L":
				if animated || imageSeen || (alphaSeen && c.kind != "VP8 ") {
					return fail("unexpected image chunk")
				}
				w, h, e := webpImageSize(c)
				if e != nil {
					return webpInfo{}, e
				}
				if w != out.Width || h != out.Height {
					return fail("image dimensions disagree")
				}
				imageSeen = true
			case "ICCP":
				if i != 0 {
					return fail("ICCP must precede image")
				}
			case "EXIF", "XMP ":
				if !imageSeen && len(out.Frames) == 0 {
					return fail("metadata precedes image")
				}
			default:
				return fail("unsupported chunk " + c.kind)
			}
		}
	}
	if animated {
		if !animSeen || len(out.Frames) == 0 {
			return fail("missing animation frames")
		}
	} else {
		if !imageSeen {
			return fail("missing image")
		}
		out.Frames = []image.Rectangle{image.Rect(0, 0, out.Width, out.Height)}
	}
	return out, nil
}
