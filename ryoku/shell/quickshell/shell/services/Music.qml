pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import "../utils/menupoll.js" as MenuPoll

// The enriched now-playing track: the full-size cover the daemon resolved and
// this song's synced lyrics, plus the playback clock every lyric surface follows.
// Media picks the player; this pushes that pick to ryoku-shell (music.go), which
// owns the LRCLIB and cover-art fetches, and subscribes to the frame it streams
// back, so no surface here makes an HTTP request.
//
// `hold` is owner-refcounted like AudioBars: the position poll and the line
// tracker run only while a visible surface claims them, so a hidden lyric sheet
// costs nothing. MPRIS never pushes position, hence the poll; between polls the
// clock is interpolated, so the highlight moves at 60fps rather than in 250ms
// steps.
Singleton {
    id: root

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    property var frame: ({})
    readonly property string source: root.frame.source || ""
    readonly property string art: root.frame.art || ""
    // the track's looping backdrop (Spotify Canvas), a URL or path the daemon
    // resolved; empty when none. The widget streams it directly.
    readonly property string canvas: root.frame.canvas || ""
    readonly property bool hasArt: root.art.length > 0
    // The daemon publishes a filesystem path; Image wants a URL.
    readonly property string artUrl: root.hasArt ? "file://" + root.art : ""
    readonly property var lines: root.frame.lyrics || []
    readonly property var plain: root.frame.plain || []
    readonly property string status: root.frame.lyricsStatus || "idle"
    readonly property bool synced: root.status === "ok" && root.lines.length > 0
    readonly property bool unsynced: root.status === "plain" && root.plain.length > 0
    readonly property bool searching: root.status === "loading"

    // The track as a surface shows it. Held here rather than read off the player
    // at each call site so every music surface names the song identically, and so
    // the desktop widget (whose own Theme singleton shadows the service one) can
    // still reach the joined artist list.
    readonly property string title: Media.player ? (Media.player.trackTitle || "") : ""
    readonly property string artist: Media.player
        ? Theme.joinArtists(Media.player.trackArtists, Media.player.trackArtist) : ""
    readonly property string album: Media.player ? (Media.player.trackAlbum || "") : ""

    // "m:ss", or "h:mm:ss" past the hour: the clock every player surface prints.
    function stamp(seconds) {
        const total = Math.max(0, Math.floor(seconds));
        const h = Math.floor(total / 3600);
        const m = Math.floor((total % 3600) / 60);
        const r = total % 60;
        const ss = (r < 10 ? "0" : "") + r;
        if (h >= 1)
            return h + ":" + (m < 10 ? "0" : "") + m + ":" + ss;
        return m + ":" + ss;
    }

    // ── who wants the playback clock ─────────────────────────────────────────
    property var owners: []
    readonly property bool held: root.owners.length > 0
    function hold(owner, on) {
        root.owners = MenuPoll.setOwnership(root.owners, owner, on);
    }
    function holdClock(owner, on) {
        root.hold(owner, on);
    }

    // ── the playback clock ───────────────────────────────────────────────────
    // `anchor` is the last position MPRIS reported and `anchorMs` when it
    // arrived, so `elapsed` reads as a continuous time while playing.
    property real anchor: 0
    property real anchorMs: 0
    property real elapsed: 0
    readonly property var player: Media.player
    readonly property bool playing: Media.playing
    readonly property real length: root.player && root.player.length > 0 ? root.player.length : 0
    readonly property real position: root.elapsed
    readonly property real duration: root.length

    function reanchor(pos) {
        root.anchor = Math.max(0, pos);
        root.anchorMs = Date.now();
        root.elapsed = root.anchor;
    }
    function estimate() {
        if (!root.playing)
            return root.anchor;
        return root.anchor + (Date.now() - root.anchorMs) / 1000;
    }
    // Seeking from a surface: move the player and re-anchor at once, so the rail
    // and the lyric line jump with the pointer instead of after the next poll.
    function seek(seconds) {
        if (!root.player || !root.player.canSeek)
            return;
        const target = Math.max(0, Math.min(seconds, root.length > 0 ? root.length : seconds));
        root.player.position = target;
        root.reanchor(target);
    }
    function seekPosition(seconds) {
        root.seek(seconds);
    }

    onPlayingChanged: root.reanchor(root.estimate())

    // MPRIS reports position only when asked. Four polls a second keep the
    // anchor honest (a seek elsewhere, a stream stall) while the frame ticker
    // below carries the motion. Both pause while nobody can see the desktop.
    readonly property bool ticking: root.held && root.playing && !Presence.dark
    function refresh() {
        root.player.positionChanged();
        const reported = root.player.position;
        if (Math.abs(reported - root.estimate()) > 0.35 || reported === 0)
            root.reanchor(reported);
    }
    Timer {
        interval: 250
        repeat: true
        running: root.ticking && root.player !== null
        onTriggered: root.refresh()
    }
    Timer {
        interval: 60
        repeat: true
        running: root.ticking
        onTriggered: root.elapsed = root.estimate()
    }
    // Back from dark: land on the live position now instead of on the next tick.
    Connections {
        target: Presence
        function onDarkChanged() {
            if (Presence.dark || !root.held || !root.playing)
                return;
            if (root.player)
                root.refresh();
            root.elapsed = root.estimate();
        }
    }

    // ── the current lyric line ───────────────────────────────────────────────
    // -1 until the first timestamp is reached, which is how a sheet knows to
    // show its intro rather than highlighting the opening line early.
    readonly property int index: {
        if (!root.synced)
            return -1;
        const t = root.elapsed;
        let found = -1;
        for (let i = 0; i < root.lines.length; i++) {
            if (root.lines[i].t <= t)
                found = i;
            else
                break;
        }
        return found;
    }

    // ── the track pushed to the daemon ───────────────────────────────────────
    // Every shell surface pushes the same pick and MPRIS repeats metadata on
    // each property change; the daemon dedupes by track, so this stays a plain
    // write on every change.
    function push() {
        const p = Media.player;
        if (!p) {
            root.send("music.track", { title: "" });
            return;
        }
        root.send("music.track", {
            title: root.title,
            artist: root.artist,
            album: root.album,
            artUrl: p.trackArtUrl || "",
            url: (p.metadata && p.metadata["xesam:url"]) ? String(p.metadata["xesam:url"]) : "",
            length: p.length > 0 ? p.length : 0,
            player: p.dbusName || ""
        });
    }

    function apply(line) {
        try {
            const f = JSON.parse(line);
            root.frame = f;
        } catch (e) {
            // A malformed frame must never wedge the sheet; keep the last one.
        }
    }

    function send(method, args) {
        ctl.queued += "call " + method + " " + JSON.stringify(args) + "\n";
        if (ctl.connected)
            ctl.flushQueued();
        else
            ctl.connected = true;
    }

    onPlayerChanged: {
        root.reanchor(root.player ? root.player.position : 0);
        root.push();
    }
    Connections {
        target: Media.player
        ignoreUnknownSignals: true
        function onTrackTitleChanged() { root.push(); }
        function onTrackArtistChanged() { root.push(); }
        function onTrackAlbumChanged() { root.push(); }
        function onTrackArtUrlChanged() { root.push(); }
        function onLengthChanged() { root.push(); }
        function onPostTrackChanged() {
            root.reanchor(0);
            root.push();
        }
    }

    Socket {
        id: sub
        path: root.sockPath
        parser: SplitParser { onRead: line => root.apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe music\n");
                flush();
                root.push();
            } else {
                retry.restart();
            }
        }
    }

    // The daemon may be down when the shell loads (or restart under it); retry
    // quietly so the sheet repopulates once it returns.
    Timer {
        id: retry
        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }

    Socket {
        id: ctl
        path: root.sockPath
        property string queued: ""

        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }

        onConnectionStateChanged: if (connected) flushQueued()
    }
}
