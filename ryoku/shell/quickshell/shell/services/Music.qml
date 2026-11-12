pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Blobs
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../utils/menupoll.js" as MenuPoll

// The enriched now-playing track: the full-size cover the daemon resolved and
// this song's synced lyrics, plus the playback clock every lyric surface follows.
// Media picks the player; this pushes that pick to ryoku-shell (music.go), which
// owns the LRCLIB and cover-art fetches, and subscribes to the frame it streams
// back, so no surface here makes an HTTP request.
//
// `hold` is owner-refcounted like AudioBars: the native clock and position poll
// run only while a visible surface claims them. MPRIS never pushes position, so
// the clock interpolates between its correction polls.
Singleton {
    id: root

    property var frame: ({})
    readonly property string source: root.frame.source || ""
    readonly property string art: root.frame.art || ""
    // the track's looping backdrop (Spotify Canvas), a URL or path the daemon
    // resolved; empty when none. The widget streams it directly.
    readonly property string canvas: root.frame.canvas || ""
    readonly property bool hasArt: root.art.length > 0
    // The daemon publishes a filesystem path; Image wants a URL.
    readonly property string artUrl: root.hasArt ? "file://" + root.art : ""
    // never a web URL: an https Image runs Qt's TLS off the GUI thread and can segfault qs (quickshell#1368)
    function coverFor(player) {
        if (!player)
            return "";
        if (player === Media.player && root.hasArt)
            return root.artUrl;
        const raw = String(player.trackArtUrl || "");
        return raw.startsWith("file://") ? raw : "";
    }
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
    property alias anchor: playbackClock.anchorPosition
    property real anchorMs: 0
    property real elapsed: playbackClock.elapsed
    readonly property var player: Media.player
    readonly property bool playing: Media.playing
    readonly property real length: root.player && root.player.length > 0 ? root.player.length : 0
    readonly property real position: root.elapsed
    readonly property real duration: root.length

    PlaybackClock {
        id: playbackClock
        running: root.ticking
        interval: 60
        rate: 1.0
        length: root.length
        lineTimes: root.synced ? root.lines.map(line => Number(line.t)) : []
    }

    function reanchor(pos) {
        playbackClock.reanchor(pos);
        root.anchorMs = Date.now();
    }
    function estimate() {
        return playbackClock.elapsed;
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

    // MPRIS reports position only when asked. The native clock carries motion
    // between polls; a poll only moves its anchor when the player has drifted.
    readonly property bool ticking: root.held && root.playing && !Presence.dark
    function refresh() {
        root.player.positionChanged();
        const reported = root.player.position;
        if (Math.abs(reported - playbackClock.elapsed) > 0.35)
            root.reanchor(reported);
    }
    Timer {
        interval: 250
        repeat: true
        running: root.ticking && root.player !== null
        onTriggered: root.refresh()
    }
    onTickingChanged: {
        if (root.ticking && root.player)
            root.refresh();
    }

    // ── the current lyric line ───────────────────────────────────────────────
    // -1 until the first timestamp is reached, which is how a sheet knows to
    // show its intro rather than highlighting the opening line early.
    readonly property int index: root.synced ? playbackClock.lineIndex : -1

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

    function apply(f) {
        if (f && typeof f === "object")
            root.frame = f;
    }

    function send(method, args) {
        Bus.call(method, args);
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

    BusTopic {
        id: topicFeed
        topic: "music"
        onUpdated: root.apply(topicFeed.data)
    }

    // a restarted daemon needs the current track again for art and lyrics
    Connections {
        target: Bus
        function onConnectedChanged() {
            if (Bus.connected)
                root.push();
        }
    }

    Component.onCompleted: if (Bus.connected) root.push()
}
