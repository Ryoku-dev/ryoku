package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"slices"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sensitive"
)

// oniNotifyPerMinute is the most lines oni.notify shows in any minute,
// whoever sends them; more are refused, never queued.
const oniNotifyPerMinute = 3

// oniNotify is when the lines of the last minute were shown.
type oniNotify struct {
	mu    sync.Mutex
	shown []time.Time
}

// take reports whether a line may be shown at now, counting it if so.
func (n *oniNotify) take(now time.Time) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.shown = slices.DeleteFunc(n.shown, func(at time.Time) bool { return now.Sub(at) >= time.Minute })
	if len(n.shown) >= oniNotifyPerMinute {
		return false
	}
	n.shown = append(n.shown, now)
	return true
}

// oniAskReply is what the person's answer to a question the MCP bridge
// asked came to.
type oniAskReply struct {
	res protocol.OniAskResult
	err error
}

// reply hands what q's answer came to the bridge's waiting call, once; a
// question nobody waits on drops it.
func (q *oniQuestion) reply(res protocol.OniAskResult, err error) {
	if q.answered == nil {
		return
	}
	select {
	case q.answered <- oniAskReply{res: res, err: err}:
	default:
	}
}

// oniAskAwait waits for the person to answer the consent card of q, which
// the MCP bridge asked: a yes comes back as the answer's words, a no, an
// expiry or a question dropped as declined. The bridge never sees the
// picture or its preview.
func (d *Daemon) oniAskAwait(q *oniQuestion) (protocol.OniAskResult, error) {
	timer := time.NewTimer(feature.ConsentTTL + oniAskTimeout + 5*time.Second)
	defer timer.Stop()
	select {
	case r := <-q.answered:
		if r.err != nil {
			return protocol.OniAskResult{}, r.err
		}
		return protocol.OniAskResult{Status: r.res.Status, Answer: r.res.Answer}, nil
	case <-timer.C:
		return protocol.OniAskResult{Status: protocol.OniAskDeclined}, nil
	}
}

// bridgeConsentAnswer lets the surface drawing Oni answer the consent card
// of the question the MCP bridge asked: the person's press is the answer
// the bridge waits on. ok is false for any other caller or question.
func (d *Daemon) bridgeConsentAnswer(c *connection, p protocol.OniConsentAnswerParams) (protocol.OniAskResult, bool, error) {
	d.oni.clientsMu.Lock()
	isPresenter := d.oni.presenter == c
	d.oni.clientsMu.Unlock()
	if !isPresenter {
		return protocol.OniAskResult{}, false, nil
	}
	a := &d.oni.ask
	a.mu.Lock()
	bridged := a.pending != nil && a.pending.answered != nil && a.pending.id == p.ID
	a.mu.Unlock()
	if !bridged {
		return protocol.OniAskResult{}, false, nil
	}
	allow := p.Answer == "yes"
	q, err := d.oniConsentTake(p.ID, allow, func(q *oniQuestion) error {
		if q.answered == nil {
			return protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only whoever asked, or the Hyakki app, answers this question."))
		}
		return nil
	})
	if err != nil {
		return protocol.OniAskResult{}, true, err
	}
	res, err := d.oniConsentGive(q, allow)
	q.reply(res, err)
	return res, true, err
}

// remindAddCall serves oni.reminders add: one reminder or timer the store
// keeps within its own bounds, its words screened like a spoken note's.
// It needs reflex.reminders, and voice for a timer.
func (d *Daemon) remindAddCall(p protocol.OniRemindersParams) (protocol.OniRemindersResult, error) {
	kind := p.Kind
	if kind != remind.KindReminder && kind != remind.KindTimer {
		return protocol.OniRemindersResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A reminder is a reminder or a timer."))
	}
	if err := d.remindRefusal(kind); err != nil {
		return protocol.OniRemindersResult{}, err
	}
	text := strings.TrimSpace(p.Text)
	if text != "" && (sensitive.Number(text) != sensitive.None || sensitive.Secret(text)) {
		return protocol.OniRemindersResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A reminder holds no private number or secret."))
	}
	id := make([]byte, protocol.OniReminderIDLen/2)
	if _, err := rand.Read(id); err != nil {
		return protocol.OniRemindersResult{}, protocol.Errorf(protocol.CodeInternal, "%s", i18n.T("The reminder could not be kept."))
	}
	rem := remind.Reminder{ID: hex.EncodeToString(id), Kind: kind, Due: p.Due, Created: d.reflexClock().Now(), Text: text}
	r := &d.oni.remind
	r.mu.Lock()
	err := d.remindStoreLocked().Add(rem)
	if err == nil {
		d.remindChangedLocked()
	}
	r.mu.Unlock()
	if err != nil {
		return protocol.OniRemindersResult{}, err
	}
	d.stateChanged()
	return d.remindersCall(protocol.OniRemindersParams{Action: protocol.OniRemindersList})
}

