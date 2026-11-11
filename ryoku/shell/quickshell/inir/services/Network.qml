pragma Singleton
pragma ComponentBehavior: Bound

// Took many bits from https://github.com/caelestia-dots/shell (GPLv3)

import Quickshell
import Quickshell.Io
import QtQuick
import inir.modules.common.functions
import inir.services
import inir.services.network
import shell.services as Ryoku

/**
 * Network controls with link state from Ryoku's shared daemon topic.
 */
Singleton {
    id: root

    readonly property bool wifi: simulatedLink.length > 0
        ? simulatedLink === "wifi"
        : Ryoku.Network.kind === "wifi"
    readonly property bool ethernet: simulatedLink.length > 0
        ? simulatedLink === "ethernet"
        : Ryoku.Network.kind === "ethernet"

    readonly property bool wifiEnabled: simulatedLink.length > 0
        ? simulatedLink === "wifi" || simulatedLink === "searching" || simulatedLink === "connecting"
        : Ryoku.Network.wifiRadio
    property bool wifiScanning: false
    property bool wifiConnecting: connectProc.running
    property WifiAccessPoint wifiConnectTarget
    readonly property list<WifiAccessPoint> wifiNetworks: []
    readonly property WifiAccessPoint active: wifiNetworks.find(n => n.active) ?? null
    readonly property string wifiStatus: simulatedLink.length > 0
        ? simulatedLink === "wifi" ? "connected"
        : simulatedLink === "connecting" ? "connecting"
        : simulatedLink === "searching" ? "disconnected"
        : "disabled"
        : Ryoku.Network.wifiConnectivity === "Connected"
        ? (Ryoku.Network.connectivity === "limited" ? "limited" : "connected")
        : Ryoku.Network.wifiConnectivity === "Connecting" ? "connecting"
        : wifiEnabled ? "disconnected" : "disabled"

    readonly property string networkName: simulatedLink === "wifi" ? "Simulated Wi-Fi"
        : simulatedLink === "ethernet" ? "Simulated cable"
        : Ryoku.Network.kind === "wifi" ? Ryoku.Network.activeSsid
        : Ryoku.Network.kind === "ethernet" ? "Ethernet" : ""
    property int simulatedStrength: 80
    readonly property int networkStrength: simulatedLink === "wifi"
        ? simulatedStrength
        : Math.round(Ryoku.Network.level * 100)

    readonly property string connectivity: Ryoku.Network.connectivity
    property string simulatedConnectivity: ""
    // `inir network simulateLink "wifi 40"` stands in for NetworkManager's link state (wifi strength, searching,
    // connecting, radio off, cable, no adapter) until `off` or a restart; the probes stop writing while it holds.
    property string simulatedLink: ""
    readonly property string effectiveConnectivity: simulatedConnectivity || connectivity
    readonly property bool online: effectiveConnectivity === "full" || effectiveConnectivity === "unknown"
    readonly property string offlineReason: online ? ""
        : effectiveConnectivity === "portal" ? Translation.tr("Sign in to the network to go online")
        : effectiveConnectivity === "limited" ? Translation.tr("Connected, but no internet")
        : Translation.tr("No internet connection")
    // Gated on being *connected*, not on the radio being powered. wifiStatus is
    // only ever "connecting"/"disconnected" while the radio is on — i.e. while
    // wifiEnabled is true — so keying the strength icons off wifiEnabled both
    // rendered a stale full-strength icon after disconnecting and left the
    // "not_connected"/"wifi_find" branches permanently unreachable.
    property string materialSymbol: root.ethernet
        ? "lan"
        : !root.wifiEnabled
            ? "signal_wifi_off"
            : (root.wifiStatus === "connected" || root.wifiStatus === "limited")
                ? (
                    Network.networkStrength > 83 ? "signal_wifi_4_bar" :
                    Network.networkStrength > 67 ? "network_wifi" :
                    Network.networkStrength > 50 ? "network_wifi_3_bar" :
                    Network.networkStrength > 33 ? "network_wifi_2_bar" :
                    Network.networkStrength > 17 ? "network_wifi_1_bar" :
                    "signal_wifi_0_bar"
                )
                : (root.wifiStatus === "connecting")
                    ? "signal_wifi_statusbar_not_connected"
                    : (root.wifiStatus === "disconnected")
                        ? "wifi_find"
                        : (root.wifiStatus === "disabled")
                            ? "signal_wifi_off"
                            : "signal_wifi_bad"

    // Control
    function enableWifi(enabled = true): void {
        const cmd = enabled ? "on" : "off";
        enableWifiProc.exec(["nmcli", "radio", "wifi", cmd]);
    }

    function toggleWifi(): void {
        enableWifi(!wifiEnabled);
    }

    function rescanWifi(): void {
        wifiScanning = true;
        getNetworks.running = true;
        rescanProcess.running = true;
    }

    function connectToWifiNetwork(accessPoint: WifiAccessPoint): void {
        accessPoint.askingPassword = false;
        root.wifiConnectTarget = accessPoint;
        // We use this instead of `nmcli connection up SSID` because this also creates a connection profile
        connectProc.exec(["nmcli", "dev", "wifi", "connect", accessPoint.ssid])

    }

    function disconnectWifiNetwork(): void {
        if (active) disconnectProc.exec(["nmcli", "connection", "down", active.ssid]);
    }

    function refreshActiveNetworkDetails(): void {
        if (!getNetworks.running) {
            getNetworks.running = true;
        }
    }

    function openPublicWifiPortal() {
        ShellExec.execDetachedArgs(["xdg-open", "https://nmcheck.gnome.org/"], "Open network check") // From some StackExchange thread, seems to work
    }

    function accessPointDetails(accessPoint, includeBssid = false): string {
        if (!accessPoint) return "";
        const details = [];
        if (accessPoint.strength >= 0)
            details.push(`${accessPoint.strength}%`);
        if (accessPoint.bandLabel.length > 0) {
            details.push(accessPoint.frequency > 0
                ? `${accessPoint.bandLabel} (${accessPoint.frequency} MHz)`
                : accessPoint.bandLabel);
        }
        if (accessPoint.rate.length > 0)
            details.push(accessPoint.rate);
        if (includeBssid && accessPoint.bssid.length > 0)
            details.push(accessPoint.bssid);
        return details.join(" | ");
    }

    function connectionTooltip(includeBssid = false): string {
        if (!root.wifiEnabled) return Translation.tr("Wi-Fi is disabled");
        if (root.ethernet) return Translation.tr("Ethernet connected");
        if (!root.networkName) return Translation.tr("Not connected");
        const details = root.accessPointDetails(root.active, includeBssid);
        return details.length > 0 ? `${root.networkName} | ${details}` : root.networkName;
    }

    function changePassword(network: WifiAccessPoint, password: string, username = ""): void {
        // TODO: enterprise wifi with username
        network.askingPassword = false;
        // changePasswordProc.onExited re-runs connectProc, whose own onExited has
        // already nulled the target — restore it so the retry can report back.
        root.wifiConnectTarget = network;
        changePasswordProc.exec({
            "command": ["nmcli", "connection", "modify", network.ssid, "wifi-sec.psk", password]
        })
    }

    Process {
        id: enableWifiProc
    }

    Process {
        id: connectProc
        environment: ({
            LANG: "C",
            LC_ALL: "C"
        })
        stdout: SplitParser {
            onRead: line => {
                // print(line)
                getNetworks.running = true
            }
        }
        stderr: SplitParser {
            onRead: line => {
                // print("err:", line)
                if (line.includes("Secrets were required") && root.wifiConnectTarget) {
                    root.wifiConnectTarget.askingPassword = true
                }
            }
        }
        onExited: (exitCode, exitStatus) => {
            // Guarded: this handler nulls the target, and changePasswordProc re-runs
            // connectProc, so a password retry lands here a second time.
            if (!root.wifiConnectTarget) return;
            root.wifiConnectTarget.askingPassword = (exitCode !== 0)
            root.wifiConnectTarget = null
        }
    }

    Process {
        id: disconnectProc
        stdout: SplitParser {
            onRead: getNetworks.running = true
        }
    }

    Process {
        id: changePasswordProc
        onExited: { // Re-attempt connection after changing password
            connectProc.running = false
            connectProc.running = true
        }
    }

    Process {
        id: rescanProcess
        command: ["nmcli", "dev", "wifi", "list", "--rescan", "yes"]
        stdout: SplitParser {
            onRead: {
                wifiScanning = false;
                getNetworks.running = true;
            }
        }
    }


    function update() {
        if (root.simulatedLink.length > 0)
            return;
        root.networkChanged();
        if (!getNetworks.running)
            getNetworks.running = true;
    }

    signal networkChanged()

    Component.onCompleted: Qt.callLater(() => root.update())

    Connections {
        target: Ryoku.Network
        function onConnectionChanged(): void { root.update() }
        function onWifiChanged(): void { root.update() }
        function onWiredChanged(): void { root.update() }
        function onVpnProfilesChanged(): void { root.update() }
        function onWgTunnelsChanged(): void { root.update() }
    }

    Process {
        id: getNetworks
        running: false
        command: ["nmcli", "-g", "ACTIVE,SIGNAL,FREQ,SSID,BSSID,SECURITY,RATE", "device", "wifi", "list", "--rescan", "no"]
        environment: ({
            LANG: "C",
            LC_ALL: "C"
        })
        stdout: StdioCollector {
            onStreamFinished: {
                const PLACEHOLDER = "STRINGWHICHHOPEFULLYWONTBEUSED";
                const rep = new RegExp("\\\\:", "g");
                const rep2 = new RegExp(PLACEHOLDER, "g");

                const allNetworks = text.trim().split("\n").map(n => {
                    const net = n.replace(rep, PLACEHOLDER).split(":");
                    return {
                        active: net[0] === "yes",
                        strength: parseInt(net[1]),
                        frequency: parseInt(net[2]),
                        ssid: net[3],
                        bssid: net[4]?.replace(rep2, ":") ?? "",
                        security: net[5] || "",
                        rate: net[6] || ""
                    };
                }).filter(n => n.ssid && n.ssid.length > 0);

                // Group networks by SSID and prioritize connected ones
                const networkMap = new Map();
                for (const network of allNetworks) {
                    const existing = networkMap.get(network.ssid);
                    if (!existing) {
                        networkMap.set(network.ssid, network);
                    } else {
                        // Prioritize active/connected networks
                        if (network.active && !existing.active) {
                            networkMap.set(network.ssid, network);
                        } else if (!network.active && !existing.active) {
                            // If both are inactive, keep the one with better signal
                            if (network.strength > existing.strength) {
                                networkMap.set(network.ssid, network);
                            }
                        }
                        // If existing is active and new is not, keep existing
                    }
                }

                const wifiNetworks = Array.from(networkMap.values());

                const rNetworks = root.wifiNetworks;

                const destroyed = rNetworks.filter(rn => !wifiNetworks.find(n => n.frequency === rn.frequency && n.ssid === rn.ssid && n.bssid === rn.bssid));
                for (const network of destroyed)
                    rNetworks.splice(rNetworks.indexOf(network), 1).forEach(n => n.destroy());

                for (const network of wifiNetworks) {
                    const match = rNetworks.find(n => n.frequency === network.frequency && n.ssid === network.ssid && n.bssid === network.bssid);
                    if (match) {
                        match.lastIpcObject = network;
                    } else {
                        rNetworks.push(apComp.createObject(root, {
                            lastIpcObject: network
                        }));
                    }
                }
            }
        }
    }

    Process {
        id: connectivityCheck
        command: ["nmcli", "networking", "connectivity", "check"]
        onExited: root.update()
    }

    IpcHandler {
        target: "network"

        function status(): string {
            return JSON.stringify({
                online: root.online,
                connectivity: root.effectiveConnectivity,
                simulated: root.simulatedConnectivity !== "",
                simulatedLink: root.simulatedLink,
                ethernet: root.ethernet,
                wifi: root.wifiStatus,
                name: root.networkName
            });
        }
        function check(): void {
            connectivityCheck.running = true;
        }
        function simulate(state: string): string {
            const s = (state ?? "").trim();
            root.simulatedConnectivity = ["full", "limited", "portal", "none"].includes(s) ? s : "";
            return root.effectiveConnectivity;
        }
        // "wifi 40", "searching", "connecting", "radio off", "ethernet", "none" (no adapter) or "off".
        function simulateLink(spec: string): string {
            const words = String(spec ?? "").trim().toLowerCase().split(/\s+/).filter(w => w.length > 0);
            const kind = words[0] ?? "off";
            if (!["wifi", "searching", "connecting", "radio", "ethernet", "none"].includes(kind)) {
                root.simulatedLink = "";
                root.update();
                return status();
            }
            root.simulatedLink = kind;
            root.simulatedStrength = kind === "wifi"
                ? Math.max(0, Math.min(100, parseInt(words[1] ?? "80") || 0))
                : 0;
            return status();
        }
    }

    Component {
        id: apComp

        WifiAccessPoint {}
    }
}
