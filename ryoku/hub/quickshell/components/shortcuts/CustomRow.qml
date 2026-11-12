import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "binds.js" as Binds

// one of the user's own shortcuts: its keys, what it does, and the command it runs
Column {
    id: item

    required property int customIndex
    required property var host
    property bool first: false
    readonly property var entry: item.host.custom[item.customIndex] || ({})
    readonly property string action: item.entry.action || "exec"
    readonly property bool runs: item.action === "exec"
    readonly property string rowId: "custom." + item.customIndex
    property string attempt: ""
    property string attemptNote: ""
    readonly property string standingNote: {
        const h = Binds.clash(item.host.holders, item.entry.keys || "", { kind: "custom", index: item.customIndex });
        return h ? I18n.tr("Already used by %1").arg(item.host.holderName(h)) : "";
    }
    readonly property Item focusTarget: field
    readonly property alias settingRow: settingRow
    readonly property alias chordField: field

    spacing: LithicTokens.space.s2

    function title() {
        if (!item.runs)
            return item.host.actionLabel(item.action);
        const command = String(item.entry.value || "").trim().split(/\s+/)[0] || "";
        const program = command.slice(command.lastIndexOf("/") + 1);
        return program.length > 0 ? I18n.tr("Run %1").arg(program) : I18n.tr("Run a command");
    }

    function fieldChord() {
        return item.host.shown(item.attempt.length > 0 ? item.attempt : (item.entry.keys || ""));
    }

    function rebindField() {
        field.chord = Qt.binding(item.fieldChord);
    }

    function take(chord) {
        const store = Binds.toStore(chord);
        const h = Binds.clash(item.host.holders, store, { kind: "custom", index: item.customIndex });
        if (h) {
            item.attempt = store;
            item.attemptNote = I18n.tr("Already used by %1").arg(item.host.holderName(h));
            item.rebindField();
            return;
        }
        item.attempt = "";
        item.attemptNote = "";
        item.host.patchCustom(item.customIndex, "keys", store);
        item.rebindField();
    }

    SettingRow {
        id: settingRow
        width: parent.width
        first: item.first
        title: item.title()
        detail: item.host.customReason(item.customIndex)
        highlighted: item.host.highlightId === item.rowId

        ChordField {
            id: field
            width: Math.min(Math.max(implicitWidth, 200),
                            LithicTokens.size.settingsControlMax - remove.width - LithicTokens.space.s2)
            enabled: item.host.ready
            accessibleName: I18n.tr("Keys for %1").arg(settingRow.title)
            chord: item.fieldChord()
            conflict: item.attemptNote.length > 0 ? item.attemptNote : item.standingNote
            onRecordingChanged: {
                if (recording) {
                    item.attempt = "";
                    item.attemptNote = "";
                }
                item.host.noteRecording(field, recording);
            }
            onRecorded: chord => item.take(chord)
            onCleared: {
                item.rebindField();
                item.host.patchCustom(item.customIndex, "keys", "");
            }
            onActiveFocusChanged: {
                if (!activeFocus && item.attempt.length > 0) {
                    item.attempt = "";
                    item.attemptNote = "";
                    item.rebindField();
                }
            }
        }

        IconPlaque {
            id: remove
            y: (LithicTokens.size.textFieldHeight - height) / 2
            kind: "quiet"
            small: true
            glyph: "delete"
            enabled: item.host.ready
            accessibleName: I18n.tr("Remove %1").arg(settingRow.title)
            onClicked: item.host.removeCustom(item.customIndex)
        }
    }

    Row {
        width: parent.width
        spacing: LithicTokens.space.s2

        Picker {
            id: actionPicker
            width: 200
            options: item.host.actionOptions.map(o => I18n.tr(o.label))
            currentIndex: item.host.actionOptions.findIndex(o => o.key === item.action)
            overlayParent: item.host.hub.overlayItem
            enabled: item.host.ready
            Accessible.name: I18n.tr("What %1 does").arg(settingRow.title)
            onActivated: index => item.host.patchCustom(item.customIndex, "action", item.host.actionOptions[index].key)
        }

        TextField {
            visible: item.runs
            width: parent.width - actionPicker.width - parent.spacing
            mono: true
            enabled: item.host.ready
            placeholderText: I18n.tr("Command to run, like firefox")
            text: item.entry.value || ""
            Accessible.name: I18n.tr("Command")
            onEditingFinished: {
                if (text !== (item.entry.value || ""))
                    item.host.patchCustom(item.customIndex, "value", text);
            }
        }
    }
}
