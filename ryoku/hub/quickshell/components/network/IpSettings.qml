import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// the IPv4 settings of one saved connection (by uuid): DHCP or a manual address, gateway and DNS,
// read and written through the shell daemon. On a live connection the daemon puts the old settings
// back after a countdown unless they are kept here, so a wrong address or gateway that cuts the
// computer off undoes itself
Column {
    id: ed

    property string uuid: ""
    // a tunnel's address is always set by hand and has no gateway
    property bool tunnel: false

    property bool loaded: false
    property bool busy: false
    property string error: ""
    property string method: "auto"
    property bool dirty: false
    // seconds left before the daemon reverts, 0 when nothing waits
    property int revertLeft: 0
    property real revertUntil: 0
    // the uuid the fields hold, so a row that takes another connection reloads
    property string shownUuid: ""
    readonly property bool manual: ed.tunnel || ed.method === "manual"
    readonly property bool valid: ed.method === "auto" && !ed.tunnel || addressField.text.trim().length > 0
    readonly property bool wide: ed.width >= 520

    width: parent ? parent.width : 0
    spacing: LithicTokens.space.s3
    topPadding: LithicTokens.space.s2
    bottomPadding: LithicTokens.space.s2

    function servers(text) {
        return String(text || "").split(/[\s,]+/).filter(s => s.length > 0);
    }

    function tick() {
        ed.revertLeft = ed.revertUntil > 0 ? Math.max(0, Math.ceil((ed.revertUntil - Date.now()) / 1000)) : 0;
    }

    function begin() {
        ed.shownUuid = ed.uuid;
        ed.error = "";
        ed.dirty = false;
        ed.revertUntil = ed.uuid !== "" ? NetworkLink.revertUntil(ed.uuid) : 0;
        ed.tick();
        if (ed.revertLeft > 0)
            countdown.restart();
        else
            countdown.stop();
        ed.load();
    }

    function load() {
        if (ed.uuid === "")
            return;
        ed.loaded = false;
        var uuid = ed.uuid;
        NetworkLink.ipGet(uuid, function (ok, error, result) {
            if (uuid !== ed.uuid)
                return;
            if (!ok || !result) {
                ed.error = error || I18n.tr("Could not read this network's IP settings.");
                return;
            }
            ed.method = ed.tunnel || result.method === "manual" ? "manual" : "auto";
            addressField.text = result.address || "";
            gatewayField.text = result.gateway || "";
            dnsField.text = (result.dns || []).join(", ");
            ed.dirty = false;
            ed.error = "";
            ed.loaded = true;
        });
    }

    function apply() {
        ed.busy = true;
        ed.error = "";
        NetworkLink.ipSet(ed.uuid, {
            method: ed.method,
            address: ed.manual ? addressField.text.trim() : "",
            gateway: ed.manual ? gatewayField.text.trim() : "",
            dns: ed.servers(dnsField.text)
        }, function (ok, error, result) {
            ed.busy = false;
            if (!ok) {
                ed.error = error || I18n.tr("Could not apply these settings.");
                return;
            }
            ed.dirty = false;
            var secs = result && result.revertSeconds ? result.revertSeconds : 0;
            ed.revertUntil = secs > 0 ? Date.now() + secs * 1000 : 0;
            NetworkLink.setRevertUntil(ed.uuid, ed.revertUntil);
            ed.tick();
            if (ed.revertLeft > 0)
                countdown.restart();
        });
    }

    function settle() {
        countdown.stop();
        ed.revertUntil = 0;
        ed.revertLeft = 0;
        ed.dirty = false;
        NetworkLink.setRevertUntil(ed.uuid, 0);
    }

    function keep() {
        ed.settle();
        NetworkLink.ipConfirm(ed.uuid, function (ok, error) {
            if (!ok)
                ed.error = error || I18n.tr("The settings were already undone.");
            ed.load();
        });
    }

    function undo() {
        ed.settle();
        NetworkLink.ipRevertNow(ed.uuid, function (ok, error) {
            if (!ok)
                ed.error = error;
            ed.load();
        });
    }

    property bool built: false
    onUuidChanged: if (ed.built) ed.begin()
    Component.onCompleted: {
        ed.built = true;
        ed.begin();
    }

    Connections {
        target: NetworkLink
        function onUsableChanged() {
            if (NetworkLink.usable && !ed.loaded)
                ed.load();
        }
    }

    Timer {
        id: countdown
        interval: 1000
        repeat: true
        onTriggered: {
            ed.tick();
            if (ed.revertLeft === 0) {
                stop();
                ed.revertUntil = 0;
                NetworkLink.setRevertUntil(ed.uuid, 0);
                // the daemon has put the old settings back; show them
                reverted.restart();
            }
        }
    }

    Timer {
        id: reverted
        interval: 1500
        onTriggered: ed.load()
    }

    SettingRow {
        visible: !ed.tunnel
        width: parent.width
        title: I18n.tr("Addressing")
        detail: I18n.tr("Where this computer gets its address")

        Segmented {
            id: methodSeg
            anchors.verticalCenter: parent.verticalCenter
            enabled: ed.loaded && !ed.busy && NetworkLink.usable
            model: [I18n.tr("Automatic (DHCP)"), I18n.tr("Manual")]
            accessibleName: I18n.tr("How this computer gets its address")
            onActivated: index => {
                ed.method = index === 0 ? "auto" : "manual";
                ed.dirty = true;
            }
            Binding {
                target: methodSeg
                property: "currentIndex"
                value: ed.method === "manual" ? 1 : 0
            }
        }
    }

    Flow {
        visible: ed.manual
        width: parent.width
        spacing: LithicTokens.space.s3

        TextField {
            id: addressField
            width: ed.tunnel || !ed.wide ? parent.width : (parent.width - parent.spacing) * 0.55
            label: I18n.tr("Address")
            mono: true
            enabled: ed.loaded && !ed.busy && NetworkLink.usable
            placeholderText: ed.tunnel ? "10.0.0.2/32" : "192.168.1.50/24"
            onTextEdited: ed.dirty = true
            onAccepted: if (ed.dirty && ed.valid) ed.apply()
        }
        TextField {
            id: gatewayField
            visible: !ed.tunnel
            width: ed.wide ? (parent.width - parent.spacing) * 0.45 : parent.width
            label: I18n.tr("Gateway")
            mono: true
            enabled: ed.loaded && !ed.busy && NetworkLink.usable
            placeholderText: "192.168.1.1"
            onTextEdited: ed.dirty = true
            onAccepted: if (ed.dirty && ed.valid) ed.apply()
        }
    }

    TextField {
        id: dnsField
        width: parent.width
        label: I18n.tr("DNS servers")
        helpText: ed.tunnel ? I18n.tr("Separated by spaces or commas. Empty keeps the tunnel's own.")
                            : I18n.tr("Separated by spaces or commas. Empty uses the network's own.")
        mono: true
        enabled: ed.loaded && !ed.busy && NetworkLink.usable
        placeholderText: ed.tunnel ? I18n.tr("none") : I18n.tr("from the network")
        onTextEdited: ed.dirty = true
        onAccepted: if (ed.dirty && ed.valid) ed.apply()
    }

    Row {
        visible: ed.revertLeft === 0
        anchors.right: parent.right
        spacing: LithicTokens.space.s2

        Plaque {
            kind: "gold"
            small: true
            busy: ed.busy
            enabled: ed.loaded && ed.dirty && ed.valid && NetworkLink.usable
            text: ed.busy ? I18n.tr("Applying…") : I18n.tr("Apply")
            onClicked: ed.apply()
        }
    }

    Item {
        visible: ed.revertLeft > 0
        width: parent.width
        height: Math.max(keepNote.implicitHeight, keepRow.implicitHeight)

        Label {
            id: keepNote
            anchors.left: parent.left
            anchors.right: keepRow.left
            anchors.rightMargin: LithicTokens.space.s3
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Keep these settings? They are undone in %1 s unless you keep them.").arg(ed.revertLeft)
            typeRole: "detail"
            inkRole: "fg"
            numeric: true
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Row {
            id: keepRow
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: LithicTokens.space.s2

            Plaque {
                kind: "tonal"
                small: true
                text: I18n.tr("Undo now")
                onClicked: ed.undo()
            }
            Plaque {
                kind: "gold"
                small: true
                text: I18n.tr("Keep")
                onClicked: ed.keep()
            }
        }
    }

    Label {
        visible: ed.error !== "" || NetworkLink.unavailable !== ""
        width: parent.width
        text: NetworkLink.unavailable !== "" ? NetworkLink.unavailable : ed.error
        typeRole: "detail"
        ink: Lithic.jasper
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}
