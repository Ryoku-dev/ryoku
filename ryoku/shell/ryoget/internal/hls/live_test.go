//go:build live

package hls

import (
	"context"
	"testing"
)

func TestLiveVariants(t *testing.T) {
	vs, err := Variants(context.Background(), nil, "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		t.Logf("%dx%d %d bps %s %s", v.Width, v.Height, v.Bandwidth, v.Codecs, v.URL)
	}
}
