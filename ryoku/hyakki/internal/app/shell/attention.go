package shell

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"ryoku-i18n"

	"gioui.org/io/system"

	"ryoku-rashin/internal/app/desktop"
	agentpage "ryoku-rashin/internal/app/pages/agent"
	"ryoku-rashin/internal/app/state"
)

// follows the store, not the frames: a window nobody looks at draws none
type attention struct {
	notifier *desktop.Notifier
	// set by the window as it gains and loses the keyboard, and by the setting
	focused, muted atomic.Bool

	mu     sync.Mutex
	last   map[string]agentpage.Standing
	primed bool
	// pending approvals and unread reports already seen, by id
	asked, reported map[string]bool
}

// the first call only takes stock, what was already so when the app started is not news
func (a *attention) notices(snapshot state.Snapshot) []desktop.Notice {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := agentpage.Standings(snapshot)
	before, primed := a.last, a.primed
	a.last, a.primed = now, snapshot.Connection == state.Ready
	asked, reported := teamsStanding(snapshot)
	askedBefore, reportedBefore := a.asked, a.reported
	a.asked, a.reported = asked, reported
	if !primed {
		return nil
	}
	var notices []desktop.Notice
	for id, standing := range now {
		was, known := before[id]
		notice := desktop.Notice{Key: id, Body: standing.Matter}
		switch {
		case standing.Failed && !(known && was.Failed):
			notice.Title = i18n.Tf("%s failed", standing.Title)
		case standing.Needs && !(known && was.Needs):
			notice.Title = i18n.Tf("%s needs you", standing.Title)
		case known && was.Working && !standing.Working && !standing.Needs:
			notice.Title = i18n.Tf("%s finished", standing.Title)
		default:
			continue
		}
		// a session this window has not opened is only known by where it works
		if notice.Body == "" {
			notice.Body = i18n.Tf("In %s", standing.Place)
		}
		notices = append(notices, notice)
	}
	return append(notices, teamsNotices(snapshot, askedBefore, reportedBefore)...)
}

// runs on the connection goroutine after every change
func (a *attention) storeChanged(snapshot state.Snapshot) {
	notices := a.notices(snapshot)
	if a.notifier == nil || a.focused.Load() || a.muted.Load() {
		return
	}
	for _, notice := range notices {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = a.notifier.Post(ctx, notice)
		}()
	}
}

// a desktop without a notification service leaves it taking stock and saying nothing
func (r *runtime) watchAttention(ctx context.Context, shell *uiShell) *attention {
	watch := new(attention)
	watch.muted.Store(shell.preferences.MuteNotices)
	notifier, err := desktop.NewNotifier(ctx, func(key string) {
		shell.posts.Post(func() {
			shell.openNotice(key)
			shell.windowAction(system.ActionRaise)
		})
	})
	if err == nil {
		watch.notifier = notifier
		go func() {
			<-ctx.Done()
			notifier.Close()
		}()
	}
	remove := r.hub.Add(func() { watch.storeChanged(r.store.Snapshot()) })
	go func() {
		<-ctx.Done()
		remove()
	}()
	return watch
}
