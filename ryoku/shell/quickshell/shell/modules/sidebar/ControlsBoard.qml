pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Ryoku.Blobs
import Ryoku.Ui.Singletons
import shell.services
import "cards" as Cards

Item {
    id: root

    required property real s
    required property var screen
    required property bool active
    property string page: "controls"
    signal requestClose()

    readonly property string currentPage: ["wifi", "bluetooth"].indexOf(page) >= 0 ? page : "controls"
    readonly property bool overviewActive: active && currentPage === "controls"
    readonly property bool motionAllowed: !Tokens.reduceMotion && !Motion.reduce

    implicitHeight: currentPage === "controls" ? overview.implicitHeight : Tokens.cellH * 4 * s

    function showPage(value): void {
        SidebarState.selectTab(root.screen, value);
    }

    function session(action): void {
        root.requestClose();
        SessionActions.run(action);
    }

    Column {
        id: overview
        anchors.left: parent.left
        anchors.right: parent.right
        visible: root.currentPage === "controls"
        spacing: Tokens.s3 * root.s

        Repeater {
            model: SidebarState.visibleSections

            delegate: Loader {
                id: sectionLoader
                required property var modelData
                width: overview.width
                active: root.currentPage === "controls"
                sourceComponent: modelData.id === "vitals" ? vitalsSection
                    : modelData.id === "connections" ? connectionsSection
                    : modelData.id === "powerProfile" ? profileSection
                    : modelData.id === "media" ? mediaSection
                    : modelData.id === "levels" ? levelsSection
                    : bottomSection
                height: item ? item.implicitHeight : 0
                opacity: active ? 1 : 0
                transform: Translate { y: sectionLoader.active ? 0 : Tokens.s2 * root.s }

                Behavior on opacity {
                    enabled: root.motionAllowed
                    NumberAnimation {
                        duration: SidebarState.motionDuration(Tokens.swap)
                        easing.type: Tokens.ease
                    }
                }
            }
        }
    }

    Component {
        id: vitalsSection
        ControlsHero {
            width: overview.width
            height: implicitHeight
            s: root.s
            active: root.overviewActive
        }
    }

    Component {
        id: connectionsSection
        ControlsConnections {
            width: overview.width
            height: implicitHeight
            s: root.s
            screen: root.screen
            active: root.overviewActive
            onOpenPage: page => root.showPage(page)
        }
    }

    Component {
        id: profileSection
        PowerProfileControl {
            width: overview.width
            height: implicitHeight
            s: root.s
            active: root.overviewActive
        }
    }

    Component {
        id: mediaSection
        ControlsMedia {
            width: overview.width
            height: implicitHeight
            s: root.s
            active: root.overviewActive
        }
    }

    Component {
        id: levelsSection
        ControlsLevels {
            width: overview.width
            height: implicitHeight
            s: root.s
            screen: root.screen
            active: root.overviewActive
        }
    }

    Component {
        id: bottomSection
        ControlsBar {
            width: overview.width
            height: implicitHeight
            s: root.s
            screen: root.screen
            active: root.overviewActive
            onSession: action => root.session(action)
            onOpenExtensions: root.showPage("plugins")
            onRequestClose: root.requestClose()
        }
    }

    QQC.ScrollView {
        id: detailScroll
        anchors.fill: parent
        visible: root.currentPage !== "controls"
        clip: true
        contentWidth: availableWidth
        QQC.ScrollBar.horizontal.policy: QQC.ScrollBar.AlwaysOff

        Loader {
            width: detailScroll.availableWidth
            active: root.currentPage !== "controls"
            sourceComponent: root.currentPage === "wifi" ? wifiPage : bluetoothPage
        }
    }

    Component {
        id: wifiPage
        Cards.SystemWifiPage {
            s: root.s
            active: root.active && root.currentPage === "wifi"
            onBackRequested: root.showPage("controls")
        }
    }
    Component {
        id: bluetoothPage
        Cards.SystemBluetoothPage {
            s: root.s
            active: root.active && root.currentPage === "bluetooth"
            onBackRequested: root.showPage("controls")
        }
    }
}
