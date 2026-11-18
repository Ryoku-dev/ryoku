pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import ".."
import "../look"

// Material's capture bar, after Android 17's and ChromeOS's screen capture: an
// M3 Expressive floating toolbar 24 above the bottom edge with a FAB beside it.
// The actions are a connected button group whose chosen segment grows into a
// labelled pill; the targets are icon toggles whose chosen one is a primary
// squircle. Delay and the rest live in an M3 menu behind More. The FAB takes
// the whole screen it is on. It rises on the fast spatial spring and the FAB
// pops in just after it.
Item {
    id: root

    required property Item bar
    readonly property real u: root.bar.u
    readonly property real reveal: Math.max(0, Math.min(1, root.rise))
    readonly property rect plane: Qt.rect(group.x, group.restY, group.width, group.height)

    property real rise: 0
    Behavior on rise {
        id: riseMotion
        NumberAnimation {
            duration: Look.dur(riseMotion.targetValue > 0.5 ? 380 : 150)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: riseMotion.targetValue > 0.5 ? Look.springFast : Look.accelerate
        }
    }
    property real pop: 0
    Behavior on pop {
        id: popMotion
        SequentialAnimation {
            PauseAnimation { duration: Look.dur(popMotion.targetValue > 0.5 ? 60 : 0) }
            NumberAnimation {
                duration: Look.dur(popMotion.targetValue > 0.5 ? 380 : 120)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: popMotion.targetValue > 0.5 ? Look.springFast : Look.accelerate
            }
        }
    }
    Component.onCompleted: {
        root.rise = Qt.binding(() => root.bar.shown ? 1 : 0);
        root.pop = Qt.binding(() => root.bar.shown ? 1 : 0);
    }

    readonly property real seg: 40

    Item {
        id: group

        readonly property real restY: root.height - height - Math.round(24 * root.u)
        width: pill.width + Math.round(8 * root.u) + fab.width
        height: Math.round(56 * root.u)
        x: Math.round((root.width - width) / 2)
        y: group.restY + Math.round(24 * root.u) * (1 - root.rise)
        enabled: root.bar.shown

        Item {
            id: pill
            width: Math.ceil(content.implicitWidth + 16 * root.u)
            height: parent.height
            opacity: Math.min(1, root.rise * 3)
            scale: 0.9 + 0.1 * root.rise

            MouseArea {
                anchors.fill: parent
                hoverEnabled: true
            }

            Surface {
                anchors.fill: parent
                u: root.u
                radius: height / 2
                elevation: 3
            }

            Row {
                id: content
                x: Math.round(8 * root.u)
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(4 * root.u)

                // the connected button group: inner corners small, the chosen segment a full pill
                Row {
                    id: actions
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Math.round(2 * root.u)

                    Repeater {
                        model: root.bar.actions
                        delegate: Item {
                            id: segment
                            required property var modelData
                            required property int index
                            readonly property bool current: root.bar.activeAction === segment.modelData.id
                            readonly property bool first: segment.index === 0
                            readonly property bool last: segment.index === root.bar.actions.length - 1
                            readonly property real full: height / 2
                            readonly property real inner: 8 * root.u

                            height: Math.round(root.seg * root.u)
                            width: Math.round(root.seg * root.u) + (segment.current ? Math.ceil(name.implicitWidth + 8 * root.u) : 0)
                            Behavior on width {
                                NumberAnimation {
                                    duration: Look.dur(420)
                                    easing.type: Easing.BezierSpline
                                    easing.bezierCurve: Look.springDefault
                                }
                            }
                            scale: area.pressed ? 0.92 : 1
                            Behavior on scale {
                                NumberAnimation {
                                    duration: Look.dur(area.pressed ? 90 : 260)
                                    easing.type: Easing.BezierSpline
                                    easing.bezierCurve: area.pressed ? Look.decelerate : Look.springFast
                                }
                            }

                            Rectangle {
                                anchors.fill: parent
                                color: segment.current ? (area.containsMouse ? Lithic.mix(Look.container, Look.containerInk, 0.08) : Look.container)
                                    : area.containsMouse ? Lithic.mix(Look.well, Look.fg, 0.08) : Look.well
                                topLeftRadius: segment.current || segment.first ? segment.full : segment.inner
                                bottomLeftRadius: segment.current || segment.first ? segment.full : segment.inner
                                topRightRadius: segment.current || segment.last ? segment.full : segment.inner
                                bottomRightRadius: segment.current || segment.last ? segment.full : segment.inner
                                antialiasing: true
                                Behavior on color {
                                    ColorAnimation { duration: Look.dur(150) }
                                }
                                Behavior on topLeftRadius { NumberAnimation { duration: Look.dur(300); easing.type: Easing.OutCubic } }
                                Behavior on bottomLeftRadius { NumberAnimation { duration: Look.dur(300); easing.type: Easing.OutCubic } }
                                Behavior on topRightRadius { NumberAnimation { duration: Look.dur(300); easing.type: Easing.OutCubic } }
                                Behavior on bottomRightRadius { NumberAnimation { duration: Look.dur(300); easing.type: Easing.OutCubic } }
                            }

                            Item {
                                anchors.fill: parent
                                clip: true

                                Icon {
                                    id: glyph
                                    x: Math.round(10 * root.u)
                                    anchors.verticalCenter: parent.verticalCenter
                                    name: segment.modelData.icon
                                    size: 20 * root.u
                                    tint: segment.current ? Look.containerInk : Look.fg2
                                }
                                Text {
                                    id: name
                                    x: glyph.x + glyph.width + Math.round(8 * root.u)
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: segment.modelData.label
                                    color: Look.containerInk
                                    opacity: segment.current ? 1 : 0
                                    font.family: Look.sans
                                    font.pixelSize: Math.round(14 * root.u)
                                    font.weight: Font.DemiBold
                                    renderType: Text.CurveRendering
                                    Behavior on opacity {
                                        NumberAnimation { duration: Look.dur(segment.current ? 220 : 90) }
                                    }
                                }
                            }

                            MouseArea {
                                id: area
                                anchors.fill: parent
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: root.bar.actionPicked(segment.modelData.id)
                            }
                            Tip {
                                z: 100
                                u: root.u
                                anchors.horizontalCenter: parent.horizontalCenter
                                y: -height - 14 * root.u
                                text: segment.modelData.label
                                keyHint: String(segment.index + 1)
                                shown: area.containsMouse && !segment.current
                            }
                        }
                    }
                }

                Divider { u: root.u; length: 24; pad: 4; anchors.verticalCenter: parent.verticalCenter }

                Row {
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Math.round(2 * root.u)

                    GlyphButton {
                        u: root.u; cellWidth: root.seg; cellHeight: root.seg; iconSize: 20
                        icon: "region"
                        checked: root.bar.target === "region"
                        tip: I18n.tr("Region"); keyHint: I18n.tr("Space")
                        onClicked: root.bar.targetPicked("region")
                    }
                    GlyphButton {
                        visible: root.bar.windowTarget
                        u: root.u; cellWidth: root.seg; cellHeight: root.seg; iconSize: 20
                        icon: "window"
                        checked: root.bar.target === "window"
                        tip: I18n.tr("Window"); keyHint: I18n.tr("Space")
                        onClicked: root.bar.targetPicked("window")
                    }
                    GlyphButton {
                        u: root.u; cellWidth: root.seg; cellHeight: root.seg; iconSize: 20
                        icon: "screen"
                        checked: root.bar.target === "monitor"
                        tip: I18n.tr("Screen"); keyHint: I18n.tr("Space")
                        onClicked: root.bar.targetPicked("monitor")
                    }
                }

                // the record options cross-fade in while Record is chosen
                Item {
                    anchors.verticalCenter: parent.verticalCenter
                    height: recordRow.implicitHeight
                    width: root.bar.recording ? recordRow.implicitWidth : 0
                    clip: width < recordRow.implicitWidth
                    visible: width > 0
                    Behavior on width {
                        NumberAnimation { duration: Look.dur(380); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springDefault }
                    }

                    Row {
                        id: recordRow
                        spacing: Math.round(2 * root.u)
                        opacity: root.bar.recording ? 1 : 0
                        Behavior on opacity {
                            NumberAnimation { duration: Look.dur(root.bar.recording ? 220 : 90) }
                        }

                        Divider { u: root.u; length: 24; pad: 4; anchors.verticalCenter: parent.verticalCenter }
                        GlyphButton {
                            u: root.u; cellWidth: root.seg; cellHeight: root.seg; iconSize: 20
                            icon: "sound"; checked: root.bar.audioOn
                            checkedFill: Look.container; checkedInk: Look.containerInk
                            tip: I18n.tr("Record system audio")
                            onClicked: root.bar.audioToggled()
                        }
                        GlyphButton {
                            u: root.u; cellWidth: root.seg; cellHeight: root.seg; iconSize: 20
                            icon: "mic"; checked: root.bar.micOn
                            checkedFill: Look.container; checkedInk: Look.containerInk
                            tip: I18n.tr("Record microphone audio")
                            onClicked: root.bar.micToggled()
                        }
                        GlyphButton {
                            u: root.u; cellWidth: root.seg; cellHeight: root.seg; iconSize: 20
                            icon: "keyboard"; checked: root.bar.keypressesOn
                            checkedFill: Look.container; checkedInk: Look.containerInk
                            tip: root.bar.keypressesOn ? I18n.tr("Hide key presses") : I18n.tr("Show key presses")
                            onClicked: root.bar.keypressesToggled()
                        }
                        GlyphButton {
                            u: root.u; cellWidth: root.seg; cellHeight: root.seg; iconSize: 20
                            icon: "webcam"; checked: root.bar.webcamOn
                            checkedFill: Look.container; checkedInk: Look.containerInk
                            tip: root.bar.webcamOn ? I18n.tr("Hide webcam") : I18n.tr("Show webcam")
                            onClicked: root.bar.webcamToggled()
                        }
                    }
                }

                Divider { u: root.u; length: 24; pad: 4; anchors.verticalCenter: parent.verticalCenter }

                Badge {
                    id: delayChip
                    visible: root.bar.delaySeconds > 0
                    anchors.verticalCenter: parent.verticalCenter
                    u: root.u
                    text: root.bar.delaySeconds + "s"
                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.bar.menuRequested(root.bar.menu === "more" ? "" : "more")
                    }
                }

                GlyphButton {
                    id: more
                    anchors.verticalCenter: parent.verticalCenter
                    u: root.u; cellWidth: root.seg; cellHeight: root.seg; iconSize: 20
                    icon: "more"
                    checked: root.bar.menu === "more"
                    checkedFill: Look.hover; checkedInk: Look.fg
                    tip: I18n.tr("More")
                    onClicked: root.bar.menuRequested(root.bar.menu === "more" ? "" : "more")
                }
                GlyphButton {
                    anchors.verticalCenter: parent.verticalCenter
                    u: root.u; cellWidth: root.seg; cellHeight: root.seg; iconSize: 18
                    icon: "cancel"
                    tip: I18n.tr("Close"); keyHint: "Esc"
                    onClicked: root.bar.closeRequested()
                }
            }
        }

        // the FAB: the whole screen this bar is on
        Item {
            id: fab
            x: pill.width + Math.round(8 * root.u)
            width: Math.round(56 * root.u)
            height: width
            scale: Math.max(0, root.pop) * (fabArea.pressed ? 0.92 : 1)
            Behavior on scale {
                enabled: root.pop >= 1
                NumberAnimation {
                    duration: Look.dur(fabArea.pressed ? 90 : 260)
                    easing.type: Easing.BezierSpline
                    easing.bezierCurve: fabArea.pressed ? Look.decelerate : Look.springFast
                }
            }

            Surface {
                anchors.fill: parent
                u: root.u
                radius: 16 * root.u
                fill: fabArea.containsMouse ? Lithic.hoverFill(Look.sel) : Look.sel
                stroke: "transparent"
                elevation: 3
            }
            Icon {
                anchors.centerIn: parent
                name: "camera"
                size: 24 * root.u
                tint: Look.onSel
                opacity: root.bar.recording ? 0 : 1
                scale: root.bar.recording ? 0.6 : 1
                Behavior on opacity { NumberAnimation { duration: Look.dur(120) } }
                Behavior on scale { NumberAnimation { duration: Look.dur(260); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springFast } }
            }
            // the record dot grows out of the camera's lens
            Rectangle {
                anchors.centerIn: parent
                width: Math.round(18 * root.u) * (root.bar.recording ? 1 : 0)
                height: width
                radius: width / 2
                color: Look.onSel
                Behavior on width { NumberAnimation { duration: Look.dur(320); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springFast } }
            }
            MouseArea {
                id: fabArea
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
                shown: fabArea.containsMouse
            }
        }
    }

    Menu {
        u: root.u
        anchor: more
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
        MenuItem { u: root.u; kind: "separator" }
        MenuItem {
            u: root.u; icon: "eyedropper"; text: I18n.tr("Pick a colour"); keyHint: "C"
            onTriggered: root.bar.colorPickRequested()
        }
        MenuItem {
            u: root.u; icon: "folder"; text: I18n.tr("Open screenshots folder")
            onTriggered: root.bar.folderRequested()
        }
        MenuItem {
            u: root.u; icon: "gear"; text: I18n.tr("Settings")
            onTriggered: root.bar.settingsRequested()
        }
    }
}
