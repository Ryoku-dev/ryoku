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
    readonly property bool isAsk: root.kind === "ask"
    readonly property bool isFile: root.kind === "file"

    // The row's actions, primary first.
    readonly property var actions: {
        const row = root.row;
        if (!row || !row.primaryAction || root.isAsk)
            return [];
        return [row.primaryAction].concat(Array.isArray(row.secondaryActions) ? row.secondaryActions : []);
    }
    // An app's open windows, reached by the same Tab focus after its actions.
    readonly property var windows: root.kind === "app" && root.engine
        ? (root.engine.windowsFor(root.row) || []) : []

    readonly property var actionItems: root.actions.map((action, index) => ({
        label: root.isFile && action.id === "reveal" ? I18n.tr("Show in folder") : String(action.name || ""),
        primary: index === 0
    }))
    readonly property var windowItems: root.windows.map(win => ({
        label: String(win.title || ""),
        meta: root.workspaceLabel(win.workspace),
        glyph: "select_window"
    }))

    readonly property var askView: root.isAsk && view.item ? view.item : null
    readonly property int actionCount: root.isAsk
        ? (root.askView ? root.askView.chipCount : 0)
        : root.actions.length + root.windows.length
    readonly property bool askBusy: root.askView !== null && root.askView.busy
    readonly property int actionFocus: root.actionsFocused && root.actionIndex < root.actions.length
        ? root.actionIndex : -1
    readonly property int windowFocus: root.actionsFocused && root.actionIndex >= root.actions.length
        ? root.actionIndex - root.actions.length : -1

    // The selection-change rise the sub-views give their icon.
    property real rise: 0
    property real fade: 1
    readonly property alias scroller: scroll
    readonly property string selectionKey: root.row ? String(root.row.resultKey || "") : root.kind

    signal leaveActions()

    function moveAction(delta) {
        if (root.isAsk) {
            if (root.askView)
                root.askView.step(delta);
            return;
        }
        if (root.actionCount === 0)
            return;
        root.actionIndex = Math.max(0, Math.min(root.actionCount - 1, root.actionIndex + delta));
    }

    function runAction() {
        if (root.isAsk) {
            if (root.askView)
                root.askView.enter();
            return;
        }
        root.runAt(root.actionsFocused ? root.actionIndex : 0);
    }

    function runAt(index) {
        if (!root.engine || !root.row)
            return;
        if (index >= 0 && index < root.actions.length) {
            root.engine.run(root.row, root.actions[index]);
            return;
        }
        const win = root.windows[index - root.actions.length];
        if (win && typeof win.focus === "function")
            win.focus();
    }

    function cancelAsk() {
        if (root.askView)
            root.askView.cancel();
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
                case "ask": return askPane;
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
        id: askPane
        LeadAsk { lead: root }
    }
    Component {
        id: genericView
        LeadGeneric { lead: root }
    }
}
