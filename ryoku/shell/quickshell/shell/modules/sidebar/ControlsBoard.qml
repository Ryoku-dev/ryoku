pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "cards" as Cards
import "../ask/look"

Item {
    id: root

    required property real s
    required property var screen
    required property bool active
    property string page: "controls"
    property Item card: null
    // the shipped panel names the person in its head so System leaves it out
    property bool heroIdentity: true
    property var pluginCards: []
    property real bleed: 0
    signal requestClose()

    readonly property string currentPage: root.page === "wifi" || root.page === "bluetooth" ? root.page
        : root.page === "plugins" && root.pluginCards.length > 0 ? "plugins" : "controls"
    readonly property bool overviewActive: root.active && root.currentPage === "controls"
    readonly property real contentHeight: overview.current ? overview.viewHeight
        : wifi.current ? wifi.viewHeight : bluetooth.current ? bluetooth.viewHeight : plugins.viewHeight
    property int direction: 1
    property bool handing: false
    readonly property real headGap: Math.round((QsLook.lithic ? 8 : 10) * root.s)
    signal revealRun()

    implicitHeight: root.shownHeight
    property real shownHeight: root.contentHeight
    Behavior on shownHeight {
        enabled: root.handing
        NumberAnimation { duration: QsLook.dur(Look.viewMs); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.arrive }
    }

    onCurrentPageChanged: {
        root.direction = root.currentPage === "controls" ? -1 : 1;
        root.handing = true;
        handOver.restart();
    }
    onOverviewActiveChanged: if (root.overviewActive && root.currentPage === "controls") root.revealRun()

    Timer {
        id: handOver
        interval: Look.viewMs + 40
        onTriggered: root.handing = false
    }

    function showPage(value): void {
        SidebarState.selectTab(root.screen, value);
    }

    function back(): bool {
        if (root.currentPage === "controls")
            return false;
        root.showPage("controls");
        return true;
    }

    function session(action): void {
        root.requestClose();
        SessionActions.run(action);
    }

    function titleFor(block): string {
        return block === "connections" ? I18n.tr("Connections")
            : block === "levels" ? I18n.tr("Volume and brightness")
            : block === "media" ? I18n.tr("Now playing")
            : block === "powerProfile" ? I18n.tr("Power mode")
            : block === "vitals" ? I18n.tr("System")
            : I18n.tr("Session");
    }

    Flickable {
        id: scroller
        anchors.fill: parent
        clip: root.height < root.contentHeight - 0.5 || root.handing
        contentWidth: width
        contentHeight: root.contentHeight
        boundsBehavior: Flickable.StopAtBounds
        interactive: contentHeight > height + 0.5

        Item {
            id: stack
            width: scroller.width
            height: root.contentHeight

            PageSlot {
                id: overview
                s: root.s
                keep: true
                current: root.currentPage === "controls"
                direction: root.direction
                sourceComponent: overviewView
            }
            PageSlot {
                id: wifi
                s: root.s
                current: root.currentPage === "wifi"
                direction: root.direction
                sourceComponent: wifiPage
            }
            PageSlot {
                id: bluetooth
                s: root.s
                current: root.currentPage === "bluetooth"
                direction: root.direction
                sourceComponent: bluetoothPage
            }
            PageSlot {
                id: plugins
                s: root.s
                current: root.currentPage === "plugins"
                direction: root.direction
                sourceComponent: extensionsPage
            }
        }
    }
    ScrollRail {
        anchors.top: scroller.top
        anchors.bottom: scroller.bottom
        anchors.right: scroller.right
        anchors.rightMargin: -Math.round(root.bleed / 2)
        flickable: scroller
    }

    Component {
        id: overviewView

        Column {
            id: blocks
            spacing: Math.round(QsLook.gap * root.s)

            Repeater {
                model: SidebarState.visibleSections

                delegate: Item {
                    id: section

                    required property var modelData
                    required property int index
                    readonly property string blockId: section.modelData.id
                    readonly property bool headed: QsLook.headed(section.blockId)
                    readonly property real blockHeight: block.item ? block.item.implicitHeight : 0
                    readonly property bool terminal: section.index === SidebarState.visibleSections.length - 1

                    width: blocks.width
                    height: section.blockHeight > 0
                        ? (section.headed ? head.implicitHeight + root.headGap : 0) + section.blockHeight : 0
                    visible: section.blockHeight > 0

                    property real rise: 1
                    opacity: section.rise
                    transform: Translate { y: (1 - section.rise) * QsLook.revealTravel * root.s }
                    SequentialAnimation {
                        id: riseRun
                        PauseAnimation { duration: QsLook.panelMs(60 + Math.min(section.index, 5) * QsLook.revealStagger) }
                        NumberAnimation {
                            target: section
                            property: "rise"
                            to: 1
                            duration: QsLook.panelMs(QsLook.revealMs)
                            easing.type: Easing.BezierSpline
                            easing.bezierCurve: Lithic.curve
                        }
                    }
                    Connections {
                        target: root
                        function onRevealRun() {
                            if (QsLook.still || section.index > 5) {
                                section.rise = 1;
                                return;
                            }
                            section.rise = 0;
                            riseRun.restart();
                        }
                    }

                    SectionHead {
                        id: head
                        visible: section.headed
                        width: parent.width
                        s: root.s
                        title: root.titleFor(section.blockId)
                        ordinal: String(section.index + 1).padStart(2, "0") + " / "
                            + String(SidebarState.visibleSections.length).padStart(2, "0")
                    }
                    Loader {
                        id: block
                        y: section.headed ? head.implicitHeight + root.headGap : 0
                        width: parent.width
                        height: section.blockHeight
                        sourceComponent: section.blockId === "vitals" ? vitalsBlock
                            : section.blockId === "connections" ? connectionsBlock
                            : section.blockId === "powerProfile" ? profileBlock
                            : section.blockId === "media" ? mediaBlock
                            : section.blockId === "levels" ? levelsBlock
                            : sessionBlock
                        onLoaded: {
                            if (section.blockId === "bottomControls") {
                                item.terminal = Qt.binding(() => section.terminal);
                                item.bleed = Qt.binding(() => root.bleed);
                            }
                        }
                    }
                }
            }
        }
    }

    Component {
        id: vitalsBlock
        ControlsHero {
            s: root.s
            active: root.overviewActive
            identity: root.heroIdentity
        }
    }
    Component {
        id: connectionsBlock
        ControlsConnections {
            s: root.s
            screen: root.screen
            active: root.overviewActive
            onOpenPage: page => root.showPage(page)
            onRequestClose: root.requestClose()
        }
    }
    Component {
        id: profileBlock
        PowerProfileControl {
            s: root.s
            active: root.overviewActive
        }
    }
    Component {
        id: mediaBlock
        ControlsMedia {
            s: root.s
            active: root.overviewActive
        }
    }
    Component {
        id: levelsBlock
        ControlsLevels {
            s: root.s
            screen: root.screen
            active: root.overviewActive
        }
    }
    Component {
        id: sessionBlock
        ControlsBar {
            s: root.s
            screen: root.screen
            card: root.card
            active: root.overviewActive
            onSession: action => root.session(action)
            onOpenExtensions: root.showPage("plugins")
            onRequestClose: root.requestClose()
        }
    }

    Component {
        id: wifiPage
        Cards.SystemWifiPage {
            s: root.s
            card: root.card
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
    Component {
        id: extensionsPage
        ExtensionsBoard {
            s: root.s
            plugins: root.pluginCards
            active: root.active && root.currentPage === "plugins"
            onRequestClose: root.requestClose()
        }
    }
}
