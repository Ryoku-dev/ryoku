// Package hostrule says where a credential may go: how hosts are written and
// read, which request hosts a pattern takes, and which names reach only this
// computer. The daemon's vault and the app read hosts through it alike.
package hostrule

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// MaxHosts is how many hosts one credential may go to.
const MaxHosts = 32

var labelRule = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// Split reads hosts as a person types them: commas, semicolons or white
// space between them, lower case, each once. It checks nothing.
func Split(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})
	hosts := make([]string, 0, len(fields))
	for _, host := range fields {
		if !slices.Contains(hosts, host) {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

// Check normalises where a credential may go: lower-case DNS names with an
// optional :port, or *.example.org for every name under one domain. A name
// without a port means the scheme's own port.
func Check(hosts []string) ([]string, error) {
	var out []string
	for _, raw := range hosts {
		host := strings.ToLower(strings.TrimSpace(raw))
		if host == "" {
			continue
		}
		if !Valid(host) {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s",
				i18n.Tf("%q is not a host: write a name such as api.github.com, with :port when it needs one, or *.example.org.", raw))
		}
		if !slices.Contains(out, host) {
			out = append(out, host)
		}
	}
	if len(out) > MaxHosts {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A credential may go to at most %d hosts.", MaxHosts))
	}
	return out, nil
}

// Valid reports a lower-case DNS name, with an optional :port and an
// optional leading *. for every name under a domain.
func Valid(host string) bool {
	name, port, hasPort := strings.Cut(host, ":")
	if hasPort {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
			return false
		}
	}
	wildcard := strings.HasPrefix(name, "*.")
	if wildcard {
		name = strings.TrimPrefix(name, "*.")
	}
	labels := strings.Split(name, ".")
	if len(name) > 253 || (wildcard && len(labels) < 2) {
		return false
	}
	for _, label := range labels {
		if !labelRule.MatchString(label) {
			return false
		}
	}
	return true
}

// Allows reports whether a request host, as hostname with :port only when
// the port is not the scheme's own, may receive a credential limited to
// patterns. A *.example.org pattern takes every name under example.org but
// not example.org itself.
func Allows(patterns []string, host string) bool {
	host = strings.ToLower(host)
	for _, pattern := range patterns {
		if pattern == host {
			return true
		}
		suffix, wildcard := strings.CutPrefix(pattern, "*")
		if !wildcard {
			continue
		}
		patternName, patternPort, _ := strings.Cut(suffix, ":")
		hostName, hostPort, _ := strings.Cut(host, ":")
		if patternPort == hostPort && strings.HasSuffix(hostName, patternName) && len(hostName) > len(patternName) {
			return true
		}
	}
	return false
}

// IsLocal reports a name that reaches only this computer.
func IsLocal(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
