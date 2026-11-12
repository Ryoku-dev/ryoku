package backend

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// Bare drops the request address from a transport error. The address may
// hold what was searched for or remembered, which must not reach a log or
// the app.
func Bare(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

// LeavesMachine reports whether an engine at address is off this computer,
// so what it is handed leaves it. Anything that is not plainly loopback
// counts as leaving.
func LeavesMachine(address string) bool {
	u, err := url.Parse(address)
	if err != nil {
		return true
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}
