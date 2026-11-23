pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "../kit" as K
import "lib/rows.js" as Rows

// The right pane: the selected result in detail. It reads the engine, picks a
// sub-view by the row's provider, and owns the action focus the sheet hands it
// on Tab. The separation from the track list is drawn here, per design.
Item {
    id: root

    property var engine: null
    property var row: null
    property bool actionsFocused: false
    property int actionIndex: 0

    readonly property string kind: {
        if (!root.row)
            return root.engine && root.engine.mode === "apps"
                && String(root.engine.query || "").length > 0
                && root.engine.busy !== true ? "empty" : "";
        const key = Rows.kindKey(root.row);
        return key === "action" || key === "other" ? "generic" : key;
    }
    readonly property bool isFile: root.kind === "file"
    // a window row's own window, which the lead offers to kill after its actions
    readonly property string windowId: root.kind === "window" && root.row
        ? String(root.row.windowAddress || "") : ""

    // The row's actions, primary first.
    readonly property var actions: {
        const row = root.row;
        if (!row || !row.primaryAction)
            return [];
        return [row.primaryAction].concat(Array.isArray(row.secondaryActions) ? row.secondaryActions : []);
    }
    // An app's open windows, reached by the same Tab focus after its actions.
    readonly property var windows: root.kind === "app" && root.engine
        ? (root.engine.windowsFor(root.row) || []) : []

    readonly property var actionItems: {
        const items = root.actions.map((action, index) => ({
            label: root.isFile && action.id === "reveal" ? I18n.tr("Show in folder") : String(action.name || ""),
            primary: index === 0
        }));
        if (root.windowId.length > 0)
            items.push(root.killItem(root.windowId, I18n.tr("Kill")));
        return items;
    }
    readonly property var windowItems: root.windows.map(win => ({
        label: String(win.title || ""),
        meta: root.workspaceLabel(win.workspace),
        detail: root.memoryOf(win.id),
        glyph: "select_window",
        kill: true,
        armed: root.engine !== null && root.engine.armedKill === String(win.id)
    }))
    // the kill entry a window row adds after its own actions
    readonly property int killIndex: root.windowId.length > 0 ? root.actions.length : -1

    readonly property int actionCount: root.actionItems.length + root.windows.length
    readonly property int actionFocus: root.actionsFocused && root.actionIndex < root.actionItems.length
        ? root.actionIndex : -1
    readonly property int windowFocus: root.actionsFocused && root.actionIndex >= root.actionItems.length
        ? root.actionIndex - root.actionItems.length : -1
    // what Shift+Delete or a kill button stops: the focused open window, else a window row's own
    readonly property string killTarget: root.windowFocus >= 0 && root.windowFocus < root.windows.length
        ? String(root.windows[root.windowFocus].id) : root.windowId
    // every window this lead names, so their programs' memory is read once per open
    readonly property var infoIds: root.windowId.length > 0 ? [root.windowId]
        : root.windows.map(win => String(win.id))

    // The selection-change rise the sub-views give their icon.
    property real rise: 0
    property real fade: 1
    readonly property alias scroller: scroll
    readonly property string selectionKey: root.row ? String(root.row.resultKey || "") : root.kind

    signal leaveActions()

    function moveAction(delta) {
        if (root.actionCount === 0)
            return;
        root.actionIndex = Math.max(0, Math.min(root.actionCount - 1, root.actionIndex + delta));
    }

    function killItem(id, label) {
        const armed = root.engine !== null && root.engine.armedKill === id;
        return {
            label: armed ? I18n.tr("Press again to kill") : label,
            danger: true,
            armed: armed,
            glyph: "dangerous"
        };
    }

    function memoryOf(id) {
        const info = root.engine ? root.engine.windowInfo[String(id)] : null;
        return info ? Rows.memoryLabel(info.bytes) : "";
    }

    function runAction() {
        root.runAt(root.actionsFocused ? root.actionIndex : 0);
    }

    function runAt(index) {
        if (!root.engine || !root.row)
            return;
        if (index === root.killIndex) {
            root.engine.requestKill(root.windowId);
            return;
        }
        if (index >= 0 && index < root.actions.length) {
            root.engine.run(root.row, root.actions[index]);
            return;
        }
        const win = root.windows[index - root.actionItems.length];
        if (win && typeof win.focus === "function")
            win.focus();
    }

    function killWindowAt(index) {
        const win = root.windows[index];
        if (win && root.engine)
            root.engine.requestKill(String(win.id));
    }

    function workspaceLabel(key) {
        const id = String(key || "");
        if (id.length === 0)
            return "";
        const ws = Wm.workspaceById(id) || Wm.workspaceByName(id);
        return ws && String(ws.name || "").length > 0 ? String(ws.name) : id;
    }

    onSelectionKeyChanged: {
        root.actionIndex = 0;
        scroll.contentY = 0;
        if (!K.Look.microMotion || K.Style.still) {
            arrival.stop();
            root.rise = 0;
            root.fade = 1;
            return;
        }
        arrival.restart();
    }
    onActionCountChanged: {
        if (root.actionIndex > Math.max(0, root.actionCount - 1))
            root.actionIndex = Math.max(0, root.actionCount - 1);
        if (root.actionsFocused && root.actionCount === 0)
            root.leaveActions();
    }
    onInfoIdsChanged: {
        if (root.engine && root.infoIds.length > 0)
            root.engine.needWindowInfo(root.infoIds);
    }

    Keys.onShortcutOverride: event => {
        if (event.key === Qt.Key_Escape && root.actionsFocused)
            event.accepted = true;
    }
    Keys.onPressed: event => {
        if (root.actionsFocused && (event.key === Qt.Key_Escape || event.key === Qt.Key_Backtab)) {
            root.leaveActions();
            event.accepted = true;
        }
    }

    ParallelAnimation {
        id: arrival
        NumberAnimation {
            target: root
            property: "fade"
            from: 0.2
            to: 1
            duration: K.Style.ms(90)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: K.Look.enterCurve
        }
        NumberAnimation {
            target: root
            property: "rise"
            from: 4
            to: 0
            duration: K.Style.ms(90)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: K.Look.enterCurve
        }
    }

    // the height the lead would like, so a short list can pour deeper instead of clipping it
    readonly property real naturalHeight: scroll.contentHeight
        + scroll.anchors.topMargin + scroll.anchors.bottomMargin

    // Lithic rules the column off with a hairline; the round designs lift the
    // lead onto its own plane.
    Rectangle {
        visible: K.Look.lithic
        x: 0
        width: K.Look.hairline
        height: parent.height
        color: K.Style.line
    }

    K.Shape {
        anchors.fill: parent
        visible: !K.Look.lithic
        cutSize: K.Look.material ? K.Look.cutXl : K.Look.adwaita ? K.Look.cutLg : K.Look.cutSm
        fill: K.Style.tint5
        strokeWidth: K.Look.fluent ? K.Style.border : 0
        strokeColor: K.Style.lineSoft
    }

    K.Scroller {
        id: scroll
        anchors.fill: parent
        anchors.leftMargin: K.Style.s4 + (K.Look.lithic ? K.Look.hairline : 0)
        anchors.rightMargin: K.Look.lithic ? 0 : K.Style.s4
        anchors.topMargin: K.Look.lithic ? K.Style.s2 : K.Style.s4
        anchors.bottomMargin: K.Look.lithic ? K.Style.s2 : K.Style.s4

        Loader {
            id: view
            width: scroll.width
            height: item ? item.implicitHeight : 0
            opacity: root.fade
            active: root.kind.length > 0
            sourceComponent: {
                switch (root.kind) {
                case "app": return appView;
                case "calc": return calcView;
                case "file": return fileView;
                case "window": return windowView;
                case "web": return webView;
                case "media": return mediaView;
                default: return genericView;
                }
            }
        }
    }

    Component {
        id: appView
        LeadApp { lead: root }
    }
    Component {
        id: calcView
        LeadCalc { lead: root }
    }
    Component {
        id: fileView
        LeadFile { lead: root }
    }
    Component {
        id: windowView
        LeadWindow { lead: root }
    }
    Component {
        id: webView
        LeadWeb { lead: root }
    }
    Component {
        id: mediaView
        LeadMedia { lead: root }
    }
    Component {
        id: genericView
        LeadGeneric { lead: root }
    }
}
