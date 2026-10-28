pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Ryoku.Ui
import Ryoku.Ui.Singletons
import stage
import stage.modules.common as StageIsland
import "../desktop"
import "../desktop/Singletons" as WidgetStore
import "../visualizer/Singletons" as VizCfg
import "Singletons" as StageCfg

Item {
    id: page

    readonly property string target: StageCfg.StageSession.inspecting
    readonly property bool plugin: page.target.indexOf("plugin:") === 0
    readonly property var provider: StageIsland.Config.widgetProvider
    readonly property real scaleValue: page.plugin
        ? page.pluginValue("scale", 0.85)
        : (WidgetStore.Config.get(page.target + "Scale", StageCfg.StageSession.monitor) || 1)
    readonly property bool lockedValue: page.plugin
        ? page.pluginValue("locked", false) === true
        : WidgetStore.Config.get(page.target + "Locked", StageCfg.StageSession.monitor) === true
    readonly property string wall: page.provider
        ? page.provider.wallpaperPath : ""
    readonly property string stageMode: wall === "" ? "off"
        : StageCfg.StageBackend.effectFor(wall)
    readonly property bool frontAvailable: page.stageMode === "depth"
        || page.stageMode === "parallax"
    readonly property string frontId: page.plugin ? page.target.slice(7) : page.target
    readonly property bool inFront: StageCfg.Config.isFront(
        page.stageMode, page.frontId)

    property string scaleTarget: ""
    property real scaleBefore: 1
    property real scaleAfter: 1
    property var scalePluginPlacement: null

    property string depthTarget: ""
    property real depthBefore: 0.5
    property real depthAfter: 0.5
    property var depthPluginPlacement: null

    readonly property real depthValue: page.depthTarget === page.target
        ? page.depthAfter
        : (page.plugin ? Number(page.pluginValue("depth", 0.5))
        : WidgetStore.Config.widgetDepth(page.target, StageCfg.StageSession.monitor))
    readonly property bool depthPickable: page.provider
        && StageCfg.StageBackend.spatialFor(page.provider.wallpaperPath)


    function pluginPlacementFor(id) {
        if (String(id).indexOf("plugin:") !== 0)
            return null;
        const pluginId = String(id).slice(7);
        const entry = (WidgetStore.Registry.allPlugins || []).find(candidate => candidate.id === pluginId);
        return entry && entry.placement ? entry.placement : null;
    }

    function pluginPlacement() {
        return page.pluginPlacementFor(page.target);
    }

    function pluginValue(key, fallback) {
        const placement = page.pluginPlacement();
        const desktop = placement && placement.desktopWidget ? placement.desktopWidget : null;
        return desktop && desktop[key] !== undefined ? desktop[key] : fallback;
    }

    function applyPluginPlacement(id, placement, key, value) {
        if (!page.provider || !placement)
            return;
        const desktop = placement.desktopWidget || ({});
        const x = desktop.x !== undefined ? desktop.x : 80;
        const y = desktop.y !== undefined ? desktop.y : 80;
        const scale = key === "scale" ? value : (desktop.scale !== undefined ? desktop.scale : 0.85);
        const locked = key === "locked" ? value : desktop.locked === true;
        const opacity = desktop.opacity !== undefined ? desktop.opacity : 1;
        const depth = key === "depth" ? value
            : (desktop.depth !== undefined ? desktop.depth : "");
        page.provider.enqueue([page.provider.placeTool, id.slice(7), "desktopWidget",
            "" + x, "" + y, "" + scale, "" + locked, "" + opacity,
            depth === "" ? "" : "" + depth]);
    }

    function setQuick(key, value) {
        if (page.target === "" || !page.provider)
            return;
        if (page.plugin) {
            const placement = page.pluginPlacement();
            if (!placement)
                return;
            const before = page.pluginValue(key, key === "scale" ? 0.85 : false);
            if (before === value)
                return;
            const id = page.target;
            const snap = JSON.parse(JSON.stringify(placement));
            page.applyPluginPlacement(id, snap, key, value);
            GlobalStates.editHistoryPush({
                undo: () => page.applyPluginPlacement(id, snap, key, before),
                redo: () => page.applyPluginPlacement(id, snap, key, value)
            });
            return;
        }
        const monitor = StageCfg.StageSession.monitor;
        const configKey = page.target + key.charAt(0).toUpperCase() + key.slice(1);
        const before = WidgetStore.Config.get(configKey, monitor);
        if (before === value)
            return;
        WidgetStore.Config.setFor(monitor, configKey, value);
        GlobalStates.editHistoryPush({
            undo: () => WidgetStore.Config.setFor(monitor, configKey, before),
            redo: () => WidgetStore.Config.setFor(monitor, configKey, value)
        });
    }

    function setScale(value) {
        const clamped = Math.max(0.5, Math.min(2.5, value));
        if (page.scaleTarget !== page.target) {
            page.finishScale();
            page.scaleTarget = page.target;
            page.scaleBefore = page.scaleValue;
            page.scaleAfter = page.scaleValue;
            page.scalePluginPlacement = page.plugin
                ? JSON.parse(JSON.stringify(page.pluginPlacement())) : null;
        }
        page.scaleAfter = clamped;
        if (page.plugin) {
            if (page.scalePluginPlacement)
                page.applyPluginPlacement(page.target, page.scalePluginPlacement, "scale", clamped);
        } else {
            WidgetStore.Config.setLiveFor(StageCfg.StageSession.monitor,
                page.target + "Scale", clamped);
        }
        scaleCommit.restart();
    }

    function finishScale() {
        if (page.scaleTarget === "")
            return;
        scaleCommit.stop();
        const id = page.scaleTarget;
        const before = page.scaleBefore;
        const after = page.scaleAfter;
        const pluginSnap = page.scalePluginPlacement;
        const monitor = StageCfg.StageSession.monitor;
        page.scaleTarget = "";
        page.scalePluginPlacement = null;
        if (Math.abs(after - before) < 0.001)
            return;
        if (id.indexOf("plugin:") !== 0)
            WidgetStore.Config.setFor(monitor, id + "Scale", after);
        GlobalStates.editHistoryPush({
            undo: () => {
                if (id.indexOf("plugin:") === 0)
                    page.applyPluginPlacement(id, pluginSnap, "scale", before);
                else
                    WidgetStore.Config.setFor(monitor, id + "Scale", before);
            },
            redo: () => {
                if (id.indexOf("plugin:") === 0)
                    page.applyPluginPlacement(id, pluginSnap, "scale", after);
                else
                    WidgetStore.Config.setFor(monitor, id + "Scale", after);
            }
        });
    }

    function setDepth(front) {
        const id = page.frontId;
        const mode = page.stageMode;
        const before = StageCfg.Config.isFront(mode, id);
        if (before === front)
            return;
        StageCfg.Config.setFront(mode, id, front);
        GlobalStates.editHistoryPush({
            undo: () => StageCfg.Config.setFront(mode, id, before),
            redo: () => StageCfg.Config.setFront(mode, id, front)
        });
    }

    function setWallDepth(value) {
        const clamped = Math.max(0, Math.min(1, value));
        if (page.depthTarget !== page.target) {
            page.finishDepth();
            page.depthTarget = page.target;
            page.depthBefore = page.depthValue;
            page.depthAfter = page.depthValue;
            page.depthPluginPlacement = page.plugin
                ? JSON.parse(JSON.stringify(page.pluginPlacement())) : null;
        }
        page.depthAfter = clamped;
        if (page.plugin) {
            if (page.depthPluginPlacement)
                page.applyPluginPlacement(page.target, page.depthPluginPlacement,
                    "depth", clamped);
        } else {
            WidgetStore.Config.setWidgetDepth(
                StageCfg.StageSession.monitor, page.target, clamped);
        }
        depthCommit.restart();
    }

    function finishDepth() {
        if (page.depthTarget === "")
            return;
        depthCommit.stop();
        const id = page.depthTarget;
        const before = page.depthBefore;
        const after = page.depthAfter;
        const pluginSnap = page.depthPluginPlacement;
        const monitor = StageCfg.StageSession.monitor;
        page.depthTarget = "";
        page.depthPluginPlacement = null;
        if (Math.abs(after - before) < 0.001)
            return;
        if (id.indexOf("plugin:") !== 0)
            WidgetStore.Config.setWidgetDepth(monitor, id, after);
        GlobalStates.editHistoryPush({
            undo: () => {
                if (id.indexOf("plugin:") === 0)
                    page.applyPluginPlacement(id, pluginSnap, "depth", before);
                else
                    WidgetStore.Config.setWidgetDepth(monitor, id, before);
            },
            redo: () => {
                if (id.indexOf("plugin:") === 0)
                    page.applyPluginPlacement(id, pluginSnap, "depth", after);
                else
                    WidgetStore.Config.setWidgetDepth(monitor, id, after);
            }
        });
    }

    function removeTarget() {
        page.finishScale();
        const id = page.target;
        const provider = page.provider;
        if (!provider || id === "")
            return;
        const before = provider.snapshot(id);
        const back = StageCfg.StageSession.inspectingBack || "widgets";
        provider.removeWidget(id);
        if (before)
            GlobalStates.editHistoryPush({
                undo: () => provider.restore(id, before),
                redo: () => provider.restore(id, null)
            });
        StageCfg.StageSession.remove(id);
        StageCfg.StageSession.openSection(back);
    }

    function routeVisualizer() {
        if (page.target.indexOf("visualizer:") !== 0)
            return false;
        const instanceId = page.target.slice("visualizer:".length);
        if (instanceId !== "")
            VizCfg.Config.setActive(instanceId);
        StageCfg.StageSession.inspecting = "";
        StageCfg.StageSession.openSection("visualizer");
        return true;
    }

    onTargetChanged: {
        page.finishScale();
        page.finishDepth();
        WidgetStore.Config.selectMonitor(StageCfg.StageSession.monitor, true);
        page.routeVisualizer();
    }
    Component.onCompleted: {
        WidgetStore.Config.selectMonitor(StageCfg.StageSession.monitor, true);
        page.routeVisualizer();
    }
    Component.onDestruction: {
        page.finishScale();
        page.finishDepth();
        if (GlobalStates.editDrawerSection !== "widget")
            StageCfg.StageSession.inspecting = "";
    }

    Timer {
        id: scaleCommit
        interval: 180
        onTriggered: page.finishScale()
    }
    Timer {
        id: depthCommit
        interval: 180
        onTriggered: page.finishDepth()
    }

    Flickable {
        id: pageScroll
        anchors.fill: parent
        clip: true
        contentWidth: width
        contentHeight: content.implicitHeight + Tokens.s2
        boundsBehavior: Flickable.StopAtBounds

        QQC.ScrollBar.vertical: ScrollRail {
            policy: pageScroll.contentHeight > pageScroll.height
                ? QQC.ScrollBar.AlwaysOn : QQC.ScrollBar.AlwaysOff
        }
        WheelScroll {}

        Column {
            id: content
            x: Tokens.s4
            width: Math.max(0, pageScroll.width - Tokens.s4 * 2)
            spacing: Tokens.s4

            SettingCard {
                id: quickCard
                width: parent.width
                title: I18n.tr("QUICK CONTROLS")
                kana: "調整"
                collapsible: true
                summary: Math.round(page.scaleValue * 100) + "%"

                SettingRow {
                    width: parent.width
                    label: I18n.tr("Size")
                    desc: I18n.tr("Scale this widget without changing its anchor")
                    block: true
                    value: Math.round(page.scaleValue * 100) + "%"
                    Row {
                        width: parent.width
                        height: Tokens.ctlH
                        spacing: Tokens.s2
                        Slid {
                            width: Math.max(40, parent.width - percent.implicitWidth - reset.implicitWidth - parent.spacing * 2)
                            anchors.verticalCenter: parent.verticalCenter
                            from: 0.5
                            to: 2.5
                            value: page.scaleValue
                            onModified: value => page.setScale(value)
                        }
                        Text {
                            id: percent
                            anchors.verticalCenter: parent.verticalCenter
                            text: Math.round(page.scaleValue * 100) + "%"
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fBody
                        }
                        Btn {
                            id: reset
                            anchors.verticalCenter: parent.verticalCenter
                            compact: true
                            text: I18n.tr("RESET")
                            armed: Math.abs(page.scaleValue - 1) > 0.001
                            onAct: {
                                page.finishScale();
                                page.setQuick("scale", 1);
                            }
                        }
                    }
                }

                SettingRow {
                    width: parent.width
                    divider: true
                    label: I18n.tr("Lock position")
                    desc: I18n.tr("Keep this widget from moving on the desktop")
                    controlWidth: 54
                    Sw {
                        width: parent.width
                        on: page.lockedValue
                        onToggled: value => page.setQuick("locked", value)
                    }
                }

                SettingRow {
                    visible: page.frontAvailable
                    width: parent.width
                    divider: true
                    label: I18n.tr("Stack")
                    desc: I18n.tr("Layer the widget above or below the cut-outs")
                    controlWidth: 130
                    Seg {
                        width: parent.width
                        options: ["behind", "front"]
                        labels: ({ behind: I18n.tr("Behind"), front: I18n.tr("In front") })
                        current: page.inFront ? "front" : "behind"
                        onChose: key => page.setDepth(key === "front")
                    }
                }

                SettingRow {
                    visible: page.depthPickable
                    width: parent.width
                    divider: true
                    label: I18n.tr("Wall depth")
                    desc: I18n.tr("Ride the wallpaper's 3D depth with the mesh")
                    value: Math.round(page.depthValue * 100) + "%"
                    Row {
                        width: parent.width
                        height: Tokens.ctlH
                        spacing: Tokens.s2
                        Slid {
                            width: Math.max(40,
                                parent.width - pickBtn.implicitWidth - parent.spacing * 2)
                            anchors.verticalCenter: parent.verticalCenter
                            from: 0
                            to: 1
                            value: page.depthValue
                            onModified: value => page.setWallDepth(value)
                        }
                        Btn {
                            id: pickBtn
                            anchors.verticalCenter: parent.verticalCenter
                            compact: true
                            text: I18n.tr("PICK")
                            armed: StageSession.depthPick === page.target
                            onAct: StageSession.depthPick =
                                (StageSession.depthPick === page.target ? "" : page.target)
                        }
                    }
                }

                SettingRow {
                    width: parent.width
                    divider: true
                    label: I18n.tr("Remove")
                    desc: I18n.tr("Take this widget off the desktop")
                    controlWidth: 78
                    Btn {
                        width: parent.width
                        compact: true
                        text: I18n.tr("REMOVE")
                        onAct: page.removeTarget()
                    }
                }
            }

            Item {
                id: inspectorHost
                visible: !page.plugin
                width: parent.width
                height: visible ? embeddedInspector.implicitHeight : 0

                WidgetInspector {
                    id: embeddedInspector
                    width: parent.width
                    height: parent.height
                    embedded: true
                    scope: page.target
                    slot: null
                }
            }
        }
    }
}
