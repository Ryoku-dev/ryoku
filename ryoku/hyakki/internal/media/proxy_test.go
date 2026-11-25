package media

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVideoRejectsNonPublicLinksBeforeStartingExtractor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	marker := filepath.Join(dir, "extractor-started")
	script := "#!/bin/sh\nprintf started > '" + marker + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "yt-dlp"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		"http://127.0.0.1/video", "http://localhost/video", "http://10.0.0.1/video",
		"http://169.254.169.254/latest/meta-data", "http://[::1]/video", "http://[::ffff:127.0.0.1]/video",
	} {
		t.Run(raw, func(t *testing.T) {
			accepted, err := AcceptsVideo(context.Background(), raw)
			if accepted || err == nil || !strings.Contains(err.Error(), "public internet address") {
				t.Fatalf("AcceptsVideo(%q) = %v, %v", raw, accepted, err)
			}
			if _, err := Video(context.Background(), raw, "en", func(string) {}); err == nil || !strings.Contains(err.Error(), "public internet address") {
				t.Fatalf("Video(%q) did not refuse the address: %v", raw, err)
			}
		})
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("extractor ran for a private destination: %v", err)
	}
}

func TestVideoProxyRefusesPrivateCONNECT(t *testing.T) {
	proxy, err := newVideoProxy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	parsed, err := url.Parse(proxy.url)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", parsed.Host, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprint(conn, "CONNECT 127.0.0.1:443 HTTP/1.1\r\nHost: 127.0.0.1:443\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusForbidden || !strings.Contains(string(body), "Private source addresses") {
		t.Fatalf("CONNECT was not rejected by the public-address guard: %d, %q, %v", response.StatusCode, body, err)
	}
}

func TestVideoProxyRefusesPrivateHTTPDestination(t *testing.T) {
	proxy, err := newVideoProxy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	parsed, err := url.Parse(proxy.url)
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(parsed)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	response, err := client.Get("http://169.254.169.254/latest/meta-data")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusForbidden || !strings.Contains(string(body), "Private source addresses") {
		t.Fatalf("HTTP destination was not rejected: %d, %q, %v", response.StatusCode, body, err)
	}
}

func TestVideoProbeUsesProxyAndRemovesBypassEnvironment(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("NO_PROXY", "*")
	t.Setenv("no_proxy", "*")
	script := "#!/bin/sh\n[ -z \"$NO_PROXY\" ] && [ -z \"$no_proxy\" ] || exit 1\nwhile [ \"$#\" -gt 0 ]; do\nif [ \"$1\" = --proxy ]; then shift; proxy=$1; fi\nshift\ndone\ncase \"$proxy\" in http://127.0.0.1:*) exit 0;; *) exit 2;; esac\n"
	if err := os.WriteFile(filepath.Join(dir, "yt-dlp"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	accepted, err := AcceptsVideo(context.Background(), "https://93.184.216.34/video")
	if err != nil || !accepted {
		t.Fatalf("guarded probe = %v, %v", accepted, err)
	}
}
