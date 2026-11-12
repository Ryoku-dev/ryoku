import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "binds.js" as Binds

// one shortcut the window manager lists: its keys, Turn off, and Reset when changed
SettingRow {
    id: row

    required property var bind
    required property var host
    readonly property string def: row.bind.combo || ""
    readonly property bool editable: row.bind.rebindable === true
    readonly property bool off: Binds.isOff(row.host.disabled, row.def)
    readonly property bool rebound: Binds.isRebound(row.host.rebinds, row.def)
    readonly property string current: Binds.effective(row.host.rebinds, row.def)
    // a refused chord in store form, shown with why until the next try
    property string attempt: ""
    property string attemptNote: ""
    readonly property string standingNote: {
        if (row.off || !row.rebound)
            return "";
        const h = Binds.clash(row.host.holders, row.current, { kind: "bind", id: row.bind.id });
        return h ? I18n.tr("Already used by %1").arg(row.host.holderName(h)) : "";
    }
    readonly property Item focusTarget: field.enabled ? field : (offPlaque.visible ? offPlaque : null)

    title: I18n.tr(row.bind.label)
    detail: row.bind.hint ? I18n.tr(row.bind.hint) : (row.editable ? "" : I18n.tr("Uses its own key, so it cannot be changed"))
    changed: row.rebound || row.off
    highlighted: row.host.highlightId === row.bind.id
    onResetRequested: row.host.resetBind(row.bind)

    function fieldChord() {
        if (row.attempt.length > 0)
            return row.host.shown(row.attempt);
        return row.off ? "" : row.host.shown(row.current);
    }

    // ChordField sets its own chord on a recording; hand the binding back
    function rebindField() {
        field.chord = Qt.binding(row.fieldChord);
    }

    function refuse(chord, note) {
        row.attempt = chord;
        row.attemptNote = note;
        row.rebindField();
    }

    function take(chord) {
        let store = Binds.toStore(chord);
        if (Binds.isFamily(row.def)) {
            const family = Binds.familyFrom(store);
            if (family === "") {
                row.refuse(store, I18n.tr("This shortcut needs a number key, like %1")
                           .arg(field.displayKeys(Binds.toField(row.def.replace("{n}", "1"))).join("+")));
                return;
            }
            store = family;
        }
        const h = Binds.clash(row.host.holders, store, { kind: "bind", id: row.bind.id });
        if (h) {
            row.refuse(store, I18n.tr("Already used by %1").arg(row.host.holderName(h)));
            return;
        }
        row.attempt = "";
        row.attemptNote = "";
        row.host.rebind(row.bind, store);
        row.rebindField();
    }

    ChordField {
        id: field
        width: Math.min(Math.max(implicitWidth, 200),
                        LithicTokens.size.settingsControlMax - (offPlaque.visible ? offPlaque.width + LithicTokens.space.s2 : 0))
        enabled: row.editable && !row.off && row.host.ready
        accessibleName: I18n.tr(row.bind.label)
        placeholder: row.off ? I18n.tr("Turned off") : I18n.tr("Not set")
        chord: row.fieldChord()
        conflict: row.attemptNote.length > 0 ? row.attemptNote : row.standingNote
        onRecordingChanged: {
            if (recording) {
                row.attempt = "";
                row.attemptNote = "";
            }
            row.host.noteRecording(field, recording);
        }
        onRecorded: chord => row.take(chord)
        onCleared: {
            row.rebindField();
            row.host.setOff(row.bind, true);
        }
        onActiveFocusChanged: {
            if (!activeFocus && row.attempt.length > 0) {
                row.attempt = "";
                row.attemptNote = "";
                row.rebindField();
            }
        }
    }

    Plaque {
        id: offPlaque
        // level with the field box, not with its conflict line
        y: (LithicTokens.size.textFieldHeight - height) / 2
        visible: row.editable
        width: row.host.offWidth
        kind: "quiet"
        small: true
        enabled: row.host.ready
        text: row.off ? I18n.tr("Turn on") : I18n.tr("Turn off")
        accessibleName: (row.off ? I18n.tr("Turn on %1") : I18n.tr("Turn off %1")).arg(I18n.tr(row.bind.label))
        onClicked: row.host.setOff(row.bind, !row.off)
    }
}
