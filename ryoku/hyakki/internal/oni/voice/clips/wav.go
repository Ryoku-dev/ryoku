package clips

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Formats DecodeWAV reads (the WAVE format tags).
const (
	wavPCM        = 1
	wavFloat      = 3
	wavExtensible = 0xfffe
)

// DecodeWAV reads a RIFF WAVE buffer into mono float32 samples in [-1, 1]
// and its sample rate. It takes 8, 16, 24 and 32-bit integer PCM and 32-bit
// float; channels are averaged. A data chunk that claims more bytes than the
// buffer holds runs to the end of the buffer: a writer streaming to a pipe,
// such as espeak-ng --stdout, cannot know the size when it writes the header.
func DecodeWAV(b []byte) (pcm []float32, rateHz int, err error) {
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, 0, errors.New("wav: not a RIFF WAVE buffer")
	}
	var (
		format, channels, bits int
		haveFmt                bool
	)
	for off := 12; off+8 <= len(b); {
		id, size := string(b[off:off+4]), int(binary.LittleEndian.Uint32(b[off+4:off+8]))
		body := b[off+8:]
		switch id {
		case "fmt ":
			if size < 16 || size > len(body) {
				return nil, 0, errors.New("wav: short fmt chunk")
			}
			format = int(binary.LittleEndian.Uint16(body[0:2]))
			channels = int(binary.LittleEndian.Uint16(body[2:4]))
			rateHz = int(binary.LittleEndian.Uint32(body[4:8]))
			bits = int(binary.LittleEndian.Uint16(body[14:16]))
			if format == wavExtensible {
				if size < 40 {
					return nil, 0, errors.New("wav: short extensible fmt chunk")
				}
				format = int(binary.LittleEndian.Uint16(body[24:26]))
			}
			haveFmt = true
		case "data":
			if !haveFmt {
				return nil, 0, errors.New("wav: data before fmt")
			}
			if size > len(body) {
				size = len(body)
			}
			return samples(body[:size], format, channels, bits, rateHz)
		}
		if size > len(body) {
			break
		}
		off += 8 + size + size&1
	}
	return nil, 0, errors.New("wav: no data chunk")
}

func samples(data []byte, format, channels, bits, rateHz int) ([]float32, int, error) {
	if channels < 1 || channels > 8 {
		return nil, 0, fmt.Errorf("wav: %d channels", channels)
	}
	if rateHz < 1000 || rateHz > 384000 {
		return nil, 0, fmt.Errorf("wav: rate %d Hz", rateHz)
	}
	var read func([]byte) float64
	switch {
	case format == wavPCM && bits == 8:
		read = func(s []byte) float64 { return (float64(s[0]) - 128) / 128 }
	case format == wavPCM && bits == 16:
		read = func(s []byte) float64 { return float64(int16(binary.LittleEndian.Uint16(s))) / 32768 }
	case format == wavPCM && bits == 24:
		read = func(s []byte) float64 {
			v := int32(uint32(s[0])<<8|uint32(s[1])<<16|uint32(s[2])<<24) >> 8
			return float64(v) / (1 << 23)
		}
	case format == wavPCM && bits == 32:
		read = func(s []byte) float64 { return float64(int32(binary.LittleEndian.Uint32(s))) / (1 << 31) }
	case format == wavFloat && bits == 32:
		read = func(s []byte) float64 { return float64(math.Float32frombits(binary.LittleEndian.Uint32(s))) }
	default:
		return nil, 0, fmt.Errorf("wav: format %d with %d bits is not read", format, bits)
	}
	width := bits / 8
	frame := width * channels
	n := len(data) / frame
	out := make([]float32, n)
	for i := range out {
		var sum float64
		for c := range channels {
			sum += read(data[i*frame+c*width:])
		}
		v := sum / float64(channels)
		if math.IsNaN(v) {
			v = 0
		}
		out[i] = float32(max(-1, min(1, v)))
	}
	return out, rateHz, nil
}

// Resample converts mono samples from one rate to another by linear
// interpolation; going down, each output sample first averages the input it
// spans, so speech does not alias. Equal rates return pcm unchanged.
func Resample(pcm []float32, from, to int) []float32 {
	if from == to || len(pcm) == 0 || from <= 0 || to <= 0 {
		return pcm
	}
	n := int(math.Round(float64(len(pcm)) * float64(to) / float64(from)))
	out := make([]float32, n)
	step := float64(from) / float64(to)
	if step > 1 {
		for i := range out {
			lo := int(float64(i) * step)
			hi := min(len(pcm), max(lo+1, int(float64(i+1)*step)))
			var sum float64
			for _, v := range pcm[lo:hi] {
				sum += float64(v)
			}
			out[i] = float32(sum / float64(hi-lo))
		}
		return out
	}
	last := len(pcm) - 1
	for i := range out {
		x := float64(i) * step
		j := int(x)
		if j >= last {
			out[i] = pcm[last]
			continue
		}
		f := float32(x - float64(j))
		out[i] = pcm[j]*(1-f) + pcm[j+1]*f
	}
	return out
}
