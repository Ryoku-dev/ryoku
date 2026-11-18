pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import ".."
import "../look"

// Fluent's capture toolbar, after the Windows 11 Snipping Tool: an acrylic card
// 16 below the top edge with subtle buttons. The chosen action and the chosen
// target each wear a subtle fill and an accent pill under the glyph that slides
// between them. Full screen takes the screen at once, as on Windows. The delay
// is a drop-down; colour, folder and settings sit behind "…". It slides down as
// it fades in and leaves quicker than it came.
Item {
    id: root

    required property Item bar
    readonly property real u: root.bar.u
    readonly property real reveal: root.enter
    readonly property rect plane: Qt.rect(card.x, card.restY, card.width, card.height)

    property real enter: 0
    Behavior on enter {
        id: enterMotion
        NumberAnimation {
            duration: Look.dur(enterMotion.targetValue > 0.5 ? 250 : 150)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: enterMotion.targetValue > 0.5 ? Look.fluentIn : Look.fluentOut
        }
    }
    Component.onCompleted: root.enter = Qt.binding(() => root.bar.shown ? 1 : 0)

    readonly property real bw: 40
    readonly property real bh: 36

    component Indicator: Rectangle {
        property Item at: null
        x: at ? at.x + (at.width - width) / 2 : 0
        y: at ? at.y + at.height - height - Math.round(3 * root.u) : 0
        width: Math.round(16 * root.u)
        height: Math.round(3 * root.u)
        radius: height / 2
        color: Look.sel
        visible: at !== null && at.visible
        Behavior on x {
            NumberAnimation { duration: Look.dur(167); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.fluentIn }
        }
    }

    Item {
        id: card

        readonly property real restY: Math.round(root.bar.topInset + 16 * root.u)
        width: Math.ceil(content.implicitWidth + 12 * root.u)
        height: Math.round(48 * root.u)
        x: Math.round((root.width - width) / 2)
        y: card.restY - Math.round(12 * root.u) * (1 - root.enter)
        opacity: root.enter
        enabled: root.bar.shown

        MouseArea {
            anchors.fill: parent
            hoverEnabled: true
        }

        Surface {
            anchors.fill: parent
            u: root.u
            radius: 8 * root.u
            elevation: 3
        }

        Row {
            id: content
            x: Math.round(6 * root.u)
            anchors.verticalCenter: parent.verticalCenter
            spacing: Math.round(2 * root.u)

            Item {
                id: actionGroup
                anchors.verticalCenter: parent.verticalCenter
                width: actionRow.implicitWidth
                height: actionRow.implicitHeight
                property Item current: null

                Row {
                    id: actionRow
                    spacing: Math.round(2 * root.u)
                    Repeater {
                        model: root.bar.actions
                        delegate: GlyphButton {
                            id: action
                            required property var modelData
                            required property int index
                            u: root.u; cellWidth: root.bw; cellHeight: root.bh; iconSize: 18
                            icon: action.modelData.icon
                            checked: root.bar.activeAction === action.modelData.id
                            checkedFill: Look.hover; checkedInk: Look.fg
                            tip: action.modelData.label; keyHint: String(action.index + 1)
                            tipBelow: true
                            onClicked: root.bar.actionPicked(action.modelData.id)
                            onCheckedChanged: if (action.checked) actionGroup.current = action
                            Component.onCompleted: if (action.checked) actionGroup.current = action
                        }
                    }
                }
                Indicator { at: actionGroup.current }
            }

            Divider { u: root.u; length: 24; pad: 3; anchors.verticalCenter: parent.verticalCenter }

            Item {
                id: targetGroup
                anchors.verticalCenter: parent.verticalCenter
                width: targetRow.implicitWidth
                height: targetRow.implicitHeight
                readonly property Item current: root.bar.target === "window" ? windowTarget
                    : root.bar.target === "monitor" ? screenTarget : regionTarget

                Row {
                    id: targetRow
                    spacing: Math.round(2 * root.u)
                    GlyphButton {
                        id: regionTarget
                        u: root.u; cellWidth: root.bw; cellHeight: root.bh; iconSize: 18
                        icon: "region"
                        checked: root.bar.target === "region"
                        checkedFill: Look.hover; checkedInk: Look.fg
                        tip: I18n.tr("Region"); keyHint: I18n.tr("Space"); tipBelow: true
                        onClicked: root.bar.targetPicked("region")
                    }
                    GlyphButton {
                        id: windowTarget
                        visible: root.bar.windowTarget
                        u: root.u; cellWidth: root.bw; cellHeight: root.bh; iconSize: 18
                        icon: "window"
                        checked: root.bar.target === "window"
                        checkedFill: Look.hover; checkedInk: Look.fg
                        tip: I18n.tr("Window"); keyHint: I18n.tr("Space"); tipBelow: true
                        onClicked: root.bar.targetPicked("window")
                    }
                    GlyphButton {
                        id: screenTarget
                        u: root.u; cellWidth: root.bw; cellHeight: root.bh; iconSize: 18
                        icon: "screen"
                        checked: root.bar.target === "monitor"
                        checkedFill: Look.hover; checkedInk: Look.fg
                        tip: I18n.tr("Full screen"); keyHint: "\u23ce"; tipBelow: true
                        onClicked: root.bar.screenRequested()
                    }
                }
                Indicator { at: targetGroup.current }
            }

            Item {
                anchors.verticalCenter: parent.verticalCenter
                height: recordRow.implicitHeight
                width: root.bar.recording ? recordRow.implicitWidth : 0
                clip: width < recordRow.implicitWidth
                visible: width > 0
                opacity: root.bar.recording ? 1 : 0
                Behavior on width {
                    NumberAnimation { duration: Look.dur(250); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.fluentIn }
                }
                Behavior on opacity {
                    NumberAnimation { duration: Look.dur(167) }
                }

                Row {
                    id: recordRow
                    spacing: Math.round(2 * root.u)
                    Divider { u: root.u; length: 24; pad: 3; anchors.verticalCenter: parent.verticalCenter }
                    GlyphButton {
                        u: root.u; cellWidth: root.bw; cellHeight: root.bh; iconSize: 18
                        icon: "sound"; checked: root.bar.audioOn; tipBelow: true
                        tip: I18n.tr("Record system audio")
                        onClicked: root.bar.audioToggled()
                    }
                    GlyphButton {
                        u: root.u; cellWidth: root.bw; cellHeight: root.bh; iconSize: 18
                        icon: "mic"; checked: root.bar.micOn; tipBelow: true
                        tip: I18n.tr("Record microphone audio")
                        onClicked: root.bar.micToggled()
                    }
                    GlyphButton {
                        u: root.u; cellWidth: root.bw; cellHeight: root.bh; iconSize: 18
                        icon: "keyboard"; checked: root.bar.keypressesOn; tipBelow: true
                        tip: root.bar.keypressesOn ? I18n.tr("Hide key presses") : I18n.tr("Show key presses")
                        onClicked: root.bar.keypressesToggled()
                    }
                    GlyphButton {
                        u: root.u; cellWidth: root.bw; cellHeight: root.bh; iconSize: 18
                        icon: "webcam"; checked: root.bar.webcamOn; tipBelow: true
                        tip: root.bar.webcamOn ? I18n.tr("Hide webcam") : I18n.tr("Show webcam")
                        onClicked: root.bar.webcamToggled()
                    }
                }
            }

            Divider { u: root.u; length: 24; pad: 3; anchors.verticalCenter: parent.verticalCenter }

            // the delay drop-down: the timer, the delay once one is set, a chevron
            Item {
                id: delayButton
                readonly property bool open: root.bar.menu === "delay"
                anchors.verticalCenter: parent.verticalCenter
                width: Math.ceil(delayRow.implicitWidth + 16 * root.u)
                height: Math.round(root.bh * root.u)

                Rectangle {
                    anchors.fill: parent
                    radius: 4 * root.u
                    color: delayArea.pressed ? Look.press : (delayArea.containsMouse || delayButton.open) ? Look.hover : "transparent"
                    Behavior on color {
                        ColorAnimation { duration: Look.dur(Look.stateMs) }
                    }
                }
                Row {
                    id: delayRow
                    anchors.centerIn: parent
                    spacing: Math.round(4 * root.u)
                    Icon {
                        anchors.verticalCenter: parent.verticalCenter
                        name: "timer"
                        size: 18 * root.u
                        tint: Look.fg
                    }
                    Text {
                        visible: root.bar.delaySeconds > 0
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.bar.delaySeconds + "s"
                        color: Look.fg
                        font.family: Look.sans
                        font.pixelSize: Math.round(13 * root.u)
                        font.features: ({ "tnum": 1 })
                        renderType: Text.CurveRendering
                    }
                    Icon {
                        anchors.verticalCenter: parent.verticalCenter
                        name: "chevron"
                        size: 12 * root.u
                        tint: Look.fg2
                        rotation: delayButton.open ? 180 : 0
                        Behavior on rotation {
                            NumberAnimation { duration: Look.dur(167); easing.type: Easing.OutCubic }
                        }
                    }
                }
                MouseArea {
                    id: delayArea
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: root.bar.menuRequested(delayButton.open ? "" : "delay")
                }
                Tip {
                    z: 100
                    u: root.u
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: parent.height + 6 * root.u
                    text: I18n.tr("Delay")
                    shown: delayArea.containsMouse && !delayButton.open
                }
            }

            GlyphButton {
                id: more
                anchors.verticalCenter: parent.verticalCenter
                u: root.u; cellWidth: root.bw; cellHeight: root.bh; iconSize: 18
                icon: "moreH"
                checked: root.bar.menu === "more"
                checkedFill: Look.hover; checkedInk: Look.fg
                tip: I18n.tr("More"); tipBelow: true
                onClicked: root.bar.menuRequested(root.bar.menu === "more" ? "" : "more")
            }
            GlyphButton {
                anchors.verticalCenter: parent.verticalCenter
                u: root.u; cellWidth: root.bw; cellHeight: root.bh; iconSize: 16
                icon: "cancel"
                danger: true
                tip: I18n.tr("Close"); keyHint: "Esc"; tipBelow: true
                onClicked: root.bar.closeRequested()
            }
        }
    }

    Menu {
        u: root.u
        anchor: delayButton
        bounds: root
        preferAbove: false
        minWidth: 140
        open: root.bar.menu === "delay" && root.bar.shown
        onDismissed: root.bar.menuRequested("")

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
    }

    Menu {
        u: root.u
        anchor: more
        bounds: root
        preferAbove: false
        open: root.bar.menu === "more" && root.bar.shown
        onDismissed: root.bar.menuRequested("")

        MenuItem {
            u: root.u; icon: "eyedropper"; text: I18n.tr("Pick a colour"); keyHint: "C"
            onTriggered: root.bar.colorPickRequested()
        }
        MenuItem {
            u: root.u; icon: "folder"; text: I18n.tr("Open screenshots folder")
            onTriggered: root.bar.folderRequested()
        }
        MenuItem { u: root.u; kind: "separator" }
        MenuItem {
            u: root.u; icon: "gear"; text: I18n.tr("Settings")
            onTriggered: root.bar.settingsRequested()
        }
    }
}
