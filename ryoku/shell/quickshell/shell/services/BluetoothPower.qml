pragma Singleton

import QtQuick
import Qt.labs.folderlistmodel
import Quickshell
import Quickshell.Bluetooth
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "BluetoothRfkill.js" as Rfkill
import "BluetoothFeed.js" as Feed

Singleton {
    id: root

    readonly property var quickshellAdapter: {
        const adapters = Bluetooth.adapters.values;
        if (adapters.length === 0)
            return null;
        const preferred = Bluetooth.defaultAdapter;
        return preferred && adapters.indexOf(preferred) >= 0 ? preferred : adapters[0];
    }
    // The daemon's `bluetooth` frame covers what Quickshell's BlueZ client cannot:
    // bluetoothd started after the shell, or gone while Quickshell keeps the adapter.
    property var feed: null
    readonly property var view: Feed.resolve(root.quickshellAdapter !== null,
        root.quickshellAdapter?.enabled ?? false, root.feed)
    readonly property var adapter: root.view.useQuickshell ? root.quickshellAdapter : null
    readonly property var devices: root.adapter !== null && Bluetooth.devices
        ? Bluetooth.devices.values : []
    readonly property bool enabled: root.view.enabled
    readonly property bool available: root.view.present || root.blocked
    readonly property bool blocked: root.softBlocked || root.hardBlocked
        || root.adapter?.state === BluetoothAdapterState.Blocked
    property bool softBlocked: false
    property bool hardBlocked: false

    // Last power choice made here: -1 none, 0 off, 1 on.
    property int wanted: -1
    readonly property bool settled: Rfkill.settled(root.wanted, root.enabled, root.blocked)
    readonly property int adapterState: root.adapter?.state ?? -1
    readonly property int adapterCount: Bluetooth.adapters.values.length
    readonly property string rfkillFolder: "file:///sys/class/rfkill"

    function setEnabled(on: bool): void {
        root.wanted = on ? 1 : 0;
        Quickshell.execDetached(["ryoku-bluetooth-power", on ? "on" : "off"]);
        root.rescanRfkill();
        recheck.interval = Rfkill.firstRecheck;
        recheck.restart();
    }

    function toggle(): void {
        setEnabled(!enabled);
    }

    function rescanRfkill(): void {
        rfkillRescan.restart();
    }

    function collectRfkill(): void {
        if (rfkillDir.status !== FolderListModel.Ready || rfkillEntries.count !== rfkillDir.count)
            return;
        const entries = [];
        for (let i = 0; i < rfkillEntries.count; i++)
            entries.push(rfkillEntries.objectAt(i));
        const state = Rfkill.blockState(entries);
        if (state === null)
            return;
        root.softBlocked = state.soft;
        root.hardBlocked = state.hard;
    }

    onAdapterChanged: root.rescanRfkill()
    onAdapterCountChanged: root.rescanRfkill()
    onAdapterStateChanged: root.rescanRfkill()
    onFeedChanged: root.rescanRfkill()

    function applyFeed(data: var): void {
        const frame = Feed.frameFrom(data);
        if (frame !== null && JSON.stringify(frame) !== JSON.stringify(root.feed))
            root.feed = frame;
    }

    // A daemon without the topic never sends a frame, so feed stays null and
    // Quickshell's own adapter view is used, as with no daemon at all.
    BusTopic {
        id: topicFeed
        topic: "bluetooth"
        onUpdated: root.applyFeed(topicFeed.data)
    }

    Connections {
        target: Bus
        function onConnectedChanged() {
            if (!Bus.connected)
                root.feed = null;
        }
    }
    onSettledChanged: if (root.settled) recheck.stop()

    Timer {
        id: rfkillRescan
        interval: 150
        onTriggered: {
            rfkillDir.folder = "";
            rfkillDir.folder = root.rfkillFolder;
        }
    }

    // A platform switch can flip rfkill while its controller is gone from BlueZ, with no signal.
    Timer {
        id: recheck
        interval: Rfkill.firstRecheck
        onTriggered: {
            if (root.settled)
                return;
            root.rescanRfkill();
            const next = Rfkill.nextRecheck(interval);
            if (next > 0) {
                interval = next;
                restart();
            }
        }
    }

    FolderListModel {
        id: rfkillDir
        folder: root.rfkillFolder
        showDotAndDotDot: false
        showHidden: false
        sortField: FolderListModel.Unsorted
        onStatusChanged: Qt.callLater(root.collectRfkill)
        onCountChanged: Qt.callLater(root.collectRfkill)
    }

    Instantiator {
        id: rfkillEntries
        model: rfkillDir
        delegate: QtObject {
            id: entry

            required property string filePath
            property int pending: 3
            property string type: ""
            property string soft: ""
            property string hard: ""

            function settle(): void {
                entry.pending -= 1;
                if (entry.pending === 0)
                    Qt.callLater(root.collectRfkill);
            }

            property FileView typeFile: FileView {
                path: entry.filePath + "/type"
                printErrors: false
                onLoaded: { entry.type = text().trim(); entry.settle(); }
                onLoadFailed: entry.settle()
            }
            property FileView softFile: FileView {
                path: entry.filePath + "/soft"
                printErrors: false
                onLoaded: { entry.soft = text().trim(); entry.settle(); }
                onLoadFailed: entry.settle()
            }
            property FileView hardFile: FileView {
                path: entry.filePath + "/hard"
                printErrors: false
                onLoaded: { entry.hard = text().trim(); entry.settle(); }
                onLoadFailed: entry.settle()
            }
        }
        onObjectAdded: Qt.callLater(root.collectRfkill)
    }
}
