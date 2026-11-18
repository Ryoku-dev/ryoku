import QtQuick
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import ".."
import "../Singletons"
import "../components"
import "../components/hyprdrive"
import "../schema/hyprdrive.js" as Schema

// Hyprdrive: its switch, every shortcut it would add with the Ryoku one it would take
// over, and its settings, which the window manager lists
Item {
    id: root

    required property var hub

    // `ryoku-hub desktop hyprdrive status`: the binds, their conflicts and what is taken
    property var status: null

    function focusRow(rowId) {
        page.focusRow(rowId);
    }

    readonly property bool on: Changes.value(Schema.enabledBind) === true

    readonly property var schema: {
        Changes.revision;
        var binds = (root.status && root.status.binds) || [];
        return {
            title: Schema.title,
            lead: Schema.lead,
            sections: Schema.sections,
            related: Schema.related,
            rows: Schema.rows.concat(binds.map(b => Object.assign(Schema.bindRow(b, root.status), { component: bindControl })))
        };
    }

    readonly property string stateLine: {
        if (!root.status)
            return "";
        if (root.on && root.status.installed === false)
            return I18n.tr("Hyprdrive is not built for this version of Hyprland yet. Rebuild it on Window manager.");
        var line = I18n.tr(Schema.takeoverLine(root.status, root.on));
        if (!root.on)
            line = (line ? line + " " : "") + I18n.tr("Its settings, Mosaic and the infinite canvas among them, show once it is on.");
        return line;
    }

    // what turning Hyprdrive on does to the keyboard, said before it happens: every
    // shortcut it adds, every Ryoku one it replaces, and the ones still waiting
    function enableWarning() {
        var binds = (root.status && root.status.binds) || [];
        var keys = b => (b.chords || []).length > 0 ? " (" + b.chords.map(Schema.chordText).join(", ") + ")"
                                                      : Schema.fixedKeys[b.id] ? " (" + Schema.fixedKeys[b.id] + ")" : "";
        var adds = binds.filter(b => b.on).map(b => b.label + keys(b));
        var over = [];
        binds.filter(b => b.on).forEach(b => (b.conflicts || []).forEach(c => over.push(Schema.chordText(c.chord) + " (" + c.label + ")")));
        var waiting = binds.filter(b => !b.on && (b.conflicts || []).length > 0).length;
        var parts = [I18n.tr("Hyprdrive overrides keyboard shortcuts while it is on.")];
        if (adds.length > 0)
            parts.push(I18n.tr("It adds: %1.").arg(adds.join("; ")));
        parts.push(over.length > 0 ? I18n.tr("It replaces these Ryoku shortcuts: %1.").arg(over.join(", "))
                                   : I18n.tr("As your shortcuts stand, it replaces none of Ryoku's."));
        // the canvas zoom changes what holding a key does
        var cfg = k => Changes.value({ store: "desktop", key: "wm.hyprland.plugins.hyprdrive.config.hyprdrive:" + k });
        if (cfg("mosaic:super_zoom") === true && cfg("mosaic:infinite_canvas") === true) {
            var chord = typeof cfg("mosaic:zoom_key") === "string" && cfg("mosaic:zoom_key").trim() !== "" ? cfg("mosaic:zoom_key") : "SUPER + ALT";
            parts.push(I18n.tr("On the infinite canvas, holding %1 zooms out.").arg(Schema.chordText(chord)));
        }
        if (waiting > 0)
            parts.push(waiting === 1 ? I18n.tr("One more would replace a Ryoku shortcut and stays off until you turn it on.")
                                     : I18n.tr("%1 more would replace Ryoku shortcuts and stay off until you turn them on.").arg(waiting));
        parts.push(I18n.tr("Change the keys or turn any of them off under Shortcuts; turning Hyprdrive off gives every shortcut back."));
        return parts.join(" ");
    }

    // the switch asks before it turns Hyprdrive on
    function adapt(rows) {
        return rows.map(r => {
            if (r.id === "plugins-hyprdrive-enabled")
                return Object.assign({}, r, { control: "custom", component: enableToggle });
            // the canvas zoom is a shortcut: the key to hold, and Turn off / on
            if (r.id === "plugins-hyprdrive-config-hyprdrive-mosaic-super-zoom")
                return Object.assign({}, r, {
                    control: "custom", component: zoomControl,
                    taken: (root.status && root.status.taken) || [], others: (root.status && root.status.binds) || []
                });
            return r;
        });
    }

    SettingsPage {
        id: page
        adaptRows: rows => root.adapt(rows)
        anchors.fill: parent
        hub: root.hub
        pageId: "hyprdrive"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ keys: keysLead })
        rimLight: true
    }

    // the status follows every change: a rebind on Keyboard shortcuts frees or takes a chord
    Connections {
        target: Changes
        function onRevisionChanged() {
            refresh.restart();
        }
    }
    Timer {
        id: refresh
        interval: 400
        onTriggered: statusProc.running = true
    }
    Component.onCompleted: statusProc.running = true

    Process {
        id: statusProc
        command: ["ryoku-hub", "desktop", "hyprdrive", "status"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    root.status = JSON.parse(this.text);
                } catch (e) {
                    root.status = null;
                }
            }
        }
    }

    Component {
        id: zoomControl
        ZoomControl {}
    }

    Component {
        id: bindControl
        BindControl {}
    }

    Component {
        id: enableToggle
        Toggle {
            id: toggle
            property var row: null
            property var value: undefined
            property var commit: null
            checked: toggle.value === true
            Accessible.name: I18n.tr("Hyprdrive")
            function activate() {
                toggle.toggle();
                toggle.toggled();
            }
            onToggled: {
                const want = toggle.checked;
                toggle.checked = Qt.binding(() => toggle.value === true);
                if (!want) {
                    toggle.commit(false);
                    return;
                }
                root.hub.confirm({
                    title: I18n.tr("Turn on Hyprdrive?"),
                    text: root.enableWarning(),
                    actionText: I18n.tr("Turn on"),
                    destructive: false
                }, () => toggle.commit(true), null);
            }
        }
    }

    Component {
        id: keysLead
        Label {
            width: parent ? parent.width : 0
            text: I18n.tr("Every shortcut Hyprdrive adds, and the Ryoku shortcut it would replace. Click a field and press new keys to change one; clearing it gives the shipped keys back. One that would replace a Ryoku shortcut stays off until you turn it on, and turning Hyprdrive off gives every shortcut back.")
            typeRole: "detail"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }
}
