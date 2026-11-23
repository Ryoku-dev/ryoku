pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "../kit" as K
import "lib/rows.js" as Rows

// The foot: keycap hints for where the keyboard is, the result count, and a
// short flash ("Copied") that stands in for the hints.
Item {
    id: root

    property var engine: null
    property bool actionsFocused: false
    // the window Shift+Delete would stop, "" when none is in reach
    property string killTarget: ""
    property string flashText: ""

    readonly property var hints: {
        const kill = root.killTarget.length > 0 ? [[I18n.tr("Shift+Del"), I18n.tr("Kill")]] : [];
        if (root.actionsFocused)
            return [[I18n.tr("Enter"), I18n.tr("Run")]].concat(kill, [[I18n.tr("Esc"), I18n.tr("Back")]]);
        return [
            [I18n.tr("Enter"), I18n.tr("Open")],
            [I18n.tr("Tab"), I18n.tr("Actions")]
        ].concat(kill, [
            [I18n.tr("Alt+1-9"), I18n.tr("Pick")],
            ["=", I18n.tr("Calculate")],
            ["?", I18n.tr("Web")],
            ["/", I18n.tr("Actions")],
            [I18n.tr("Esc"), I18n.tr("Close")]
        ]);
    }

    function flash(text) {
        root.flashText = String(text || "");
        flashTimer.restart();
    }

    implicitHeight: 34

    Timer {
        id: flashTimer
        interval: 1200
        onTriggered: root.flashText = ""
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: K.Style.border
        visible: K.Look.lithic || K.Look.fluent
        color: K.Look.lithic ? K.Style.line : K.Style.lineSoft
    }

    Row {
        id: hintRow
        readonly property real room: count.x - K.Style.s3
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        spacing: K.Style.s3
        opacity: root.flashText.length === 0 ? 1 : 0

        Behavior on opacity {
            enabled: !K.Style.still
            NumberAnimation { duration: K.Style.quick }
        }

        Repeater {
            model: root.hints

            delegate: Row {
                id: hint

                required property var modelData

                spacing: K.Style.s1
                opacity: hint.x + hint.width <= hintRow.room ? 1 : 0

                KeyCap {
                    anchors.verticalCenter: parent.verticalCenter
                    text: String(hint.modelData[0])
                }

                K.Label {
                    anchors.verticalCenter: parent.verticalCenter
                    text: String(hint.modelData[1])
                    role: "caption"
                    tone: "muted"
                }
            }
        }
    }

    K.Label {
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        text: root.flashText
        role: "label"
        tone: "ink"
        opacity: root.flashText.length > 0 ? 1 : 0

        Behavior on opacity {
            enabled: !K.Style.still
            NumberAnimation { duration: K.Style.quick }
        }
    }

    Text {
        id: count
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        text: I18n.tr("%1 results").arg(Rows.countLabel(root.engine ? root.engine.resultCount : 0, K.Look.lithic))
        color: K.Style.inkMuted
        font.family: K.Look.lithic ? K.Style.mono : K.Style.font
        font.pixelSize: K.Style.caption
        font.features: K.Style.tabular
        font.letterSpacing: K.Look.lithic ? K.Style.track : 0
        font.capitalization: K.Look.lithic ? Font.AllUppercase : Font.MixedCase
        renderType: Text.NativeRendering
    }
}
