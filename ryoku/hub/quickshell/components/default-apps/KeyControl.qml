import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../shortcuts/binds.js" as Binds

// the keys that open one app: record a new chord, Backspace turns the key off
Item {
    id: control

    property var row: ({})
    property var value
    property var commit: null
    readonly property var host: control.row.host || null
    readonly property string current: control.value === undefined || control.value === null ? "" : String(control.value)
    // a refused chord in store form, shown with why until the next try
    property string attempt: ""
    property string attemptNote: ""
    readonly property string standingNote: {
        if (!control.host || control.current.length === 0)
            return "";
        var h = Binds.clash(control.host.holders, control.current, { kind: "bind", id: control.row.bindId });
        return h ? I18n.tr("Already used by %1").arg(control.host.holderName(h)) : "";
    }

    implicitWidth: field.width
    implicitHeight: field.implicitHeight
    width: implicitWidth

    function fieldChord() {
        if (control.attempt.length > 0)
            return control.host ? control.host.shown(control.attempt) : control.attempt;
        return control.host && control.current.length > 0 ? control.host.shown(control.current) : "";
    }

    function rebindField() {
        field.chord = Qt.binding(control.fieldChord);
    }

    function take(chord) {
        var store = Binds.toStore(chord);
        var h = control.host ? Binds.clash(control.host.holders, store, { kind: "bind", id: control.row.bindId }) : null;
        if (h) {
            control.attempt = store;
            control.attemptNote = I18n.tr("Already used by %1").arg(control.host.holderName(h));
            control.rebindField();
            return;
        }
        control.attempt = "";
        control.attemptNote = "";
        if (control.commit)
            control.commit(store);
        control.rebindField();
    }

    ChordField {
        id: field
        width: Math.min(Math.max(implicitWidth, 200), LithicTokens.size.settingsControlMax)
        enabled: !!control.host && control.host.ready
        accessibleName: I18n.tr(control.row.label || "")
        placeholder: control.current.length === 0 ? I18n.tr("Turned off") : I18n.tr("Not set")
        chord: control.fieldChord()
        conflict: control.attemptNote.length > 0 ? control.attemptNote : control.standingNote
        onRecordingChanged: {
            if (recording) {
                control.attempt = "";
                control.attemptNote = "";
            }
            if (control.host)
                control.host.noteRecording(field, recording);
        }
        onRecorded: chord => control.take(chord)
        onCleared: {
            control.rebindField();
            if (control.commit)
                control.commit("");
        }
        onActiveFocusChanged: {
            if (!activeFocus && control.attempt.length > 0) {
                control.attempt = "";
                control.attemptNote = "";
                control.rebindField();
            }
        }
    }
}
