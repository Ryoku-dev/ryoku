pragma Singleton

import QtQuick

// Dock -> music-card hover state, mirroring DockPreview: DockMedia writes the
// chip's screen-centre and the band edge on hover; MusicHoverPopout reads them.
QtObject {
    id: root

    property bool hovered: false
    property string monitorName: ""
    property real gx: -1
    property real gy: -1
    property string edge: "bottom"
    property real margin: 0

    // Per-output hold: the card on that output is open, or the pointer is on
    // it. The map is replaced, never mutated, so the bindings that read it
    // re-evaluate.
    property var _held: ({})

    function setHeld(monitor, held) {
        const name = String(monitor ?? "");
        if (name === "" || (root._held[name] === true) === held)
            return;
        const next = Object.assign({}, root._held);
        if (held === true)
            next[name] = true;
        else
            delete next[name];
        root._held = next;
    }

    function heldOn(monitor) {
        return root._held[String(monitor ?? "")] === true;
    }
}
