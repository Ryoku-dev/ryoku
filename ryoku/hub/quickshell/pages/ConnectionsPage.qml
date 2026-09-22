pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Shapes
import Quickshell.Io
import Quickshell.Networking
import Quickshell.Bluetooth
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Connections (SYSTEM). A self-contained, full-bleed pluggable page: its own
// head, a bone-pill tab strip (Wi-Fi / Bluetooth / Hotspot), and a Loader that
// swaps between three live subtabs. Every backend is a system tool -- the
// Quickshell Networking/Bluetooth services plus nmcli/bluetoothctl -- so the
// page applies immediately and writes no Ryoku config. The subtab bodies are
// ported verbatim from the old WifiTab/BluetoothTab/HotspotTab: identical
// process wiring, identical logic; only the presentation is recast to
// monochrome. State is by word and inversion, never colour; signal strength is
// drawn in ink bars. The Loader destroys the outgoing tab on every switch, so
// each body re-reads system state on construction and stops its own scan on
// destruction, exactly as the old three-file version did.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true

    // which subtab is showing. transient, not persisted; resets to Wi-Fi on
    // every mount (matches the old page's `property string sub: "wifi"`).
    property string sub: "wifi"

    readonly property var tabs: [
        { "key": "wifi", "label": I18n.tr("Wi-Fi") },
        { "key": "bluetooth", "label": I18n.tr("Bluetooth") },
        { "key": "hotspot", "label": I18n.tr("Hotspot") }
    ]
    readonly property int tabW: 120
    function tabIndex(k) {
        for (var i = 0; i < tabs.length; i++)
            if (tabs[i].key === k)
                return i;
        return 0;
    }

    // Full machine isolation is intentionally outside the Wi-Fi subtab: it
    // blocks every non-loopback path (including Ethernet, VPN, LAN and IPv6).
    // The helper owns the firewall and NetworkManager transition; this page
    // only asks it for a state and renders the result.
    property string killState: "checking"
    property string killError: ""
    property bool killBusy: false
    readonly property bool killActive: killState === "on" || killState === "armed" || killState === "blocked"

    function refreshKillState() {
        if (!killStatus.running)
            killStatus.running = true;
    }

    function toggleKillSwitch() {
        killBusy = true;
        killError = "";
        killSetProc.target = killState === "off" ? "on" : "off";
        killSetProc.running = true;
    }

    Process {
        id: killStatus
        command: ["pkexec", "/usr/bin/ryoku-network-kill", "status"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                var state = this.text.trim();
                if (state === "on" || state === "off" || state === "armed" || state === "blocked") {
                    pg.killState = state;
                    pg.killError = "";
                } else {
                    pg.killState = "unknown";
                    pg.killError = I18n.tr("Could not read the network kill switch.");
                }
        }
        }
        stderr: StdioCollector {}
        onExited: function(exitCode) {
            if (exitCode !== 0) {
                pg.killState = "unknown";
                pg.killError = I18n.tr("Could not read the network kill switch.");
            }
        }
    }

    Process {
        id: killSetProc
        property string target: ""
        command: ["pkexec", "/usr/bin/ryoku-network-kill", target]
        stderr: StdioCollector {}
        onExited: function(exitCode) {
            pg.killBusy = false;
            if (exitCode !== 0)
                pg.killError = I18n.tr("Could not change the network kill switch.");
            pg.refreshKillState();
        }
    }

    // ── shared drawn chrome (monochrome, ink) ──────────────────────────────

    // signal strength as four ascending ink bars. data drawn in ink, sharp
    // pixels: the shape is the point (DESIGN section 4). onBone flips the ramp
    // for a bar sitting on an inverted (connected) row.
    component SignalBars: Item {
        id: sbars
        property int strength: 0
        property bool onBone: false
        implicitWidth: 21
        implicitHeight: 16
        readonly property int filled: Math.max(0, Math.min(4, Math.ceil(strength / 25)))
        readonly property color lit: onBone ? Tokens.inkOnBone : Tokens.ink
        readonly property color dim: onBone ? Tokens.lineOnBone : Tokens.inkFaint
        Repeater {
            model: 4
            Rectangle {
                required property int index
                x: index * 6
                width: 3
                height: 4 + index * 4
                y: sbars.height - height
                radius: Tokens.radius
                antialiasing: false
                color: sbars.filled > index ? sbars.lit : sbars.dim
            }
        }
    }

    // a padlock silhouette for secured networks. redundant with the "Secured"
    // word, kept because it is a listed surface; drawn, not a font glyph.
    component Lock: Item {
        id: lk
        property color tint: Tokens.inkMuted
        implicitWidth: 12
        implicitHeight: 14
        Shape {
            x: 0; y: 0
            width: 12; height: 8
            preferredRendererType: Shape.CurveRenderer
            antialiasing: true
            ShapePath {
                strokeColor: lk.tint
                strokeWidth: 1.4
                fillColor: "transparent"
                capStyle: ShapePath.RoundCap
                joinStyle: ShapePath.RoundJoin
                PathSvg { path: "M2.4 8 V4.2 A3.6 3.6 0 0 1 9.6 4.2 V8" }
            }
        }
        Rectangle {
            x: 0.5; y: 6
            width: 11; height: 8
            radius: 1
            color: lk.tint
        }
    }

    // the Bluetooth rune, verbatim path from the old tab, stroked in ink.
    component BtRune: Item {
        id: bru
        property color tint: Tokens.inkMuted
        property real span: 24
        implicitWidth: bru.span
        implicitHeight: bru.span
        Shape {
            anchors.centerIn: parent
            width: 24; height: 24
            scale: bru.span / 24
            transformOrigin: Item.Center
            preferredRendererType: Shape.CurveRenderer
            antialiasing: true
            ShapePath {
                strokeColor: bru.tint
                strokeWidth: 1.7
                fillColor: "transparent"
                capStyle: ShapePath.RoundCap
                joinStyle: ShapePath.RoundJoin
                PathSvg { path: "M6.5 6.5l11 11L12 23V1l5.5 5.5L6.5 17.5" }
            }
        }
    }

    // wifi glyph for the hotspot card: two arcs over a dot, drawn in ink.
    component WifiGlyph: Item {
        id: wg
        property color tint: Tokens.inkMuted
        implicitWidth: 22
        implicitHeight: 22
        Shape {
            anchors.fill: parent
            preferredRendererType: Shape.CurveRenderer
            antialiasing: true
            ShapePath {
                strokeColor: wg.tint
                strokeWidth: 1.6
                fillColor: "transparent"
                capStyle: ShapePath.RoundCap
                PathSvg { path: "M3 9 Q11 1 19 9" }
            }
            ShapePath {
                strokeColor: wg.tint
                strokeWidth: 1.6
                fillColor: "transparent"
                capStyle: ShapePath.RoundCap
                PathSvg { path: "M6 12 Q11 6.5 16 12" }
            }
        }
        Rectangle {
            width: 3; height: 3
            radius: 1.5
            color: wg.tint
            anchors.horizontalCenter: parent.horizontalCenter
            y: 15
        }
    }

    // the one perpetual animation allowed on an app surface (DESIGN section 5):
    // a 600/600 heartbeat, 1.0 <-> 0.3. used for the pairing pulse.
    component PulseDot: Rectangle {
        id: pd
        property bool on: false
        implicitWidth: 8
        implicitHeight: 8
        radius: 4
        color: Tokens.ink
        visible: on
        SequentialAnimation on opacity {
            running: pd.on
            loops: Animation.Infinite
            NumberAnimation { from: 1; to: 0.3; duration: 600; easing.type: Easing.InOutSine }
            NumberAnimation { from: 0.3; to: 1; duration: 600; easing.type: Easing.InOutSine }
        }
    }

    // a small tappable pill (Pair / Disconnect / Show / Hide). 22 tall so it
    // sits inside a device tile; a shorter sibling of Ryoku.Ui Btn.
    component MiniPill: Rectangle {
        id: mp
        property string text: ""
        property bool armed: true
        signal act()
        implicitHeight: 22
        implicitWidth: mpLab.implicitWidth + 18
        radius: Tokens.radius
        opacity: armed ? 1 : 0.3
        color: mph.hovered && armed ? Tokens.tint10 : "transparent"
        border.width: Tokens.border
        border.color: mph.hovered && armed ? Tokens.lineStrong : Tokens.line
        Behavior on color { ColorAnimation { duration: Tokens.snap } }
        Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
        Text {
            id: mpLab
            anchors.centerIn: parent
            text: mp.text
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            font.weight: Font.Medium
        }
        HoverHandler { id: mph; enabled: mp.armed; cursorShape: Qt.PointingHandCursor }
        TapHandler { enabled: mp.armed; onTapped: mp.act() }
    }

    // an error is inverted text and the word (DESIGN section 1): a small bone
    // chip carrying the failure, no red. used for connect/pair failures.
    component ErrChip: Rectangle {
        id: ec
        property string text: ""
        implicitHeight: 22
        implicitWidth: ecLab.implicitWidth + 16
        radius: Tokens.radius
        color: Tokens.bone
        Text {
            id: ecLab
            anchors.centerIn: parent
            text: ec.text
            color: Tokens.inkOnBone
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            font.weight: Font.Medium
        }
    }

    // an editable label/value row (hotspot credentials). driven entirely by
    // properties + signals so it can live at file scope; the body wires it to
    // hsEdit/hsDraft. tap the value to drop in the inline editor; Enter commits,
    // Esc (or focus loss) cancels.
    component CredRow: Item {
        id: cr
        property string field: ""
        property string label: ""
        property string value: ""
        property string placeholder: ""
        property bool secret: false
        property bool editing: false
        property string draft: ""
        property bool reveal: false
        readonly property bool tooShort: cr.field === "pw" && cr.editing
            && cr.draft.length > 0 && cr.draft.length < 8
        signal beginEdit()
        signal draftEdited(string v)
        signal commit()
        signal cancel()

        width: parent ? parent.width : 0
        implicitHeight: 44 + (cr.tooShort ? hint.implicitHeight + 4 : 0)

        onEditingChanged: if (cr.editing) Qt.callLater(crInput.forceActiveFocus)

        Rectangle {
            id: crField
            anchors { left: parent.left; right: parent.right; top: parent.top }
            height: 44
            radius: Tokens.radius
            color: cr.editing ? Tokens.tint5 : "transparent"
            border.width: cr.editing ? 2 : Tokens.border
            border.color: cr.editing ? Tokens.ink : Tokens.line
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
            Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

            Text {
                id: crLabel
                anchors.left: parent.left
                anchors.leftMargin: Tokens.s4
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr(cr.label)
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                font.weight: Font.Medium
            }

            // read-only value + tap-to-edit affordance.
            Item {
                visible: !cr.editing
                anchors.left: crLabel.right
                anchors.right: (cr.secret && cr.value.length > 0) ? revealPill.left : parent.right
                anchors.rightMargin: Tokens.s4
                anchors.verticalCenter: parent.verticalCenter
                height: parent.height

                Text {
                    id: crValue
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    readonly property bool hidden: cr.secret && !cr.reveal && cr.value.length > 0
                    text: cr.value.length === 0
                        ? (cr.placeholder.length ? cr.placeholder : I18n.tr("tap to set"))
                        : (hidden ? "\u2022".repeat(Math.max(cr.value.length, 8)) : cr.value)
                    color: cr.value.length === 0 ? Tokens.inkFaint : Tokens.ink
                    font.family: cr.secret ? Tokens.mono : Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    font.weight: Font.Medium
                    font.features: ({ "tnum": 1 })
                    elide: Text.ElideLeft
                    maximumLineCount: 1
                }

                HoverHandler { cursorShape: Qt.PointingHandCursor }
                TapHandler { onTapped: cr.beginEdit() }
            }

            MiniPill {
                id: revealPill
                visible: cr.secret && !cr.editing && cr.value.length > 0
                anchors.right: parent.right
                anchors.rightMargin: Tokens.s3
                anchors.verticalCenter: parent.verticalCenter
                text: cr.reveal ? I18n.tr("Hide") : I18n.tr("Show")
                onAct: cr.reveal = !cr.reveal
            }

            TextInput {
                id: crInput
                visible: cr.editing
                anchors.left: crLabel.right
                anchors.leftMargin: Tokens.s3
                anchors.right: parent.right
                anchors.rightMargin: Tokens.s3
                anchors.verticalCenter: parent.verticalCenter
                horizontalAlignment: TextInput.AlignRight
                color: Tokens.ink
                font.family: cr.secret ? Tokens.mono : Tokens.ui
                font.pixelSize: Tokens.fSmall
                font.weight: Font.Medium
                selectByMouse: true
                selectionColor: Tokens.ink
                selectedTextColor: Tokens.inkOnBone
                text: cr.editing ? cr.draft : ""
                onTextEdited: cr.draftEdited(text)
                onAccepted: cr.commit()
                onActiveFocusChanged: if (!activeFocus && cr.editing) cr.commit()
                Keys.onEscapePressed: (event) => { cr.cancel(); event.accepted = true; }

                Text {
                    anchors.fill: parent
                    visible: crInput.text === ""
                    text: cr.field === "pw" ? I18n.tr("8+ characters") : I18n.tr("Network name")
                    color: Tokens.inkMuted
                    font: crInput.font
                    horizontalAlignment: Text.AlignRight
                    verticalAlignment: Text.AlignVCenter
                }
            }
        }

        // live guidance while typing a too-short password. muted, not a
        // failure: real rejection happens silently in commitHotspotEdit.
        Text {
            id: hint
            visible: cr.tooShort
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s4
            anchors.top: crField.bottom
            anchors.topMargin: 4
            text: I18n.tr("WPA2 needs at least 8 characters")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            font.weight: Font.Medium
        }
    }

    // ── Wi-Fi subtab ────────────────────────────────────────────────────────

    // master on/off, rescan (spins ~10s), live list sorted by signal. security
    // + known-profile ground truth come from nmcli; the Quickshell service
    // doesn't expose them. tap a secured unknown net -> inline password row that
    // runs `nmcli --ask dev wifi connect`, secret piped through stdin so it
    // never lands in /proc/<pid>/cmdline.
    component WifiBody: Item {
        id: wifi

        property bool active: true

        readonly property var devices: (typeof Networking !== "undefined" && Networking && Networking.devices) ? Networking.devices.values : []
        readonly property var wifiDev: devices.find(function(d) { return d && d.type === DeviceType.Wifi }) || null
        readonly property bool wifiOn: (typeof Networking !== "undefined" && Networking) ? Networking.wifiEnabled : false
        readonly property var nets: (wifiDev && wifiDev.networks) ? wifiDev.networks.values : []
        readonly property var netsSorted: nets
            .slice()
            .filter(function(n) { return n && n.name && n.name.length > 0; })
            .sort(function(a, b) {
                return ((b ? b.signalStrength : 0) || 0) - ((a ? a.signalStrength : 0) || 0);
            })

        property var securityMap: ({})
        property var knownProfiles: ({})

        // per-saved-network autoconnect, read live from `nmcli`. a profile
        // rejoins on its own by default; turning it off stops the machine
        // associating with that SSID unprompted. autoBusy gates the toggle
        // while nmcli rewrites the profile.
        property var autoconnectMap: ({})
        property bool autoBusy: false

        // dual and tri-band SSIDs expose one BSSID per band. bandMap keys each
        // SSID to its per-band BSSIDs so a join can pin a band; selectedBands is
        // the user's per-SSID pick ("" means let NM choose, which favours the
        // 2.4 GHz AP at any range).
        property var bandMap: ({})
        property var selectedBands: ({})

        property string expandedSsid: ""
        property bool connecting: false
        property bool connectFailed: false
        property bool scanning: false

        // NetworkManager's Wi-Fi backend, read live from `ryoku-wifi-backend get`.
        // iwd is the default; wpa_supplicant is the escape hatch for WPA3 / phone
        // hotspots iwd stumbles on. backendBusy gates the segment while pkexec
        // rewrites the NM drop-in and restarts the service.
        property string wifiBackend: "iwd"
        property bool backendBusy: false

        // NM rescan = fresh model array, so the delegate tears down mid-typing.
        // draft lives on the body; the password field re-fills from it on rebuild.
        property string pwDraft: ""
        property string pendingPw: ""
        property string attemptSsid: ""
        property bool attemptWasKnown: false

        readonly property real colMax: 720

        function isSecured(ssid) {
            var sec = wifi.securityMap[ssid];
            return sec !== undefined && sec !== "" && sec !== "--";
        }

        function refresh() {
            secProc.running = true;
            profProc.running = true;
        }

        // flip a saved profile's autoconnect. the profile name equals the SSID
        // for a network the row reads as saved (nmcli names it after the SSID),
        // so the same key targets it. refresh re-reads the map on completion.
        function setAutoconnect(ssid, enabled) {
            if (wifi.autoBusy || !ssid.length)
                return;
            wifi.autoBusy = true;
            autoProc.command = ["nmcli", "connection", "modify", ssid, "connection.autoconnect", enabled ? "yes" : "no"];
            autoProc.running = true;
        }

        // flip the NM backend. no-op if unchanged or already switching; the
        // shipped polkit rule lets wheel run this via pkexec with no prompt.
        function setBackend(choice) {
            if (wifi.backendBusy || choice === wifi.wifiBackend)
                return;
            wifi.backendBusy = true;
            backendSetProc.command = ["pkexec", "ryoku-wifi-backend", choice];
            backendSetProc.running = true;
        }

        // split one `nmcli -t` line into its fields at unescaped colons, undoing
        // nmcli's escaping (`\:` for a colon inside a field, `\\` for a
        // backslash). a BSSID carries five colons, so the old last-colon split
        // could never reach the fields past it.
        function terseFields(line) {
            var fields = [];
            var cur = "";
            for (var i = 0; i < line.length; i++) {
                var c = line[i];
                if (c === "\\" && i + 1 < line.length && (line[i + 1] === ":" || line[i + 1] === "\\")) {
                    cur += line[i + 1];
                    i++;
                } else if (c === ":") {
                    fields.push(cur);
                    cur = "";
                } else {
                    cur += c;
                }
            }
            fields.push(cur);
            return fields;
        }

        // 2.4 / 5 / 6 GHz bucket for a channel frequency in MHz (contract A);
        // "" when the frequency is unknown.
        function bandForFreq(mhz) {
            if (!mhz || mhz <= 0)
                return "";
            if (mhz < 2500)
                return "2.4";
            if (mhz < 5925)
                return "5";
            return "6";
        }

        function bandLabel(b) {
            if (b === "6")
                return I18n.tr("6 GHz");
            if (b === "5")
                return I18n.tr("5 GHz");
            return I18n.tr("2.4 GHz");
        }

        // the BSSID of the band the user picked for this SSID, or "" when none
        // is picked (keep NM's own strongest-BSS choice).
        function pickedBssid(ssid) {
            var band = wifi.selectedBands[ssid];
            if (!band)
                return "";
            var entries = wifi.bandMap[ssid];
            if (!entries)
                return "";
            for (var i = 0; i < entries.length; i++)
                if (entries[i].band === band)
                    return entries[i].bssid;
            return "";
        }

        // record the per-SSID band pick as a fresh object so the chips re-read it.
        function selectBand(ssid, band) {
            var next = {};
            for (var k in wifi.selectedBands)
                next[k] = wifi.selectedBands[k];
            next[ssid] = band;
            wifi.selectedBands = next;
        }

        // row click: connected -> disconnect, known or open -> connect, else
        // expand the inline password row.
        function activateNetwork(net) {
            if (!net)
                return;
            var ssid = net.name || "";
            if (wifi.expandedSsid === ssid) {
                wifi.expandedSsid = "";
                return;
            }
            if (net.connected) {
                if (typeof net.disconnect === "function")
                    net.disconnect();
                return;
            }
            var secKnown = wifi.securityMap[ssid] !== undefined;
            if (wifi.knownProfiles[ssid] === true || (secKnown && !wifi.isSecured(ssid))) {
                wifi.expandedSsid = "";
                var bssid = wifi.pickedBssid(ssid);
                if (bssid.length) {
                    wifi.connectBand(ssid, bssid);
                    return;
                }
                if (typeof net.connect === "function")
                    net.connect();
                wifi.refresh();
                return;
            }
            wifi.connectFailed = false;
            wifi.pwDraft = "";
            wifi.expandedSsid = ssid;
        }

        // `nmcli --ask`, password through stdin. /proc/<pid>/cmdline is world-
        // readable for the whole attempt, so it MUST NOT be in argv.
        function connectWithPassword(ssid, pw) {
            if (connProc.running || !pw.length)
                return;
            wifi.connecting = true;
            wifi.connectFailed = false;
            wifi.attemptSsid = ssid;
            wifi.attemptWasKnown = wifi.knownProfiles[ssid] === true;
            wifi.pendingPw = pw;
            // a picked band joins its exact BSSID; an empty pick keeps the SSID
            // so NM chooses the strongest AP (today's behaviour).
            var bssid = wifi.pickedBssid(ssid);
            connProc.command = ["nmcli", "--ask", "dev", "wifi", "connect", bssid.length ? bssid : ssid];
            connProc.running = true;
        }

        // known/open join pinned to a band's BSSID. nmcli's connect target
        // accepts a BSSID, so this reaches the exact AP instead of NM's
        // strongest pick; it shares connProc's success and cleanup wiring and
        // needs no password.
        function connectBand(ssid, bssid) {
            if (connProc.running)
                return;
            wifi.connecting = true;
            wifi.connectFailed = false;
            wifi.attemptSsid = ssid;
            wifi.attemptWasKnown = wifi.knownProfiles[ssid] === true;
            wifi.pendingPw = "";
            connProc.command = ["nmcli", "dev", "wifi", "connect", bssid];
            connProc.running = true;
        }

        // reload pulse. forces an nmcli rescan and spins the button up to 10s.
        function startScan() {
            if (!wifi.wifiOn)
                return;
            wifi.scanning = true;
            rescanProc.running = true;
            scanTimer.restart();
        }

        function stopScan() {
            wifi.scanning = false;
            scanTimer.stop();
        }

        Component.onCompleted: { wifi.refresh(); backendGetProc.running = true; }

        onWifiOnChanged: if (!wifi.wifiOn) wifi.stopScan()

        Binding {
            target: wifi.wifiDev
            property: "scannerEnabled"
            value: wifi.active && wifi.wifiOn
            when: wifi.wifiDev !== null
        }

        Timer {
            id: scanTimer
            interval: 10000
            onTriggered: wifi.stopScan()
        }

        Process {
            id: rescanProc
            command: ["nmcli", "dev", "wifi", "rescan"]
        }

        Process {
            id: secProc
            command: ["nmcli", "-t", "-f", "SSID,SECURITY,FREQ,BSSID", "dev", "wifi", "list"]
            stdout: StdioCollector {
                onStreamFinished: {
                    var secs = {};
                    var bands = {};
                    var lines = this.text.split("\n");
                    for (var i = 0; i < lines.length; i++) {
                        if (!lines[i].length)
                            continue;
                        var f = wifi.terseFields(lines[i]);
                        if (f.length < 4 || !f[0].length)
                            continue;
                        secs[f[0]] = f[1];
                        var mhz = parseInt(f[2], 10);
                        var band = wifi.bandForFreq(mhz);
                        if (!band.length)
                            continue;
                        var entries = bands[f[0]] || [];
                        var seen = false;
                        for (var j = 0; j < entries.length; j++)
                            if (entries[j].band === band) {
                                seen = true;
                                break;
                            }
                        // nmcli lists strongest first, so the first BSSID kept
                        // for a band is that band's strongest AP.
                        if (!seen)
                            entries.push({ band: band, bssid: f[3], freq: mhz });
                        bands[f[0]] = entries;
                    }
                    for (var s in bands)
                        bands[s].sort(function(a, b) { return a.freq - b.freq; });
                    wifi.securityMap = secs;
                    wifi.bandMap = bands;
                }
            }
        }

        Process {
            id: profProc
            command: ["nmcli", "-t", "-f", "NAME,TYPE,AUTOCONNECT", "connection", "show"]
            stdout: StdioCollector {
                onStreamFinished: {
                    var set = {};
                    var auto = {};
                    var lines = this.text.split("\n");
                    for (var i = 0; i < lines.length; i++) {
                        var f = wifi.terseFields(lines[i]);
                        if (f.length >= 3 && f[0].length && f[1] === "802-11-wireless") {
                            set[f[0]] = true;
                            auto[f[0]] = f[2] === "yes";
                        }
                    }
                    wifi.knownProfiles = set;
                    wifi.autoconnectMap = auto;
                }
            }
        }

        Process {
            id: connProc
            stdinEnabled: true
            stdout: StdioCollector {}
            stderr: StdioCollector {}
            onStarted: {
                write(wifi.pendingPw + "\n");
                wifi.pendingPw = "";
            }
            onExited: function(exitCode) {
                wifi.connecting = false;
                if (exitCode === 0) {
                    wifi.expandedSsid = "";
                    wifi.pwDraft = "";
                    wifi.connectFailed = false;
                    wifi.refresh();
                } else {
                    wifi.connectFailed = true;
                    if (!wifi.attemptWasKnown && wifi.attemptSsid.length) {
                        cleanupProc.command = ["nmcli", "connection", "delete", "id", wifi.attemptSsid];
                        cleanupProc.running = true;
                    }
                }
            }
        }

        // a failed `nmcli dev wifi connect` leaves a profile named after the
        // SSID (nmcli names it after the SSID even when the join used a BSSID),
        // so the cleanup keys on attemptSsid, not the BSSID. without deleting it
        // the next click reads the network as known and silently fails forever.
        Process {
            id: cleanupProc
            onExited: wifi.refresh()
        }

        // apply an autoconnect flip, then re-read the profiles so the toggle
        // reflects the stored state rather than the click.
        Process {
            id: autoProc
            stdout: StdioCollector {}
            stderr: StdioCollector {}
            onExited: {
                wifi.autoBusy = false;
                wifi.refresh();
            }
        }

        onNetsChanged: if (wifi.active) secRefresh.restart()

        Timer {
            id: secRefresh
            interval: 1200
            onTriggered: if (wifi.active) secProc.running = true
        }

        // read the current NM Wi-Fi backend (read-only, unprivileged).
        Process {
            id: backendGetProc
            command: ["ryoku-wifi-backend", "get"]
            stdout: StdioCollector {
                onStreamFinished: {
                    var v = this.text.trim();
                    if (v === "iwd" || v === "wpa_supplicant")
                        wifi.wifiBackend = v;
                }
            }
        }

        // apply the switch via pkexec; the polkit rule grants wheel passwordless.
        // NetworkManager restarts, so on exit re-read state and, once it settles,
        // refresh + rescan so the list repopulates on the new backend.
        Process {
            id: backendSetProc
            stdout: StdioCollector {}
            stderr: StdioCollector {}
            onExited: {
                wifi.backendBusy = false;
                backendGetProc.running = true;
                backendSettle.restart();
            }
        }

        Timer {
            id: backendSettle
            interval: 1500
            onTriggered: {
                wifi.refresh();
                wifi.startScan();
            }
        }

        Item {
            id: wifiContent
            // the list reads as one column; with the poster that used to fill the
            // right side gone, that column is centred rather than pinned left.
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            anchors.horizontalCenter: parent.horizontalCenter
            width: Math.min(parent.width, wifi.colMax)

            // header row: "WI-FI" label + hairline + scan button.
            Item {
                id: wifiBar
                anchors.top: parent.top
                anchors.left: parent.left
                anchors.right: parent.right
                height: 40

                Text {
                    id: wifiSecLabel
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("WI-FI")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackMark
                }

                Rectangle {
                    anchors.left: wifiSecLabel.right
                    anchors.leftMargin: Tokens.s3
                    anchors.right: scanBtn.visible ? scanBtn.left : parent.right
                    anchors.rightMargin: scanBtn.visible ? Tokens.s3 : 0
                    anchors.verticalCenter: parent.verticalCenter
                    height: 1
                    color: Tokens.lineSoft
                }

                Btn {
                    id: scanBtn
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    visible: wifi.wifiOn
                    text: wifi.scanning ? I18n.tr("Scanning\u2026") : I18n.tr("Scan")
                    armed: !wifi.scanning
                    onAct: wifi.startScan()
                }
            }

            // master on/off. mirrors NM live state; no draft, applies at once.
            Item {
                id: wifiToggleRow
                anchors.top: wifiBar.bottom
                anchors.topMargin: Tokens.s3
                anchors.left: parent.left
                anchors.right: parent.right
                height: 34

                Text {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Wi-Fi")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow
                    font.weight: Font.Medium
                }

                Sw {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    on: wifi.wifiOn
                    onToggled: (v) => {
                        if (typeof Networking !== "undefined" && Networking)
                            Networking.wifiEnabled = v;
                    }
                }
            }

            Rectangle {
                id: wifiDivider
                anchors.top: wifiToggleRow.bottom
                anchors.topMargin: Tokens.s2
                anchors.left: parent.left
                anchors.right: parent.right
                height: 1
                color: Tokens.lineSoft
            }

            // empty states.
            Text {
                anchors.top: wifiDivider.bottom
                anchors.topMargin: Tokens.s6
                anchors.horizontalCenter: parent.horizontalCenter
                visible: !wifi.wifiOn
                text: I18n.tr("Wi-Fi is off.")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                font.weight: Font.Medium
            }

            Text {
                anchors.top: wifiDivider.bottom
                anchors.topMargin: Tokens.s6
                anchors.horizontalCenter: parent.horizontalCenter
                visible: wifi.wifiOn && wifi.netsSorted.length === 0
                text: I18n.tr("Searching networks\u2026")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                font.weight: Font.Medium
            }

            // live network list.
            Flickable {
                id: netFlick
                anchors.top: wifiDivider.bottom
                anchors.topMargin: Tokens.s2
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: backendRow.top
                anchors.bottomMargin: Tokens.s2
                visible: wifi.wifiOn && wifi.netsSorted.length > 0
                contentWidth: width
                contentHeight: netCol.implicitHeight + 16
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }

                Column {
                    id: netCol
                    width: netFlick.width - Tokens.s3
                    topPadding: 2
                    spacing: 4

                    Repeater {
                        model: wifi.netsSorted

                        delegate: Column {
                            id: netItem
                            required property var modelData

                            readonly property string ssid: (modelData && modelData.name) ? modelData.name : ""
                            readonly property bool isActive: modelData ? modelData.connected === true : false
                            readonly property bool secured: wifi.isSecured(ssid)
                            readonly property bool known: wifi.knownProfiles[ssid] === true
                            readonly property bool expanded: ssid.length > 0 && wifi.expandedSsid === ssid
                            // Quickshell reports signalStrength as a 0..1 ratio;
                            // the bars and the % readout speak 0..100.
                            readonly property int strength: modelData ? Math.round((modelData.signalStrength || 0) * 100) : 0

                            width: netCol.width
                            spacing: 4

                            function syncPwField() {
                                pwField.text = wifi.pwDraft;
                                pwField.cursorPosition = pwField.text.length;
                                pwField.forceActiveFocus();
                            }

                            onExpandedChanged: if (expanded) Qt.callLater(syncPwField)
                            Component.onCompleted: if (expanded) Qt.callLater(syncPwField)

                            // the row. connected = the ON member of the exclusive
                            // Wi-Fi set, so it inverts to bone (DESIGN section 1).
                            Rectangle {
                                id: rowBg
                                width: parent.width
                                height: 46
                                radius: Tokens.radius
                                color: netItem.isActive
                                    ? Tokens.bone
                                    : (rowHover.hovered ? Tokens.tint5 : "transparent")
                                Behavior on color { ColorAnimation { duration: Tokens.snap } }

                                HoverHandler { id: rowHover; cursorShape: Qt.PointingHandCursor }
                                TapHandler { onTapped: wifi.activateNetwork(netItem.modelData) }

                                SignalBars {
                                    id: bars
                                    anchors.left: parent.left
                                    anchors.leftMargin: Tokens.s4
                                    anchors.verticalCenter: parent.verticalCenter
                                    strength: netItem.strength
                                    onBone: netItem.isActive
                                }

                                Column {
                                    anchors.left: bars.right
                                    anchors.leftMargin: Tokens.s3
                                    anchors.right: rowRight.left
                                    anchors.rightMargin: Tokens.s3
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: 1

                                    Text {
                                        width: parent.width
                                        text: netItem.ssid
                                        color: netItem.isActive ? Tokens.inkOnBone : Tokens.ink
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fBody
                                        font.weight: netItem.isActive ? Font.DemiBold : Font.Medium
                                        elide: Text.ElideRight
                                    }

                                    Text {
                                        width: parent.width
                                        text: netItem.isActive
                                            ? I18n.tr("Connected")
                                            : (netItem.known
                                                ? (netItem.secured ? I18n.tr("Saved \u00b7 Secured") : I18n.tr("Saved \u00b7 Open"))
                                                : (netItem.secured ? I18n.tr("Secured") : I18n.tr("Open")))
                                        color: netItem.isActive ? Tokens.inkOnBoneDim : Tokens.inkMuted
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fMicro
                                        font.weight: Font.Medium
                                        elide: Text.ElideRight
                                    }
                                }

                                Row {
                                    id: rowRight
                                    anchors.right: parent.right
                                    anchors.rightMargin: Tokens.s4
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s2

                                    Lock {
                                        anchors.verticalCenter: parent.verticalCenter
                                        visible: netItem.secured
                                        tint: netItem.isActive ? Tokens.inkOnBone : Tokens.inkMuted
                                    }

                                    Text {
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: netItem.strength + "%"
                                        color: netItem.isActive ? Tokens.inkOnBoneDim : Tokens.inkMuted
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fMicro
                                        font.weight: Font.Medium
                                        font.features: ({ "tnum": 1 })
                                    }
                                }
                            }

                            // band selector. a dual or tri-band SSID resolves to
                            // one BSSID per band, and NM's SSID join takes the
                            // strongest, which is the 2.4 GHz AP at any range.
                            // picking a band pins the join to that band's BSSID;
                            // single-band rows collapse this to nothing.
                            Item {
                                id: bandRow
                                readonly property var entries: wifi.bandMap[netItem.ssid] || []
                                width: parent.width
                                height: entries.length > 1 ? 30 : 0
                                clip: true
                                visible: height > 0.5

                                Row {
                                    anchors.left: parent.left
                                    anchors.leftMargin: Tokens.s4
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s2

                                    Repeater {
                                        model: bandRow.entries

                                        delegate: Btn {
                                            required property var modelData
                                            // the row by id, not `parent`: a Repeater delegate has no
                                            // parent yet while it is being created, so anchoring to it
                                            // logged a TypeError for every band chip built.
                                            anchors.verticalCenter: bandRow.verticalCenter
                                            compact: true
                                            text: wifi.bandLabel(modelData.band)
                                            primary: wifi.selectedBands[netItem.ssid] === modelData.band
                                            armed: !wifi.connecting
                                            onAct: wifi.selectBand(netItem.ssid,
                                                wifi.selectedBands[netItem.ssid] === modelData.band ? "" : modelData.band)
                                        }
                                    }
                                }
                            }

                            // autoconnect toggle. a saved profile rejoins on its
                            // own by default; turning it off stops the machine
                            // associating with this SSID unprompted, so a spoofed
                            // AP can no longer pull the device in. saved rows only.
                            Item {
                                id: autoRow
                                readonly property bool on: wifi.autoconnectMap[netItem.ssid] !== false
                                width: parent.width
                                height: netItem.known ? 30 : 0
                                clip: true
                                visible: height > 0.5

                                Row {
                                    anchors.left: parent.left
                                    anchors.leftMargin: Tokens.s4
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s2

                                    Text {
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: I18n.tr("Auto-reconnect")
                                        color: Tokens.inkMuted
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fMicro
                                        font.weight: Font.Medium
                                    }

                                    Btn {
                                        anchors.verticalCenter: parent.verticalCenter
                                        compact: true
                                        text: autoRow.on ? I18n.tr("On") : I18n.tr("Off")
                                        primary: autoRow.on
                                        armed: !wifi.autoBusy
                                        onAct: wifi.setAutoconnect(netItem.ssid, !autoRow.on)
                                    }
                                }
                            }

                            // password row. secured + unknown only.
                            Item {
                                width: parent.width
                                height: netItem.expanded ? 44 : 0
                                clip: true
                                visible: height > 0.5
                                opacity: netItem.expanded ? 1 : 0
                                Behavior on height { NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease } }
                                Behavior on opacity { NumberAnimation { duration: Tokens.snap } }

                                Rectangle {
                                    id: pwBg
                                    anchors.left: parent.left
                                    anchors.leftMargin: Tokens.s4
                                    anchors.right: pwRight.left
                                    anchors.rightMargin: Tokens.s3
                                    anchors.verticalCenter: parent.verticalCenter
                                    height: 32
                                    radius: Tokens.radius
                                    color: "transparent"
                                    border.width: pwField.activeFocus ? 2 : Tokens.border
                                    border.color: pwField.activeFocus ? Tokens.ink : Tokens.line
                                    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

                                    TextInput {
                                        id: pwField
                                        anchors.fill: parent
                                        anchors.leftMargin: Tokens.s3
                                        anchors.rightMargin: Tokens.s3
                                        verticalAlignment: TextInput.AlignVCenter
                                        color: Tokens.ink
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fSmall
                                        echoMode: TextInput.Password
                                        selectByMouse: true
                                        selectionColor: Tokens.ink
                                        selectedTextColor: Tokens.inkOnBone
                                        onTextEdited: wifi.pwDraft = text
                                        onAccepted: wifi.connectWithPassword(netItem.ssid, text)

                                        Text {
                                            anchors.fill: parent
                                            visible: pwField.text === ""
                                            text: I18n.tr("Password")
                                            color: Tokens.inkMuted
                                            font: pwField.font
                                            verticalAlignment: Text.AlignVCenter
                                        }
                                    }
                                }

                                Row {
                                    id: pwRight
                                    anchors.right: parent.right
                                    anchors.rightMargin: Tokens.s4
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s2

                                    IconBtn {
                                        anchors.verticalCenter: parent.verticalCenter
                                        glyph: "\u00d7"
                                        onAct: wifi.expandedSsid = ""
                                    }

                                    Text {
                                        anchors.verticalCenter: parent.verticalCenter
                                        visible: wifi.connecting
                                        text: I18n.tr("Connecting\u2026")
                                        color: Tokens.inkMuted
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fMicro
                                        font.weight: Font.Medium
                                    }

                                    Btn {
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: I18n.tr("Connect")
                                        primary: true
                                        armed: !wifi.connecting && pwField.text.length > 0
                                        onAct: wifi.connectWithPassword(netItem.ssid, pwField.text)
                                    }
                                }
                            }

                            Item {
                                width: parent.width
                                height: (netItem.expanded && wifi.connectFailed) ? 24 : 0
                                clip: true
                                visible: height > 0.5
                                Behavior on height { NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease } }

                                ErrChip {
                                    anchors.left: parent.left
                                    anchors.leftMargin: Tokens.s4
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: I18n.tr("Connection failed")
                                }
                            }
                        }
                    }
                }
            }

            // advanced: NetworkManager's Wi-Fi backend. an unobtrusive footer
            // beneath the list; the selected engine inverts to bone, the other is
            // tappable. rare enough to sit out of the way, but reachable when a
            // WPA3 AP or phone hotspot won't associate under iwd.
            Item {
                id: backendRow
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                height: 54

                Rectangle {
                    anchors.top: parent.top
                    anchors.left: parent.left
                    anchors.right: parent.right
                    height: 1
                    color: Tokens.lineSoft
                }

                Column {
                    anchors.left: parent.left
                    anchors.right: backendSeg.left
                    anchors.rightMargin: Tokens.s3
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 2

                    Text {
                        text: I18n.tr("Wi-Fi backend")
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fRow
                        font.weight: Font.Medium
                    }

                    Text {
                        width: parent.width
                        text: I18n.tr("Switch to wpa_supplicant if a WPA3 network or Android hotspot won\u2019t connect.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.weight: Font.Medium
                        wrapMode: Text.WordWrap
                    }
                }

                Row {
                    id: backendSeg
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s2

                    Btn {
                        anchors.verticalCenter: parent.verticalCenter
                        compact: true
                        text: I18n.tr("iwd")
                        primary: wifi.wifiBackend === "iwd"
                        armed: !wifi.backendBusy
                        onAct: wifi.setBackend("iwd")
                    }

                    Btn {
                        anchors.verticalCenter: parent.verticalCenter
                        compact: true
                        text: I18n.tr("wpa_supplicant")
                        primary: wifi.wifiBackend === "wpa_supplicant"
                        armed: !wifi.backendBusy
                        onAct: wifi.setBackend("wpa_supplicant")
                    }
                }
            }
        }
    }

    // ── Bluetooth subtab ─────────────────────────────────────────────────────

    // adapter toggle, scan with 25s auto-stop, live device list. known devices
    // use Quickshell's connect/disconnect; unpaired ones run bluetoothctl
    // pair-trust-connect with a pairing pulse and a transient failure chip.
    component BtBody: Item {
        id: bt

        readonly property var adapter: (typeof Bluetooth !== "undefined" && Bluetooth) ? Bluetooth.defaultAdapter : null
        readonly property var devices: (typeof Bluetooth !== "undefined" && Bluetooth && Bluetooth.devices) ? Bluetooth.devices.values : []
        readonly property bool adapterOn: adapter ? adapter.enabled === true : false
        readonly property bool discovering: adapter ? adapter.discovering === true : false
        readonly property bool hasAdapter: adapter !== null
        // rfkill (airplane mode) blocks the radio at the kernel; BlueZ then
        // refuses Powered=true. surfaced so the toggle can unblock first.
        readonly property bool blocked: (adapter && typeof BluetoothAdapterState !== "undefined")
            ? adapter.state === BluetoothAdapterState.Blocked : false
        readonly property int connectedCount: {
            var n = 0;
            for (var i = 0; i < devices.length; i++)
                if (devices[i] && devices[i].connected)
                    n++;
            return n;
        }

        // BlueZ hands the cache out unordered. connected first, then paired,
        // then named, nameless MACs last; a scan shouldn't churn useful rows.
        readonly property var devicesSorted: devices.slice().sort(function(a, b) {
            function rank(d) {
                if (!d) return 3;
                if (d.connected) return 0;
                if (d.paired) return 1;
                return (d.name && d.name.length) ? 2 : 3;
            }
            var r = rank(a) - rank(b);
            if (r !== 0) return r;
            return String((a && a.name) || "").localeCompare(String((b && b.name) || ""));
        })

        property string pairingAddress: ""
        property string failedAddress: ""
        property string serviceError: ""

        function metaFor(d) {
            if (!d) return "";
            var parts = [];
            if (d.connected) parts.push("connected");
            else if (d.paired) parts.push("paired");
            if (d.state !== undefined && typeof BluetoothDeviceState !== "undefined") {
                var st = BluetoothDeviceState.toString(d.state);
                if (st && st.length > 0 && parts.indexOf(st.toLowerCase()) === -1)
                    parts.push(st.toLowerCase());
            }
            return parts.join(" \u00b7 ");
        }

        function batteryLevel(d) {
            if (!d || d.battery === undefined || d.battery === null) return -1;
            var b = d.battery;
            if (b <= 0) return -1;
            if (b <= 1) b = b * 100;
            return Math.round(b);
        }

        // row click: disconnect if connected, connect if paired, else run the
        // bluetoothctl pair-trust-connect flow.
        function activateDevice(d) {
            if (!d)
                return;
            if (d.connected) {
                if (typeof d.disconnect === "function")
                    d.disconnect();
                return;
            }
            if (d.paired) {
                if (typeof d.connect === "function")
                    d.connect();
                return;
            }
            bt.pairDevice(d);
        }

        function pairDevice(d) {
            if (!d || !d.address || pairProc.running)
                return;
            bt.pairingAddress = d.address;
            bt.failedAddress = "";
            pairProc.command = ["sh", "-c",
                'timeout 30 bluetoothctl pair "$1" && bluetoothctl trust "$1" && timeout 30 bluetoothctl connect "$1"',
                "sh", d.address];
            pairProc.running = true;
        }

        // one entry point for the adapter toggle. a blocked radio is unblocked
        // first (/dev/rfkill is seat-writable via systemd uaccess, no root), and
        // powered on when the unblock lands; everything else is a plain flip.
        function setAdapterEnabled(v) {
            if (!bt.adapter)
                return;
            if (v && (bt.blocked || unblockProc.running)) {
                if (!unblockProc.running)
                    unblockProc.running = true;
                return;
            }
            bt.adapter.enabled = v;
        }

        function startService() {
            if (svcProc.running)
                return;
            bt.serviceError = "";
            svcProc.running = true;
        }

        // leaving the subtab (or closing the hub) mid-scan stops discovery so
        // BlueZ isn't left chewing the radio in the background.
        Component.onDestruction: {
            scanTimer.stop();
            if (bt.adapter && bt.adapter.discovering)
                bt.adapter.discovering = false;
        }

        Timer {
            id: scanTimer
            interval: 25000
            repeat: false
            onTriggered: if (bt.adapter) bt.adapter.discovering = false
        }

        Timer {
            id: failTimer
            interval: 4000
            repeat: false
            onTriggered: bt.failedAddress = ""
        }

        Process {
            id: pairProc
            stdout: StdioCollector {}
            stderr: StdioCollector {}
            onExited: function(exitCode) {
                var addr = bt.pairingAddress;
                bt.pairingAddress = "";
                if (exitCode !== 0) {
                    bt.failedAddress = addr;
                    failTimer.restart();
                }
            }
        }

        // rfkill unblock, then power the adapter once the radio is free.
        Process {
            id: unblockProc
            command: ["rfkill", "unblock", "bluetooth"]
            onExited: if (bt.adapter) bt.adapter.enabled = true
        }

        // revive a stopped bluetoothd. pkexec raises the polkit prompt;
        // enable --now so it also survives the next boot.
        Process {
            id: svcProc
            command: ["pkexec", "systemctl", "enable", "--now", "bluetooth.service"]
            stdout: StdioCollector {}
            stderr: StdioCollector {}
            onExited: function(exitCode) {
                if (exitCode !== 0) {
                    bt.serviceError = I18n.tr("Could not start the bluetooth service.");
                    svcErrTimer.restart();
                }
            }
        }

        Timer {
            id: svcErrTimer
            interval: 6000
            repeat: false
            onTriggered: bt.serviceError = ""
        }

        Item {
            id: btContent
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.bottom: parent.bottom
            width: Math.min(parent.width, 720)

            Item {
                id: btHeader
                anchors.top: parent.top
                anchors.left: parent.left
                anchors.right: parent.right
                height: 48

                Row {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3

                    BtRune {
                        anchors.verticalCenter: parent.verticalCenter
                        tint: bt.adapterOn ? Tokens.ink : Tokens.inkMuted
                    }

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("BLUETOOTH")
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackMark
                    }

                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: bt.adapterOn
                        text: {
                            var known = bt.devices.length;
                            if (known === 0)
                                return bt.discovering ? I18n.tr("Scanning\u2026") : I18n.tr("No devices yet");
                            if (bt.connectedCount > 0)
                                return I18n.tr("%1 connected \u00b7 %2 known").arg(bt.connectedCount).arg(known);
                            return I18n.tr("%1 known").arg(known);
                        }
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        font.weight: Font.Medium
                    }
                }

                Row {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3

                    // scan toggle (visible only while the adapter is on).
                    // discovering inverts the button (bone) to read as the ON
                    // state; tapping re-arms the 25s timer.
                    Btn {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: bt.adapterOn
                        text: bt.discovering ? I18n.tr("Scanning\u2026") : I18n.tr("Scan")
                        primary: bt.discovering
                        onAct: {
                            if (!bt.adapter)
                                return;
                            bt.adapter.discovering = !bt.adapter.discovering;
                            if (bt.adapter.discovering)
                                scanTimer.restart();
                            else
                                scanTimer.stop();
                        }
                    }

                    // one primary toggle for the whole adapter, hidden when the
                    // service is gone: a switch that can't act shouldn't look live.
                    Sw {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: bt.hasAdapter
                        on: bt.adapterOn
                        onToggled: (v) => bt.setAdapterEnabled(v)
                    }
                }
            }

            Rectangle {
                id: btRule
                anchors.top: btHeader.bottom
                anchors.topMargin: Tokens.s2
                anchors.left: parent.left
                anchors.right: parent.right
                height: 1
                color: Tokens.lineSoft
            }

            Item {
                id: btBodyArea
                anchors.top: btRule.bottom
                anchors.topMargin: Tokens.s5
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: parent.bottom

                // off / empty placeholder, centred so the page never looks broken.
                Column {
                    anchors.centerIn: parent
                    visible: !bt.hasAdapter || !bt.adapterOn || bt.devices.length === 0
                    spacing: Tokens.s3

                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: !bt.hasAdapter ? I18n.tr("Bluetooth isn't available.")
                            : bt.blocked ? I18n.tr("Bluetooth is blocked.")
                            : !bt.adapterOn ? I18n.tr("Bluetooth is off.")
                            : (bt.discovering ? I18n.tr("Scanning\u2026") : I18n.tr("No devices yet."))
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fRow
                        font.weight: Font.Medium
                    }
                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        visible: !bt.hasAdapter || !bt.adapterOn || (!bt.discovering && bt.devices.length === 0)
                        text: !bt.hasAdapter ? I18n.tr("The bluetooth service (bluez) isn't running.")
                            : bt.blocked ? I18n.tr("The radio is off at the hardware level (rfkill); the toggle unblocks it.")
                            : !bt.adapterOn ? I18n.tr("Turn the adapter on to see nearby and paired devices.")
                            : I18n.tr("Hit Scan to discover nearby devices.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        font.weight: Font.Medium
                    }

                    // service repair, shown only when org.bluez is missing entirely.
                    Btn {
                        anchors.horizontalCenter: parent.horizontalCenter
                        visible: !bt.hasAdapter
                        text: svcProc.running ? I18n.tr("Starting\u2026") : I18n.tr("Start service")
                        armed: !svcProc.running
                        onAct: bt.startService()
                    }

                    ErrChip {
                        anchors.horizontalCenter: parent.horizontalCenter
                        visible: bt.serviceError.length > 0
                        text: bt.serviceError
                    }
                }

                Flickable {
                    id: devFlick
                    visible: bt.adapterOn && bt.devices.length > 0
                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    anchors.horizontalCenter: parent.horizontalCenter
                    width: Math.min(parent.width, 640)
                    contentWidth: width
                    contentHeight: devCol.implicitHeight + 8
                    clip: true
                    boundsBehavior: Flickable.StopAtBounds
                    ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                    WheelScroll { }

                    Column {
                        id: devCol
                        width: devFlick.width - Tokens.s3
                        spacing: Tokens.s2

                        Repeater {
                            model: bt.devicesSorted

                            delegate: Column {
                                id: dev

                                required property var modelData
                                readonly property bool isConnected: modelData ? modelData.connected === true : false
                                readonly property bool isPaired: modelData ? modelData.paired === true : false
                                readonly property string addr: (modelData && modelData.address) ? modelData.address : ""
                                readonly property bool pairing: addr.length > 0 && bt.pairingAddress === addr
                                readonly property bool failed: addr.length > 0 && bt.failedAddress === addr
                                readonly property int battery: bt.batteryLevel(modelData)

                                width: parent.width
                                spacing: 4

                                // connection is set membership, not an exclusive
                                // ON, so it is marked by the word and an ink
                                // border rather than a full inversion.
                                Rectangle {
                                    id: tile
                                    width: parent.width
                                    height: 64
                                    radius: Tokens.radius
                                    color: rowHov.hovered ? Tokens.tint5 : "transparent"
                                    border.width: Tokens.border
                                    border.color: dev.isConnected ? Tokens.ink
                                        : (rowHov.hovered ? Tokens.lineStrong : Tokens.line)
                                    Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

                                    HoverHandler { id: rowHov; cursorShape: Qt.PointingHandCursor }
                                    TapHandler { onTapped: bt.activateDevice(dev.modelData) }

                                    Rectangle {
                                        id: iconTile
                                        anchors.left: parent.left
                                        anchors.leftMargin: Tokens.s3
                                        anchors.verticalCenter: parent.verticalCenter
                                        width: 40
                                        height: 40
                                        radius: Tokens.radius
                                        color: "transparent"
                                        border.width: Tokens.border
                                        border.color: dev.isConnected ? Tokens.line : Tokens.lineSoft

                                        BtRune {
                                            anchors.centerIn: parent
                                            span: 18
                                            tint: dev.isConnected ? Tokens.ink : Tokens.inkMuted
                                        }
                                    }

                                    Column {
                                        anchors.left: iconTile.right
                                        anchors.leftMargin: Tokens.s3
                                        anchors.right: devRight.left
                                        anchors.rightMargin: Tokens.s3
                                        anchors.verticalCenter: parent.verticalCenter
                                        spacing: 2

                                        Text {
                                            width: parent.width
                                            text: BtName.label(dev.modelData) || I18n.tr("Unknown")
                                            color: Tokens.ink
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fBody
                                            font.weight: dev.isConnected ? Font.DemiBold : Font.Medium
                                            elide: Text.ElideRight
                                        }
                                        Text {
                                            width: parent.width
                                            visible: text.length > 0
                                            text: dev.pairing ? I18n.tr("pairing\u2026") : bt.metaFor(dev.modelData)
                                            color: Tokens.inkMuted
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fMicro
                                            font.weight: Font.Medium
                                            elide: Text.ElideRight
                                        }
                                    }

                                    Row {
                                        id: devRight
                                        anchors.right: parent.right
                                        anchors.rightMargin: Tokens.s3
                                        anchors.verticalCenter: parent.verticalCenter
                                        spacing: Tokens.s2

                                        PulseDot {
                                            anchors.verticalCenter: parent.verticalCenter
                                            on: dev.pairing
                                        }

                                        // battery pill (connected + has a level).
                                        Rectangle {
                                            anchors.verticalCenter: parent.verticalCenter
                                            visible: dev.isConnected && dev.battery >= 0
                                            radius: Tokens.radius
                                            color: "transparent"
                                            border.width: Tokens.border
                                            border.color: Tokens.line
                                            height: 22
                                            width: battTxt.implicitWidth + 18

                                            Text {
                                                id: battTxt
                                                anchors.centerIn: parent
                                                text: Math.max(0, dev.battery) + "%"
                                                color: Tokens.ink
                                                font.family: Tokens.ui
                                                font.pixelSize: Tokens.fMicro
                                                font.weight: Font.Medium
                                                font.features: ({ "tnum": 1 })
                                            }
                                        }

                                        MiniPill {
                                            anchors.verticalCenter: parent.verticalCenter
                                            visible: !dev.isPaired && !dev.pairing
                                            text: I18n.tr("Pair")
                                            onAct: bt.activateDevice(dev.modelData)
                                        }

                                        MiniPill {
                                            anchors.verticalCenter: parent.verticalCenter
                                            visible: dev.isConnected
                                            text: I18n.tr("Disconnect")
                                            onAct: bt.activateDevice(dev.modelData)
                                        }
                                    }
                                }

                                ErrChip {
                                    visible: dev.failed
                                    x: 66
                                    text: I18n.tr("Pairing failed")
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // ── Hotspot subtab ───────────────────────────────────────────────────────

    // brings the persistent RyokuHotspot NetworkManager profile up/down through
    // nmcli, with an editable SSID + WPA2 password. state and credentials read
    // straight from NM on entry. name + password ride in as positional args,
    // NEVER spliced into the shell string.
    component HotspotBody: Item {
        id: hs

        readonly property var devices: (typeof Networking !== "undefined" && Networking && Networking.devices) ? Networking.devices.values : []
        readonly property var wifiDev: hs.devices.find(function(d) { return d && d.type === DeviceType.Wifi }) || null

        readonly property string hsCon: "RyokuHotspot"
        readonly property string hsIface: hs.wifiDev ? (hs.wifiDev.name || "wlan0") : "wlan0"
        property string hsName: "Ryoku"
        property string hsPw: ""
        property bool hsActive: false
        property bool hsBusy: false
        property string hsEdit: ""
        property string hsDraft: ""

        function applyHotspot() {
            if (hs.hsBusy || hs.hsPw.length < 8)
                return;
            hs.hsBusy = true;
            hsApplyProc.command = ["sh", "-c",
                'c="' + hs.hsCon + '"; '
                + 'if nmcli -t connection show "$c" >/dev/null 2>&1; then '
                +   'nmcli connection modify "$c" 802-11-wireless.ssid "$1" 802-11-wireless-security.key-mgmt wpa-psk 802-11-wireless-security.psk "$2"; '
                + 'else '
                +   'nmcli connection add type wifi ifname "$3" con-name "$c" autoconnect no 802-11-wireless.ssid "$1" 802-11-wireless.mode ap 802-11-wireless-security.key-mgmt wpa-psk 802-11-wireless-security.psk "$2" ipv4.method shared; '
                + 'fi; '
                + 'nmcli connection up "$c"',
                "sh", hs.hsName, hs.hsPw, hs.hsIface];
            hsApplyProc.running = true;
        }

        function stopHotspot() {
            if (hs.hsBusy)
                return;
            hs.hsBusy = true;
            hsDownProc.running = true;
        }

        function refreshHotspot() {
            hsStateProc.running = true;
            hsReadProc.running = true;
        }

        // commit an inline name or password edit. a password shorter than the
        // WPA2 8-char minimum is dropped silently. a live hotspot re-applies so
        // the change takes effect at once.
        function commitHotspotEdit() {
            if (hs.hsEdit === "name") {
                if (hs.hsDraft.length)
                    hs.hsName = hs.hsDraft;
            } else if (hs.hsEdit === "pw") {
                if (hs.hsDraft.length >= 8)
                    hs.hsPw = hs.hsDraft;
            }
            hs.hsEdit = "";
            hs.hsDraft = "";
            if (hs.hsActive)
                hs.applyHotspot();
        }

        // 8-char WPA2 password from an unambiguous alphabet (no 0/O/1/l/I).
        function generatePw() {
            var cs = "abcdefghijkmnpqrstuvwxyz23456789";
            var s = "";
            for (var i = 0; i < 8; i++)
                s += cs.charAt(Math.floor(Math.random() * cs.length));
            return s;
        }

        Component.onCompleted: hs.refreshHotspot()

        Process {
            id: hsApplyProc
            onExited: {
                hs.hsBusy = false;
                hs.refreshHotspot();
            }
        }

        Process {
            id: hsDownProc
            command: ["nmcli", "connection", "down", hs.hsCon]
            onExited: {
                hs.hsBusy = false;
                hs.refreshHotspot();
            }
        }

        Process {
            id: hsStateProc
            command: ["sh", "-c", "nmcli -t -f NAME connection show --active | grep -qxF -- \"$1\" && echo on || echo off", "sh", hs.hsCon]
            stdout: StdioCollector {
                onStreamFinished: hs.hsActive = this.text.trim() === "on"
            }
        }

        Process {
            id: hsReadProc
            command: ["nmcli", "-t", "-s", "-g", "802-11-wireless.ssid,802-11-wireless-security.psk", "connection", "show", hs.hsCon]
            stdout: StdioCollector {
                onStreamFinished: {
                    var lines = this.text.split("\n");
                    if (lines.length >= 1 && lines[0].length)
                        hs.hsName = lines[0];
                    if (lines.length >= 2 && lines[1].length)
                        hs.hsPw = lines[1];
                }
            }
        }

        Column {
            id: hsHead
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            spacing: Tokens.s2

            Text {
                text: I18n.tr("Share this machine's connection as a Wi-Fi hotspot. NetworkManager owns the profile (named %1); changes to the network name or password apply at once when the hotspot is live.").arg(hs.hsCon)
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                font.weight: Font.Medium
                width: Math.min(parent.width, 720)
                wrapMode: Text.WordWrap
            }
        }

        Column {
            id: hsForm
            anchors.left: parent.left
            anchors.top: hsHead.bottom
            anchors.topMargin: Tokens.s6
            width: Math.min(parent.width, 600)
            spacing: Tokens.s5

            // big toggle card: glyph, label, live status, switch. active is a
            // boolean ON marked by the word + an ink border (the Sw knob also
            // reads ON); no full inversion.
            Rectangle {
                width: parent.width
                height: 76
                radius: Tokens.radius
                color: "transparent"
                border.width: Tokens.border
                border.color: hs.hsActive ? Tokens.ink : Tokens.line
                Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

                WifiGlyph {
                    id: hsGlyph
                    anchors.left: parent.left
                    anchors.leftMargin: Tokens.s5
                    anchors.verticalCenter: parent.verticalCenter
                    tint: hs.hsActive ? Tokens.ink : Tokens.inkMuted
                }

                Column {
                    anchors.left: hsGlyph.right
                    anchors.leftMargin: Tokens.s3
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 2

                    Text {
                        text: I18n.tr("Hotspot")
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fRow
                        font.weight: Font.DemiBold
                    }
                    Text {
                        text: hs.hsBusy ? I18n.tr("Working\u2026")
                            : (hs.hsActive ? I18n.tr("Active on %1").arg(hs.hsIface) : I18n.tr("Off"))
                        color: hs.hsActive ? Tokens.ink : Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        font.weight: Font.Medium
                    }
                }

                Sw {
                    anchors.right: parent.right
                    anchors.rightMargin: Tokens.s5
                    anchors.verticalCenter: parent.verticalCenter
                    on: hs.hsActive
                    enabled: !hs.hsBusy
                    opacity: hs.hsBusy ? 0.3 : 1
                    onToggled: {
                        if (hs.hsActive) {
                            hs.stopHotspot();
                        } else {
                            if (hs.hsPw.length < 8)
                                hs.hsPw = hs.generatePw();
                            hs.applyHotspot();
                        }
                    }
                }
            }

            // credentials.
            Column {
                width: parent.width
                spacing: Tokens.s3

                Row {
                    width: parent.width
                    spacing: Tokens.s2
                    Rectangle { width: 4; height: 4; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                    Text {
                        text: I18n.tr("DETAILS")
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackMark
                        anchors.verticalCenter: parent.verticalCenter
                    }
                    Rectangle {
                        width: Math.max(0, parent.parent.width - 100)
                        height: 1
                        color: Tokens.lineSoft
                        anchors.verticalCenter: parent.verticalCenter
                    }
                }

                CredRow {
                    width: parent.width
                    field: "name"
                    label: I18n.tr("Network name")
                    value: hs.hsName
                    placeholder: "Ryoku"
                    editing: hs.hsEdit === "name"
                    draft: hs.hsDraft
                    onBeginEdit: { hs.hsDraft = value; hs.hsEdit = "name"; }
                    onDraftEdited: (v) => hs.hsDraft = v
                    onCommit: hs.commitHotspotEdit()
                    onCancel: { hs.hsEdit = ""; hs.hsDraft = ""; }
                }

                CredRow {
                    width: parent.width
                    field: "pw"
                    label: I18n.tr("Password")
                    value: hs.hsPw
                    placeholder: I18n.tr("Tap to set")
                    secret: true
                    editing: hs.hsEdit === "pw"
                    draft: hs.hsDraft
                    onBeginEdit: { hs.hsDraft = value; hs.hsEdit = "pw"; }
                    onDraftEdited: (v) => hs.hsDraft = v
                    onCommit: hs.commitHotspotEdit()
                    onCancel: { hs.hsEdit = ""; hs.hsDraft = ""; }
                }
            }
        }
    }

    Component { id: wifiComp; WifiBody {} }
    Component { id: btComp; BtBody {} }
    Component { id: hsComp; HotspotBody {} }

    // a faint 接続 watermark dresses the whole page, like every section
    Watermark { anchors.fill: parent; text: "\u63a5\u7d9a" }

    // ── layout: head, bone-pill tab strip, body ─────────────────────────────
    Item {
        id: content
        anchors.fill: parent
        anchors.margins: Tokens.s6

        // head: eyebrow, Fraunces title, blurb (matches every page).
        Column {
            id: head
            anchors { left: parent.left; right: parent.right; top: parent.top }
            // the register row sits off the title: a rule over a 32px
            // title needs more than the gap between two lines of body text
            spacing: Tokens.s3

            Row {
                // the register row holds a fixed box, so the rule and the seal keep
                // their distance from the title on every page
                height: Tokens.s5
                spacing: Tokens.s2
                Rectangle {
                    width: 16; height: 1; color: Tokens.ink
                    anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: "\u529b"; color: Tokens.ink; font.family: Tokens.jp
                    font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: I18n.tr("DEVICES"); color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
            Text {
                text: I18n.tr("Connections"); color: Tokens.ink
                font.family: Tokens.display; font.pixelSize: Tokens.fTitle
            }
            Text {
                width: Math.min(parent.width, 720)
                text: I18n.tr("Wi-Fi, Bluetooth and this machine's hotspot, applied live.")
                color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
            }
        }

        // The kill switch is a machine-wide firewall, not a radio toggle. The
        // copy names SSH because an activated switch severs remote sessions too.
        Rectangle {
            id: killSwitch
            anchors { left: parent.left; right: parent.right; rightMargin: Tokens.s5; top: head.bottom; topMargin: Tokens.s4 }
            implicitHeight: killCopy.implicitHeight + Tokens.s4
            radius: Tokens.radius
            color: pg.killActive ? Tokens.bone : "transparent"
            border.width: Tokens.border
            border.color: pg.killActive ? Tokens.bone : Tokens.line

            Column {
                id: killCopy
                anchors { left: parent.left; right: killAction.left; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter; leftMargin: Tokens.s3 }
                spacing: Tokens.s1
                Text {
                    text: pg.killActive ? I18n.tr("INTERNET ISOLATED") : I18n.tr("INTERNET KILL SWITCH")
                    color: pg.killActive ? Tokens.inkOnBone : Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackMark
                }
                Text {
                    width: parent.width
                    text: pg.killState === "armed" ? I18n.tr("Firewall repair required before this machine is safe.")
                        : (pg.killState === "blocked" ? I18n.tr("Traffic is blocked but the boot guard is not armed.")
                        : I18n.tr("Blocks Wi-Fi, Ethernet, VPN, LAN and IPv4/IPv6. It also severs SSH."))
                    color: pg.killActive ? Tokens.inkOnBone : Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    wrapMode: Text.WordWrap
                }
                Text {
                    visible: pg.killError !== ""
                    text: pg.killError
                    color: pg.killActive ? Tokens.inkOnBone : Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    wrapMode: Text.WordWrap
                }
            }

            MiniPill {
                id: killAction
                anchors { right: parent.right; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                text: pg.killBusy ? I18n.tr("WORKING") : (pg.killState === "off" ? I18n.tr("ISOLATE") : I18n.tr("RESTORE"))
                armed: !pg.killBusy && pg.killState !== "checking"
                onAct: pg.toggleKillSwitch()
            }
        }

        Text {
            id: killWarning
            anchors { left: killSwitch.left; right: killSwitch.right; top: killSwitch.bottom; topMargin: Tokens.s1 }
            visible: pg.killState === "off"
            text: I18n.tr("Use ISOLATE from the physical machine; every remote connection is cut immediately.")
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            wrapMode: Text.WordWrap
        }


        // the shared Tabs plate: selection is the // lead on bone, no slider.
        Tabs {
            id: tabStrip
            anchors { left: parent.left; top: killWarning.bottom; topMargin: Tokens.s4 }
            options: pg.tabs.map(function (t) { return t.label; })
            current: {
                for (var i = 0; i < pg.tabs.length; i++)
                    if (pg.tabs[i].key === pg.sub) return pg.tabs[i].label;
                return pg.tabs[0].label;
            }
            onChose: (label) => {
                for (var i = 0; i < pg.tabs.length; i++)
                    if (pg.tabs[i].label === label) { pg.sub = pg.tabs[i].key; break; }
            }
        }

        // body: swaps source on tab change; the outgoing tab is destroyed, so
        // each body re-reads state and stops its own scan (as in the old page).
        // an unrecognised sub falls through to Hotspot, preserving old behaviour.
        Loader {
            id: body
            anchors {
                left: parent.left
                right: parent.right
                rightMargin: 0
                top: tabStrip.bottom; bottom: parent.bottom
                topMargin: Tokens.s5
            }
            sourceComponent: pg.sub === "wifi" ? wifiComp
                : (pg.sub === "bluetooth" ? btComp : hsComp)
        }

    }
}
