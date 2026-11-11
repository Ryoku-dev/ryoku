package teams

import (
	"encoding/base64"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// mailPassword is the mail account's password no agent may ever read.
const mailPassword = "pw_correct_horse_battery_4471"

// smtpServer is a mail server on this computer that speaks just enough SMTP
// for one client: EHLO, AUTH PLAIN, MAIL, RCPT, DATA and QUIT. It records the
// password of every login, the recipients and the messages.
type smtpServer struct {
	ln         net.Listener
	mu         sync.Mutex
	sessions   int
	passwords  []string
	recipients []string
	messages   []string
	// echoes refuses every login, quoting the password back in base64 as a
	// careless server might
	echoes bool
}

func newSMTPServer(t *testing.T) *smtpServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &smtpServer{ln: ln}
	go srv.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return srv
}

func (srv *smtpServer) serve() {
	for {
		conn, err := srv.ln.Accept()
		if err != nil {
			return
		}
		go srv.session(conn)
	}
}

func (srv *smtpServer) session(conn net.Conn) {
	defer conn.Close()
	srv.mu.Lock()
	srv.sessions++
	srv.mu.Unlock()
	text := textproto.NewConn(conn)
	reply := func(lines ...string) bool {
		for _, line := range lines {
			if text.PrintfLine("%s", line) != nil {
				return false
			}
		}
		return true
	}
	if !reply("220 localhost ESMTP") {
		return
	}
	for {
		line, err := text.ReadLine()
		if err != nil {
			return
		}
		verb, arg, _ := strings.Cut(line, " ")
		ok := true
		switch strings.ToUpper(verb) {
		case "EHLO", "HELO":
			ok = reply("250-localhost", "250 AUTH PLAIN")
		case "AUTH":
			mechanism, payload, _ := strings.Cut(arg, " ")
			raw, err := base64.StdEncoding.DecodeString(payload)
			parts := strings.Split(string(raw), "\x00")
			if !strings.EqualFold(mechanism, "PLAIN") || err != nil || len(parts) != 3 {
				ok = reply("535 not a login")
				break
			}
			srv.mu.Lock()
			srv.passwords = append(srv.passwords, parts[2])
			echoes := srv.echoes
			srv.mu.Unlock()
			if echoes {
				ok = reply("535 no login for " + base64.StdEncoding.EncodeToString([]byte(parts[2])))
				break
			}
			ok = reply("235 accepted")
		case "RCPT":
			srv.mu.Lock()
			srv.recipients = append(srv.recipients, strings.Trim(strings.TrimPrefix(arg, "TO:"), "<>"))
			srv.mu.Unlock()
			ok = reply("250 ok")
		case "DATA":
			if !reply("354 go on") {
				return
			}
			message, err := text.ReadDotBytes()
			if err != nil {
				return
			}
			srv.mu.Lock()
			srv.messages = append(srv.messages, string(message))
			srv.mu.Unlock()
			ok = reply("250 queued")
		case "QUIT":
			reply("221 bye")
			return
		default:
			ok = reply("250 ok")
		}
		if !ok {
			return
		}
	}
}

// seen returns what the server recorded.
func (srv *smtpServer) seen() (sessions int, passwords, recipients, messages []string) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return srv.sessions, append([]string(nil), srv.passwords...), append([]string(nil), srv.recipients...), append([]string(nil), srv.messages...)
}

func TestMailWaitsForTheUserThenLogsInWithTheVaultPassword(t *testing.T) {
	h := newHarness(t, 0)
	srv := newSMTPServer(t)
	team := h.newTeam(t, "Letters", func(p *protocol.TeamCreateParams) { p.Tools.Mail = true })
	h.mailAccount(t, srv)
	ref := h.agent(t, h.newTask(t, team, "Send the weekly notes to Ana.").ID)

	asked := do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentMail, protocol.AgentMailParams{
		AgentRef: ref, To: []string{"ana@example.org"}, Subject: "Weekly notes", Body: "Here are this week's notes.",
	})
	if sessions, _, _, _ := srv.seen(); sessions != 0 {
		t.Fatal("the mail went out before the user approved it")
	}
	approval := h.pending(t)
	if approval.Kind != protocol.ApprovalMail {
		t.Fatalf("the mail waits on a %q approval", approval.Kind)
	}

	decided := do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: approval.ID, Decision: protocol.DecideOnce})
	if decided.Status != protocol.ApprovalApproved {
		t.Fatalf("the approved mail's approval is %q, result %q", decided.Status, decided.Result)
	}
	_, passwords, recipients, messages := srv.seen()
	if len(passwords) != 1 || passwords[0] != mailPassword {
		t.Fatal("the server did not get one login with the vault's password")
	}
	if len(recipients) != 1 || recipients[0] != "ana@example.org" {
		t.Fatalf("the mail went to %q", recipients)
	}
	if len(messages) != 1 || !strings.Contains(messages[0], "Subject: Weekly notes") || !strings.Contains(messages[0], "Here are this week's notes.") {
		t.Fatalf("the server got %q", messages)
	}
	waitFor(t, "the agent to hear the answer", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	h.noSecret(t, mailPassword, asked, decided, h.vaultInfo(t))
}

// mailAccount puts the mail password in the vault and sets up the account
// on srv with it.
func (h *harness) mailAccount(t *testing.T, srv *smtpServer) {
	t.Helper()
	password := do[protocol.Credential](t, h, protocol.MVaultAdd, protocol.VaultAddParams{
		Name: "SMTP_PASSWORD", Use: protocol.CredentialMail, Value: mailPassword,
	})
	do[protocol.VaultInfo](t, h, protocol.MVaultMail, protocol.MailAccountParams{Account: &protocol.MailAccount{
		Address: "team@example.org", Host: "127.0.0.1", Port: srv.ln.Addr().(*net.TCPAddr).Port,
		Security: protocol.MailPlain, Username: "team@example.org", Credential: password.ID,
	}})
}

func TestAMailServersRefusalNeverShowsThePasswordInAnyForm(t *testing.T) {
	h := newHarness(t, 0)
	srv := newSMTPServer(t)
	srv.mu.Lock()
	srv.echoes = true
	srv.mu.Unlock()
	h.mailAccount(t, srv)

	result := do[protocol.MailTestResult](t, h, protocol.MVaultMailTest, nil)
	if _, passwords, _, _ := srv.seen(); len(passwords) != 1 {
		t.Fatalf("the server saw %d logins, want 1", len(passwords))
	}
	if result.OK || strings.Contains(result.Text, base64.StdEncoding.EncodeToString([]byte(mailPassword))) || !strings.Contains(result.Text, "‹SMTP_PASSWORD›") {
		t.Fatalf("the refused login reads %q, want the password named in every form", result.Text)
	}
	h.noSecret(t, mailPassword, result)
}

// vaultInfo is the vault as clients see it.
func (h *harness) vaultInfo(t *testing.T) protocol.VaultInfo {
	t.Helper()
	return do[protocol.VaultInfo](t, h, protocol.MVaultInfo, nil)
}