// oniNotifyCall serves oni.notify: one plain line in Oni's pill, headed
// with the name of the program on the MCP bridge that sent it, at most
// oniNotifyPerMinute a minute. It is refused while Oni is off, hidden or
// not on screen, and shows no link, private number or secret.
func (d *Daemon) oniNotifyCall(c *connection, p protocol.OniNotifyParams) (protocol.Empty, error) {
	text := strings.TrimSpace(p.Text)
	lower := strings.ToLower(text)
	switch {
	case !protocol.ValidOniNotifyText(text) || text == "":
		return protocol.Empty{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A notice is one line of at most %d characters.", protocol.OniNotifyMax))
	case strings.Contains(lower, "://") || strings.Contains(lower, "www."):
		return protocol.Empty{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A notice carries no links."))
	case sensitive.Number(text) != sensitive.None || sensitive.Secret(text):
		return protocol.Empty{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A notice holds no private number or secret."))
	}
	settings := protocol.EffectiveOni(d.Settings())
	svc := d.oni.service.Load()
	switch {
	case !settings.Master:
		return protocol.Empty{}, protocol.Errorf(protocol.CodeDisabled, "%s", i18n.T("Oni is off."))
	case settings.Hidden || svc == nil:
		return protocol.Empty{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Oni is not on screen now."))
	}
	if !d.oni.notify.take(time.Now()) {
		return protocol.Empty{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("At most %d notices a minute. Wait a little.", oniNotifyPerMinute))
	}
	c.mu.RLock()
	name := c.name
	c.mu.RUnlock()
	if name == "" {
		name = "Hyakki"
	}
	if err := svc.Notice(oniText(name+": "+text, oniBubbleMax)); err != nil {
		return protocol.Empty{}, presenceRefusal(err)
	}
	return protocol.Empty{}, nil
}

// oniMCPStatus serves oni.mcp: the bridge's switch, the tools turned on,
// the approved programs and those connected that wait. A program on the
// bridge sees its own entry alone.
func (d *Daemon) oniMCPStatus(c *connection) protocol.OniMCPStatus {
	settings := protocol.EffectiveOni(d.Settings()).MCP
	out := protocol.OniMCPStatus{Enabled: settings.Enabled, Tools: []string{}, Clients: []protocol.OniMCPClient{}}
	for _, id := range protocol.OniMCPTools {
		if settings.Tools[id] {
			out.Tools = append(out.Tools, id)
		}
	}
	connected := map[string]bool{}
	var waiting []string
	d.mu.RLock()
	for other := range d.clients {
		other.mu.RLock()
		if other.kind == protocol.ClientMCP && other.name != "" {
			if !connected[other.name] && !slices.Contains(settings.Clients, other.name) {
				waiting = append(waiting, other.name)
			}
			connected[other.name] = true
		}
		other.mu.RUnlock()
	}
	d.mu.RUnlock()
	slices.Sort(waiting)
	for _, name := range settings.Clients {
		out.Clients = append(out.Clients, protocol.OniMCPClient{Name: name, State: protocol.OniMCPClientApproved, Connected: connected[name]})
	}
	for _, name := range waiting {
		out.Clients = append(out.Clients, protocol.OniMCPClient{Name: name, State: protocol.OniMCPClientWaiting, Connected: true})
	}
	c.mu.RLock()
	kind, self := c.kind, c.name
	c.mu.RUnlock()
	if kind == protocol.ClientMCP {
		out.Clients = slices.DeleteFunc(out.Clients, func(entry protocol.OniMCPClient) bool { return entry.Name != self })
	}
	return out
}
