pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import Caelestia.Config
import caelestia.components
import caelestia.services

Item {
    id: root

    required property var bar
    required property ShellScreen screen
    required property bool fullscreen
    Config.screen: root.screen.name
    readonly property int barThickness: bar.thickness

    implicitWidth: container.implicitWidth
    implicitHeight: container.implicitHeight

    StyledClippingRect {
        id: container

        readonly property bool onSpecial: false
        readonly property bool isHorizontal: root.bar.isHorizontal
        property var kwinWindowList: Kwin.windowList

        readonly property int desktopCount: Math.max(1, Kwin.workspaces.length)
        readonly property int activeWsId: Kwin.activeWorkspaceFor(root.screen.name)
        readonly property int shown: Math.max(1, Config.bar.workspaces.shown)
        readonly property var occupied: {
            const occ = {};
            for (let i = 1; i <= container.desktopCount; ++i) {
                occ[i] = false;
            }
            const kwinList = container.kwinWindowList;
            if (kwinList) {
                for (let i = 0; i < kwinList.length; ++i) {
                    const w = kwinList[i];
                    if (Config.bar.workspaces.perMonitor && w.output !== root.screen.name)
                        continue;
                    if (w.workspace && typeof w.workspace.id === "number") {
                        occ[w.workspace.id] = true;
                    }
                }
            }
            return occ;
        }
        readonly property var wsIds: {
            if (Config.bar.workspaces.showUnoccupied) {
                const ids = [];
                const start = Math.floor((container.activeWsId - 1) / container.shown) * container.shown;
                for (let i = 0; i < container.shown; ++i) {
                    const id = start + i + 1;
                    if (id <= container.desktopCount)
                        ids.push(id);
                }
                return ids;
            }
            const ids = [];
            for (let id = 1; id <= container.desktopCount; ++id) {
                if (container.occupied[id] || id === container.activeWsId)
                    ids.push(id);
            }
            const currentIdx = ids.indexOf(container.activeWsId);
            if (currentIdx < 0)
                return [];
            const end = Math.max(currentIdx + 1, Math.min(container.shown, ids.length));
            return ids.slice(Math.max(0, end - container.shown), end);
        }
        readonly property var pills: {
            void workspaces.count;
            return container.wsIds.map((_, i) => workspaces.itemAt(i)).filter(p => p);
        }
        property real blur: onSpecial ? 1 : 0

        implicitWidth: isHorizontal ? (layout.implicitWidth + Tokens.padding.small) : barThickness
        implicitHeight: isHorizontal ? barThickness : (layout.implicitHeight + Tokens.padding.small)
        color: Colours.tPalette.m3surfaceContainer
        radius: Tokens.rounding.full

        Connections {
            function onWorkspacesChanged() {
                Kwin.refreshWindows();
            }

            target: Kwin
        }
        Item {
            anchors.fill: parent
            scale: container.onSpecial ? 0.8 : 1
            opacity: container.onSpecial ? 0.5 : 1
            layer.enabled: container.blur > 0
            layer.effect: MultiEffect {
                blurEnabled: true
                blur: container.blur
                blurMax: 32
            }

            Loader {
                asynchronous: true
                active: Config.bar.workspaces.occupiedBg
                anchors.fill: layout
                sourceComponent: OccupiedBg {
                    workspaces: container.pills
                    wsSpacing: Math.floor(Tokens.spacing.small)
                    isHorizontal: container.isHorizontal
                }
            }
            Loader {
                asynchronous: true
                active: opacity > 0
                opacity: Config.bar.workspaces.showUnoccupied ? 0 : 1
                anchors.fill: layout
                sourceComponent: GapMarkers {
                    workspaces: container.pills
                    wsSpacing: Math.floor(Tokens.spacing.small)
                    isHorizontal: container.isHorizontal
                }

                Behavior on opacity {
                    Anim {
                        type: Anim.DefaultEffects
                    }
                }
            }
            GridLayout {
                id: layout

                anchors.centerIn: parent
                columns: isHorizontal ? -1 : 1
                rows: isHorizontal ? 1 : -1
                flow: isHorizontal ? GridLayout.LeftToRight : GridLayout.TopToBottom
                columnSpacing: Math.floor(Tokens.spacing.small)
                rowSpacing: Math.floor(Tokens.spacing.small)

                Repeater {
                    id: workspaces

                    model: container.wsIds

                    Workspace {
                        required property int modelData

                        ws: modelData
                        activeWsId: container.activeWsId
                        occupied: container.occupied
                        screenName: root.screen.name
                    }
                }
            }
            Loader {
                asynchronous: true
                anchors.horizontalCenter: isHorizontal ? undefined : parent.horizontalCenter
                anchors.verticalCenter: isHorizontal ? parent.verticalCenter : undefined
                active: Config.bar.workspaces.activeIndicator
                sourceComponent: ActiveIndicator {
                    activeWsId: container.activeWsId
                    workspaces: container.pills
                    mask: layout
                    fullscreen: root.fullscreen
                    screenName: root.screen.name
                }
            }
            MouseArea {
                anchors.fill: layout
                acceptedButtons: Qt.LeftButton | Qt.RightButton
                onClicked: event => {
                    if (event.button === Qt.RightButton) {
                        const popouts = root.bar?.popouts;
                        if (!popouts)
                            return;
                        popouts.currentName = "workspacescontext";
                        popouts.currentCenter = root.bar.isHorizontal ? root.mapToItem(null, root.implicitWidth / 2, 0).x : (root.mapToItem(null, 0, root.implicitHeight / 2).y ?? 0);
                        popouts.hasCurrent = true;
                        return;
                    }
                    const pill = container.pills.find(p => container.isHorizontal ? event.x >= p.x && event.x <= p.x + p.width : event.y >= p.y && event.y <= p.y + p.height);
                    const ws = pill?.ws;
                    if (!ws)
                        return;
                    if (container.activeWsId !== ws)
                        Kwin.setDesktop(ws);
                }
                onWheel: event => {
                    if (!Config.bar.scrollActions.workspaces) return;

                    if (event.angleDelta.y > 0 || event.angleDelta.x > 0) {
                        Kwin.previousDesktop();
                    } else if (event.angleDelta.y < 0 || event.angleDelta.x < 0) {
                        Kwin.nextDesktop();
                    }
                }
            }
            Behavior on scale {
                Anim {}
            }
            Behavior on opacity {
                Anim {
                    type: Anim.DefaultEffects
                }
            }
        }
        Loader {
            id: specialWs

            asynchronous: true
            anchors.fill: parent
            anchors.margins: Tokens.padding.extraSmall
            active: opacity > 0
            scale: container.onSpecial ? 1 : 0.5
            opacity: container.onSpecial ? 1 : 0
            sourceComponent: SpecialWorkspaces {
                screen: root.screen
            }

            Behavior on scale {
                Anim {}
            }
            Behavior on opacity {
                Anim {
                    type: Anim.DefaultEffects
                }
            }
        }
        Behavior on blur {
            Anim {
                type: Anim.StandardSmall
            }
        }
    }
}
