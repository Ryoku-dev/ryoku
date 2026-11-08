pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Widgets
import Ryoku.Bus
import "../../shared/Singletons"
import Ryoku.Ui.Singletons
import shell.services as Svc

Item {
    id: root

    property real s: 1
    property bool open: false
    property bool animationsEnabled: true
    property var entry: null
    property bool focusActive: false
    property string focusedAddress: ""
    property int focusSerial: 0
    property string observedEntryKey: ""

    signal focusRequested(string address)

    readonly property var windows: {
        void Wm.windows;
        var result = root.entry;
        if (!result || !root.open || String(result.type || "") !== "App")
            return [];

        var wantedAddress = String(result.windowAddress || "");
        var wantedId = String(result.appId || result.id || "")
            .toLowerCase().replace(/\.desktop$/, "");
        var wantedTitle = String(result.title || "").toLowerCase();
        var wantedTokens = wantedTitle.split(/[^a-z0-9]+/).filter(
            token => token.length > 2);
        var values = Wm.windows || [];
        var out = [];
        for (var index = 0; index < values.length; index++) {
            var w = values[index];
            if (!w || !w.id)
                continue;
            var address = String(w.id);
            var cls = String(w.appId || "");
            var lowerClass = cls.toLowerCase();
            var lowerTitle = String(w.title || "").toLowerCase();
            var exactWindow = wantedAddress.length > 0
                && address === wantedAddress;
            var exactId = wantedId.length > 0
                && (lowerClass === wantedId
                    || lowerClass.split(".").pop() === wantedId);
            var titleMatch = wantedTokens.some(token =>
                lowerClass.indexOf(token) >= 0 || lowerTitle.indexOf(token) >= 0);
            if (exactWindow || exactId || titleMatch) {
                out.push({
                    address: address,
                    title: String(w.title || cls || I18n.tr("OPEN WINDOW")),
                    cls: cls,
                    workspace: String(w.workspace || I18n.tr("CURRENT"))
                });
            }
        }
        return out.slice(0, 6);
    }

    readonly property bool hasWindows: windows.length > 0
    readonly property int windowCount: windows.length
    readonly property var windowAddresses: windows.map(window => window.address)
    BusListModel {
        id: railRows
        key: "address"
        roles: ["address", "title", "cls", "workspace"]
        rows: root.windows
    }
    // Leave a real breathing gap below the ledger: the rail reads as a small
    // overview floater, not a continuation of the result list.
    readonly property real targetHeight: open && hasWindows ? 96 * s : 0

    implicitHeight: targetHeight
    height: targetHeight
    clip: true

    Behavior on height {
        enabled: root.animationsEnabled
        NumberAnimation { duration: Motion.drawer; easing.type: Easing.OutCubic }
    }

    function resetFocus() {
        focusedAddress = windows.length > 0 ? String(windows[0].address) : "";
        if (focusedAddress.length > 0)
            focusRequested(focusedAddress);
    }

    function resetForEntry() {
        var next = entry ? String(entry.resultKey || entry.id || "") : "";
        if (next === observedEntryKey)
            return;
        observedEntryKey = next;
        resetFocus();
    }

    function focusRelative(delta) {
        if (windows.length === 0)
            return;
        var current = -1;
        for (var index = 0; index < windows.length; index++)
            if (String(windows[index].address) === focusedAddress)
                current = index;
        var next = current < 0 ? 0 : (current + delta + windows.length)
            % windows.length;
        focusedAddress = String(windows[next].address);
        focusSerial++;
        focusRequested(focusedAddress);
    }

    onEntryChanged: resetForEntry()
    onWindowsChanged: {
        if (focusedAddress.length === 0 && windows.length > 0)
            resetFocus();
    }

    Item {
        id: railHeader
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: 20 * root.s
        anchors.leftMargin: 8 * root.s
        anchors.rightMargin: 8 * root.s

        Text {
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            text: root.windows.length === 1
                ? I18n.tr("OPEN WINDOWS  /  %1 WINDOW").arg(root.windows.length)
                : I18n.tr("OPEN WINDOWS  /  %1 WINDOWS").arg(root.windows.length)
            color: Theme.subtle
            font.family: Theme.mono
            font.pixelSize: 7.5 * root.s
            font.weight: Font.DemiBold
            font.letterSpacing: 0.8 * root.s
        }

        Text {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            text: root.focusActive
                ? I18n.tr("LEFT / RIGHT  ·  ENTER  SWITCH")
                : I18n.tr("TAB  ·  FOCUS WINDOWS")
            color: root.focusActive ? Theme.bright : Theme.faint
            font.family: Theme.mono
            font.pixelSize: 7 * root.s
            font.letterSpacing: 0.5 * root.s
        }
    }

    ListView {
        id: windowList
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: railHeader.bottom
        anchors.bottom: parent.bottom
        anchors.leftMargin: 8 * root.s
        anchors.rightMargin: 8 * root.s
        orientation: ListView.Horizontal
        spacing: 7 * root.s
        clip: true
        model: railRows
        boundsBehavior: Flickable.StopAtBounds

        delegate: Rectangle {
            id: tile
            required property var modelData
            readonly property bool focused: String(modelData.address)
                === root.focusedAddress
            readonly property bool railSelected: focused && root.focusActive
            width: 184 * root.s
            height: 64 * root.s
            radius: Math.max(2, 5 * root.s)
            color: railSelected ? Theme.leadContainer : Theme.drawerRaised
            border.width: railSelected ? 2 : 1
            border.color: railSelected ? Theme.bright : Theme.hair

            Behavior on color {
                ColorAnimation { duration: Motion.selection; easing.type: Easing.OutCubic }
            }
            Behavior on border.color {
                ColorAnimation { duration: Motion.selection; easing.type: Easing.OutCubic }
            }

            Rectangle {
                id: iconPlate
                anchors.left: parent.left
                anchors.leftMargin: 7 * root.s
                anchors.verticalCenter: parent.verticalCenter
                width: 32 * root.s
                height: width
                radius: Math.max(2, 4 * root.s)
                color: tile.railSelected
                    ? Qt.rgba(0, 0, 0, 0.18) : Theme.drawerRaised

                IconImage {
                    anchors.centerIn: parent
                    implicitSize: 21 * root.s
                    source: tile.modelData.cls
                        ? Svc.Icons.path(tile.modelData.cls,
                            "application-x-executable") : ""
                }
            }

            Column {
                anchors.left: iconPlate.right
                anchors.leftMargin: 7 * root.s
                anchors.right: parent.right
                anchors.rightMargin: 7 * root.s
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2 * root.s

                Text {
                    width: parent.width
                    text: tile.modelData.title
                    color: tile.railSelected ? Theme.onLead : Theme.cream
                    font.family: Theme.font
                    font.pixelSize: 9.2 * root.s
                    font.weight: tile.railSelected ? Font.DemiBold : Font.Medium
                    elide: Text.ElideRight
                }

                Text {
                    width: parent.width
                    text: I18n.tr("OPEN · %1")
                        .arg(String(tile.modelData.workspace).toUpperCase())
                    color: Theme.faint
                    font.family: Theme.mono
                    font.pixelSize: 6.5 * root.s
                    font.letterSpacing: 0.5 * root.s
                    elide: Text.ElideRight
                }
            }

        }
    }
}
