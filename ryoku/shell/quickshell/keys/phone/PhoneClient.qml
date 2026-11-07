pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

Item {
    id: client

    readonly property string runtimeDir: Quickshell.env("XDG_RUNTIME_DIR") + "/ryoku/phone"
    property var state: client.emptyState()
    property var contacts: []
    property var apps: []
    property bool contactsOpened: false
    property bool appsOpened: false
    property bool shellDown: false
    property int revision: 0
    property bool stateLoaded: false

    readonly property var devices: Array.isArray(client.state.devices) ? client.state.devices : []
    readonly property var pairedDevices: client.devices.filter(device => device && device.paired === true)
    readonly property var activeDevice: {
        const wanted = String(client.state.activeDeviceId || "");
        for (let i = 0; i < client.pairedDevices.length; i++) {
            if (String(client.pairedDevices[i].id || "") === wanted)
                return client.pairedDevices[i];
        }
        return client.pairedDevices.length > 0 ? client.pairedDevices[0] : null;
    }
    readonly property bool ready: client.state.enabled === true
        && client.state.installed === true && client.pairedDevices.length > 0
    readonly property var notifications: Array.isArray(client.state.notifications)
        ? client.state.notifications : []

    function emptyState() {
        return {
            schema: 1,
            updatedAt: 0,
            enabled: false,
            installed: false,
            configured: false,
            install: { busy: false, action: "", error: "" },
            networkIsolation: false,
            kdeconnect: { running: false, error: "" },
            activeDeviceId: "",
            devices: [],
            pairing: { outgoing: "", incoming: [] },
            notifications: [],
            runCommands: [],
            call: { state: "idle", name: "", number: "" },
            alarm: { nextAt: 0 },
            adb: { installed: false, state: "none", serial: "", wireless: false, model: "", candidates: [] },
            scrcpy: { installed: false, version: "", appMode: false },
            mirror: { running: false, starting: false, recording: false, error: "" },
            webcam: { running: false, starting: false, device: "", facing: "back", ready: false, error: "" },
            mic: { running: false, starting: false, error: "" },
            contacts: { count: 0, ready: false },
            apps: { count: 0, loading: false, error: "" },
            toast: { seq: 0, text: "", error: false }
        };
    }

    function readState() {
        try {
            const raw = stateFile.text();
            const parsed = raw && raw.trim().length > 0 ? JSON.parse(raw) : null;
            if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)
                    || Number(parsed.schema) !== 1)
                return;
            client.state = parsed;
            client.shellDown = false;
            client.revision++;
        } catch (error) {
        }
    }

    function readContacts() {
        if (!client.contactsOpened)
            return;
        try {
            const raw = contactsFile.text();
            const parsed = raw && raw.trim().length > 0 ? JSON.parse(raw) : [];
            if (!Array.isArray(parsed))
                return;
            client.contacts = parsed;
            client.revision++;
        } catch (error) {
        }
    }

    function readApps() {
        if (!client.appsOpened)
            return;
        try {
            const raw = appsFile.text();
            const parsed = raw && raw.trim().length > 0 ? JSON.parse(raw) : [];
            if (!Array.isArray(parsed))
                return;
            client.apps = parsed;
            client.revision++;
        } catch (error) {
        }
    }

    function openContacts() {
        if (client.contactsOpened)
            return;
        client.contactsOpened = true;
        Qt.callLater(client.readContacts);
    }

    function openApps() {
        if (client.appsOpened)
            return;
        client.appsOpened = true;
        Qt.callLater(client.readApps);
    }

    function argv(fn, args) {
        const command = ["ryoku-shell", "ipc", "call", "phone", String(fn)];
        const values = Array.isArray(args) ? args : [];
        for (let i = 0; i < values.length; i++)
            command.push(String(values[i]));
        return command;
    }

    function resultText(stdoutText, stderrText) {
        let text = String(stdoutText || "").trim();
        if (text.startsWith("err "))
            text = text.slice(4).trim();
        if (!text || text === "ok")
            text = String(stderrText || "").trim();
        return text;
    }

    function finishCall(process, exitCode, stdoutText, stderrText) {
        if (process.finished)
            return;
        process.finished = true;
        const text = client.resultText(stdoutText, stderrText);
        const ok = exitCode === 0 && !String(stdoutText || "").trim().startsWith("err ");
        if (exitCode !== 0)
            client.shellDown = true;
        else
            client.shellDown = false;
        if (typeof process.done === "function")
            process.done(ok, ok ? "" : (text || I18n.tr("The phone action could not be completed.")));
        Qt.callLater(process.destroy);
    }

    function call(fn, args, done) {
        const process = callProcess.createObject(client, {
            command: client.argv(fn, args),
            environment: Spawn.env,
            done: done || null
        });
        if (!process) {
            if (typeof done === "function")
                done(false, I18n.tr("The phone action could not be started."));
            return null;
        }
        process.running = true;
        return process;
    }

    function refresh() {
        client.call("refresh", [], function(ok) {
            if (ok)
                Qt.callLater(client.readState);
        });
    }

    function activate() {
        client.refresh();
    }

    FileView {
        id: stateFile
        path: client.runtimeDir + "/state.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onLoaded: {
            client.readState();
            client.stateLoaded = true;
        }
        onLoadFailed: {
            client.stateLoaded = true;
            client.shellDown = true;
            client.revision++;
        }
        onFileChanged: reload()
    }

    FileView {
        id: contactsFile
        path: client.contactsOpened ? client.runtimeDir + "/contacts.json" : ""
        blockLoading: true
        watchChanges: client.contactsOpened
        printErrors: false
        onLoaded: client.readContacts()
        onFileChanged: reload()
    }

    FileView {
        id: appsFile
        path: client.appsOpened ? client.runtimeDir + "/apps.json" : ""
        blockLoading: true
        watchChanges: client.appsOpened
        printErrors: false
        onLoaded: client.readApps()
        onFileChanged: reload()
    }

    Component {
        id: callProcess

        Process {
            id: process
            property var done: null
            property bool finished: false
            property bool exited: false
            property bool outputDone: false
            property bool errorsDone: false
            property int resultCode: -1
            property string outputText: ""
            property string errorText: ""

            function maybeFinish() {
                if (process.exited && process.outputDone && process.errorsDone)
                    client.finishCall(process, process.resultCode, process.outputText, process.errorText);
            }

            stdout: StdioCollector {
                onStreamFinished: {
                    process.outputText = text;
                    process.outputDone = true;
                    process.maybeFinish();
                }
            }
            stderr: StdioCollector {
                onStreamFinished: {
                    process.errorText = text;
                    process.errorsDone = true;
                    process.maybeFinish();
                }
            }
            onExited: exitCode => {
                process.resultCode = exitCode;
                process.exited = true;
                process.maybeFinish();
            }
        }
    }

    Component.onCompleted: Qt.callLater(client.readState)
}
