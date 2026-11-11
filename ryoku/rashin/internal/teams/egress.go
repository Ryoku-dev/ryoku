package teams

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/hostrule"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

// Limits on a request made with a credential and on what the agent is shown.
const (
	// maxEgressBody is the longest body the approval card shows whole; a
	// longer one is refused, so the user approves every byte that goes
	maxEgressBody     = 8 << 10
	maxEgressResponse = 64 << 10
	maxAgentResponse  = 16 << 10
	// maxSavedResponse is the most of an answer saved to the team's folder
	maxSavedResponse = 8 << 20
	// egressSlack is read past the shown body, the longest form of a value
	// with its prefix (16 KiB escaped in JSON, at most six bytes a byte), so
	// a value cut by the limit is dropped rather than shown in part
	egressSlack   = 128 << 10
	egressTimeout = 30 * time.Second
)

// egressMethods are the methods an agent may use with a credential.
var egressMethods = []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

// keptHeaders are the response headers the agent is shown.
var keptHeaders = []string{"Content-Type", "Content-Length", "Location", "Link", "X-Ratelimit-Remaining"}

// refusedHeaders are request headers an agent may not set: the ones that
// carry credentials and the ones that frame the request.
var refusedHeaders = []string{"Authorization", "Cookie", "Proxy-Authorization", "Host", "Content-Length", "Transfer-Encoding", "Connection"}

// shapingHeaders are request headers an agent may not set because they
// change what the user approved or how the answer comes back: a method
// override runs another method than the card shows, and a range or an
// encoding returns the body in slices or compressed, where scrubbing cannot
// find a value.
var shapingHeaders = []string{"X-Http-Method-Override", "X-Http-Method", "X-Method-Override", "Range", "If-Range", "Accept-Encoding", "Te"}

var headerName = regexp.MustCompile("^[A-Za-z0-9!#$%&'*+.^_`|~-]{1,64}$")

// egressClient never follows a redirect: a credential goes only to the host
// it was approved for, so the agent is told the status and Location instead.
// Its transport ignores the proxy settings of the environment, which would
// send the request and its credential through another host.
var egressClient = &http.Client{
	Timeout: egressTimeout,
	Transport: &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: egressTimeout,
		ExpectContinueTimeout: time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          16,
		ForceAttemptHTTP2:     true,
	},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// egressRequest is the exact request an agent asked for, with where its
