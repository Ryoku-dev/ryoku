import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "gesture"

// The touchpad gesture list on Input. Each row is one trigger and one action.
// The list on screen is the stored list, with one exception: a store that still
// has only the old workspace-swipe toggle shows that swipe as the first row
// until the list is edited. Widths are the card's, never the length of a sentence.
Item {
    id: root

    property var hub
    implicitHeight: column.implicitHeight
    height: implicitHeight

    readonly property var dirKeys: ["horizontal", "vertical", "left", "right", "up", "down", "swipe", "pinch", "pinchin", "pinchout"]
    readonly property var dirLabels: ({
        horizontal: I18n.tr("Left and right"),
        vertical: I18n.tr("Up and down"),
        left: I18n.tr("Left"),
        right: I18n.tr("Right"),
        up: I18n.tr("Up"),
        down: I18n.tr("Down"),
        swipe: I18n.tr("Any swipe"),
        pinch: I18n.tr("Pinch"),
        pinchin: I18n.tr("Pinch in"),
        pinchout: I18n.tr("Pinch out")
    })
    readonly property var actKeys: ["workspace", "special", "move", "resize", "close", "float", "fullscreen", "cursor_zoom", "scroll_move"]
    readonly property var actLabels: ({
        workspace: I18n.tr("Switch workspace"),
        special: I18n.tr("Special workspace"),
        move: I18n.tr("Move window"),
        resize: I18n.tr("Resize window"),
        close: I18n.tr("Close window"),
        float: I18n.tr("Float window"),
        fullscreen: I18n.tr("Fullscreen window"),
        cursor_zoom: I18n.tr("Cursor zoom"),
        scroll_move: I18n.tr("Scroll-drag window")
    })
    readonly property var modeKeys: ({
        float: ["toggle", "float", "tile"],
        fullscreen: ["fullscreen", "maximize"],
        cursor_zoom: ["live", "toggle", "mult"]
    })
    readonly property var modeLabels: ({
        toggle: I18n.tr("Toggle"),
        float: I18n.tr("Float"),
        tile: I18n.tr("Tile"),
        fullscreen: I18n.tr("Fullscreen"),
        maximize: I18n.tr("Maximize"),
        live: I18n.tr("Follow fingers"),
        mult: I18n.tr("Multiply")
    })

    function hv(path) { return root.hub ? root.hub.hyprVal(path) : undefined }
    function he(path, v) { if (root.hub) root.hub.hyprEdit(path, v) }

    readonly property var rows: {
        var g = root.hv("desktop.input.gestures");
        if (Array.isArray(g) && g.length > 0)
            return g;
        if (root.hv("desktop.input.workspaceSwipe") === true) {
            var n = root.hv("desktop.input.swipeFingers");
            if (n !== 3 && n !== 4)
                n = 3;
            return [{ enabled: true, fingers: n, direction: "horizontal", action: "workspace", mode: "", workspaceName: "", zoomLevel: "" }];
        }
        return [];
    }

    function fingerCount(g) { return !g.fingers || g.fingers < 3 ? 3 : g.fingers }

    function axisOf(direction) {
        if (direction === "left" || direction === "right" || direction === "horizontal")
            return "horizontal";
        if (direction === "up" || direction === "down" || direction === "vertical")
            return "vertical";
        if (direction === "pinch" || direction === "pinchin" || direction === "pinchout")
            return "pinch";
        if (direction === "swipe")
            return "swipe";
        return direction || "";
    }

    function overshadows(earlier, later) {
        if (!earlier.enabled || !later.enabled)
            return false;
        if (root.fingerCount(earlier) !== root.fingerCount(later))
            return false;
        if (earlier.direction === later.direction)
            return true;
        if (earlier.direction === "swipe")
            return root.axisOf(later.direction) !== "pinch";
        if (earlier.direction === "horizontal")
            return later.direction === "left" || later.direction === "right";
        if (earlier.direction === "vertical")
            return later.direction === "up" || later.direction === "down";
        if (earlier.direction === "pinch")
            return later.direction === "pinchin" || later.direction === "pinchout";
        return false;
    }

    function otherFingerCountOnAxis(g) {
        if (!g.enabled || !g.direction)
            return false;
        var fingers = root.fingerCount(g);
        var axis = root.axisOf(g.direction);
        for (var i = 0; i < root.rows.length; i++) {
            var o = root.rows[i];
            if (o === g || !o.enabled || !o.direction)
                continue;
            if (root.fingerCount(o) === fingers)
                continue;
            if (root.axisOf(o.direction) === axis)
                return true;
        }
        return false;
    }

    function slotError(g, index) {
        if (!g.enabled)
            return "";
        if (!g.direction || !g.action)
            return I18n.tr("Pick a direction and an action.");
        for (var i = 0; i < index; i++) {
            if (root.overshadows(root.rows[i], g))
                return I18n.tr("A gesture above already uses this finger count and direction.");
        }
        if (g.action === "cursor_zoom" && g.mode !== "live") {
            var z = parseFloat(g.zoomLevel);
            if (!g.zoomLevel || isNaN(z) || z <= 0)
                return I18n.tr("Set a zoom level above 0. A level below 1 zooms out.");
            if (g.mode === "toggle" && z <= 1)
                return I18n.tr("Toggle needs a zoom level above 1.");
            if (g.mode === "mult" && g.direction === "pinch" && z > 1)
                return I18n.tr("Multiplying a pinch by more than 1 only zooms in. Choose Follow fingers.");
        }
        return "";
    }

    function zoomHint(g) {
        if (!g || g.action !== "cursor_zoom")
            return "";
        if (g.mode === "toggle")
            return I18n.tr("Jumps to the zoom level. The next pinch returns to normal size. Either direction flips it, so a pinch meant to zoom out can zoom in.");
        if (g.mode === "mult")
            return I18n.tr("Multiplies by the zoom level on every pinch. Above 1 only zooms in. Below 1 zooms out, and it cannot go under normal size.");
        return I18n.tr("Spread fingers to zoom in, pinch together to zoom out. It stops at normal size.");
    }

    function conflictHint(g) {
        if (!root.otherFingerCountOnAxis(g))
            return "";
        return I18n.tr("A 3-finger and a 4-finger gesture share this direction. The touchpad locks onto the first count it sees, so a 4-finger swipe that starts at 3 fingers runs the 3-finger gesture.");
    }

    function defaultMode(action) {
        var values = root.modeKeys[action] || [];
        return values.length > 0 ? values[0] : "";
    }

    function blank() {
        return { enabled: true, fingers: 3, direction: "", action: "", mode: "", workspaceName: "", zoomLevel: "" };
    }

    function copyRows() {
        var next = [];
        for (var i = 0; i < root.rows.length; i++)
            next.push(Object.assign({}, root.rows[i]));
        return next;
    }

    function incomplete(g) {
        return g.enabled && (!g.direction || !g.action);
    }

    function persist(next) {
        root.he("desktop.input.gestures", next);
        root.he("desktop.input.workspaceSwipe", false);
    }

    function setGesture(index, updates) {
        var next = root.copyRows();
        var row = Object.assign({}, next[index], updates);
        if (updates.action !== undefined) {
            row.mode = root.defaultMode(updates.action);
            if (updates.action !== "cursor_zoom")
                row.zoomLevel = "";
            if (updates.action !== "special")
                row.workspaceName = "";
        }
        next[index] = row;
        root.persist(next);
    }

    function addGesture() {
        for (var i = 0; i < root.rows.length; i++) {
            if (root.incomplete(root.rows[i]))
                return;
        }
        var next = root.copyRows();
        next.push(root.blank());
        root.persist(next);
    }

    function removeGesture(index) {
        var next = root.copyRows();
        next.splice(index, 1);
        root.persist(next);
    }

    Column {
        id: column
        width: root.width
        spacing: Tokens.s3

        GestureNote {
            noteWidth: root.width
            text: I18n.tr("A 3-finger swipe left or right switches workspace. Any other finger count and direction can close, float, fullscreen, zoom, or open a special workspace.")
        }

        Repeater {
            model: root.rows
            delegate: SettingCard {
                required property int index
                required property var modelData
                width: root.width
                collapsible: false
                title: I18n.tr("GESTURE") + " " + (index + 1)

                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: false
                    label: I18n.tr("Enabled")
                    controlWidth: 54
                    Sw {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        on: modelData.enabled === true
                        onToggled: (v) => root.setGesture(index, { enabled: v })
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Fingers")
                    controlWidth: 140
                    Seg {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        options: ["3", "4"]
                        labels: ({ "3": I18n.tr("3 fingers"), "4": I18n.tr("4 fingers") })
                        current: String(root.fingerCount(modelData))
                        onChose: (key) => root.setGesture(index, { fingers: parseInt(key, 10) })
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    block: true
                    label: I18n.tr("Direction")
                    Item {
                        width: root.width - Tokens.s4 * 2
                        implicitHeight: dirChips.height
                        Chips {
                            id: dirChips
                            width: parent.width
                            options: root.dirKeys
                            labels: root.dirLabels
                            current: modelData.direction || ""
                            onChose: (key) => root.setGesture(index, { direction: key })
                        }
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    block: true
                    label: I18n.tr("Action")
                    Item {
                        width: root.width - Tokens.s4 * 2
                        implicitHeight: actChips.height
                        Chips {
                            id: actChips
                            width: parent.width
                            options: root.actKeys
                            labels: root.actLabels
                            current: modelData.action || ""
                            onChose: (key) => root.setGesture(index, { action: key })
                        }
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    visible: (root.modeKeys[modelData.action] || []).length > 0
                    label: I18n.tr("Mode")
                    block: true
                    Item {
                        width: root.width - Tokens.s4 * 2
                        implicitHeight: modeSeg.implicitHeight
                        Seg {
                            id: modeSeg
                            width: parent.width
                            options: root.modeKeys[modelData.action] || []
                            labels: root.modeLabels
                            current: modelData.mode || ""
                            onChose: (key) => root.setGesture(index, { mode: key })
                        }
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    visible: modelData.action === "special"
                    label: I18n.tr("Special workspace")
                    footH: 36
                    Field {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        tabular: true
                        text: modelData.workspaceName || ""
                        placeholder: I18n.tr("scratch")
                        onCommitted: (v) => root.setGesture(index, { workspaceName: v })
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    visible: modelData.action === "cursor_zoom" && modelData.mode !== "live"
                    label: I18n.tr("Zoom level")
                    desc: I18n.tr("Above 1 zooms in. Below 1 zooms out.")
                    footH: 36
                    Field {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        tabular: true
                        text: modelData.zoomLevel || ""
                        placeholder: "1.5"
                        onCommitted: (v) => root.setGesture(index, { zoomLevel: v })
                    }
                }
                Item {
                    width: root.width
                    implicitHeight: notes.implicitHeight
                    Column {
                        id: notes
                        width: parent.width
                        GestureNote { noteWidth: root.width; text: root.conflictHint(modelData) }
                        GestureNote { noteWidth: root.width; text: root.zoomHint(modelData) }
                        GestureNote { noteWidth: root.width; text: root.slotError(modelData, index) }
                    }
                }
                SettingRow {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Remove this gesture")
                    controlWidth: 90
                    Btn {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        compact: true
                        armed: true
                        text: I18n.tr("REMOVE")
                        onAct: root.removeGesture(index)
                    }
                }
            }
        }

        Item {
            width: root.width
            implicitHeight: addBtn.implicitHeight
            Btn {
                id: addBtn
                text: I18n.tr("ADD GESTURE")
                onAct: root.addGesture()
            }
        }
    }
}
