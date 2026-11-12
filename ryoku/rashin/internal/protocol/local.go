package protocol

import (
	"net/netip"
	"net/url"
	"strings"
)

// Local providers: model servers on this computer or the local network that
// speak the OpenAI chat API. They need no key; Rashin finds their models by
// asking the server.
const (
	ProviderOllamaLocal = "ollama-local"
	ProviderLlamaCpp    = "llamacpp"
)

// LocalProviders are the local providers' ids in the order they are shown.
var LocalProviders = []string{ProviderOllamaLocal, ProviderLlamaCpp}

// DefaultLocalAddresses are where each local provider listens out of the box.
func DefaultLocalAddresses() map[string]string {
	return map[string]string{
		ProviderOllamaLocal: "http://127.0.0.1:11434/v1",
		ProviderLlamaCpp:    "http://127.0.0.1:8080/v1",
	}
}

// ParseLocalAddress normalises the address of a local provider's API and
// reports whether it may be used: http or https, no credentials, query or
// fragment, and a host that is localhost or a loopback or private-network IP
// address. A name other than localhost is refused, since what it resolves to
// can change. An address without a path gets /v1, the API's usual root.
func ParseLocalAddress(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", false
	}
	host := u.Hostname()
	if !strings.EqualFold(host, "localhost") {
		ip, err := netip.ParseAddr(host)
		if err != nil || !PrivateIP(ip) {
			return "", false
		}
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path == "" {
		u.Path = "/v1"
	}
	u.RawPath = ""
	return u.String(), true
}

// PrivateIP reports a loopback address or one on a private network (RFC 1918,
// RFC 4193). Link-local addresses are not private here: a cloud machine's
// metadata service lives there.
func PrivateIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsLoopback() || ip.IsPrivate()
}
