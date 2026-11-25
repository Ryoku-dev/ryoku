// Package act carries out the executable part of a reflex plan. It sits
// outside internal/oni on purpose: Oni's own packages stay read-only, and
// every process or bus call a reflex makes lives under internal/reflex,
// each behind its package's exact allowlist.
//
// Do takes one planned action and answers with an Outcome whose Line is
// what Oni may say: one short sentence, honest about what happened. A verb
// this session cannot do says so and reports ErrUnsupported; it never
// pretends. Intents that are not executor work (notes, reminders,
// routines, Oni's own placement and settings) belong to the daemon and
// answer ErrNotExecutor.
package act

import (
	"context"
	"errors"
	"strings"

	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/reflex/audio"
	"ryoku-rashin/internal/reflex/browser"
	"ryoku-rashin/internal/reflex/launch"
	"ryoku-rashin/internal/reflex/shot"
	"ryoku-rashin/internal/reflex/wm"
)

// Outcome is what one action did. Line's first sentence is at most five
// words.
type Outcome struct {
	Done bool
	Line string
	// Launch is the start record for an app or browser action, for the
	// ledger; nil otherwise.
	Launch *launch.Record
}

// Executor carries out actions.
type Executor interface {
	Do(ctx context.Context, a reflex.Action) (Outcome, error)
	// Caps reports which intents this session can carry out. It may read
	// the window manager's capabilities, bounded by a short timeout.
	Caps() map[reflex.Intent]bool
}

var (
	// ErrUnsupported means this session cannot do the action; Line says so.
	ErrUnsupported = errors.New("act: can't do that here")
	// ErrNotExecutor means the intent is the daemon's to handle.
	ErrNotExecutor = errors.New("act: not an executor intent")
)

// The lines Oni says.
const (
	LineCantHere    = "I can't do that here."
	LineFailed      = "That didn't work."
	LineCooldown    = "Already opening that."
	LineNoApp       = "I can't find that app."
	LineNoBrowser   = "No browser is installed."
	LineBadAddress  = "That address won't open."
	LineNewTab      = "New tab."
	LineOpenSite    = "Opening the page."
	LineSearch      = "Searching now."
	LineSnapLeft    = "Snapped left."
	LineSnapRight   = "Snapped right."
	LineMaximize    = "Maximized."
	LineNoWindow    = "No window to move."
	LineVolumeUp    = "Volume up."
	LineVolumeDown  = "Volume down."
	LineVolumeFull  = "Volume is already full."
	LineMuted       = "Muted."
	LineUnmuted     = "Unmuted."
	LineMuteToggled = "Sound toggled."
	LinePlaying     = "Playing."
	LinePaused      = "Paused."
	LineToggled     = "Done."
	LineNoPlayer    = "No player is open."
	LineNothingOn   = "Nothing is playing."
	LineAlreadyOn   = "Already playing."
	LinePickArea    = "Pick an area."
	LineShotSaved   = "Screenshot saved."
	LineNoShotTool  = "No screenshot tool here."
	LineOpeningIt   = "Opening it."
	maxOpeningWords = 4
)

// Deps are the executors a session has. A nil one makes its intents
// unsupported.
type Deps struct {
	Launcher *launch.Launcher
	Browser  *browser.Browser
	Volume   *audio.Volume
	Media    *audio.Media
	WM       *wm.WM
	Shot     *shot.Shooter
}

// New returns the Executor over d.
func New(d Deps) Executor { return &executor{d} }

type executor struct{ d Deps }

// executorIntents are the intents act carries out.
var executorIntents = []reflex.Intent{
	reflex.OpenApp, reflex.OpenSite, reflex.WebSearch, reflex.NewTab,
	reflex.SnapLeft, reflex.SnapRight, reflex.Maximize, reflex.Minimize,
	reflex.VolumeUp, reflex.VolumeDown, reflex.Mute, reflex.PlayPause, reflex.Screenshot,
}

func (x *executor) Caps() map[reflex.Intent]bool {
	caps := make(map[reflex.Intent]bool, len(executorIntents))
	caps[reflex.OpenApp] = x.d.Launcher != nil && x.d.Launcher.Catalog().Len() > 0
	browserOK := x.d.Browser != nil
	caps[reflex.OpenSite], caps[reflex.WebSearch], caps[reflex.NewTab] = browserOK, browserOK, browserOK
	if x.d.WM != nil {
		sup := x.d.WM.Supports(context.Background())
		caps[reflex.SnapLeft], caps[reflex.SnapRight] = sup[wm.SnapLeft], sup[wm.SnapRight]
		caps[reflex.Maximize], caps[reflex.Minimize] = sup[wm.Maximize], sup[wm.Minimize]
	} else {
		caps[reflex.SnapLeft], caps[reflex.SnapRight], caps[reflex.Maximize], caps[reflex.Minimize] = false, false, false, false
	}
	caps[reflex.VolumeUp], caps[reflex.VolumeDown], caps[reflex.Mute] = x.d.Volume != nil, x.d.Volume != nil, x.d.Volume != nil
	caps[reflex.PlayPause] = x.d.Media != nil
	caps[reflex.Screenshot] = x.d.Shot != nil && x.d.Shot.Available()
	return caps
}

