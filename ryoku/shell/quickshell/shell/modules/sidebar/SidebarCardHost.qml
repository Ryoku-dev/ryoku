pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.PluginKit
import "SidebarCatalog.js" as SidebarCatalog

Item {
    id: root

    required property string cardId
    property var pluginEntry: null
    property real s: 1
    property bool open: false
    property real reveal: 0
    property bool tabActive: false
    property int cardIndex: 0
    property string page: ""

    signal requestClose()

    readonly property var catalogEntry: SidebarCatalog.byId(root.cardId)
    readonly property bool pluginCard: root.pluginEntry !== null
    readonly property string versionQuery: pluginCard && pluginEntry.version
        ? "?v=" + encodeURIComponent(pluginEntry.version) : ""
    readonly property string stateHome: Quickshell.env("XDG_STATE_HOME")
        || (Quickshell.env("HOME") + "/.local/state")
    readonly property string shellDir: Quickshell.env("RYOKU_SHELL_DIR")
    readonly property string placeTool: shellDir !== ""
        ? shellDir + "/quickshell/plugins/ryoku-plugins-place" : "ryoku-plugins-place"

    implicitHeight: builtinLoader.active && builtinLoader.item ? builtinLoader.item.implicitHeight
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
    }

    function loadBuiltin() {
        builtinLoader.source = "";
        if (!root.catalogEntry || root.pluginCard)
            return;
        builtinLoader.setSource(Qt.resolvedUrl(root.catalogEntry.source), {
            s: root.s,
            open: root.open,
            reveal: root.reveal,
            tabActive: root.tabActive,
            width: root.width
        });
    }

    Component.onCompleted: loadBuiltin()

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
        readonly property var pluginSettings: root.pluginEntry && root.pluginEntry.placement
            && root.pluginEntry.placement.settings ? root.pluginEntry.placement.settings : ({})
        readonly property string pluginDir: root.pluginEntry ? root.pluginEntry.dir : ""
        readonly property string stateDir: root.stateHome + "/ryoku/plugins/"
            + (root.pluginEntry ? root.pluginEntry.id : "")
        function saveSetting(key, value) {
            if (!root.pluginEntry)
                return;
            var object = {};
            object[String(key)] = value;
            settingWrite.command = [root.placeTool, root.pluginEntry.id, "settings", JSON.stringify(object)];
            settingWrite.running = true;
        }
        function saveSettings() {}
    }

    PluginObjectSlot {
        id: pluginService
        source: root.pluginCard
            ? "file://" + root.pluginEntry.dir + "/service/Main.qml" + root.versionQuery : ""
        configure: function(service) { service.pluginApi = root.pluginApi; }
    }

    PluginObjectSlot {
        id: pluginContent
        anchors { left: parent.left; right: parent.right; top: parent.top }
        width: root.width
        height: item ? item.implicitHeight : 0
        source: root.pluginCard
            ? "file://" + root.pluginEntry.dir + "/content/Widget.qml" + root.versionQuery : ""
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
