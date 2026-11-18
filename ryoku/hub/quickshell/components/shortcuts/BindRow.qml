import QtQuick
import QtQuick.Templates as T
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
    readonly property bool daemon: row.bind.kind === "daemon"
    readonly property bool off: row.daemon ? row.host.gestureOff(row.bind) : Binds.isOff(row.host.disabled, row.def)
    readonly property bool rebound: row.daemon ? row.host.gestureRebound(row.bind) : Binds.isRebound(row.host.rebinds, row.def)
    readonly property string current: row.host.effectiveBind(row.bind)
    // a refused chord in store form, shown with why until the next try
    property string attempt: ""
    property string attemptNote: ""
    readonly property string standingNote: {
        if (row.daemon || row.off || !row.rebound)
            return "";
        const h = Binds.clash(row.host.holders, row.current, { kind: "bind", id: row.bind.id });
        return h ? I18n.tr("Already used by %1").arg(row.host.holderName(h)) : "";
    }
    readonly property Item focusTarget: row.daemon ? gesture : (field.enabled ? field : (offPlaque.visible ? offPlaque : null))

    title: I18n.tr(row.bind.label)
    detail: row.daemon && row.attemptNote.length > 0 ? row.attemptNote
            : row.daemon && row.host.sightNeedsRashin ? I18n.tr("Needs Rashin on")
            : row.bind.hint ? I18n.tr(row.bind.hint) : (row.editable ? "" : I18n.tr("Uses its own key, so it cannot be changed"))
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
        visible: !row.daemon
        width: Math.min(Math.max(implicitWidth, 200),
                        LithicTokens.size.settingsControlMax - (offPlaque.visible ? offPlaque.width + LithicTokens.space.s2 : 0))
        enabled: !row.daemon && row.editable && !row.off && row.host.ready
        accessibleName: I18n.tr(row.bind.label)
        placeholder: row.off ? I18n.tr("Turned off") : I18n.tr("Not set")
        chord: row.fieldChord()
        conflict: row.attemptNote.length > 0 ? row.attemptNote : row.standingNote
        function startRecording() {
            if (!enabled)
                return;
            forceActiveFocus(Qt.OtherFocusReason);
            _recording = true;
        }
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
        // Escape drops a refused chord before it leaves the page; otherwise it passes on
        Keys.onEscapePressed: event => {
            event.accepted = row.attempt.length > 0;
            if (!event.accepted)
                return;
            row.attempt = "";
            row.attemptNote = "";
            row.rebindField();
        }
    }

    // the same field frame and width as a chord, so every row's keys line up; it opens a key menu
    T.AbstractButton {
        id: gesture
        visible: row.daemon
        enabled: row.host.ready
        width: field.width
        height: LithicTokens.size.textFieldHeight
        hoverEnabled: true
        focusPolicy: Qt.StrongFocus
        Accessible.role: Accessible.ComboBox
        Accessible.name: row.bind.whileHighlighting
            ? I18n.tr("Hold %1 while highlighting").arg(row.host.gestureLabel(row.bind))
            : I18n.tr("%1 ×%2").arg(row.host.gestureLabel(row.bind)).arg(row.bind.taps || 0)
        Keys.onReturnPressed: gestureMenu.open()
        Keys.onEnterPressed: gestureMenu.open()
        Keys.onSpacePressed: gestureMenu.open()
        onClicked: gestureMenu.open()
        // ChordField's frame (Lithic's FieldSurface is internal to the library)
        background: CutRect {
            fill: gesture.hovered ? Lithic.mix(Lithic.field, Lithic.fg, LithicTokens.alpha.hoverPlane) : Lithic.field
            strokeColor: gestureMenu.visible ? Lithic.fg2 : Lithic.edge
            strokeWidth: LithicTokens.size.hairline
            cut: LithicTokens.cut.mid + (gesture.down ? Lithic.pressCutGrow : 0)
            ChiselColorBehavior on fill { duration: gesture.hovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut }
            ChiselNumberBehavior on cut { duration: LithicTokens.motion.press }
            FocusRing {
                x: -outset
                y: -outset
                width: parent.width + outset * 2
                height: parent.height + outset * 2
                keyboardFocusVisible: gesture.visualFocus
                cut: LithicTokens.cut.mid
            }
        }
        contentItem: Item {
            Row {
                id: gestureFace
                x: LithicTokens.size.textFieldInset
                anchors.verticalCenter: parent.verticalCenter
                spacing: LithicTokens.space.s1
                Label {
                    visible: !!row.bind.whileHighlighting
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Hold")
                    typeRole: "label"
                    inkRole: "fg2"
                }
                CutRect {
                    width: gestureKey.implicitWidth + LithicTokens.space.s2 * 2
                    height: gestureKey.implicitHeight + LithicTokens.space.s1 * 2
                    cut: LithicTokens.cut.token
                    fill: Lithic.field
                    strokeColor: Lithic.edge
                    strokeWidth: LithicTokens.size.hairline
                    Label {
                        id: gestureKey
                        anchors.centerIn: parent
                        text: row.host.gestureLabel(row.bind)
                        typeRole: "label"
                        inkRole: "fg"
                    }
                }
                Label {
                    visible: !row.bind.whileHighlighting
                    anchors.verticalCenter: parent.verticalCenter
                    text: "×" + (row.bind.taps || 0)
                    typeRole: "label"
                    inkRole: "fg2"
                }
            }
            Glyph {
                anchors.right: parent.right
                anchors.rightMargin: LithicTokens.size.textFieldInset
                anchors.verticalCenter: parent.verticalCenter
                name: "expand_more"
                size: 16
                ink: gesture.hovered ? Lithic.fg : Lithic.fg2
                rotation: gestureMenu.visible ? 180 : 0
                Behavior on rotation { NumberAnimation { duration: Lithic.duration(160); easing.type: Easing.OutCubic } }
            }
        }
    }

    Popover {
        id: gestureMenu
        parent: row.host.hub.overlayItem
        anchorItem: gesture
        opener: gesture
        Menu {
            items: row.host.gestureKeys.map(key => ({
                label: I18n.tr(key.Label),
                checked: key.Name === row.current,
                disabled: key.Name === row.host.gestureOtherKey(row.bind)
            }))
            onTriggered: index => {
                row.host.setGestureKey(row.bind, row.host.gestureKeys[index].Name, error => row.attemptNote = error);
                gestureMenu.close();
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
