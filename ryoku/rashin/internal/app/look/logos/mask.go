package logos

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"

	"gioui.org/op/paint"
)

// a palette change leaves old inks behind, so past this the cache starts over
const cacheLimit = 256

type tinted struct {
	mark string
	px   int
	ink  color.NRGBA
}

var cache struct {
	sync.Mutex
	masks  map[string]*image.Gray
	images map[tinted]paint.ImageOp
}

// the mark for id, px square in ink; false where id has no mark
func Image(id string, px int, ink color.NRGBA) (paint.ImageOp, bool) {
	mark := markFor(id)
	if mark == "" || px <= 0 {
		return paint.ImageOp{}, false
	}
	key := tinted{mark: mark, px: px, ink: ink}
	cache.Lock()
	defer cache.Unlock()
	// one ImageOp per key keeps the texture Gio uploaded for it
	if kept, ok := cache.images[key]; ok {
		return kept, true
	}
	mask := cache.masks[mark]
	if mask == nil {
		mask = decode(mark)
		if mask == nil {
			return paint.ImageOp{}, false
		}
		if cache.masks == nil {
			cache.masks = make(map[string]*image.Gray)
		}
		cache.masks[mark] = mask
	}
	if cache.images == nil || len(cache.images) >= cacheLimit {
		cache.images = make(map[tinted]paint.ImageOp)
	}
	result := paint.NewImageOp(tint(scale(mask, px), px, ink))
	cache.images[key] = result
	return result, true
}

func decode(mark string) *image.Gray {
	data, err := files.ReadFile("masks/" + mark + ".png")
	if err != nil {
		return nil
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	gray, ok := decoded.(*image.Gray)
	if !ok || gray.Rect.Dx() != gray.Rect.Dy() {
		return nil
	}
	return gray
}

type tap struct {
	at     int
	weight float32
}

// each target pixel takes the mean of the source area it covers, so a thin stroke keeps its weight
func taps(from, to int) [][]tap {
	ratio := float32(from) / float32(to)
	result := make([][]tap, to)
	for index := range result {
		start, end := float32(index)*ratio, float32(index+1)*ratio
		for at := int(start); at < from && float32(at) < end; at++ {
			if overlap := min(end, float32(at+1)) - max(start, float32(at)); overlap > 0 {
				result[index] = append(result[index], tap{at: at, weight: overlap / ratio})
			}
		}
	}
	return result
}

// coverage 0 to 1, px by px, row after row
func scale(mask *image.Gray, px int) []float32 {
	from := mask.Rect.Dx()
	steps := taps(from, px)
	across := make([]float32, from*px)
	for y := range from {
		row := mask.Pix[y*mask.Stride : y*mask.Stride+from]
		for x, step := range steps {
			var sum float32
			for _, tap := range step {
				sum += float32(row[tap.at]) * tap.weight
			}
			across[y*px+x] = sum / 255
		}
	}
	result := make([]float32, px*px)
	for y, step := range steps {
		for x := range px {
			var sum float32
			for _, tap := range step {
				sum += across[tap.at*px+x] * tap.weight
			}
			result[y*px+x] = sum
		}
	}
	return result
}

func tint(coverage []float32, px int, ink color.NRGBA) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, px, px))
	for index, value := range coverage {
		pixel := result.Pix[index*4 : index*4+4]
		pixel[0], pixel[1], pixel[2] = ink.R, ink.G, ink.B
		pixel[3] = uint8(min(max(value, 0), 1)*float32(ink.A) + 0.5)
	}
	return result
}
