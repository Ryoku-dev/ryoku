pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io

/// The network API the bar and its popouts read. The data is the ryoku-shell
/// daemon's `network` topic, which owns NetworkManager, so nothing here talks to
/// D-Bus or runs nmcli. Access points, the active one and the saved profiles keep
/// the object shapes the QML expects.
Singleton {
    id: root

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    property var _wifi: ({ present: false, enabled: false, connectivity: "Disconnected", ssid: "", strength: 0 })
    property var _wired: ({ present: false, connectivity: "Disconnected" })
    property var _aps: []
    property var _vpn: []

    readonly property bool isConnected: root._wifi.connectivity === "Connected" || root._wired.connectivity === "Connected"
    readonly property bool wifiEnabled: root._wifi.enabled === true
    property bool scanning: false
    readonly property string connectingSsid: root.pendingConnection ? root.pendingConnection.ssid : ""

    // shortcut: the daemon publishes no per-device readout, so the detail pages stay
    // empty, add a device topic to the daemon when they matter.
    readonly property var wirelessDeviceDetails: ({})
    readonly property var ethernetDeviceDetails: ({})

    property list<EthernetDevice> __ethernetDevices: []
    readonly property list<EthernetDevice> ethernetDevices: __ethernetDevices
    readonly property EthernetDevice activeEthernet: __ethernetDevices.find(d => d.connected) ?? null
    readonly property bool hasAvailableEthernet: __ethernetDevices.some(d => d.state !== "unavailable")
    property string ethernetSpeed: ""
    property string ethernetDataUsage: ""

    readonly property var vpnConnections: root._vpn.map(v => ({
        name: v.name,
        uuid: v.uuid,
        type: v.service,
        service: v.service,
        active: v.active === true,
        connecting: v.connecting === true,
        ip4: v.ip4 || ""
    }))
    readonly property var activeVpn: root.vpnConnections.find(v => v.active) ?? null
    property string vpnPendingConnection: ""

    // The access point runs through NetworkManager's own hotspot (nmcli), under a
    // connection named "Hotspot": the Ryoku network daemon has no access point mode.
    readonly property QtObject hotspot: QtObject {
        id: hs
        property bool supported: false
        property bool enabled: false
        property bool busy: false
        property var _callback: null
        function enable(name: string, password: string, callback: var): void {
            if (password.length > 0 && password.length < 8) {
                if (callback)
                    callback({ success: false, error: qsTr("The hotspot password needs at least 8 characters") });
                return;
            }
            const cmd = ["nmcli", "device", "wifi", "hotspot", "con-name", "Hotspot", "ssid", name];
            hs._run(password.length > 0 ? cmd.concat(["password", password]) : cmd, callback);
        }
        function disable(callback: var): void {
            hs._run(["nmcli", "connection", "down", "Hotspot"], callback);
        }
        function _run(cmd: var, callback: var): void {
            if (hs.busy)
                return;
            hs.busy = true;
            hs._callback = callback;
            hotspotProc.command = cmd;
            hotspotProc.running = true;
        }
    }

    Process {
        id: hotspotProc
        stderr: StdioCollector { id: hotspotErr }
        onExited: code => {
            const cb = root.hotspot._callback;
            root.hotspot._callback = null;
            root.hotspot.busy = false;
            hotspotState.running = true;
            if (cb)
                cb(code === 0 ? { success: true } : { success: false, error: hotspotErr.text.trim() });
        }
    }

    // whether there is a Wi-Fi device to host it, and whether it is up now
    Process {
        id: hotspotState
        running: true
        command: ["sh", "-c", "nmcli -t -f TYPE device; echo --; nmcli -t -f NAME connection show --active"]
        stdout: StdioCollector {
            onStreamFinished: {
                const [devices, active] = text.split("--\n");
                root.hotspot.supported = (devices || "").split("\n").includes("wifi");
                root.hotspot.enabled = (active || "").split("\n").includes("Hotspot");
            }
        }
    }

    readonly property list<string> savedConnections: root._aps.filter(a => a.saved).map(a => a.ssid)
    readonly property list<string> savedConnectionSsids: root.savedConnections
    property list<SavedProfile> __savedConnectionProfiles: []
    readonly property list<SavedProfile> savedConnectionProfiles: __savedConnectionProfiles

    property list<AccessPoint> __networks: []
    readonly property list<AccessPoint> networks: __networks

    property AccessPoint active: null

    property var pendingConnection: null

    readonly property alias connectionCheckTimer: connectionCheckTimer
    readonly property alias immediateCheckTimer: immediateCheckTimer

    signal connectionSuccessful(string ssid)
    signal connectionFailed(string ssid)

    property int _nextCallId: 1
    property var _callbacks: ({})

    function _call(method: string, args: var, callback: var): void {
        const id = root._nextCallId++;
        if (callback) {
            const cbs = root._callbacks;
            cbs[id] = callback;
            root._callbacks = cbs;
        }
        ctl.queued += "call " + method + " " + JSON.stringify(Object.assign({}, args, { id: id })) + "\n";
        if (ctl.connected)
            ctl.flushQueued();
        else
            ctl.connected = true;
    }

    function _result(ok: bool, error: string): var {
        return { success: ok, output: ok ? "Done" : "", error: ok ? "" : (error || qsTr("Request failed")), exitCode: ok ? 0 : 1 };
    }

    function _applyFrame(line: string): void {
        try {
            const f = JSON.parse(line);
            if (f.wifi)
                root._wifi = f.wifi;
            if (f.wired)
                root._wired = f.wired;
            root._aps = Array.isArray(f.accessPoints) ? f.accessPoints : [];
            root._vpn = Array.isArray(f.vpn) ? f.vpn : [];
            root.rebuildNetworkList();
            root.rebuildEthernetDevices();
            root.rebuildSavedProfiles();
            root.rebuildActive();
            if (root.vpnPendingConnection) {
                const v = root._vpn.find(x => x.name === root.vpnPendingConnection);
                if (!v || v.connecting !== true)
                    root.vpnPendingConnection = "";
            }
        } catch (e) {
            // A malformed frame keeps the last good state.
        }
    }

    function rebuildActive(): void {
        const raw = root._aps.find(a => a.active) ?? null;
        if (raw && raw.ssid && raw.ssid.length > 0) {
            const found = __networks.find(n => n.ssid === raw.ssid) ?? null;
            if (found) {
                root.active = found;
            } else if (!root.active || root.active.ssid !== raw.ssid) {
                root.active = apComp.createObject(root, { lastIpcObject: raw });
            }
            if (root.pendingConnection && root.active && root.active.ssid && root.active.ssid.toLowerCase().trim() === root.pendingConnection.ssid.toLowerCase().trim()) {
                const pending = root.pendingConnection;
                root.pendingConnection = null;
                connectionCheckTimer.stop();
                immediateCheckTimer.stop();
                immediateCheckTimer.checkCount = 0;
                root.connectionSuccessful(pending.ssid);
                if (pending.callback && typeof pending.callback === "function")
                    pending.callback({ success: true, output: "Connected", error: "", exitCode: 0 });
            }
        } else if (root.active !== null) {
            root.active = null;
        }
    }

    function rebuildNetworkList(): void {
        const rawList = root._aps;
        const newList = [];
        const oldMap = new Map();

        for (let i = 0; i < __networks.length; i++) {
            const ap = __networks[i];
            if (ap && ap.ssid)
                oldMap.set(ap.bssid || ap.ssid, ap);
        }

        for (let i = 0; i < rawList.length; i++) {
            const raw = rawList[i];
            const key = raw.bssid || raw.ssid;
            const existing = oldMap.get(key);
            if (existing) {
                existing.lastIpcObject = raw;
                newList.push(existing);
                oldMap.delete(key);
            } else {
                newList.push(apComp.createObject(root, { lastIpcObject: raw }));
            }
        }

        oldMap.forEach(ap => ap.destroy());
        root.__networks = newList;
    }

    // One wired device at most, taken from the wired section of the frame.
    function rebuildEthernetDevices(): void {
        const wired = root._wired;
        const state = wired.connectivity === "Connected" ? 10 : (wired.connectivity === "Connecting" ? 4 : 3);
        const raw = { "interface": "ethernet", type: "ethernet", state: state, connected: state === 10, connection: "" };
        if (!wired.present) {
            for (const d of __ethernetDevices)
                d.destroy();
            root.__ethernetDevices = [];
        } else if (__ethernetDevices.length > 0) {
            __ethernetDevices[0].lastIpcObject = raw;
        } else {
            root.__ethernetDevices = [ethDevComp.createObject(root, { lastIpcObject: raw })];
        }
    }

    function rebuildSavedProfiles(): void {
        const saved = root._aps.filter(a => a.saved);
        const seen = new Set();
        const newList = [];
        for (const raw of saved) {
            if (seen.has(raw.ssid))
                continue;
            seen.add(raw.ssid);
            newList.push(savedProfileComp.createObject(root, {
                lastIpcObject: { ssid: raw.ssid, id: raw.ssid, uuid: "", security: raw.security, active: raw.active }
            }));
        }
        for (const old of __savedConnectionProfiles)
            old.destroy();
        root.__savedConnectionProfiles = newList;
    }

    function _startPending(ssid: string, bssid: string, callback: var): void {
        root.pendingConnection = { ssid: ssid, bssid: bssid || "", callback: callback };
        connectionCheckTimer.start();
        immediateCheckTimer.checkCount = 0;
        immediateCheckTimer.start();
    }

    function _clearPending(): void {
        root.pendingConnection = null;
        connectionCheckTimer.stop();
        immediateCheckTimer.stop();
        immediateCheckTimer.checkCount = 0;
    }

    function connectToNetwork(ssid: string, password: string, bssid: string, callback: var): void {
        root._startPending(ssid, bssid, callback);
        root._call("network.wifiConnect", { ssid: ssid, password: password || "", bssid: bssid || "", hidden: false }, result => {
            if (!result.success) {
                root._clearPending();
                root.connectionFailed(ssid);
                if (callback && typeof callback === "function")
                    callback(result);
            }
        });
    }

    function connectToNetworkWithPasswordCheck(ssid: string, isSecure: bool, callback: var, bssid: string): void {
        if (isSecure && !root.hasSavedProfile(ssid)) {
            if (callback && typeof callback === "function")
                callback({ success: false, output: "", error: "", exitCode: 1, needsPassword: true });
            return;
        }
        root.connectToNetwork(ssid, "", bssid, callback);
    }

    // shortcut: the daemon keys saved profiles by SSID, so a UUID cannot be targeted.
    function connectToNetworkByUuid(uuid: string, callback: var): void {
        if (callback && typeof callback === "function")
            callback(root._result(false, qsTr("Saved profiles are selected by network name")));
    }

    function loadSavedConnections(callback: var): void {
        if (callback && typeof callback === "function")
            callback(root.savedConnections);
    }

    function connectVpn(connectionName: string, callback: var): void {
        const v = root._vpn.find(x => x.name === connectionName);
        if (!v)
            return;
        root.vpnPendingConnection = connectionName;
        root._call("network.vpnActivate", { uuid: v.uuid }, result => {
            if (!result.success)
                root.vpnPendingConnection = "";
            if (callback && typeof callback === "function")
                callback(result);
        });
    }

    function disconnectVpn(connectionName: string, callback: var): void {
        const v = root._vpn.find(x => x.name === connectionName);
        if (v)
            root._call("network.vpnDeactivate", { uuid: v.uuid }, callback);
    }

    function connectEthernet(connectionName: string, interfaceName: string, callback: var): void {
        if (callback && typeof callback === "function")
            callback(root._result(false, qsTr("Wired connections are managed by NetworkManager")));
    }

    function disconnectEthernet(connectionName: string, callback: var): void {
        root.connectEthernet(connectionName, "", callback);
    }

    function hasSavedProfile(ssid: string): bool {
        return root._aps.some(a => a.ssid === ssid && a.saved);
    }

    function forgetNetwork(ssid: string, callback: var): void {
        root._call("network.wifiForget", { ssid: ssid }, callback);
    }

    function forgetNetworkByUuid(uuid: string, callback: var): void {
        if (callback && typeof callback === "function")
            callback(root._result(false, qsTr("Saved profiles are selected by network name")));
    }

    function disconnectFromNetwork(): void {
        root._call("network.wifiDisconnect", {}, null);
    }

    function rescanWifi(): void {
        root.scanning = true;
        scanTimer.restart();
        root._call("network.wifiScan", {}, null);
    }

    function enableWifi(enabled: bool, callback: var): void {
        root._call("network.wifiSetEnabled", { enabled: enabled }, callback);
    }

    function toggleWifi(callback: var): void {
        root.enableWifi(!root.wifiEnabled, callback);
    }

    function getWirelessDeviceDetails(interfaceName: string, callback: var): void {
        if (callback && typeof callback === "function")
            callback({});
    }

    function getEthernetDeviceDetails(interfaceName: string, callback: var): void {
        if (callback && typeof callback === "function")
            callback({});
    }

    function findNetwork(ssid: string): var {
        return networks.find(n => n.ssid === ssid) ?? null;
    }

    function securityLabel(keyMgmt: string): string {
        switch ((keyMgmt || "").trim().toLowerCase()) {
        case "":
        case "none":
            return qsTr("Open");
        case "sae":
            return "WPA3";
        case "wpa-psk":
            return "WPA2";
        case "wpa-eap":
        case "wpa-eap-suite-b-192":
            return qsTr("Enterprise");
        case "owe":
            return qsTr("Enhanced Open");
        case "ieee8021x":
            return "802.1X";
        default:
            return keyMgmt.trim();
        }
    }

    function getEthernetInterfaces(callback: var): void {
        const names = root.__ethernetDevices.map(d => d.iface);
        if (callback && typeof callback === "function")
            callback(names);
    }

    function getEthernetSpeed(interfaceName: string): void {
        root.ethernetSpeed = "";
    }

    function getEthernetDataUsage(interfaceName: string, callback: var): void {
        root.ethernetDataUsage = "";
        if (callback && typeof callback === "function")
            callback("");
    }

    // shortcut: addresses and routes are NetworkManager's to edit, these answer with
    // an empty config and a refusal until the daemon grows the calls.
    function getIpv4Config(connectionId: string, callback: var): void {
        if (callback && typeof callback === "function")
            callback({});
    }

    function setIpv4Config(connectionId: string, config: var, callback: var): void {
        if (callback && typeof callback === "function")
            callback(root._result(false, qsTr("Address settings are not editable here")));
    }

    function setAutoconnect(connectionId: string, enabled: bool, callback: var): void {
        root._call("network.wifiSetAutoconnect", { ssid: connectionId, autoconnect: enabled }, callback);
    }

    function addHiddenNetwork(ssid: string, password: string, security: string, hidden: bool, callback: var): void {
        root._startPending(ssid, "", callback);
        root._call("network.wifiConnect", { ssid: ssid, password: password || "", bssid: "", hidden: hidden === true }, result => {
            if (!result.success) {
                root._clearPending();
                root.connectionFailed(ssid);
                if (callback && typeof callback === "function")
                    callback(result);
            }
        });
    }

    Timer {
        id: scanTimer

        interval: 4000
        onTriggered: root.scanning = false
    }

    Timer {
        id: connectionCheckTimer

        interval: 20000

        onTriggered: {
            if (!root.pendingConnection)
                return;
            const pending = root.pendingConnection;
            const connected = root.active && root.active.ssid && root.active.ssid.toLowerCase().trim() === pending.ssid.toLowerCase().trim();
            root._clearPending();
            if (connected) {
                root.connectionSuccessful(pending.ssid);
                if (pending.callback && typeof pending.callback === "function")
                    pending.callback({ success: true, output: "Connected", error: "", exitCode: 0 });
            } else {
                root.connectionFailed(pending.ssid);
                if (pending.callback && typeof pending.callback === "function")
                    pending.callback({ success: false, output: "", error: "Connection timeout", exitCode: -1, needsPassword: false });
            }
        }
    }

    Timer {
        id: immediateCheckTimer

        property int checkCount: 0

        interval: 500
        repeat: true
        onTriggered: {
            if (!root.pendingConnection) {
                stop();
                checkCount = 0;
                return;
            }
            checkCount++;
            const pending = root.pendingConnection;
            const connected = root.active && root.active.ssid && root.active.ssid.toLowerCase().trim() === pending.ssid.toLowerCase().trim();
            if (connected) {
                root._clearPending();
                root.connectionSuccessful(pending.ssid);
                if (pending.callback && typeof pending.callback === "function")
                    pending.callback({ success: true, output: "Connected", error: "", exitCode: 0 });
            } else if (checkCount >= 6) {
                stop();
                checkCount = 0;
            }
        }
    }

    // The daemon may be down when the shell loads, or restart under it; retry
    // quietly so the view fills in once it returns.
    Timer {
        id: retry

        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }

    Socket {
        id: sub

        path: root.sockPath
        parser: SplitParser { onRead: line => root._applyFrame(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe network\n");
                flush();
            } else {
                retry.restart();
            }
        }
    }

    Socket {
        id: ctl

        property string queued: ""

        path: root.sockPath
        parser: SplitParser {
            onRead: line => {
                try {
                    const r = JSON.parse(line);
                    const cb = root._callbacks[r.id];
                    if (cb) {
                        const cbs = root._callbacks;
                        delete cbs[r.id];
                        root._callbacks = cbs;
                        cb(root._result(r.ok === true, r.error || ""));
                    }
                } catch (e) {
                    // A reply that does not parse has no caller to wake.
                }
            }
        }

        function flushQueued(): void {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }

        onConnectionStateChanged: if (connected) flushQueued()
    }

    Component {
        id: apComp

        AccessPoint {}
    }

    Component {
        id: ethDevComp

        EthernetDevice {}
    }

    Component {
        id: savedProfileComp

        SavedProfile {}
    }

    component AccessPoint: QtObject {
        required property var lastIpcObject

        readonly property string ssid: lastIpcObject.ssid ?? ""
        readonly property string bssid: lastIpcObject.bssid ?? ""
        readonly property int strength: lastIpcObject.strength ?? 0
        readonly property int frequency: lastIpcObject.frequency ?? 0
        readonly property bool active: lastIpcObject.active ?? false
        readonly property string security: lastIpcObject.security ?? ""
        readonly property bool isSecure: (lastIpcObject.security ?? "").length > 0
    }

    component SavedProfile: QtObject {
        required property var lastIpcObject

        property bool duplicate: false

        readonly property string ssid: lastIpcObject.ssid ?? ""
        readonly property string id: lastIpcObject.id ?? ""
        readonly property string uuid: lastIpcObject.uuid ?? ""
        readonly property string path: lastIpcObject.path ?? ""
        readonly property string security: lastIpcObject.security ?? ""
        readonly property bool active: lastIpcObject.active ?? false

        readonly property string disambiguator: id && id !== ssid ? id : uuid.slice(-4).toUpperCase()
    }

    component EthernetDevice: QtObject {
        required property var lastIpcObject

        readonly property string iface: lastIpcObject["interface"] ?? ""
        readonly property string type: lastIpcObject.type ?? ""
        readonly property string state: deviceStateName(lastIpcObject.state)
        readonly property string connection: lastIpcObject.connection ?? ""
        readonly property bool connected: lastIpcObject.connected ?? false
        readonly property string ipAddress: lastIpcObject.ipAddress ?? ""
        readonly property string gateway: lastIpcObject.gateway ?? ""
        readonly property var dns: lastIpcObject.dns ?? []
        readonly property string subnet: lastIpcObject.subnet ?? ""
        readonly property string macAddress: lastIpcObject.macAddress ?? ""
        readonly property string speed: lastIpcObject.speed ?? ""

        function deviceStateName(state: var): string {
            const names = ["unknown", "unmanaged", "unavailable", "disconnected", "prepare", "config", "need-auth", "ip-config", "ip-check", "secondaries", "activated", "deactivating", "failed"];
            if (typeof state === "number" && state >= 0 && state < names.length)
                return names[state];
            return state ?? "unknown";
        }
    }
}
