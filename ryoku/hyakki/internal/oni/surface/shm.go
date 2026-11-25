package surface

import "image"

// toARGB8888 copies src into dst, a little-endian premultiplied ARGB8888
// buffer of src's size (wl_shm's universal format).
func toARGB8888(dst []byte, src *image.NRGBA) {
	b := src.Bounds()
	w := b.Dx()
	for y := range b.Dy() {
		row := src.Pix[y*src.Stride : y*src.Stride+w*4]
		out := dst[y*w*4 : (y+1)*w*4]
		for x := 0; x < w*4; x += 4 {
			r, g, bl, a := uint32(row[x]), uint32(row[x+1]), uint32(row[x+2]), uint32(row[x+3])
			switch a {
			case 0:
				out[x], out[x+1], out[x+2], out[x+3] = 0, 0, 0, 0
			case 255:
				out[x], out[x+1], out[x+2], out[x+3] = byte(bl), byte(g), byte(r), 255
			default:
				out[x] = byte((bl*a + 127) / 255)
				out[x+1] = byte((g*a + 127) / 255)
				out[x+2] = byte((r*a + 127) / 255)
				out[x+3] = byte(a)
			}
		}
	}
}
