pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons

// One state source feeds both the shipped visuals and installed designs.
// Volume, microphone, and brightness all read the daemon's `audio`/`osd`
// topics, and keyboard layout comes from the shared layout service.
//
// A fresh shell receives initial device values as it connects. Those signals
// are not user actions, so volume, microphone and brightness arm after a short
// settle instead of flashing at login. The explicit preview is the only event
// allowed through that startup grace.
QtObject {
    id: root

    property string kind: "volume"
    property bool suppressed: false

    readonly property bool isVolume: kind === "volume"
    readonly property bool isMic: kind === "mic"
    readonly property bool isBrightness: kind === "brightness"
    readonly property bool isKeyboard: kind === "keyboard"

    readonly property bool muted: isMic ? OsdFeed.micMuted : OsdFeed.muted
    // Audio boost can legitimately reach 150%, so the scale must keep unity
    // visible instead of making every boosted keypress look identical.
    readonly property real maxValue: (isVolume && Config.audio
        && Config.audio.boost === true) ? 1.5 : 1.0
    readonly property real value: isBrightness
        ? OsdFeed.brightness
        : Math.max(0, Math.min(root.maxValue,
            isMic ? OsdFeed.micVolume : OsdFeed.volume))
    readonly property bool over: root.value > 1.005
    readonly property string layout: KeyboardLayout.variant || I18n.tr("Unknown")
    readonly property string label: isKeyboard
        ? root.layout : Math.round(root.value * 100) + "%"

    // Muted wins, then audio and microphone use the reference 66/33/0 buckets.
    // Brightness uses high above 66, medium above 33 and low below that.
    readonly property string iconName: {
        if (isKeyboard)
            return "input-keyboard";
        const pct = Math.round(value * 100);
        if (isBrightness)
            return pct > 66 ? "brightness-high" : pct > 33 ? "brightness-medium" : "brightness-low";
        if (isMic)
            return (muted || pct <= 0) ? "microphone-sensitivity-muted"
                : pct > 66 ? "microphone-sensitivity-high"
                : pct > 33 ? "microphone-sensitivity-medium"
                : "microphone-sensitivity-low";
        return (muted || pct <= 0) ? "audio-volume-muted"
            : pct > 66 ? "audio-volume-high"
            : pct > 33 ? "audio-volume-medium"
            : "audio-volume-low";
    }

    // Each accepted trigger restarts the hold. The keyboard layout arrives from
    // the compositor once at login like the devices do, so it takes the same
    // grace and only a later switch shows.
    property bool flashing: false
    property bool armed: false
    readonly property var gateSource: (root.isBrightness || root.isVolume
            || root.isMic) ? OsdFeed
        : (root.isKeyboard && KeyboardLayout.variant !== "") ? KeyboardLayout : null

    function armWhenReady() {
        if (root.gateSource && !root.armed && !root.armTimer.running)
            root.armTimer.restart();
    }

    function flash(bypassGrace) {
        if (root.suppressed || (!root.armed && bypassGrace !== true))
            return;
        root.flashing = true;
        root.hideTimer.restart();
    }

    onGateSourceChanged: root.armWhenReady()
    onKindChanged: root.armWhenReady()
    onSuppressedChanged: if (suppressed) {
        root.hideTimer.stop();
        flashing = false;
    }
    Component.onCompleted: root.armWhenReady()

    property Timer armTimer: Timer {
        interval: 700
        onTriggered: root.armed = true
    }

    property Timer hideTimer: Timer {
        interval: Motion.osdHide
        onTriggered: root.flashing = false
    }

    property Connections audioEvents: Connections {
        target: (root.isVolume || root.isMic) ? OsdFeed : null
        function onVolumeChanged() {
            if (root.isVolume)
                root.flash(false);
        }
        function onMutedChanged() {
            if (root.isVolume)
                root.flash(false);
        }
        function onMicVolumeChanged() {
            if (root.isMic && OsdFeed.acceptsMicVolume(OsdFeed.micVolume))
                root.flash(false);
        }
        function onMicMutedChanged() {
            if (root.isMic)
                root.flash(false);
        }
    }

    property Connections brightnessEvents: Connections {
        target: root.isBrightness ? OsdFeed : null
        function onBrightnessSeqChanged() { root.flash(false); }
    }

    property Connections previewEvents: Connections {
        target: root.isVolume ? OsdFeed : null
        function onPreviewSeqChanged() { root.flash(true); }
    }
    property Connections micRequests: Connections {
        target: root.isMic ? OsdFeed : null
        function onMicRequested() { root.flash(true); }
    }

    // KeyboardLayout only carries the layout name; the old signatureChanged
    // handler named a signal it never had, so a switch never showed.
    property Connections keyboardEvents: Connections {
        target: root.isKeyboard ? KeyboardLayout : null
        function onVariantChanged() { root.flash(false); }
    }
}
