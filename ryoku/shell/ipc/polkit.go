package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

// polkit.go is the PolicyKit1 authentication agent. It registers on the system
// bus, receives BeginAuthentication from polkitd, runs the PAM conversation
// through the setuid (or socket-activated) polkit-agent-helper-1, streams the
// prompt state to QML over the "polkit" state topic, and takes the typed secret
// and the cancel intent back over control calls. QML (RyokuPolkitDialog) only
// renders and forwards keystrokes; the D-Bus and PAM plumbing lives here, keeping
// the shell's state-in-daemon / render-in-QML split. Contract 13 sec 2b, 4, 6.
//
// The helper line protocol (verified against polkit src/polkitagent): the helper
// prints "PAM_PROMPT_ECHO_OFF <msg>" / "PAM_PROMPT_ECHO_ON <msg>" (each expects a
// response line on stdin), "PAM_TEXT_INFO <msg>", "PAM_ERROR_MSG <msg>", then a
// final "SUCCESS" or "FAILURE". Messages are g_strescape'd; we g_strcompress them
// back. On SUCCESS the helper itself (running privileged) has already told polkitd
// the authorization was gained, so the agent only reports the outcome.
//
// SAFETY: registration is gated behind RYOKU_POLKIT_AGENT=1. Only one agent may
// hold a session at a time, and the full PAM flow cannot be exercised without a
// live privileged action, so the daemon does not silently take the session's
// polkit slot. With the flag set it registers (and fails closed if another agent
// already holds the slot); without it, the topic and calls still exist but no
// agent is registered, so whatever handles polkit today keeps working.

const (
	polkitAuthorityName  = "org.freedesktop.PolicyKit1"
	polkitAuthorityPath  = "/org/freedesktop/PolicyKit1/Authority"
	polkitAuthorityIface = "org.freedesktop.PolicyKit1.Authority"
	polkitAgentIface     = "org.freedesktop.PolicyKit1.AuthenticationAgent"
	polkitAgentPath      = "/org/freedesktop/PolicyKit1/AuthenticationAgent"
	polkitErrorFailed    = "org.freedesktop.PolicyKit1.Error.Failed"

	polkitMaxAttempts = 3

	// literal strings preserved from the reference (contract 13 sec 6).
	polkitRetryError = "Authentication failed. Try again."
	polkitFailed     = "authentication failed"
	polkitCancelled  = "cancelled by user"
	polkitNoIdentity = "no unix-user identity found"
)

// polkitHelperPath is the setuid helper; a package var so tests can substitute a
// fake. The real path is fixed by the polkit install.
var polkitHelperPath = "/usr/lib/polkit-1/polkit-agent-helper-1"

// polkitHelperSocket is where systemd listens for polkit 126 and newer, which
// dropped setuid from the helper. Connecting spawns a privileged instance with
// that connection as its stdio (polkit-agent-helper.socket, Accept=yes). Running
// the binary ourselves with --socket-activated instead only produces an
// unprivileged helper that fails PAM before it can ask anything, so the socket
// has to be dialled when it is there. A package var so tests can point it away.
var polkitHelperSocket = "/run/polkit/agent-helper.socket"

// polkitHelperIO is one helper conversation, whichever way it was opened: the
// same line protocol either way, so converse does not care which it got.
type polkitHelperIO struct {
	w    io.WriteCloser
	r    io.Reader
	stop func()
}

// openPolkitHelper dials the socket when systemd is listening, else runs the
// setuid binary over pipes. Returns the transport and the first line to send.
func openPolkitHelper(username, cookie string) (*polkitHelperIO, string, error) {
	if _, err := os.Stat(polkitHelperSocket); err == nil {
		conn, err := net.Dial("unix", polkitHelperSocket)
		if err == nil {
			return &polkitHelperIO{w: conn, r: conn, stop: func() { _ = conn.Close() }},
				username + "\n" + cookie + "\n", nil
		}
		// Fall through: a listening socket we cannot reach is no better than none.
	}
	// No socket: run the binary. Setuid it can authenticate on its own; without
	// either it still gets the pre-126 argv, which is all an old install offers.
	argv, initial := polkitHelperCommand(username, cookie)
	cmd := exec.Command(argv[0], argv[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, "", err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, "", err
	}
	return &polkitHelperIO{w: stdin, r: stdout, stop: func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}}, initial, nil
}

// polkitIdentity is one entry of the a(sa{sv}) identities array polkitd sends.
type polkitIdentity struct {
	Kind    string
	Details map[string]dbus.Variant
}

// polkitSubject is the (sa{sv}) subject passed to RegisterAuthenticationAgent.
type polkitSubject struct {
	Kind    string
	Details map[string]dbus.Variant
}

