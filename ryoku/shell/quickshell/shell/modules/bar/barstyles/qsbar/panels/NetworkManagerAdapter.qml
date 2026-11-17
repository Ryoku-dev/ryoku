import QtQuick
import Quickshell.Networking
import Quickshell.Io
import Ryoku.Ui.Singletons

Item {
    id: adapter

    property var panel: null
    property bool panelOpen: false

    readonly property bool backendAvailable: Networking.backend === NetworkBackendType.NetworkManager
    readonly property var networkDevices: Networking.devices ? Networking.devices.values : []
    readonly property var wifiDevice: findDevice(DeviceType.Wifi)
    readonly property var wifiNetworkObjects: wifiDevice && wifiDevice.networks ? wifiDevice.networks.values : []
    readonly property bool available: backendAvailable && wifiDevice !== null
    readonly property bool wifiBlocked: !Networking.wifiEnabled

    property var networks: []
    property var savedProfiles: []
    property bool profilesLoaded: false
    property bool scanning: false

    function findDevice(type) {
        var devices = networkDevices || []
        for (var i = 0; i < devices.length; i++) {
            if (devices[i] && devices[i].type === type)
                return devices[i]
        }
        return null
    }

    function signalBars(strength) {
        var percent = Math.max(0, Math.min(100, Math.round((strength || 0) * 100)))
        if (percent <= 0)
            return 0
        return Math.max(1, Math.min(4, Math.ceil(percent / 25)))
    }

    // Classify the AP security so the panel can pick the right join flow. Collapsing every
    // non-open type to "psk" (as before) sent enterprise/WEP/OWE/unknown networks through the
    // inline passphrase prompt, which cannot actually authenticate them.
    function securityName(network) {
        if (!network)
            return "unknown"
        switch (network.security) {
        case WifiSecurityType.Open:
            return "open"
        case WifiSecurityType.WpaPsk:
        case WifiSecurityType.Wpa2Psk:
        case WifiSecurityType.Sae:          // WPA/WPA2/WPA3-Personal: a single passphrase
            return "psk"
        case WifiSecurityType.Owe:
            return "owe"                     // enhanced-open: no passphrase, needs NM to negotiate
        case WifiSecurityType.StaticWep:
        case WifiSecurityType.DynamicWep:
            return "wep"                     // legacy key, not a wpa-psk secret
        case WifiSecurityType.Wpa2Eap:
        case WifiSecurityType.WpaEap:
        case WifiSecurityType.Wpa3SuiteB192:
        case WifiSecurityType.Leap:
            return "enterprise"              // 802.1X: identity/cert, never a bare passphrase
        default:
            return "unknown"
        }
    }

    function securityLabel(network) {
        if (!network)
            return I18n.tr("Unknown")
        switch (network.security) {
        case WifiSecurityType.Open:
            return I18n.tr("Open")
        case WifiSecurityType.WpaPsk:
            return I18n.tr("WPA Personal")
        case WifiSecurityType.Wpa2Psk:
            return I18n.tr("WPA2 Personal")
        case WifiSecurityType.Sae:
            return I18n.tr("WPA3 Personal")
        case WifiSecurityType.Owe:
            return I18n.tr("Enhanced Open (OWE)")
        case WifiSecurityType.StaticWep:
        case WifiSecurityType.DynamicWep:
            return "WEP"
        case WifiSecurityType.WpaEap:
            return I18n.tr("WPA Enterprise")
        case WifiSecurityType.Wpa2Eap:
            return I18n.tr("WPA2 Enterprise")
        case WifiSecurityType.Wpa3SuiteB192:
            return I18n.tr("WPA3 Enterprise")
        case WifiSecurityType.Leap:
            return "LEAP"
        default:
            return I18n.tr("Unknown")
        }
    }

    function profileSecurityLabel(keyManagement) {
        switch ((keyManagement || "").toLowerCase()) {
        case "wpa-psk":
            return I18n.tr("WPA Personal profile")
        case "sae":
            return I18n.tr("WPA3 Personal profile")
        case "owe":
            return I18n.tr("Enhanced Open profile")
        case "wpa-eap":
            return I18n.tr("WPA Enterprise profile")
        case "ieee8021x":
            return I18n.tr("802.1X profile")
        case "none":
            return I18n.tr("Open or WEP profile")
        default:
            return I18n.tr("Saved Wi-Fi profile")
        }
    }

    // Only WPA-Personal passphrase types can be joined via network.connectWithPsk(psk).
    // Everything else must go through the full NetworkManager settings flow.
    function isOutOfRange(reason) {
        return reason === ConnectionFailReason.WifiNetworkLost
    }

    function supportsInlinePsk(sec) {
        return sec === "psk"
    }

    function syncNetworks() {
        var nets = []
        var objects = wifiNetworkObjects || []
        for (var i = 0; i < objects.length; i++) {
            var network = objects[i]
            if (!network)
                continue

            nets.push({
                network: network,
                conn: !!network.connected,
                known: !!network.known,
                ssid: network.name || "",
                sec: securityName(network),
                securityLabel: securityLabel(network),
                sig: signalBars(network.signalStrength),
                visible: true,
                profileUuid: "",
                lastSuccessful: 0,
                entryKey: "network:" + (network.name || "") + ":" + securityName(network)
            })
        }

        for (var p = 0; p < savedProfiles.length; p++) {
            var profile = savedProfiles[p]
            var represented = false
            for (var n = 0; n < nets.length; n++) {
                if (nets[n].ssid === profile.name && nets[n].profileUuid === "") {
                    represented = true
                    nets[n].known = true
                    nets[n].profileUuid = profile.uuid
                    nets[n].lastSuccessful = profile.lastSuccessful
                    nets[n].entryKey = "profile:" + profile.uuid
                    break
                }
            }
            if (!represented) {
                nets.push({
                    network: null,
                    conn: false,
                    known: true,
                    ssid: profile.name,
                    sec: "saved",
                    securityLabel: profileSecurityLabel(profile.keyManagement),
                    sig: 0,
                    visible: false,
                    profileUuid: profile.uuid,
                    lastSuccessful: profile.lastSuccessful,
                    entryKey: "profile:" + profile.uuid
                })
            }
        }

        nets.sort(function(a, b) {
            if (a.conn !== b.conn)
                return a.conn ? -1 : 1
            if (a.known !== b.known)
                return a.known ? -1 : 1
            return b.sig - a.sig
        })
        networks = nets
        syncPanel()
    }

    function syncPanel() {
        if (!panel)
            return

        panel.hasWifi = wifiDevice !== null
        panel.wifiBlocked = wifiBlocked
        panel.scanning = scanning
        panel.networks = networks
        panel.known = []
        panel.nmProfilesLoaded = profilesLoaded
    }

    function scan() {
        if (!available || wifiBlocked)
            return

        scanning = true
        if (wifiDevice)
            wifiDevice.scannerEnabled = true
        scanClearTimer.restart()
        syncNetworks()
    }

    function toggleWifi() {
        Networking.wifiEnabled = !Networking.wifiEnabled
        syncPanel()
        if (Networking.wifiEnabled)
            scan()
    }

    function connectTo(entry) {
        if (!entry)
            return

        if (!entry.network && entry.profileUuid) {
            runProfileAction("connect", entry)
            return
        }
        if (!entry.network)
            return

        if (entry.sec === "open" || entry.known || entry.conn) {
            entry.network.connect()
            refreshAfterAction.restart()
            return
        }

        // Only WPA-Personal networks can be joined with an inline passphrase; enterprise, WEP,
        // OWE and unknown types would get an incorrect PSK prompt, so hand them to NM settings.
        if (supportsInlinePsk(entry.sec)) {
            if (panel && panel.root)
                panel.beginNmPassword(entry)
            return
        }

        if (panel) {
            panel.networkActionError = I18n.tr("This network type needs Wi-Fi settings to connect")
            if (panel.root)
                panel.root.networkVisible = false
            if (typeof panel.openWifiSettings === "function")
                panel.openWifiSettings()
        }
    }

    function forgetNetwork(entry) {
        if (!entry || !entry.known)
            return
        if (entry.network && (entry.network.known || !entry.profileUuid)) {
            entry.network.forget()
            refreshAfterAction.restart()
        } else if (entry.profileUuid) {
            runProfileAction("forget", entry)
        }
    }

    function runProfileAction(action, entry) {
        if (!entry || !entry.profileUuid || profileAction.running)
            return
        if (panel)
            panel.networkActionError = ""
        profileAction.action = action
        profileAction.command = action === "connect"
            ? ["nmcli", "--wait", "20", "connection", "up", "uuid", entry.profileUuid]
            : ["nmcli", "--wait", "20", "connection", "delete", "uuid", entry.profileUuid]
        profileAction.running = true
    }

    // a known network keeps its saved secret, so a new passphrase replaces the
    // profile: forget it, then join once NM reports it gone
    property var pskRetryNetwork: null
    property string pskRetryValue: ""
    property int pskRetryTicks: 0
    function connectWithPsk(network, psk) {
        if (!network || psk === "")
            return

        if (network.known) {
            pskRetryNetwork = network
            pskRetryValue = psk
            pskRetryTicks = 0
            network.forget()
            pskRetryTimer.restart()
            return
        }
        network.connectWithPsk(psk)
        refreshAfterAction.restart()
    }
    Timer {
        id: pskRetryTimer
        interval: 150
        repeat: true
        onTriggered: {
            var net = adapter.pskRetryNetwork
            adapter.pskRetryTicks++
            if (net && net.known && adapter.pskRetryTicks < 20)
                return
            stop()
            var psk = adapter.pskRetryValue
            adapter.pskRetryNetwork = null
            adapter.pskRetryValue = ""
            if (!net)
                return
            net.connectWithPsk(psk)
            refreshAfterAction.restart()
        }
    }

    function refresh() {
        if (wifiDevice)
            wifiDevice.scannerEnabled = panelOpen && !wifiBlocked
        if (panelOpen && !profileList.running)
            profileList.running = true
        syncNetworks()
    }

    onPanelOpenChanged: refresh()
    onNetworkDevicesChanged: refresh()
    onWifiDeviceChanged: refresh()
    onWifiNetworkObjectsChanged: syncNetworks()
    onWifiBlockedChanged: refresh()
    onAvailableChanged: refresh()

    Component.onCompleted: refresh()
    Component.onDestruction: {
        if (wifiDevice)
            wifiDevice.scannerEnabled = false
        profileList.running = false
        profileAction.running = false
    }

    Timer {
        id: scanClearTimer
        interval: 1600
        onTriggered: {
            adapter.scanning = false
            adapter.syncPanel()
        }
    }

    Process {
        id: profileList
        // Quickshell exposes saved settings only through currently visible
        // WifiNetwork objects. Query NetworkManager for the missing profiles,
        // using the actual 802.11 SSID rather than the editable connection id.
        command: ["bash", "-c",
            "connections=$(nmcli -t -f UUID,TYPE connection show) || exit 1; " +
            "printf '__READY__\\n'; " +
            "while IFS=: read -r uuid type; do " +
            "case \"$type\" in 802-11-wireless|wifi) ;; *) continue ;; esac; " +
            "mapfile -t details < <(nmcli --escape no -g 802-11-wireless.ssid,connection.timestamp,802-11-wireless-security.key-mgmt connection show uuid \"$uuid\"); " +
            "ssid=${details[0]-}; timestamp=${details[1]:-0}; key_mgmt=${details[2]:-unknown}; " +
            "[ -n \"$ssid\" ] || continue; " +
            "case \"$timestamp\" in ''|*[!0-9]*) timestamp=0 ;; esac; " +
            "[ -n \"$key_mgmt\" ] || key_mgmt=unknown; " +
            "printf '%s\\t%s\\t%s\\t%s\\n' \"$uuid\" \"$ssid\" \"$timestamp\" \"$key_mgmt\"; " +
            "done <<< \"$connections\""]
        stdout: StdioCollector {
            waitForEnd: true
            onStreamFinished: {
                var profiles = []
                var lines = text.trim().split("\n")
                var ready = lines.length > 0 && lines[0] === "__READY__"
                adapter.profilesLoaded = ready
                if (!ready) {
                    adapter.syncPanel()
                    return
                }
                for (var i = 1; i < lines.length; i++) {
                    if (lines[i] === "")
                        continue
                    var fields = lines[i].split("\t")
                    if (fields.length >= 4) {
                        var timestamp = parseInt(fields[fields.length - 2]) || 0
                        profiles.push({
                            uuid: fields[0],
                            name: fields.slice(1, fields.length - 2).join("\t"),
                            lastSuccessful: timestamp,
                            keyManagement: fields[fields.length - 1]
                        })
                    }
                }
                adapter.savedProfiles = profiles
                adapter.syncNetworks()
            }
        }
    }

    Process {
        id: profileAction
        property string action: ""
        stdout: StdioCollector { id: profileActionOut; waitForEnd: true }
        stderr: StdioCollector { id: profileActionErr; waitForEnd: true }
        onExited: function(exitCode, exitStatus) {
            if (exitCode !== 0 && adapter.panel) {
                var message = profileActionErr.text.trim()
                adapter.panel.networkActionError = message !== ""
                    ? message.split("\n")[0]
                    : (action === "connect" ? I18n.tr("Connection failed") : I18n.tr("Could not forget network"))
            }
            if (adapter.panelOpen) {
                profileList.running = false
                profileList.running = true
            }
            refreshAfterAction.restart()
        }
    }

    Timer {
        id: refreshAfterAction
        interval: 1500
        onTriggered: adapter.refresh()
    }
}
