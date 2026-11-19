pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../lib/search.js" as Search
import "phone.js" as Phone

Item {
    id: root
    property var memory: null
    property var downloads: null
    property bool active: false
    property bool current: false
    property bool searching: false
    property string pageId: "overview"
    property var state: ({})
    property var contacts: []
    property var apps: []
    property int revision: 0
    property var knownNotifications: null
    property var pendingNotifications: ({})
    property string knownCall: ""
    property bool pendingCall: false
    property var actionQueue: []
    property var actionRequest: null
    property int nextRequestId: 0
    readonly property var devices: Array.isArray(state.devices) ? state.devices.filter(d => d && d.paired) : []
    readonly property var device: Phone.device(state)
    readonly property bool ready: state.enabled === true && state.installed === true && device !== null
    readonly property bool reachable: ready && device.reachable === true
    readonly property int charge: reachable && Number(device.charge) >= 0 ? Number(device.charge) : -1
    readonly property var notifications: ready && Array.isArray(state.notifications) ? state.notifications : []
    readonly property var figure: ({ text: charge >= 0 ? charge + "%" : ready ? I18n.tr("Not connected") : "", value: charge >= 0 ? charge / 100 : -1, tone: "" })
    readonly property var lit: pendingCall ? ({ text: I18n.tr("Incoming call"), tone: "wait" }) : Object.keys(pendingNotifications).length ? ({ text: I18n.tr("%1 new").arg(Object.keys(pendingNotifications).length), tone: "wait" }) : null
    readonly property bool readContacts: active && ready && (searching || (current && pageId === "contacts"))
    readonly property bool readApps: active && ready && (searching || (current && pageId === "apps"))
    signal failure(string text)
    signal completed(int requestId, bool success)

    function seen() { pendingNotifications = ({}); pendingCall = false; }
    onCurrentChanged: if (current) seen()
    onActiveChanged: if (!active) cancelActions()
    Component.onDestruction: cancelActions()
    function readState() {
        if (!active) return;
        try {
            const parsed = JSON.parse(file.text());
            if (!parsed || Number(parsed.schema) !== 1) return;
            const delta = Phone.arrivals(knownNotifications, pendingNotifications, parsed.notifications || [], current);
            knownNotifications = delta.known;
            pendingNotifications = delta.pending;
            const call = parsed.call || {};
            const key = call.state === "ringing" ? String(call.number || call.name || "ringing") : "";
            if (key && key !== knownCall && !current) pendingCall = true;
            knownCall = key;
            state = parsed;
            revision++;
        } catch (error) {}
    }
    function readList(fileView, kind) {
        if (kind === "contacts" ? !readContacts : !readApps) return;
        try {
            const rows = JSON.parse(fileView.text());
            if (!Array.isArray(rows)) return;
            if (kind === "contacts") contacts = rows; else apps = rows;
            revision++;
        } catch (error) {}
    }
    function call(name, args) {
        if (!active) return 0;
        const request = { id: ++nextRequestId, command: ["ryoku-shell", "ipc", "call", "phone", name].concat((args || []).map(value => String(value))) };
        actionQueue = actionQueue.concat([request]);
        startNextAction();
        return request.id;
    }
    function startNextAction() {
        if (!active || actionProc.running || actionRequest || !actionQueue.length) return;
        actionRequest = actionQueue[0];
        actionQueue = actionQueue.slice(1);
        actionProc.didStart = false;
        actionProc.command = actionRequest.command;
        actionProc.running = true;
    }
    function finishAction(code) {
        const request = actionRequest;
        actionRequest = null;
        if (active && request) {
            const reply = actionOut.text.trim();
            const refused = reply.indexOf("err ") === 0;
            const success = code === 0 && !refused;
            if (!success) failure(refused ? reply.slice(4) : actionErr.text.trim() || I18n.tr("Phone action failed"));
            completed(request.id, success);
        }
        Qt.callLater(startNextAction);
    }
    function cancelActions() {
        actionQueue = [];
        actionRequest = null;
        if (actionProc.running) actionProc.signal(9);
    }
    function searchAll(query) {
        const pages = [
            { id: "overview", title: I18n.tr("Overview"), glyph: "smartphone" },
            { id: "notifications", title: I18n.tr("Notifications"), glyph: "notifications" },
            { id: "screen", title: I18n.tr("Screen"), glyph: "cast", texts: [I18n.tr("Mirror"), I18n.tr("Record screen"), I18n.tr("Webcam"), I18n.tr("Microphone")] },
            { id: "contacts", title: I18n.tr("Contacts"), glyph: "contacts" },
            { id: "apps", title: I18n.tr("Apps"), glyph: "apps" }
        ];
        let rows = Search.listRows(pages, query, p => ({ id: "page-" + p.id, glyph: p.glyph, title: p.title, subtitle: device ? String(device.name || "") : I18n.tr("Set up your phone"), texts: p.texts || [], open: { page: p.id } }));
        rows = rows.concat(Search.listRows(notifications, query, n => ({ id: "note-" + n.id, glyph: "notifications", title: n.title || n.app, subtitle: n.body || "", texts: [n.app || ""], open: { page: "notifications", query: n.title || n.app || "" } })));
        rows = rows.concat(Search.listRows(contacts, query, c => ({ id: "contact-" + c.id, glyph: "person", title: c.name, subtitle: Phone.numbers(c).join(" · "), open: { page: "contacts", query: c.name } })));
        return rows.concat(Search.listRows(apps, query, a => ({ id: "app-" + a.package, glyph: "apps", title: a.label || a.package, subtitle: a.package, open: { page: "apps", query: a.label || a.package } })));
    }
    Process {
        id: actionProc
        property bool didStart: false
        running: false
        environment: Spawn.env
        stdout: StdioCollector { id: actionOut }
        stderr: StdioCollector { id: actionErr }
        onStarted: didStart = true
        onExited: (code, status) => root.finishAction(code)
        onRunningChanged: if (!running && !didStart) root.finishAction(-1)
    }
    FileView {
        id: file
        path: root.active ? Quickshell.env("XDG_RUNTIME_DIR") + "/ryoku/phone/state.json" : ""
        watchChanges: root.active
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.readState()
    }
    FileView {
        id: contactsFile
        path: root.readContacts ? Quickshell.env("XDG_RUNTIME_DIR") + "/ryoku/phone/contacts.json" : ""
        watchChanges: root.readContacts
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.readList(contactsFile, "contacts")
    }
    FileView {
        id: appsFile
        path: root.readApps ? Quickshell.env("XDG_RUNTIME_DIR") + "/ryoku/phone/apps.json" : ""
        watchChanges: root.readApps
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.readList(appsFile, "apps")
    }
}