// polkitFrame is the QML view of the current prompt.
type polkitFrame struct {
	Active  bool   `json:"active"`
	Message string `json:"message"`
	Info    string `json:"info"`
	Error   string `json:"error"`
	Prompt  string `json:"prompt"`
	Echo    bool   `json:"echo"`
	// Fingerprint narrates pam_fprintd's own PAM messages to the reader
	// animation: "" none, "scanning" a finger is being read, "fail" no match.
	Fingerprint string `json:"fingerprint"`
}

// polkitPrompt is one BeginAuthentication in flight. respCh carries the password
// from QML; cancelCh fires on QML cancel or a CancelAuthentication for the cookie.
type polkitPrompt struct {
	cookie   string
	respCh   chan string
	cancelCh chan struct{}
	once     sync.Once
}

func (p *polkitPrompt) cancel() { p.once.Do(func() { close(p.cancelCh) }) }

type polkitAgent struct {
	conn    *dbus.Conn
	topic   *stateTopic
	mu      sync.Mutex
	state   polkitFrame
	current *polkitPrompt
}

// startPolkit registers the polkit topic and control calls unconditionally, then
// registers the D-Bus agent only when RYOKU_POLKIT_AGENT=1 (see SAFETY above).
func (d *daemon) startPolkit() {
	a := &polkitAgent{topic: d.registerTopic("polkit")}
	d.polkit = a
	a.publish()

	d.registerCall("polkit.submit", func(raw json.RawMessage) (any, error) {
		var p struct {
			Password string `json:"password"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		a.submit(p.Password)
		return map[string]any{"ok": true}, nil
	})
	d.registerCall("polkit.cancel", func(json.RawMessage) (any, error) {
		a.cancelCurrent()
		return map[string]any{"ok": true}, nil
	})

	if os.Getenv("RYOKU_POLKIT_AGENT") != "1" {
		log.Printf("ryoku-shell: polkit agent not registered (set RYOKU_POLKIT_AGENT=1 to enable)")
		return
	}
	if _, err := os.Stat(polkitHelperPath); err != nil {
		log.Printf("ryoku-shell: polkit agent disabled: helper %s missing: %v", polkitHelperPath, err)
		return
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Printf("ryoku-shell: polkit agent disabled: %v", err)
		return
	}
	a.conn = conn
	if err := conn.Export(a, dbus.ObjectPath(polkitAgentPath), polkitAgentIface); err != nil {
		log.Printf("ryoku-shell: polkit agent export failed: %v", err)
		return
	}
	subject, err := polkitSessionSubject(conn)
	if err != nil {
		log.Printf("ryoku-shell: polkit agent disabled: no session subject: %v", err)
		return
	}
	locale := polkitLocale()
	call := conn.Object(polkitAuthorityName, dbus.ObjectPath(polkitAuthorityPath)).Call(
		polkitAuthorityIface+".RegisterAuthenticationAgent", 0, subject, locale, polkitAgentPath)
	if call.Err != nil {
		// Fails closed: another agent already holds the session's slot, so the
		// existing agent keeps working and Ryoku does not fight it.
		log.Printf("ryoku-shell: polkit agent not registered (slot held?): %v", call.Err)
		return
	}
	log.Printf("ryoku-shell: polkit agent registered")
}

// BeginAuthentication is polkitd asking the user to authenticate. It blocks (in
// its own goroutine) for the whole PAM conversation and returns nil on success or
// a Failed error on failure/cancel, matching the reference outcome strings.
func (a *polkitAgent) BeginAuthentication(actionID, message, iconName string, details map[string]string, cookie string, identities []polkitIdentity) *dbus.Error {
	username, ok := polkitPickUser(identities)
	if !ok {
		return dbus.NewError(polkitErrorFailed, []interface{}{polkitNoIdentity})
	}

	p := &polkitPrompt{cookie: cookie, respCh: make(chan string, 1), cancelCh: make(chan struct{})}
	a.mu.Lock()
	a.current = p
	a.state = polkitFrame{Active: true, Message: message}
	a.mu.Unlock()
	a.publish()

	defer func() {
		a.mu.Lock()
		if a.current == p {
			a.current = nil
			a.state = polkitFrame{}
		}
		a.mu.Unlock()
		a.publish()
	}()

	for attempt := range polkitMaxAttempts {
		ok, cancelled, err := a.converse(username, cookie, p)
		if cancelled {
			return dbus.NewError(polkitErrorFailed, []interface{}{polkitCancelled})
		}
		if err != nil {
			log.Printf("ryoku-shell: polkit conversation: %v", err)
		}
		if ok {
			return nil
		}
		if attempt < polkitMaxAttempts-1 {
			a.setError(polkitRetryError)
		}
	}
	return dbus.NewError(polkitErrorFailed, []interface{}{polkitFailed})
}

// CancelAuthentication is polkitd withdrawing the request for a cookie.
func (a *polkitAgent) CancelAuthentication(cookie string) *dbus.Error {
	a.mu.Lock()
	p := a.current
	a.mu.Unlock()
	if p != nil && p.cookie == cookie {
		p.cancel()
	}
	return nil
}

// converse runs one helper attempt. Returns (gained, cancelled, err).
func (a *polkitAgent) converse(username, cookie string, p *polkitPrompt) (bool, bool, error) {
	h, initial, err := openPolkitHelper(username, cookie)
	if err != nil {
		return false, false, err
	}
	stdin := h.w
	if _, err := io.WriteString(stdin, initial); err != nil {
		h.stop()
		return false, false, err
	}

	// Read the helper's stdout on its own goroutine so a prompt still on screen
	// never stops us from noticing the helper finishing. A prompt-racing PAM
	// module (pam-fprint-grosshack) leaves a password prompt pending while it
	// verifies a fingerprint from a second thread; on a touch PAM returns
	// PAM_SUCCESS and the helper exits 0 without ever reading that prompt. Reading
	// in the background lets the loop act on SUCCESS (or the helper closing) the
	// moment it arrives, instead of stalling until the user answers a prompt that
	// success has already made unnecessary (issue #237).
	lines := make(chan string)
	scanErr := make(chan error, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		sc := bufio.NewScanner(h.r)
		for sc.Scan() {
			select {
			case lines <- sc.Text():
			case <-done:
				return
			}
		}
		scanErr <- sc.Err()
		close(lines)
	}()

	gained := false
	awaiting := false // a prompt is on screen, waiting for the QML response
	a.setFingerprint("")
	for {
		// Only accept a QML response while a prompt is actually pending, so a
		// queued submit is never consumed before its prompt appears.
		var respCh chan string
		if awaiting {
			respCh = p.respCh
		}
		select {
		case line, ok := <-lines:
			if !ok {
				// The helper closed its stdout: exiting 0 after a SUCCESS is a
				// fingerprint (or password) win even with a prompt left pending.
				h.stop()
				return gained, false, <-scanErr
			}
			switch {
			case strings.HasPrefix(line, "PAM_PROMPT_ECHO_OFF "):
				a.setPrompt(unescapeGStr(strings.TrimPrefix(line, "PAM_PROMPT_ECHO_OFF ")), false)
				awaiting = true
			case strings.HasPrefix(line, "PAM_PROMPT_ECHO_ON "):
				a.setPrompt(unescapeGStr(strings.TrimPrefix(line, "PAM_PROMPT_ECHO_ON ")), true)
				awaiting = true
			case strings.HasPrefix(line, "PAM_TEXT_INFO "):
				msg := unescapeGStr(strings.TrimPrefix(line, "PAM_TEXT_INFO "))
				if mentionsFinger(msg) {
					a.setFingerprint("scanning")
				}
				a.setInfo(msg)
			case strings.HasPrefix(line, "PAM_ERROR_MSG "):
				msg := unescapeGStr(strings.TrimPrefix(line, "PAM_ERROR_MSG "))
				if mentionsFinger(msg) {
					a.setFingerprint("fail")
				}
				a.setError(msg)
			case line == "SUCCESS":
				gained = true
			case line == "FAILURE":
				gained = false
			default:
				// Unrecognized lines are surfaced as info, matching the
				// reference's tolerance, and logged for diagnosis.
				log.Printf("ryoku-shell: polkit helper said %q", line)
				a.setInfo(unescapeGStr(line))
			}
		case pw := <-respCh:
			awaiting = false
			// The helper may have already exited on a racing fingerprint win; a
			// broken pipe here is harmless, the closed stdout ends the loop.
			_, _ = io.WriteString(stdin, pw+"\n")
		case <-p.cancelCh:
			h.stop()
			return false, true, nil
		}
	}
}

// setPrompt publishes the prompt QML must render and answer over polkit.submit.
func (a *polkitAgent) setPrompt(prompt string, echo bool) {
	a.mu.Lock()
	a.state.Prompt = prompt
	a.state.Echo = echo
	a.mu.Unlock()
	a.publish()
}

func (a *polkitAgent) submit(pw string) {
	a.mu.Lock()
	p := a.current
	a.mu.Unlock()
	if p == nil {
		return
	}
	select {
	case p.respCh <- pw:
	default:
	}
}

func (a *polkitAgent) cancelCurrent() {
	a.mu.Lock()
	p := a.current
	a.mu.Unlock()
	if p != nil {
		p.cancel()
	}
}

func (a *polkitAgent) setError(msg string) {
	a.mu.Lock()
	a.state.Error = msg
	a.state.Prompt = ""
	a.mu.Unlock()
	a.publish()
}

func (a *polkitAgent) setInfo(msg string) {
	a.mu.Lock()
	a.state.Info = msg
	a.mu.Unlock()
	a.publish()
}

func (a *polkitAgent) setFingerprint(state string) {
	a.mu.Lock()
	a.state.Fingerprint = state
	a.mu.Unlock()
	a.publish()
}

// mentionsFinger spots pam_fprintd's narration ("place your finger", "swipe
// your finger", "failed to match fingerprint") so the reader animation keys off
// the live PAM stream rather than guessing whether fprintd is in the stack.
func mentionsFinger(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "finger") || strings.Contains(m, "swipe")
}

func (a *polkitAgent) publish() {
	a.mu.Lock()
	frame, err := json.Marshal(a.state)
	a.mu.Unlock()
	if err != nil || a.topic == nil {
		return
	}
	a.topic.publish(frame)
}

// polkitHelperCommand returns the helper argv and the bytes to write to its stdin
// first. A setuid helper takes the username as argv[1] and the cookie on stdin; a
// non-setuid (socket-activated) helper takes --socket-activated and reads the
// username then the cookie from stdin (contract 13 sec 9; polkit read_cookie).
func polkitHelperCommand(username, cookie string) ([]string, string) {
	if polkitHelperIsSetuid(polkitHelperPath) {
		return []string{polkitHelperPath, username}, cookie + "\n"
	}
	return []string{polkitHelperPath, "--socket-activated"}, username + "\n" + cookie + "\n"
}

func polkitHelperIsSetuid(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSetuid != 0
}

// polkitPickUser returns the login name of the first unix-user identity, matching
// the reference's uid -> getpwuid resolution.
func polkitPickUser(identities []polkitIdentity) (string, bool) {
	for _, id := range identities {
		if id.Kind != "unix-user" {
			continue
		}
		v, ok := id.Details["uid"]
		if !ok {
			continue
		}
		uid, ok := v.Value().(uint32)
		if !ok {
			continue
		}
		u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
		if err != nil {
			continue
		}
		return u.Username, true
	}
	return "", false
}

// polkitSessionSubject builds the unix-session subject for this process, taking
// the session id from XDG_SESSION_ID, falling back to logind GetSessionByPID.
func polkitSessionSubject(conn *dbus.Conn) (polkitSubject, error) {
	id := os.Getenv("XDG_SESSION_ID")
	if id == "" {
		id = seatSessionForUser(conn)
	}
	if id == "" {
		var path dbus.ObjectPath
		err := conn.Object("org.freedesktop.login1", "/org/freedesktop/login1").Call(
			"org.freedesktop.login1.Manager.GetSessionByPID", 0, uint32(os.Getpid())).Store(&path)
		if err != nil {
			return polkitSubject{}, err
		}
		v, err := conn.Object("org.freedesktop.login1", path).GetProperty("org.freedesktop.login1.Session.Id")
		if err != nil {
			return polkitSubject{}, err
		}
		if s, ok := v.Value().(string); ok {
			id = s
		}
	}
	if id == "" {
		return polkitSubject{}, fmt.Errorf("no session id")
	}
	return polkitSubject{
		Kind:    "unix-session",
		Details: map[string]dbus.Variant{"session-id": dbus.MakeVariant(id)},
	}, nil
}

func polkitLocale() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return "en_US.UTF-8"
}

// unescapeGStr reverses g_strescape: the helper escapes its PAM messages before
// writing them, so \n \t \r \b \f \v \\ \" and octal \NNN come back to bytes.
func unescapeGStr(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch n := s[i]; n {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		case '0', '1', '2', '3', '4', '5', '6', '7':
			// up to three octal digits
			oct := string(n)
			for len(oct) < 3 && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7' {
				i++
				oct += string(s[i])
			}
			if v, err := strconv.ParseUint(oct, 8, 8); err == nil {
				b.WriteByte(byte(v))
			}
		default:
			b.WriteByte(n)
		}
	}
	return b.String()
}

// seatSessionForUser finds this user's seated session. Under the systemd user
// manager the daemon sits outside the login session, so GetSessionByPID finds
// nothing and the agent would register for no one; logind still knows which
// session has the seat.
func seatSessionForUser(conn *dbus.Conn) string {
	var sessions []struct {
		ID   string
		UID  uint32
		User string
		Seat string
		Path dbus.ObjectPath
	}
	err := conn.Object("org.freedesktop.login1", "/org/freedesktop/login1").Call(
		"org.freedesktop.login1.Manager.ListSessions", 0).Store(&sessions)
	if err != nil {
		return ""
	}
	uid := uint32(os.Getuid())
	for _, s := range sessions {
		if s.UID == uid && s.Seat != "" {
			return s.ID
		}
	}
	return ""
}
