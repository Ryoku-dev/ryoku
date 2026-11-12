import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/default-apps"
import "../components/shortcuts/binds.js" as Binds
import "../schema/default-apps.js" as Schema

// the app each job opens and the key that opens it
Item {
    id: root

    required property var hub
    readonly property string pageId: "default-apps"

    property var roles: []
    property bool rolesLoaded: false
    property var categories: []
    property bool legendLoaded: false
    property Item recordingField: null
    property var otherRow: null

    readonly property bool ready: Changes.desktopReady && root.legendLoaded
    readonly property bool supported: Settings.modelsKey("desktop.apps")

    readonly property var rebindsBind: ({ store: "desktop", key: "keybindRebinds" })
    readonly property var offBind: ({ store: "desktop", key: "keybindDisabled" })
    readonly property var rebinds: Changes.value(root.rebindsBind) || ({})
    readonly property var disabled: Changes.value(root.offBind) || []
    readonly property var custom: Changes.value({ store: "desktop", key: "keybinds" }) || []
    readonly property var binds: {
        var out = [];
        for (var c = 0; c < root.categories.length; c++)
            out = out.concat((root.categories[c].binds || []).filter(b => b.kind !== "custom"));
        return out;
    }
    readonly property var holders: Binds.holders(root.binds, root.rebinds, root.disabled, root.custom)
    readonly property var faces: ({
        "{n}": I18n.tr("1 … 0"),
        "KP_{n}": I18n.tr("Num 1 … 0")
    })

    // what KeyControl asks of its host
    function shown(chord) {
        return Binds.shown(chord, root.faces);
    }

    function holderName(h) {
        return h.kind === "bind" ? I18n.tr(h.label) : I18n.tr("one of your own shortcuts");
    }

    function noteRecording(field, on) {
        if (on) {
            if (root.recordingField === null && Settings.supports("submap"))
                Settings.send("wm.act", { action: "submap.enter", args: ["record"] });
            root.recordingField = field;
            recordTimeout.restart();
        } else if (root.recordingField === field) {
            root.recordingField = null;
            recordTimeout.stop();
            if (Settings.supports("submap"))
                Settings.send("wm.act", { action: "submap.reset", args: [] });
        }
    }

    function binOf(cmd) {
        var f = String(cmd || "").trim().split(/\s+/);
        var at = f.indexOf("-e");
        return at >= 0 && at + 1 < f.length ? f[at + 1] : (f[0] || "");
    }

    // a command in words: the installed app it starts, else its program
    function commandLabel(cmd) {
        var name = finderIndex.nameOf(cmd);
        if (name.length > 0)
            return name;
        for (var i = 0; i < root.roles.length; i++) {
            var c = (root.roles[i].candidates || []).find(c => c.cmd === cmd);
            if (c)
                return I18n.tr(c.label);
        }
        return String(cmd || "");
    }

    function roleOf(id) {
        return root.roles.find(r => r.role === id) || null;
    }

    function bindFor(role) {
        if (!role)
            return null;
        return root.binds.find(b => b.id === "app." + role.role)
            || (role.combo ? root.binds.find(b => Binds.norm(b.combo) === Binds.norm(role.combo)) : null)
            || null;
    }

    function chooseOther(row, opener) {
        root.otherRow = row;
        otherFinder.clear();
        otherCommand.text = Changes.value(row.bind) || "";
        otherDialog.opener = opener || null;
        otherDialog.open();
    }

    function useOther() {
        var cmd = otherCommand.text.trim();
        if (!root.otherRow || cmd.length === 0)
            return;
        Changes.set(root.otherRow.bind, cmd, root.otherRow.label, root.pageId);
    }

    function effectiveLabel(spec, role) {
        var v = String(Changes.value({ store: "desktop", key: "apps." + spec.id, fallback: "" }) || "");
        if (v.length > 0)
            return root.commandLabel(v);
        return role && role.fallback ? root.fallbackLabel(role) : "";
    }

    function fallbackLabel(role) {
        if (!role || !role.fallback)
            return "";
        var c = (role.candidates || []).find(c => c.cmd === role.fallback);
        if (c)
            return I18n.tr(c.label);
        var b = root.binOf(role.fallback);
        return b.charAt(0).toUpperCase() + b.slice(1);
    }

    readonly property var appRows: {
        if (!root.rolesLoaded)
            return [];
        return Schema.roles.map(spec => {
            var role = root.roleOf(spec.id) || { role: spec.id, fallback: "", candidates: [] };
            var bind = { store: "desktop", key: "apps." + spec.id, fallback: "" };
            var v = String(Changes.value(bind) || "");
            var known = v.length === 0 || (role.candidates || []).some(c => c.cmd === v);
            return {
                id: spec.id, section: "apps", label: spec.label,
                detail: known ? spec.detail : I18n.tr("Runs %1").arg(v),
                control: "custom", bind: bind, "default": "", activatable: true,
                unavailable: root.supported ? "" : "This window manager cannot choose these apps.",
                candidates: role.candidates || [], fallback: role.fallback || "",
                fallbackLabel: root.fallbackLabel(role), host: root
            };
        });
    }

    // each key row is named for its key ("Browser key"), so its reset, its field and the row
    // itself never share a name with the app row above
    readonly property var keyRows: {
        if (!root.rolesLoaded || !root.legendLoaded)
            return [];
        return Schema.roles.map(spec => {
            var role = root.roleOf(spec.id);
            var bind = root.bindFor(role);
            var app = root.effectiveLabel(spec, role);
            var name = I18n.tr("%1 key").arg(I18n.tr(spec.label));
            var opens = app.length > 0 ? I18n.tr("Opens %1").arg(app) : I18n.tr("Choose its app above first");
            if (!bind)
                return { id: spec.id + "-key", section: "keys", label: name,
                         detail: I18n.tr("No key opens it yet. Add one of your own in Keyboard shortcuts."),
                         control: "link", link: "shortcuts/add-shortcut", actionText: "Add a key" };
            return {
                id: spec.id + "-key", section: "keys", label: name, detail: opens,
                changeLabel: name,
                control: "custom", bind: { store: "launch-keys", key: bind.combo },
                "default": Binds.toStore(bind.combo), bindId: bind.id, host: root,
                unavailable: bind.unhonored ? bind.unhonored
                    : bind.rebindable === true ? "" : "Uses its own key, so it cannot be changed"
            };
        });
    }

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: root.appRows.concat(root.keyRows)
    })

    // a landing waits until the apps and their keys have arrived
    readonly property bool complete: root.rolesLoaded && root.legendLoaded && Changes.desktopReady
    property string pendingRow: ""
    onCompleteChanged: {
        if (root.complete && root.pendingRow !== "") {
            var id = root.pendingRow;
            root.pendingRow = "";
            Qt.callLater(page.focusRow, id);
        }
    }

    function focusRow(rowId) {
        if (!root.complete) {
            root.pendingRow = rowId;
            return;
        }
        page.focusRow(rowId);
    }

    Component.onCompleted: Changes.ensureStore("launch-keys", Qt.resolvedUrl("../components/default-apps/LaunchKeysStore.qml"))

    Process {
        command: ["ryoku-hub", "apps"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    root.roles = JSON.parse(this.text) || [];
                } catch (e) {
                    root.roles = [];
                }
                root.rolesLoaded = true;
            }
        }
    }

    Process {
        id: legend
        command: ["ryoku-hub", "keybinds"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    root.categories = JSON.parse(this.text).categories || [];
                } catch (e) {
                    root.categories = [];
                }
                root.legendLoaded = true;
            }
        }
    }

    // the saved legend says which keys the window manager honours
    Connections {
        target: Changes
        function onDesktopCommittedChanged() {
            if (!legend.running)
                legend.running = true;
        }
    }

    // never leave the keyboard held: a ceiling ends any recording
    Timer {
        id: recordTimeout
        interval: 15000
        onTriggered: {
            if (root.recordingField)
                root.recordingField.cancelRecording();
        }
    }

    ShortcutInhibitor {
        window: root.QsWindow ? root.QsWindow.window : null
        enabled: root.recordingField !== null
    }

    // names installed apps for commands; never shown
    AppFinder {
        id: finderIndex
        visible: false
    }

    readonly property var controlMap: {
        var m = {};
        root.appRows.forEach(r => m[r.id] = roleControl);
        root.keyRows.forEach(r => {
            if (r.control === "custom")
                m[r.id] = keyControl;
        });
        return m;
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: root.pageId
        schema: root.schema
        controls: root.controlMap
        footers: ({ keys: keyErrors })
    }

    Component {
        id: roleControl
        RoleControl {}
    }

    Component {
        id: keyControl
        KeyControl {}
    }

    // a key write the window manager refused, with Retry
    Component {
        id: keyErrors
        Column {
            spacing: LithicTokens.space.s2
            Repeater {
                model: [root.rebindsBind, root.offBind].filter(b => Changes.error(b).length > 0)
                delegate: Row {
                    required property var modelData
                    width: parent.width
                    spacing: LithicTokens.space.s3
                    Label {
                        anchors.verticalCenter: parent.verticalCenter
                        width: Math.min(implicitWidth, parent.width - retry.width - LithicTokens.space.s3)
                        text: Changes.error(parent.modelData)
                        typeRole: "detail"
                        ink: Lithic.jasper
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                    Plaque {
                        id: retry
                        kind: "quiet"
                        small: true
                        glyph: "refresh"
                        text: I18n.tr("Retry")
                        onClicked: Changes.retry(parent.modelData)
                    }
                }
            }
        }
    }

    Dialog {
        id: otherDialog
        parent: root.hub.overlayItem
        title: root.otherRow ? I18n.tr("Choose the app for %1").arg(I18n.tr(root.otherRow.label).toLowerCase()) : ""
        actions: [
            { text: I18n.tr("Cancel"), kind: "quiet" },
            { text: I18n.tr("Use this app"), kind: "gold", enabled: otherCommand.text.trim().length > 0 }
        ]
        onActionTriggered: index => {
            if (index === 1)
                root.useOther();
        }

        Column {
            width: parent ? parent.width : 0
            spacing: LithicTokens.space.s4

            AppFinder {
                id: otherFinder
                width: parent.width
                plane: Lithic.stone4
                current: otherCommand.text.trim()
                onPicked: (name, command) => otherCommand.text = command
            }

            TextField {
                id: otherCommand
                width: parent.width
                label: I18n.tr("Command")
                helpText: I18n.tr("Or type the command that starts it, such as kitty -e nvim")
                mono: true
                onAccepted: {
                    if (text.trim().length > 0) {
                        root.useOther();
                        otherDialog.close();
                    }
                }
            }
        }
    }
}
