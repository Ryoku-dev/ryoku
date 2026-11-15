import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../nav"

// a piece of Settings at its real size, drawn in one design: real tree lines, a card of real
// rows with a switch, a choice and a slider, and the page's buttons. It can be used, so a
// design is tried by hand before it is chosen. Colours are the desktop's choice, the same in
// every design
DesignScope {
    id: root

    property alias design: root.lithicDesign
    readonly property var r: {
        Lithic.source;
        Lithic.mode;
        Lithic.contrast;
        Lithic.tint;
        return Lithic.rolesFor(root.lithicDesign);
    }
    readonly property var p: Tone.planes(root.lithicDesign, root.r)

    implicitHeight: 276

    CutRect {
        anchors.fill: parent
        cut: Tone.cardCut
        fill: root.p.side
        strokeWidth: 1
        strokeColor: Lithic.alpha(root.r.fg, 0.1)
        Behavior on fill { ColorAnimation { duration: Tone.move } }
    }

    Column {
        id: tree
        x: 10
        y: 12
        width: 196
        spacing: 2

        Repeater {
            model: [
                { glyph: "palette", text: "Appearance", folder: true },
                { glyph: "style", text: "Style", on: true },
                { glyph: "rounded_corner", text: "Window style" },
                { glyph: "match_case", text: "Fonts & icons" },
                { glyph: "dashboard", text: "Desktop", folder: true }
            ]
            delegate: NavRow {
                required property var modelData
                width: tree.width
                folder: modelData.folder === true
                expanded: modelData.text === "Appearance"
                depth: modelData.folder ? 0 : 1
                glyph: modelData.glyph
                text: I18n.tr(modelData.text)
                selected: modelData.on === true
                wash: modelData.on === true
                focusPolicy: Qt.NoFocus
            }
        }
    }

    CutRect {
        id: pane
        x: tree.x + tree.width + 12
        y: 8
        width: parent.width - x - 8
        height: parent.height - 16
        cut: Tone.paneCut
        fill: root.p.pane
        strokeWidth: 1
        strokeColor: root.p.paneStroke

        Label {
            id: heading
            x: 18
            y: 14
            text: I18n.tr("Sample section")
            typeRole: "section"
        }
        // a stamp in the corner, so the sample never reads as a setting
        Label {
            anchors.right: parent.right
            anchors.rightMargin: 18
            anchors.verticalCenter: heading.verticalCenter
            text: I18n.tr("Example")
            typeRole: "group"
            inkRole: "fg3"
            font.capitalization: Font.AllUppercase
            font.letterSpacing: 1.6
        }

        CutRect {
            id: card
            x: 18
            anchors.top: heading.bottom
            anchors.topMargin: 10
            width: parent.width - 36
            height: rows.implicitHeight + 8
            cut: Tone.cardCut
            fill: root.p.card
            strokeWidth: 1
            strokeColor: root.p.cardStroke

            Column {
                id: rows
                x: 16
                y: 4
                width: parent.width - 32

                Item {
                    width: rows.width
                    height: 46
                    Label {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("A switch")
                        typeRole: "row"
                    }
                    Toggle {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        checked: true
                        focusPolicy: Qt.NoFocus
                    }
                }
                Rectangle { width: rows.width; height: 1; color: Lithic.alpha(root.r.fg, 0.08) }
                Item {
                    width: rows.width
                    height: 50
                    Label {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("A choice")
                        typeRole: "row"
                    }
                    Segmented {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        model: [I18n.tr("One"), I18n.tr("Two"), I18n.tr("Three")]
                        currentIndex: 1
                        focusPolicy: Qt.NoFocus
                        onActivated: index => currentIndex = index
                    }
                }
                Rectangle { width: rows.width; height: 1; color: Lithic.alpha(root.r.fg, 0.08) }
                Item {
                    width: rows.width
                    height: 46
                    Label {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("A slider")
                        typeRole: "row"
                    }
                    Slider {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        width: Math.min(220, rows.width * 0.45)
                        from: 0
                        to: 30
                        value: 16
                        focusPolicy: Qt.NoFocus
                    }
                }
            }
        }

        Row {
            anchors.right: card.right
            anchors.top: card.bottom
            anchors.topMargin: 12
            spacing: 8
            Plaque {
                kind: "quiet"
                small: true
                text: I18n.tr("A button")
                focusPolicy: Qt.NoFocus
            }
            Plaque {
                small: true
                glyph: "star"
                text: I18n.tr("The action")
                focusPolicy: Qt.NoFocus
            }
        }
    }
}
