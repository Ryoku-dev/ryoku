import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// the app one job opens: the default, an installed candidate, your own
// command, or another app found by name
Item {
    id: control

    property var row: ({})
    property var value
    property var commit: null
    readonly property var host: control.row.host || null
    readonly property string current: control.value === undefined || control.value === null ? "" : String(control.value)

    readonly property var entries: {
        var r = control.row;
        var out = [];
        out.push({ label: r.fallbackLabel ? I18n.tr("%1 (default)").arg(r.fallbackLabel) : I18n.tr("Not set"), value: "" });
        var cands = (r.candidates || []).filter(c => c.installed === true && c.cmd !== r.fallback);
        for (var i = 0; i < cands.length; i++)
            out.push({ label: I18n.tr(cands[i].label), value: cands[i].cmd });
        if (control.current.length > 0 && !out.some(e => e.value === control.current))
            out.push({ label: control.host ? control.host.commandLabel(control.current) : control.current, value: control.current });
        out.push({ divider: true, label: "" });
        out.push({ label: I18n.tr("Another app\u2026"), glyph: "search", value: null });
        return out;
    }

    implicitWidth: picker.width
    implicitHeight: picker.implicitHeight
    width: implicitWidth

    function activate() {
        picker.open();
    }

    Picker {
        id: picker
        width: Math.min(Math.max(implicitWidth, 280), LithicTokens.size.settingsControlMax)
        options: control.entries
        currentIndex: control.entries.findIndex(e => !e.divider && e.value === control.current)
        overlayParent: control.host ? control.host.hub.overlayItem : null
        Accessible.name: I18n.tr(control.row.label || "")
        onActivated: index => {
            var e = control.entries[index];
            // the picker shows what is really set: the write, or "Another app…", may not change it
            picker.currentIndex = Qt.binding(() => control.entries.findIndex(x => !x.divider && x.value === control.current));
            if (!e || e.divider)
                return;
            if (e.value === null) {
                if (control.host)
                    control.host.chooseOther(control.row, picker);
                return;
            }
            if (control.commit)
                control.commit(e.value);
        }
    }
}
