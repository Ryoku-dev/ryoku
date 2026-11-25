package teams

import (
	"bytes"
	"compress/flate"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// apiToken is the credential value no agent may ever read.
const apiToken = "tok_9f8e7d6c5b4a39281706f5e4d3c2b1a0"

// bigHead and bigTail surround the Authorization echoed in an answer longer
// than an agent is ever shown.
var bigHead, bigTail = strings.Repeat("a", 70<<10), strings.Repeat("b", 10<<10)

// apiServer is a server on this computer that echoes the Authorization it
// got in its body and Link header, between bigHead and bigTail on /big,
// redirects /move to /moved and records what reached each path.
type apiServer struct {
	*httptest.Server
	mu   sync.Mutex
	hits map[string][]string
}

func newAPIServer(t *testing.T) *apiServer {
	t.Helper()
	a := &apiServer{hits: make(map[string][]string)}
	a.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		a.mu.Lock()
		a.hits[r.URL.Path] = append(a.hits[r.URL.Path], auth)
		a.mu.Unlock()
		if r.URL.Path == "/move" {
			w.Header().Set("Location", "/moved?echo="+url.QueryEscape(auth))
			w.WriteHeader(http.StatusFound)
			return
		}
		w.Header().Set("Link", "<"+auth+">")
		if r.URL.Path == "/big" {
			fmt.Fprintf(w, "%s you sent %s %s", bigHead, auth, bigTail)
			return
		}
		fmt.Fprintf(w, "you sent %s", auth)
	}))
	t.Cleanup(a.Close)
	return a
}

// auths lists the Authorization of every request that reached path.
func (a *apiServer) auths(path string) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.hits[path]...)
}

func (a *apiServer) total() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	total := 0
	for _, hits := range a.hits {
		total += len(hits)
	}
	return total
}

// host is the server as credential hosts name it: 127.0.0.1:port.
func (a *apiServer) host() string {
	u, _ := url.Parse(a.URL)
	return u.Host
}

func (a *apiServer) port() string {
	u, _ := url.Parse(a.URL)
	return u.Port()
}

// addToken puts API_TOKEN in the vault for hosts, granted to teams.
func addToken(t *testing.T, h *harness, hosts []string, teams ...protocol.Team) {
	t.Helper()
	var ids []string
	for _, team := range teams {
		ids = append(ids, team.ID)
	}
	do[protocol.Credential](t, h, protocol.MVaultAdd, protocol.VaultAddParams{
		Name: "API_TOKEN", Use: protocol.CredentialHeader, Prefix: "Bearer ", Hosts: hosts, Teams: ids, Value: apiToken,
	})
}

func httpCall(ref protocol.AgentRef, rawURL string) protocol.AgentHTTPParams {
	return protocol.AgentHTTPParams{AgentRef: ref, Method: http.MethodGet, URL: rawURL, Credential: "API_TOKEN"}
}

func TestACredentialRequestWaitsForTheUserAndRunsOnceWithTheValueHidden(t *testing.T) {
	h := newHarness(t, 0)
	api := newAPIServer(t)
	team := h.newTeam(t, "Ops", nil)
	addToken(t, h, []string{api.host()}, team)
	ref := h.agent(t, h.newTask(t, team, "Check the API.").ID)

	waiting := do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, httpCall(ref, api.URL+"/data"))
	if waiting.Waiting == "" || api.total() != 0 {
		t.Fatalf("a first use went out before the user allowed it: %+v", waiting)
	}
	asked := h.pending(t)
	if asked.Kind != protocol.ApprovalCredential {
		t.Fatalf("the request waits on a %q approval", asked.Kind)
	}

	decided := do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce})
	if got := api.auths("/data"); len(got) != 1 || got[0] != "Bearer "+apiToken {
		t.Fatalf("the server got Authorization %q, want the credential once", got)
	}
	waitFor(t, "the agent to hear the answer", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	if note := h.sessions.notes(ref.Session)[0]; !strings.Contains(note, "you sent ‹API_TOKEN›") {
		t.Fatalf("the agent was told %q, want the response with the value named", note)
	}

	refused(t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce}, protocol.CodeConflict)
	if api.total() != 1 {
		t.Fatalf("a second answer sent the request again: %d requests", api.total())
	}
	h.noSecret(t, apiToken, waiting, decided, h.task(t, asked.Task))
}

