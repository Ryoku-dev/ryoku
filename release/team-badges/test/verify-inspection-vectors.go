// Standalone cross-language vector checker; go run this file from repository root.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func main() {
	path := "release/team-badges/test/inspection-vectors.json"
	if len(os.Args) == 2 {
		path = os.Args[1]
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var fixture struct {
		Vectors []struct {
			Domain       string          `json:"domain"`
			PublicKey    string          `json:"publicKey"`
			Payload      json.RawMessage `json:"payload"`
			CanonicalHex string          `json:"canonicalHex"`
			PayloadSHA   string          `json:"payloadSha"`
			MessageHex   string          `json:"messageHex"`
			Envelope     struct {
				Payload string `json:"payload"`
				Sig     string `json:"sig"`
			} `json:"envelope"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		panic(err)
	}
	for _, vector := range fixture.Vectors {
		decoder := json.NewDecoder(bytes.NewReader(vector.Payload))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			panic(err)
		}
		var output bytes.Buffer
		encoder := json.NewEncoder(&output)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(value); err != nil {
			panic(err)
		}
		canonical := bytes.TrimSuffix(output.Bytes(), []byte("\n"))
		require(hex.EncodeToString(canonical) == vector.CanonicalHex, "canonical bytes differ")
		digest := sha256.Sum256(canonical)
		require(hex.EncodeToString(digest[:]) == vector.PayloadSHA, "payload digest differs")
		message := append(append([]byte(vector.Domain), 0), canonical...)
		require(hex.EncodeToString(message) == vector.MessageHex, "domain bytes differ")
		payload, err := base64.StdEncoding.Strict().DecodeString(vector.Envelope.Payload)
		if err != nil {
			panic(err)
		}
		require(bytes.Equal(payload, canonical), "envelope bytes differ")
		public, err := base64.StdEncoding.Strict().DecodeString(vector.PublicKey)
		if err != nil {
			panic(err)
		}
		signature, err := base64.StdEncoding.Strict().DecodeString(vector.Envelope.Sig)
		if err != nil {
			panic(err)
		}
		require(ed25519.Verify(public, message, signature), "signature differs")
	}
	fmt.Printf("Verified %d Go canonical/signature vectors\n", len(fixture.Vectors))
}
