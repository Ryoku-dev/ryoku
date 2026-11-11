package hostrule

import (
	"slices"
	"testing"
)

func TestAllowsMatchesHostsExactlyAndBySuffix(t *testing.T) {
	patterns := []string{"api.github.com", "*.example.org", "localhost:8025"}
	for host, want := range map[string]bool{
		"api.github.com":      true,
		"API.GitHub.com":      true,
		"api.github.com:8443": false,
		"github.com":          false,
		"a.example.org":       true,
		"b.a.example.org":     true,
		"example.org":         false,
		"badexample.org":      false,
		"localhost:8025":      true,
		"localhost":           false,
	} {
		if Allows(patterns, host) != want {
			t.Errorf("Allows(%q) = %v, want %v", host, !want, want)
		}
	}
}

func TestSplitReadsHostsAsTyped(t *testing.T) {
	got := Split("API.github.com, *.example.org;localhost:8025  api.github.com\n")
	want := []string{"api.github.com", "*.example.org", "localhost:8025"}
	if !slices.Equal(got, want) {
		t.Fatalf("Split = %q, want %q", got, want)
	}
}

func TestCheckRefusesWhatIsNoHost(t *testing.T) {
	for _, host := range []string{"exa mple.org", "*.org", "a.org:0", "a.org:080", "-a.org"} {
		if _, err := Check([]string{host}); err == nil {
			t.Errorf("Check took %q", host)
		}
	}
	hosts, err := Check([]string{" A.org ", "a.org", "", "*.example.org:8443"})
	if err != nil || !slices.Equal(hosts, []string{"a.org", "*.example.org:8443"}) {
		t.Fatalf("Check = %q, %v", hosts, err)
	}
}

func TestIsLocalTakesOnlyThisComputer(t *testing.T) {
	for host, want := range map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true, "LocalHost": true, "mail.example.org": false, "10.0.0.1": false} {
		if IsLocal(host) != want {
			t.Errorf("IsLocal(%q) = %v, want %v", host, !want, want)
		}
	}
}
