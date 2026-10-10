pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// ryoport, the harbour: one console for every machine you command -- the local
// iron in the yard (QEMU), and the distant ports reached over SSH. A hub shell:
// the rail owns navigation, three plates fill the stage (the harbour dashboard,
// the machine yard, the remote fleet). Paper and ink; the frame carries no accent,
// the content none at all.
Rectangle {
    id: app

    implicitWidth: 1180
    implicitHeight: 760
    color: Tokens.paper

    property string section: "dashboard"     // dashboard | machines | remotes | passthrough | consoles
    property bool settingsOpen: false
    property bool logOpen: false

    focus: true
    onSectionChanged: app.refocus()
    function refocus() {
        if (app.section === "machines") machines.forceActiveFocus();
        else if (app.section === "remotes") remotes.forceActiveFocus();
        else if (app.section === "consoles") consoles.forceActiveFocus();
        else if (app.section === "dashboard") dashboard.forceActiveFocus();
        else app.forceActiveFocus();
    }

    Keys.onEscapePressed: (e) => {
        if (addConsoleSheet.open) { addConsoleSheet.closed(); e.accepted = true; }
        else if (harbourLog.open) { harbourLog.open = false; e.accepted = true; }
        else if (app.settingsOpen) { app.settingsOpen = false; e.accepted = true; }
        else if (app.section !== "dashboard") { app.section = "dashboard"; e.accepted = true; }
        else e.accepted = false;
    }
    Shortcut { sequence: "Ctrl+Q"; onActivated: app.requestQuit() }
    Shortcut { sequence: "Ctrl+1"; onActivated: app.section = "dashboard" }
    Shortcut { sequence: "Ctrl+2"; onActivated: app.section = "machines" }
    Shortcut { sequence: "Ctrl+3"; onActivated: app.section = "remotes" }
    Shortcut { sequence: "Ctrl+4"; onActivated: app.section = "passthrough" }
    Shortcut { sequence: "Ctrl+5"; onActivated: app.section = "consoles" }
    Shortcut { sequence: "Ctrl+L"; onActivated: app.logOpen = !app.logOpen }
    Shortcut { sequence: "Ctrl+N"; onActivated: app.openAddRemote("") }

    // the dashboard, the remotes page, and the consoles page all read live
    // remote health, so the probe timers run for any of them; the yard-only
    // view lets them rest.
    Binding { target: Remotes; property: "active"; value: app.section === "remotes" || app.section === "dashboard" || app.section === "consoles" }
    Binding { target: Vm; property: "metricsActive"; value: app.section === "dashboard" || (app.section === "machines" && machines.mode === "library") }
    // the passthrough lane polls the looking-glass engine only while shown.
    Binding { target: Lg; property: "poll"; value: app.section === "passthrough" }

    function requestQuit() {
        if (Vm.downloading && !quitArm.running) {
            Vm.info(I18n.tr("A download is running. Cancel it, or quit again to abandon it"));
            quitArm.restart();
            return;
        }
        Qt.quit();
    }
    Timer { id: quitArm; interval: 3000 }

    function openAddRemote(a) {
        app.section = "remotes";
        addSheet.editAlias = a;
        addSheet.open = true;
    }
    function openAddConsole(id, kind) {
        app.section = "consoles";
        addConsoleSheet.editId = id || "";
        addConsoleSheet.kind = kind || "serial";
        addConsoleSheet.open = true;
    }
    function openLog() { app.logOpen = true; }

    Component.onCompleted: {
        var s = Quickshell.env("RYOPORT_SECTION");
        if (s === "dashboard" || s === "machines" || s === "remotes" || s === "passthrough" || s === "consoles") app.section = s;
        var m = Quickshell.env("RYOVM_START_MODE");
        if (m === "catalog" || m === "instant") app.section = "machines";
        app.refocus();
    }

    // ── the rail ────────────────────────────────────────────────────────────
    Rail {
        id: rail
        anchors { left: parent.left; top: parent.top; bottom: parent.bottom }
        section: app.section
        onNavigate: (key) => app.section = key
        onOpenSettings: app.settingsOpen = true
        onRequestQuit: app.requestQuit()
        onOpenLog: app.logOpen = true
    }

    // ── the stage: the plates, crossfaded ───────────────────────────────────
    Item {
        id: stage
        anchors { left: rail.right; top: parent.top; right: parent.right; bottom: parent.bottom }

        Dashboard {
            id: dashboard
            anchors.fill: parent
            active: app.section === "dashboard"
            opacity: active ? 1 : 0
            visible: opacity > 0
            Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
            onNavigate: (key) => app.section = key
            onNewMachine: { Vm.loadCatalog(false); app.section = "machines"; machines.mode = "new"; machines.channel = "catalog"; }
            onNewRemote: app.openAddRemote("")
            onOpenMachine: (name) => { Vm.select(name); app.section = "machines"; machines.mode = "library"; }
            onOpenRemote: (alias) => { Remotes.select(alias); app.section = "remotes"; }
            onOpenConsole: (id) => { consoles.selectedId = id; app.section = "consoles"; }
            onOpenLog: app.openLog()
        }

        Machines {
            id: machines
            anchors.fill: parent
            active: app.section === "machines"
            focus: app.section === "machines"
            opacity: active ? 1 : 0
            visible: opacity > 0
            Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
            onInstallEngineRequested: settings.installEngine()
        }

        RemotesPage {
            id: remotes
            anchors.fill: parent
            active: app.section === "remotes"
            opacity: active ? 1 : 0
            visible: opacity > 0
            Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
            onNewRemote: app.openAddRemote("")
            onEditRemote: (a) => app.openAddRemote(a)
        }

        PassthroughPage {
            id: passthrough
            anchors.fill: parent
            active: app.section === "passthrough"
            opacity: active ? 1 : 0
            visible: opacity > 0
            Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
        }

        ConsolesPage {
            id: consoles
            anchors.fill: parent
            active: app.section === "consoles"
            opacity: active ? 1 : 0
            visible: opacity > 0
            Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }
            onNewConsole: (kind) => app.openAddConsole("", kind)
            onEditConsole: (id) => app.openAddConsole(id, "")
        }
    }

    // ── overlays ─────────────────────────────────────────────────────────────
    SettingsPanel {
        id: settings
        anchors.fill: parent
        open: app.settingsOpen
        onClosed: app.settingsOpen = false
    }

    HarbourLog {
        id: harbourLog
        anchors.fill: parent
        open: app.logOpen
        onClosed: app.logOpen = false
        onOpenResource: (kind, key) => {
            app.logOpen = false;
            if (kind === "vm") { Vm.select(key); app.section = "machines"; machines.mode = "library"; }
            else if (kind === "remote") { Remotes.select(key); app.section = "remotes"; }
            else if (kind === "console") { consoles.selectedId = key; app.section = "consoles"; }
        }
    }

    AddRemote {
        id: addSheet
        onClosed: { addSheet.open = false; addSheet.editAlias = ""; }
    }

    AddConsole {
        id: addConsoleSheet
        onClosed: { addConsoleSheet.open = false; addConsoleSheet.editId = ""; }
    }
}
