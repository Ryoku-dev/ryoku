pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import ".."
import "../look"

// Adwaita's capture panel, after the GNOME Shell screenshot UI: a card 36 above
// the bottom edge with its close button riding the top-right corner. Big toggle
// buttons pick what is taken (Selection, Window, Screen); below them the menu
// button, the round shutter in the middle, which takes the screen the panel is
// on, and a linked group of the five actions. It fades in as it rises 16 px;
// button backgrounds change in 100 ms, with no springs.
Item {
    id: root

    required property Item bar
    readonly property real u: root.bar.u
    readonly property real reveal: root.appear
    readonly property rect plane: Qt.rect(card.x, card.restY, card.width, card.height)

    property real appear: 0
    Behavior on appear {
        NumberAnimation {
            duration: Look.dur(200)
            easing.type: Easing.OutQuad
        }
    }
    Component.onCompleted: root.appear = Qt.binding(() => root.bar.shown ? 1 : 0)

    readonly property real padding: 12 * root.u
    readonly property real gap: 12 * root.u
    readonly property real btn: 36
    readonly property real side: Math.max(leftGroup.implicitWidth, rightGroup.implicitWidth)

    Item {
        id: card

        readonly property real restY: root.height - height - Math.round(36 * root.u)
        width: Math.ceil(Math.max(types.implicitWidth, root.side * 2 + shutter.width + root.gap * 2) + root.padding * 2)
        height: Math.ceil(types.implicitHeight + root.gap + shutter.height + root.padding * 2)
        x: Math.round((root.width - width) / 2)
        y: card.restY + Math.round(16 * root.u) * (1 - root.appear)
        opacity: root.appear
        enabled: root.bar.shown

        MouseArea {
            anchors.fill: parent
            hoverEnabled: true
        }

        Surface {
            anchors.fill: parent
            u: root.u
            radius: 24 * root.u
            elevation: 3
        }

        // the three big toggles: what is taken
        Row {
            id: types
            anchors.horizontalCenter: parent.horizontalCenter
            y: root.padding
            spacing: Math.round(6 * root.u)

            Repeater {
                model: [
                    { id: "region", icon: "region", label: I18n.tr("Selection") },
                    { id: "window", icon: "window", label: I18n.tr("Window") },
                    { id: "monitor", icon: "screen", label: I18n.tr("Screen") }
                ]
                delegate: Item {
                    id: type
                    required property var modelData
                    readonly property bool current: root.bar.target === type.modelData.id
                    visible: type.modelData.id !== "window" || root.bar.windowTarget
                    width: Math.round(96 * root.u)
                    height: Math.round(68 * root.u)

                    Rectangle {
                        anchors.fill: parent
                        radius: 12 * root.u
                        color: type.current ? (typeArea.containsMouse ? Lithic.alpha(Look.fg, 0.2) : Look.checkedFill)
                            : typeArea.pressed ? Look.press : typeArea.containsMouse ? Lithic.alpha(Look.fg, 0.07) : "transparent"
                        Behavior on color {
                            ColorAnimation { duration: Look.dur(100) }
                        }
                    }
                    Column {
                        anchors.centerIn: parent
                        spacing: Math.round(6 * root.u)
                        Icon {
                            anchors.horizontalCenter: parent.horizontalCenter
                            name: type.modelData.icon
                            size: 24 * root.u
                            tint: Look.fg
                        }
                        Text {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: type.modelData.label
                            color: Look.fg
                            font.family: Look.sans
                            font.pixelSize: Math.round(13 * root.u)
                            font.weight: type.current ? Font.Bold : Font.Normal
                            renderType: Text.CurveRendering
                        }
                    }
                    MouseArea {
                        id: typeArea
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.bar.targetPicked(type.modelData.id)
                    }
                }
            }
        }

        // left: the record toggles and the menu button
        Row {
            id: leftGroup
            x: root.padding
            anchors.verticalCenter: shutter.verticalCenter
            spacing: Math.round(4 * root.u)

            GlyphButton {
                visible: root.bar.recording
                u: root.u; cellWidth: root.btn; cellHeight: root.btn; iconSize: 18
                icon: "sound"; checked: root.bar.audioOn
                tip: I18n.tr("Record system audio")
                onClicked: root.bar.audioToggled()
            }
            GlyphButton {
                visible: root.bar.recording
                u: root.u; cellWidth: root.btn; cellHeight: root.btn; iconSize: 18
                icon: "mic"; checked: root.bar.micOn
                tip: I18n.tr("Record microphone audio")
                onClicked: root.bar.micToggled()
            }
            GlyphButton {
                id: menuButton
                u: root.u; cellWidth: root.btn; cellHeight: root.btn; iconSize: 18
                icon: "menu"
                checked: root.bar.menu === "more"
                tip: I18n.tr("Options")
                onClicked: root.bar.menuRequested(root.bar.menu === "more" ? "" : "more")
            }
            Badge {
                visible: root.bar.delaySeconds > 0
                anchors.verticalCenter: parent.verticalCenter
                u: root.u
                text: root.bar.delaySeconds + "s"
            }
        }

        // the shutter: a ring around a disc, red for a recording
        Item {
            id: shutter
            anchors.horizontalCenter: parent.horizontalCenter
            y: types.y + types.height + root.gap
            width: Math.round(56 * root.u)
            height: width

            Rectangle {
                anchors.fill: parent
                radius: width / 2
                color: "transparent"
                border.width: Math.round(3 * root.u)
                border.color: Look.fg
            }
            Rectangle {
                anchors.centerIn: parent
                width: Math.round(44 * root.u) * (shutterArea.pressed ? 0.9 : 1)
                height: width
                radius: width / 2
                color: root.bar.recording ? Look.alert
                    : shutterArea.containsMouse ? Lithic.mix(Look.fg, Look.plane, 0.12) : Look.fg
                Behavior on color {
                    ColorAnimation { duration: Look.dur(100) }
                }
                Behavior on width {
                    NumberAnimation { duration: Look.dur(100); easing.type: Easing.OutQuad }
                }
            }
            MouseArea {
                id: shutterArea
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: root.bar.screenRequested()
            }
            Tip {
                z: 100
                u: root.u
                anchors.horizontalCenter: parent.horizontalCenter
                y: -height - 10 * root.u
                text: root.bar.recording ? I18n.tr("Record this screen") : I18n.tr("Capture this monitor")
                keyHint: "\u23ce"
                shown: shutterArea.containsMouse
            }
        }

        // right: the five actions as one linked group
        Row {
            id: rightGroup
            anchors.right: parent.right
            anchors.rightMargin: root.padding
            anchors.verticalCenter: shutter.verticalCenter
            spacing: Math.max(1, Math.round(root.u))

            Repeater {
                model: root.bar.actions
                delegate: Item {
                    id: link
                    required property var modelData
                    required property int index
                    readonly property bool current: root.bar.activeAction === link.modelData.id
                    readonly property bool first: link.index === 0
                    readonly property bool last: link.index === root.bar.actions.length - 1
                    readonly property real r: 6 * root.u
                    width: Math.round(root.btn * root.u)
                    height: Math.round(32 * root.u)

                    Rectangle {
                        anchors.fill: parent
                        color: link.current ? (linkArea.containsMouse ? Lithic.alpha(Look.fg, 0.2) : Look.checkedFill)
                            : linkArea.pressed ? Look.press : linkArea.containsMouse ? Lithic.alpha(Look.fg, 0.12) : Lithic.alpha(Look.fg, 0.07)
                        topLeftRadius: link.first ? link.r : 0
                        bottomLeftRadius: link.first ? link.r : 0
                        topRightRadius: link.last ? link.r : 0
                        bottomRightRadius: link.last ? link.r : 0
                        Behavior on color {
                            ColorAnimation { duration: Look.dur(100) }
                        }
                    }
                    Icon {
                        anchors.centerIn: parent
                        name: link.modelData.id === "ocr" ? "text" : link.modelData.icon
                        size: 16 * root.u
                        tint: Look.fg
                    }
                    MouseArea {
                        id: linkArea
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.bar.actionPicked(link.modelData.id)
                    }
                    Tip {
                        z: 100
                        u: root.u
                        anchors.horizontalCenter: parent.horizontalCenter
                        y: -height - 8 * root.u
                        text: link.modelData.label
                        keyHint: String(link.index + 1)
                        shown: linkArea.containsMouse
                    }
                }
            }
        }

        // the close button sits on the card's top-right corner
        Item {
            x: card.width - width / 2 - Math.round(4 * root.u)
            y: -height / 2 + Math.round(4 * root.u)
            width: Math.round(32 * root.u)
            height: width

            Rectangle {
                anchors.fill: parent
                radius: width / 2
                color: closeArea.pressed ? Lithic.mix(Look.stone3, Look.fg, 0.2)
                    : closeArea.containsMouse ? Lithic.mix(Look.stone3, Look.fg, 0.12) : Look.stone3
                border.width: Math.max(1, Math.round(root.u))
                border.color: Look.planeStroke
                Behavior on color {
                    ColorAnimation { duration: Look.dur(100) }
                }
            }
            Icon {
                anchors.centerIn: parent
                name: "cancel"
                size: 14 * root.u
                tint: Look.fg
            }
            MouseArea {
                id: closeArea
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: root.bar.closeRequested()
            }
        }
    }

    // the popover opens above the whole card, over the menu button, never across the toggles
    Item {
        id: menuAnchor
        x: card.x + leftGroup.x + menuButton.x
        y: card.restY
        width: menuButton.width
        height: 0
    }

    Menu {
        u: root.u
        anchor: menuAnchor
        bounds: root
        open: root.bar.menu === "more" && root.bar.shown
        onDismissed: root.bar.menuRequested("")

        MenuItem { u: root.u; kind: "heading"; text: I18n.tr("Delay") }
        Repeater {
            model: root.bar.delays
            delegate: MenuItem {
                required property int modelData
                u: root.u
                checkable: "radio"
                checked: root.bar.delaySeconds === modelData
                text: root.bar.delayLabel(modelData)
                onTriggered: root.bar.delayPicked(modelData)
            }
        }
        MenuItem { u: root.u; kind: "separator"; visible: root.bar.recording }
        MenuItem {
            visible: root.bar.recording
            u: root.u; checkable: "check"; checked: root.bar.keypressesOn
            text: I18n.tr("Show key presses")
            onTriggered: root.bar.keypressesToggled()
        }
        MenuItem {
            visible: root.bar.recording
            u: root.u; checkable: "check"; checked: root.bar.webcamOn
            text: I18n.tr("Show webcam")
            onTriggered: root.bar.webcamToggled()
        }
        MenuItem { u: root.u; kind: "separator" }
        MenuItem {
            u: root.u; text: I18n.tr("Pick a colour"); keyHint: "C"
            onTriggered: root.bar.colorPickRequested()
        }
        MenuItem {
            u: root.u; text: I18n.tr("Open screenshots folder")
            onTriggered: root.bar.folderRequested()
        }
        MenuItem {
            u: root.u; text: I18n.tr("Settings")
            onTriggered: root.bar.settingsRequested()
        }
    }
}
