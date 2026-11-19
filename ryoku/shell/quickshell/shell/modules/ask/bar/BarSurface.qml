pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import ".."
import "../look"
import "../views"
import "../lib/keys.js" as Chords
import "../lib/root.js" as RootLogic

// The Rashin bar's body: one field, a stack of views under it, a foot that says
// what Enter does, and the Action Panel. It owns the keyboard and the stack; a
// view only answers the contract (bar-spec section 7).
FocusScope {
    id: root

    required property real s
    required property RashinClient rashin
    required property RashinSession session
    required property SearchClient search
    required property DownloadsClient downloads
    property bool active: false
    property string screenName: ""
    // ask (the root list) | web
    property string mode: "ask"
    property real baseWidth: 720 * s
    property real maximumWidth: baseWidth
    property real maximumHeight: 640 * s

    // [{view, args, query}]; the root is always first
    property var stack: [{ view: "root", args: ({}), query: "" }]
    property int direction: 1
    readonly property string topView: root.stack[root.stack.length - 1].view
    readonly property ViewSlot slot: root.topView === "ask" ? askSlot : root.topView === "web" ? webSlot : rootSlot
    readonly property var view: root.slot.view
    readonly property string outward: root.topView === "web" ? "web" : "ask"

    readonly property var primaryAction: root.view && root.view.primaryAction ? root.view.primaryAction : null
    readonly property var secondaryAction: root.view && root.view.secondaryAction ? root.view.secondaryAction : null
    readonly property var viewActions: root.view && root.view.actions ? root.view.actions : []
    // the panel lists what Enter and Ctrl+Enter do first, then the view's actions
    readonly property var panelActions: {
        const out = [];
        if (root.primaryAction)
            out.push({ id: "\u0000primary", label: root.primaryAction.label, glyph: root.primaryAction.glyph || "keyboard_return", hint: root.primaryAction.hint || "Enter", section: "" });
        if (root.secondaryAction)
            out.push({ id: "\u0000secondary", label: root.secondaryAction.label, glyph: root.secondaryAction.glyph || "subdirectory_arrow_right", hint: root.secondaryAction.hint || "Ctrl+Enter", section: "" });
        return out.concat(root.viewActions);
    }

    readonly property real stageBudget: Math.max(80 * root.s, root.maximumHeight - field.height - foot.height - 2)
    readonly property real stageWant: {
        const natural = root.view ? Math.min(root.stageBudget, root.view.implicitHeight) : 0;
        return panel.open ? Math.max(natural, Math.min(root.stageBudget, panel.height + 8 * root.s)) : natural;
    }
    property real stageHeight: root.stageWant
    readonly property real widthWant: root.view && root.view.preferredWidth > 0
        ? Math.min(root.maximumWidth, root.view.preferredWidth) : Math.min(root.maximumWidth, root.baseWidth)
    property real presentedWidth: root.widthWant
    readonly property real presentedHeight: field.height + root.stageHeight + foot.height

    signal requestClose()
    signal modeRequested(string mode)
    signal bubblePinned(string question, string answer)

    implicitWidth: root.presentedWidth
    implicitHeight: root.presentedHeight
    focus: true

    // the bar grows and shrinks with what it holds; it never jumps
    Behavior on stageHeight {
        enabled: root.active
        NumberAnimation { duration: Look.dur(Look.morphMs); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.arrive }
    }
    Behavior on presentedWidth {
        enabled: root.active
        NumberAnimation { duration: Look.dur(Look.morphMs); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.arrive }
    }

    function focusField() {
        if (panel.open)
            return;
        field.focusField();
    }

    function indexOfView(name) {
        for (var i = 0; i < root.stack.length; i++) {
            if (root.stack[i].view === name)
                return i;
        }
        return -1;
    }

    function inStack(name) {
        return root.indexOfView(name) >= 0;
    }

    function entryFor(name) {
        const at = root.indexOfView(name);
        return at >= 0 ? root.stack[at] : ({ view: name, args: ({}), query: "" });
    }

    // push opens a view over the current one; a view already below is returned to.
    function push(name, args, text) {
        if (name === "downloads") {
            const handoff = Object.assign({}, args || ({}));
            if (String(text || "").length > 0)
                handoff.query = String(text);
            field.text = "";
            ShellState.openKitDownloads(root.screenName, handoff);
            return;
        }
        if (["ask", "web"].indexOf(name) < 0)
            return;
        panel.open = false;
        var next = root.stack.slice();
        const last = next.length - 1;
        // the root gives its text away; a view keeps its own for the way back
        next[last] = { view: next[last].view, args: next[last].args, query: last === 0 ? "" : field.text };
        const at = root.indexOfView(name);
        if (at >= 0)
            next = next.slice(0, at);
        next.push({ view: name, args: args || ({}), query: String(text || "") });
        root.direction = 1;
        root.stack = next;
        field.text = String(text || "");
        root.focusField();
    }

    function pop() {
        if (root.stack.length <= 1)
            return false;
        panel.open = false;
        const next = root.stack.slice(0, -1);
        root.direction = -1;
        root.stack = next;
        field.text = next[next.length - 1].query;
        root.focusField();
        return true;
    }

    function resetTo(target) {
        panel.open = false;
        root.direction = 1;
        root.stack = target === "ask"
            ? [{ view: "root", args: ({}), query: "" }]
            : [{ view: "root", args: ({}), query: "" }, { view: target, args: ({}), query: "" }];
        field.text = "";
    }

    function quickAsk() {
        if (root.topView === "ask")
            return;
        root.push("ask", { question: field.text.trim() }, "");
    }

    function escapeLadder() {
        if (panel.open) {
            root.closePanel();
        } else if (root.view && root.view.handleEscape()) {
            return;
        } else if (field.text.length > 0) {
            field.clear();
        } else if (!root.pop()) {
            root.requestClose();
        }
    }

    function openPanel() {
        if (root.panelActions.length === 0)
            return;
        panel.open = true;
    }

    function closePanel() {
        panel.open = false;
        field.focusField();
    }

    function runPanel(id) {
        root.closePanel();
        if (!root.view)
            return;
        if (id === "\u0000primary")
            root.view.activate();
        else if (id === "\u0000secondary")
            root.view.activateSecondary();
        else
            root.view.act(id);
    }

    function handleKey(event) {
        const mods = event.modifiers & (Qt.ControlModifier | Qt.ShiftModifier | Qt.AltModifier);
        const ctrl = (mods & Qt.ControlModifier) !== 0;
        const plain = mods === 0;
        const key = event.key;
        if (!root.view)
            return;
        if (key === Qt.Key_Escape) {
            root.escapeLadder();
        } else if (ctrl && key === Qt.Key_K) {
            root.openPanel();
        } else if (key === Qt.Key_Return || key === Qt.Key_Enter) {
            if (ctrl)
                root.view.activateSecondary();
            else
                root.view.activate();
        } else if (key === Qt.Key_Down || key === Qt.Key_Up) {
            const delta = key === Qt.Key_Down ? 1 : -1;
            if (ctrl)
                root.view.jumpSection(delta);
            else
                root.view.move(delta);
        } else if (key === Qt.Key_PageDown || key === Qt.Key_PageUp) {
            root.view.page(key === Qt.Key_PageDown ? 1 : -1);
        } else if (key === Qt.Key_Tab && plain) {
            root.quickAsk();
        } else if (key === Qt.Key_Backspace && field.text.length === 0) {
            root.pop();
        } else if (mods === Qt.ControlModifier && (key === Qt.Key_1 || key === Qt.Key_2 || key === Qt.Key_3)) {
            const target = key === Qt.Key_1 ? "ask" : key === Qt.Key_2 ? "web" : "downloads";
            if (root.topView !== target)
                root.push(target, ({}), "");
        } else {
            const action = Chords.find(root.viewActions, key, mods);
            if (!action)
                return;
            root.view.act(String(action.id));
        }
        event.accepted = true;
    }

    function modeView(mode) {
        return mode === "web" ? "web" : "ask";
    }

    function applyMode() {
        const target = root.modeView(root.mode);
        if (target !== root.outward)
            root.resetTo(target);
    }

    function announce() {
        if (root.active && root.outward !== root.modeView(root.mode))
            root.modeRequested(root.outward);
    }

    // both run after the change that caused them has settled
    onModeChanged: Qt.callLater(root.applyMode)
    onOutwardChanged: Qt.callLater(root.announce)
    onActiveChanged: {
        if (!root.active) {
            panel.open = false;
            return;
        }
        root.applyMode();
        Qt.callLater(root.focusField);
    }
    Component.onCompleted: root.applyMode()

    SearchField {
        id: field
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        s: root.s
        token: root.view && root.topView !== "root" ? String(root.view.title || "") : ""
        placeholder: root.view ? String(root.view.placeholder || "") : ""
        accessory: root.view && root.view.accessory ? root.view.accessory : null
        onKeyPressed: event => root.handleKey(event)
        onBackRequested: root.pop()
        onEdited: text => {
            if (root.topView !== "root")
                return;
            const route = RootLogic.prefixRoute(text);
            if (route && route.external) {
                field.text = "";
                ShellState.openKitDownloads(root.screenName, { query: route.rest });
            } else if (route)
                root.push(route.view, ({}), route.rest);
        }
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: field.bottom
        height: 1
        color: Look.rule
        visible: !Look.fluent
        z: 3
    }

    Item {
        id: stage
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: field.bottom
        height: root.stageHeight
        clip: true

        ViewSlot {
            id: rootSlot
            s: root.s
            name: "root"
            live: true
            current: root.topView === "root"
            direction: root.direction
            barActive: root.active
            fieldText: field.text
            maximumHeight: root.stageBudget
            sourceComponent: RootView {
                s: root.s
                rashin: root.rashin
                session: root.session
                downloads: root.downloads
            }
        }
        ViewSlot {
            id: askSlot
            s: root.s
            name: "ask"
            live: root.inStack("ask")
            current: root.topView === "ask"
            direction: root.direction
            barActive: root.active
            fieldText: field.text
            heldText: root.entryFor("ask").query
            args: root.entryFor("ask").args
            maximumHeight: root.stageBudget
            sourceComponent: AskView {
                s: root.s
                rashin: root.rashin
                session: root.session
                onPinRequested: (question, answer) => root.bubblePinned(question, answer)
            }
        }
        ViewSlot {
            id: webSlot
            s: root.s
            name: "web"
            live: root.inStack("web")
            current: root.topView === "web"
            direction: root.direction
            barActive: root.active
            fieldText: field.text
            heldText: root.entryFor("web").query
            args: root.entryFor("web").args
            maximumHeight: root.stageBudget
            sourceComponent: WebView {
                s: root.s
                search: root.search
                downloads: root.downloads
            }
        }
    }

    Connections {
        target: root.view
        ignoreUnknownSignals: true
        function onPushRequested(view, args) { root.push(view, args, args && args.query ? String(args.query) : ""); }
        function onCloseRequested() { root.requestClose(); }
        function onHud(text) { foot.flash(text, false); }
        function onAlert(text) { foot.flash(text, true); }
        function onMenuRequested() { root.openPanel(); }
        function onQueryRequested(text) { field.text = String(text || ""); }
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: foot.top
        height: 1
        color: Look.rule
        z: 3
    }

    ActionBar {
        id: foot
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: stage.bottom
        s: root.s
        context: root.view && root.view.status !== undefined ? String(root.view.status || "") : ""
        contextItem: root.view && root.view.statusItem ? root.view.statusItem : null
        primary: root.primaryAction
        hasActions: root.panelActions.length > 0
        panelOpen: panel.open
        onPrimaryClicked: if (root.view) root.view.activate()
        onActionsClicked: panel.open ? root.closePanel() : root.openPanel()
    }

    // a press anywhere else in the bar puts the panel away
    MouseArea {
        anchors.fill: parent
        z: 4
        visible: panel.open
        acceptedButtons: Qt.AllButtons
        onPressed: root.closePanel()
    }

    ActionPanel {
        id: panel
        z: 5
        anchors.right: parent.right
        anchors.rightMargin: Math.round(8 * root.s)
        anchors.bottom: foot.top
        anchors.bottomMargin: Math.round(4 * root.s)
        s: root.s
        actions: root.panelActions
        note: root.view && root.view.actionNote ? String(root.view.actionNote) : ""
        maximumHeight: Math.max(120 * root.s, root.stageBudget - 12 * root.s)
        onPicked: id => root.runPanel(id)
        onDismissed: root.closePanel()
    }
}
