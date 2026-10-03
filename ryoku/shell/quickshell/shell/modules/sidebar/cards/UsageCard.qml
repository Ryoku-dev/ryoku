pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services
import ".."

Item {
    id: root

    required property real s
    required property bool open
    required property real reveal
    required property bool tabActive
    property int index: 0
    signal requestClose()

    implicitHeight: card.implicitHeight

    SystemClock {
        id: clock
        precision: SystemClock.Minutes
        enabled: root.open && root.tabActive
    }

    SidebarCardShell {
        id: card
        width: root.width
        index: root.index
        open: root.open
        reveal: root.reveal
        tabActive: root.tabActive
        title: I18n.tr("Screen time")
        glyph: "monitor_heart"
        eyebrow: Qt.formatDate(clock.date, "ddd, MMM d")

        Column {
            width: parent.width
            spacing: Tokens.s5 * root.s

            Rectangle {
                width: parent.width
                implicitHeight: hero.implicitHeight + Tokens.s5 * root.s * 2
                radius: Tokens.radius * root.s
                color: Tokens.tint5
                border.width: Tokens.border
                border.color: Tokens.lineSoft
                clip: true

                Rectangle {
                    anchors.left: parent.left
                    anchors.top: parent.top
                    width: parent.width * 0.72
                    height: parent.height
                    opacity: 0.15
                    gradient: Gradient {
                        orientation: Gradient.Horizontal
                        GradientStop { position: 0; color: Tokens.sun }
                        GradientStop { position: 1; color: "transparent" }
                    }
                }

                Column {
                    id: hero
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.margins: Tokens.s5 * root.s
                    spacing: Tokens.s1 * root.s

                    Text {
                        text: I18n.tr("ACTIVE TODAY")
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fMicro * root.s
                        font.weight: Font.DemiBold
                        font.letterSpacing: Tokens.trackMark
                    }
                    Text {
                        text: ScreenTime.fmtDuration(ScreenTime.activeToday)
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: (Tokens.fHero + 10) * root.s
                        font.weight: Font.Medium
                    }
                    Text {
                        text: I18n.tr("Private, local activity for this desktop")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                    }
                }
            }

            Section {
                width: parent.width
                title: I18n.tr("THIS WEEK")

                Item {
                    width: parent.width
                    height: 126 * root.s

                    Row {
                        id: weekRow
                        anchors.fill: parent
                        spacing: Tokens.s2 * root.s
                        readonly property real cellW: (width - spacing * 6) / 7
                        readonly property real maxBarH: height - 28 * root.s

                        Repeater {
                            model: ScreenTime.weekly
                            delegate: Item {
                                id: day
                                required property var modelData
                                width: weekRow.cellW
                                height: weekRow.height

                                Rectangle {
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.bottom: dayLabel.top
                                    anchors.bottomMargin: Tokens.s2 * root.s
                                    height: weekRow.maxBarH
                                    radius: Tokens.radius * root.s
                                    color: Tokens.tint5
                                    border.width: Tokens.border
                                    border.color: Tokens.lineSoft

                                    Rectangle {
                                        anchors.left: parent.left
                                        anchors.right: parent.right
                                        anchors.bottom: parent.bottom
                                        height: Math.max(4 * root.s, parent.height * day.modelData.total / ScreenTime.weeklyMax)
                                        radius: parent.radius
                                        color: day.modelData.isToday ? Tokens.sun : Tokens.inkMuted
                                        opacity: day.modelData.isToday ? 1 : 0.45
                                        Behavior on height {
                                            NumberAnimation {
                                                duration: Tokens.move
                                                easing.type: Tokens.ease
                                            }
                                        }
                                    }
                                }

                                Text {
                                    id: dayLabel
                                    anchors.bottom: parent.bottom
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    text: I18n.tr(day.modelData.label)
                                    color: day.modelData.isToday ? Tokens.sun : Tokens.inkMuted
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny * root.s
                                    font.weight: day.modelData.isToday ? Font.DemiBold : Font.Normal
                                }
                            }
                        }
                    }
                }
            }

            Section {
                width: parent.width
                title: I18n.tr("MOST USED")

                Column {
                    width: parent.width
                    spacing: Tokens.s2 * root.s

                    Text {
                        width: parent.width
                        visible: ScreenTime.topApps.length === 0
                        text: I18n.tr("Nothing tracked yet. Usage builds as you work.")
                        wrapMode: Text.WordWrap
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                    }

                    Repeater {
                        model: ScreenTime.topApps
                        delegate: Rectangle {
                            id: appRow
                            required property var modelData
                            width: parent.width
                            height: 52 * root.s
                            radius: Tokens.radius * root.s
                            color: appHover.hovered ? Tokens.tint10 : Tokens.tint5
                            border.width: Tokens.border
                            border.color: appHover.hovered ? Tokens.lineStrong : Tokens.lineSoft
                            y: appHover.hovered && !Tokens.reduceMotion ? -2 * root.s : 0
                            Behavior on y { NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap } }
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }

                            Image {
                                id: appIcon
                                anchors.left: parent.left
                                anchors.leftMargin: Tokens.s3 * root.s
                                anchors.verticalCenter: parent.verticalCenter
                                width: 26 * root.s
                                height: width
                                visible: appRow.modelData.icon !== ""
                                source: appRow.modelData.icon
                                sourceSize.width: width * 2
                                sourceSize.height: height * 2
                                smooth: true
                            }

                            Rectangle {
                                anchors.fill: appIcon
                                visible: appRow.modelData.icon === ""
                                radius: Tokens.radius * root.s
                                color: Tokens.tint10
                                Text {
                                    anchors.centerIn: parent
                                    text: String(appRow.modelData.name).charAt(0).toUpperCase()
                                    color: Tokens.inkDim
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fBody * root.s
                                }
                            }

                            Column {
                                anchors.left: appIcon.right
                                anchors.right: appTime.left
                                anchors.leftMargin: Tokens.s3 * root.s
                                anchors.rightMargin: Tokens.s3 * root.s
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s1 * root.s

                                Text {
                                    width: parent.width
                                    text: appRow.modelData.name
                                    elide: Text.ElideRight
                                    color: Tokens.ink
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fSmall * root.s
                                    font.weight: Font.Medium
                                }
                                Rectangle {
                                    width: parent.width
                                    height: 3 * root.s
                                    radius: height / 2
                                    color: Tokens.tint10
                                    Rectangle {
                                        width: parent.width * Math.max(0.03, appRow.modelData.seconds / ScreenTime.topSeconds)
                                        height: parent.height
                                        radius: parent.radius
                                        color: Tokens.sun
                                        Behavior on width { NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease } }
                                    }
                                }
                            }

                            Text {
                                id: appTime
                                anchors.right: parent.right
                                anchors.rightMargin: Tokens.s3 * root.s
                                anchors.verticalCenter: parent.verticalCenter
                                text: ScreenTime.fmtDuration(appRow.modelData.seconds)
                                color: Tokens.inkDim
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fTiny * root.s
                            }

                            HoverHandler { id: appHover }
                        }
                    }
                }
            }
        }
    }
}
