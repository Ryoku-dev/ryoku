package attachment

import (
	"bytes"
	"image"
	_ "image/gif"
	"image/jpeg"
	pngcodec "image/png"
	"io"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

func shrinkImage(source io.Reader) (Image, error) {
	picture, _, err := image.Decode(source)
	if err != nil {
		return Image{}, err
	}
	opaque := false
	if p, ok := picture.(interface{ Opaque() bool }); ok {
		opaque = p.Opaque()
	}
	mime := "image/png"
	if opaque {
		mime = "image/jpeg"
	}
	bounds := picture.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	var encoded bytes.Buffer
	for {
		encoded.Reset()
		if opaque {
			err = jpeg.Encode(&encoded, picture, &jpeg.Options{Quality: 85})
		} else {
			err = pngcodec.Encode(&encoded, picture)
		}
		if err != nil {
			return Image{}, err
		}
		if encoded.Len() <= MaxImage {
			return Image{Mime: mime, Data: encoded.Bytes()}, nil
		}
		// Each pass reduces the longest side by a quarter, keeping the ratio.
		longest := max(width, height)
		next := max(1, longest*3/4)
		w, h := max(1, width*next/longest), max(1, height*next/longest)
		smaller := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.ApproxBiLinear.Scale(smaller, smaller.Bounds(), picture, picture.Bounds(), draw.Src, nil)
		picture = smaller
		width, height = w, h
	}
}
