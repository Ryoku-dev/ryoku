pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.PluginKit
import Lithic
import shell.services
import "SidebarCatalog.js" as SidebarCatalog

Item {
    id: root

    required property string cardId
    property var pluginEntry: null
    required property real s
    property bool open: false
    property real reveal: 0
    property bool tabActive: false
    property int cardIndex: 0
    property string page: ""
    property bool compact: false
    property real viewportHeight: 0
    property bool initialized: false

    signal requestClose()
    function normalizedEntry(entry) {
        if (!entry)
            return null;
        const result = {};
        for (const key in entry)
            result[key] = entry[key];
        const placement = {};
        const sourcePlacement = entry.placement || {};
        for (const placementKey in sourcePlacement)
            placement[placementKey] = sourcePlacement[placementKey];
        const sidebarCard = {};
        const sourceCard = sourcePlacement.sidebarCard || {};
        for (const cardKey in sourceCard)
            sidebarCard[cardKey] = sourceCard[cardKey];
        sidebarCard.side = "left";
        placement.sidebarCard = sidebarCard;
        result.placement = placement;
        return result;
    }

    readonly property var effectivePluginEntry: root.normalizedEntry(root.pluginEntry)

    readonly property var catalogEntry: SidebarCatalog.byId(root.cardId)
    readonly property bool pluginCard: root.pluginEntry !== null
    readonly property string builtinSource: root.catalogEntry ? root.catalogEntry.source : ""
    readonly property var pluginManifest: root.pluginCard && root.effectivePluginEntry.manifest
        ? root.effectivePluginEntry.manifest : ({})
    readonly property string pluginName: {
        var name = typeof root.pluginManifest.name === "string"
            ? root.pluginManifest.name.trim() : "";
        return name !== "" ? name : root.cardId;
    }
    readonly property string pluginDescription: typeof root.pluginManifest.description === "string"
        ? root.pluginManifest.description.trim() : ""
    readonly property bool pluginHasNativeCompact: root.pluginCard && pluginContent.item !== null
        && ("compact" in pluginContent.item)
    readonly property bool genericPluginSummary: root.pluginCard && root.compact
        && pluginContent.item !== null && !root.pluginHasNativeCompact
    readonly property string versionQuery: pluginCard && effectivePluginEntry.version
        ? "?v=" + encodeURIComponent(effectivePluginEntry.version) : ""
    readonly property string stateHome: Quickshell.env("XDG_STATE_HOME")
        || (Quickshell.env("HOME") + "/.local/state")
    readonly property string shellDir: Quickshell.env("RYOKU_SHELL_DIR")
    readonly property string placeTool: shellDir !== ""
        ? shellDir + "/quickshell/plugins/ryoku-plugins-place" : "ryoku-plugins-place"

    implicitHeight: builtinLoader.active && builtinLoader.item ? builtinLoader.item.implicitHeight
        : root.genericPluginSummary ? genericSummary.implicitHeight
        : pluginContent.item ? pluginContent.item.implicitHeight : 0
    height: implicitHeight
    visible: implicitHeight > 0

    function bindContract(item) {
        if (!item)
            return;
        item.s = Qt.binding(function() { return root.s; });
        item.open = Qt.binding(function() { return root.open; });
        item.reveal = Qt.binding(function() { return root.reveal; });
        item.tabActive = Qt.binding(function() { return root.tabActive; });
        item.width = Qt.binding(function() { return root.width; });
        if ("index" in item)
            item.index = Qt.binding(function() { return root.cardIndex; });
        if ("page" in item)
            item.page = Qt.binding(function() { return root.page; });
        if ("compact" in item)
            item.compact = Qt.binding(function() { return root.compact; });
        if ("viewportHeight" in item)
            item.viewportHeight = Qt.binding(function() { return root.viewportHeight; });
    }

    function loadBuiltin() {
        builtinLoader.source = "";
        if (!root.catalogEntry || root.pluginCard)
            return;
        builtinLoader.setSource(Qt.resolvedUrl(root.builtinSource), {
            s: root.s,
            open: root.open,
            reveal: root.reveal,
            tabActive: root.tabActive,
            width: root.width
        });
    }

    onBuiltinSourceChanged: if (root.initialized) root.loadBuiltin()
    Component.onCompleted: { root.initialized = true; root.loadBuiltin(); }

    Loader {
        id: builtinLoader
        anchors { left: parent.left; right: parent.right; top: parent.top }
        active: root.catalogEntry !== null && !root.pluginCard
        onLoaded: root.bindContract(item)
    }

    Connections {
        target: builtinLoader.item
        ignoreUnknownSignals: true
        function onRequestClose() { root.requestClose(); }
    }

    property var pluginApi: QtObject {
        readonly property var mainInstance: pluginService.item
        readonly property var pluginSettings: root.effectivePluginEntry
            && root.effectivePluginEntry.placement
            && root.effectivePluginEntry.placement.settings
            ? root.effectivePluginEntry.placement.settings : ({})
        readonly property string pluginDir: root.effectivePluginEntry
            ? root.effectivePluginEntry.dir : ""
        readonly property string stateDir: root.stateHome + "/ryoku/plugins/"
            + (root.effectivePluginEntry ? root.effectivePluginEntry.id : "")
        function saveSetting(key, value) {
            if (!root.effectivePluginEntry)
                return;
            var object = {};
            object[String(key)] = value;
            settingWrite.command = [root.placeTool, root.effectivePluginEntry.id, "settings", JSON.stringify(object)];
            settingWrite.running = true;
        }
        function saveSettings() {}
    }

    Column {
        id: genericSummary
        anchors { left: parent.left; right: parent.right; top: parent.top }
        visible: root.genericPluginSummary
        spacing: Math.round(4 * root.s)

        Text {
            width: parent.width
            text: root.pluginName
            textFormat: Text.PlainText
            color: Lithic.fg
            font.family: QsLook.sans
            font.pixelSize: Math.round(14 * root.s)
            font.weight: Font.Medium
            elide: Text.ElideRight
            renderType: Text.NativeRendering
        }

        Text {
            width: parent.width
            visible: root.pluginDescription !== ""
            text: root.pluginDescription
            textFormat: Text.PlainText
            color: Lithic.fg2
            font.family: QsLook.sans
            font.pixelSize: Math.round(12 * root.s)
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }
    }

    PluginObjectSlot {
        id: pluginService
        source: root.pluginCard
            ? "file://" + root.effectivePluginEntry.dir + "/service/Main.qml" + root.versionQuery : ""
        configure: function(service) { service.pluginApi = root.pluginApi; }
    }

    PluginObjectSlot {
        id: pluginContent
        anchors { left: parent.left; right: parent.right; top: parent.top }
        width: root.width
        height: item ? item.implicitHeight : 0
        opacity: root.genericPluginSummary ? 0 : 1
        enabled: !root.genericPluginSummary
        source: root.pluginCard
            ? "file://" + root.effectivePluginEntry.dir + "/content/Widget.qml" + root.versionQuery : ""
        configure: function(content) {
            content.pluginApi = root.pluginApi;
            root.bindContract(content);
            if ("density" in content)
                content.density = "sidebar";
            if ("widthBudget" in content)
                content.widthBudget = root.width;
            if ("active" in content)
                content.active = Qt.binding(function() { return root.tabActive && root.open; });
        }
    }

    Connections {
        target: pluginContent.item
        ignoreUnknownSignals: true
        function onRequestClose() { root.requestClose(); }
    }

    Process { id: settingWrite }
}
