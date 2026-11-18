import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import "lib/keymap.js" as Keymap
import Ryoku.Ui.Singletons
import "look"

// The launch-key rebind. ryoku-hub owns desktop.json, so the chosen chord is
// written there as a keybind rebind and the provider emits it; this row only
// reads the store to show the current chord.
Item {
    id: hk

    property real u: 1
    // The shipped chord desktop.keybindRebinds is keyed by.
    property string defaultChord: ""
    property string hotkey: "-"
    property bool listening: false

    signal rebound()
    signal closeRequested()

    implicitWidth: row.implicitWidth
    implicitHeight: row.implicitHeight

    readonly property string desktopPath: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/desktop.json"

    FileView {
        id: store
        path: hk.desktopPath
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: hk.refresh()
        onLoadFailed: hk.hotkey = hk.defaultChord
    }

    function refresh() {
        var chosen = "";
        try {
            var o = JSON.parse(store.text() || "{}");
            var r = o && o.desktop && o.desktop.keybindRebinds;
            if (r && typeof r[hk.defaultChord] === "string")
                chosen = r[hk.defaultChord];
        } catch (e) {}
        hk.hotkey = chosen !== "" ? chosen : hk.defaultChord;
    }
    onDefaultChordChanged: hk.refresh()
    Component.onCompleted: hk.refresh()

    function applyBind(bind) {
        hk.hotkey = bind;
        hk.listening = false;
        Quickshell.execDetached(["ryoku-hub", "desktop", "set-rebind", hk.defaultChord, bind]);
        hk.rebound();
    }

    RowLayout {
        id: row
        anchors.fill: parent
        spacing: Math.round(10 * hk.u)

        Column {
            Layout.fillWidth: true
            spacing: Math.round(2 * hk.u)

            Text {
                text: Look.lithic ? I18n.tr("Launch key").toUpperCase() : I18n.tr("Launch key")
                color: Look.lithic ? Look.fg3 : Look.fg2
                font.family: Look.lithic ? Look.mono : Look.sans
                font.pixelSize: Math.round((Look.lithic ? 9 : 12) * hk.u)
                font.weight: Look.lithic ? Font.Bold : Font.Medium
                font.letterSpacing: Look.lithic ? Look.tracking * hk.u : 0
                renderType: Text.CurveRendering
            }
            Text {
                text: hk.hotkey
                color: Look.fg
                font.family: Look.mono
                font.pixelSize: Math.round(12 * hk.u)
                renderType: Text.CurveRendering
            }
        }

        TextButton {
            u: hk.u
            primary: hk.listening
            cellHeight: 28
            text: hk.listening ? I18n.tr("Press a key…") : I18n.tr("Record")
            onClicked: {
                hk.listening = !hk.listening;
                if (hk.listening) keyCatcher.forceActiveFocus();
            }
        }
    }

    Item {
        id: keyCatcher
        focus: hk.visible
        Keys.onPressed: (e) => {
            e.accepted = true;
            if (e.key === Qt.Key_Escape) {
                if (hk.listening) hk.listening = false;
                else hk.closeRequested();
                return;
            }
            if (!hk.listening) return;
            var bind = Keymap.bindString(e.key, e.modifiers, e.text);
            if (bind !== null) hk.applyBind(bind);
        }
    }
}
