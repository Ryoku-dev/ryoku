package web

import (
	"context"
	"net/netip"
	"strings"
	"testing"

	"ryoku-rashin/internal/engine/tool"
)

func TestFetchRejectsPrivateAddressesAndCredentials(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "http://127.0.0.1/", "http://[::1]/", "http://10.4.5.6/", "http://100.64.1.1/", "https://user:secret@example.com/"} {
		_, err := fetchPublic(context.Background(), raw)
		if err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	for _, addr := range []string{"127.0.0.1", "::1", "10.0.0.7", "fe80::1", "100.64.0.1", "192.0.2.10", "198.18.0.1", "2001:db8::1"} {
		if publicAddress(netip.MustParseAddr(addr)) {
			t.Errorf("accepted %s", addr)
		}
	}
}
func TestFetchToolRejectsLocalURL(t *testing.T) {
	result := fetchTool{}.Run(context.Background(), tool.Call{Args: []byte(`{"url":"http://127.0.0.1/private"}`)}, tool.Env{})
	if !result.IsError || !strings.Contains(result.Text, "127.0.0.1") {
		t.Fatalf("result: %+v", result)
	}
}
func TestHTMLTextStripsScriptsAndKeepsText(t *testing.T) {
	got := htmlText("<html><style>secret</style><p>Hi &amp; welcome</p><script>malice()</script><p>News</p></html>")
	if strings.Contains(got, "secret") || strings.Contains(got, "malice") || !strings.Contains(got, "Hi & welcome") || !strings.Contains(got, "News") {
		t.Fatalf("rendered: %q", got)
	}
}
