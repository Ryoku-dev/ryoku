pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import "Singletons"
import Ryoku.Ui.Singletons
import shell.services as Services
import "ContributedWidgets.js" as ContributedWidgets
import "ii/IiRoster.js" as IiRoster
import "ii/IiQuickControls.js" as IiQuickControls
import "../stage/Singletons" as StageCfg
import "../visualizer/Singletons" as VizCfg
import stage.modules.common.functions as StageFunctions
import stage.modules.common as StageCommon

// A desktop widget's right-click menu, built on the shared DesktopMenu chrome in
// the quick-settings sidebar idiom: a short card that names the widget, offers
// its quick knobs -- Style, Size, Lock -- then a primary Customize row that opens
// the widget inspector for everything else (colour, shape, snap and the widget's
// own options), Hide, and the shell globals (Settings, Reload). The full control
// set moved to WidgetInspector so this menu stays short; both write the same
// widgets Config the drag and Ryoku Settings do. The bare-wallpaper right-click
// is a separate surface, DesktopContextMenu.
Item {
    id: menu

    anchors.fill: parent

    // Exposed so the host menu surface maps only while this menu is on screen.
    readonly property bool showing: shell.showing

    // Raised when Customize is chosen; the desktop layer opens the inspector.
    signal customizeRequested(string widget)

    property string scope: "desktop"   // desktop | clock | ...
    property string monitor: ""
    // The wallpaper of the monitor whose right-click opened this menu; the
    // Depth row gates on it, so the lift is offered only where the scene has
    // cut-outs. Set by the owning desktop with openFor.
    property string wall: ""

    readonly property bool isWidget: menu.scope !== "desktop"
    // The visualiser is a framed widget like the rest, but its store is its
    // own: no per-widget lock, no scale (the grip sizes its box), and a look
    // catalogue walked through the visualiser config rather than a design ladder.
    readonly property bool isVisualizer: menu.scope === "visualizer"
    readonly property bool isStats: menu.scope === "stats"
    readonly property bool isNotes: menu.scope === "notes"
    readonly property bool isCalendar: menu.scope === "calendar"
    readonly property bool isMusic: menu.scope === "music"
    readonly property bool isAio: menu.scope === "aio"
    readonly property bool isDayprogress: menu.scope === "dayprogress"
    readonly property bool isShape: menu.scope === "shape"
    readonly property bool locked: menu.isWidget && !menu.isVisualizer
        ? menu.configValue(menu.scope + "Locked") : false
    readonly property string stageMode: menu.wall === "" ? "off"
        : StageCfg.StageBackend.effectFor(menu.wall)
    readonly property bool stageActive: menu.stageMode === "depth"
        || menu.stageMode === "parallax"
    readonly property bool lifted: StageCfg.Config.isFront(
        menu.stageMode, menu.scope)

    // clock faces persist as <scope>Design; the calendar and the music sheet
    // persist their look as <scope>Style; shape as <scope>Kind.
    readonly property string designKey: menu.isCalendar || menu.isMusic || menu.isAio || menu.isDayprogress
        ? menu.scope + "Style"
        : menu.isShape ? menu.scope + "Kind"
        : menu.scope + "Design"
    readonly property string curDesign: menu.isWidget ? (menu.configValue(menu.designKey) ?? "") : ""

    readonly property var contributedMatch: ContributedWidgets.find(
        Services.Shells.contributions("desktopWidgets"), menu.scope)
    readonly property bool isContributed: menu.contributedMatch !== null
    readonly property var contributedSet: menu.isContributed
        ? menu.contributedMatch.set : null
    readonly property var contributedRecord: menu.isContributed
        ? menu.contributedMatch.record : null
    readonly property var iiFace: IiRoster.byPrefix(menu.scope)
    readonly property bool isIi: menu.iiFace !== null
    readonly property var hostedFace: menu.isContributed
        ? menu.contributedRecord : menu.iiFace
    readonly property bool isHosted: menu.isContributed || menu.isIi
    readonly property bool isRyokuStyle: menu.isContributed
        && menu.configValue(menu.scope + "Style") === "ryoku"
    readonly property var facePresets: menu.isContributed
        && Array.isArray(menu.contributedRecord.sizes)
        ? menu.contributedRecord.sizes : []
    readonly property string facePresetKey: menu.scope + "Size"
    readonly property string curFacePreset: menu.facePresets.length > 0
        ? (menu.configValue(menu.facePresetKey) || menu.facePresets[0]) : ""

    // the kanji seal each scope carries in the menu masthead (docs/ui-ux.md).
    readonly property var glosses: ({
        "desktop": "卓上",
        "clock": "時計",
        "calendar": "暦",
        "music": "音楽",
        "aio": "一体",
        "stats": "計測",
        "weather": "天気",
        "notes": "筆記",
        "dayprogress": "経過",
        "shape": "図形"
    })

    // Human names for the built-in scopes; the masthead never shows a raw key.
    readonly property var builtinLabels: ({
        clock: "Clock", calendar: "Calendar", music: "Music", aio: "All-in-one",
        stats: "System stats", weather: "Weather", notes: "Notes",
        dayprogress: "Day Progress", shape: "Shape"
    })

    // the widgets that cycle through a set of looks from the quick Style row; the
    // rest (stats, notes) have a single look and skip the row.
    readonly property var designLists: ({
        clock: ["digital", "minimal", "grand", "column", "outline", "banner", "analog", "flip", "rings", "bighour", "metal", "goodnight"],
        calendar: ["glass", "paper"],
        music: ["cover", "glass"],
        aio: ["wide", "tall"],
        weather: ["compact", "full"],
        dayprogress: ["ring", "arc"],
        shape: ["dot", "ring", "diamond", "square"]
    })
    readonly property bool hasDesign: menu.isWidget && !menu.isContributed && !menu.isIi
        && (menu.designLists[menu.scope] !== undefined)

    // The source menu and resize grip share the same named detents.
    readonly property real curScale: menu.isWidget
        ? (menu.configValue(menu.scope + "Scale") || 1) : 1
    readonly property real scaleStep: StageFunctions.EditModeLogic.nearestSizeStep(menu.curScale)
    readonly property bool canGrow: StageFunctions.EditModeLogic.steppedScale(menu.curScale, 1) !== null
    readonly property bool canShrink: StageFunctions.EditModeLogic.steppedScale(menu.curScale, -1) !== null
    readonly property var iiControls: menu.isIi
        ? IiQuickControls.forWidget(menu.iiFace.id) : []

    function iiControlStore() {
        if (!menu.isIi)
            return null;
        let store = StageCommon.Config.options;
        const parts = String(IiQuickControls.storeFor(menu.iiFace.id)).split(".");
        for (let i = 0; i < parts.length && store !== undefined && store !== null; i++)
            store = store[parts[i]];
        return store;
    }
    function iiControlValue(control) {
        if (!control)
            return undefined;
        let value = menu.iiControlStore();
        const parts = String(control.key).split(".");
        for (let i = 0; i < parts.length && value !== undefined && value !== null; i++)
            value = value[parts[i]];
        return value === undefined || value === null ? control.def : value;
    }
    function iiControlChoiceLabel(control, value) {
        const choices = control?.choices ?? [];
        for (let i = 0; i < choices.length; i++)
            if (choices[i].value === value)
                return I18n.tr(choices[i].label);
        return choices.length > 0 ? I18n.tr(choices[0].label) : "";
    }
    function setIiControlValue(control, value) {
        if (!StageCommon.Config.ready || !control)
            return;
        let target = menu.iiControlStore();
        const parts = String(control.key).split(".");
        for (let i = 0; i < parts.length - 1; i++) {
            if (target === undefined || target === null)
                return;
            target = target[parts[i]];
        }
        if (target !== undefined && target !== null)
            target[parts[parts.length - 1]] = value;
    }
    function triggerIiControl(control) {
        const current = menu.iiControlValue(control);
        if (control.type === "toggle") {
            menu.setIiControlValue(control, current !== true);
            return;
        }
        const choices = control.choices ?? [];
        if (choices.length === 0)
            return;
        let index = -1;
        for (let i = 0; i < choices.length; i++) {
            if (choices[i].value === current) {
                index = i;
                break;
            }
        }
        menu.setIiControlValue(control, choices[(index + 1) % choices.length].value);
    }

    function openFor(widget, x, y, wall) { menu.scope = widget; menu.wall = wall; shell.px = x; shell.py = y; shell.open = true; }
    function close() { shell.open = false; }
    function cap(s) { return s.length > 0 ? s.charAt(0).toUpperCase() + s.slice(1) : s; }
    function configValue(key) { return Config.get(key, menu.monitor); }

    function cycleDesign() {
        const d = menu.designLists[menu.scope];
        if (!d)
            return;
        Config.set(menu.designKey, d[(d.indexOf(menu.curDesign) + 1) % d.length]);
    }
    function cycleFacePreset() {
        const presets = menu.facePresets;
        if (!presets || presets.length < 2)
            return;
        const current = presets.indexOf(menu.curFacePreset);
        Config.set(menu.facePresetKey, presets[(current + 1) % presets.length]);
    }
    function stepScale(direction) {
        const next = StageFunctions.EditModeLogic.steppedScale(menu.curScale, direction);
        if (next !== null)
            Config.setFor(menu.monitor, menu.scope + "Scale", next);
    }
    function resetScale() {
        Config.setFor(menu.monitor, menu.scope + "Scale", 1);
    }
    function openSettings() {
        Spawn.run(["ryoku-shell", "hub", "open"]);
        menu.close();
    }
    function refreshShell() {
        Quickshell.execDetached(["ryoku-shell", "reload"]);
        menu.close();
    }

    component ScaleButton: Rectangle {
        id: scaleButton
        property string symbol: ""
        property bool available: true
        signal activated()
        width: 26
        height: 26
        radius: 6
        color: scaleMouse.pressed ? Theme.tilePress
            : scaleMouse.containsMouse ? Theme.tileHover : "transparent"
        border.width: 1
        border.color: scaleButton.available ? Theme.lineStrong : Theme.line
        opacity: scaleButton.available ? 1 : 0.38
        Text {
            anchors.centerIn: parent
            text: scaleButton.symbol
            color: Theme.ink
            font.family: Theme.font
            font.pixelSize: 17
            font.weight: Font.DemiBold
        }
        MouseArea {
            id: scaleMouse
            anchors.fill: parent
            enabled: scaleButton.available
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onClicked: scaleButton.activated()
        }
    }

    DesktopMenu {
        id: shell
        title: menu.isHosted ? I18n.tr(menu.hostedFace.label)
            : I18n.tr(menu.builtinLabels[menu.scope] || menu.cap(menu.scope))
        gloss: menu.glosses[menu.scope] || (menu.isHosted ? menu.hostedFace.gloss : "")

        // ── quick knobs ────────────────────────────────────────────────
        // Native widgets cycle their look; hosted faces flip between their
        // upstream skin and Ryoku's backing. Everything finer opens Customize.
        MenuRow {
            visible: menu.hasDesign
            label: I18n.tr("Style")
            value: menu.cap(menu.curDesign)
            closeOnTrigger: false
            onTriggered: menu.cycleDesign()
        }
        MenuRow {
            visible: menu.isContributed
            label: I18n.tr("Style")
            value: menu.isRyokuStyle ? "Ryoku" : "Original"
            on: menu.isRyokuStyle
            closeOnTrigger: false
            onTriggered: Config.set(menu.scope + "Style",
                menu.isRyokuStyle ? menu.contributedSet.id : "ryoku")
        }
        MenuRow {
            visible: menu.isContributed && menu.facePresets.length > 1
            label: I18n.tr("Preset")
            value: menu.cap(menu.curFacePreset)
            closeOnTrigger: false
            onTriggered: menu.cycleFacePreset()
        }
        MenuRow {
            visible: menu.isVisualizer
            label: I18n.tr("Style")
            value: menu.cap(VizCfg.Config.styleId)
            closeOnTrigger: false
            onTriggered: VizCfg.Config.cycleStyle(1)
        }
        Repeater {
            model: menu.iiControls
            delegate: MenuRow {
                id: iiQuickRow
                required property var modelData
                readonly property var controlValue: menu.iiControlValue(iiQuickRow.modelData)
                label: I18n.tr(iiQuickRow.modelData.label)
                value: iiQuickRow.modelData.type === "toggle"
                    ? I18n.tr(iiQuickRow.controlValue === true ? "On" : "Off")
                    : menu.iiControlChoiceLabel(iiQuickRow.modelData, iiQuickRow.controlValue)
                on: iiQuickRow.modelData.type === "toggle"
                    && iiQuickRow.controlValue === true
                closeOnTrigger: false
                onTriggered: menu.triggerIiControl(iiQuickRow.modelData)
            }
        }
        Item {
            visible: menu.isWidget && !menu.isVisualizer
            width: parent ? parent.width : 0
            implicitHeight: 34

            Rectangle {
                anchors.fill: parent
                radius: 6
                color: "transparent"
                border.width: 1
                border.color: Theme.line
            }
            MouseArea {
                anchors.fill: parent
            }
            Text {
                anchors.left: parent.left
                anchors.leftMargin: 9
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Size")
                color: Theme.inkSoft
                font.family: Theme.font
                font.pixelSize: 13
                font.weight: Font.DemiBold
            }
            Row {
                anchors.right: parent.right
                anchors.rightMargin: 4
                anchors.verticalCenter: parent.verticalCenter
                spacing: 5
                ScaleButton {
                    symbol: "−"
                    available: menu.canShrink
                    onActivated: menu.stepScale(-1)
                }
                Text {
                    width: 42
                    height: 26
                    verticalAlignment: Text.AlignVCenter
                    horizontalAlignment: Text.AlignHCenter
                    text: Math.round(menu.scaleStep * 100) + "%"
                    color: Theme.inkDim
                    font.family: Theme.mono
                    font.pixelSize: 11
                    font.weight: Font.Medium
                }
                ScaleButton {
                    symbol: "+"
                    available: menu.canGrow
                    onActivated: menu.stepScale(1)
                }
            }
        }
        MenuRow {
            visible: menu.isWidget && !menu.isVisualizer
                && Math.abs(menu.curScale - 1) > 0.001
            label: I18n.tr("Reset size")
            icon: "fit_screen"
            closeOnTrigger: false
            onTriggered: menu.resetScale()
        }
        MenuRow {
            visible: menu.isWidget && !menu.isVisualizer
            label: I18n.tr("Lock")
            value: menu.locked ? "On" : "Off"
            on: menu.locked
            closeOnTrigger: false
            onTriggered: Config.toggle(menu.scope + "Locked")
        }
        MenuRow {
            visible: menu.isWidget && menu.stageActive
            label: I18n.tr("Depth")
            value: menu.lifted ? I18n.tr("In front") : I18n.tr("Behind")
            on: menu.lifted
            closeOnTrigger: false
            onTriggered: StageCfg.Config.setFront(
                menu.stageMode, menu.scope, !menu.lifted)
        }

        // ── customize ──────────────────────────────────────────────────
        // The primary action: the widget inspector docks beside the widget with
        // its colour, shape, placement and own option groups on tabs.
        MenuRow {
            visible: menu.isWidget
            label: I18n.tr("Customize…")
            icon: "tune"
            accent: true
            onTriggered: menu.customizeRequested(menu.scope)
        }
        MenuRow {
            visible: menu.isWidget
            label: I18n.tr("Hide")
            onTriggered: menu.isVisualizer ? VizCfg.Config.setEnabled(false)
                : Config.set(menu.scope + "Enabled", false)
        }

        // ── globals ────────────────────────────────────────────────────
        MenuSection {}
        MenuRow { label: I18n.tr("Settings"); closeOnTrigger: false; onTriggered: menu.openSettings() }
        MenuRow { label: I18n.tr("Reload shell"); closeOnTrigger: false; onTriggered: menu.refreshShell() }
    }
}
