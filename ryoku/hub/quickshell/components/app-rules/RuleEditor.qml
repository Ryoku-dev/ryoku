import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../schema/app-rules.js" as Schema
import "rules.js" as Rules

// the editor ListEditor opens for one rule: which windows, then what changes
Column {
    id: ed

    property var item: ({})
    // what the window manager writes (rule actions, look fields), and the other rules' app keys
    property var listed: Rules.vocabulary([], [])
    property var taken: []
    property Item overlayParent: null

    property string cls: ""
    property string matchTitle: ""
    property var values: ({})

    readonly property var offered: Rules.offered(ed.listed)
    readonly property bool named: ed.cls.trim() !== "" || ed.matchTitle.trim() !== ""
    readonly property bool clash: ed.taken.indexOf(Rules.keyOf(ed.cls.trim(), ed.matchTitle.trim())) >= 0
    readonly property bool typedOk: ed.offered.every(p => p.kind !== "text" || Rules.problem(p, ed.values[p.id]) === "")
    readonly property bool anything: Rules.anySet({ values: ed.values, extras: ed.item.extras || [] })
    readonly property bool valid: ed.named && !ed.clash && ed.typedOk && ed.anything

    function value() {
        return {
            cls: ed.cls.trim(), title: ed.matchTitle.trim(), values: Changes.clone(ed.values),
            extras: Changes.clone(ed.item.extras || []), keep: Changes.clone(ed.item.keep || {})
        };
    }

    function setValue(id, v) {
        var next = Object.assign({}, ed.values);
        if (v === undefined)
            delete next[id];
        else
            next[id] = v;
        ed.values = next;
    }

    Component.onCompleted: {
        ed.cls = String(ed.item.cls || "");
        ed.matchTitle = String(ed.item.title || "");
        ed.values = Changes.clone(ed.item.values || {});
        clsField.text = ed.cls;
        titleField.text = ed.matchTitle;
    }

    readonly property var apps: {
        var out = [], seen = {};
        var src = DesktopEntries.applications ? DesktopEntries.applications.values : [];
        for (var i = 0; i < src.length; i++) {
            var e = src[i];
            if (!e || e.noDisplay)
                continue;
            var id = String(e.startupClass || e.id || "").replace(/\.desktop$/, "");
            if (id === "" || seen[id])
                continue;
            seen[id] = true;
            out.push({ label: e.name || id, id: id });
        }
        out.sort((a, b) => a.label.toLowerCase().localeCompare(b.label.toLowerCase()));
        return out;
    }

    readonly property var openWindows: {
        var out = [], seen = {};
        var ws = Settings.windows || [];
        for (var i = 0; i < ws.length; i++) {
            var id = String(ws[i].appId || "");
            var t = String(ws[i].title || "");
            if (id === "" || seen[id + "\n" + t])
                continue;
            seen[id + "\n" + t] = true;
            out.push({ label: t !== "" ? t : id, id: id });
        }
        return out;
    }

    width: parent ? parent.width : 0
    spacing: LithicTokens.space.s3

    Label {
        width: parent.width
        text: I18n.tr("Which windows")
        typeRole: "label"
    }

    Flow {
        width: parent.width
        spacing: LithicTokens.space.s3

        TextField {
            id: clsField
            width: ed.width >= 560 ? (ed.width - LithicTokens.space.s3) / 2 : ed.width
            label: I18n.tr("App id")
            placeholderText: "firefox"
            mono: true
            onTextEdited: ed.cls = text
        }
        TextField {
            id: titleField
            width: clsField.width
            label: I18n.tr("Window title")
            placeholderText: I18n.tr("Any title")
            onTextEdited: ed.matchTitle = text
        }
    }

    Row {
        spacing: LithicTokens.space.s2

        Plaque {
            id: appsPlaque
            kind: "tonal"
            small: true
            glyph: "apps"
            text: I18n.tr("Choose an app")
            onClicked: appsPop.open()
        }
        Plaque {
            id: windowsPlaque
            kind: "tonal"
            small: true
            glyph: "select_window"
            text: I18n.tr("Choose an open window")
            onClicked: windowsPop.open()
        }
    }

    Label {
        width: parent.width
        visible: !ed.named || ed.clash
        text: ed.clash ? I18n.tr("There is already a rule for this app and title. Edit that rule instead.")
            : I18n.tr("Name an app, a window title or both. An empty one matches any.")
        typeRole: "detail"
        ink: ed.clash ? Lithic.jasper : Lithic.fg2
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Repeater {
        model: Schema.groups.filter(g => ed.offered.some(p => p.group === g.id))

        delegate: Column {
            id: group
            required property var modelData
            readonly property var rows: ed.offered.filter(p => p.group === group.modelData.id)
            width: ed.width
            spacing: 0

            Item { width: 1; height: LithicTokens.space.s2 }
            Label {
                width: parent.width
                bottomPadding: LithicTokens.space.s1
                text: I18n.tr(group.modelData.title)
                typeRole: "label"
            }
            Repeater {
                model: group.rows
                delegate: PropRow {
                    required property var modelData
                    required property int index
                    prop: modelData
                    first: index === 0
                    value: ed.values[modelData.id]
                    options: modelData.kind === "mode" || modelData.kind === "tri" ? Rules.optionsOf(modelData, ed.listed) : (modelData.options || [])
                    overlayParent: ed.overlayParent
                    onChosen: v => ed.setValue(modelData.id, v)
                }
            }
        }
    }

    Label {
        width: parent.width
        visible: !ed.anything
        text: I18n.tr("Choose at least one change.")
        typeRole: "detail"
        inkRole: "fg3"
    }

    ChooserPopover {
        id: appsPop
        parent: ed.overlayParent ? ed.overlayParent : ed
        anchorItem: appsPlaque
        opener: appsPlaque
        entries: ed.apps
        emptyText: I18n.tr("No apps found")
        onChosen: e => {
            ed.cls = e.id;
            clsField.text = e.id;
        }
    }

    ChooserPopover {
        id: windowsPop
        parent: ed.overlayParent ? ed.overlayParent : ed
        anchorItem: windowsPlaque
        opener: windowsPlaque
        entries: ed.openWindows
        emptyText: I18n.tr("No windows are open")
        onChosen: e => {
            ed.cls = e.id;
            clsField.text = e.id;
        }
    }
}