func TestAlwaysTrustsOneCredentialOnOneHostForOneTeam(t *testing.T) {
	h := newHarness(t, 0)
	api := newAPIServer(t)
	ops := h.newTeam(t, "Ops", nil)
	support := h.newTeam(t, "Support", nil)
	addToken(t, h, []string{api.host(), "localhost:" + api.port()}, ops, support)
	ref := h.agent(t, h.newTask(t, ops, "Check the API.").ID)

	do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, httpCall(ref, api.URL+"/data"))
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: h.pending(t).ID, Decision: protocol.DecideAlways})
	waitFor(t, "the agent to hear the answer", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })

	direct := do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, httpCall(ref, api.URL+"/data"))
	if direct.Waiting != "" || direct.Status != http.StatusOK || len(api.auths("/data")) != 2 {
		t.Fatalf("a trusted use asked again or did not go: %+v", direct)
	}
	if !strings.Contains(direct.Body, "‹API_TOKEN›") || !strings.Contains(direct.Headers["link"], "‹API_TOKEN›") {
		t.Fatalf("the response is %+v, want the value named in body and headers", direct)
	}

	otherHost := do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, httpCall(ref, "http://localhost:"+api.port()+"/data"))
	if otherHost.Waiting == "" {
		t.Fatal("trust for one host let the credential go to another without asking")
	}
	theirs := do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, httpCall(h.agent(t, h.newTask(t, support, "Check it too.").ID), api.URL+"/data"))
	if theirs.Waiting == "" {
		t.Fatal("trust for one team let another team send the credential without asking")
	}
	if api.total() != 2 {
		t.Fatalf("%d requests went out, want only the approved and the trusted one", api.total())
	}
	h.noSecret(t, apiToken, direct, otherHost, theirs)
}

func TestACredentialGoesNowhereItIsNotAllowed(t *testing.T) {
	h := newHarness(t, 0)
	api := newAPIServer(t)
	stranger := newAPIServer(t)
	ops := h.newTeam(t, "Ops", nil)
	outsiders := h.newTeam(t, "Outsiders", nil)
	addToken(t, h, []string{api.host()}, ops)
	opsRef := h.agent(t, h.newTask(t, ops, "Check the API.").ID)
	outsiderRef := h.agent(t, h.newTask(t, outsiders, "Check the API too.").ID)

	ownAuth := httpCall(opsRef, api.URL+"/data")
	ownAuth.Headers = map[string]string{"authorization": "Bearer guessed"}
	cases := []struct {
		name   string
		params protocol.AgentHTTPParams
		code   string
	}{
		{"a host it may not go to", httpCall(opsRef, stranger.URL+"/data"), protocol.CodeBadRequest},
		{"a team it is not granted to", httpCall(outsiderRef, api.URL+"/data"), protocol.CodeNotFound},
		{"an Authorization of the agent's own", ownAuth, protocol.CodeBadRequest},
		{"a credential not in the vault", protocol.AgentHTTPParams{AgentRef: opsRef, URL: api.URL + "/data", Credential: "OTHER_TOKEN"}, protocol.CodeNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			refused(t, h, protocol.MTeamsAgentHTTP, c.params, c.code)
		})
	}
	if api.total() != 0 || stranger.total() != 0 {
		t.Fatalf("a refused request went out: %d and %d requests", api.total(), stranger.total())
	}
	if waiting := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals; len(waiting) != 0 {
		t.Fatalf("a refused request asked the user: %+v", waiting)
	}
}

func TestARedirectIsReportedNotFollowed(t *testing.T) {
	h := newHarness(t, 0)
	api := newAPIServer(t)
	team := h.newTeam(t, "Ops", nil)
	addToken(t, h, []string{api.host()}, team)
	ref := h.agent(t, h.newTask(t, team, "Check the API.").ID)
	do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, httpCall(ref, api.URL+"/data"))
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: h.pending(t).ID, Decision: protocol.DecideAlways})

	moved := do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, httpCall(ref, api.URL+"/move"))
	if moved.Status != http.StatusFound || !strings.HasPrefix(moved.Headers["location"], "/moved") {
		t.Fatalf("a redirect answered %+v, want its status and Location", moved)
	}
	if got := api.auths("/moved"); len(got) != 0 {
		t.Fatalf("the redirect was followed with Authorization %q", got)
	}
	h.noSecret(t, apiToken, moved)
}

func TestTheApprovalShowsEveryHeaderValueAndTheWholeBody(t *testing.T) {
	h := newHarness(t, 0)
	api := newAPIServer(t)
	team := h.newTeam(t, "Ops", nil)
	addToken(t, h, []string{api.host()}, team)
	ref := h.agent(t, h.newTask(t, team, "Post the report.").ID)

	post := httpCall(ref, api.URL+"/data")
	post.Method = http.MethodPost
	post.Headers = map[string]string{"x-report-target": "the public archive"}
	post.Body = strings.Repeat("a", 3<<10) + " and the end of the body"
	do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, post)
	detail := h.pending(t).Detail
	if !strings.Contains(detail, "X-Report-Target: the public archive") || !strings.Contains(detail, post.Body) {
		t.Fatalf("the approval shows %q, want every header with its value and the whole body", detail)
	}

	long := post
	long.Body = strings.Repeat("a", maxEgressBody+1)
	refused(t, h, protocol.MTeamsAgentHTTP, long, protocol.CodeBadRequest)
	if api.total() != 0 {
		t.Fatalf("%d requests went out before the user approved any", api.total())
	}
}

