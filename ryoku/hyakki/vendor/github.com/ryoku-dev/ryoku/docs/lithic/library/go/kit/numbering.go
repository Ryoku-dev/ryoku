package kit

import (
	"strconv"
	"sync/atomic"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
)

// padded holds "00" .. "99" so Pad2 allocates nothing for the numbers a sheet counts.
var padded = func() (out [100]string) {
	for n := range out {
		out[n] = string([]byte{byte('0' + n/10), byte('0' + n%10)})
	}
	return out
}()

// Pad2 zero-pads n to two digits, as the Hub numbers its sheet (registry/numbering.js:7-9):
// 3 is "03", 12 is "12", 120 is "120".
func Pad2(n int) string {
	if n >= 0 && n < 100 {
		return padded[n]
	}
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// Ordinal is a part's place among its siblings, "01 / 08", as the Hub marks its ruled
// sections (SettingsPage.qml:772). The masthead's page place is written without the spaces,
// "P " + Pad2(at) + "/" + Pad2(of) (Masthead.qml:38-39).
func Ordinal(n, of int) string {
	return Pad2(n) + " / " + Pad2(of)
}

var focused atomic.Bool

// SetFocused tells the kit whether the app's window is the focused one. Call it from the
// window's app.ConfigEvent (Config.Focused). Until it is called the window counts as not
// focused, so nothing ambient runs in a window that never says.
func SetFocused(on bool) { focused.Store(on) }

// Ambient reports whether the sheet may show its small life at rest, the Hub's Ambient.on
// (Hub.qml:405-410): the window is focused (SetFocused), motion is not reduced, and gtx is
// live (a disabled or headless context has none). Gate a ticking clock or a scan on it, so an
// idle or unfocused window draws no frames.
func Ambient(gtx layout.Context) bool {
	return focused.Load() && !anim.Reduced() && gtx.Source.Enabled()
}

// Clock is the masthead's clock: "12:40", with ":07" seconds while Ambient is on, when it
// also asks for a frame at the next second. Off Ambient it asks for nothing: as in the Hub
// (Ambient.qml:18-24) the clock rests until the window is focused again. It keeps its last
// string, so a frame within the same second allocates nothing.
type Clock struct {
	at      int64
	seconds bool
	text    string
}

// Text returns the clock at gtx.Now.
func (clock *Clock) Text(gtx layout.Context) string {
	now := gtx.Now
	if now.IsZero() {
		now = time.Now()
	}
	seconds := Ambient(gtx)
	at := now.Unix()
	if seconds {
		gtx.Execute(op.InvalidateCmd{At: now.Truncate(time.Second).Add(time.Second)})
	} else {
		at -= int64(now.Second())
	}
	if at != clock.at || seconds != clock.seconds || clock.text == "" {
		clock.at, clock.seconds = at, seconds
		clock.text = Pad2(now.Hour()) + ":" + Pad2(now.Minute())
		if seconds {
			clock.text += ":" + Pad2(now.Second())
		}
	}
	return clock.text
}
