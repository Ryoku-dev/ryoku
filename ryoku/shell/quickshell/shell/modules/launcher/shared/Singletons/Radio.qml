pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Services.Mpris
import "../lib/radio.js" as RadioLib

// Live-radio state for the "@" provider, mirrored from ryoku-cmd-radio. The
// resident launcher also acts as the radio's minder. Two jobs beyond mirroring:
//  - collision: the moment any real player that is not the radio (and not a
//    live wallpaper, those are mpv too) starts playing, the radio steps aside
//    (stop --aside) instead of talking over it; the provider offers resume.
//  - identity: isRadio(p) is how every surface tells the radio mpv apart, by
//    its mpv D-Bus name plus the engine's forced "LIVE · " title.
Singleton {
    id: root

    property bool available: false
    property bool availabilityKnown: false
    property bool on: false
    property string station: ""
    property string label: ""
    property bool live: false          // primary (resolved) stream, not the direct fallback
    property bool fellBack: false
    property var aside: null           // {station,label} the collision watcher parked
    property bool busy: false          // a verb is in flight; poll answers will settle it

    function isRadio(p) {
        return !!p && RadioLib.isRadioPlayer(p.dbusName, p.trackTitle);
    }

    // is the radio's mpv actually on the players bus yet? Drives the tuning
    // chip (the state says on air while yt-dlp still resolves, up to 25s of
    // honest silence) and arms the minder when the radio was started outside
    // this UI (a terminal, a keybind): a LIVE mpv appearing is evidence enough
    // to go ask the engine what's playing.
    readonly property bool playerPresent: {
        var list = Players.realPlayers();
        for (var i = 0; i < list.length; i++)
            if (isRadio(list[i]))
                return true;
        return false;
    }
    onPlayerPresentChanged: if (playerPresent && !on) refresh()
    readonly property bool tuning: on && !playerPresent

    // an explicit tune-in means the radio has the floor: pause whatever real
    // player is sounding (never a live wallpaper) instead of instantly losing
    // to it, the collision watcher only referees music that starts LATER.
    // Whoever was sounding at tune-in is tolerated: if its pause doesn't
    // take (canPause=false, or a stream that shrugs off MPRIS Pause), it still
    // never parks the radio the user explicitly chose over it.
    property var _tolerated: ({})
    function _quellRivals() {
        var g = {};
        var list = Players.realPlayers();
        for (var i = 0; i < list.length; i++) {
            var p = list[i];
            if (!p || !p.isPlaying || !RadioLib.countsAsMusic(p.trackTitle))
                continue;
            g[String(p.dbusName || "")] = true;
            if (p.canPause)
                p.pause();
        }
        _tolerated = g;
    }
    function start(id) { _quellRivals(); _grace.restart(); _run(["start"].concat(id ? [id] : [])); }
    function stop() { _run(["stop"]); }
    function resume() { _quellRivals(); _grace.restart(); _run(["resume"]); }
    function toggle(id) { _run(["toggle"].concat(id ? [id] : [])); }
    // the pause above lands asynchronously; without a beat of grace the watcher
    // would see the old Playing state and park the radio it just started.
    Timer { id: _grace; interval: 2500 }

    function _run(args) {
        busy = true;
        verbProc.command = ["ryoku-cmd-radio"].concat(args);
        verbProc.running = true;
    }
    Process {
        id: verbProc
        onExited: {
            root.busy = false;
            root.refresh();
        }
    }

    function refresh() {
        statusProc.running = false;
        statusProc.running = true;
    }
    Process {
        id: statusProc
        command: ["ryoku-cmd-radio", "status"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var s = JSON.parse(text);
                    root.available = s.available === true;
                    root.availabilityKnown = true;
                    root.on = s.on === true;
                    root.station = s.station || "";
                    root.label = s.label || "";
                    root.live = s.live === true;
                    root.fellBack = s.fellBack === true;
                    root.aside = s.aside || null;
                    // an open "@" list re-pulls its rows on the bump, so a verb's
                    // outcome (stop becomes resume, on-air moves first) paints
                    // without retyping.
                    Dispatcher.notifyAsync();
                } catch (e) {}
            }
        }
    }
    // cheap heartbeat only while there is something to track: a dead supervisor
    // (stream gone for good) must read as off, and an aside state must survive a
    // launcher reopen. One initial read covers a restart mid-broadcast.
    Timer {
        interval: 4000
        repeat: true
        running: root.on || root.aside !== null
        onTriggered: {
            root.refresh();
            // grandfathering ends the moment the old music actually stops: a
            // later resume of the same player is fresh music and parks the
            // radio like anything else. Pruned here, not in the binding,
            // bindings must not mutate state.
            var g = root._tolerated;
            var changed = false;
            var list = Players.realPlayers();
            var playing = {};
            for (var i = 0; i < list.length; i++)
                if (list[i] && list[i].isPlaying)
                    playing[String(list[i].dbusName || "")] = true;
            for (var k in g)
                if (!playing[k]) { delete g[k]; changed = true; }
            if (changed)
                root._tolerated = g;
        }
    }
    Component.onCompleted: refresh()

    // ---- collision: new music sets the radio aside --------------------------
    // Bound over the players list so it re-evaluates on any playback change.
    // Wallpaper mpvs never count (a live wallpaper "plays" all day), nor does
    // the radio itself.
    readonly property bool rivalPlaying: {
        if (!root.on || _grace.running)
            return false;
        var list = Players.realPlayers();
        for (var i = 0; i < list.length; i++) {
            var p = list[i];
            if (!p || !p.isPlaying)
                continue;
            // countsAsMusic drops the radio itself, wallpapers, URL titles and
            // the titleless registration beat a fresh mpv goes through, the
            // radio's own player must never read as the rival that kills it.
            if (!RadioLib.countsAsMusic(p.trackTitle))
                continue;
            // the music the user tuned in over stays tolerated until it
            // actually stops; only fresh (or resumed) music parks the radio.
            if (_tolerated[String(p.dbusName || "")] === true)
                continue;
            return true;
        }
        return false;
    }
    onRivalPlayingChanged: {
        if (rivalPlaying && root.on && !root.busy)
            _run(["stop", "--aside"]);
    }
}