func (x *executor) Do(ctx context.Context, a reflex.Action) (Outcome, error) {
	switch a.Intent {
	case reflex.OpenApp:
		if x.d.Launcher == nil {
			return unsupported()
		}
		rec, err := x.d.Launcher.Launch(ctx, a.Arg)
		return launched(rec, err, opening(rec.Name))
	case reflex.NewTab, reflex.OpenSite, reflex.WebSearch:
		if x.d.Browser == nil {
			return Outcome{Line: LineNoBrowser}, ErrUnsupported
		}
		var (
			rec  launch.Record
			err  error
			line string
		)
		switch a.Intent {
		case reflex.NewTab:
			rec, err = x.d.Browser.NewTab(ctx)
			line = LineNewTab
		case reflex.OpenSite:
			rec, err = x.d.Browser.Open(ctx, a.Arg)
			line = LineOpenSite
		default:
			rec, err = x.d.Browser.Search(ctx, a.Arg)
			line = LineSearch
		}
		return launched(rec, err, line)
	case reflex.SnapLeft, reflex.SnapRight, reflex.Maximize, reflex.Minimize:
		if x.d.WM == nil {
			return unsupported()
		}
		err := x.d.WM.Do(ctx, wm.Verb(a.Intent))
		switch {
		case errors.Is(err, wm.ErrUnsupported):
			return unsupported()
		case errors.Is(err, wm.ErrNoWindow):
			return Outcome{Line: LineNoWindow}, err
		case err != nil:
			return Outcome{Line: LineFailed}, err
		}
		return Outcome{Done: true, Line: map[reflex.Intent]string{
			reflex.SnapLeft: LineSnapLeft, reflex.SnapRight: LineSnapRight, reflex.Maximize: LineMaximize,
		}[a.Intent]}, nil
	case reflex.VolumeUp, reflex.VolumeDown, reflex.Mute:
		return x.volume(ctx, a)
	case reflex.PlayPause:
		return x.media(ctx, a.Arg)
	case reflex.Screenshot:
		if x.d.Shot == nil {
			return Outcome{Line: LineNoShotTool}, shot.ErrUnavailable
		}
		res, err := x.d.Shot.Take(ctx)
		switch {
		case errors.Is(err, shot.ErrUnavailable):
			return Outcome{Line: LineNoShotTool}, err
		case err != nil:
			return Outcome{Line: LineFailed}, err
		case res.Picking:
			return Outcome{Done: true, Line: LinePickArea}, nil
		}
		return Outcome{Done: true, Line: LineShotSaved}, nil
	}
	if _, ok := reflex.Lookup(a.Intent); ok {
		return Outcome{}, ErrNotExecutor
	}
	return unsupported()
}

func (x *executor) volume(ctx context.Context, a reflex.Action) (Outcome, error) {
	if x.d.Volume == nil {
		return unsupported()
	}
	var err error
	line := ""
	switch a.Intent {
	case reflex.VolumeUp:
		err, line = x.d.Volume.Up(ctx), LineVolumeUp
	case reflex.VolumeDown:
		err, line = x.d.Volume.Down(ctx), LineVolumeDown
	default:
		switch a.Arg {
		case reflex.MuteOn:
			err, line = x.d.Volume.Mute(ctx, audio.MuteOn), LineMuted
		case reflex.MuteOff:
			err, line = x.d.Volume.Mute(ctx, audio.MuteOff), LineUnmuted
		default:
			err, line = x.d.Volume.Mute(ctx, audio.MuteToggle), LineMuteToggled
		}
	}
	switch {
	case errors.Is(err, audio.ErrAtFull):
		return Outcome{Line: LineVolumeFull}, nil
	case err != nil:
		return Outcome{Line: LineFailed}, err
	}
	return Outcome{Done: true, Line: line}, nil
}

func (x *executor) media(ctx context.Context, arg string) (Outcome, error) {
	if x.d.Media == nil {
		return unsupported()
	}
	verb := audio.MediaToggle
	switch arg {
	case reflex.MediaPlay:
		verb = audio.MediaPlay
	case reflex.MediaPause:
		verb = audio.MediaPause
	}
	did, err := x.d.Media.Do(ctx, verb)
	switch {
	case errors.Is(err, audio.ErrNoPlayer):
		return Outcome{Line: LineNoPlayer}, nil
	case errors.Is(err, audio.ErrNothingPlaying):
		return Outcome{Line: LineNothingOn}, nil
	case errors.Is(err, audio.ErrAlreadyPlaying):
		return Outcome{Line: LineAlreadyOn}, nil
	case err != nil:
		return Outcome{Line: LineFailed}, err
	}
	line := map[audio.MediaDid]string{audio.DidPlay: LinePlaying, audio.DidPause: LinePaused, audio.DidToggle: LineToggled}[did]
	return Outcome{Done: true, Line: line}, nil
}

func unsupported() (Outcome, error) { return Outcome{Line: LineCantHere}, ErrUnsupported }

// launched turns a start into an outcome. A cooldown refusal is its own
// answer, distinct from a failure: nothing was started and nothing failed.
func launched(rec launch.Record, err error, line string) (Outcome, error) {
	switch {
	case errors.Is(err, launch.ErrCooldown):
		return Outcome{Line: LineCooldown}, err
	case errors.Is(err, launch.ErrNotFound):
		return Outcome{Line: LineNoApp}, err
	case errors.Is(err, launch.ErrBadURL):
		return Outcome{Line: LineBadAddress}, err
	case err != nil:
		return Outcome{Line: LineFailed}, err
	}
	return Outcome{Done: true, Line: line, Launch: &rec}, nil
}

// opening is "Opening NAME." when the name is short enough for the
// five-word first sentence, else "Opening it."
func opening(name string) string {
	if n := len(strings.Fields(name)); n == 0 || n > maxOpeningWords {
		return LineOpeningIt
	}
	return "Opening " + strings.TrimRight(name, ".!?") + "."
}
