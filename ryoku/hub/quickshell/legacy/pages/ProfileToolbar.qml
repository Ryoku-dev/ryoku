pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

// The Profile edit toolbar: a bottom bar shown in EDIT mode. It carries only what
// has no home on the plate itself, the preset + hero side and the share /
// lifecycle actions. Blocks are toggled by the eye chips on the plate and text is
// click-to-edit in place, so they are not repeated here.
Rectangle {
    id: bar

    signal exportImage
    signal exportProfile
    signal importProfile
    signal resetAll
    signal done

    property bool confirmingReset: false

    implicitHeight: 52
    implicitWidth: (bar.confirmingReset ? resetRow.width : mainRow.width) + Tokens.s5 * 2
    radius: Tokens.radius
    color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.94)
    border.width: Tokens.border
    border.color: Tokens.lineStrong

    function preset() { return ProfileStore.get("preset", "full"); }

    // a preset writes only visibility + side; later per-block/text edits win.
    function applyPreset(key) {
        if (key === "MINIMAL")
            ProfileStore.put({
                "preset": "minimal",
                "blocks": {
                    "epithets": false, "telemetry": false, "packages": false,
                    "palette": false, "barcode": false, "signal": false,
                    "watermark": false, "specs": true, "marginalia": true
                }
            });
        else
            ProfileStore.put({
                "preset": "full",
                "blocks": {
                    "epithets": true, "telemetry": true, "specs": true,
                    "packages": true, "palette": true, "barcode": true,
                    "signal": true, "watermark": true, "marginalia": true
                }
            });
    }

    // ── the resting toolbar ────────────────────────────────────────────────────
    Row {
        id: mainRow
        anchors.centerIn: parent
        spacing: Tokens.s4
        visible: !bar.confirmingReset

        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("PRESET")
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: 9
            font.letterSpacing: 0.6
        }
        Seg {
            anchors.verticalCenter: parent.verticalCenter
            options: ["FULL", "MINIMAL"]
            current: bar.preset() === "minimal" ? "MINIMAL" : "FULL"
            onChose: key => bar.applyPreset(key)
        }

        Rectangle {
            anchors.verticalCenter: parent.verticalCenter
            width: Tokens.border
            height: 24
            color: Tokens.line
        }

        Btn {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("IMPORT")
            onAct: bar.importProfile()
        }
        Btn {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("EXPORT PROFILE")
            onAct: bar.exportProfile()
        }
        Btn {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("EXPORT IMAGE")
            onAct: bar.exportImage()
        }
        Btn {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("RESET")
            onAct: bar.confirmingReset = true
        }
        Btn {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("DONE")
            primary: true
            onAct: bar.done()
        }
    }

    // ── reset confirm (bone plate) ─────────────────────────────────────────────
    Row {
        id: resetRow
        anchors.centerIn: parent
        spacing: Tokens.s4
        visible: bar.confirmingReset

        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Reset to the stock plate? This clears your customization.")
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: 11
        }
        Btn {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("CANCEL")
            onAct: bar.confirmingReset = false
        }
        Btn {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("RESET")
            primary: true
            onAct: {
                bar.confirmingReset = false;
                bar.resetAll();
            }
        }
    }
}
