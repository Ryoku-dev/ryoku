pragma ComponentBehavior: Bound

import QtQuick
import ".."
import shell.services
import "../../../components"
import Ryoku.Ui.Singletons

// Network popout: a frame-edge card (shared PopoutCard, so it opens and melts
// like the music card) leading with the current link (wired or the joined
// Wi-Fi), then the available networks as compact rows -- tap an open or saved
// one to join, or expand a secured one for its password. Wi-Fi power and scan
// ride the head. All live off the daemon `network` topic via the Network
// singleton; the surface takes on-demand keyboard only while a password field
// is focused, and dismisses on an outside click.
Item {
    id: root

    property real s: 1
    property bool open: false

    // a hidden network has no scanned AP, so it joins through the daemon intent
    // with 802-11-wireless.hidden set; these drive the inline join card.
    property bool hiddenOpen: false
    property string hiddenSsid: ""
    property string hiddenPassword: ""
    property int hiddenPendingId: -1
    property bool hiddenConnecting: false
    property bool hiddenError: false

    function openHidden() {
        root.hiddenOpen = true;
        root.hiddenError = false;
        Qt.callLater(function() { if (hiddenSsidField.visible) hiddenSsidField.forceActiveFocus(); });
    }
    function cancelHidden() {
        root.hiddenOpen = false;
        root.hiddenConnecting = false;
        root.hiddenPendingId = -1;
        root.hiddenSsid = "";
        root.hiddenPassword = "";
        root.hiddenError = false;
    }
    function submitHidden() {
        if (root.hiddenSsid === "" || root.hiddenConnecting)
            return;
        root.hiddenError = false;
        root.hiddenConnecting = true;
        root.hiddenPendingId = Network.connectWifi(root.hiddenSsid, root.hiddenPassword, "", true);
    }
    Connections {
        target: Network
        function onReplied(id, ok, error) {
            if (id !== root.hiddenPendingId)
                return;
            root.hiddenConnecting = false;
            root.hiddenPendingId = -1;
            if (ok)
                root.cancelHidden();
            else {
                root.hiddenError = true;
                root.hiddenPassword = "";
            }
        }
    }

    readonly property real pad: 11 * root.s
    readonly property color ink: Theme.ink(Theme.effectiveSurface)
    readonly property color inkDim: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
    readonly property color line: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.14)

    readonly property bool wifiOn: Network.wifiRadio
    readonly property bool wiredUp: Network.kind === "ethernet"
    readonly property bool wifiUp: Network.kind === "wifi"
    readonly property string activeSsid: Network.activeSsid
    readonly property bool anyLink: root.wifiOn || root.wiredUp
    property bool scanning: false

    // available networks: one row per SSID, however many bands or access
    // points broadcast it, the strongest standing for the row. Joins leave the
    // access point to NetworkManager so it can roam across bands; the joined
    // SSID is the header card, not a row.
    readonly property var availableNets: {
        const aps = Network.accessPoints;
        const best = ({});
        for (let i = 0; i < aps.length; i++) {
            const ap = aps[i];
            if (!ap || !ap.ssid || ap.ssid === root.activeSsid)
                continue;
            const prev = best[ap.ssid];
            const saved = !!ap.saved || (!!prev && !!prev.saved);
            if (!prev || (ap.strength || 0) > (prev.strength || 0))
                best[ap.ssid] = Object.assign({}, ap, { saved: saved });
            else if (saved && !prev.saved)
                best[ap.ssid] = Object.assign({}, prev, { saved: true });
        }
        const out = [];
        for (const k in best) {
            const ap = best[k];
            // NetworkManager saves a profile as a first join starts; until the
            // join settles the row stays unsaved, so it neither moves lists nor
            // loses its password field
            out.push(root.joining[ap.ssid] ? Object.assign({}, ap, { saved: false }) : ap);
        }
        out.sort((x, y) => (y.strength || 0) - (x.strength || 0));
        return out;
    }
    readonly property var savedNets: root.availableNets.filter(n => !!n.saved)
    readonly property var otherNets: root.availableNets.filter(n => !n.saved)

    onAvailableNetsChanged: if (root.availableNets.length > 0) root.scanning = false

    function secured(ap) { return ap && ap.security && ap.security !== "None"; }

    onOpenChanged: {
        Network.setVpnPolling(root, root.open);
        if (root.open) {
            root.scanning = true;
            Network.refresh();
            scanClear.restart();
        } else {
            root.scanning = false;
            scanClear.stop();
        }
    }
    Component.onDestruction: Network.setVpnPolling(root, false)

    Timer { id: rescan; interval: 30000; repeat: true; running: root.open && root.wifiOn; onTriggered: Network.refresh() }
    Timer { id: scanClear; interval: 15000; onTriggered: root.scanning = false }

    function toggleWifi() { Network.setWifiEnabled(!root.wifiOn); }
    function scan() {
        if (!root.wifiOn)
            return;
        root.scanning = true;
        Network.refresh();
        scanClear.restart();
    }

    implicitWidth: 250 * root.s
    implicitHeight: content.implicitHeight + root.pad * 2

    PopoutCard { anchors.fill: parent }

    // Row state is kept here, keyed by SSID: the list is rebuilt from a fresh
    // array on every network frame, which destroyed a row mid-join along with
    // its field, its pending reply and its error.
    property var rowState: ({})
    property var pendingJoins: ({})
    property var joining: ({})
    readonly property var pwDrafts: ({})
    property string focusSsid: ""
    property bool heroOpen: false
    property bool heroConfirm: false
    onActiveSsidChanged: {
        root.heroOpen = false;
        root.heroConfirm = false;
    }

    function st(ssid) { return root.rowState[ssid] || ({}); }
    function setSt(ssid, patch) {
        const m = Object.assign({}, root.rowState);
        m[ssid] = Object.assign({}, m[ssid] || ({}), patch);
        root.rowState = m;
    }
    function toggleExpanded(ssid) {
        const open = !root.st(ssid).expanded;
        root.setSt(ssid, { expanded: open });
        if (open)
            root.focusSsid = ssid;
    }
    function setJoining(ssid, on) {
        const m = Object.assign({}, root.joining);
        if (on)
            m[ssid] = true;
        else
            delete m[ssid];
        root.joining = m;
    }
    function join(ap, pw) {
        root.setSt(ap.ssid, { connecting: true, error: false });
        if (!ap.saved)
            root.setJoining(ap.ssid, true);
        const id = Network.connectWifi(ap.ssid, pw, "");
        root.pendingJoins[id] = { ssid: ap.ssid };
    }
    function forget(ssid) {
        root.setSt(ssid, { expanded: false, reprompt: false, error: false, connecting: false });
        root.pwDrafts[ssid] = "";
        Network.forgetWifi(ssid);
    }
    Connections {
        target: Network
        function onReplied(id, ok, error) {
            const j = root.pendingJoins[id];
            if (!j)
                return;
            delete root.pendingJoins[id];
            root.setJoining(j.ssid, false);
            root.pwDrafts[j.ssid] = "";
            // pwEpoch tells the row to empty its field, so a failed password is
            // never left for the next attempt to be typed onto
            const epoch = (root.st(j.ssid).pwEpoch || 0) + 1;
            if (ok) {
                root.setSt(j.ssid, { connecting: false, expanded: false, error: false, reprompt: false, pwEpoch: epoch });
            } else {
                // ask for the password again, also when the network reads as
                // saved: a stored secret failed, or a failed first join left
                // its profile behind
                root.setSt(j.ssid, { connecting: false, expanded: true, error: true,
                    reprompt: true, pwEpoch: epoch });
                root.focusSsid = j.ssid;
            }
        }
    }

    // an available-network row: signal + SSID (+ lock), tap to join or, when a
    // password is needed, expand an inline field.
    component ApRow: Column {
        id: apr
        required property var ap
        readonly property var rs: root.st(apr.ap.ssid)
        readonly property bool expanded: !!apr.rs.expanded
        readonly property bool connecting: !!apr.rs.connecting
        readonly property bool errorShown: !!apr.rs.error
        readonly property bool sec: root.secured(apr.ap)
        readonly property bool reprompt: !!apr.rs.reprompt
        readonly property bool needsPw: apr.sec && (!apr.ap.saved || apr.reprompt)
        property int seenEpoch: 0

        width: parent ? parent.width : 0
        spacing: 4 * root.s

        function doConnect() {
            root.join(apr.ap, apr.needsPw ? pwField.text : "");
        }
        function focusField() {
            if (apr.expanded && apr.needsPw && root.focusSsid === apr.ap.ssid)
                pwField.forceActiveFocus();
        }
        Component.onCompleted: {
            apr.seenEpoch = apr.rs.pwEpoch || 0;
            pwField.text = root.pwDrafts[apr.ap.ssid] || "";
            apr.focusField();
        }
        onExpandedChanged: apr.focusField()
        onRsChanged: {
            const e = apr.rs.pwEpoch || 0;
            if (e !== apr.seenEpoch) {
                apr.seenEpoch = e;
                pwField.text = "";
                apr.focusField();
            }
        }

        Item {
            width: parent.width
            height: 26 * root.s
            Rectangle {
                anchors.fill: parent
                radius: 3 * root.s
                color: apTap.pressed ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.16)
                    : (apHover.hovered ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.08) : "transparent")
                border.width: Theme.borderWidth
                border.color: root.line
                Behavior on color { ColorAnimation { duration: Motion.fast } }
            }
            GlyphIcon {
                id: apIcon
                anchors.left: parent.left
                anchors.leftMargin: 7 * root.s
                anchors.verticalCenter: parent.verticalCenter
                width: 14 * root.s
                height: width
                name: "wifi"
                stroke: 1.6
                color: root.inkDim
                opacity: 0.4 + 0.6 * Math.max(0, Math.min(1, (apr.ap.strength || 0) / 100))
            }
            Text {
                anchors.left: apIcon.right
                anchors.leftMargin: 7 * root.s
                anchors.right: apMeta.left
                anchors.rightMargin: 6 * root.s
                anchors.verticalCenter: parent.verticalCenter
                text: apr.ap.ssid
                color: root.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 11 * root.s
                elide: Text.ElideRight
            }
            Row {
                id: apMeta
                anchors.right: parent.right
                anchors.rightMargin: (apr.ap.saved ? 30 : 8) * root.s
                anchors.verticalCenter: parent.verticalCenter
                spacing: 5 * root.s
                GlyphIcon {
                    visible: apr.sec
                    anchors.verticalCenter: parent.verticalCenter
                    width: 11 * root.s
                    height: width
                    name: "lock"
                    stroke: 1.6
                    color: root.inkDim
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: apr.connecting ? "…" : Math.round(apr.ap.strength || 0) + "%"
                    color: root.inkDim
                    font.family: Theme.mono
                    font.pixelSize: 9.5 * root.s
                }
            }
            HoverHandler { id: apHover; cursorShape: Qt.PointingHandCursor }
            MouseArea {
                id: apTap
                anchors.fill: parent
                onClicked: {
                    if (apr.needsPw)
                        root.toggleExpanded(apr.ap.ssid);
                    else
                        apr.doConnect();
                }
            }
            // a saved row's drawer holds Forget; above the row's tap area
            Rectangle {
                visible: apr.ap.saved
                z: 2
                anchors.right: parent.right
                anchors.rightMargin: 3 * root.s
                anchors.verticalCenter: parent.verticalCenter
                width: 24 * root.s
                height: 20 * root.s
                radius: 3 * root.s
                color: optMa.containsMouse || apr.expanded ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.14) : "transparent"
                Text {
                    anchors.centerIn: parent
                    text: "\u22EF"
                    color: root.ink
                    font.family: Theme.fontPrimary
                    font.pixelSize: 14 * root.s
                }
                MouseArea {
                    id: optMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: root.toggleExpanded(apr.ap.ssid)
                }
            }
        }

        // the drawer: the password (secured and unsaved, or saved with a failed
        // password) and, for a saved network, Forget
        Column {
            width: parent.width
            spacing: 4 * root.s
            visible: apr.expanded && (apr.needsPw || apr.ap.saved)

            Rectangle {
                visible: apr.needsPw
                width: parent.width
                height: 26 * root.s
                radius: 3 * root.s
                color: "transparent"
                border.width: Theme.borderWidth
                border.color: root.line
                TextInput {
                    id: pwField
                    anchors.left: parent.left
                    anchors.right: eye.left
                    anchors.leftMargin: 8 * root.s
                    anchors.rightMargin: 4 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    color: root.ink
                    font.family: Theme.fontPrimary
                    font.pixelSize: 11 * root.s
                    echoMode: eye.show ? TextInput.Normal : TextInput.Password
                    clip: true
                    onTextEdited: root.pwDrafts[apr.ap.ssid] = text
                    onAccepted: apr.doConnect()
                    Text {
                        anchors.fill: parent
                        verticalAlignment: Text.AlignVCenter
                        text: I18n.tr("Password")
                        color: root.inkDim
                        font: pwField.font
                        visible: pwField.text.length === 0 && !pwField.activeFocus
                    }
                }
                GlyphIcon {
                    id: eye
                    property bool show: false
                    anchors.right: parent.right
                    anchors.rightMargin: 7 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    width: 13 * root.s
                    height: width
                    name: eye.show ? "close" : "info"
                    stroke: 1.6
                    color: root.inkDim
                    TapHandler { onTapped: eye.show = !eye.show }
                }
            }
            Text {
                width: parent.width
                visible: apr.errorShown
                text: apr.ap.saved
                    ? I18n.tr("Wrong password, or a different network with the same name?")
                    : I18n.tr("Wrong password or connection failed")
                color: Theme.error
                wrapMode: Text.WordWrap
                font.family: Theme.fontPrimary
                font.pixelSize: 9.5 * root.s
            }
            PopoutAction {
                visible: apr.needsPw
                width: parent.width
                s: root.s
                enabled: !apr.connecting
                label: apr.connecting ? I18n.tr("Connecting…") : I18n.tr("Connect")
                onClicked: apr.doConnect()
            }
            PopoutAction {
                visible: apr.ap.saved
                width: parent.width
                s: root.s
                destructive: true
                enabled: !apr.connecting
                label: I18n.tr("Forget network")
                onClicked: {
                    pwField.text = "";
                    root.forget(apr.ap.ssid);
                }
            }
        }
    }

    Column {
        id: content
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: root.pad
        spacing: 8 * root.s

        // head: label, wifi toggle, scan.
        Item {
            width: parent.width
            height: 20 * root.s
            Text {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("NETWORK")
                color: root.inkDim
                font.family: Theme.mono
                font.pixelSize: 9 * root.s
                font.letterSpacing: 1.6
                font.weight: Font.Medium
            }
            Row {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 5 * root.s
                PopoutHeadButton {
                    visible: root.wifiOn
                    s: root.s
                    glyph: "reboot"
                    active: root.scanning
                    spin: root.scanning
                    onClicked: root.scan()
                }
                PopoutHeadButton {
                    s: root.s
                    glyph: "wifi"
                    active: root.wifiOn
                    onClicked: root.toggleWifi()
                }
            }
        }

        // OFF plate (wifi off and no wired link).
        Column {
            width: parent.width
            spacing: 7 * root.s
            visible: !root.wifiOn && !root.wiredUp
            topPadding: 6 * root.s
            bottomPadding: 4 * root.s
            GlyphIcon {
                anchors.horizontalCenter: parent.horizontalCenter
                width: 27 * root.s
                height: width
                name: "wifi"
                stroke: 1.6
                color: root.inkDim
                opacity: 0.5
            }
            Text {
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                text: I18n.tr("Wi-Fi is off")
                color: root.ink
                font.family: Theme.fontPrimary
                font.pixelSize: 12 * root.s
                font.weight: Font.DemiBold
            }
            PopoutAction {
                anchors.horizontalCenter: parent.horizontalCenter
                s: root.s
                label: I18n.tr("Turn on")
                onClicked: root.toggleWifi()
            }
        }

        // HERO: the current link.
        Item {
            width: parent.width
            height: 36 * root.s
            visible: root.anyLink
            Rectangle {
                id: netTile
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: 30 * root.s
                height: width
                radius: 4 * root.s
                color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.05)
                border.width: Theme.borderWidth
                border.color: root.line
                GlyphIcon {
                    anchors.centerIn: parent
                    width: 16 * root.s
                    height: width
                    name: root.wiredUp ? "ethernet" : "wifi"
                    stroke: 1.7
                    color: (root.wiredUp || root.wifiUp) ? root.ink : root.inkDim
                }
            }
            Rectangle {
                id: heroOpt
                visible: root.wifiUp
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                width: 22 * root.s
                height: 22 * root.s
                radius: 3 * root.s
                color: heroOptMa.containsMouse || root.heroOpen ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.14) : "transparent"
                Text {
                    anchors.centerIn: parent
                    text: "\u22EF"
                    color: root.ink
                    font.family: Theme.fontPrimary
                    font.pixelSize: 14 * root.s
                }
                MouseArea {
                    id: heroOptMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        root.heroOpen = !root.heroOpen;
                        root.heroConfirm = false;
                    }
                }
            }
            PopoutAction {
                id: disc
                anchors.right: heroOpt.visible ? heroOpt.left : parent.right
                anchors.rightMargin: heroOpt.visible ? 4 * root.s : 0
                anchors.verticalCenter: parent.verticalCenter
                visible: root.wifiUp
                s: root.s
                destructive: true
                label: I18n.tr("Disconnect")
                onClicked: Network.disconnectWifi()
            }
            Column {
                anchors.left: netTile.right
                anchors.leftMargin: 9 * root.s
                anchors.right: disc.visible ? disc.left : parent.right
                anchors.rightMargin: 8 * root.s
                anchors.verticalCenter: parent.verticalCenter
                spacing: 1 * root.s
                Text {
                    width: parent.width
                    text: root.wiredUp ? I18n.tr("Wired")
                        : root.wifiUp ? root.activeSsid
                        : I18n.tr("Not connected")
                    color: root.ink
                    font.family: Theme.fontPrimary
                    font.pixelSize: 12.5 * root.s
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    text: {
                        if (root.wiredUp) return I18n.tr("Connected");
                        if (root.wifiUp) {
                            const s = Math.round(Network.wifi.strength || 0);
                            const base = I18n.tr("Connected · %1%").arg(s);
                            return Network.vpnActive ? base + " · " + Network.vpnName : base;
                        }
                        return Network.wifiConnectivity === "Connecting" ? I18n.tr("Connecting…") : I18n.tr("Choose a network");
                    }
                    color: root.inkDim
                    font.family: Theme.mono
                    font.pixelSize: 9 * root.s
                    elide: Text.ElideRight
                }
            }
        }
        // forgetting the network you are on also disconnects, so it is armed
        // by a first press
        PopoutAction {
            width: parent.width
            visible: root.wifiUp && root.heroOpen
            s: root.s
            destructive: true
            label: root.heroConfirm ? I18n.tr("Press again to forget %1").arg(root.activeSsid) : I18n.tr("Forget network")
            onClicked: {
                if (!root.heroConfirm) {
                    root.heroConfirm = true;
                    heroDisarm.restart();
                    return;
                }
                root.heroConfirm = false;
                root.heroOpen = false;
                root.forget(root.activeSsid);
            }
            Timer {
                id: heroDisarm
                interval: 3000
                onTriggered: root.heroConfirm = false
            }
        }

        // saved networks in range, then the rest
        Text {
            width: parent.width
            visible: root.wifiOn && root.savedNets.length > 0
            text: I18n.tr("SAVED")
            color: root.inkDim
            font.family: Theme.mono
            font.pixelSize: 8.5 * root.s
            font.letterSpacing: 1.5
        }
        Column {
            width: parent.width
            spacing: 5 * root.s
            visible: root.wifiOn && root.savedNets.length > 0
            Repeater {
                model: root.savedNets
                delegate: ApRow {
                    required property var modelData
                    ap: modelData
                }
            }
        }
        Text {
            width: parent.width
            visible: root.wifiOn
            text: root.savedNets.length > 0
                ? (root.scanning ? I18n.tr("OTHER NETWORKS · SCANNING") : I18n.tr("OTHER NETWORKS"))
                : (root.scanning ? I18n.tr("NETWORKS · SCANNING") : I18n.tr("NETWORKS"))
            color: root.inkDim
            font.family: Theme.mono
            font.pixelSize: 8.5 * root.s
            font.letterSpacing: 1.5
        }
        Column {
            width: parent.width
            spacing: 5 * root.s
            visible: root.wifiOn
            Repeater {
                model: root.otherNets
                delegate: ApRow {
                    required property var modelData
                    ap: modelData
                }
            }
        }
        Text {
            width: parent.width
            visible: root.wifiOn && root.availableNets.length === 0
            horizontalAlignment: Text.AlignHCenter
            text: root.scanning ? I18n.tr("Scanning…") : I18n.tr("No networks found")
            color: root.inkDim
            font.family: Theme.fontPrimary
            font.pixelSize: 10 * root.s
            topPadding: 2 * root.s
        }
        Text {
            width: parent.width
            visible: root.wifiOn && !root.hiddenOpen
            horizontalAlignment: Text.AlignHCenter
            topPadding: 4 * root.s
            text: I18n.tr("Connect to a hidden network")
            color: hiddenLinkHover.hovered ? root.ink : root.inkDim
            font.family: Theme.fontPrimary
            font.pixelSize: 10 * root.s
            Behavior on color { ColorAnimation { duration: 120 } }
            HoverHandler { id: hiddenLinkHover }
            TapHandler { onTapped: root.openHidden() }
        }
        Column {
            width: parent.width
            spacing: 4 * root.s
            visible: root.wifiOn && root.hiddenOpen

            Rectangle {
                width: parent.width
                height: 26 * root.s
                radius: 3 * root.s
                color: "transparent"
                border.width: Theme.borderWidth
                border.color: root.line
                TextInput {
                    id: hiddenSsidField
                    anchors.fill: parent
                    anchors.leftMargin: 8 * root.s
                    anchors.rightMargin: 8 * root.s
                    verticalAlignment: TextInput.AlignVCenter
                    color: root.ink
                    font.family: Theme.fontPrimary
                    font.pixelSize: 11 * root.s
                    clip: true
                    text: root.hiddenSsid
                    onTextChanged: { root.hiddenSsid = text; root.hiddenError = false; }
                    Keys.onPressed: function(event) { if (event.key === Qt.Key_Escape) { root.cancelHidden(); event.accepted = true; } }
                    Text {
                        anchors.fill: parent
                        verticalAlignment: Text.AlignVCenter
                        text: I18n.tr("Network name (SSID)")
                        color: root.inkDim
                        font: hiddenSsidField.font
                        visible: hiddenSsidField.text.length === 0 && !hiddenSsidField.activeFocus
                    }
                }
            }
            Rectangle {
                width: parent.width
                height: 26 * root.s
                radius: 3 * root.s
                color: "transparent"
                border.width: Theme.borderWidth
                border.color: root.line
                TextInput {
                    anchors.fill: parent
                    anchors.leftMargin: 8 * root.s
                    anchors.rightMargin: 8 * root.s
                    verticalAlignment: TextInput.AlignVCenter
                    color: root.ink
                    font.family: Theme.fontPrimary
                    font.pixelSize: 11 * root.s
                    echoMode: TextInput.Password
                    clip: true
                    text: root.hiddenPassword
                    onTextChanged: { root.hiddenPassword = text; root.hiddenError = false; }
                    onAccepted: root.submitHidden()
                    Keys.onPressed: function(event) { if (event.key === Qt.Key_Escape) { root.cancelHidden(); event.accepted = true; } }
                    Text {
                        anchors.fill: parent
                        verticalAlignment: Text.AlignVCenter
                        text: I18n.tr("Password (leave empty if open)")
                        color: root.inkDim
                        font: parent.font
                        visible: parent.text.length === 0 && !parent.activeFocus
                    }
                }
            }
            Text {
                width: parent.width
                visible: root.hiddenError
                text: I18n.tr("Wrong password or connection failed")
                color: Theme.error
                wrapMode: Text.WordWrap
                font.family: Theme.fontPrimary
                font.pixelSize: 9.5 * root.s
            }
            PopoutAction {
                width: parent.width
                s: root.s
                enabled: root.hiddenSsid !== "" && !root.hiddenConnecting
                label: root.hiddenConnecting ? I18n.tr("Connecting…") : I18n.tr("Connect")
                onClicked: root.submitHidden()
            }
            PopoutAction {
                width: parent.width
                s: root.s
                label: I18n.tr("Cancel")
                onClicked: root.cancelHidden()
            }
        }
    }
}
