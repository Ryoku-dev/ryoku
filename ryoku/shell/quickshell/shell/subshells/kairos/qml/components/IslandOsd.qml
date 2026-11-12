pragma ComponentBehavior: Bound

import QtQuick
import shell.barkit as Pill
import "../kit"

// same sources as ryoku's own osd so triggers and the login grace match
Item {
    id: osd

    property bool active: true
    property bool blocked: false

    readonly property bool flashing: volume.flashing || mic.flashing || brightness.flashing
    readonly property bool showing: osd.active && osd.flashing && !osd.blocked
    property string kind: "volume"
    readonly property var source: osd.kind === "mic" ? mic : osd.kind === "brightness" ? brightness : volume
    readonly property real wantedWidth: 268

    property real progress: osd.showing ? 1 : 0
    Behavior on progress {
        enabled: !Md.still
        Spring {}
    }

    readonly property real level: Math.max(0, Math.min(1, osd.source.value))
    readonly property string glyph: {
        var pct = Math.round(osd.source.value * 100);
        if (osd.kind === "brightness")
            return pct > 66 ? "brightness_high" : pct > 33 ? "brightness_medium" : "brightness_low";
        if (osd.kind === "mic")
            return (osd.source.muted || pct <= 0) ? "mic_off" : "mic";
        if (osd.source.muted || pct <= 0)
            return "volume_off";
        return pct < 50 ? "volume_down" : "volume_up";
    }

    Pill.OsdSource {
        id: volume
        kind: "volume"
        suppressed: !osd.active
        onFlashingChanged: if (flashing) osd.kind = "volume"
    }
    Pill.OsdSource {
        id: mic
        kind: "mic"
        suppressed: !osd.active
        onFlashingChanged: if (flashing) osd.kind = "mic"
    }
    Pill.OsdSource {
        id: brightness
        kind: "brightness"
        suppressed: !osd.active
        onFlashingChanged: if (flashing) osd.kind = "brightness"
    }

    visible: osd.progress > 0.01
    opacity: Math.max(0, Math.min(1, (osd.progress - 0.4) / 0.6))

    Row {
        anchors.centerIn: parent
        spacing: 12

        Icon {
            anchors.verticalCenter: parent.verticalCenter
            name: osd.glyph
            size: 18
            filled: true
            color: osd.source.muted ? Md.inkFaint : Md.ink
        }
        Rectangle {
            anchors.verticalCenter: parent.verticalCenter
            width: osd.wantedWidth - 120
            height: 6
            radius: 3
            color: Md.containerHighest

            Rectangle {
                width: parent.width * osd.level
                height: parent.height
                radius: parent.radius
                color: osd.source.over ? Md.error : osd.source.muted ? Md.inkFaint : Md.accent
                Behavior on width {
                    enabled: !Md.still
                    NumberAnimation { duration: Md.fast }
                }
            }
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            width: 38
            text: Math.round(osd.source.value * 100) + "%"
            color: Md.ink
            font.family: Md.mono
            font.pixelSize: 13
        }
    }
}
