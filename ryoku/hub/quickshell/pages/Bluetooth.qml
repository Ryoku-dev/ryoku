import QtQuick
import Quickshell.Io
import Quickshell.Bluetooth
import Lithic
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/wifi"
import "../schema/bluetooth.js" as Schema

// the Bluetooth radio and its devices; every change is a command (the power and link
// helpers, bluetoothctl), so nothing here writes to BlueZ over its bus
Item {
    id: root

    required property var hub
    property string pendingRow: ""

    readonly property var store: Changes.stores["bluetooth"] || null
    readonly property var powerBind: ({ store: "bluetooth", key: "power" })
    readonly property var adapter: Bluetooth.defaultAdapter
    readonly property bool hasAdapter: root.adapter !== null
    readonly property bool powered: root.hasAdapter && root.adapter.enabled === true
    readonly property bool discovering: root.hasAdapter && root.adapter.discovering === true
    readonly property var devices: Bluetooth.devices ? Bluetooth.devices.values : []
    property bool softBlocked: false
    property bool hardBlocked: false
    readonly property bool blocked: root.softBlocked || root.hardBlocked
        || (root.hasAdapter && root.adapter.state === BluetoothAdapterState.Blocked)

    // address -> device
    readonly property var byAddress: {
        var out = {};
        for (var i = 0; i < root.devices.length; i++) {
            var d = root.devices[i];
            if (d && d.address)
                out[d.address] = d;
        }
        return out;
    }
    readonly property int connectedCount: root.devices.filter(d => d && d.connected).length
    readonly property int pairedCount: root.devices.filter(d => d && d.paired).length

    // connected, then paired, then named, nameless addresses last; a scan does not churn useful rows
    readonly property var order: Object.keys(root.byAddress).sort(function (a, b) {
        var da = root.byAddress[a], db = root.byAddress[b];
        var r = root.rank(da) - root.rank(db);
        if (r !== 0)
            return r;
        return root.label(da).localeCompare(root.label(db));
    })
    onOrderChanged: root.syncList()

    // address of the device a helper is working on, and what it does: pair, connect, disconnect
    property string busyAddress: ""
    property string busyKind: ""
    // the running link, scan and service commands (see run), so each can be stopped
    property var linkJob: null
    property var errors: ({})
    property bool scanning: false
    property var scanJob: null
    property string scanError: ""
    property bool serviceBusy: false
    property var serviceJob: null
    property string serviceError: ""

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: []
    })

    readonly property string stateLine: !root.hasAdapter ? ""
        : !root.powered ? I18n.tr("Bluetooth is off.")
        : root.connectedCount === 1 ? I18n.tr("%1 is connected.").arg(root.label(root.devices.find(d => d && d.connected)))
        : root.connectedCount > 1 ? I18n.tr("%1 devices are connected.").arg(root.connectedCount)
        : I18n.tr("Bluetooth is on. Nothing is connected.")

    ListModel { id: devModel }

    function syncList() {
        var keys = root.order;
        for (var i = devModel.count - 1; i >= 0; i--)
            if (keys.indexOf(devModel.get(i).address) < 0)
                devModel.remove(i);
        for (var j = 0; j < keys.length; j++) {
            var at = -1;
            for (var k = j; k < devModel.count; k++)
                if (devModel.get(k).address === keys[j]) {
                    at = k;
                    break;
                }
            if (at < 0)
                devModel.insert(j, { address: keys[j] });
            else if (at !== j)
                devModel.move(at, j, 1);
        }
    }

    function rank(d) {
        if (!d)
            return 3;
        if (d.connected)
            return 0;
        if (d.paired)
            return 1;
        return RU.BtName.looksLikeAddress(RU.BtName.label(d)) ? 3 : 2;
    }

    function label(d) {
        var name = d ? RU.BtName.label(d) : "";
        return name !== "" ? name : I18n.tr("Unknown device");
    }

    // the freedesktop icon BlueZ reports, in words
    function kind(d) {
        var icon = d && d.icon ? String(d.icon) : "";
        if (/headset|headphone/.test(icon))
            return I18n.tr("Headphones");
        if (/audio|speaker/.test(icon))
            return I18n.tr("Speaker");
        if (/keyboard/.test(icon))
            return I18n.tr("Keyboard");
        if (/mouse/.test(icon))
            return I18n.tr("Mouse");
        if (/gaming|joystick/.test(icon))
            return I18n.tr("Game controller");
        if (/tablet/.test(icon))
            return I18n.tr("Drawing tablet");
        if (/phone/.test(icon))
            return I18n.tr("Phone");
        if (/computer/.test(icon))
            return I18n.tr("Computer");
        if (/camera|video/.test(icon))
            return I18n.tr("Camera");
        if (/printer/.test(icon))
            return I18n.tr("Printer");
        return "";
    }

    function battery(d) {
        if (!d || !d.batteryAvailable)
            return -1;
        var b = Number(d.battery);
        if (!(b > 0))
            return -1;
        return Math.round(b <= 1 ? b * 100 : b);
    }

    function detailFor(d) {
        if (!d)
            return "";
        var parts = [];
        var k = root.kind(d);
        if (k !== "")
            parts.push(k);
        if (root.busyAddress === d.address)
            parts.push(root.busyKind === "disconnect" ? I18n.tr("disconnecting…")
                       : root.busyKind === "pair" ? I18n.tr("pairing…") : I18n.tr("connecting…"));
        else if (d.connected)
            parts.push(I18n.tr("connected"));
        else if (d.paired)
            parts.push(I18n.tr("paired"));
        else
            parts.push(I18n.tr("not paired"));
        var b = root.battery(d);
        if (d.connected && b >= 0)
            parts.push(I18n.tr("battery %1%").arg(b));
        var text = parts.join(", ");
        return text.charAt(0).toUpperCase() + text.slice(1);
    }

    function setError(address, message) {
        var next = Object.assign({}, root.errors);
        if (message)
            next[address] = message;
        else
            delete next[address];
        root.errors = next;
    }

    // what BlueZ or a helper said, in words with the next step
    function linkReason(why, pairing) {
        var r = String(why || "");
        if (/Authentication(Failed|Canceled|Rejected|Timeout)|AuthenticationError/i.test(r))
            return pairing ? I18n.tr("The device refused to pair. Put it in pairing mode and try again.")
                           : I18n.tr("The device refused the connection. Turn it on, confirm on it if it asks, and try again.");
        if (/InProgress|Busy/i.test(r))
            return I18n.tr("The device is busy with another request. Try again in a moment.");
        if (/ConnectionAttemptFailed|page-timeout|HostDown|NotAvailable|DoesNotExist|not available|timed? ?out/i.test(r))
            return I18n.tr("The device did not answer. Make sure it is on and nearby, then try again.");
        var plain = r.replace(/^[\w-]+:\s*/, "").replace(/org\.bluez\.Error\.\w+:?\s*/g, "").trim();
        return plain !== "" ? I18n.tr("%1. Make sure the device is on and nearby, then try again.").arg(plain.replace(/\.$/, ""))
                            : I18n.tr("Make sure the device is on and nearby, then try again.");
    }

    function act(d) {
        if (!d || root.busyAddress !== "")
            return;
        if (d.connected)
            root.disconnect(d);
        else
            root.link(d);
    }

    // what a failed command said, in words with the next step; a missing program
    // already reads that way, a command past its limit means the device did not answer
    function failure(r, pairing, lead) {
        return lead.arg(r.missing ? r.why : root.linkReason(r.late ? "timed out" : r.why, pairing));
    }

    // the link helper stops discovery, pairs and trusts when needed, then connects;
    // it bounds every step itself, so 150 s only catches a helper that hangs
    function link(d) {
        root.stopScan();
        var addr = d.address, paired = d.paired;
        root.busyAddress = addr;
        root.busyKind = paired ? "connect" : "pair";
        root.setError(addr, "");
        root.linkJob = root.run(["ryoku-bluetooth-link", addr], function (r) {
            root.busyAddress = "";
            root.linkJob = null;
            if (!r.ok)
                root.setError(addr, paired ? root.failure(r, false, I18n.tr("Could not connect. %1"))
                                           : root.failure(r, true, I18n.tr("Pairing failed. %1")));
        }, 150000);
    }

    function disconnect(d) {
        var addr = d.address;
        root.busyAddress = addr;
        root.busyKind = "disconnect";
        root.setError(addr, "");
        root.linkJob = root.run(["bluetoothctl", "disconnect", addr], function (r) {
            root.busyAddress = "";
            root.linkJob = null;
            if (!r.ok)
                root.setError(addr, root.failure(r, false, I18n.tr("Could not disconnect. %1")));
        }, 20000);
    }

    function cancelLink() {
        var job = root.linkJob, addr = root.busyAddress, kind = root.busyKind;
        if (!job)
            return;
        root.stop(job);
        root.busyAddress = "";
        root.linkJob = null;
        root.setError(addr, I18n.tr("Stopped before it finished. Retry when you are ready."));
        if (kind === "pair")
            root.run(["bluetoothctl", "cancel-pairing", addr], null, 10000);
    }

    // discovery lasts as long as bluetoothctl runs, 25 s at most; stopping it ends the scan
    function startScan() {
        if (!root.powered || root.scanJob)
            return;
        root.scanning = true;
        root.scanError = "";
        root.scanJob = root.run(["bluetoothctl", "--timeout", "25", "scan", "on"], function (r) {
            root.scanJob = null;
            root.scanning = false;
            if (!r.ok)
                root.scanError = r.missing ? I18n.tr("Could not look for devices. %1").arg(r.why)
                               : r.late || r.why === "" ? I18n.tr("Could not look for devices. Scan again in a moment.")
                               : I18n.tr("Could not look for devices: %1. Scan again in a moment.")
                                     .arg(r.why.replace(/^[\w-]+:\s*/, "").replace(/org\.bluez\.Error\.\w+:?\s*/g, "").replace(/[:.]\s*$/, "").trim());
        }, 40000);
    }

    function stopScan() {
        root.stop(root.scanJob);
        root.scanJob = null;
        root.scanning = false;
    }

    // the service starts behind a password prompt, so it waits as long as the prompt does
    function startService() {
        if (root.serviceBusy)
            return;
        root.serviceBusy = true;
        root.serviceError = "";
        root.serviceJob = root.run(["pkexec", "ryoku-host", "svc", "--system", "enable", "--now", "bluetooth"], function (r) {
            root.serviceBusy = false;
            root.serviceJob = null;
            if (!r.ok)
                root.serviceError = r.missing ? r.why
                    : r.code === 126 || r.code === 127 || /another user|dismissed|not authori[sz]ed/i.test(r.why)
                    ? I18n.tr("The Bluetooth service did not start: the password prompt was closed or refused.")
                    : I18n.tr("Could not start the Bluetooth service: %1").arg(r.why || I18n.tr("no reason given"));
            rfkill.running = true;
        });
    }

    function cancelService() {
        if (!root.serviceJob)
            return;
        root.stop(root.serviceJob);
        root.serviceJob = null;
        root.serviceBusy = false;
        root.serviceError = I18n.tr("Stopped before it finished. Retry when you are ready.");
        rfkill.running = true;
    }

    // a press while the radio is still changing is ignored: the switch stays usable so the keys stay on it
    function flip(on) {
        if (root.store && root.store.busy)
            return;
        Changes.set(root.powerBind, on, I18n.tr("Bluetooth"), "bluetooth", on ? I18n.tr("Bluetooth turned on") : I18n.tr("Bluetooth turned off"));
    }

    // the keys are on a control inside item, put there from the keyboard
    function keysIn(item) {
        var f = root.Window.activeFocusItem;
        for (var p = f; p; p = p.parent)
            if (p === item)
                return !!f.visualFocus;
        return false;
    }

    // run argv once through Changes.run, after timeout ms at most when given; done({ ok, why,
    // code, late, missing }) is called once unless the job is stopped first. why is the last
    // line it printed, or for a missing program the plain words saying so. The job comes back
    // for stop(), or null when it already ended (a program that is not there at all)
    function run(argv, done, timeout) {
        var job = { proc: null, over: false, stopped: false };
        var p = Changes.run(argv, function (ok, out, err, code, late) {
            job.over = true;
            if (job.stopped || !done)
                return;
            var missing = !ok && code === -1 && !late;
            var lines = String(err + "\n" + out).split("\n").map(l => l.trim()).filter(l => l.length > 0);
            done({ ok: ok, code: code, late: !!late, missing: missing,
                   why: ok ? "" : missing ? String(err) : lines.length > 0 ? lines[lines.length - 1] : "" });
        }, { timeout: timeout || 0 });
        if (job.over)
            return null;
        job.proc = p;
        return job;
    }

    // end a job without hearing back from it
    function stop(job) {
        if (!job || job.over || job.stopped)
            return;
        job.stopped = true;
        if (job.proc)
            job.proc.running = false;
    }

    function sectionIndex(id) {
        for (var i = 0; i < page.layout.length; i++)
            if (page.layout[i].id === id)
                return i;
        return -1;
    }

    // a deep link names a row, or a section ("bluetooth/radio"), which needs no row
    function isSectionLink(id) {
        return !Schema.rows.some(r => r.id === id) && Schema.sections.some(s => s.id === id);
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (rowId !== root.pendingRow)
            focusRetry.tries = 0;
        if ((page.layout.length === 0 || (!page.rowItems[rowId] && !root.isSectionLink(rowId))) && focusRetry.tries < 30) {
            root.pendingRow = rowId;
            focusRetry.tries++;
            focusRetry.restart();
            return;
        }
        root.pendingRow = "";
        if (root.isSectionLink(rowId)) {
            page.focusRow(rowId);
            return;
        }
        var item = page.rowItems[rowId];
        if (item && item.visible) {
            page.land(rowId);
            return;
        }
        var row = Schema.rows.find(r => r.id === rowId);
        var at = root.sectionIndex(row ? row.section : "");
        if (at >= 0)
            page.chooseSection(at);
    }

    Timer {
        id: focusRetry
        property int tries: 0
        interval: 100
        onTriggered: root.focusRow(root.pendingRow)
    }

    Process {
        id: rfkill
        command: ["rfkill", "-n", "-o", "TYPE,SOFT,HARD", "list", "bluetooth"]
        stdout: StdioCollector {
            onStreamFinished: {
                var rows = this.text.trim().split("\n").map(l => l.trim().split(/\s+/)).filter(f => f[0] === "bluetooth");
                root.softBlocked = rows.some(f => f[1] === "blocked");
                root.hardBlocked = rows.some(f => f[2] === "blocked");
            }
        }
    }

    onAdapterChanged: rfkill.running = true
    onPoweredChanged: {
        if (!root.powered)
            root.stopScan();
    }

    // leaving the page mid-scan ends discovery so the radio is not left searching; a link or
    // the service start ends with the page too, as nothing is left to report it
    Component.onDestruction: {
        root.stop(root.scanJob);
        root.stop(root.linkJob);
        root.stop(root.serviceJob);
    }

    Component.onCompleted: {
        Changes.ensureStore("bluetooth", Qt.resolvedUrl("../components/bluetooth/BluetoothStore.qml"));
        root.syncList();
        rfkill.running = true;
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "bluetooth"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ radio: radioBody, devices: devicesBody })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Component {
        id: radioBody
        Slab {
            width: parent ? parent.width : 0

            Column {
                width: parent.width

                NetRow {
                    id: radioRow
                    readonly property bool busy: !!root.store && root.store.busy
                    // a soft block is lifted by turning the radio on, so the switch stays
                    readonly property bool switchable: root.hasAdapter || (root.softBlocked && !root.hardBlocked)
                    host: page
                    rowId: "bluetooth-on"
                    first: true
                    title: I18n.tr("Bluetooth")
                    // the switch keeps the keys while the radio changes; a press meanwhile is ignored
                    activatable: radioRow.switchable
                    unavailable: root.hardBlocked ? I18n.tr("Turned off by a switch or key on this computer") : ""
                    detail: root.serviceBusy ? I18n.tr("Starting the Bluetooth service…")
                          : radioPace.slow ? (root.powered ? I18n.tr("Turning off…") : I18n.tr("Turning on…"))
                          : !radioRow.switchable ? I18n.tr("The Bluetooth service is not running. Start it to use Bluetooth.")
                          : root.softBlocked && !root.powered ? I18n.tr("Off. The radio is blocked; turning it on unblocks it.")
                          : root.powered ? I18n.tr("On") : I18n.tr("Off")
                    error: root.serviceError !== "" ? root.serviceError : Changes.error(root.powerBind)
                    onActivated: root.flip(!root.powered)
                    onRetry: {
                        if (root.serviceError !== "")
                            root.startService();
                        else
                            Changes.retry(root.powerBind);
                    }

                    Pace { id: radioPace; running: radioRow.busy || root.serviceBusy }

                    Working {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: radioPace.slow && !root.serviceBusy
                        active: visible
                    }
                    Plaque {
                        id: serviceButton
                        anchors.verticalCenter: parent.verticalCenter
                        visible: !radioRow.switchable && !root.hardBlocked
                        // busy, not disabled, so the keys stay on it while the service starts
                        busy: root.serviceBusy && radioPace.slow
                        kind: "tonal"
                        small: true
                        text: I18n.tr("Start service")
                        onClicked: root.startService()
                        // once the service runs the switch takes its place, and the keys with it
                        onVisibleChanged: {
                            if (serviceButton.visible || !serviceButton.activeFocus)
                                return;
                            var reason = serviceButton.visualFocus ? Qt.TabFocusReason : Qt.OtherFocusReason;
                            Qt.callLater(() => radioToggle.forceActiveFocus(reason));
                        }
                    }
                    Toggle {
                        id: radioToggle
                        anchors.verticalCenter: parent.verticalCenter
                        visible: radioRow.switchable
                        checked: root.powered
                        Accessible.name: I18n.tr("Bluetooth")
                        onToggled: {
                            var want = radioToggle.checked;
                            radioToggle.checked = Qt.binding(() => root.powered);
                            root.flip(want);
                        }
                    }
                }

                LongWait {
                    visible: radioPace.lagging && root.serviceBusy
                    text: I18n.tr("Still starting the Bluetooth service. If a password prompt is open, answer it there.")
                    onCancelled: {
                        var keys = root.keysIn(parent);
                        root.cancelService();
                        if (keys && serviceButton.visible)
                            serviceButton.forceActiveFocus(Qt.TabFocusReason);
                    }
                }
            }
        }
    }

    Component {
        id: devicesBody
        Slab {
            width: parent ? parent.width : 0

            Column {
                width: parent.width

                NetRow {
                    host: page
                    rowId: "devices"
                    first: true
                    title: root.powered ? I18n.tr("Find a device")
                         : root.hasAdapter || (root.softBlocked && !root.hardBlocked) ? I18n.tr("Bluetooth is off") : I18n.tr("Bluetooth is not available")
                    detail: root.hardBlocked ? I18n.tr("Turn Bluetooth on with the switch or key on this computer")
                          : !root.hasAdapter && !root.softBlocked ? I18n.tr("Start the Bluetooth service above to pair and connect devices")
                          : !root.powered ? I18n.tr("Turn it on to pair and connect devices")
                          : root.scanning || root.discovering ? I18n.tr("Looking for devices nearby. Put the device in pairing mode.")
                          : root.connectedCount > 0 ? I18n.tr("%1 connected, %2 paired").arg(root.connectedCount).arg(root.pairedCount)
                          : root.pairedCount > 0 ? I18n.tr("%1 paired, none connected").arg(root.pairedCount)
                          : I18n.tr("Scan to find devices in pairing mode")
                    error: root.powered ? root.scanError : ""
                    onRetry: root.startScan()

                    Working {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: root.powered && (root.scanning || root.discovering)
                        active: visible
                    }
                    Plaque {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: root.powered
                        enabled: root.busyAddress === ""
                        kind: "quiet"
                        small: true
                        glyph: root.scanning ? "stop" : "refresh"
                        text: root.scanning ? I18n.tr("Stop") : I18n.tr("Scan")
                        onClicked: root.scanning ? root.stopScan() : root.startScan()
                    }
                }

                Repeater {
                    model: root.powered ? devModel : null
                    delegate: DeviceItem {}
                }
            }
        }
    }

    component DeviceItem: Column {
        id: devItem

        required property string address
        readonly property var dev: root.byAddress[devItem.address] || null
        readonly property bool mine: root.busyAddress === devItem.address

        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s2

        Pace { id: devPace; running: devItem.mine }

        NetRow {
            host: page
            title: root.label(devItem.dev)
            detail: root.detailFor(devItem.dev)
            error: root.errors[devItem.address] || ""
            onRetry: {
                // Retry goes away once the device is working again; the keys move to its button
                var keys = root.keysIn(devItem);
                root.act(devItem.dev);
                if (keys)
                    devAction.forceActiveFocus(Qt.TabFocusReason);
            }

            Plaque {
                id: devAction
                anchors.verticalCenter: parent.verticalCenter
                // the device being worked on keeps its button, busy rather than disabled, so the
                // keys stay on it; the others wait their turn. One read of busyAddress, so the
                // button is never disabled for a moment while it changes (that drops the keys)
                enabled: root.busyAddress === "" || root.busyAddress === devItem.address
                busy: root.busyAddress === devItem.address && devPace.slow
                kind: devItem.dev && devItem.dev.connected ? "quiet" : "tonal"
                small: true
                text: !devItem.dev ? "" : devItem.dev.connected ? I18n.tr("Disconnect")
                    : devItem.dev.paired ? I18n.tr("Connect") : I18n.tr("Pair")
                accessibleName: text + " " + root.label(devItem.dev)
                onClicked: root.act(devItem.dev)
            }
        }

        LongWait {
            visible: devPace.lagging && root.busyKind !== "disconnect"
            text: root.busyKind === "pair" ? I18n.tr("Still pairing with %1. Confirm on the device if it asks.").arg(root.label(devItem.dev))
                                           : I18n.tr("Still connecting to %1…").arg(root.label(devItem.dev))
            onCancelled: {
                // the panel goes away with the work; the keys go back to the device's button
                var keys = root.keysIn(devItem);
                root.cancelLink();
                if (keys)
                    devAction.forceActiveFocus(Qt.TabFocusReason);
            }
        }
    }
}
