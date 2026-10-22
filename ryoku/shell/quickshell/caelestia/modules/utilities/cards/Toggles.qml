pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Bluetooth
import Caelestia.Components
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils
import caelestia.modules.bar.popouts as BarPopouts

StyledRect {
    id: root

    required property DrawerVisibilities visibilities
    required property BarPopouts.Wrapper popouts

    readonly property real nonAnimHeight: implicitHeight

    readonly property var quickToggles: {
        const configToggles = Config.utilities.quickToggles || [];
        const disabledIds = new Set(configToggles.filter(t => t.enabled === false).map(t => t.id));

        const builtIn = [
            {
                id: "hotspot"
            },
            {
                id: "restartShell"
            },
            {
                id: "badapple"
            },
            {
                id: "pauseWallpaper"
            },
            {
                id: "nightlight"
            },
            {
                id: "easyeffects"
            }
        ].filter(t => !disabledIds.has(t.id));

        const allToggles = [...configToggles.filter(t => !disabledIds.has(t.id)), ...builtIn];
        if (Config.utilities.quickTogglesCustomOrder ?? false) {
            const order = {};
            configToggles.forEach((t, i) => {
                order[t.id] = i;
            });
            allToggles.sort((a, b) => (order[a.id] ?? 9999) - (order[b.id] ?? 9999));
        }
        const seenIds = new Set();

        return allToggles.filter(item => {
            if (seenIds.has(item.id))
                return false;
            seenIds.add(item.id);

            if (item.id === "vpn") {
                return GlobalConfig.utilities.vpn.selectedProvider.length > 0;
            }

            if (item.id === "hotspot") {
                return Nmcli.hotspot.supported;
            }

            if (item.id === "easyeffects") {
                return EasyEffects.available;
            }

            return true;
        });
    }

    readonly property int perPage: Math.max(2, Math.min(12, Config.utilities.quickTogglesPerPage ?? 6))
    readonly property int pageCount: Math.max(1, Math.ceil(quickToggles.length / perPage))
    property int currentPage: 0
    property bool dragging: false
    property var dragOrder: []
    property string dragId: ""
    property real ghostX: 0
    property real ghostY: 0
    property double lastEdgeSwitch: 0

    function pageSlice(): var {
        const src = root.dragging && root.dragOrder.length > 0 ? root.dragOrder : root.quickToggles;
        return src.slice(root.currentPage * root.perPage, (root.currentPage + 1) * root.perPage);
    }

    function iconFor(id: string): string {
        switch (id) {
        case "wifi": return "wifi";
        case "bluetooth": return "bluetooth";
        case "mic": return "mic";
        case "settings": return "settings";
        case "gameMode": return "gamepad";
        case "colorpicker": return "colorize";
        case "wallpaper": return "wallpaper";
        case "restartShell": return "restart_alt";
        case "pauseWallpaper": return "pause";
        case "easyeffects": return "graphic_eq";
        case "nightlight": return "bedtime";
        case "hotspot": return "wifi_tethering";
        case "dnd": return "notifications_off";
        case "vpn": return "vpn_key";
        case "badapple": return "nutrition";
        default: return "toggle_on";
        }
    }

    function toggleAt(row, px: real, py: real): var {
        const kids = row.children;
        for (let i = 0; i < kids.length; i++) {
            const c = kids[i];
            if (!c.visible || c.width <= 0 || c.modelData === undefined)
                continue;
            const lp = c.mapFromItem(root, px, py);
            if (lp.x >= 0 && lp.y >= 0 && lp.x <= c.width && lp.y <= c.height)
                return c;
        }
        return null;
    }

    function rowModels(top: bool): var {
        const slice = root.pageSlice();
        const split = Math.ceil(slice.length / 2);
        return top ? slice.slice(0, split) : slice.slice(split);
    }

    function requestPage(page: int): void {
        const clamped = Math.max(0, Math.min(root.pageCount - 1, page));
        if (clamped !== root.currentPage)
            root.currentPage = clamped;
    }

    // These guards also run during component init, when pageCount can still hold
    // its default 0; without the lower clamp currentPage ends up -1, every page
    // slice comes out empty and the card renders collapsed until a wheel event
    // happens to call requestPage.
    function clampPage(): void {
        const clamped = Math.max(0, Math.min(root.pageCount - 1, root.currentPage));
        if (clamped !== root.currentPage)
            root.currentPage = clamped;
    }

    function startDrag(item, px: real, py: real): void {
        root.dragOrder = root.quickToggles.map(t => ({
                    id: t.id
                }));
        root.dragId = item.modelData.id;
        root.dragging = true;
        root.ghostX = px;
        root.ghostY = py;
        const hit = root.toggleAt(rowTop, px, py) || root.toggleAt(rowBottom, px, py);
        if (hit)
            hit.opacity = 0.35;
    }

    function moveDrag(px: real, py: real): void {
        root.ghostX = px;
        root.ghostY = py;
        // Drag to edge switches page (with cooldown so it doesn't flip rapidly).
        const now = Date.now();
        if (now - root.lastEdgeSwitch > 450) {
            if (px < 24 && root.currentPage > 0) {
                root.lastEdgeSwitch = now;
                root.requestPage(root.currentPage - 1);
                return;
            }
            if (px > root.width - 24 && root.currentPage < root.pageCount - 1) {
                root.lastEdgeSwitch = now;
                root.requestPage(root.currentPage + 1);
                return;
            }
        }
        const rows = [rowTop, rowBottom];
        const slice = root.pageSlice();
        const split = Math.ceil(slice.length / 2);
        const pageStart = root.currentPage * root.perPage;
        for (let r = 0; r < rows.length; r++) {
            const row = rows[r];
            if (!row.visible || row.model.length <= 0)
                continue;
            if (px >= row.x - 20 && px <= row.x + row.width + 20 && py >= row.y - row.height / 2 && py <= row.y + row.height * 1.5) {
                const base = pageStart + (row === rowBottom ? split : 0);
                let slot = Math.floor((px - row.x) / (row.width / row.model.length));
                if (slot < 0)
                    slot = 0;
                if (slot > row.model.length - 1)
                    slot = row.model.length - 1;
                const ids = root.dragOrder.slice();
                let from = -1;
                for (let i = 0; i < ids.length; i++)
                    if (ids[i].id === root.dragId)
                        from = i;
                const to = base + slot;
                if (from >= 0 && to >= 0 && from !== to) {
                    const moved = ids.splice(from, 1)[0];
                    ids.splice(to > ids.length ? ids.length : to, 0, moved);
                    root.dragOrder = ids;
                }
                return;
            }
        }
    }

    function finishDrag(save: bool): void {
        if (save && root.dragId) {
            const stored = Config.utilities.quickToggles || [];
            const flags = {};
            stored.forEach(t => {
                flags[t.id] = t.enabled !== false;
            });
            const ids = root.dragOrder.slice();
            Object.keys(flags).forEach(id => {
                let found = false;
                for (let i = 0; i < ids.length; i++)
                    if (ids[i].id === id)
                        found = true;
                if (!found)
                    ids.push({
                        id: id
                    });
            });
            GlobalConfig.utilities.quickToggles = ids.map(item => ({
                        id: item.id,
                        enabled: flags[item.id] !== false
                    }));
            GlobalConfig.utilities.quickTogglesCustomOrder = true;
        }
        const rows = [rowTop, rowBottom];
        for (let r = 0; r < rows.length; r++) {
            const kids = rows[r].children;
            for (let i = 0; i < kids.length; i++)
                if (kids[i].modelData !== undefined)
                    kids[i].opacity = 1;
        }
        root.dragging = false;
        root.dragId = "";
        root.dragOrder = [];
    }

    onPageCountChanged: root.clampPage()
    onPerPageChanged: root.clampPage()

    Layout.fillWidth: true
    implicitHeight: layout.implicitHeight + Tokens.padding.extraLargeIncreased

    radius: Tokens.rounding.large
    color: Colours.tPalette.m3surfaceContainer

    Timer {
        id: execTimer

        interval: 250
        repeat: false

        property var pendingAction: null
        onTriggered: {
            if (pendingAction) pendingAction();
            pendingAction = null;
        }
    }

    ColumnLayout {
        id: layout

        anchors.fill: parent
        anchors.margins: Tokens.padding.large
        spacing: Tokens.spacing.medium

        RowLayout {
            StyledText {
                Layout.fillWidth: true
                text: qsTr("Quick Toggles")
                font: Tokens.font.body.medium
            }

            IconButton {
                icon: "tune"
                type: IconButton.Text
                onClicked: {
                    root.visibilities.utilities = false;
                    execTimer.pendingAction = () => Hub.open("shellpanels");
                    execTimer.restart();
                }
            }
        }

        QuickToggleRow {
            id: rowTop

            model: root.rowModels(true)
        }

        QuickToggleRow {
            id: rowBottom

            visible: model.length > 0
            model: root.rowModels(false)
        }

        Row {
            id: pageDots

            visible: root.pageCount > 1
            Layout.alignment: Qt.AlignHCenter
            spacing: Tokens.spacing.extraSmall

            Repeater {
                model: root.pageCount

                delegate: StyledRect {
                    required property int index

                    implicitWidth: index === root.currentPage ? 16 : 6
                    implicitHeight: 6
                    radius: Tokens.rounding.full
                    color: index === root.currentPage ? Colours.palette.m3primary : Colours.palette.m3outlineVariant

                    Behavior on implicitWidth {
                        Anim {
                            type: Anim.Spatial
                        }
                    }

                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.requestPage(parent.index)
                    }
                }
            }
        }
    }

    WheelHandler {
        acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
        onWheel: event => {
            if (root.pageCount <= 1 || root.dragging)
                return;
            const d = event.angleDelta.y !== 0 ? event.angleDelta.y : event.angleDelta.x;
            if (d < 0)
                root.requestPage(root.currentPage + 1);
            else if (d > 0)
                root.requestPage(root.currentPage - 1);
        }
    }


    MouseArea {
        id: rowOverlay

        property bool pressed: false
        property real pressX: 0
        property real pressY: 0
        property var pressHit: null

        x: layout.x
        y: layout.y + rowTop.y
        width: layout.width
        height: (rowBottom.visible ? rowBottom.y + rowBottom.height : rowTop.y + rowTop.height) - rowTop.y
        onPressed: mouse => {
            const p = rowOverlay.mapToItem(root, mouse.x, mouse.y);
            pressX = p.x;
            pressY = p.y;
            pressed = true;
            pressHit = root.toggleAt(rowTop, p.x, p.y) || root.toggleAt(rowBottom, p.x, p.y);
        }
        onPositionChanged: mouse => {
            if (!pressed)
                return;
            const p = rowOverlay.mapToItem(root, mouse.x, mouse.y);
            if (!root.dragging) {
                if (Math.hypot(p.x - pressX, p.y - pressY) > 10 && pressHit)
                    root.startDrag(pressHit, p.x, p.y);
            } else {
                root.moveDrag(p.x, p.y);
            }
        }
        onReleased: {
            if (root.dragging) {
                root.finishDrag(true);
            } else if (pressHit) {
                pressHit.clicked();
            }
            pressed = false;
            pressHit = null;
        }
        onCanceled: {
            if (root.dragging)
                root.finishDrag(false);
            pressed = false;
            pressHit = null;
        }
    }

    StyledRect {
        visible: root.dragging
        x: root.ghostX - 26
        y: root.ghostY - 26
        implicitWidth: 52
        implicitHeight: 52
        radius: Tokens.rounding.large
        color: Colours.palette.m3secondaryContainer
        opacity: 0.9

        MaterialIcon {
            anchors.centerIn: parent
            text: root.iconFor(root.dragId)
            color: Colours.palette.m3onSecondaryContainer
            fontStyle: Tokens.font.icon.large
        }
    }

    component QuickToggleRow: ButtonRow {
        property alias model: repeater.model

        Layout.fillWidth: true
        spacing: Tokens.spacing.small

        Repeater {
            id: repeater

            delegate: DelegateChooser {
                role: "id"

                DelegateChoice {
                    roleValue: "wifi"
                    delegate: Toggle {
                        icon: "wifi"
                        checked: Nmcli.wifiEnabled
                        onClicked: Nmcli.toggleWifi()
                    }
                }
                DelegateChoice {
                    roleValue: "hotspot"
                    delegate: Toggle {
                        icon: "wifi_tethering"
                        checked: Nmcli.hotspot.enabled
                        onClicked: HotspotSwitch.toggle()
                    }
                }
                DelegateChoice {
                    roleValue: "bluetooth"
                    delegate: Toggle {
                        icon: "bluetooth"
                        checked: Bluetooth.defaultAdapter?.enabled ?? false // qmllint disable unresolved-type
                        onClicked: {
                            const adapter = Bluetooth.defaultAdapter; // qmllint disable unresolved-type
                            if (adapter)
                                adapter.enabled = !adapter.enabled;
                        }
                    }
                }
                DelegateChoice {
                    roleValue: "mic"
                    delegate: Toggle {
                        icon: "mic"
                        checked: !Audio.sourceMuted
                        onClicked: {
                            const audio = Audio.source?.audio;
                            if (audio)
                                audio.muted = !audio.muted;
                        }
                    }
                }
                DelegateChoice {
                    roleValue: "settings"
                    delegate: Toggle {
                        icon: "settings"
                        inactiveOnColour: Colours.palette.m3onSurfaceVariant
                        isToggle: false
                        onClicked: {
                            root.visibilities.utilities = false;
                            execTimer.pendingAction = () => Hub.open("shellstyle");
                            execTimer.restart();
                        }
                    }
                }
                DelegateChoice {
                    roleValue: "colorpicker"
                    delegate: Toggle {
                        icon: "colorize"
                        inactiveOnColour: Colours.palette.m3onSurfaceVariant
                        isToggle: false
                        onClicked: {
                            root.visibilities.utilities = false;
                            execTimer.pendingAction = () => ColorPicker.pickColor();
                            execTimer.restart();
                        }
                    }
                }
                DelegateChoice {
                    roleValue: "gameMode"
                    delegate: Toggle {
                        icon: "gamepad"
                        checked: GameMode.enabled
                        onClicked: GameMode.enabled = !GameMode.enabled
                    }
                }
                DelegateChoice {
                    roleValue: "dnd"
                    delegate: Toggle {
                        icon: "notifications_off"
                        checked: Notifs.dnd
                        onClicked: Notifs.dnd = !Notifs.dnd
                    }
                }
                DelegateChoice {
                    roleValue: "vpn"
                    delegate: Toggle {
                        icon: "vpn_key"
                        checked: VPN.connected && VPN.status.state !== "needs-auth" && VPN.status.state !== "error"
                        enabled: !VPN.connecting && !VPN.disconnecting
                        isToggle: VPN.status.state !== "needs-auth" && VPN.status.state !== "error"
                        inactiveOnColour: Colours.palette.m3onSurfaceVariant
                        onClicked: VPN.toggle()
                    }
                }
                DelegateChoice {
                    roleValue: "wallpaper"
                    delegate: Toggle {
                        icon: "wallpaper"
                        isToggle: false
                        inactiveOnColour: Colours.palette.m3onSurfaceVariant
                        onClicked: {
                            Visibilities.launcherInitialSearch = `${GlobalConfig.launcher.actionPrefix}wallpaper `;
                            const visibilities = Visibilities.getForActive();
                            visibilities.launcher = true;
                        }
                    }
                }
                DelegateChoice {
                    roleValue: "restartShell"
                    delegate: Toggle {
                        icon: "restart_alt"
                        isToggle: false
                        inactiveOnColour: Colours.palette.m3onSurfaceVariant
                        onClicked: {
                            Launch.exec(["bash", "-c", `bash "${Quickshell.shellPath("scripts/restart_shell.sh")}"`]);
                        }
                    }
                }
                DelegateChoice {
                    roleValue: "pauseWallpaper"
                    delegate: Toggle {
                        id: pauseWallpaperToggle

                        icon: "pause"
                        isToggle: true

                        Component.onCompleted: checked = Qt.binding(() => GlobalConfig.background.videoWallpaperPaused)
                        onClicked: {
                            const newVal = !GlobalConfig.background.videoWallpaperPaused;
                            GlobalConfig.background.videoWallpaperPaused = newVal;
                        }
                    }
                }
                DelegateChoice {
                    roleValue: "easyeffects"
                    delegate: Toggle {
                        checked: EasyEffects.active
                        icon: "graphic_eq"
                        onClicked: EasyEffects.toggle()

                        // Right-click opens the application itself. Toggling the
                        // service on is only half of what people want from
                        // EasyEffects -- the other half is changing what it does,
                        // and that lives in its own window.
                        //
                        // Only the right button is accepted here, so the left one
                        // falls through to the button underneath and keeps working
                        // as the toggle. Adding a second signal to ButtonBase would
                        // have reached every button in the shell for the sake of
                        // one.
                        MouseArea {
                            acceptedButtons: Qt.RightButton
                            anchors.fill: parent
                            onClicked: {
                                EasyEffects.open();
                                root.visibilities.utilities = false;
                            }
                        }
                    }
                }
                DelegateChoice {
                    roleValue: "nightlight"
                    delegate: Toggle {
                        icon: "bedtime"
                        checked: HyprSunset.active
                        onClicked: {
                            HyprSunset.toggleNightLight();
                        }
                    }
                }
            }
        }
    }

    component Toggle: IconButton {
        required property var modelData

        inactiveColour: Colours.layer(Colours.palette.m3surfaceContainerHighest, 2)
        fillWidth: true
        isToggle: true
        isRound: true
        shapeMorph: true

        Behavior on x {
            Anim {
                type: Anim.SlowSpatial
            }
        }

        Behavior on y {
            Anim {
                type: Anim.SlowSpatial
            }
        }
    }
}