func TestAnAgentMayNotChangeTheMethodOrHowTheAnswerComesBack(t *testing.T) {
	h := newHarness(t, 0)
	api := newAPIServer(t)
	team := h.newTeam(t, "Ops", nil)
	addToken(t, h, []string{api.host()}, team)
	ref := h.agent(t, h.newTask(t, team, "Check the API.").ID)
	for _, name := range []string{"X-HTTP-Method-Override", "x-http-method", "X-Method-Override", "Range", "If-Range", "Accept-Encoding", "TE"} {
		t.Run(name, func(t *testing.T) {
			call := httpCall(ref, api.URL+"/data")
			call.Headers = map[string]string{name: "x"}
			refused(t, h, protocol.MTeamsAgentHTTP, call, protocol.CodeBadRequest)
		})
	}
	if waiting := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals; len(waiting) != 0 || api.total() != 0 {
		t.Fatalf("a refused header asked the user or went out: %d approvals, %d requests", len(waiting), api.total())
	}
}

// encodedForms are a value in standard and URL-safe base64, padded and not.
func encodedForms(value string) []string {
	raw := []byte(value)
	return []string{
		base64.StdEncoding.EncodeToString(raw), base64.RawStdEncoding.EncodeToString(raw),
		base64.URLEncoding.EncodeToString(raw), base64.RawURLEncoding.EncodeToString(raw),
	}
}

// deflate compresses text the way a server answering Content-Encoding:
// deflate does.
func deflate(text string) []byte {
	var b bytes.Buffer
	w, _ := flate.NewWriter(&b, flate.BestCompression)
	_, _ = w.Write([]byte(text))
	_ = w.Close()
	return b.Bytes()
}

func TestEveryFormOfTheValueIsTakenOutOfTheAnswer(t *testing.T) {
	h := newHarness(t, 0)
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if r.URL.Path == "/deflate" {
			w.Header().Set("Content-Encoding", "deflate")
			_, _ = w.Write(deflate(auth))
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		quoted, _ := json.Marshal(auth)
		w.Header().Set("Link", "<"+base64.StdEncoding.EncodeToString([]byte(auth))+">")
		fmt.Fprintf(w, "%s %s %s %s", strings.Join(encodedForms(auth), " "), strings.Join(encodedForms(token), " "), url.PathEscape(auth), quoted)
	}))
	t.Cleanup(echo.Close)
	team := h.newTeam(t, "Ops", nil)
	addToken(t, h, []string{strings.TrimPrefix(echo.URL, "http://")}, team)
	ref := h.agent(t, h.newTask(t, team, "Check the API.").ID)
	do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, httpCall(ref, echo.URL+"/forms"))
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: h.pending(t).ID, Decision: protocol.DecideAlways})
	waitFor(t, "the agent to hear the answer", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })

	direct := do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, httpCall(ref, echo.URL+"/forms"))
	shown := direct.Body + " " + direct.Headers["link"] + " " + h.sessions.notes(ref.Session)[0]
	for _, form := range append(encodedForms("Bearer "+apiToken), encodedForms(apiToken)...) {
		if strings.Contains(shown, form) {
			t.Errorf("the answer shows the value as %s", form)
		}
	}
	if !strings.Contains(direct.Body, "‹API_TOKEN›") || !strings.Contains(direct.Headers["link"], "‹API_TOKEN›") {
		t.Fatalf("the answer is %+v, want the value named in body and headers", direct)
	}

	packed := do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, httpCall(ref, echo.URL+"/deflate"))
	if strings.Contains(packed.Body, string(deflate("Bearer "+apiToken))) {
		t.Fatal("a deflated answer, which scrubbing cannot read, was shown")
	}
	h.noSecret(t, apiToken, direct, packed)
}

func TestEgressGoesStraightToTheHostOverModernTLS(t *testing.T) {
	transport, ok := egressClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("egress uses %T, want a transport of its own rather than the shared default", egressClient.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("egress follows the environment's proxy, which would carry the credential through another host")
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Fatal("egress allows TLS older than 1.2")
	}
}

