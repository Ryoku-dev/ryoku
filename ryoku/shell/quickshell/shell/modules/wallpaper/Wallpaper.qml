pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io

/**
 * Ryoku wallpaper topic bridge, one instance per monitor (the shell root's
 * per-screen scope constructs it with `screen`).
 *
 * Ryogami (the Go wallpaper daemon) publishes one coalesced full-state frame
 * {default: ENTRY, outputs: {connector: ENTRY}} per revision on the `wallpaper`
 * topic of $XDG_RUNTIME_DIR/ryogami.sock. This bridge subscribes and re-exposes
 * this output's entry (outputs[screen.name] or, absent an override, default)
 * as the wallpaper/video urls and fit that the desktop's backdrop
 * (modules/desktop -> WallpaperMod.Backdrop) paints: stills with the reveal
 * transition and live clips through the in-shell QtMultimedia player. Stage
 * cuts and renders its own subject from the daemon `stage` topic; the frame's
 * `depth` fold stays ryogami's pixel-lock subject, unchanged on the wire
 * (docs/stage.md). Contract 08 sec 1, 2.6, 5, 7.
 *
 * The ryogami wallpaper picker (Super+W) sets wallpapers through ryogami,
 * which feeds this same topic.
 */
Item {
    id: root

    // The monitor this bridge tracks, supplied by the shell root's per-screen
    // scope (contract 08 sec 7: hotplug adds a monitor -> a new instance here).
    required property var screen

    WallpaperFrame {
        id: frame
        screenName: root.screen ? root.screen.name : ""
    }
    readonly property string wallpaperUrl: frame.path.length > 0
        ? "file://" + frame.path + "?v=" + frame.revision : ""
    readonly property string wallpaperPath: frame.path
    readonly property string fit: frame.fit
    // The reveal preset for the current revision (null = plain crossfade).
    readonly property var transition: frame.transition
    // The video clip for a live wallpaper ("" for a still).
    readonly property string videoUrl: frame.videoPath.length > 0
        ? "file://" + frame.videoPath : ""
    // The in-shell clip's audio, threaded to the backdrop's player: muted by
    // default, volume 0-100 (Backdrop scales it to 0..1).
    readonly property bool videoMuted: frame.mute
    readonly property int videoVolume: frame.volume
    readonly property bool videoPaused: frame.paused
    // The ryogami-live yield flag: hide the in-shell painter while the C
    // player owns the background layer; false for the in-shell engine.
    readonly property bool live: frame.live
    // The first topic frame gates readiness, not the in-shell decode: the
    // backdrop itself waits on the Image decode before revealing.
    readonly property bool reloadReady: frame.ready

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryogami.sock"

    function apply(line: string): void {
        frame.apply(line);
    }

    // QLocalSocket can retain an error from a failed reconnect after a daemon
    // restart. Recreate the socket for each attempt so an old peer or error
    // cannot detach a subscription that has just connected.
    function reconnect(): void {
        if (subscriber.active)
            subscriber.active = false;
    }

    Loader {
        id: subscriber
        active: true
        sourceComponent: Socket {
            path: root.sockPath
            parser: SplitParser {
                onRead: line => root.apply(line)
            }
            Component.onCompleted: connected = true
            onConnectionStateChanged: {
                if (connected) {
                    write("subscribe wallpaper\n");
                    flush();
                } else {
                    root.reconnect();
                }
            }
            onError: root.reconnect()
        }
    }

    // Ryogami may be down while deploy or update replaces it. Every retry gets
    // a new socket and therefore a fresh retained frame and subscription.
    Timer {
        id: retry
        interval: 500
        repeat: true
        running: !subscriber.active
        onTriggered: subscriber.active = true
    }
}
