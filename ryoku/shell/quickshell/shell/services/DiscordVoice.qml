pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io

// Discord voice for the Game Bar. One JSON-line bridge process owns the Discord
// RPC socket or the Vesktop companion socket; this singleton folds its frames
// into a stable participant model so avatars are not rebuilt when someone speaks.
Singleton {
    id: root

    property string status: "idle"
    property string backend: ""
    readonly property string backendLabel: root.backend === "vesktop" ? "Vesktop"
        : (root.backend === "discord" ? "Discord" : "")
    property string errorMessage: ""
    property var channel: null
    readonly property alias participantModel: participantsModel
    readonly property int participantCount: participantsModel.count
    // Bumped whenever the model changes, so a delegate reading one row through
    // get() refreshes: get() alone does not establish a binding dependency.
    property int participantRevision: 0
    property bool muted: false
    property bool deafened: false

    property int restartAttempts: 0
    property int reconnectAttempts: 0
    property var pendingMessages: []

    readonly property bool authenticated: root.status === "authenticated" || root.channel !== null
    readonly property bool inVoice: root.channel !== null

    // The bridge only runs while the Voice widget is on screen.
    readonly property bool wanted: Config.gamebar.enabled
        && GameBarStore.visibleCards.indexOf("voice") !== -1
    onWantedChanged: root.wanted ? root.start(false) : root.stop()
    Component.onCompleted: if (root.wanted) root.start(false)

    readonly property bool companionNeeded: {
        const text = root.errorMessage.toLowerCase();
        return text.includes("companion") || text.includes("vesktop");
    }
    readonly property int maxRestartAttempts: 5

    property string installerState: "idle"
    property string installerLine: ""
    readonly property bool installerBusy: root.installerState === "running"

    ListModel {
        id: participantsModel
        dynamicRoles: true
    }

    function updateParticipants(users) {
        const incoming = users || [];
        const ids = new Set(incoming.map(user => String(user.id || "")));

        for (let index = participantsModel.count - 1; index >= 0; --index) {
            const current = participantsModel.get(index).participant;
            if (!ids.has(String(current && current.id ? current.id : "")))
                participantsModel.remove(index);
        }

        for (let i = 0; i < incoming.length; i++) {
            const user = incoming[i];
            const id = String(user.id || "");
            let found = -1;
            for (let index = 0; index < participantsModel.count; index++) {
                const current = participantsModel.get(index).participant;
                if (String(current && current.id ? current.id : "") === id) {
                    found = index;
                    break;
                }
            }
            if (found >= 0)
                participantsModel.setProperty(found, "participant", user);
            else
                participantsModel.append({ participant: user });
        }
        root.participantRevision++;
    }

    function participantAt(index) {
        const entry = participantsModel.get(index);
        return entry ? entry.participant : null;
    }

    function avatarUrl(user, size) {
        if (!user || !user.id || !user.avatar)
            return "";
        return "https://cdn.discordapp.com/avatars/" + user.id + "/" + user.avatar
            + ".png?size=" + (size || 128);
    }

    function send(message) {
        if (!bridge.running) {
            root.pendingMessages = root.pendingMessages.concat([message]);
            root.start(true);
            return;
        }
        bridge.write(JSON.stringify(message) + "\n");
    }

    function connect() {
        root.disarmReconnect();
        root.issueConnect();
    }

    function issueConnect() {
        root.errorMessage = "";
        // A bridge that is not running already connects on its own and emits
        // ready; queueing a connect here would ask it a second time.
        if (!bridge.running) {
            root.start(true);
            return;
        }
        root.send({ cmd: "connect" });
    }

    function backoffDelay(attempt) {
        return Math.min(30000, 1000 * Math.pow(2, Math.max(0, attempt - 1)));
    }

    function armReconnect() {
        if (!root.wanted)
            return;
        root.reconnectAttempts++;
        reconnectTimer.interval = root.backoffDelay(root.reconnectAttempts);
        reconnectTimer.restart();
    }

    function disarmReconnect() {
        reconnectTimer.stop();
        root.reconnectAttempts = 0;
    }

    function authorize() {
        root.send({ cmd: "authorize" });
    }

    function setMuted(value) {
        root.send({ cmd: "set_voice_settings", mute: value });
    }

    function setDeafened(value) {
        root.send({ cmd: "set_voice_settings", deaf: value });
    }

    function start(manual) {
        // A queued restart would start a second bridge after this one exits.
        restartTimer.stop();
        if (bridge.running)
            return;
        if (manual) {
            root.restartAttempts = 0;
            root.errorMessage = "";
        }
        root.status = "starting";
        bridge.running = true;
    }

    function stop() {
        restartTimer.stop();
        root.disarmReconnect();
        if (bridge.running)
            bridge.running = false;
        root.status = "idle";
        root.backend = "";
        root.errorMessage = "";
        root.pendingMessages = [];
        root.restartAttempts = 0;
        root.muted = false;
        root.deafened = false;
        root.channel = null;
        root.updateParticipants([]);
    }

    function flushPendingMessages() {
        const queued = root.pendingMessages;
        root.pendingMessages = [];
        for (let i = 0; i < queued.length; i++)
            bridge.write(JSON.stringify(queued[i]) + "\n");
    }

    function handleLine(line) {
        let message;
        try {
            message = JSON.parse(line);
        } catch (error) {
            return;
        }
        switch (message.type) {
        case "ready":
            root.issueConnect();
            break;
        case "backend":
            root.backend = message.backend || "";
            break;
        case "connected":
            root.status = "connected";
            root.restartAttempts = 0;
            root.disarmReconnect();
            break;
        case "auth_required":
            root.status = "auth_required";
            break;
        case "authorizing":
            root.status = "authorizing";
            break;
        case "authenticated":
            root.status = "authenticated";
            root.restartAttempts = 0;
            root.disarmReconnect();
            break;
        case "voice_channel":
            root.channel = message.channel || null;
            root.updateParticipants(message.users);
            break;
        case "voice_state":
            root.updateParticipants(message.users);
            break;
        case "voice_settings":
            root.muted = message.mute === true;
            root.deafened = message.deaf === true;
            break;
        case "unavailable":
            root.status = "unavailable";
            root.errorMessage = message.message || "";
            root.channel = null;
            root.updateParticipants([]);
            root.armReconnect();
            break;
        case "companion_error":
            root.errorMessage = message.message || "";
            break;
        case "disconnected":
            root.status = "disconnected";
            root.backend = "";
            root.channel = null;
            root.updateParticipants([]);
            if (message.reason)
                root.errorMessage = message.reason;
            root.armReconnect();
            break;
        case "error":
            root.status = "auth_required";
            root.errorMessage = message.message || "Discord RPC error";
            break;
        }
    }

    function installCompanion() {
        if (installer.running)
            return;
        root.installerState = "running";
        root.installerLine = "";
        installer.running = true;
    }

    Timer {
        id: restartTimer
        onTriggered: root.start(false)
    }

    Timer {
        id: reconnectTimer
        onTriggered: root.issueConnect()
    }

    Process {
        id: bridge
        command: ["python3", Quickshell.shellPath("modules/gamebar/discordVoice/bridge.py")]
        stdinEnabled: true
        onStarted: root.flushPendingMessages()
        stdout: SplitParser { onRead: data => root.handleLine(data) }
        stderr: SplitParser { onRead: data => console.warn("[DiscordVoice]", data) }
        onExited: (code, status) => {
            root.disarmReconnect();
            root.channel = null;
            root.updateParticipants([]);
            if (!root.wanted) {
                root.status = "idle";
                root.backend = "";
                return;
            }
            if (root.restartAttempts >= root.maxRestartAttempts) {
                root.status = "stopped";
                root.errorMessage = "The Discord bridge stopped after repeated failures";
                return;
            }
            root.restartAttempts++;
            root.status = "restarting";
            restartTimer.interval = root.backoffDelay(root.restartAttempts);
            restartTimer.restart();
        }
    }

    Process {
        id: installer
        command: ["bash", Quickshell.shellPath("modules/gamebar/discordVoice/install_companion.sh")]
        stdout: SplitParser {
            onRead: data => {
                const line = data.trim();
                if (line)
                    root.installerLine = line;
            }
        }
        stderr: SplitParser {
            onRead: data => {
                const line = data.trim();
                if (line)
                    root.installerLine = line;
            }
        }
        onExited: (code, status) => {
            root.installerState = code === 0 ? "done" : "failed";
        }
    }
}
