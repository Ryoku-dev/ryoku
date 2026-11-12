pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

// IPv4 settings for one saved connection (by UUID): DHCP or a manual address,
// gateway and DNS, read and written through the shell daemon (network.ipGet /
// network.ipSet). On a live connection the daemon puts the old settings back
// after a countdown unless they are kept here, so a wrong address or gateway
// that cuts the machine off undoes itself.
Item {
    id: ed

    property string uuid: ""
    property bool loaded: false
    property bool busy: false
    property string error: ""
    property string method: "auto"
    // seconds left before the daemon reverts; 0 when nothing is waiting
    property int revertLeft: 0
    // when the daemon reverts (ms since the epoch); 0 when nothing is waiting
    property real revertUntil: 0
    // edited here since the last load or apply
    property bool dirty: false
    // the uuid whose settings the fields hold
    property string shownUuid: ""
    // the field being typed in ("address", "gateway", "dns"; "" for none),
    // so a rebuilt editor gives it the keyboard back
    property string focusField: ""

    implicitHeight: col.implicitHeight + Tokens.s4 * 2

    function servers(text) {
        return String(text || "").split(/[\s,]+/).filter(function(s) { return s.length > 0; });
    }

    // The list that holds this editor may rebuild its rows on any refresh,
    // destroying it. Edits not applied yet and a running revert countdown are
    // kept in NetworkTopic so the rebuilt editor picks them up instead of
    // losing what was typed, the field being typed in or the Keep button.
    function stash(uuid) {
        if (!uuid.length)
            return;
        var waiting = ed.revertUntil > Date.now();
        NetworkTopic.ipStash(uuid, ed.dirty || waiting || ed.focusField !== "" ? {
            method: ed.method,
            address: addressField.text,
            gateway: gatewayField.text,
            dns: dnsField.text,
            dirty: ed.dirty,
            focus: ed.focusField,
            revertUntil: waiting ? ed.revertUntil : 0
        } : null);
    }
    function begin() {
        if (ed.shownUuid.length && ed.shownUuid !== ed.uuid)
            ed.stash(ed.shownUuid);
        ed.shownUuid = ed.uuid;
        var kept = NetworkTopic.ipStashed(ed.uuid);
        if (!kept) {
            ed.load();
            return;
        }
        ed.method = kept.method;
        addressField.text = kept.address;
        gatewayField.text = kept.gateway;
        dnsField.text = kept.dns;
        ed.dirty = kept.dirty;
        ed.revertUntil = kept.revertUntil;
        ed.error = "";
        ed.loaded = true;
        ed.tick();
        if (ed.revertLeft > 0)
            countdown.restart();
        var field = ({ address: addressField, gateway: gatewayField, dns: dnsField })[kept.focus];
        if (field)
            field.grabFocus();
    }
    // a field losing focus only clears focusField once nothing else in the
    // box has it; the check waits a turn, so a rebuild (which drops focus as
    // it tears the box down) still stashes the field that had it
    function focusMoved(key, focused) {
        if (focused)
            ed.focusField = key;
        else
            blurCheck.restart();
    }
    function tick() {
        ed.revertLeft = ed.revertUntil > 0 ? Math.max(0, Math.ceil((ed.revertUntil - Date.now()) / 1000)) : 0;
    }
    function load() {
        if (!ed.uuid.length)
            return;
        ed.loaded = false;
        var uuid = ed.uuid;
        NetworkTopic.ipGet(uuid, function(ok, error, result) {
            if (uuid !== ed.uuid)
                return;
            if (!ok || !result) {
                ed.error = error || I18n.tr("Could not read this network's IP settings.");
                return;
            }
            ed.method = result.method === "manual" ? "manual" : "auto";
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
        NetworkTopic.ipSet(ed.uuid, {
            method: ed.method,
            address: ed.method === "manual" ? addressField.text.trim() : "",
            gateway: ed.method === "manual" ? gatewayField.text.trim() : "",
            dns: ed.servers(dnsField.text)
        }, function(ok, error, result) {
            ed.busy = false;
            if (!ok) {
                ed.error = error || I18n.tr("Could not apply these settings.");
                return;
            }
            ed.dirty = false;
            var secs = result && result.revertSeconds ? result.revertSeconds : 0;
            ed.revertUntil = secs > 0 ? Date.now() + secs * 1000 : 0;
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
        NetworkTopic.ipStash(ed.uuid, null);
    }
    function keep() {
        ed.settle();
        NetworkTopic.ipConfirm(ed.uuid, function(ok, error) {
            if (!ok)
                ed.error = error || I18n.tr("The settings were already undone.");
            ed.load();
        });
    }
    function undo() {
        ed.settle();
        NetworkTopic.ipRevert(ed.uuid, function(ok, error) {
            if (!ok)
                ed.error = error;
            ed.load();
        });
    }

    // a uuid given at creation starts on completion; a later one (a Loader
    // binding it after creation) starts again
    property bool built: false
    onUuidChanged: if (ed.built) ed.begin()
    Component.onCompleted: {
        ed.built = true;
        ed.begin();
    }
    Component.onDestruction: ed.stash(ed.shownUuid)

    Timer {
        id: countdown
        interval: 1000
        repeat: true
        onTriggered: {
            ed.tick();
            if (ed.revertLeft === 0) {
                stop();
                ed.revertUntil = 0;
                NetworkTopic.ipStash(ed.uuid, null);
                // the daemon has put the old settings back; show them
                revertedReload.restart();
            }
        }
    }
    Timer {
        id: blurCheck
        interval: 0
        onTriggered: {
            if (!addressField.focused && !gatewayField.focused && !dnsField.focused)
                ed.focusField = "";
        }
    }
    Timer {
        id: revertedReload
        interval: 1500
        onTriggered: ed.load()
    }

    // the label column of the box; the fields fill what is left
    readonly property int labelWidth: 84
    readonly property int fieldsWidth: Math.max(160, ed.width - Tokens.s4 * 3 - ed.labelWidth)

    component BoxLabel: Text {
        width: ed.labelWidth
        color: Tokens.inkDim
        font.family: Tokens.ui
        font.pixelSize: Tokens.fMicro
        font.weight: Font.Medium
        font.letterSpacing: Tokens.trackLabel
    }

    Rectangle {
        id: box
        width: ed.width
        height: col.implicitHeight + Tokens.s4 * 2
        color: "transparent"
        radius: Tokens.radius
        border.width: Tokens.border
        border.color: Tokens.line
    }

    Column {
        id: col
        x: Tokens.s4
        y: Tokens.s4
        width: ed.width - Tokens.s4 * 2
        spacing: Tokens.s3
        enabled: ed.loaded && !ed.busy

        Row {
            spacing: Tokens.s4
            BoxLabel {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("METHOD")
            }
            Seg {
                anchors.verticalCenter: parent.verticalCenter
                options: ["auto", "manual"]
                labels: ({ "auto": I18n.tr("Automatic (DHCP)"), "manual": I18n.tr("Manual") })
                current: ed.method
                onChose: function(key) {
                    ed.method = key;
                    ed.dirty = true;
                }
            }
        }

        Row {
            visible: ed.method === "manual"
            spacing: Tokens.s4
            BoxLabel {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("ADDRESS")
            }
            Row {
                spacing: Tokens.s2
                Field {
                    id: addressField
                    onEdited: ed.dirty = true
                    onFocusedChanged: ed.focusMoved("address", focused)
                    width: Math.min(200, (ed.fieldsWidth - gatewayLabel.implicitWidth - Tokens.s2 * 2) * 0.55)
                    tabular: true
                    placeholder: "192.168.1.50/24"
                }
                Text {
                    id: gatewayLabel
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("GATEWAY")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro
                    font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackLabel
                }
                Field {
                    id: gatewayField
                    onEdited: ed.dirty = true
                    onFocusedChanged: ed.focusMoved("gateway", focused)
                    width: Math.min(160, (ed.fieldsWidth - gatewayLabel.implicitWidth - Tokens.s2 * 2) * 0.45)
                    tabular: true
                    placeholder: "192.168.1.1"
                }
            }
        }

        Row {
            spacing: Tokens.s4
            BoxLabel {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("DNS")
            }
            Row {
                spacing: Tokens.s2
                Field {
                    id: dnsField
                    onEdited: ed.dirty = true
                    onFocusedChanged: ed.focusMoved("dns", focused)
                    width: Math.min(300, ed.fieldsWidth - applyBtn.width - Tokens.s2)
                    tabular: true
                    placeholder: I18n.tr("from the network")
                }
                Btn {
                    id: applyBtn
                    anchors.verticalCenter: parent.verticalCenter
                    visible: ed.revertLeft === 0
                    compact: true
                    primary: true
                    armed: ed.loaded && !ed.busy && (ed.method === "auto" || addressField.text.trim().length > 0)
                    text: ed.busy ? I18n.tr("Applying...") : I18n.tr("Apply")
                    onAct: ed.apply()
                }
            }
        }

        Row {
            spacing: Tokens.s2
            visible: ed.revertLeft > 0
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Keep these settings? Reverting in %1 s").arg(ed.revertLeft)
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
            Btn {
                compact: true
                primary: true
                text: I18n.tr("Keep")
                onAct: ed.keep()
            }
            Btn {
                compact: true
                text: I18n.tr("Revert now")
                onAct: ed.undo()
            }
        }

        Text {
            visible: ed.error !== ""
            width: parent.width
            text: ed.error
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.WordWrap
        }
    }
}
