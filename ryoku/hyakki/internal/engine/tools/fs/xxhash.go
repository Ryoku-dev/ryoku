package fs

import (
	"encoding/binary"
	"math/bits"
)

// xxHash32 primes (Yann Collet's XXH32 specification).
const (
	xxPrime1 uint32 = 2654435761
	xxPrime2 uint32 = 2246822519
	xxPrime3 uint32 = 3266489917
	xxPrime4 uint32 = 668265263
	xxPrime5 uint32 = 374761393
)

// xxhash32 is XXH32 of data with the given seed. A local copy keeps hashline
// tags byte-identical to omp's (crates/edit/src/store.rs) without a module.
func xxhash32(data []byte, seed uint32) uint32 {
	n := len(data)
	var h uint32
	p := data
	if n >= 16 {
		v1 := seed + xxPrime1 + xxPrime2
		v2 := seed + xxPrime2
		v3 := seed
		v4 := seed - xxPrime1
		for len(p) >= 16 {
			v1 = xxRound(v1, binary.LittleEndian.Uint32(p[0:]))
			v2 = xxRound(v2, binary.LittleEndian.Uint32(p[4:]))
			v3 = xxRound(v3, binary.LittleEndian.Uint32(p[8:]))
			v4 = xxRound(v4, binary.LittleEndian.Uint32(p[12:]))
			p = p[16:]
		}
		h = bits.RotateLeft32(v1, 1) + bits.RotateLeft32(v2, 7) + bits.RotateLeft32(v3, 12) + bits.RotateLeft32(v4, 18)
	} else {
		h = seed + xxPrime5
	}
	h += uint32(n)
	for len(p) >= 4 {
		h += binary.LittleEndian.Uint32(p) * xxPrime3
		h = bits.RotateLeft32(h, 17) * xxPrime4
		p = p[4:]
	}
	for _, b := range p {
		h += uint32(b) * xxPrime5
		h = bits.RotateLeft32(h, 11) * xxPrime1
	}
	h ^= h >> 15
	h *= xxPrime2
	h ^= h >> 13
	h *= xxPrime3
	h ^= h >> 16
	return h
}

func xxRound(acc, lane uint32) uint32 {
	acc += lane * xxPrime2
	acc = bits.RotateLeft32(acc, 13)
	return acc * xxPrime1
}
