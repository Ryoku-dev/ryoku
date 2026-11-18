import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../shortcuts/binds.js" as Binds

// the canvas zoom as a shortcut: the chord you hold, recorded like any other
// (modifiers alone too, such as Super+Alt), and Turn off / Turn on. A chord Ryoku or
// another Hyprdrive shortcut already uses is refused; clearing the field gives
// Super+Alt back.
Row {
    id: root

    // set by RowControl: the row (mosaic:super_zoom; row.taken and row.others as on
    // the other shortcut rows), its on/off value and its commit
    property var row: null
    property var value: undefined
    property var commit: null

    readonly property string shipped: "SUPER + ALT"
    readonly property bool on: root.value === true
    readonly property var keyBind: ({ store: "desktop", key: "wm.hyprland.plugins.hyprdrive.config.hyprdrive:mosaic:zoom_key" })
    readonly property string current: {
        var v = Changes.value(root.keyBind);
        return typeof v === "string" && v.trim() !== "" ? v : root.shipped;
    }
    property string note: ""

    spacing: LithicTokens.space.s2

    function activate() {
        if (root.commit)
            root.commit(!root.on);
    }

    function norm(chord) {
        var mods = ["SUPER", "SHIFT", "CTRL", "ALT"];
        var held = [], keys = [];
        String(chord || "").split("+").forEach(function (part) {
            var t = part.trim().toUpperCase();
            if (t === "CONTROL")
                t = "CTRL";
            if (t === "")
                return;
            (mods.indexOf(t) >= 0 ? held : keys).push(t);
        });
        held.sort();
        return held.concat(keys).join("+");
    }

    function holder(store) {
        var n = root.norm(store);
        var taken = (root.row && root.row.taken) || [];
        for (var i = 0; i < taken.length; i++)
            if (root.norm(taken[i].chord) === n)
                return taken[i].label;
        var others = (root.row && root.row.others) || [];
        for (var j = 0; j < others.length; j++)
            if ((others[j].chords || []).some(c => root.norm(c) === n))
                return I18n.tr("Hyprdrive's %1").arg(I18n.tr(others[j].label));
        return "";
    }

    function write(store) {
        Changes.set(root.keyBind, store, I18n.tr("Keys for the canvas zoom"), "hyprdrive");
    }

    function rebindField() {
        field.chord = Qt.binding(() => Binds.toField(root.current));
    }

    ChordField {
        id: field
        width: 200
        enabled: root.on
        // modifiers alone (Super+Alt) where the field can record them; an older
        // Lithic records a chord with a key and still loads
        Component.onCompleted: {
            if ("modifiersAlone" in field)
                field.modifiersAlone = true;
        }
        accessibleName: I18n.tr("Keys to hold for the canvas zoom")
        placeholder: root.on ? I18n.tr("Not set") : I18n.tr("Turned off")
        chord: Binds.toField(root.current)
        conflict: root.note
        onRecordingChanged: {
            if (recording)
                root.note = "";
        }
        onRecorded: chord => {
            var store = Binds.toStore(chord);
            var h = root.holder(store);
            if (h) {
                root.note = I18n.tr("Already used by %1").arg(h);
                root.rebindField();
                return;
            }
            root.note = "";
            root.write(store);
            root.rebindField();
        }
        onCleared: {
            root.note = "";
            root.write(root.shipped);
            root.rebindField();
        }
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
