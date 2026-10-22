pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import QtQuick.Templates
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services

Item {
    id: root

    required property real nonAnimWidth
    required property ScreenState screenState
    required property var tabs

    readonly property alias count: bar.count

    implicitHeight: bar.implicitHeight + bar.anchors.topMargin + indicator.implicitHeight + indicator.anchors.topMargin + separator.implicitHeight

    // A plain row, not a TabBar: a TabBar row over its contentModel left a tab added
    // after it was built (Performance coming back with Vitals) empty. The selected tab
    // lives in screenState.dashboardTab.
    RowLayout {
        id: bar

        readonly property int currentIndex: root.screenState.dashboardTab
        readonly property int count: tabRepeater.count
        readonly property Item currentItem: count > 0 ? tabRepeater.itemAt(currentIndex) : null

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.topMargin: Tokens.sizes.dashboard.tabIndicatorSpacing

        spacing: 0

        Repeater {
            id: tabRepeater

            model: ScriptModel {
                values: root.tabs
            }

            delegate: Tab {
                required property var modelData

                iconName: modelData.iconName
                text: modelData.text
            }
        }
    }

    Item {
        id: indicator

        anchors.top: bar.bottom
        anchors.topMargin: 5

        implicitWidth: {
            const tab = bar.currentItem;
            if (tab)
                return tab.implicitWidth;
            const width = (root.nonAnimWidth - bar.spacing * (bar.count - 1)) / bar.count;
            return width;
        }
        implicitHeight: 3

        x: {
            const tab = bar.currentItem;
            const width = (root.nonAnimWidth - bar.spacing * (bar.count - 1)) / bar.count;
            const tabWidth = tab?.implicitWidth ?? width;
            return width * bar.currentIndex + (width - tabWidth) / 2;
        }

        clip: true

        StyledRect {
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            implicitHeight: parent.implicitHeight * 2

            color: Colours.palette.m3primary
            radius: Tokens.rounding.full
        }

        Behavior on x {
            Anim {}
        }

        Behavior on implicitWidth {
            Anim {}
        }
    }

    StyledRect {
        id: separator

        anchors.top: indicator.bottom
        anchors.left: parent.left
        anchors.right: parent.right

        implicitHeight: 1
        color: Colours.palette.m3outlineVariant
    }

    component Tab: TabButton {
        id: tab

        required property string iconName
        required property int index
        readonly property bool current: index === root.screenState.dashboardTab

        Layout.fillWidth: true
        Layout.preferredWidth: 1
        implicitWidth: implicitContentWidth
        implicitHeight: implicitContentHeight
        background: null

        contentItem: CustomMouseArea {
            id: mouse

            function onWheel(event: WheelEvent): void {
                if (event.angleDelta.y < 0)
                    root.screenState.dashboardTab = Math.min(root.screenState.dashboardTab + 1, bar.count - 1);
                else if (event.angleDelta.y > 0)
                    root.screenState.dashboardTab = Math.max(root.screenState.dashboardTab - 1, 0);
            }

            implicitWidth: Math.max(icon.width, label.width)
            implicitHeight: icon.height + label.height

            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor

            onPressed: root.screenState.dashboardTab = tab.index

            StateLayer {
                id: stateLayer

                anchors.fill: undefined
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                implicitHeight: parent.height + Tokens.sizes.dashboard.tabIndicatorSpacing * 2

                radius: Tokens.rounding.medium
                color: tab.current ? Colours.palette.m3primary : Colours.palette.m3onSurface
                onClicked: root.screenState.dashboardTab = tab.index
            }

            MaterialIcon {
                id: icon

                anchors.horizontalCenter: parent.horizontalCenter
                anchors.bottom: label.top

                text: tab.iconName
                color: tab.current ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                fill: tab.current ? 1 : 0
                fontStyle: Tokens.font.icon.medium

                Behavior on fill {
                    Anim {
                        type: Anim.DefaultEffects
                    }
                }
            }

            StyledText {
                id: label

                anchors.horizontalCenter: parent.horizontalCenter
                anchors.bottom: parent.bottom

                text: tab.text
                color: tab.current ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
            }
        }
    }
}
