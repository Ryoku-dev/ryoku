pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia
import Caelestia.Config
import caelestia.services

Singleton {
    id: root

    property bool connected: false
    property var status: ({
            connected: false,
            state: "disconnected",
            reason: "",
            authUrl: "",
            server: ""
        })

    property bool connectPending: false
    property bool disconnectPending: false
    readonly property bool connecting: connectProc.running || connectPending
    readonly property bool disconnecting: disconnectProc.running || disconnectPending

    readonly property string selectedProvider: GlobalConfig.utilities.vpn.selectedProvider

    property double connectedSince: 0
    property string bytesIn: ""
    property string bytesOut: ""
    property int pingMs: -1
    property string serverLocation: ""

    property string pendingSwitchProvider: ""

    property bool connectExited
    property bool disconnectExited

    property bool autoConnectPending
    property bool registerSent: false
    property int statusGen: 0

    readonly property var selected: root.providers.find(p => p.id === root.selectedProvider) ?? null

    readonly property var active: {
        const custom = root.selected;
        const name = custom ? (custom.name || "custom") : "wireguard";
        const adapter = adapters.find(a => a.name === name) ?? null;
        const iface = (custom ? custom.interface : "") || (adapter ? adapter.iface : "") || (adapter ? "" : name);
        const resolve = c => typeof c === "function" ? c(iface) : c;
        return {
            name: name,
            displayName: (custom ? custom.displayName : "") || resolve(adapter ? adapter.display : null) || name,
            interface: iface,
            connectCmd: (custom && custom.connectCmd.length > 0 ? custom.connectCmd : resolve(adapter ? adapter.connectCmd : null)) || [name, "up"],
            disconnectCmd: (custom && custom.disconnectCmd.length > 0 ? custom.disconnectCmd : resolve(adapter ? adapter.disconnectCmd : null)) || [name, "down"],
            statusCmd: (adapter ? adapter.statusCmd : null) || ["ip", "link", "show"],
            parse: (adapter ? adapter.parse : null) || (out => root.parseInterfaceStatus(out, iface)),
            service: adapter ? adapter.service : `${name}d`,
            connectHint: adapter ? adapter.connectHint : null,
            registerCmd: adapter ? adapter.registerCmd : null,
            serverCmd: adapter ? adapter.serverCmd : null,
            parseServer: adapter ? adapter.parseServer : null
        };
    }

    readonly property string providerName: active.name
    readonly property string interfaceName: active.interface
    readonly property var currentConfig: active

    readonly property var adapters: [wireguardAdapter, warpAdapter, netbirdAdapter, tailscaleAdapter]

    readonly property var providers: GlobalConfig.utilities.vpn.provider.values

    readonly property list<string> optionalKeys: ["displayName", "interface", "connectCmd", "disconnectCmd"]

    function generateId(): string {
        return `vpn-${Date.now().toString(36)}-${Math.floor(Math.random() * 0x1000000).toString(36)}`;
    }

    function setOrReset(provider: var, key: string, value: var): void {
        if (value && value.length > 0)
            provider[key] = value;
        else
            provider.resetOption(key);
    }

    function buildProviderProps(id: string, data: var): var {
        const props = {
            "id": id,
            "name": data.name
        };
        for (const key of root.optionalKeys)
            if (data[key] && data[key].length > 0)
                props[key] = data[key];
        return props;
    }

    function providerIdAt(index: int): string {
        return GlobalConfig.utilities.vpn.provider.at(index)?.id ?? "";
    }

    function addProvider(data: var): void {
        GlobalConfig.utilities.vpn.provider.insert(root.buildProviderProps(root.generateId(), data));
    }

    function updateProvider(index: int, data: var): void {
        const provider = GlobalConfig.utilities.vpn.provider.at(index);
        if (!provider)
            return;
        if (provider.id.length === 0)
            provider.id = root.generateId();
        provider.name = data.name;
        for (const key of root.optionalKeys)
            root.setOrReset(provider, key, data[key]);
    }

    function deleteProvider(index: int): void {
        GlobalConfig.utilities.vpn.provider.remove(index);
    }

    function setActiveProvider(index: int): void {
        const id = root.providerIdAt(index);
        if (id.length === 0)
            return;
        if (root.connected) {
            root.pendingSwitchProvider = id;
            root.disconnect();
        } else {
            applySelectedProvider(id);
        }
    }

    function applySelectedProvider(id: string): void {
        GlobalConfig.utilities.vpn.selectedProvider = id;
    }

    function ensureSelection(): void {
        const providers = root.providers;
        if (providers.some(p => p.id === root.selectedProvider))
            return;
        const next = providers.length > 0 ? providers[0].id : "";
        if (next !== root.selectedProvider)
            GlobalConfig.utilities.vpn.selectedProvider = next;
    }

    function connect(): void {
        if (status.state === "needs-auth" && status.authUrl) {
            emitStatusToast(status);
            return;
        }
        if (!connected && !connecting) {
            connectPending = true;
            connectProc.exec(active.connectCmd);
        }
    }

    function disconnect(): void {
        if (connected && !connecting) {
            disconnectPending = true;
            disconnectProc.exec(active.disconnectCmd);
        }
    }

    function toggle(): void {
        connected ? disconnect() : connect();
    }

    function reportConnectFailure(reason: string): void {
        connectPending = false;
        connected = false;
        connectedChanged();
        if (GlobalConfig.utilities.toasts.vpnChanged)
            Toaster.toast(qsTr("VPN connection failed"), reason, "vpn_key_alert");
    }

    function reportDisconnectFailure(reason: string): void {
        disconnectPending = false;
        connectedChanged();
        if (GlobalConfig.utilities.toasts.vpnChanged)
            Toaster.toast(qsTr("VPN disconnection failed"), reason, "vpn_key_alert");
    }

    function checkStatus(): void {
        if (root.selectedProvider.length > 0) {
            statusProc.running = true;
        }
    }

    function formatBytes(bytes: var): string {
        return Units.formatBytes(bytes);
    }

    function refreshStats(): void {
        if (!connected)
            return;
        const iface = active.interface;
        if (iface.length > 0) {
            // The interface name is user config, so it travels as a positional
            // argument instead of being interpolated into the shell string.
            statsProc.command = ["sh", "-c", 'cat "/sys/class/net/$1/statistics/rx_bytes" "/sys/class/net/$1/statistics/tx_bytes" 2>/dev/null', "--", iface];
            statsProc.running = true;
            // Measure latency over the tunnel by binding the ping to the VPN
            // interface (-I), so the result reflects the VPN path, not the LAN.
            if (!pingProc.running) {
                pingProc.command = ["sh", "-c", 'ping -c1 -W2 -I "$1" 1.1.1.1 2>/dev/null || ping -c1 -W2 1.1.1.1 2>/dev/null', "--", iface];
                pingProc.running = true;
            }
        }
        if (active.serverCmd && serverLocation.length === 0)
            serverProc.exec(active.serverCmd);
    }

    function parseTailscaleStatus(output: string): var {
        const status = {
            connected: false,
            state: "disconnected",
            reason: "",
            authUrl: "",
            server: ""
        };

        if (!output || output.trim().length === 0) {
            return status;
        }

        if (output.includes("Logged out") || output.includes("Stopped") || output.includes("not running") || output.includes("Tailscale is not running")) {
            status.state = "disconnected";
            return status;
        }

        try {
            const data = JSON.parse(output);
            const backendState = data.BackendState || "";

            if (backendState === "Running") {
                status.connected = true;
                status.state = "connected";

                try {
                    const peers = data.Peer || {};
                    for (const key in peers) {
                        const p = peers[key];
                        if (p && p.ExitNode) {
                            status.server = (p.DNSName || p.HostName || "").replace(/\.$/, "");
                            break;
                        }
                    }
                } catch (e2) {}
            } else if (backendState === "Starting") {
                status.state = "connecting";
            } else if (backendState === "NeedsLogin" || backendState === "NeedsMachineAuth") {
                status.state = "needs-auth";
                status.reason = backendState === "NeedsLogin" ? qsTr("Login required") : qsTr("Machine authorization required");
                status.authUrl = data.AuthURL || "";
            }
        } catch (e) {
            if (output.includes("error") || output.includes("Error") || output.includes("failed")) {
                status.state = "disconnected";
                status.reason = qsTr("Tailscale may not be running");
            } else {
                status.state = "disconnected";
            }
        }
        return status;
    }

    function parseNetBirdStatus(output: string): var {
        const status = {
            connected: false,
            state: "disconnected",
            reason: "",
            authUrl: "",
            server: ""
        };
        try {
            const data = JSON.parse(output);
            const mgmtConnected = data.management?.connected;
            const signalConnected = data.signal?.connected;

            if (mgmtConnected && signalConnected) {
                status.connected = true;
                status.state = "connected";
                const url = data.management?.url || data.management?.URL || "";
                if (url)
                    status.server = url.replace(/^https?:\/\//, "").replace(/:\d+$/, "");
            } else if (data.management?.error) {
                const error = data.management.error;
                if (error.includes("auth") || error.includes("login")) {
                    status.state = "needs-auth";
                    status.reason = qsTr("Authentication required");
                } else {
                    status.reason = error;
                }
            }
        } catch (e) {
            status.state = "error";
            status.reason = "Failed to parse status";
        }
        return status;
    }

    function parseWarpStatus(output: string): var {
        const status = {
            connected: false,
            state: "disconnected",
            reason: "",
            authUrl: "",
            server: ""
        };

        if (output.includes("Registration Missing") || output.includes("registration") || output.includes("register") || output.includes("Unable to connect")) {
            status.state = "needs-auth";
            status.reason = "WARP registration required";
        } else if (output.includes("Disconnected")) {
            status.state = "disconnected";
        } else if (output.includes("Connecting")) {
            status.state = "connecting";
        } else if (output.includes("Connected")) {
            status.connected = true;
            status.state = "connected";
        } else {
            status.state = "error";
            status.reason = "Unknown WARP status";
        }
        return status;
    }

    function parseInterfaceStatus(output: string, iface: string): var {
        const status = {
            connected: false,
            state: "disconnected",
            reason: "",
            authUrl: "",
            server: ""
        };

        if (iface && output.includes(iface + ":")) {
            status.connected = true;
            status.state = "connected";
        }
        return status;
    }

    function parseWarpServer(output: string): string {
        const lines = output.split("\n");
        for (const line of lines) {
            const m = line.match(/Endpoint[^\d]*([\d.]+)/i);
            if (m)
                return m[1];
        }
        return "";
    }

    function extractAuthUrl(text: string): string {
        const urlMatch = text.match(/(https?:\/\/[^\s]+)/);
        return urlMatch ? urlMatch[1] : "";
    }

    function createAuthStatus(authUrl: string): var {
        return {
            connected: false,
            state: "needs-auth",
            reason: qsTr("Authentication required"),
            authUrl: authUrl,
            server: ""
        };
    }

    function updateStatus(newStatus: var): void {
        const oldState = status.state;
        if (newStatus.state === "needs-auth" && !newStatus.authUrl && status.authUrl) {
            newStatus.authUrl = status.authUrl;
        }

        status = newStatus;
        root.connected = newStatus.connected;

        root.connectPending = false;
        root.disconnectPending = false;

        if (newStatus.connected && newStatus.server)
            root.serverLocation = newStatus.server;

        if (oldState !== newStatus.state) {
            emitStatusToast(newStatus);
        }

        if (root.autoConnectPending) {
            root.autoConnectPending = false;
            if (!newStatus.connected)
                root.connect();
        }
    }

    function emitStatusToast(statusObj: var): void {
        if (!GlobalConfig.utilities.toasts.vpnChanged)
            return;

        const displayName = active.displayName || "VPN";

        switch (statusObj.state) {
        case "connected":
            Toaster.toast(qsTr("VPN connected"), qsTr("Connected to %1").arg(displayName), "vpn_key");
            break;
        case "disconnected":
            Toaster.toast(qsTr("VPN disconnected"), qsTr("Disconnected from %1").arg(displayName), "vpn_key_off");
            break;
        case "needs-auth":
            const authMsg = statusObj.reason || qsTr("Authentication required");
            Toaster.toast(qsTr("VPN authentication required"), qsTr("%1: %2").arg(displayName).arg(authMsg), "vpn_lock");
            break;
        case "error":
            if (status.state === "connected" || status.state === "connecting" || status.state === "needs-auth") {
                const errMsg = statusObj.reason || "Unknown error";
                Toaster.toast(qsTr("VPN error"), qsTr("%1: %2").arg(displayName).arg(errMsg), "error");
            }
            break;
        }
    }

    function ensureProviderIds(): void {
        for (const provider of root.providers)
            if (provider.id.length === 0)
                provider.id = root.generateId();
    }

    onConnectedChanged: {
        if (connected) {
            if (connectedSince === 0)
                connectedSince = Date.now();
        } else {
            connectedSince = 0;
            bytesIn = "";
            bytesOut = "";
            serverLocation = "";
            pingMs = -1;
        }

        if (pendingSwitchProvider.length === 0 && GlobalConfig.utilities.vpn.enabled !== connected)
            GlobalConfig.utilities.vpn.enabled = connected;

        if (!connected && pendingSwitchProvider.length > 0) {
            const id = pendingSwitchProvider;
            pendingSwitchProvider = "";
            Qt.callLater(() => {
                applySelectedProvider(id);
                root.connect();
            });
        }
    }

    onStatusChanged: {
        if (status.state === "needs-auth" && !registerSent && active.registerCmd) {
            registerSent = true;
            registerProc.exec(active.registerCmd);
        } else if (status.state !== "needs-auth") {
            registerSent = false;
        }
    }

    onProvidersChanged: {
        root.ensureSelection();
    }

    onSelectedProviderChanged: {
        status = {
            connected: false,
            state: "disconnected",
            reason: "",
            authUrl: "",
            server: ""
        };
        root.connected = false;
        root.connectPending = false;
        root.disconnectPending = false;
        root.serverLocation = "";
        root.bytesIn = "";
        root.bytesOut = "";
        root.pingMs = -1;
        statusCheckTimer.start();
    }

    Component.onCompleted: {
        root.ensureProviderIds();
        root.ensureSelection();
        if (root.selectedProvider.length > 0) {
            root.autoConnectPending = GlobalConfig.utilities.vpn.enabled;
            statusCheckTimer.start();
        }
    }


    Adapter {
        id: wireguardAdapter

        name: "wireguard"
        display: iface => iface
        connectCmd: iface => ["pkexec", "wg-quick", "up", iface]
        disconnectCmd: iface => ["pkexec", "wg-quick", "down", iface]
        connectHint: error => error.includes("Unknown device type") || error.includes("Protocol not supported") ? "WireGuard module not loaded. Run: sudo modprobe wireguard" : ""
    }

    Adapter {
        id: warpAdapter

        name: "warp"
        display: "Warp"
        iface: "CloudflareWARP"
        service: "warp-svc"
        connectCmd: ["warp-cli", "connect"]
        disconnectCmd: ["warp-cli", "disconnect"]
        statusCmd: ["warp-cli", "status"]
        parse: out => root.parseWarpStatus(out)
        registerCmd: ["warp-cli", "registration", "new"]
        serverCmd: ["warp-cli", "tunnel", "stats"]
        parseServer: out => root.parseWarpServer(out)
    }

    Adapter {
        id: netbirdAdapter

        name: "netbird"
        display: "NetBird"
        iface: "wt0"
        service: "netbird"
        connectCmd: ["netbird", "up", "--no-browser"]
        disconnectCmd: ["netbird", "down"]
        statusCmd: ["netbird", "status", "--json"]
        parse: out => root.parseNetBirdStatus(out)
    }

    Adapter {
        id: tailscaleAdapter

        name: "tailscale"
        display: "Tailscale"
        iface: "tailscale0"
        service: "tailscaled"
        connectCmd: ["tailscale", "up"]
        disconnectCmd: ["tailscale", "down"]
        statusCmd: ["tailscale", "status", "--json"]
        parse: out => root.parseTailscaleStatus(out)
        connectHint: error => error.includes("Access denied") || error.includes("checkprefs access denied") ? "Permission denied. Run in terminal: sudo tailscale set --operator=$USER" : ""
    }


    Process {
        id: nmMonitor

        running: root.selectedProvider.length > 0
        command: ["nmcli", "monitor"]
        stdout: SplitParser {
            onRead: statusCheckTimer.restart()
        }
    }

    Process {
        id: statusProc

        property int gen: 0

        command: root.active.statusCmd
        onRunningChanged: {
            if (running)
                gen = ++root.statusGen;
        }
        // qmllint disable incompatible-type
        environment: ({
                // qmllint enable incompatible-type
                LANG: "C.UTF-8",
                LC_ALL: "C.UTF-8"
            })
        stdout: StdioCollector {
            onStreamFinished: {
                if (statusProc.gen !== root.statusGen)
                    return; // stale output from a provider that is no longer selected
                const newStatus = root.active.parse(text);
                root.updateStatus(newStatus);
            }
        }
        stderr: StdioCollector {
            onStreamFinished: {
                if (text.trim().length === 0)
                    return;

                const daemonDown = text.includes("doesn't appear to be running") || text.includes("failed to connect") || text.includes("daemon is not running") || (text.includes("not running") && root.active.service);
                if (daemonDown && root.active.service) {
                    root.updateStatus({
                        connected: false,
                        state: "disconnected",
                        reason: `Service not running (run: sudo systemctl start ${root.active.service})`,
                        authUrl: "",
                        server: ""
                    });
                }
            }
        }
    }

    Process {
        id: connectProc

        onRunningChanged: {
            if (running) {
                root.connectExited = false;
                return;
            }

            if (!root.connectExited) {
                console.warn(lc, `Failed to start connect command '${command.join(" ")}'`);
                root.reportConnectFailure(qsTr("Could not start %1. Is it installed?").arg(root.active.displayName));
            }
        }

        onExited: exitCode => { // qmllint disable signal-handler-parameters
            root.connectExited = true;

            // Deferred so an auth URL parsed from the output wins the race.
            Qt.callLater(() => {
                if (root.status.state === "needs-auth")
                    return;

                if (exitCode !== 0) {
                    console.warn(lc, `Connect command '${command.join(" ")}' failed with exit code`, exitCode);
                    root.reportConnectFailure(qsTr("Could not connect to %1").arg(root.active.displayName));
                    return;
                }

                statusCheckTimer.start();
            });
        }
        stdout: SplitParser {
            onRead: data => {
                const authUrl = root.extractAuthUrl(data);
                if (authUrl) {
                    root.updateStatus(root.createAuthStatus(authUrl));
                }
            }
        }
        stderr: StdioCollector {
            onStreamFinished: {
                const error = text.trim();

                const hint = root.active.connectHint ? root.active.connectHint(error) : "";
                if (hint) {
                    root.updateStatus({
                        connected: false,
                        state: "disconnected",
                        reason: hint,
                        authUrl: "",
                        server: ""
                    });
                    return;
                }

                const authUrl = root.extractAuthUrl(error);

                if (authUrl) {
                    root.updateStatus(root.createAuthStatus(authUrl));
                } else if (error.includes("already exists")) {
                    root.connectPending = false;
                    root.connected = true;
                }
            }
        }
    }

    Process {
        id: disconnectProc

        onRunningChanged: {
            if (running) {
                root.disconnectExited = false;
                return;
            }

            if (!root.disconnectExited) {
                console.warn(lc, `Failed to start disconnect command '${command.join(" ")}'`);
                root.reportDisconnectFailure(qsTr("Could not start %1. Is it installed?").arg(root.active.displayName));
            }
        }

        onExited: { // qmllint disable signal-handler-parameters
            root.disconnectExited = true;
            statusCheckTimer.start();
        }
        stderr: StdioCollector {
            onStreamFinished: {
                const error = text.trim();
                if (error && !error.includes("[#]")) {
                    console.warn(lc, "Disconnection error:", error);
                }
            }
        }
    }

    Process {
        id: registerProc

        onExited: exitCode => { // qmllint disable signal-handler-parameters
            if (exitCode === 0) {
                statusCheckTimer.start();
            }
        }
    }

    Process {
        id: statsProc

        stdout: StdioCollector {
            onStreamFinished: {
                const nums = text.trim().split("\n").map(n => parseInt(n.trim(), 10)).filter(n => !isNaN(n));
                if (nums.length >= 2) {
                    root.bytesIn = root.formatBytes(nums[0]);
                    root.bytesOut = root.formatBytes(nums[1]);
                }
            }
        }
    }

    Process {
        id: pingProc

        stdout: StdioCollector {
            onStreamFinished: {
                const m = text.match(/time[=<]\s*([\d.]+)\s*ms/i);
                if (m) {
                    root.pingMs = Math.round(parseFloat(m[1]));
                } else if (root.connected) {
                    root.pingMs = -1;
                }
            }
        }
    }

    Process {
        id: serverProc

        stdout: StdioCollector {
            onStreamFinished: {
                if (root.active.parseServer) {
                    const server = root.active.parseServer(text);
                    if (server)
                        root.serverLocation = server;
                }
            }
        }
    }

    Timer {
        id: statusCheckTimer

        interval: 500
        onTriggered: root.checkStatus()
    }

    LoggingCategory {
        id: lc

        name: "caelestia.qml.services.vpn"
        defaultLogLevel: LoggingCategory.Info
    }

    component Adapter: QtObject {
        required property string name
        property var display
        property string iface
        property string service
        property var connectCmd
        property var disconnectCmd
        property var statusCmd
        property var parse
        property var connectHint
        property var registerCmd
        property var serverCmd
        property var parseServer
    }
}