func TestAWholeAnswerIsSavedToTheTeamsFolderWithTheValueTakenOut(t *testing.T) {
	h := newHarness(t, 0)
	api := newAPIServer(t)
	team := h.newTeam(t, "Ops", nil)
	addToken(t, h, []string{api.host()}, team)
	ref := h.agent(t, h.newTask(t, team, "Review the issues.").ID)
	want := bigHead + " you sent ‹API_TOKEN› " + bigTail
	saved := filepath.Join(team.Folder, "reviews", "issues.json")

	call := httpCall(ref, api.URL+"/big")
	call.SaveAs = "reviews/issues.json"
	waiting := do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, call)
	if waiting.Waiting == "" || api.total() != 0 {
		t.Fatalf("a first use went out before the user allowed it: %+v", waiting)
	}
	asked := h.pending(t)
	if !strings.Contains(asked.Detail, "Saves the answer to reviews/issues.json") {
		t.Fatalf("the approval shows %q, want where the answer is saved", asked.Detail)
	}
	if _, err := os.Stat(saved); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("something was saved before the user approved: %v", err)
	}

	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideAlways})
	waitFor(t, "the agent to hear the answer", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	data, err := os.ReadFile(saved)
	if err != nil || string(data) != want {
		t.Fatalf("saved %d bytes (%v), want the whole answer of %d bytes with the value named", len(data), err, len(want))
	}
	info, err := os.Stat(saved)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the saved answer is %v (%v), want it readable only by the user", info, err)
	}
	note := h.sessions.notes(ref.Session)[0]
	if !strings.Contains(note, fmt.Sprintf("Saved %d bytes to reviews/issues.json.", len(want))) || len(note) > maxAgentResponse+1<<10 {
		t.Fatalf("the agent was told %d bytes, want where the answer went and only its start", len(note))
	}
	refused(t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce}, protocol.CodeConflict)
	if got := api.auths("/big"); len(got) != 1 {
		t.Fatalf("the approved request went out %d times, want once", len(got))
	}

	call.SaveAs = "reviews/again.json"
	direct := do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, call)
	if direct.Waiting != "" || direct.Status != http.StatusOK || !direct.Truncated || len(direct.Body) != maxAgentResponse || !strings.HasPrefix(want, direct.Body) {
		t.Fatalf("a trusted save showed %d bytes, truncated %v; want the first %d bytes of the answer", len(direct.Body), direct.Truncated, maxAgentResponse)
	}
	if direct.Saved != fmt.Sprintf("Saved %d bytes to reviews/again.json.", len(want)) {
		t.Fatalf("a trusted save says %q, want how much went where", direct.Saved)
	}
	if again, err := os.ReadFile(filepath.Join(team.Folder, "reviews", "again.json")); err != nil || string(again) != want {
		t.Fatalf("the trusted save holds %d bytes (%v), want the whole answer", len(again), err)
	}
	h.noSecret(t, apiToken, waiting, direct)
}

func TestASaveGoesNowhereOutsideTheTeamsFolder(t *testing.T) {
	h := newHarness(t, 0)
	api := newAPIServer(t)
	team := h.newTeam(t, "Ops", nil)
	addToken(t, h, []string{api.host()}, team)
	ref := h.agent(t, h.newTask(t, team, "Review the issues.").ID)
	outside := t.TempDir()
	if err := os.MkdirAll(team.Folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(team.Folder, "out")); err != nil {
		t.Fatal(err)
	}
	nothingOutside := func() {
		t.Helper()
		if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
			t.Fatalf("the folder a link points to holds %d entries (%v), want none", len(entries), err)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(team.Folder), "x")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("a save went next to the team's folder: %v", err)
		}
	}

	cases := []struct{ name, saveAs string }{
		{"a path out of the folder", "../x"},
		{"an absolute path", "/etc/x"},
		{"a link out of the folder", "out/x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			call := httpCall(ref, api.URL+"/data")
			call.SaveAs = c.saveAs
			refused(t, h, protocol.MTeamsAgentHTTP, call, protocol.CodeBadRequest)
		})
	}
	if waiting := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals; len(waiting) != 0 || api.total() != 0 {
		t.Fatalf("a refused save asked the user or went out: %d approvals, %d requests", len(waiting), api.total())
	}
	nothingOutside()

	// a link made while the user decides still cannot lead the save out
	call := httpCall(ref, api.URL+"/data")
	call.SaveAs = "later/x"
	do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, call)
	if err := os.Symlink(outside, filepath.Join(team.Folder, "later")); err != nil {
		t.Fatal(err)
	}
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: h.pending(t).ID, Decision: protocol.DecideOnce})
	waitFor(t, "the agent to hear the answer", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	if note := h.sessions.notes(ref.Session)[0]; !strings.Contains(note, "could not be saved to later/x") {
		t.Fatalf("the agent was told %q, want that the save failed", note)
	}
	nothingOutside()
}
