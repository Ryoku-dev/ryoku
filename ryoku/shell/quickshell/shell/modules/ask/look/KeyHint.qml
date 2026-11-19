pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../lib/keys.js" as Chords

// A chord as key caps ("Ctrl+K" is two caps). Keys with a familiar mark draw it;
// modifiers stay words.
Row {
    id: root

    property real s: 1
    property string hint: ""
    property color ink: Lithic.fg3

    readonly property var marks: ({
        "Enter": "keyboard_return", "Tab": "keyboard_tab", "Backspace": "backspace",
        "Up": "arrow_upward", "Down": "arrow_downward", "Left": "arrow_back", "Right": "arrow_forward"
    })
    readonly property var words: ({
        "Ctrl": I18n.tr("Ctrl"), "Shift": I18n.tr("Shift"), "Alt": I18n.tr("Alt"), "Esc": I18n.tr("Esc"),
        "Space": I18n.tr("Space"), "Delete": I18n.tr("Del"), "PageUp": I18n.tr("PgUp"), "PageDown": I18n.tr("PgDn")
    })

    spacing: Math.round(3 * root.s)

    Repeater {
        model: Chords.caps(root.hint)

        delegate: Shape {
            id: cap

            required property string modelData
            readonly property string mark: root.marks[cap.modelData] || ""

            s: root.s
            cutSize: Look.cutToken
            height: Math.round(20 * root.s)
            width: Math.max(height, Math.ceil((cap.mark.length > 0 ? markGlyph.implicitWidth : word.implicitWidth) + 10 * root.s))
            fill: Look.cap
            strokeColor: Look.capStroke
            strokeWidth: Qt.colorEqual(Look.capStroke, "transparent") ? 0 : 1

            Symbol {
                id: markGlyph
                anchors.centerIn: parent
                visible: cap.mark.length > 0
                s: root.s
                name: cap.mark
                size: 13
                ink: root.ink
            }
            Text {
                id: word
                anchors.centerIn: parent
                visible: cap.mark.length === 0
                text: root.words[cap.modelData] || cap.modelData
                color: root.ink
                font.family: Look.monoMicro ? Look.mono : Look.sans
                font.pixelSize: Math.round((Look.monoMicro ? 9 : 11) * root.s)
                font.weight: Look.monoMicro ? Font.Normal : Font.Medium
                font.capitalization: Look.monoMicro ? Font.AllUppercase : Font.MixedCase
                renderType: Text.NativeRendering
            }
        }
    }
}
