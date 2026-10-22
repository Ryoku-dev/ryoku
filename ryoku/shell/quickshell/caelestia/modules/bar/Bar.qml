pragma ComponentBehavior: Bound

import "popouts" as BarPopouts
import "components"
import "components/workspaces"
import "components/performance"
import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Services.UPower
import Caelestia.Config
import Caelestia.Services
import caelestia.components
import caelestia.services
import caelestia.utils

Item {
    id: root

    required property ShellScreen screen
    Config.screen: screen.name
    required property DrawerVisibilities visibilities
    required property BarPopouts.Wrapper popouts
    required property bool fullscreen
    property Item currentHoveredItem: null

    readonly property int vPadding: Tokens.padding.large

    readonly property real rawScale: !isNaN(Config.bar.scale) ? Config.bar.scale : 1.0
    readonly property real barScale: rawScale < 1.0 ? Math.sqrt(Math.max(0.1, rawScale)) : rawScale
    readonly property int thickness: Math.round(Tokens.sizes.bar.innerWidth * barScale)

    readonly property bool isHorizontal: Config.bar.position === "top" || Config.bar.position === "bottom"

    readonly property real leftZoneSize: isHorizontal ? leftLayout.implicitWidth : leftLayout.implicitHeight
    readonly property real middleZoneSize: isHorizontal ? middleLayout.implicitWidth : middleLayout.implicitHeight
    readonly property real rightZoneSize: isHorizontal ? rightLayout.implicitWidth : rightLayout.implicitHeight

    property var leftEntries: {
        let entries = Config.bar.entries || [];
        return entries.filter(e => e.enabled && HostState.allows(e) && (!e.zone || e.zone === "left") && e.id !== "spacer");
    }

    property var middleEntries: {
        let entries = Config.bar.entries || [];
        return entries.filter(e => e.enabled && HostState.allows(e) && e.zone === "middle" && e.id !== "spacer");
    }

    property var rightEntries: {
        let entries = Config.bar.entries || [];
        return entries.filter(e => e.enabled && HostState.allows(e) && e.zone === "right" && e.id !== "spacer");
    }

    function resetHover(): void {
        if (currentHoveredItem) {
            if (currentHoveredItem.hasOwnProperty("hoverPos"))
                currentHoveredItem.hoverPos = -1;
            currentHoveredItem = null;
        }
    }

    function getLoaderAt(x, y) {
        let items = [leftLayout, middleLayout, rightLayout];
        for (let i = 0; i < items.length; i++) {
            let layout = items[i];
            let localPos = mapToItem(layout, x, y);
            if (localPos.x >= 0 && localPos.x <= layout.width && localPos.y >= 0 && localPos.y <= layout.height) {
                let ch = layout.childAt(localPos.x, localPos.y);
                if (ch && ch.hasOwnProperty("id")) return ch;
            }
        }
        return null;
    }

    function closeTray(): void {
        if (!Config.bar.tray.compact)
            return;

        let repeaters = [leftRepeater, middleRepeater, rightRepeater];
        for (let r = 0; r < 3; r++) {
            let rep = repeaters[r];
            for (let i = 0; i < rep.count; i++) {
                const loader = rep.itemAt(i) as WrappedLoader;
                if (loader?.enabled && loader.id === "tray") {
                    const tray = loader.item as Tray;
                    if (tray && (Config.bar.popouts.tray || !tray.pinned)) {
                        tray.expanded = false;
                        tray.pinned = false;
                    }
                }
            }
        }
    }

    function checkPopout(pos: real): void {
        if (popouts.hasCurrent && (popouts.currentName === "clockcontext" || popouts.currentName === "dockcontext" || popouts.currentName === "greetercontext" || popouts.currentName === "statusiconscontext" || popouts.currentName === "workspacescontext" || popouts.currentName === "traycontext" || popouts.currentName === "githubcontext" || popouts.currentName === "osiconcontext" || popouts.currentName === "dockbgcontext"))
            return;

        const ch = getLoaderAt(isHorizontal ? pos : width / 2, isHorizontal ? height / 2 : pos) as WrappedLoader;

        if (currentHoveredItem && currentHoveredItem !== ch?.item) {
            if (currentHoveredItem.hasOwnProperty("hoverPos"))
                currentHoveredItem.hoverPos = -1;
        }
        currentHoveredItem = ch ? ch.item : null;

        if (ch?.id !== "tray")
            closeTray();

        if (!ch) {
            if (popouts.hasCurrent && (popouts.currentName === "dockhover" || popouts.currentName === "greeter")) return;
            if (!Config.bar.popouts.tray && popouts.currentName.startsWith("traymenu")) return;
            popouts.hasCurrent = false;
            return;
        }

        const id = ch.id;
        let mappedChPos = mapFromItem(ch, 0, 0);
        const top = isHorizontal ? mappedChPos.x : mappedChPos.y;

        if (id === "tray" && !Config.bar.popouts.tray) {
            return;
        } else if (id === "tray" && Config.bar.popouts.tray && !visibilities.sidebar) {
            const tray = ch.item as Tray;
            const mouseMap = mapToItem(tray.expandIcon, isHorizontal ? pos : tray.implicitWidth / 2, isHorizontal ? tray.implicitHeight / 2 : pos);
            if (!Config.bar.tray.compact || (tray.expanded && !tray.expandIcon.contains(mouseMap))) {
                const traySize = isHorizontal ? tray.layout.implicitWidth : tray.layout.implicitHeight;
                const localX = isHorizontal ? tray.layout.mapFromItem(null, pos, 0).x : tray.layout.mapFromItem(null, 0, pos).y;
                const index = Math.floor((localX / (traySize + Tokens.spacing.small)) * tray.items.count);
                const trayItem = tray.items.itemAt(index);
                if (trayItem && localX >= 0 && localX <= traySize) {
                    popouts.currentName = `traymenu${index}`;
                    popouts.currentCenter = isHorizontal ? trayItem.mapToItem(null, trayItem.implicitWidth / 2, 0).x : trayItem.mapToItem(null, 0, trayItem.implicitHeight / 2).y;
                    popouts.hasCurrent = true;
                } else {
                    popouts.hasCurrent = false;
                }
            } else {
                popouts.hasCurrent = false;
                tray.expanded = true;
            }
        } else if (id === "statusIcons") {
            const statusIcons = ch.item as StatusIcons;
            const items = statusIcons.items;
            const localX = isHorizontal ? items.mapFromItem(null, pos, 0).x : items.width / 2;
            const localY = isHorizontal ? items.height / 2 : items.mapFromItem(null, 0, pos).y;
            statusIcons.hoverPos = isHorizontal ? localX : localY;

            if (!Config.bar.popouts.statusIcons) return;

            let icon = items.childAt(localX, localY);
            if (!icon) {
                let bestDist = 1e9;
                for (let i = 0; i < items.children.length; i++) {
                    const child = items.children[i];
                    if (!child.visible || !child.name) continue;
                    const center = isHorizontal
                        ? child.mapToItem(items, child.width / 2, 0).x
                        : child.mapToItem(items, 0, child.height / 2).y;
                    const dist = Math.abs((isHorizontal ? localX : localY) - center);
                    if (dist < bestDist) {
                        bestDist = dist;
                        icon = child;
                    }
                }
            }
            if (icon) {
                popouts.currentName = icon.popoutName ?? icon.name;
                popouts.currentCenter = isHorizontal ? icon.mapToItem(null, icon.width / 2, 0).x : icon.mapToItem(null, 0, icon.height / 2).y;
                popouts.hasCurrent = true;
            } else {
                popouts.hasCurrent = false;
            }
        } else if (id === "greeter" && Config.bar.popouts.greeter && Config.bar.greeter.showOnHover) {
            const item = ch.item as Item;
            if (item) {
                const relPos = pos - top;
                const inside = isHorizontal ? (relPos >= 0 && relPos <= item.implicitWidth) : (relPos >= 0 && relPos <= item.implicitHeight);
                if (inside) {
                    if (!popouts.hasCurrent || popouts.currentName !== "greetercontext") {
                        popouts.currentName = "greeter";
                        popouts.currentCenter = isHorizontal ? item.mapToItem(null, item.implicitWidth / 2, 0).x : (item.mapToItem(null, 0, item.implicitHeight / 2).y ?? 0);
                        popouts.hasCurrent = true;
                    }
                } else {
                    if (popouts.currentName !== "greetercontext") {
                        popouts.hasCurrent = false;
                    }
                }
            } else {
                popouts.hasCurrent = false;
            }
        } else if (id === "dock") {
            if (popouts.hasCurrent && (popouts.currentName === "dockcontext" || popouts.currentName === "dockbgcontext" || popouts.currentName === "greeter" || popouts.currentName === "greetercontext" || popouts.currentName === "statusiconscontext")) return;

            const item = ch.item;
            if (item && typeof item.handleHover === "function") {
                const relPos = pos - top;
                item.handleHover(relPos, isHorizontal, popouts);
                return;
            }
            popouts.hasCurrent = false;
        } else if (id === "github") {
            const item = ch.item as Item;
            if (item) {
                const relPos = pos - top;
                const inside = isHorizontal ? (relPos >= 0 && relPos <= item.implicitWidth) : (relPos >= 0 && relPos <= item.implicitHeight);
                if (inside) {
                    popouts.currentName = "github";
                    popouts.currentCenter = isHorizontal ? item.mapToItem(null, item.implicitWidth / 2, 0).x : (item.mapToItem(null, 0, item.implicitHeight / 2).y ?? 0);
                    popouts.hasCurrent = true;
                } else {
                    popouts.hasCurrent = false;
                }
            } else {
                popouts.hasCurrent = false;
            }
        } else if (id === "media") {
            const item = ch.item as Item;
            if (item) {
                const relPos = pos - top;
                const inside = isHorizontal ? (relPos >= 0 && relPos <= item.implicitWidth) : (relPos >= 0 && relPos <= item.implicitHeight);
                if (inside) {
                    popouts.currentName = "media";
                    popouts.currentCenter = isHorizontal ? item.mapToItem(null, item.implicitWidth / 2, 0).x : (item.mapToItem(null, 0, item.implicitHeight / 2).y ?? 0);
                    popouts.hasCurrent = true;
                } else {
                    popouts.hasCurrent = false;
                }
            } else {
                popouts.hasCurrent = false;
            }
        } else if (id === "updateIndicator") {
            const item = ch.item as Item;
            if (item) {
                const relPos = pos - top;
                const inside = isHorizontal ? (relPos >= 0 && relPos <= item.implicitWidth) : (relPos >= 0 && relPos <= item.implicitHeight);
                if (inside) {
                    popouts.currentName = "updateIndicator";
                    popouts.currentCenter = isHorizontal ? item.mapToItem(null, item.implicitWidth / 2, 0).x : (item.mapToItem(null, 0, item.implicitHeight / 2).y ?? 0);
                    popouts.hasCurrent = true;
                } else {
                    popouts.hasCurrent = false;
                }
            } else {
                popouts.hasCurrent = false;
            }
        } else if (id === "clock") {
            if (Config.bar.popouts.clock) {
                const item = ch.item as Item;
                if (item) {
                    const relPos = pos - top;
                    const inside = isHorizontal ? (relPos >= 0 && relPos <= item.implicitWidth) : (relPos >= 0 && relPos <= item.implicitHeight);
                    if (inside) {
                        popouts.currentName = "clock";
                        popouts.currentCenter = isHorizontal ? item.mapToItem(null, item.implicitWidth / 2, 0).x : (item.mapToItem(null, 0, item.implicitHeight / 2).y ?? 0);
                        popouts.hasCurrent = true;
                    } else {
                        popouts.hasCurrent = false;
                    }
                } else {
                    popouts.hasCurrent = false;
                }
            } else {
                popouts.hasCurrent = false;
            }
        } else {
            popouts.hasCurrent = false;
        }
    }

    function handleWheel(pos: real, angleDelta: point): void {
        const ch = getLoaderAt(isHorizontal ? pos : width / 2, isHorizontal ? height / 2 : pos) as WrappedLoader;

        if (ch?.id === "dock") {
            if (ch.item && typeof ch.item.handleWheel === "function") {
                ch.item.handleWheel(angleDelta);
            }
            return;
        }

        if (ch?.id === "workspaces" && Config.bar.scrollActions.workspaces) {
            const mon = (Config.bar.workspaces.perMonitor ? Kwin.monitorFor(screen) : Kwin.focusedMonitor);
            const specialWs = mon?.lastIpcObject.specialWorkspace.name;
            if (specialWs?.length > 0)
                Kwin.dispatch(Kwin.usingLua ? `hl.dsp.workspace.toggle_special("${specialWs.slice(8)}")` : `togglespecialworkspace ${specialWs.slice(8)}`);
            else {
                const activeId = Kwin.activeWsId;
                if (angleDelta.y < 0 || activeId > 1)
                    Kwin.dispatch(Kwin.usingLua ? `hl.dsp.focus({ workspace = "r${angleDelta.y > 0 ? "-" : "+"}1" })` : `workspace r${angleDelta.y > 0 ? "-" : "+"}1`);
            }
        } else if ((isHorizontal ? pos < screen.width / 2 : pos < screen.height / 2) && Config.bar.scrollActions.volume) {
            if (angleDelta.y > 0)
                Audio.incrementVolume();
            else if (angleDelta.y < 0)
                Audio.decrementVolume();
        } else if (Config.bar.scrollActions.brightness) {
            const monitor = Brightness.getMonitorForScreen(screen);
            if (angleDelta.y > 0)
                monitor.stepBrightness(GlobalConfig.services.brightnessIncrement);
            else if (angleDelta.y < 0)
                monitor.stepBrightness(-GlobalConfig.services.brightnessIncrement);
        }
    }

    Component.onCompleted: Logger.mark("bar-ready")

    clip: true

    GridLayout {
        id: leftLayout

        anchors.left: isHorizontal ? parent.left : undefined
        anchors.top: !isHorizontal ? parent.top : undefined
        anchors.verticalCenter: isHorizontal ? parent.verticalCenter : undefined
        anchors.horizontalCenter: !isHorizontal ? parent.horizontalCenter : undefined

        anchors.leftMargin: isHorizontal ? root.vPadding : 0
        anchors.topMargin: !isHorizontal ? root.vPadding : 0

        columns: isHorizontal ? -1 : 1
        rows: isHorizontal ? 1 : -1
        flow: isHorizontal ? GridLayout.LeftToRight : GridLayout.TopToBottom
        columnSpacing: Tokens.spacing.medium
        rowSpacing: Tokens.spacing.medium

        Repeater {
            id: leftRepeater

            model: root.leftEntries
            delegate: barDelegate
        }
    }

    GridLayout {
        id: middleLayout

        property real idealX: (parent.width - width) / 2
        property real minX: leftLayout.x + leftLayout.width + Tokens.spacing.medium
        property real maxX: rightLayout.x - width - Tokens.spacing.medium
        property real idealY: (parent.height - height) / 2
        property real minY: leftLayout.y + leftLayout.height + Tokens.spacing.medium
        property real maxY: rightLayout.y - height - Tokens.spacing.medium

        anchors.verticalCenter: isHorizontal ? parent.verticalCenter : undefined
        anchors.horizontalCenter: !isHorizontal ? parent.horizontalCenter : undefined
        columns: isHorizontal ? -1 : 1
        rows: isHorizontal ? 1 : -1
        flow: isHorizontal ? GridLayout.LeftToRight : GridLayout.TopToBottom
        columnSpacing: Tokens.spacing.medium
        rowSpacing: Tokens.spacing.medium

        Binding on x {
            when: isHorizontal
            value: Math.max(middleLayout.minX, Math.min(middleLayout.idealX, middleLayout.maxX))
        }

        Binding on y {
            when: !isHorizontal
            value: Math.max(middleLayout.minY, Math.min(middleLayout.idealY, middleLayout.maxY))
        }

        Repeater {
            id: middleRepeater

            model: root.middleEntries
            delegate: barDelegate
        }
    }

    GridLayout {
        id: rightLayout

        anchors.right: isHorizontal ? parent.right : undefined
        anchors.bottom: !isHorizontal ? parent.bottom : undefined
        anchors.verticalCenter: isHorizontal ? parent.verticalCenter : undefined
        anchors.horizontalCenter: !isHorizontal ? parent.horizontalCenter : undefined

        anchors.rightMargin: isHorizontal ? root.vPadding : 0
        anchors.bottomMargin: !isHorizontal ? root.vPadding : 0

        columns: isHorizontal ? -1 : 1
        rows: isHorizontal ? 1 : -1
        flow: isHorizontal ? GridLayout.LeftToRight : GridLayout.TopToBottom
        columnSpacing: Tokens.spacing.medium
        rowSpacing: Tokens.spacing.medium

        Repeater {
            id: rightRepeater

            model: root.rightEntries
            delegate: barDelegate
        }
    }

    DelegateChooser {
        id: barDelegate

        role: "id"

            DelegateChoice {
                roleValue: "logo"
                delegate: WrappedLoader {
                    sourceComponent: OsIcon {
                        bar: root
                    }
                }
            }
            DelegateChoice {
                roleValue: "workspaces"
                delegate: WrappedLoader {
                    sourceComponent: Workspaces {
                        bar: root
                        screen: root.screen
                        fullscreen: root.fullscreen
                    }
                }
            }
            DelegateChoice {
                roleValue: "dock"
                delegate: WrappedLoader {
                    sourceComponent: Dock {
                        bar: root
                    }
                }
            }
            DelegateChoice {
                roleValue: "greeter"
                delegate: WrappedLoader {
                    sourceComponent: Greeter {
                        bar: root
                        monitor: Brightness.getMonitorForScreen(root.screen)
                    }
                }
            }
            DelegateChoice {
                roleValue: "tray"
                delegate: WrappedLoader {
                    sourceComponent: Tray {
                        popouts: root.popouts
                    }
                }
            }
            DelegateChoice {
                roleValue: "clock"
                delegate: WrappedLoader {
                    sourceComponent: Clock {
                        bar: root
                    }
                }
            }
            DelegateChoice {
                roleValue: "statusIcons"
                delegate: WrappedLoader {
                    sourceComponent: StatusIcons {
                        bar: root
                    }
                }
            }
            DelegateChoice {
                roleValue: "kbLayoutIndicator"
                delegate: WrappedLoader {
                    visible: enabled && (Kwin.kbLayout || "").length > 0
                    sourceComponent: KbLayoutIndicator {}
                }
            }
            DelegateChoice {
                roleValue: "notificationsIndicator"
                delegate: WrappedLoader {
                    sourceComponent: NotificationsIndicator {}
                }
            }
            DelegateChoice {
                roleValue: "updateIndicator"
                delegate: WrappedLoader {
                    visible: enabled && GlobalConfig.general.checkUpdates
                    sourceComponent: UpdateIndicator {}
                }
            }
            DelegateChoice {
                roleValue: "perfCpu"
                delegate: WrappedLoader {
                    visible: enabled && Cpu.name.length > 0
                    sourceComponent: PerfCpu {}
                }
            }
            DelegateChoice {
                roleValue: "perfMemory"
                delegate: WrappedLoader {
                    visible: enabled && Memory.total > 1
                    sourceComponent: PerfMemory {}
                }
            }
            DelegateChoice {
                roleValue: "perfStorage"
                delegate: WrappedLoader {
                    visible: enabled && Storage.disks.length > 0
                    sourceComponent: PerfStorage {}
                }
            }
            DelegateChoice {
                roleValue: "perfNetwork"
                delegate: WrappedLoader {
                    sourceComponent: PerfNetwork {}
                }
            }
            DelegateChoice {
                roleValue: "perfGpu"
                delegate: WrappedLoader {
                    visible: enabled && Gpu.type !== Gpu.None
                    sourceComponent: PerfGpu {}
                }
            }
            DelegateChoice {
                roleValue: "perfBattery"
                delegate: WrappedLoader {
                    visible: enabled && UPower.displayDevice.isLaptopBattery
                    sourceComponent: PerfBattery {}
                }
            }
            DelegateChoice {
                roleValue: "github"
                delegate: WrappedLoader {
                    visible: enabled && GithubStore.available
                    sourceComponent: GithubActivity {
                        popouts: root.popouts
                    }
                }
            }
            DelegateChoice {
                roleValue: "media"
                delegate: WrappedLoader {
                    sourceComponent: Media {
                        popouts: root.popouts
                    }
                }
            }
            DelegateChoice {
                roleValue: "power"
                delegate: WrappedLoader {
                    sourceComponent: Power {
                        visibilities: root.visibilities
                    }
                }
            }
        }

    component WrappedLoader: Loader {
        required enabled
        required property string id
        required property int index

        asynchronous: false
        Layout.alignment: root.isHorizontal ? Qt.AlignVCenter : Qt.AlignHCenter









        Layout.preferredWidth: implicitWidth
        Layout.preferredHeight: implicitHeight
        Layout.maximumWidth: implicitWidth
        Layout.maximumHeight: implicitHeight
        Layout.minimumWidth: implicitWidth
        Layout.minimumHeight: implicitHeight

        visible: enabled
        active: enabled
    }
}
