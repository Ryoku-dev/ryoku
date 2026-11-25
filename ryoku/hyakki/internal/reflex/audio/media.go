package audio

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/godbus/dbus/v5"

	"ryoku-rashin/internal/reflex/bus"
)

const (
	busName      = "org.freedesktop.DBus"
	busPath      = "/org/freedesktop/DBus"
	listNames    = "org.freedesktop.DBus.ListNames"
	propsGet     = "org.freedesktop.DBus.Properties.Get"
	playerPrefix = "org.mpris.MediaPlayer2."
	playerPath   = "/org/mpris/MediaPlayer2"
	playerIface  = "org.mpris.MediaPlayer2.Player"
	statusProp   = "PlaybackStatus"
	// playPause is the one player method this package calls; it takes no
	// argument, and the player chosen decides which way it flips.
	playPause = playerIface + ".PlayPause"
)

// maxPlayers bounds how many players one action looks at.
const maxPlayers = 16

// AllowedCall reports whether one bus call is one this package may make:
// the bus's name list, a player's playback status, or a player's PlayPause
// with no arguments. Nothing else, and only at the player's standard
// object path.
func AllowedCall(dest, path, method string, args []any) bool {
	switch {
	case dest == busName && path == busPath && method == listNames:
		return len(args) == 0
	case !isPlayer(dest) || path != playerPath:
		return false
	case method == propsGet:
		return len(args) == 2 && args[0] == playerIface && args[1] == statusProp
	}
	return method == playPause && len(args) == 0
}

// isPlayer reports whether name is a media player's well-known bus name.
func isPlayer(name string) bool {
	rest, ok := strings.CutPrefix(name, playerPrefix)
	if !ok || rest == "" || len(name) > 255 {
		return false
	}
	for _, r := range rest {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}

// MediaVerb is what a media action asks for.
type MediaVerb int

const (
	MediaToggle MediaVerb = iota
	MediaPlay
	MediaPause
)

// MediaDid is what a media action did.
type MediaDid int

const (
	DidNothing MediaDid = iota
	DidPlay             // a paused player was started
	DidPause            // a playing player was paused
	DidToggle           // a player that reports no status was flipped
)

var (
	// ErrNoPlayer means no media player is on the bus.
	ErrNoPlayer = errors.New("audio: no media player")
	// ErrNothingPlaying means pause was asked and nothing plays, or play
	// was asked and nothing is paused.
	ErrNothingPlaying = errors.New("audio: nothing is playing")
	// ErrAlreadyPlaying means play was asked and something already plays.
	ErrAlreadyPlaying = errors.New("audio: already playing")
)

// Media plays and pauses media players.
type Media struct{ dial bus.Dial }

// NewMedia returns a Media that opens a connection with dial for each
// action (bus.Session for the real one) and closes it after. Every call is
// checked against AllowedCall.
func NewMedia(dial bus.Dial) *Media { return &Media{dial: bus.Guard(AllowedCall, dial)} }

// Do flips one player with PlayPause: for a pause, the first playing
// player; for a play, the first paused one; for a toggle, whichever of
// those comes first (pausing first). When no player reports its status,
// the first player is flipped.
func (m *Media) Do(ctx context.Context, verb MediaVerb) (MediaDid, error) {
	conn, err := m.dial(ctx)
	if err != nil {
		return DidNothing, err
	}
	defer conn.Close()
	var names []string
	if err := conn.Call(ctx, busName, busPath, listNames, nil, &names); err != nil {
		return DidNothing, err
	}
	var players []string
	for _, n := range names {
		if isPlayer(n) {
			players = append(players, n)
		}
	}
	slices.Sort(players)
	if len(players) > maxPlayers {
		players = players[:maxPlayers]
	}
	if len(players) == 0 {
		return DidNothing, ErrNoPlayer
	}
	var playing, paused string
	known := false
	for _, p := range players {
		var v dbus.Variant
		if conn.Call(ctx, p, playerPath, propsGet, []any{playerIface, statusProp}, &v) != nil {
			continue
		}
		known = true
		switch s, _ := v.Value().(string); s {
		case "Playing":
			if playing == "" {
				playing = p
			}
		case "Paused":
			if paused == "" {
				paused = p
			}
		}
	}
	switch {
	case !known:
		return DidToggle, conn.Call(ctx, players[0], playerPath, playPause, nil)
	case verb != MediaPlay && playing != "":
		return DidPause, conn.Call(ctx, playing, playerPath, playPause, nil)
	case verb == MediaPause:
		return DidNothing, ErrNothingPlaying
	case verb == MediaPlay && playing != "":
		return DidNothing, ErrAlreadyPlaying
	case paused != "":
		return DidPlay, conn.Call(ctx, paused, playerPath, playPause, nil)
	}
	return DidNothing, ErrNothingPlaying
}
