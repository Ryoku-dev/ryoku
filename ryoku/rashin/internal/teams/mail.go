package teams

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/hostrule"
	"ryoku-rashin/internal/protocol"
)

// Limits on a mail an agent sends.
const (
	maxMailBody       = 64 << 10
	maxMailRecipients = 20
	maxMailSubject    = 200
	mailTimeout       = 30 * time.Second
)

// mailParams is the exact mail an agent asked to send. An approval stores
// it, so what the user approved is what goes.
type mailParams struct {
	To      []string `json:"to"`
	Cc      []string `json:"cc,omitempty"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}

// recipients lists every address the mail goes to.
func (m mailParams) recipients() []string {
	return append(append([]string(nil), m.To...), m.Cc...)
}

// detail shows the user the whole mail as it would go.
func (m mailParams) detail() string {
	lines := []string{i18n.Tf("To: %s", strings.Join(m.To, ", "))}
	if len(m.Cc) > 0 {
		lines = append(lines, i18n.Tf("Cc: %s", strings.Join(m.Cc, ", ")))
	}
	lines = append(lines, i18n.Tf("Subject: %s", m.Subject))
	return strings.Join(lines, "\n") + "\n\n" + m.Body
}

// sendMail sends one mail through the account: the TLS the account asks
// for, a PLAIN login with its password, then the message.
func sendMail(ctx context.Context, account protocol.MailAccount, password string, m mailParams, now time.Time) error {
	client, err := dialMail(ctx, account)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := mailLogin(client, account, password); err != nil {
		return err
	}
	if err := client.Mail(account.Address); err != nil {
		return err
	}
	for _, recipient := range m.recipients() {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(mailMessage(account, m, now)); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// testMail connects, secures the connection, logs in and quits. No mail is
// sent.
func testMail(ctx context.Context, account protocol.MailAccount, password string) error {
	client, err := dialMail(ctx, account)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := mailLogin(client, account, password); err != nil {
		return err
	}
	return client.Quit()
}

// dialMail opens an SMTP session secured the way the account says: TLS from
// the first byte, STARTTLS that the server must offer, or nothing at all
// only for a server on this computer.
func dialMail(ctx context.Context, account protocol.MailAccount) (*smtp.Client, error) {
	if account.Security == protocol.MailPlain && !hostrule.IsLocal(account.Host) {
		return nil, errors.New(i18n.T("Only a mail server on this computer may be reached without encryption."))
	}
	ctx, cancel := context.WithTimeout(ctx, mailTimeout)
	defer cancel()
	address := net.JoinHostPort(account.Host, strconv.Itoa(account.Port))
	dialer := &net.Dialer{}
	config := &tls.Config{ServerName: account.Host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	var err error
	switch account.Security {
	case protocol.MailTLS:
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: config}).DialContext(ctx, "tcp", address)
	case protocol.MailStartTLS, protocol.MailPlain:
		conn, err = dialer.DialContext(ctx, "tcp", address)
	default:
		return nil, errors.New(i18n.Tf("%q is not a mail security mode.", account.Security))
	}
	if err != nil {
		return nil, err
	}
	// one deadline for the whole session, so a silent server cannot hold a task
	if err := conn.SetDeadline(time.Now().Add(mailTimeout)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	client, err := smtp.NewClient(conn, account.Host)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if account.Security == protocol.MailStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			_ = client.Close()
			return nil, errors.New(i18n.T("The server does not offer STARTTLS, so the login would travel unencrypted."))
		}
		if err := client.StartTLS(config); err != nil {
			_ = client.Close()
			return nil, err
		}
	}
	return client, nil
}

// mailLogin logs in with PLAIN when the account has a password. net/smtp
// itself refuses PLAIN over an unencrypted connection to another computer.
func mailLogin(client *smtp.Client, account protocol.MailAccount, password string) error {
	if account.Credential == "" {
		return nil
	}
	if ok, _ := client.Extension("AUTH"); !ok {
		return errors.New(i18n.T("The server does not take a login."))
	}
	username := account.Username
	if username == "" {
		username = account.Address
	}
	return client.Auth(smtp.PlainAuth("", username, password, account.Host))
}

// mailMessage writes the message: UTF-8 plain text, quoted-printable, the
// subject and sender's name RFC 2047-encoded when they are not ASCII.
func mailMessage(account protocol.MailAccount, m mailParams, now time.Time) []byte {
	from := (&mail.Address{Name: account.Name, Address: account.Address}).String()
	var b strings.Builder
	header := func(name, value string) { b.WriteString(name + ": " + value + "\r\n") }
	header("From", from)
	header("To", strings.Join(m.To, ", "))
	if len(m.Cc) > 0 {
		header("Cc", strings.Join(m.Cc, ", "))
	}
	header("Subject", encodeHeader(m.Subject))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", messageID(account.Address))
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=UTF-8")
	header("Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")
	body := quotedprintable.NewWriter(&b)
	_, _ = body.Write([]byte(strings.ReplaceAll(m.Body, "\r\n", "\n")))
	_ = body.Close()
	return []byte(b.String())
}

// encodeHeader encodes a header value as RFC 2047 words when it is not
// ASCII, folding between words so no line grows past the limit.
func encodeHeader(value string) string {
	return strings.ReplaceAll(mime.QEncoding.Encode("utf-8", value), "?= =?", "?=\r\n =?")
}

// messageID makes a unique Message-ID under the sender's domain.
func messageID(address string) string {
	random := make([]byte, 16)
	_, _ = rand.Read(random)
	domain := "rashin.local"
	if at := strings.LastIndex(address, "@"); at >= 0 && at < len(address)-1 {
		domain = address[at+1:]
	}
	return "<" + hex.EncodeToString(random) + "@" + domain + ">"
}
