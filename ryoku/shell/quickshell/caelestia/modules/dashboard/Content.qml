pragma ComponentBehavior: Bound

import "../../services"
import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Widgets
import Caelestia
import Caelestia.Config
import caelestia.components
import caelestia.components.filedialog
import caelestia.utils

Item {
    id: root

    required property DrawerVisibilities visibilities
    required property ScreenState screenState
    required property FileDialog facePicker

    readonly property var dashboardTabs: {
        const allTabs = [
            {
                component: dashComponent,
                iconName: "dashboard",
                text: qsTr("Dashboard"),
                enabled: Config.dashboard.showDashboard
            },
            {
                component: mediaComponent,
                iconName: "queue_music",
                text: qsTr("Media"),
                enabled: Config.dashboard.showMedia
            },
            {
                component: performanceComponent,
                iconName: "speed",
                text: qsTr("Performance"),
                enabled: Config.dashboard.showPerformance && HostState.section("vitals")
            },
            {
                component: extensionsComponent,
                iconName: "extension",
                text: qsTr("Extensions"),
                enabled: HostState.extensionCount > 0 && HostState.section("bottomControls") && HostState.element("pluginCards")
            },
            {
                component: weatherComponent,
                iconName: "cloud",
                text: qsTr("Weather"),
                enabled: Config.dashboard.showWeather && !Paths.hosted
            },
            {
                component: notesComponent,
                iconName: "sticky_note_2",
                text: qsTr("Notes"),
                enabled: Config.dashboard.showNotes
            },
            {
                component: terminalComponent,
                iconName: "terminal",
                text: qsTr("Terminal"),
                enabled: Config.dashboard.showTerminal
            }
        ];
        return allTabs.filter(tab => tab.enabled);
    }

    readonly property real nonAnimWidth: view.implicitWidth + viewWrapper.anchors.margins * 2
    readonly property real nonAnimHeight: tabs.implicitHeight + tabs.anchors.topMargin + view.implicitHeight + viewWrapper.anchors.margins * 2

    onDashboardTabsChanged: {
        if (dashboardTabs.length > 0 && screenState.dashboardTab >= dashboardTabs.length) {
            screenState.dashboardTab = dashboardTabs.length - 1;
        }
    }

    implicitWidth: nonAnimWidth
    implicitHeight: nonAnimHeight

    Connections {
        function onOpenExtensions(): void {
            const i = root.dashboardTabs.findIndex(tab => tab.iconName === "extension");
            if (i >= 0)
                root.screenState.dashboardTab = i;
        }

        target: HostState
    }

    Tabs {
        id: tabs

        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.topMargin: CUtils.clamp(anchors.margins - Config.border.thickness, 0, anchors.margins)
        anchors.margins: Tokens.padding.large

        nonAnimWidth: root.nonAnimWidth - anchors.margins * 2
        screenState: root.screenState
        tabs: root.dashboardTabs
    }

    ClippingRectangle {
        id: viewWrapper

        anchors.top: tabs.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: Tokens.padding.large

        radius: Tokens.rounding.large
        color: "transparent"

        Flickable {
            id: view

            readonly property int currentIndex: root.screenState.dashboardTab
            readonly property Item currentItem: {
                repeater.count;
                return repeater.itemAt(currentIndex);
            }

            anchors.fill: parent

            flickableDirection: Flickable.HorizontalFlick

            implicitWidth: currentItem?.implicitWidth ?? 0
            implicitHeight: currentItem?.implicitHeight ?? 0

            contentX: currentItem?.x ?? 0
            contentWidth: row.implicitWidth
            contentHeight: row.implicitHeight

            onContentXChanged: {
                if (!moving || !currentItem)
                    return;

                const x = contentX - currentItem.x;
                if (x > currentItem.implicitWidth / 2)
                    root.screenState.dashboardTab = Math.min(root.screenState.dashboardTab + 1, tabs.count - 1);
                else if (x < -currentItem.implicitWidth / 2)
                    root.screenState.dashboardTab = Math.max(root.screenState.dashboardTab - 1, 0);
            }

            onDragEnded: {
                if (!currentItem)
                    return;

                const x = contentX - currentItem.x;
                if (x > currentItem.implicitWidth / 10)
                    root.screenState.dashboardTab = Math.min(root.screenState.dashboardTab + 1, tabs.count - 1);
                else if (x < -currentItem.implicitWidth / 10)
                    root.screenState.dashboardTab = Math.max(root.screenState.dashboardTab - 1, 0);
                else
                    contentX = Qt.binding(() => currentItem?.x ?? 0);
            }

            RowLayout {
                id: row

                Repeater {
                    id: repeater

                    model: ScriptModel {
                        values: root.dashboardTabs
                    }

                    delegate: Loader {
                        id: paneLoader

                        required property int index
                        required property var modelData

                        Layout.alignment: Qt.AlignTop

                        sourceComponent: modelData.component

                        Component.onCompleted: active = Qt.binding(() => {
                            if (!root.visibilities.dashboard)
                                return false;
                            if (active) return true;
                            if (index === view.currentIndex)
                                return true;
                            const vx = Math.floor(view.visibleArea.xPosition * view.contentWidth);
                            const vex = Math.floor(vx + view.visibleArea.widthRatio * view.contentWidth);
                            return (vx >= x && vx <= x + implicitWidth) || (vex >= x && vex <= x + implicitWidth);
                        })
                    }
                }
            }

            Component {
                id: dashComponent

                Dash {
                    visibilities: root.visibilities
                    screenState: root.screenState
                    facePicker: root.facePicker
                }
            }

            Component {
                id: mediaComponent

                Media {
                    visibilities: root.visibilities
                }
            }

            Component {
                id: performanceComponent

                Performance {}
            }

            Component {
                id: extensionsComponent

                Item {
                    implicitWidth: Tokens.sizes.dashboard.mediaWidth * 3
                    implicitHeight: 380

                    Loader {
                        id: extensions

                        anchors.fill: parent
                        sourceComponent: HostState.extensions
                    }

                    Connections {
                        function onRequestClose(): void {
                            root.visibilities.dashboard = false;
                        }

                        target: extensions.item
                    }
                }
            }

            Component {
                id: weatherComponent

                WeatherTab {}
            }

            Component {
                id: notesComponent

                NotesTab {
                    dashboardOpen: root.visibilities.dashboard
                }
            }
            
            Component {
                id: terminalComponent

                TerminalTab {}
            }

            Behavior on contentX {
                Anim {}
            }
        }
    }

    Behavior on implicitWidth {
        Anim {}
    }

    Behavior on implicitHeight {
        Anim {}
    }
}
