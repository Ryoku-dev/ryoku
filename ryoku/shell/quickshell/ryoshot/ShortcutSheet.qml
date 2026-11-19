pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Lithic
import Ryoku.Ui.Singletons
import "look"

// Every key ryoshot answers, in the design's dialog: Lithic's cut plane with
// mono caps headings and cut key plates, Material's large card, Adwaita's
// dialog, Fluent's content dialog. A click outside or Escape closes it.
Item {
    id: sheet

    property real u: 1
    property bool open: false
    property var tools: []

    signal closeRequested()

    readonly property var actions: [
        { key: "Ctrl+C", label: I18n.tr("Copy") },
        { key: "Ctrl+S", label: I18n.tr("Save") },
        { key: "Enter", label: I18n.tr("Copy and save") },
        { key: "Ctrl+P", label: I18n.tr("Pin") },
        { key: "Ctrl+U", label: I18n.tr("Upload") },
        { key: "Ctrl+K", label: I18n.tr("Send to phone") },
        { key: "Ctrl+B", label: I18n.tr("Beautify") },
        { key: "Ctrl+Z", label: I18n.tr("Undo") },
        { key: "Ctrl+Shift+Z", label: I18n.tr("Redo") },
        { key: "Ctrl+A", label: I18n.tr("Whole monitor") },
        { key: "Space", label: I18n.tr("Cycle target") },
        { key: "f", label: I18n.tr("Fill") },
        { key: "k", label: I18n.tr("Sketch") },
        { key: "[ ]", label: I18n.tr("Width") },
        { key: "1..8", label: I18n.tr("Colour") },
        { key: "i", label: I18n.tr("Pick a colour from the shot") },
        { key: "Del", label: I18n.tr("Delete") },
        { key: "?", label: I18n.tr("Keyboard shortcuts") },
        { key: "Esc", label: I18n.tr("Back") }
    ]
    // inside the Beautify studio
    readonly property var studioKeys: [
        { key: "1..7", label: I18n.tr("Pick a look") },
        { key: "[ ]", label: I18n.tr("Padding") },
        { key: "Ctrl+C", label: I18n.tr("Copy") },
        { key: "Ctrl+S", label: I18n.tr("Save") },
        { key: "Esc", label: I18n.tr("Back to markup") }
    ]

    focus: sheet.open
    enabled: sheet.open
    visible: sheet.reveal > 0

    property real reveal: sheet.open ? 1 : 0
    Behavior on reveal {
        NumberAnimation {
            duration: Look.dur(sheet.open ? Look.enterMs : Look.exitMs)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: sheet.open ? (Look.material ? Look.springDefault : Look.enterCurve) : Look.exitCurve
        }
    }

    Keys.onPressed: (e) => {
        if (e.key === Qt.Key_Escape || e.key === Qt.Key_Question) {
            e.accepted = true;
            sheet.closeRequested();
        }
    }

    MouseArea {
        anchors.fill: parent
        onClicked: sheet.closeRequested()
    }

    component Section: Text {
        Layout.columnSpan: 2
        color: Look.lithic ? Look.fg3 : Look.fg2
        font.family: Look.lithic ? Look.mono : Look.sans
        font.pixelSize: Math.round((Look.lithic ? 9 : 12) * sheet.u)
        font.weight: Look.lithic ? Font.Bold : Font.DemiBold
        font.letterSpacing: Look.lithic ? Look.tracking * sheet.u : 0
        renderType: Text.CurveRendering
    }

    component ShortcutRow: Item {
        id: row
        property string label: ""
        property string key: ""
        implicitWidth: Math.round(220 * sheet.u)
        implicitHeight: Math.round(24 * sheet.u)

        Text {
            anchors.left: parent.left
            anchors.right: cap.left
            anchors.rightMargin: Math.round(12 * sheet.u)
            anchors.verticalCenter: parent.verticalCenter
            text: row.label
            color: Look.fg
            font.family: Look.sans
            font.pixelSize: Math.round(13 * sheet.u)
            elide: Text.ElideRight
            renderType: Text.CurveRendering
        }
        Item {
            id: cap
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            implicitWidth: Math.ceil(capText.implicitWidth + 12 * sheet.u)
            implicitHeight: Math.round(20 * sheet.u)
            width: implicitWidth
            height: implicitHeight

            CutRect {
                visible: Look.lithic
                anchors.fill: parent
                design: "lithic"
                cut: 3 * sheet.u
                fill: Look.well
                strokeColor: Look.edge
                strokeWidth: Math.max(1, Math.round(sheet.u))
            }
            Rectangle {
                visible: !Look.lithic
                anchors.fill: parent
                radius: (Look.material ? 6 : Look.adwaita ? 6 : 4) * sheet.u
                color: Look.material ? Look.container : Lithic.alpha(Look.fg, 0.08)
                border.width: Look.fluent ? Math.max(1, Math.round(sheet.u)) : 0
                border.color: Look.planes.cardStroke
            }
            Text {
                id: capText
                anchors.centerIn: parent
                text: Look.lithic ? Look.keyHint(row.key).toUpperCase() : row.key
                color: Look.material ? Look.containerInk : Look.fg2
                font.family: Look.lithic ? Look.mono : Look.sans
                font.pixelSize: Math.round((Look.lithic ? 10 : 11.5) * sheet.u)
                font.weight: Font.Medium
                renderType: Text.CurveRendering
            }
        }
    }

    Item {
        id: card
        anchors.centerIn: parent
        width: Math.ceil(body.implicitWidth + 2 * card.pad)
        height: Math.ceil(body.implicitHeight + 2 * card.pad)
        readonly property real pad: (Look.lithic ? 22 : Look.material ? 24 : 24) * sheet.u

        opacity: Math.min(1, sheet.reveal * (Look.material ? 2 : 1))
        scale: Look.material ? 0.9 + 0.1 * sheet.reveal : Look.lithic ? 1 : 0.96 + 0.04 * sheet.reveal
        transform: Translate { y: (Look.lithic ? 8 : Look.fluent ? 12 : 0) * sheet.u * (1 - Math.min(1, sheet.reveal)) }

        MouseArea { anchors.fill: parent }

        Surface {
            anchors.fill: parent
            u: sheet.u
            cut: Look.cutLg
            radius: (Look.material ? 28 : Look.adwaita ? 14 : 8) * sheet.u
            elevation: 3
        }

        ColumnLayout {
            id: body
            x: card.pad
            y: card.pad
            spacing: Math.round(14 * sheet.u)

            Text {
                text: I18n.tr("Shortcuts")
                color: Look.fg
                font.family: Look.sans
                font.pixelSize: Math.round((Look.lithic ? 16 : Look.material ? 22 : 18) * sheet.u)
                font.weight: Look.lithic ? Font.Medium : Font.DemiBold
                renderType: Text.CurveRendering
            }

            GridLayout {
                columns: 2
                columnSpacing: Math.round(40 * sheet.u)
                rowSpacing: Math.round(6 * sheet.u)

                Section { text: Look.lithic ? I18n.tr("Tools").toUpperCase() : I18n.tr("Tools") }
                Repeater {
                    model: sheet.tools
                    ShortcutRow {
                        required property var modelData
                        label: modelData.label
                        key: modelData.key
                    }
                }
            }

            Divider { Layout.fillWidth: true; u: sheet.u; vertical: false }

            GridLayout {
                columns: 2
                columnSpacing: Math.round(40 * sheet.u)
                rowSpacing: Math.round(6 * sheet.u)

                Section { text: Look.lithic ? I18n.tr("Actions").toUpperCase() : I18n.tr("Actions") }
                Repeater {
                    model: sheet.actions
                    ShortcutRow {
                        required property var modelData
                        label: modelData.label
                        key: modelData.key
                    }
                }
            }

            Divider { Layout.fillWidth: true; u: sheet.u; vertical: false }

            GridLayout {
                columns: 2
                columnSpacing: Math.round(40 * sheet.u)
                rowSpacing: Math.round(6 * sheet.u)

                Section { text: Look.lithic ? I18n.tr("Beautify").toUpperCase() : I18n.tr("Beautify") }
                Repeater {
                    model: sheet.studioKeys
                    ShortcutRow {
                        required property var modelData
                        label: modelData.label
                        key: modelData.key
                    }
                }
            }
        }
    }
}
