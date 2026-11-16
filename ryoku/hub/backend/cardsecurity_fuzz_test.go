package main

import (
	"image"
	"image/color"
	"testing"
)

func codewordFromLevels(levels [][]int) []byte {
	var ticks []int
	for _, row := range levels {
		ticks = append(ticks, row[1:len(row)-1]...)
	}
	code := make([]byte, 0, codeBytes())
	for i := 0; i+3 < len(ticks); i += 4 {
		code = append(code, byte(ticks[i]<<6|ticks[i+1]<<4|ticks[i+2]<<2|ticks[i+3]))
	}
	return code
}

func FuzzCardBodyRS(f *testing.F) {
	for _, d := range []codedDesign{{name: "friend"}, {kind: 1, gallerySrc: "hawk", name: "Nero 力", tagline: "Power and beauty", owner: [3]byte{1, 2, 3}}, {kind: 2, name: "<img src=//x>"}} {
		levels, err := encodeLevels(d)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(d.bytes(), codeParity)
		f.Add(codewordFromLevels(levels), codeParity)
	}
	f.Add([]byte{}, -1)
	f.Fuzz(func(t *testing.T, raw []byte, parity int) {
		if len(raw) > 4096 {
			return
		}
		if d, err := parseDesign(raw); err == nil {
			if cleanCardText(d.name) != d.name || cleanCardText(d.tagline) != d.tagline || cleanCardText(d.epithet) != d.epithet || cleanCardText(d.gpu) != d.gpu {
				t.Fatal("unsanitized design")
			}
		}
		_, _ = rsDecode(raw, parity)
		levels := make([][]int, len(rulerY))
		for r := range levels {
			levels[r] = make([]int, rulerCount)
			for k := range levels[r] {
				if len(raw) > 0 {
					levels[r][k] = int(raw[(r*rulerCount+k)%len(raw)])
				}
			}
		}
		_, _ = decodeLevels(levels)
	})
}

func FuzzCardPixels(f *testing.F) {
	for _, d := range []codedDesign{{name: "friend"}, {kind: 1, gallerySrc: "hawk", owner: [3]byte{1, 2, 3}}, {kind: 2, name: "custom"}} {
		f.Add(d.bytes(), uint16(472), uint16(660))
	}
	f.Add([]byte{0, 255, 0, 255}, uint16(0), uint16(1))
	f.Fuzz(func(t *testing.T, data []byte, width, height uint16) {
		if len(data) > 4096 {
			return
		}
		w, h := int(width)%513, int(height)%721
		var img image.Image
		if d, err := parseDesign(data); err == nil && w >= 467 {
			levels, err := encodeLevels(d)
			if err != nil {
				t.Fatal(err)
			}
			img = drawCard(levels, float64(w)/cardW, color.RGBA{18, 12, 0, 255}, color.RGBA{226, 214, 196, 255})
		} else {
			pixels := image.NewRGBA(image.Rect(0, 0, w, h))
			for i := range pixels.Pix {
				if len(data) > 0 {
					pixels.Pix[i] = data[i%len(data)]
				}
			}
			img = pixels
		}
		if levels, err := readLevels(img); err == nil {
			_, _ = decodeLevels(levels)
		}
	})
}