// answer is saved. An approval stores it, so what the user approved is what
// runs.
type egressRequest struct {
	Credential string            `json:"credential"`
	Name       string            `json:"name"`
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Host       string            `json:"host"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	SaveAs     string            `json:"saveAs,omitempty"`
}

// requestHost names a URL's host the way credential hosts are written: the
// name, with :port only when it is not the scheme's own.
func requestHost(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" || (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		return host
	}
	return host + ":" + port
}

// newEgress checks what an agent asked for against the rules every request
// with a credential follows. Where the credential may go is checked by the
// caller, which records a refusal.
func newEgress(c protocol.Credential, p protocol.AgentHTTPParams) (egressRequest, error) {
	u, err := url.Parse(strings.TrimSpace(p.URL))
	if err != nil || u.Host == "" {
		return egressRequest{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%q is not a URL.", p.URL))
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return egressRequest{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A credential goes only over https, or over http to this computer."))
	}
	if u.User != nil {
		return egressRequest{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The URL may not carry a user name or password."))
	}
	method := strings.ToUpper(strings.TrimSpace(p.Method))
	if method == "" {
		method = http.MethodGet
	}
	if !slices.Contains(egressMethods, method) {
		return egressRequest{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%q is not a method you may use: use GET, HEAD, POST, PUT, PATCH or DELETE.", p.Method))
	}
	headers := map[string]string{}
	for name, value := range p.Headers {
		name = strings.TrimSpace(name)
		if !headerName.MatchString(name) || strings.ContainsAny(value, "\r\n\x00") {
			return egressRequest{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The header %q is not valid.", name))
		}
		canonical := textproto.CanonicalMIMEHeaderKey(name)
		if strings.EqualFold(canonical, c.Header) || slices.Contains(refusedHeaders, canonical) {
			return egressRequest{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("You may not set %s: Rashin sets it, or it would carry a credential.", canonical))
		}
		if slices.Contains(shapingHeaders, canonical) {
			return egressRequest{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("You may not set %s: it would change the method the user approves or how the answer comes back.", canonical))
		}
		headers[canonical] = value
	}
	if len(p.Body) > maxEgressBody {
		return egressRequest{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The body is longer than 8 KiB, the most the user is shown whole before approving. Send a shorter body."))
	}
	saveAs, err := saveAsPath(p.SaveAs)
	if err != nil {
		return egressRequest{}, err
	}
	return egressRequest{
		Credential: c.ID, Name: c.Name, Method: method, URL: u.String(), Host: requestHost(u),
		Headers: headers, Body: p.Body, SaveAs: saveAs,
	}, nil
}

// saveAsPath checks where an agent asked its answer saved: a file named
// relative to the team's folder, with no .. on the way. It returns the path
// cleaned, or "" when the answer is not saved.
func saveAsPath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", nil
	}
	clean := filepath.Clean(path)
	if !filepath.IsLocal(path) || slices.Contains(strings.Split(path, "/"), "..") || strings.HasSuffix(path, "/") || filepath.Base(clean) == "." {
		return "", protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%q is not a file in your team's folder: give a path relative to the folder, without .., that ends in a file name.", raw))
	}
	return clean, nil
}

// detail shows the user exactly what would be sent: the request line, every
// header the agent set with its value, and the whole body, then where the
// answer is saved. The caller redacts it.
func (r egressRequest) detail() string {
	var b strings.Builder
	b.WriteString(r.Method + " " + r.URL)
	names := make([]string, 0, len(r.Headers))
	for name := range r.Headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		b.WriteString("\n" + name + ": " + r.Headers[name])
	}
	if r.Body != "" {
		b.WriteString("\n\n" + r.Body)
	}
	if r.SaveAs != "" {
		b.WriteString("\n\n" + i18n.Tf("Saves the answer to %s", r.SaveAs))
	}
	return b.String()
}

// checkSaveAs refuses, before anything is sent, a save the team's folder
// cannot take. A folder not made yet has nothing in the way, and saving
// makes it.
func checkSaveAs(folder, path string) error {
	root, err := os.OpenRoot(folder)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err == nil {
		err = checkSaveTarget(root, path)
		_ = root.Close()
	}
	if err != nil {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Rashin cannot save to %s: %v.", path, err))
	}
	return nil
}

// checkSaveTarget refuses a path in the folder that cannot take a saved
// answer: one a link leads out of the folder, one under a file, and one that
// is a folder or a link itself.
func checkSaveTarget(root *os.Root, path string) error {
	info, err := root.Lstat(path)
	switch {
	case err == nil && !info.Mode().IsRegular():
		return errors.New(i18n.T("it is a folder or a link, not a file"))
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return err
	}
	return nil
}

// saveAnswer writes an answer to path in the team's folder, readable only by
// the user. It goes through the folder opened as a root, so no link leads
// the write out of it, and by way of a temporary file renamed into place, so
// a failed save leaves nothing behind.
func saveAnswer(folder, path string, data []byte) error {
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return err
	}
	root, err := os.OpenRoot(folder)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := checkSaveTarget(root, path); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := root.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := filepath.Join(dir, "."+newID("save")+".tmp")
	file, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = root.Rename(tmp, path)
	}
	if err != nil {
		_ = root.Remove(tmp)
	}
	return err
}

// performRequest makes an approved or trusted request: it checks the
// credential still may go there for the team, adds the value itself, and
// returns the response with every value taken out.
func (s *Service) performRequest(ctx context.Context, team protocol.Team, task, actor, initiator string, r egressRequest) (protocol.AgentHTTPResult, error) {
	c, ok := s.vault.Get(r.Credential)
	if !ok || c.Use != protocol.CredentialHeader {
		return protocol.AgentHTTPResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("%s is no longer in the vault.", r.Name))
	}
	if !slices.Contains(c.Teams, team.ID) || !hostrule.Allows(c.Hosts, r.Host) {
		s.recordCredential(protocol.VaultEvent{Credential: c.ID, Name: c.Name, Team: team.ID, Task: task, Host: r.Host, Outcome: protocol.VaultRefused, Detail: i18n.T("It may no longer go there for this team.")})
		return protocol.AgentHTTPResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s may no longer go to %s for your team.", c.Name, r.Host))
	}
	value, err := s.vault.Reveal(c.ID)
	if err != nil {
		return protocol.AgentHTTPResult{}, err
	}
	scrub := func(text string) string {
		if c.Prefix != "" {
			text = vault.Scrub(text, c.Prefix+value, c.Name)
		}
		return s.redact(vault.Scrub(text, value, c.Name))
	}
	ctx, cancel := context.WithTimeout(ctx, egressTimeout)
	defer cancel()
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, body)
	if err != nil {
		return protocol.AgentHTTPResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%q is not a URL.", r.URL))
	}
	for name, v := range r.Headers {
		req.Header.Set(name, v)
	}
	req.Header.Set(c.Header, c.Prefix+value)
	resp, err := egressClient.Do(req)
	if err != nil {
		reason := scrub(err.Error())
		s.recordCredential(protocol.VaultEvent{Credential: c.ID, Name: c.Name, Team: team.ID, Task: task, Host: r.Host, Outcome: protocol.VaultFailed, Detail: reason})
		s.record(protocol.Activity{Actor: actor, Action: "credential.use", Target: c.Name, OK: false, Team: team.ID, Task: task, Initiator: initiator, Detail: i18n.Tf("To %s: the request failed.", r.Host)})
		return protocol.AgentHTTPResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The request to %s failed: %s", r.Host, reason))
	}
	defer resp.Body.Close()
	limit := maxEgressResponse
	if r.SaveAs != "" {
		limit = maxSavedResponse
	}
	var text string
	var readErr error
	truncated, withheld := false, false
	if encoding := resp.Header.Get("Content-Encoding"); encoding != "" && !strings.EqualFold(encoding, "identity") {
		// the transport decodes only the gzip it asked for; any other encoding
		// would carry a value past scrubbing
		withheld = true
		text = i18n.Tf("(the body came encoded as %s, which Rashin cannot check for credentials, so it is not shown)", scrub(encoding))
		if r.SaveAs != "" {
			text = i18n.Tf("(the body came encoded as %s, which Rashin cannot check for credentials, so it is not shown or saved)", scrub(encoding))
		}
	} else {
		var raw []byte
		raw, readErr = io.ReadAll(io.LimitReader(resp.Body, int64(limit+egressSlack)))
		text = scrub(string(raw))
		if len(raw) == limit+egressSlack {
			// a value the read limit cut in two is in the slack, which is never shown
			text, truncated = cutBytes(text, max(len(text)-egressSlack, 0)), true
		}
		if len(text) > limit {
			text, truncated = cutBytes(text, limit), true
		}
	}
	result := protocol.AgentHTTPResult{Status: resp.StatusCode, Headers: map[string]string{}, Body: text, Truncated: truncated}
	for _, name := range keptHeaders {
		if v := resp.Header.Get(name); v != "" {
			result.Headers[strings.ToLower(name)] = scrub(v)
		}
	}
	s.recordCredential(protocol.VaultEvent{Credential: c.ID, Name: c.Name, Team: team.ID, Task: task, Host: r.Host, Outcome: protocol.VaultUsed, Detail: i18n.Tf("%s answered %d.", r.Method, resp.StatusCode)})
	s.record(protocol.Activity{Actor: actor, Action: "credential.use", Target: c.Name, OK: resp.StatusCode < 400, Team: team.ID, Task: task, Initiator: initiator, Detail: i18n.Tf("To %s: %s answered %d.", r.Host, r.Method, resp.StatusCode)})
	if r.SaveAs == "" || withheld {
		return result, nil
	}
	// a save is the whole answer or nothing
	if readErr != nil {
		return protocol.AgentHTTPResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The server answered %d, but the answer could not be read whole, so nothing was saved: %s", resp.StatusCode, scrub(readErr.Error())))
	}
	if err := saveAnswer(team.Folder, r.SaveAs, []byte(text)); err != nil {
		return protocol.AgentHTTPResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("The server answered %d, but the answer could not be saved to %s: %s", resp.StatusCode, r.SaveAs, scrub(err.Error())))
	}
	result.Saved = i18n.Tf("Saved %d bytes to %s.", len(text), r.SaveAs)
	if truncated {
		result.Saved = i18n.Tf("Saved %d bytes to %s; the answer went on past 8 MiB and the rest was not read.", len(text), r.SaveAs)
	}
	result.Body = cutBytes(text, maxAgentResponse)
	result.Truncated = truncated || len(text) > maxAgentResponse
	return result, nil
}

// httpAnswer tells an agent whose request waited on the user what came back.
func httpAnswer(result protocol.AgentHTTPResult) string {
	var b strings.Builder
	b.WriteString(i18n.Tf("The user approved. The server answered %d.", result.Status))
	if location := result.Headers["location"]; location != "" {
		b.WriteString(" " + i18n.Tf("It points to %s; Rashin does not follow it.", location))
	}
	if result.Saved != "" {
		b.WriteString(" " + result.Saved)
	}
	if result.Body != "" {
		b.WriteString("\n\n" + cutBytes(result.Body, maxAgentResponse))
		switch {
		case result.Saved != "" && result.Truncated:
			b.WriteString("\n" + i18n.T("(the response goes on in the saved file)"))
		case result.Truncated || len(result.Body) > maxAgentResponse:
			b.WriteString("\n" + i18n.T("(the response goes on; ask again for less if you need the rest)"))
		}
	}
	return b.String()
}

// cutBytes cuts text to at most n bytes without splitting a character.
func cutBytes(text string, n int) string {
	if len(text) <= n {
		return text
	}
	for n > 0 && !utf8.RuneStart(text[n]) {
		n--
	}
	return text[:n]
}
