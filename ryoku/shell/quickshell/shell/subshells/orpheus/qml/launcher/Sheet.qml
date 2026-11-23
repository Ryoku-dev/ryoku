pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import shell.services as Svc
import "../kit" as K
import "lib/geometry.js" as Geometry
import "lib/keys.js" as KeyMap

// The launcher's content, hosted by both vessels: the search line, the track
// list beside the lead, and the foot. It draws no paper of its own (the vessel's
// blob is the paper) and reports `level`, the height the blob should pour to.
FocusScope {
    id: root

    property bool shown: false
    property bool welded: false
    property real maxHeight: 600
    property real waveRoom: 0
    property string monitor: ""

    readonly property var engine: engineItem
    readonly property real level: root.animatedLevel
    readonly property bool wantsMusicEdge: Svc.Media.playing && K.Look.ambientMotion && root.shown
    property bool actionsFocused: false

    signal closeRequested()

    readonly property real screenWidth: {
        const list = Quickshell.screens;
        for (let i = 0; i < list.length; i++)
            if (list[i] && list[i].name === root.monitor)
                return list[i].width;
        return list.length > 0 ? list[0].width : 1920;
    }
    readonly property real pad: K.Style.s4
    readonly property real gap: K.Style.s2
    readonly property real searchHeight: 56
    readonly property real footHeight: 34
    readonly property real labelHeight: root.engine.section.length > 0 ? 24 : 0
    readonly property real chrome: root.pad + root.searchHeight + root.gap + root.labelHeight
        + root.gap + root.footHeight + K.Style.s1
    // Ask answers live in the lead, so asking keeps the full list block open.
    readonly property var layout: Geometry.sheetLevel({
        count: root.engine.askMode ? Geometry.MAX_ROWS : root.engine.resultCount,
        rowHeight: K.Style.rowH,
        chrome: root.chrome,
        waveRoom: root.waveRoom,
        maxHeight: root.maxHeight,
        leadHeight: Math.max(0, lead.naturalHeight - root.labelHeight)
    })
    property bool levelAnimated: false
    property real animatedLevel: root.layout.level
    readonly property var columns: Geometry.columns(content.width, K.Style.s4)

    implicitWidth: Geometry.launcherWidth(root.screenWidth) * K.Style.scale
    implicitHeight: root.level

    Behavior on animatedLevel {
        enabled: root.levelAnimated && !K.Style.still
        NumberAnimation {
            duration: K.Style.ms(220)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: K.Look.enterCurve
        }
    }

    function focusSearch() {
        search.focusInput();
    }

    function stateDump() {
        return {
            query: root.engine.query,
            mode: root.engine.mode,
            selectedIndex: root.engine.selectedIndex,
            selectedKey: root.engine.selectedKey,
            resultCount: root.engine.resultCount,
            actionsFocused: root.actionsFocused
        };
    }

    function syncShown() {
        if (!root.shown) {
            root.engine.active = false;
            return;
        }
        root.levelAnimated = false;
        root.actionsFocused = false;
        foot.flashText = "";
        root.engine.active = true;
        root.engine.reset();
        list.armOpen();
        root.focusSearch();
        // the open starts at its level, only later changes pour
        Qt.callLater(function () { root.levelAnimated = true; });
    }

    function runSelected() {
        const row = root.engine.selectedRow;
        if (row) {
            root.engine.run(row, null);
            return;
        }
        if (root.engine.mode === "apps" && root.engine.query.trim().length > 0)
            root.engine.searchWeb();
    }

    // what an empty list should tell the reader to do next
    function emptyHint() {
        const query = String(root.engine.query || "").trim();
        switch (root.engine.mode) {
        case "files":
            return query.length === 0 ? I18n.tr("No recent files yet. Type a name to search your files.")
                : I18n.tr("No file called “%1”.").arg(query);
        case "windows":
            return query.length === 0 ? I18n.tr("No windows are open.") : I18n.tr("No window called “%1”.").arg(query);
        case "actions":
            return I18n.tr("No action called “%1”.").arg(query);
        case "apps":
            return "";
        default:
            return query.length === 0 ? "" : I18n.tr("Nothing matches “%1”.").arg(query);
        }
    }

    function handleKey(event) {
        const intent = KeyMap.intentFor(event.key, event.modifiers, {
            actionsFocused: root.actionsFocused,
            leadKeys: root.engine.askMode,
            ghost: root.engine.ghost.length > 0,
            caretAtEnd: search.caretAtEnd,
            queryEmpty: root.engine.query.length === 0,
            askBusy: lead.askBusy,
            modeCount: root.engine.modes.length
        });
        if (!intent)
            return;
        event.accepted = true;
        switch (intent.kind) {
        case "move":
            root.engine.move(intent.delta);
            break;
        case "moveAction":
            lead.moveAction(intent.delta);
            break;
        case "run":
            root.runSelected();
            break;
        case "runAction":
            lead.runAction();
            break;
        case "enterActions":
            if (lead.actionCount > 0) {
                lead.actionIndex = 0;
                root.actionsFocused = true;
            }
            break;
        case "leaveActions":
            root.actionsFocused = false;
            break;
        case "cancelAsk":
            lead.cancelAsk();
            break;
        case "clear":
            root.engine.query = "";
            break;
        case "close":
            root.closeRequested();
            break;
        case "acceptGhost":
            root.engine.acceptGhost();
            break;
        case "runIndex":
            root.engine.runIndex(intent.n);
            break;
        case "mode":
            root.engine.setMode(root.engine.modes[intent.n - 1]);
            break;
        default:
            break;
        }
    }

    // Esc first steps back inside the sheet, only an empty query lets the
    // vessel's Cancel shortcut close it.
    function overrideKey(event) {
        if (event.key !== Qt.Key_Escape)
            return;
        if (root.engine.query.length > 0 || root.actionsFocused || lead.askBusy)
            event.accepted = true;
    }

    onShownChanged: root.syncShown()
    Component.onCompleted: {
        if (root.shown)
            root.syncShown();
    }

    // Keys from anything else focused inside the sheet: navigation still
    // works and typing lands in the field.
    Keys.onShortcutOverride: event => root.overrideKey(event)
    Keys.onPressed: event => {
        if (search.inputFocused)
            return;
        root.handleKey(event);
        if (event.accepted)
            return;
        const chord = event.modifiers & (Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier);
        if (chord === 0 && event.text.length > 0 && event.text.charCodeAt(0) >= 32) {
            root.engine.query = root.engine.query + event.text;
            root.focusSearch();
            event.accepted = true;
        }
    }

    Engine {
        id: engineItem
        onCloseRequested: root.closeRequested()
        onFlashed: text => foot.flash(text)
        onSelectedKeyChanged: root.actionsFocused = false
        onQueryChanged: root.actionsFocused = false
    }

    // Empty sheet space takes the click (it never closes) and hands focus back.
    MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
        onPressed: root.focusSearch()
    }

    Item {
        id: content
        anchors.fill: parent
        anchors.leftMargin: root.pad
        anchors.rightMargin: root.pad
        anchors.topMargin: root.pad
        anchors.bottomMargin: root.waveRoom + K.Style.s1

        SearchLine {
            id: search
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: root.searchHeight
            engine: root.engine
            focus: true
            onKeyPressed: event => root.handleKey(event)
            onOverrideRequested: event => root.overrideKey(event)
        }

        Item {
            id: body
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: search.bottom
            anchors.topMargin: root.gap
            anchors.bottom: foot.top
            anchors.bottomMargin: root.gap

            Item {
                id: listColumn
                anchors.left: parent.left
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                width: root.columns.list
                clip: true

                K.Label {
                    anchors.left: parent.left
                    anchors.leftMargin: K.Style.s3
                    anchors.right: parent.right
                    anchors.top: parent.top
                    height: root.labelHeight
                    verticalAlignment: Text.AlignVCenter
                    visible: root.labelHeight > 0
                    role: "micro"
                    text: root.engine.section === "pinned" ? I18n.tr("Pinned") : I18n.tr("Most used")
                }

                TrackList {
                    id: list
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.topMargin: root.labelHeight
                    anchors.bottom: parent.bottom
                    engine: root.engine
                    live: root.shown
                    matchText: root.engine.askMode || root.engine.resting ? "" : root.engine.route.query
                    onHovered: index => {
                        if (index !== root.engine.selectedIndex)
                            root.engine.pick(index);
                    }
                    onPicked: index => {
                        root.engine.pick(index);
                        if (root.engine.askMode)
                            lead.runAction();
                        else
                            root.engine.run(root.engine.rows[index], null);
                        root.focusSearch();
                    }
                }

                K.Label {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.leftMargin: K.Style.s3
                    anchors.rightMargin: K.Style.s3
                    anchors.verticalCenter: parent.verticalCenter
                    visible: root.shown && list.count === 0 && !root.engine.busy && text.length > 0
                    role: "body"
                    tone: "muted"
                    wrapMode: Text.WordWrap
                    horizontalAlignment: Text.AlignHCenter
                    text: root.emptyHint()
                }
            }

            Lead {
                id: lead
                anchors.left: listColumn.right
                anchors.leftMargin: K.Style.s4
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                engine: root.engine
                row: root.engine.selectedRow
                actionsFocused: root.actionsFocused
                onLeaveActions: {
                    root.actionsFocused = false;
                    root.focusSearch();
                }
            }
        }

        Foot {
            id: foot
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: root.footHeight
            engine: root.engine
            actionsFocused: root.actionsFocused
            askBusy: lead.askBusy
        }
    }
}
