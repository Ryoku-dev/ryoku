import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../shortcuts/binds.js" as Binds

// one Hyprdrive shortcut, as Keyboard shortcuts draws one: its keys in a field you
// record a new chord into, and Turn off / Turn on. A chord Ryoku or another Hyprdrive
// shortcut already uses is refused, saying which; clearing the field gives the shipped
// chord back. Super alone, the swipe and the wheel are not keyboard chords, so those
// show their keys and keep only the switch.
Row {
    id: root

    // set by RowControl: the row (row.hd is the window manager's status for this
    // shortcut, row.taken every chord Ryoku binds, row.others the other shortcuts),
    // its on/off value and the commit that writes it
    property var row: null
    property var value: undefined
    property var commit: null

    readonly property var hd: root.row ? root.row.hd : null
    readonly property bool on: root.value === true
    readonly property bool recordable: !!root.hd && root.hd.recordable === true
    readonly property string current: root.hd && root.hd.chords.length > 0 ? root.hd.chords[0] : ""
    property string note: ""

    spacing: LithicTokens.space.s2

    function activate() {
        if (root.commit)
            root.commit(!root.on);
    }

    // a chord in one spelling, so "SUPER + Shift + l" and "SHIFT + SUPER + L" meet
    function norm(chord) {
        var mods = ["SUPER", "SHIFT", "CTRL", "CONTROL", "ALT", "META", "MOD4", "MOD1"];
        var held = [], keys = [];
        String(chord || "").split("+").forEach(function (part) {
            var t = part.trim().toUpperCase();
            if (t === "")
                return;
            if (t === "CONTROL")
                t = "CTRL";
            if (mods.indexOf(t) >= 0)
                held.push(t);
            else
                keys.push(t);
        });
        held.sort();
        return held.concat(keys).join("+");
    }

    // who holds this chord now: a Ryoku shortcut or another Hyprdrive one
    function holder(store) {
        var n = root.norm(store);
        var taken = (root.row && root.row.taken) || [];
        for (var i = 0; i < taken.length; i++)
            if (root.norm(taken[i].chord) === n)
                return taken[i].label;
        var others = (root.row && root.row.others) || [];
        for (var j = 0; j < others.length; j++)
            if (others[j].id !== root.hd.id && (others[j].chords || []).some(c => root.norm(c) === n))
                return I18n.tr("Hyprdrive's %1").arg(I18n.tr(others[j].label));
        return "";
    }

    function chordBind() {
        return { store: "desktop", key: "wm.hyprland.plugins.hyprdrive.chords." + root.hd.id };
    }

    function writeChord(store) {
        Changes.set(root.chordBind(), store, I18n.tr("Keys for %1").arg(I18n.tr(root.row.label)), "hyprdrive");
    }

    function rebindField() {
        field.chord = Qt.binding(() => root.current ? Binds.toField(root.current) : "");
    }

    ChordField {
        id: field
        visible: root.recordable
        width: 200
        enabled: root.on
        accessibleName: I18n.tr(root.row ? root.row.label : "")
        placeholder: root.on ? I18n.tr("Not set") : I18n.tr("Turned off")
        chord: root.current ? Binds.toField(root.current) : ""
        conflict: root.note
        onRecordingChanged: {
            if (recording)
                root.note = "";
        }
        onRecorded: chord => {
            var store = Binds.toStore(chord);
            var h = root.holder(store);
            if (h && root.norm(store) !== root.norm(root.current)) {
                root.note = I18n.tr("Already used by %1").arg(h);
                root.rebindField();
                return;
            }
            root.note = "";
            // the shipped chord is no override: it is stored as none
            var shipped = root.hd.default && root.hd.default.length === 1 && root.norm(store) === root.norm(root.hd.default[0]);
            root.writeChord(shipped ? "" : store);
            root.rebindField();
        }
        onCleared: {
            root.note = "";
            root.writeChord("");
            root.rebindField();
        }
    }

    Label {
        visible: !root.recordable
        anchors.verticalCenter: parent.verticalCenter
        width: Math.min(implicitWidth, 200)
        text: root.row ? I18n.tr(root.row.keysText || "") : ""
        typeRole: "detail"
        inkRole: root.on ? "fg" : "fg3"
    }

    Plaque {
        anchors.verticalCenter: parent.verticalCenter
        kind: "quiet"
        small: true
        text: root.on ? I18n.tr("Turn off") : I18n.tr("Turn on")
        accessibleName: (root.on ? I18n.tr("Turn off %1") : I18n.tr("Turn on %1")).arg(I18n.tr(root.row ? root.row.label : ""))
        onClicked: root.activate()
    }
}
