import QtQuick
import Quickshell
import Quickshell.Wayland
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/mouse-touchpad"
import "../components/mouse-touchpad/buttons.js" as Buttons
import "../schema/mouse-touchpad.js" as Schema

// pointer, touchpad and gestures, and the extra buttons of a connected mouse
Item {
    id: root

    required property var hub

    readonly property var store: Changes.stores["mouse-touchpad"] || null
    // the button a key capture or sequence edit is for: { device, code }
    property var keysFor: null
    property var sequenceFor: null
    property string buttonsHighlight: ""

    readonly property bool capturing: capture.opened

    function bindOf(device, code) {
        return { store: "mouse-touchpad", key: "button:" + device + "|" + code };
    }

    function deviceOf(id) {
        var list = root.store ? root.store.devices : [];
        for (var i = 0; i < list.length; i++)
            if (list[i].id === id)
                return list[i];
        return null;
    }

    function buttonName(device, code) {
        return Buttons.buttonLabel(code, root.deviceOf(device), I18n.tr);
    }

    function labelOf(device, code) {
        var d = root.deviceOf(device);
        return I18n.tr("%1 on %2").arg(root.buttonName(device, code)).arg(d ? Buttons.deviceName(d) : device);
    }

    // opener: the control focus goes back to when the dialog closes
    function findButton(opener) {
        root.keysFor = null;
        root.store.capture(8000);
        capture.begin("button", "", opener);
    }

    function setKeys(device, code, opener) {
        root.keysFor = { device: device, code: code };
        capture.begin("chord", root.buttonName(device, code), opener);
    }

    function editSequence(device, code, opener) {
        root.sequenceFor = { device: device, code: code };
        sequence.opener = opener || null;
        sequence.openWith(root.buttonName(device, code), Changes.value(root.bindOf(device, code)));
    }

    function focusRow(rowId) {
        page.focusRow(rowId);
        if (rowId === "find-button" || rowId === "extra-buttons" || rowId === "key-sequences") {
            root.buttonsHighlight = "find-button";
            highlightClear.restart();
        }
    }

    Timer {
        id: highlightClear
        interval: LithicTokens.motion.highlight + 600
        onTriggered: root.buttonsHighlight = ""
    }

    Component.onCompleted: {
        Changes.ensureStore("mouse-touchpad", Qt.resolvedUrl("../components/mouse-touchpad/MouseStore.qml"));
        if (root.store)
            root.store.refresh();
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "mouse-touchpad"
        schema: Schema
        bodies: ({ buttons: buttonsBody })
    }

    Component {
        id: buttonsBody
        ExtraButtons {
            controller: root
            store: root.store
            highlightId: root.buttonsHighlight
        }
    }

    KeyCapture {
        id: capture
        parent: root.hub.overlayItem
    }

    SequenceDialog {
        id: sequence
        parent: root.hub.overlayItem
        capture: capture
        store: root.store
        onSaved: target => {
            var f = root.sequenceFor;
            if (f)
                Changes.set(root.bindOf(f.device, f.code), target, root.labelOf(f.device, f.code), "mouse-touchpad");
            sequence.close();
        }
    }

    Connections {
        target: capture
        function onKeysPressed(keys, delayMs) {
            if (capture.mode !== "chord" || !root.keysFor)
                return;
            var f = root.keysFor;
            root.keysFor = null;
            Changes.set(root.bindOf(f.device, f.code), { kind: "chord", keys: keys }, root.labelOf(f.device, f.code), "mouse-touchpad");
        }
        function onFinished(keep) {
            if (capture.mode === "chord" || capture.mode === "button")
                root.keysFor = null;
        }
    }

    // a mouse press while finding a button; an empty press means the wait ran out
    Connections {
        target: root.store
        function onPressedChanged() {
            var p = root.store.pressed;
            if (!p || !capture.opened || capture.mode !== "button")
                return;
            if (p.device === "" || p.button === 0) {
                root.store.capture(8000);
                return;
            }
            root.keysFor = { device: p.device, code: p.button };
            capture.switchTo("chord", root.buttonName(p.device, p.button));
        }
    }

    // the desktop's own shortcuts stay quiet while keys are being recorded
    ShortcutInhibitor {
        window: root.QsWindow ? root.QsWindow.window : null
        enabled: root.capturing
    }

    onCapturingChanged: {
        if (!Settings.supports("submap"))
            return;
        if (root.capturing)
            Settings.send("wm.act", { action: "submap.enter", args: ["record"] });
        else
            Settings.send("wm.act", { action: "submap.reset", args: [] });
    }
}
