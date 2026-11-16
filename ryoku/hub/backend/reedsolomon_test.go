package main

import (
	"bytes"
	"math/rand"
	"testing"
)

// Any nsym/2 bytes misread anywhere are repaired; one more is refused, never misread
// into a different message that passes.
func TestReedSolomonRepairsUpToHalfItsParity(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := range 400 {
		nsym := 8 + 2*(trial%9)
		msg := make([]byte, 10+rng.Intn(70))
		rng.Read(msg)
		code := rsEncode(msg, nsym)
		bad := append([]byte(nil), code...)
		for _, p := range rng.Perm(len(bad))[:nsym/2] {
			bad[p] ^= byte(1 + rng.Intn(255))
		}
		got, err := rsDecode(bad, nsym)
		if err != nil || !bytes.Equal(got, msg) {
			t.Fatalf("trial %d: %d errors in %d parity not repaired: %v", trial, nsym/2, nsym, err)
		}
	}
}
