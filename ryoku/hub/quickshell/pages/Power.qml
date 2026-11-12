import QtQuick
import Quickshell
import Quickshell.Services.UPower
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../schema/power.js" as Schema

// the power mode and what each mode does, the battery, power saver and the idle timers
Item {
    id: root

    required property var hub
    // the power mode whose processor settings are shown under More options
    property string editMode: ""
    // a write has run past a second
    property bool slow: false

    readonly property var power: Changes.stores["power"] || null
    readonly property var modes: Changes.stores["powermode"] || null
    readonly property var battery: UPower.displayDevice
    readonly property bool hasBattery: !!(root.battery && root.battery.ready && root.battery.isLaptopBattery)

    readonly property var modeLabels: ({ "power-saver": "Power saver", "balanced": "Balanced", "performance": "Performance" })
    readonly property var idleLabels: ({
        "battery.dimSec": I18n.tr("Dim time on battery"), "battery.lockSec": I18n.tr("Lock time on battery"),
        "battery.screenOffSec": I18n.tr("Screen off time on battery"), "battery.suspendSec": I18n.tr("Sleep time on battery"),
        "ac.dimSec": I18n.tr("Dim time when plugged in"), "ac.lockSec": I18n.tr("Lock time when plugged in"),
        "ac.screenOffSec": I18n.tr("Screen off time when plugged in"), "ac.suspendSec": I18n.tr("Sleep time when plugged in")
    })

    // rows come from what the machine reports, so a deep link follows them until it has answered
    property string pendingRow: ""
    readonly property bool dataReady: Changes.revision >= 0 && !!root.power && root.power.ready && !!root.modes && (root.modes.heard || root.modes.silent)

    function focusRow(rowId) {
        root.pendingRow = rowId || "";
        settle.restart();
    }

    // a link lands as soon as its row or section is drawn; rows above it may still
    // arrive while the machine answers (the mode waits on the desktop service), so
    // it lands again on each redraw until the answers are in, or a few seconds pass
    function landPending(force) {
        if (root.pendingRow === "" || page.layout.length === 0)
            return;
        var id = root.pendingRow;
        var known = !!page.sectionOfRow(id) || page.layout.some(s => s.id === id);
        if (known || force === true)
            page.focusRow(id);
        if (force === true || (known && root.dataReady)) {
            giveUp.stop();
            root.pendingRow = "";
        } else if (!giveUp.running) {
            giveUp.restart();
        }
    }
    onDataReadyChanged: settle.restart()
    // rows redraw while the machine's answers arrive; land once they rest
    Timer {
        id: settle
        interval: 400
        onTriggered: root.landPending()
    }
    Timer {
        id: giveUp
        interval: 5000
        onTriggered: root.landPending(true)
    }

    Component.onCompleted: {
        Changes.ensureStore("power", Qt.resolvedUrl("../components/power/PowerStore.qml"));
        Changes.ensureStore("powermode", Qt.resolvedUrl("../components/power/PowerModeStore.qml"));
        Changes.ensureStore("performance", Qt.resolvedUrl("../components/performance/PerformanceStore.qml"));
        // a store kept from an earlier visit reads the machine again
        if (root.power && root.power.ready)
            root.power.refresh();
    }

    readonly property string shownMode: {
        if (!root.power)
            return "";
        if (root.editMode !== "" && root.power.profiles.indexOf(root.editMode) >= 0)
            return root.editMode;
        // the topic is the live mode; the backend's answer is from when the page read it
        var live = root.modes && root.modes.heard ? root.modes.profile : "";
        if (live !== "" && root.power.profiles.indexOf(live) >= 0)
            return live;
        return root.power.active || root.power.profiles[0] || "";
    }
    // a mode the page has not read yet is read when it is shown
    readonly property string capsToRead: !!root.power && root.power.ready && root.shownMode !== "" && !root.power.caps[root.shownMode] ? root.shownMode : ""
    onCapsToReadChanged: {
        if (root.capsToRead !== "")
            root.power.load(root.capsToRead);
    }

    function title(s) {
        s = String(s || "").replace(/[-_]/g, " ");
        return s.length ? s.charAt(0).toUpperCase() + s.slice(1) : "";
    }

    // the schema's labels for the values this machine reports, in its order
    function optionsFor(row, values, numeric) {
        var known = {};
        (row.options || []).forEach(o => known[String(o.value)] = o.label);
        return (values || []).map(v => ({
            value: numeric ? Number(v) : v,
            label: known[String(v)] !== undefined ? known[String(v)] : (numeric ? v + "%" : root.title(v))
        }));
    }

    function settingsBool(key, fallback) {
        var v = Settings.get(key);
        return typeof v === "boolean" ? v : fallback;
    }

    function settingsNumber(key, fallback) {
        var v = Settings.get(key);
        return typeof v === "number" ? Math.round(v) : fallback;
    }

    // one schema row as it applies to this machine, or null when it does not
    function prepare(r) {
        if (r.onBattery && !root.hasBattery)
            return null;
        if (r.noBattery && root.hasBattery)
            return null;
        if (r.section === "battery" && !root.hasBattery)
            return null;
        if (r.section === "idle-battery" && !root.hasBattery)
            return null;

        if (r.id === "power-mode") {
            // no options while the mode is unknown, so no segment claims to be the one in use
            if (!root.modes || !root.modes.heard)
                return root.modes && root.modes.silent ? Object.assign({}, r, { options: [], unavailable: I18n.tr("The desktop service did not answer, so the power mode is unknown") }) : null;
            var offered = root.modes.profiles;
            if (offered.length === 0)
                return Object.assign({}, r, { unavailable: I18n.tr("The power profiles service is not running") });
            return Object.assign({}, r, { options: r.options.filter(o => offered.indexOf(o.value) >= 0) });
        }

        if (r.id === "cpu-mode")
            return root.power && root.power.profiles.length > 0 && Schema.rows.some(x => x.cpu && root.power.tunable("profile." + root.shownMode + "." + x.cpu))
                ? r : null;

        if (r.cpu || r.battery || r.idle) {
            if (!root.power)
                return null;
            var key = r.cpu ? "profile." + root.shownMode + "." + r.cpu
                : r.battery ? "battery." + r.battery : "idle." + r.idle;
            var t = root.power.tunable(key);
            if (!t) {
                // the idle switch says why the idle times are missing
                if (r.id === "idle-on" && root.power.ready && root.power.error !== "")
                    return Object.assign({}, r, { unavailable: I18n.tr("The power service did not answer") });
                return null;
            }
            var out = Object.assign({}, r, { bind: { store: "power", key: key } });
            if (r.cpu)
                out.changeLabel = I18n.tr("%1 in %2").arg(I18n.tr(r.label)).arg(I18n.tr(root.modeLabels[root.shownMode] || root.title(root.shownMode)));
            // both idle sets share their row labels, so the notice and Recently changed name the set
            else if (root.idleLabels[r.idle])
                out.changeLabel = root.idleLabels[r.idle];
            if (t.kind === "segment") {
                var numeric = r.battery === "chargeLimit";
                out.options = root.optionsFor(r, t.options, numeric);
                if (r.control === "slider")
                    out.control = out.options.length <= 5 ? "segmented" : "picker";
            } else if (t.kind === "slider" || t.kind === "stepper") {
                out.min = t.min || 0;
                out.max = t.max;
                out.step = 1;
            }
            if (r.idle && r.idle !== "enabled" && r.idle !== "onDesktops")
                out.when = { bind: { store: "power", key: "idle.enabled" }, is: [true] };
            return out;
        }

        var alerts = root.settingsBool("batteryAlerts.enabled", true);
        var warning = root.settingsNumber("batteryAlerts.warningPercent", 25);
        var critical = root.settingsNumber("batteryAlerts.criticalPercent", 10);
        var alertsOn = { bind: { store: "settings", key: "batteryAlerts.enabled", fallback: true }, is: [true] };
        switch (r.id) {
        case "battery-warning":
        case "battery-critical":
        case "battery-test":
            return Object.assign({}, r, { when: alertsOn });
        case "battery-warning-level":
            if (!alerts)
                return null;
            return Object.assign({}, r, { min: Math.max(5, critical + 1),
                when: { bind: { store: "settings", key: "batteryAlerts.warningEnabled", fallback: true }, is: [true] } });
        case "battery-critical-level":
            if (!alerts)
                return null;
            return Object.assign({}, r, { max: Math.max(1, warning - 1),
                when: { bind: { store: "settings", key: "batteryAlerts.criticalEnabled", fallback: true }, is: [true] } });
        }
        return r;
    }

    readonly property var schema: {
        Changes.revision;
        Settings.revision;
        var rows = Schema.rows.map(r => root.prepare(r)).filter(r => r !== null);
        return { title: Schema.title, lead: Schema.lead, sections: Schema.sections, related: Schema.related, rows: rows };
    }

    readonly property string batteryLine: {
        if (!root.hasBattery)
            return "";
        var pct = Math.round(Math.max(0, Math.min(1, root.battery.percentage)) * 100);
        switch (root.battery.state) {
        case UPowerDeviceState.Charging:
            return I18n.tr("Battery at %1%, charging.").arg(pct);
        case UPowerDeviceState.FullyCharged:
            return I18n.tr("Battery full.");
        case UPowerDeviceState.PendingCharge:
            return I18n.tr("Battery at %1%, plugged in and not charging.").arg(pct);
        }
        return UPower.onBattery ? I18n.tr("Running on battery at %1%.").arg(pct) : I18n.tr("Battery at %1%.").arg(pct);
    }

    readonly property string stateLine: {
        var parts = [];
        var m = root.modes ? root.modes.profile : "";
        if (m !== "")
            parts.push(I18n.tr("%1 power mode.").arg(I18n.tr(root.modeLabels[m] || root.title(m))));
        if (root.batteryLine !== "")
            parts.push(root.batteryLine);
        if (root.slow)
            parts.push(I18n.tr("Applying your change…"));
        return parts.join(" ");
    }

    readonly property bool writing: (!!root.power && Object.keys(root.power.busy).length > 0) || (!!root.modes && root.modes.calls > 0)
    onWritingChanged: {
        if (root.writing) {
            slowTimer.restart();
        } else {
            slowTimer.stop();
            root.slow = false;
        }
    }
    Timer {
        id: slowTimer
        interval: 1000
        onTriggered: root.slow = true
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "power"
        schema: root.schema
        stateLine: root.stateLine
        onLayoutChanged: settle.restart()
        controls: ({ "cpu-mode": modeChooser, "battery-test": testAlert })
        footers: root.power && root.power.ready && root.power.error !== "" ? ({ mode: readFailed }) : ({})
    }

    Component {
        id: readFailed
        Row {
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.parent ? parent.parent.width - retry.width - parent.spacing : implicitWidth)
                text: root.power ? root.power.error : ""
                typeRole: "detail"
                ink: Lithic.jasper
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: retry
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.power.refresh()
            }
        }
    }

    Component {
        id: modeChooser
        Segmented {
            property var row
            readonly property var list: root.power ? root.power.profiles : []
            width: Math.min(implicitWidth, LithicTokens.size.settingsControlMax)
            model: list.map(p => I18n.tr(root.modeLabels[p] || root.title(p)))
            currentIndex: Math.max(0, list.indexOf(root.shownMode))
            accessibleName: I18n.tr("Change what a mode does")
            // capsToRead reads a mode the page has not read yet
            onActivated: index => root.editMode = list[index]
        }
    }

    Component {
        id: testAlert
        Row {
            id: test
            property var row
            property bool sending: false
            property string result: ""
            property bool failed: false
            function activate() {
                if (test.sending)
                    return;
                test.sending = true;
                test.result = "";
                // a call with a deadline, so a daemon that never answers ends in Retry
                Changes.call("battery-alerts.test", {}, function (ok, error) {
                    test.sending = false;
                    test.failed = ok !== true;
                    test.result = ok === true ? I18n.tr("Sent")
                        : /did not answer/i.test(String(error)) ? I18n.tr("Not sent, because the desktop service did not answer. Retry once it is running.")
                        : I18n.tr("Not sent, because the desktop could not show it. Retry, and if it keeps failing, run ryoku doctor.");
                });
            }
            spacing: LithicTokens.space.s3
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, LithicTokens.size.settingsControlMax - send.width - parent.spacing)
                visible: test.result !== ""
                text: test.result
                typeRole: "detail"
                ink: test.failed ? Lithic.jasper : Lithic.fg2
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: send
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "notifications"
                busy: test.sending
                text: test.failed ? I18n.tr("Retry") : I18n.tr("Send")
                accessibleName: I18n.tr("Send a test battery alert")
                onClicked: test.activate()
            }
        }
    }
}
