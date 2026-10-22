import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.services
import caelestia.modules.bar as Bar
import caelestia.modules.dashboard as Dashboard
import caelestia.modules.launcher as Launcher
import caelestia.modules.notifications as Notifications
import caelestia.modules.osd as Osd
import caelestia.modules.overview as Overview
import caelestia.modules.session as Session
import caelestia.modules.sidebar as Sidebar
import caelestia.modules.utilities as Utilities
import caelestia.modules.bar.popouts as BarPopouts
import caelestia.modules.utilities.toasts as Toasts

Item {
    id: root

    required property ShellScreen screen
    Config.screen: screen.name
    required property ScreenState screenState
    required property DrawerVisibilities visibilities
    required property Bar.BarWrapper bar
    required property real borderThickness
    required property real overviewBorderThickness
    property var overviewAnimConfig
    readonly property alias osd: osd
    readonly property alias osdWrapper: osdWrapper
    readonly property alias notifications: notifications
    readonly property alias session: session
    readonly property alias sessionWrapper: sessionWrapper
    readonly property alias launcher: launcher
    readonly property alias dashboard: dashboard
    readonly property alias popouts: popoutsWrapper.content
    readonly property alias popoutsWrapper: popoutsWrapper
    readonly property alias utilities: utilities
    readonly property alias toasts: toasts
    readonly property alias sidebar: sidebar
    readonly property alias overview: overview
    readonly property real leftMargin: anchors.leftMargin
    readonly property real rightMargin: anchors.rightMargin
    readonly property real topMargin: anchors.topMargin
    readonly property real bottomMargin: anchors.bottomMargin
    property point cursorPos
    readonly property string notifAutoPosition: {
        const barPos = bar.position;
        const v = barPos === "bottom" ? "bottom" : "top";
        const h = barPos === "right" ? "left" : "right";
        return `${v}-${h}`;
    }
    property bool notifAutoFlipped: false
    property int _prevPopupCount: 0
    readonly property string notifPosition: {
        if (GlobalConfig.notifs.position !== "auto")
            return GlobalConfig.notifs.position;
        if (notifAutoFlipped) {
            const parts = notifAutoPosition.split("-");
            const v = parts[0] === "top" ? "bottom" : "top";
            const h = parts[1] === "left" ? "right" : "left";
            return `${v}-${h}`;
        }
        return notifAutoPosition;
    }
    readonly property bool notifAtTop: notifPosition.startsWith("top")
    readonly property bool notifAtBottom: notifPosition.startsWith("bottom")
    readonly property real notifReservedHeight: notifications.implicitHeight > 0 ? notifications.implicitHeight + Tokens.spacing.extraLarge : 0

    readonly property bool popoutIntersectsRight: {
        if (!popoutsWrapper.visible || popoutsWrapper.offsetScale >= 1) return false;
        if (bar.isHorizontal) {
            const notifLeft = notifications.x;
            const notifRight = notifLeft + (notifications.implicitWidth > 0 ? notifications.implicitWidth : Tokens.sizes.notifs.width);
            // Use where the popout ends up, not where its slide currently is, so
            // switching between bar items doesn't push the sidebar on the way.
            const popLeft = popoutsWrapper.targetX;
            const popRight = popoutsWrapper.targetX + popoutsWrapper.content.nonAnimWidth;
            return popLeft < notifRight && popRight > notifLeft;
        } else {
            const notifTop = notifications.y;
            const notifBottom = notifTop + (notifications.implicitHeight > 0 ? notifications.implicitHeight : 300);
            const popTop = popoutsWrapper.y;
            const popBottom = popoutsWrapper.y + popoutsWrapper.content.nonAnimHeight;
            return popTop < notifBottom && popBottom > notifTop;
        }
    }

    readonly property bool popoutIntersectsSidebar: {
        if (!popoutsWrapper.visible || popoutsWrapper.offsetScale >= 1) return false;
        if (!sidebar.visible) return false;
        if (bar.isHorizontal) {
            const sideLeft = sidebar.x;
            const sideRight = sideLeft + sidebar.width;
            const popLeft = popoutsWrapper.x;
            const popRight = popoutsWrapper.x + popoutsWrapper.content.nonAnimWidth;
            return popLeft < sideRight && popRight > sideLeft;
        } else {
            const sideTop = sidebar.y;
            const sideBottom = sideTop + sidebar.height;
            const popTop = popoutsWrapper.y;
            const popBottom = popoutsWrapper.y + popoutsWrapper.content.nonAnimHeight;
            return popTop < sideBottom && popBottom > sideTop;
        }
    }

    anchors.fill: parent
    anchors.leftMargin: (bar.position === "left" ? bar.implicitWidth + (GlobalConfig.appearance.islands ? Tokens.spacing.extraLarge * 2 : 0) : borderThickness + (GlobalConfig.appearance.islands ? Tokens.spacing.extraLarge : 0))
    anchors.rightMargin: (bar.position === "right" ? bar.implicitWidth + (GlobalConfig.appearance.islands ? Tokens.spacing.extraLarge * 2 : 0) : borderThickness + (GlobalConfig.appearance.islands ? Tokens.spacing.extraLarge : 0))
    anchors.topMargin: (bar.position === "top" ? bar.implicitHeight + (GlobalConfig.appearance.islands ? Tokens.spacing.extraLarge * 2 : 0) : borderThickness + (GlobalConfig.appearance.islands ? Tokens.spacing.extraLarge : 0))
    anchors.bottomMargin: (bar.position === "bottom" ? bar.implicitHeight + (GlobalConfig.appearance.islands ? Tokens.spacing.extraLarge * 2 : 0) : borderThickness + (GlobalConfig.appearance.islands ? Tokens.spacing.extraLarge : 0))
    states: [
        State {
            name: "right"
            when: bar.position === "right"

            AnchorChanges {
                target: osdWrapper
                anchors.left: parent.left
                anchors.right: undefined
            }
            AnchorChanges {
                target: osd
                anchors.left: parent.left
                anchors.right: undefined
            }
            AnchorChanges {
                target: sessionWrapper
                anchors.left: parent.left
                anchors.right: undefined
            }
            AnchorChanges {
                target: session
                anchors.left: parent.left
                anchors.right: undefined
            }
            AnchorChanges {
                target: utilities
                anchors.left: parent.left
                anchors.right: undefined
            }
            AnchorChanges {
                target: toasts
                anchors.left: sidebar.right
                anchors.right: undefined
            }
            AnchorChanges {
                target: sidebar
                anchors.left: parent.left
                anchors.right: undefined
            }
        },
        State {
            name: "bottom"
            when: bar.position === "bottom"

            AnchorChanges {
                target: utilities
                anchors.bottom: undefined
                anchors.top: parent.top
            }
            AnchorChanges {
                target: toasts
                anchors.bottom: parent.bottom
                anchors.left: parent.left
                anchors.right: undefined
            }
            AnchorChanges {
                target: sidebar
                anchors.top: utilities.bottom
                anchors.bottom: parent.bottom
            }
            PropertyChanges {
                target: sidebar
                anchors.topMargin: -4
                anchors.bottomMargin: (root.notifAtBottom ? root.notifReservedHeight : 0)
                    + (sidebar.shouldPush ? popoutsWrapper.implicitHeight + sidebar.pushSpacing : 0)
            }
        }
    ]

    Item {
        id: osdWrapper

        property string vAnchor: "center"
        property string hAnchor: bar.position === "right" ? "left" : "right"

        anchors.verticalCenter: parent.verticalCenter
        anchors.right: parent.right
        anchors.leftMargin: bar.position === "right" ? sidebar.width * (1 - sidebar.offsetScale) + session.width * (1 - session.offsetScale) : 0
        anchors.rightMargin: bar.position !== "right" ? sidebar.width * (1 - sidebar.offsetScale) + session.width * (1 - session.offsetScale) : 0
        clip: sidebar.visible || session.visible
        implicitWidth: osd.implicitWidth * (1 - osd.offsetScale)
        implicitHeight: osd.implicitHeight
        visible: osd.offsetScale < 1

        Osd.Wrapper {
            id: osd

            screen: root.screen
            visibilities: root.visibilities
            sidebarOrSessionVisible: sidebar.visible || session.visible
            anchors.verticalCenter: parent.verticalCenter
            anchors.right: parent.right
        }
    }
    Notifications.Wrapper {
        id: notifications

        readonly property string _notifV: root.notifPosition.split("-")[0]
        readonly property string _notifH: root.notifPosition.split("-")[1]
        property string vAnchor: _notifV
        property string hAnchor: _notifH
        property bool shouldPush: root.popoutIntersectsRight && !popoutsWrapper.content.isDockPopout && !sidebar.visible

        readonly property real _pushOffset: shouldPush ? (popoutsWrapper.implicitHeight + Tokens.spacing.extraLarge) : 0

        visibilities: root.visibilities
        screen: root.screen
        sidebarPanel: sidebar
        osdPanel: osdWrapper
        sessionPanel: sessionWrapper
        utilitiesPanel: utilities

        width: implicitWidth
        height: implicitHeight

        x: {
            if (_notifH === "left") return 0;
            if (_notifH === "right") return Math.max(0, parent.width - implicitWidth);
            return Math.max(0, (parent.width - implicitWidth) / 2);
        }
        y: {
            if (_notifV === "top") return _pushOffset;
            return Math.max(0, parent.height - implicitHeight - _pushOffset);
        }
    }
    Connections {
        function onPopupCountChanged() {
            const len = Notifs.popupCount;
            if (len > 0 && root._prevPopupCount === 0 && GlobalConfig.notifs.position === "auto") {
                const defParts = root.notifAutoPosition.split("-");
                const defV = defParts[0];
                const defH = defParts[1];
                const notifW = Tokens.sizes.notifs.width;
                const notifH = Math.min(root.height / 2, 600);
                const panelX = root.cursorPos.x - root.leftMargin;
                const panelY = root.cursorPos.y - root.topMargin;
                const cornerX = defH === "right" ? (root.width - notifW) : 0;
                const cornerY = defV === "bottom" ? (root.height - notifH) : 0;
                const inCorner = panelX >= cornerX && panelX <= cornerX + notifW
                    && panelY >= cornerY && panelY <= cornerY + notifH;
                root.notifAutoFlipped = inCorner;
            }
            root._prevPopupCount = len;
        }

        target: Notifs
    }
    Item {
        id: sessionWrapper

        property string vAnchor: "center"
        property string hAnchor: bar.position === "right" ? "left" : "right"

        anchors.verticalCenter: parent.verticalCenter
        anchors.right: parent.right
        anchors.leftMargin: bar.position === "right" ? sidebar.width * (1 - sidebar.offsetScale) : 0
        anchors.rightMargin: bar.position !== "right" ? sidebar.width * (1 - sidebar.offsetScale) : 0
        clip: sidebar.visible
        implicitWidth: session.implicitWidth * (1 - session.offsetScale)
        implicitHeight: session.implicitHeight
        visible: session.offsetScale < 1

        Session.Wrapper {
            id: session

            visibilities: root.visibilities
            sidebarVisible: sidebar.visible
            anchors.verticalCenter: parent.verticalCenter
            anchors.right: parent.right
        }
    }
    Launcher.Wrapper {
        id: launcher

        property string vAnchor: "bottom"
        property string hAnchor: "center"

        screen: root.screen
        visibilities: root.visibilities
        panels: root
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
    }
    Dashboard.Wrapper {
        id: dashboard

        property string vAnchor: "top"
        property string hAnchor: "center"

        screenState: root.screenState
        visibilities: root.visibilities
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.top: parent.top
    }
    BarPopouts.ClipWrapper {
        id: popoutsWrapper

        property string vAnchor: (bar.position === "top" || bar.position === "bottom") ? bar.position : "none"
        property string hAnchor: (bar.position === "left" || bar.position === "right") ? bar.position : "none"

        screen: root.screen
        bar: root.bar
        borderThickness: root.borderThickness
        visibilities: root.visibilities
    }
    Utilities.Wrapper {
        id: utilities

        property string vAnchor: bar.position === "bottom" ? "top" : "bottom"
        property string hAnchor: bar.position === "right" ? "left" : "right"

        visibilities: root.visibilities
        sidebar: sidebar
        popouts: popoutsWrapper.content
        anchors.bottom: parent.bottom
        anchors.right: parent.right
    }
    Toasts.Toasts {
        id: toasts

        property string vAnchor: "bottom"
        property string hAnchor: bar.position === "bottom" ? "left" : (bar.position === "right" ? "left" : "right")

        visibilities: root.visibilities
        anchors.bottom: sidebar.visible ? parent.bottom : utilities.top
        anchors.right: sidebar.left
        anchors.margins: Tokens.padding.medium
    }
    Sidebar.Wrapper {
        id: sidebar

        property string vAnchor: "bottom"
        property string hAnchor: bar.position === "right" ? "left" : "right"
        property bool shouldPush: root.popoutIntersectsSidebar && !popoutsWrapper.content.isDockPopout
        // A popout no wider than the sidebar is drawn joined to it (ContentWindow.qml),
        // so it goes right below the sidebar instead of leaving a gap.
        readonly property real pushSpacing: popoutsWrapper.content.implicitWidth <= Visibilities.sidebarWidthFor(Tokens.sizes.sidebar.width) + 1 ? 0 : Tokens.spacing.extraLarge

        visibilities: root.visibilities
        popouts: popoutsWrapper.content
        utilities: utilities
        followsPopout: shouldPush
        anchors.top: parent.top
        anchors.bottom: utilities.top
        anchors.right: parent.right
        anchors.topMargin: root.notifAtTop
            ? (root.notifReservedHeight + ((bar.position === "top" && shouldPush) ? popoutsWrapper.implicitHeight + sidebar.pushSpacing : 0))
            : ((bar.position === "top" && shouldPush) ? (popoutsWrapper.implicitHeight + sidebar.pushSpacing) : 0)
        anchors.bottomMargin: (bar.position === "bottom" && shouldPush) ? (popoutsWrapper.implicitHeight + sidebar.pushSpacing) : 0
    }
    Overview.Wrapper {
        id: overview

        property string vAnchor: "center"
        property string hAnchor: "center"
        property var animConfig: root.overviewAnimConfig

        screen: root.screen
        visibilities: root.visibilities
        panels: root
    }
}
