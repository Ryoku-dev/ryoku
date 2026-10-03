pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Bluetooth
import shell.services

// One renderer parameterised by statusId, exactly as the reference treats each
// status widget as its own type. The battery, network, bluetooth and the speaker
// open their popout card; notifications opens the left sidebar; the mic mutes on
// click; both audio widgets take a hover wheel for volume. Icon rules and
// self-hide are the reference literals (contract 04 sec 3.2).
Item {
    id: root

    required property string edge
    required property real scale
    required property string statusId
    signal menuRequested(string id, rect ownerRect)

    readonly property bool audioOut: statusId === "audio-output"
    readonly property bool audioIn: statusId === "audio-input"

    // battery self-hides when no battery device is present; everything else is
    // always visible (contract 04 sec 3.2). selfShown is private, so an ancestor
    // collapsing the bar (which pulls `visible` to false) cannot zero the size.
    readonly property bool selfShown: statusId !== "battery" || Battery.present
    visible: selfShown
    implicitWidth: selfShown ? btn.implicitWidth : 0
    implicitHeight: selfShown ? btn.implicitHeight : 0

    // --- icon rules (contract 04 sec 3.2), literal thresholds ----------------

    // audio: muted wins, then >66 high, >33 medium, >0 low, ==0 muted.
    function audioOutGlyph(pct, muted) {
        if (muted)
            return "audio-volume-muted";
        return pct > 66 ? "audio-volume-high" : pct > 33 ? "audio-volume-medium"
            : pct > 0 ? "audio-volume-low" : "audio-volume-muted";
    }
    // microphone sensitivity levels share the audio buckets.
    function audioInGlyph(pct, muted) {
        if (muted)
            return "microphone-sensitivity-muted";
        return pct > 66 ? "microphone-sensitivity-high" : pct > 33 ? "microphone-sensitivity-medium"
            : pct > 0 ? "microphone-sensitivity-low" : "microphone-sensitivity-muted";
    }
    // battery: buckets of ten, with a charging variant; a full battery on AC
    // still counts as charging.
    function batteryGlyph(pct, charging) {
        const b = pct > 99 ? 100 : pct > 90 ? 90 : pct > 80 ? 80 : pct > 70 ? 70
            : pct > 60 ? 60 : pct > 50 ? 50 : pct > 40 ? 40 : pct > 30 ? 30
            : pct > 20 ? 20 : pct > 10 ? 10 : 0;
        return "battery-level-" + b + (charging ? "-charging" : "");
    }
    // wifi strength: >75 excellent, >50 good, >25 ok, >0 weak, else none.
    function networkGlyph() {
        if (Network.kind === "ethernet")
            return "network-wired";
        if (Network.kind === "wifi") {
            const s = Network.level * 100;
            return "network-wireless-signal-" + (s > 75 ? "excellent" : s > 50 ? "good"
                : s > 25 ? "ok" : s > 0 ? "weak" : "none");
        }
        return Network.wifiRadio ? "network-wireless-offline" : "network-wireless-disabled";
    }
    function bluetoothGlyph() {
        const a = Bluetooth.defaultAdapter;
        if (!a)
            return "bluetooth-hardware-disabled";
        return a.enabled ? "bluetooth-active" : "bluetooth-disabled";
    }

    readonly property bool hasNotifs: Notifs.tracked.length > 0
    readonly property string glyph: {
        if (statusId === "battery")
            return batteryGlyph(Battery.pct, Battery.charging || Battery.full);
        if (statusId === "network")
            return networkGlyph();
        if (statusId === "bluetooth")
            return bluetoothGlyph();
        if (statusId === "audio-output")
            return audioOutGlyph(Audio.sink && Audio.sink.audio ? Math.round(Audio.sink.audio.volume * 100) : 0,
                Audio.sink && Audio.sink.audio ? Audio.sink.audio.muted : false);
        if (statusId === "audio-input")
            return audioInGlyph(Audio.source && Audio.source.audio ? Math.round(Audio.source.audio.volume * 100) : 0,
                Audio.source && Audio.source.audio ? Audio.source.audio.muted : false);
        return hasNotifs ? "notification-alert" : "notification";
    }

    // --- interaction (contract 04 sec 3.2, sec 4) ----------------------------

    function primary() {
        if (audioOut) {
            root.menuRequested("audio", Qt.rect(btn.x, btn.y, btn.width, btn.height));
        } else if (audioIn) {
            if (Audio.source && Audio.source.audio)
                Audio.source.audio.muted = !Audio.source.audio.muted;
        } else if (statusId === "notifications") {
            ShellState.requestSurfaceActive("sidebar-left#notices");
        } else if (statusId === "bluetooth") {
            root.menuRequested("bluetooth", Qt.rect(btn.x, btn.y, btn.width, btn.height));
        } else if (statusId === "battery") {
            root.menuRequested("battery", Qt.rect(btn.x, btn.y, btn.width, btn.height));
        } else if (statusId === "network") {
            root.menuRequested("network", Qt.rect(btn.x, btn.y, btn.width, btn.height));
        }
    }

    // step +-0.05 clamped [0,1]; wheel up increases. Output routes through the
    // daemon so the OSD + feedback sound fire (reference plays a sound on output
    // only); input is a silent direct set on the source.
    function scrollBy(steps) {
        if (audioOut) {
            Quickshell.execDetached(["ryoku-shell", "audio", steps > 0 ? "up" : "down"]);
        } else if (audioIn && Audio.source && Audio.source.audio) {
            const v = Audio.source.audio.volume + (steps > 0 ? 0.05 : -0.05);
            Audio.source.audio.volume = Math.max(0, Math.min(1, v));
        }
    }

    RailButton {
        id: btn
        anchors.centerIn: parent
        edge: root.edge
        scale: root.scale
        icon: root.glyph
        interactive: true
        scrollable: root.audioOut || root.audioIn
        onClicked: root.primary()
        onScrolled: steps => root.scrollBy(steps)
    }

    // Unread badge: a small red dot near the bell's top-right corner (contract
    // 04 sec 3.2, bar parity spec sec 4). The bell itself stays an outline glyph.
    Rectangle {
        visible: root.statusId === "notifications" && root.hasNotifs
        width: 5 * btn.glyphScale
        height: 5 * btn.glyphScale
        radius: width / 2
        color: Theme.error
        anchors.horizontalCenter: btn.horizontalCenter
        anchors.verticalCenter: btn.verticalCenter
        // Tracks the glyph so the badge stays pinned to the bell's corner rather
        // than drifting inward once the glyph stops shrinking with the band.
        anchors.horizontalCenterOffset: 5 * btn.glyphScale
        anchors.verticalCenterOffset: -5 * btn.glyphScale
    }
}
