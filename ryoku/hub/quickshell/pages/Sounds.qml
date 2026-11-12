import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons
import ".."
import "../Singletons"
import "../components"
import "../schema/sounds.js" as Schema
import "../registry/provider-map.js" as ProviderMap
import "../ShellIndex.js" as ShellIndex

// the desktop's own sound cues and the key press sound engine
Item {
    id: root

    required property var hub

    // the key sound folders, read once from the plugin roster where plugins exist
    property var keysounds: ({})
    // a cue that would not play: { event, message }
    property var playFailed: null
    property string choosingEvent: ""

    function focusRow(rowId) {
        page.focusRow(rowId);
    }

    function cueRow(event) {
        return Schema.rows.find(r => r.event === event) || null;
    }

    function cueValue(row) {
        var v = Changes.value(row.bind);
        return v && typeof v === "object" ? v : row["default"];
    }

    function fileName(path) {
        var p = String(path || "");
        return p.length > 0 ? p.substring(p.lastIndexOf("/") + 1) : "";
    }

    function setCue(row, patch) {
        var now = root.cueValue(row);
        var next = { enabled: patch.enabled !== undefined ? patch.enabled : now.enabled === true,
                     file: patch.file !== undefined ? patch.file : String(now.file || "") };
        Changes.set(row.bind, next, row.changeLabel, "sounds");
    }

    function play(event, file) {
        root.playFailed = null;
        Settings.send("sounds.preview", { event: event, file: file || "" }, function (ok, error) {
            if (!ok)
                root.playFailed = { event: event, file: file || "", message: error };
        });
    }

    function chooseFile(event) {
        var row = root.cueRow(event);
        if (!row)
            return;
        var file = String(root.cueValue(row).file || "");
        root.choosingEvent = event;
        picker.title = I18n.tr("Choose a sound for %1").arg(I18n.tr(row.label));
        picker.currentFolder = "file://" + (file.length > 0 ? file.substring(0, file.lastIndexOf("/")) : (Quickshell.env("HOME") || "/"));
        picker.open();
    }

    // the cue rows say which file plays; the folder rows open what the roster names
    function prepared(row) {
        if (row.event) {
            var file = root.fileName(root.cueValue(row).file);
            if (file.length === 0)
                return row;
            return Object.assign({}, row, { detail: I18n.tr("%1. Plays %2").arg(I18n.tr(row.detail)).arg(file) });
        }
        // the row's hand-off runs its command as an app of its own, so these name only what to open
        if (row.roster) {
            if (Settings.supports("plugins")) {
                var target = String(root.keysounds[row.roster] || "");
                if (target.length === 0)
                    return null;
                return Object.assign({}, row, { command: ["xdg-open", target] });
            }
            // without plugins (niri) only your own folder is offered, where the engine has a profile row
            if (row.roster !== "userSounds" || !root.keysoundsListed)
                return null;
            return Object.assign({}, row, { command: ["sh", "-c", "mkdir -p \"$1\" && exec xdg-open \"$1\"", "sh", root.userSoundsDir] });
        }
        return row;
    }

    // the window manager lists key-sound profiles, and reads your own from this folder
    readonly property bool keysoundsListed: {
        ProviderSchema.revision;
        return (ProviderSchema.rows || []).some(r => /(^|\.)keysounds\.profile$/.test(String(r.key || "")));
    }
    readonly property string userSoundsDir: String(Quickshell.env("XDG_DATA_HOME") || (Quickshell.env("HOME") + "/.local/share"))
        + "/ryoku/keysounds"
    // a pack you add shows in the Sound profile list once the window manager lists it again:
    // on opening the page, and when the window comes back from the file manager
    readonly property bool windowActive: root.Window.active
    onWindowActiveChanged: if (root.windowActive && root.keysoundsListed) ProviderSchema.reload()
    Component.onCompleted: if (ProviderSchema.ready && root.keysoundsListed) ProviderSchema.reload()

    // provider rows that install a key sound engine instead of setting a value
    readonly property var installRows: {
        ProviderSchema.revision;
        return (ProviderSchema.rows || []).filter(r => {
            if (r.ctl !== "install" || !r.pkg)
                return false;
            var place = ProviderMap.placeOf(r);
            return place.known && place.place && place.place[0] === "sounds";
        }).map(r => ({
            id: "install-" + r.pkg, section: ProviderMap.placeOf(r).place[1], label: r.label, detail: r.desc || "",
            control: "custom", pkg: r.pkg, launch: r.launch || "", actionText: r.actionLabel || "Install"
        }));
    }

    // engine package -> installed, as the install row's probe last found it (absent until probed)
    property var engineInstalled: ({})
    readonly property bool enginePending: root.installRows.some(r => r.section === "key-sounds" && root.engineInstalled[r.pkg] === false)

    // the window manager's on/off switch for key sounds waits for the engine it installs: it shows
    // unavailable, its stored value kept so it applies once the engine is in
    function adapt(rows) {
        if (!root.enginePending)
            return rows;
        return rows.map(r => r.section === "key-sounds" && r.bind && r.bind.store === "desktop"
            && /\.keysounds\.enabled$/.test(String(r.bind.key || ""))
            ? Object.assign({}, r, { unavailable: "Install the sound engine above first" }) : r);
    }

    readonly property var schema: {
        Changes.revision;
        root.enginePending;
        return {
            title: Schema.title,
            lead: Schema.lead,
            sections: Schema.sections,
            related: Schema.related,
            rows: Schema.rows.map(r => root.prepared(r)).filter(r => r !== null).concat(root.installRows)
        };
    }

    readonly property string stateLine: Settings.get("sounds.enabled") === false
        ? I18n.tr("Desktop sounds are off.") : ""

    // the bar style in use may play its own notification sound
    readonly property var styleItem: {
        var id = String(Settings.get("barStyle") || "") || shells.index.fallback;
        return ShellIndex.item(shells.index, id);
    }
    readonly property string ownSoundNote: {
        var item = root.styleItem;
        if (!item || !Array.isArray(item.owns) || item.owns.indexOf("notificationSound") < 0)
            return "";
        return I18n.tr("%1 plays its own notification sound, so these two stay quiet while you use it.").arg(item.name || item.id);
    }

    ShellIndexFile { id: shells }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "sounds"
        schema: root.schema
        stateLine: root.stateLine
        adaptRows: rows => root.adapt(rows)
        controls: {
            var out = {};
            for (var i = 0; i < root.schema.rows.length; i++) {
                var r = root.schema.rows[i];
                if (r.event)
                    out[r.id] = cueControl;
                else if (r.pkg)
                    out[r.id] = installControl;
            }
            return out;
        }
        // the cue sections leave while sounds are off, their footers with them
        footers: Settings.get("sounds.enabled") === false ? ({})
            : ({ notifications: notificationsFoot, power: powerFoot, devices: devicesFoot })
    }

    RU.FileDialogHost {
        id: picker
        nameFilters: [I18n.tr("Sounds") + " (*.oga *.ogg *.opus *.wav *.flac *.mp3)", I18n.tr("All files") + " (*)"]
        onAccepted: {
            var row = root.cueRow(root.choosingEvent);
            if (!row)
                return;
            var path = decodeURIComponent(String(picker.selectedFile).replace(/^file:\/\//, ""));
            root.setCue(row, { file: path });
            root.play(row.event, path);
        }
    }

    // a volume change plays a cue once the slider rests, so you hear the level
    Connections {
        target: Changes
        function onAnnounced(entry) {
            if (Changes.keyOf(entry.bind) === "settings:sounds.volume")
                volumeSettle.restart();
        }
    }
    Timer {
        id: volumeSettle
        interval: 350
        onTriggered: root.play("notification", "")
    }

    Process {
        id: roster
        command: ["ryoku-hub", "desktop", "plugins", "list"]
        running: Settings.supports("plugins")
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var plugins = JSON.parse(this.text).plugins || [];
                    root.keysounds = plugins.find(p => p.id === "keysounds") || {};
                } catch (e) {
                    root.keysounds = {};
                }
            }
        }
    }

    Component {
        id: cueControl
        Row {
            id: cue
            property var row
            // the frame keeps value on the stored cue
            property var value
            readonly property var cueNow: cue.value && typeof cue.value === "object" ? cue.value
                : (cue.row ? cue.row["default"] : ({ enabled: false, file: "" }))
            readonly property bool custom: String(cue.cueNow.file || "").length > 0
            function activate() {
                root.setCue(cue.row, { enabled: cue.cueNow.enabled !== true });
            }
            spacing: LithicTokens.space.s2

            IconPlaque {
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "play_arrow"
                accessibleName: I18n.tr("Play %1").arg(I18n.tr(cue.row ? cue.row.label : ""))
                onClicked: root.play(cue.row.event, "")
            }
            Plaque {
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: "folder_open"
                text: cue.custom ? I18n.tr("Change") : I18n.tr("Your own")
                accessibleName: I18n.tr("Choose a sound for %1").arg(I18n.tr(cue.row ? cue.row.label : ""))
                onClicked: root.chooseFile(cue.row.event)
            }
            Toggle {
                anchors.verticalCenter: parent.verticalCenter
                checked: cue.cueNow.enabled === true
                Accessible.name: I18n.tr(cue.row ? cue.row.label : "")
                onToggled: root.setCue(cue.row, { enabled: checked })
            }
        }
    }

    // probes once and again when the window comes back from the install terminal
    Component {
        id: installControl
        Item {
            id: inst
            property var row
            property bool installed: false
            property bool probed: false
            readonly property bool windowActive: Window.active
            function activate() {
                if (!inst.installed)
                    inst.install();
            }
            function install() {
                var pkg = String(inst.row.pkg || ""), launch = String(inst.row.launch || "");
                if (!/^[A-Za-z0-9@._+-]+$/.test(pkg) || (launch && !/^[A-Za-z0-9._+-]+$/.test(launch)))
                    return;
                var cmd = "sudo ryoku-host pkg install " + pkg;
                // a second reader of the keyboard would play every press twice, so start one only when none runs
                if (launch)
                    cmd += " && (command -v " + launch + " >/dev/null 2>&1 && ! pgrep -x " + launch
                        + " >/dev/null 2>&1 && setsid " + launch + " >/dev/null 2>&1 &)";
                Spawn.runApp(["kitty", "--hold", "-e", "sh", "-c", cmd], { label: "kitty" });
            }
            implicitWidth: inst.installed ? done.implicitWidth : go.implicitWidth
            implicitHeight: go.implicitHeight
            onWindowActiveChanged: if (inst.windowActive) inst.check()
            onRowChanged: inst.check()

            // the package is set here, not bound, so a probe never starts before the row names it
            property bool recheck: false
            function check() {
                var pkg = String(inst.row && inst.row.pkg || "");
                if (pkg === "")
                    return;
                if (probe.running) {
                    inst.recheck = true;
                    return;
                }
                probe.command = ["ryoku-host", "pkg", "installed", pkg];
                probe.running = true;
            }

            Process {
                id: probe
                onExited: (code, status) => {
                    inst.installed = code === 0;
                    inst.probed = true;
                    var seen = {};
                    seen[String(inst.row.pkg)] = inst.installed;
                    root.engineInstalled = Object.assign({}, root.engineInstalled, seen);
                    if (inst.recheck) {
                        inst.recheck = false;
                        inst.check();
                    }
                }
            }
            Label {
                id: done
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                visible: inst.installed
                text: I18n.tr("Installed")
                typeRole: "label"
                inkRole: "fg2"
            }
            Plaque {
                id: go
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                visible: !inst.installed
                enabled: inst.probed
                kind: "line"
                small: true
                glyph: "download"
                text: I18n.tr(inst.row ? inst.row.actionText : "Install")
                onClicked: inst.install()
            }
        }
    }

    Component {
        id: powerFoot
        PlayError { section: "power" }
    }

    Component {
        id: devicesFoot
        PlayError { section: "devices" }
    }

    Component {
        id: notificationsFoot
        Column {
            spacing: LithicTokens.space.s2
            Label {
                width: parent.width
                visible: root.ownSoundNote.length > 0
                text: root.ownSoundNote
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            PlayError {
                width: parent.width
                section: "notifications"
            }
        }
    }

    component PlayError: Row {
        property string section: ""
        readonly property var failed: root.playFailed
        readonly property var failedRow: failed ? root.cueRow(failed.event) : null
        visible: !!failedRow && failedRow.section === section
        width: parent ? parent.width : 0
        // a footer with nothing to say leaves no gap under its slab
        height: visible ? implicitHeight : 0
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, parent.width - again.width - LithicTokens.space.s3)
            // the reason is its own sentence (the daemon's words start in lower case, the frame's in
            // capitals); a reason of several sentences already says what to do next
            text: {
                var row = parent.failedRow;
                if (!row)
                    return "";
                var why = String(parent.failed.message || "").trim().replace(/[.\s]+$/, "");
                var name = I18n.tr(row.label);
                if (why === "")
                    return I18n.tr("%1 did not play. Check that the sound service is running.").arg(name);
                why = why.charAt(0).toUpperCase() + why.slice(1);
                return why.indexOf(". ") >= 0 ? I18n.tr("%1 did not play. %2.").arg(name).arg(why)
                    : I18n.tr("%1 did not play. %2. Check that the sound service is running.").arg(name).arg(why);
            }
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: again
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            // the note hides as the sound plays again: give the keyboard to the control before it first
            onClicked: {
                if (again.activeFocus) {
                    var back = again.nextItemInFocusChain(false);
                    if (back && back !== again)
                        back.forceActiveFocus(Qt.BacktabFocusReason);
                }
                root.play(parent.failed.event, parent.failed.file);
            }
        }
    }
}
