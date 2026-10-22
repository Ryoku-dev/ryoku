pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import M3Shapes
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.components.effects
import caelestia.services

StyledRect {
    id: root

    required property PopoutState popouts
    property bool _isSidebarOpen: false
    property real scaleOffset: 1.0
    property real fontScale: 1.0

    property date viewDate: new Date()
    readonly property int currMonth: viewDate.getMonth()
    readonly property int currYear: viewDate.getFullYear()
    property date _prevDate: new Date()
    readonly property int animDirection: viewDate > _prevDate ? -1 : 1
    property real animTranslate
    property real animOpacity: 1

    implicitWidth: 497 * root.scaleOffset
    implicitHeight: inner.implicitHeight + Tokens.padding.large * 2 * root.scaleOffset
    radius: Tokens.rounding.extraLarge * root.scaleOffset
    color: Colours.tPalette.m3surfaceContainer

    Anim {
        id: trOutAnim

        running: false
        target: root
        property: "animTranslate"
        to: Tokens.padding.extraLarge * root.scaleOffset * root.animDirection
        type: Anim.FastSpatial
    }

    Behavior on viewDate {
        SequentialAnimation {
            ParallelAnimation {
                ScriptAction {
                    script: Qt.callLater(() => trOutAnim.start())
                }
                Anim {
                    target: root
                    property: "animOpacity"
                    to: 0
                    type: Anim.FastEffects
                }
            }
            ScriptAction {
                script: {
                    trOutAnim.complete();
                    root.animTranslate = Tokens.padding.extraLarge * root.scaleOffset * -root.animDirection;
                }
            }
            PropertyAction {}
            ParallelAnimation {
                Anim {
                    target: root
                    property: "animTranslate"
                    to: 0
                    type: Anim.DefaultSpatial
                }
                Anim {
                    target: root
                    property: "animOpacity"
                    to: 1
                    type: Anim.DefaultEffects
                }
            }
        }
    }

    CustomMouseArea {
        function onWheel(event: WheelEvent): void {
            root._prevDate = root.viewDate;
            if (event.angleDelta.y > 0)
                root.viewDate = new Date(root.currYear, root.currMonth - 1, 1);
            else if (event.angleDelta.y < 0)
                root.viewDate = new Date(root.currYear, root.currMonth + 1, 1);
        }

        anchors.fill: parent
        acceptedButtons: Qt.MiddleButton | Qt.RightButton
        onClicked: mouse => {
            if (mouse.button === Qt.MiddleButton) {
                root._prevDate = root.viewDate;
                root.viewDate = new Date();
            } else if (mouse.button === Qt.RightButton) {
                root.popouts.currentName = "clockcontext";
            }
        }
    }

    ColumnLayout {
        id: inner

        x: Tokens.padding.large * root.scaleOffset
        y: Tokens.padding.large * root.scaleOffset
        width: root.width - Tokens.padding.large * 2 * root.scaleOffset
        spacing: Tokens.spacing.extraSmall * root.scaleOffset

        RowLayout {
            id: monthNavigationRow

            Layout.fillWidth: true
            spacing: Tokens.spacing.extraSmall * root.scaleOffset

            IconButton {
                isRound: true
                icon: "chevron_left"
                type: IconButton.Text
                font: Tokens.font.icon.builders.small.weight(Font.Bold).size(Tokens.font.icon.builders.small.build().pointSize * root.fontScale).build()
                padding: Tokens.padding.small * root.scaleOffset
                onClicked: {
                    root._prevDate = root.viewDate;
                    root.viewDate = new Date(root.currYear, root.currMonth - 1, 1);
                }
            }

            Item {
                Layout.fillWidth: true
                Layout.fillHeight: true

                implicitWidth: monthYearDisplay.implicitWidth + Tokens.padding.large * 2 * root.scaleOffset
                implicitHeight: monthYearDisplay.implicitHeight + Tokens.padding.extraSmall * 2 * root.scaleOffset

                StateLayer {
                    color: Colours.palette.m3primary
                    radius: pressed ? Tokens.rounding.small * root.scaleOffset : height / 2
                    disabled: {
                        const now = new Date();
                        return root.currMonth === now.getMonth() && root.currYear === now.getFullYear();
                    }
                    onClicked: {
                        root._prevDate = root.viewDate;
                        root.viewDate = new Date();
                    }

                    Behavior on radius {
                        Anim {
                            type: Anim.DefaultEffects
                        }
                    }
                }

                StyledText {
                    id: monthYearDisplay

                    opacity: root.animOpacity
                    transform: Translate {
                        x: root.animTranslate
                    }

                    anchors.centerIn: parent
                    text: grid.title
                    color: Colours.palette.m3primary
                    font: Tokens.font.title.builders.small.capitalisation(Font.Capitalize).size(Tokens.font.title.builders.small.build().pointSize * root.fontScale).build()
                }
            }

            IconButton {
                isRound: true
                icon: "chevron_right"
                type: IconButton.Text
                font: Tokens.font.icon.builders.small.weight(Font.Bold).size(Tokens.font.icon.builders.small.build().pointSize * root.fontScale).build()
                padding: Tokens.padding.small * root.scaleOffset
                onClicked: {
                    root._prevDate = root.viewDate;
                    root.viewDate = new Date(root.currYear, root.currMonth + 1, 1);
                }
            }
        }

        DayOfWeekRow {
            id: daysRow

            Layout.fillWidth: true
            locale: grid.locale

            delegate: StyledText {
                required property var model

                horizontalAlignment: Text.AlignHCenter
                text: model.shortName
                font: Tokens.font.body.builders.small.weight(Font.Medium).size(Tokens.font.body.builders.small.build().pointSize * root.fontScale).build()
                color: (model.day === 0 || model.day === 6) ? Colours.palette.m3tertiary : Colours.palette.m3onSurface
                renderType: Text.QtRendering
            }
        }

        Item {
            Layout.fillWidth: true
            implicitHeight: grid.implicitHeight

            opacity: root.animOpacity
            transform: Translate {
                x: root.animTranslate
            }

            MonthGrid {
                id: grid

                month: root.currMonth
                year: root.currYear

                anchors.fill: parent

                spacing: 3 * root.scaleOffset
                locale: Qt.locale()

                delegate: Item {
                    id: dayItem

                    required property var model

                    implicitWidth: implicitHeight
                    implicitHeight: text.implicitHeight + Tokens.padding.small * root.scaleOffset

                    StyledText {
                        id: text

                        anchors.centerIn: parent

                        horizontalAlignment: Text.AlignHCenter
                        text: grid.locale.toString(dayItem.model.day)
                        color: {
                            const dayOfWeek = dayItem.model.date.getDay();
                            if (dayOfWeek === 0 || dayOfWeek === 6)
                                return Colours.palette.m3tertiary;

                            return Colours.palette.m3onSurfaceVariant;
                        }
                        opacity: dayItem.model.today || dayItem.model.month === grid.month ? 1 : 0.4
                        font: Tokens.font.body.builders.small.size(Tokens.font.body.small.pointSize * root.fontScale).build()
                        renderType: Text.QtRendering
                    }
                }
            }

            MaterialShape {
                id: todayIndicator

                readonly property Item todayItem: grid.contentItem.children.find(c => c.model.today) ?? null
                property Item today

                onTodayItemChanged: {
                    if (todayItem)
                        today = todayItem;
                }

                x: today ? today.x + (today.width - implicitWidth) / 2 : 0
                y: today ? today.y + (today.height - implicitHeight) / 2 : 0

                implicitSize: today ? Math.max(today.implicitWidth, today.implicitHeight) + Tokens.padding.extraSmall * 2 * root.scaleOffset : 0
                shape: MaterialShape.Sunny

                clip: true
                color: Colours.palette.m3primary

                opacity: todayItem ? 1 : 0

                Colouriser {
                    x: -todayIndicator.x
                    y: -todayIndicator.y

                    implicitWidth: grid.width
                    implicitHeight: grid.height

                    source: grid
                    sourceColor: Colours.palette.m3onSurface
                    colorizationColor: Colours.palette.m3onPrimary
                }

                Behavior on x {
                    Anim {}
                }

                Behavior on y {
                    Anim {}
                }
            }
        }
    }
}
