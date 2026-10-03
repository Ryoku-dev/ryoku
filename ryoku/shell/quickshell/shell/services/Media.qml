pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Services.Mpris

// the one now-playing pick every surface shares: prefers a sounding player,
// falls back to the first real one. the live wallpaper (mpvpaper) registers
// on MPRIS too; a bare video filename is scenery, not music, so it never
// counts as a player here.
Singleton {
    id: root

    function isWallpaper(p) {
        return /\.(mp4|webm|mkv|gif)$/i.test(p.trackTitle || "");
    }
    // the ryoku live radio (launcher "@"): an mpv whose forced title carries
    // the LIVE prefix. Same signature the launcher matches on - a broadcast
    // gets a tally lamp instead of a seek bar (it has no position to show).
    function isRadio(p) {
        return !!p && String(p.dbusName || "").indexOf(".mpv") !== -1
            && String(p.trackTitle || "").indexOf("LIVE · ") === 0;
    }

    // Sticky pick: keep the player already on screen while it is still sounding,
    // so a second player (a browser tab, a game bleep, an Apple Music client next
    // to a video) cannot steal the pick and reload the cover and lyrics mid-song.
    // Only move on when the current one stops or leaves the set.
    function pick(players, cur) {
        var list = players.filter(function(p) { return p && !root.isWallpaper(p); });
        if (list.length === 0)
            return null;
        var curOk = cur && list.indexOf(cur) !== -1;
        if (curOk && cur.isPlaying)
            return cur;
        for (var i = 0; i < list.length; i++)
            if (list[i].isPlaying)
                return list[i];
        if (curOk && (cur.trackTitle || "").length > 0)
            return cur;
        for (var j = 0; j < list.length; j++)
            if ((list[j].trackTitle || "").length > 0)
                return list[j];
        return curOk ? cur : list[0];
    }
    property var player: null
    function repick() {
        var next = root.pick(Mpris.players.values, root.player);
        if (next !== root.player)
            root.player = next;
    }
    // Re-pick when the player set changes, and on a slow tick so the pick can
    // move off a player that just stopped (an isPlaying change does not alter the
    // set). The tick is deliberately unhurried: it settles the pick rather than
    // chasing every transient state, which is what strobed the surfaces before.
    // It only runs while at least one player exists: with an empty set there is
    // nothing to settle, and a session that never opened a player should never
    // wake for this.
    readonly property var playerSet: Mpris.players ? Mpris.players.values : []
    onPlayerSetChanged: root.repick()
    Timer { interval: 1000; repeat: true; running: root.playerSet.length > 0; onTriggered: root.repick() }
    Component.onCompleted: root.repick()
    readonly property bool playing: player !== null && player.isPlaying
    readonly property bool present: player !== null && (player.trackTitle || "").length > 0
    readonly property bool radio: player !== null && isRadio(player)
    readonly property string line: {
        if (!player)
            return "";
        var t = player.trackTitle || "";
        var a = Theme.joinArtists(player.trackArtists, player.trackArtist);
        return a.length > 0 ? t + " · " + a : t;
    }

    function toggle() {
        if (player && player.canTogglePlaying)
            player.togglePlaying();
    }
}
