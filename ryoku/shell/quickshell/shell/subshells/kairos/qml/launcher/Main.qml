// the window maps at full size so the pill can morph without buffer resizes

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import shell.services as Ryoku
import Ryoku.Ui.Singletons
import Ryoku.Text
import "../IslandMetrics.js" as Island
import "../components" as Bar
import "../kit"

Scope {
    id: root

    // launcher contract, see docs/launcher.md
    property bool openRequested: false
    readonly property bool shown: openRequested || closeSettler.running
    property string monitor: ""
    Timer {
        id: closeSettler
        interval: Math.max(1, Md.morph)
        onTriggered: root.flushBump()
    }

    function show(mon) {
        closeSettler.stop();
        root.flushBump();
        root.monitor = mon || "";
        root.query = "";
        root.sel = 0;
        settleDelay.stop();
        root.openRequested = true;
    }
    function hide() {
        if (!root.openRequested)
            return;
        closeSettler.restart();
        root.openRequested = false;
    }
    function toggle(mon) { root.openRequested ? root.hide() : root.show(mon); }
    function stateDump() {
        const e = root.cells[root.sel];
        return {
            open: root.openRequested,
            query: root.query,
            selectedIndex: root.sel,
            selectedId: e ? String(e.id) : "",
            resultCount: root.listed.length,
            suggestedCount: root.nSug,
            showHidden: root.showHidden
        };
    }

    readonly property int cols: 5
    readonly property int cellW: 88
    readonly property int cellH: 98
    readonly property int padX: 20
    readonly property int panelW: root.cols * root.cellW + 2 * root.padX
    readonly property int panelRadius: Md.rXxl
    readonly property int clockCentreY: 39
    readonly property int searchY: 72
    readonly property int searchH: 52
    readonly property int drawerY: root.searchY + root.searchH + 12
    readonly property int sugLabelH: 26
    readonly property int headerH: root.sugLabelH + root.cellH + 8
    // a row always peeks out at the bottom, with or without the suggested row
    readonly property int drawerH: root.headerH + root.cellH * 3 + 34
    readonly property int panelH: root.drawerY + root.drawerH + 12
    readonly property int pageRows: 3

    readonly property string usagePath:
        (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state"))
            + "/ryoku/launcher-usage.json"
    property var usage: ({})

    function parseUsage(raw) {
        try {
            const v = raw && raw.length ? JSON.parse(raw) : {};
            return v && typeof v === "object" && !Array.isArray(v) ? v : {};
        } catch (e) {
            return {};
        }
    }
    function count(id) {
        const c = id ? root.usage[id] : 0;
        return typeof c === "number" ? c : 0;
    }
    // re-read first so a launch another launcher counted meanwhile survives
    function bump(id) {
        if (!id)
            return;
        usageFile.reload();
        const disk = root.parseUsage(usageFile.text());
        disk[id] = (typeof disk[id] === "number" ? disk[id] : 0) + 1;
        usageFile.setText(JSON.stringify(disk));
        root.usage = disk;
    }

    FileView {
        id: usageFile
        path: root.usagePath
        blockLoading: true
        atomicWrites: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.usage = root.parseUsage(text())
        onLoadFailed: root.usage = ({})
    }

    property string query: ""
    property int sel: 0
    property bool showHidden: false
    property bool settling: false
    Timer {
        id: settleDelay
        interval: 90
        onTriggered: root.settling = false
    }
    onQueryChanged: {
        root.settling = true;
        settleDelay.restart();
    }

    readonly property var catalogue: DesktopEntries.applications.values
        .filter(a => root.showHidden || !a.noDisplay)
    readonly property var listed: FuzzyRanker.prefixRank(root.catalogue, root.query, "name")
    readonly property var suggested: root.query.length > 0 ? [] : root.catalogue
        .filter(a => root.count(a.id) > 0)
        .sort((a, b) => root.count(b.id) - root.count(a.id) || String(a.name).localeCompare(String(b.name)))
        .slice(0, root.cols)
    readonly property int nSug: root.suggested.length
    readonly property var cells: root.suggested.concat(root.listed)
    readonly property int rowCount: (root.nSug > 0 ? 1 : 0) + Math.ceil(root.listed.length / root.cols)

    function rowOf(i) {
        if (i < root.nSug)
            return 0;
        return (root.nSug > 0 ? 1 : 0) + Math.floor((i - root.nSug) / root.cols);
    }
    function colOf(i) { return i < root.nSug ? i : (i - root.nSug) % root.cols; }
    function indexAt(row, col) {
        if (root.nSug > 0 && row === 0)
            return Math.min(col, root.nSug - 1);
        const r = row - (root.nSug > 0 ? 1 : 0);
        const inRow = Math.min(root.cols, root.listed.length - r * root.cols);
        return root.nSug + r * root.cols + Math.min(col, inRow - 1);
    }
    function moveRows(d) {
        if (root.cells.length === 0)
            return;
        const row = Math.max(0, Math.min(root.rowCount - 1, root.rowOf(root.sel) + d));
        root.sel = root.indexAt(row, root.colOf(root.sel));
    }
    function moveBy(d) {
        if (root.cells.length === 0)
            return;
        root.sel = Math.max(0, Math.min(root.cells.length - 1, root.sel + d));
    }
    // counted after the close settles so the suggested row can't reshuffle mid-collapse
    property string pendingBump: ""
    function flushBump() {
        const id = root.pendingBump;
        root.pendingBump = "";
        root.bump(id);
    }
    function launch(entry, action) {
        if (!entry)
            return;
        root.flushBump();
        root.pendingBump = String(entry.id);
        root.hide();
        Ryoku.AppLaunch.run(entry, action);
    }
    function revealed(from, to, progress) {
        return Math.max(0, Math.min(1, (progress - from) / (to - from)));
    }

    Variants {
        model: Ryoku.Screens.uniqueByName(Quickshell.screens)

        PanelWindow {
            id: win
            required property var modelData
            screen: modelData
            visible: (root.shown || win.p > 0.001)
                && (root.monitor.length === 0 || root.monitor === String(modelData.name))
            color: "transparent"
            exclusionMode: ExclusionMode.Ignore
            WlrLayershell.layer: WlrLayer.Overlay
            WlrLayershell.namespace: "launcher"
            WlrLayershell.keyboardFocus:
                root.openRequested ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
            anchors { top: true; bottom: true; left: true; right: true }

            // derived, not latched: a Behavior started before the surface maps
            // would finish unseen, and a quick reopen must grow again
            readonly property bool armed: root.openRequested && win.backingWindowVisible
            property real p: win.armed ? 1 : 0

            Behavior on p {
                enabled: !Md.still
                Spring {}
            }

            readonly property real startWidth: startClock.implicitWidth + 2 * Island.restPadX
            readonly property real clockPx:
                Island.restFontSize + (Island.launcherFontSize - Island.restFontSize) * p
            readonly property real clockCentre:
                Island.restHeight / 2
                    + (root.clockCentreY - Island.restHeight / 2) * p

            property var menuEntry: null
            property Item menuReturn: null

            function openMenu(entry, at) {
                win.menuReturn = grid.activeFocus ? grid : field;
                win.menuEntry = entry;
                menu.current = 0;
                menu.x = Math.max(8, Math.min(win.width - menu.width - 8, at.x));
                const h = menu.implicitHeight;
                const below = at.y + h + 8 <= win.height;
                menu.y = Math.max(8, Math.min(win.height - h - 8, below ? at.y : at.y - h));
                menu.forceActiveFocus();
            }
            function closeMenu() {
                win.menuEntry = null;
                (win.menuReturn || field).forceActiveFocus();
            }
            function cellItem(i) {
                if (i < root.nSug)
                    return grid.headerItem ? grid.headerItem.cellAt(i) : null;
                return grid.itemAtIndex(i - root.nSug);
            }
            function menuForSelection() {
                const item = win.cellItem(root.sel);
                if (!item)
                    return;
                win.openMenu(root.cells[root.sel],
                    item.mapToItem(win.contentItem, item.width / 2, item.height / 2));
            }
            function ensureVisible() {
                if (root.sel < root.nSug)
                    grid.positionViewAtBeginning();
                else
                    grid.positionViewAtIndex(root.sel - root.nSug, GridView.Contain);
            }
            // Left and Right stay with the text cursor while there is text
            function navKey(event, fromField) {
                const k = event.key;
                const textual = fromField && field.text.length > 0;
                if (k === Qt.Key_Escape)
                    root.hide();
                else if (k === Qt.Key_Return || k === Qt.Key_Enter)
                    root.launch(root.cells[root.sel], null);
                else if (k === Qt.Key_Down)
                    root.moveRows(1);
                else if (k === Qt.Key_Up)
                    root.moveRows(-1);
                else if (k === Qt.Key_PageDown)
                    root.moveRows(root.pageRows);
                else if (k === Qt.Key_PageUp)
                    root.moveRows(-root.pageRows);
                else if (k === Qt.Key_Right && !textual)
                    root.moveBy(1);
                else if (k === Qt.Key_Left && !textual)
                    root.moveBy(-1);
                else if (k === Qt.Key_Tab || k === Qt.Key_Backtab)
                    (fromField ? grid : field).forceActiveFocus();
                else if (k === Qt.Key_Menu || (k === Qt.Key_F10 && (event.modifiers & Qt.ShiftModifier)))
                    win.menuForSelection();
                else
                    return false;
                event.accepted = true;
                return true;
            }

            Connections {
                target: root
                function onSelChanged() { win.ensureVisible(); }
                function onQueryChanged() { grid.positionViewAtBeginning(); }
                function onOpenRequestedChanged() {
                    win.menuEntry = null;
                    if (!root.openRequested)
                        return;
                    field.text = "";
                    field.forceActiveFocus();
                    grid.positionViewAtBeginning();
                }
            }

            MouseArea {
                anchors.fill: parent
                enabled: root.openRequested
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                onPressed: Qt.callLater(root.hide)
            }

            RectangularShadow {
                anchors.fill: pill
                radius: pill.radius
                blur: 16
                spread: 0
                offset: Qt.vector2d(0, 3)
                color: Qt.rgba(0, 0, 0, 0.5)
                z: -1
            }

            Rectangle {
                id: pill
                anchors.horizontalCenter: parent.horizontalCenter
                y: Island.topGap
                width: Math.round(win.startWidth + (root.panelW - win.startWidth) * win.p)
                height: Math.round(Island.restHeight
                    + (root.panelH - Island.restHeight) * win.p)
                radius: Island.restHeight / 2
                    + (root.panelRadius - Island.restHeight / 2) * win.p
                color: Md.base
                // the clip is what makes the drawer arrive as the island grows
                clip: true
                MouseArea {
                    anchors.fill: parent
                    acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                    onPressed: event => event.accepted = true
                }

                Bar.Clock { id: clock }

                // never drawn: the resting pill's width, where the morph starts
                Text {
                    id: startClock
                    visible: false
                    text: clock.time
                    font.family: Md.mono
                    font.pixelSize: Island.restFontSize
                }

                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: Math.round(win.clockCentre - height / 2)
                    text: clock.time
                    color: Md.ink
                    font.family: Md.mono
                    font.pixelSize: Math.round(win.clockPx)
                    font.weight: Font.Medium
                    font.letterSpacing: 1
                }

                Rectangle {
                    id: searchPill
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: root.searchY
                    width: root.panelW - 2 * root.padX
                    height: root.searchH
                    radius: height / 2
                    color: field.activeFocus ? Md.containerHigh : Md.container
                    opacity: root.revealed(0.35, 0.8, win.p)
                    Behavior on color {
                        enabled: !Md.still
                        ColorAnimation { duration: Md.fast }
                    }

                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.IBeamCursor
                        onPressed: field.forceActiveFocus()
                    }

                    Icon {
                        id: lens
                        anchors { left: parent.left; leftMargin: 18; verticalCenter: parent.verticalCenter }
                        name: "search"
                        size: 22
                        color: Md.inkMuted
                    }

                    TextInput {
                        id: field
                        anchors {
                            left: lens.right; leftMargin: 14
                            right: hiddenToggle.left; rightMargin: 8
                            verticalCenter: parent.verticalCenter
                        }
                        clip: true
                        color: Md.ink
                        font.family: Md.font
                        font.pixelSize: 16
                        selectionColor: Md.accent
                        selectedTextColor: Md.onAccent
                        selectByMouse: true
                        onTextChanged: if (text !== root.query) { root.query = text; root.sel = 0; }
                        Keys.onPressed: event => win.navKey(event, true)

                        Label {
                            anchors.fill: parent
                            visible: field.text.length === 0
                            role: "body"
                            font.pixelSize: 16
                            text: I18n.tr("Search apps")
                            color: Md.inkFaint
                        }
                    }

                    IconButton {
                        id: hiddenToggle
                        anchors { right: parent.right; rightMargin: 6; verticalCenter: parent.verticalCenter }
                        icon: root.showHidden ? "visibility" : "visibility_off"
                        checked: root.showHidden
                        tip: root.showHidden ? I18n.tr("Hide hidden apps") : I18n.tr("Show hidden apps")
                        activeFocusOnTab: false
                        onClicked: {
                            root.showHidden = !root.showHidden;
                            root.sel = 0;
                        }
                    }
                }

                Item {
                    id: drawer
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: root.drawerY
                    width: root.cols * root.cellW
                    height: root.drawerH
                    opacity: root.revealed(0.5, 1, win.p)

                    GridView {
                        id: grid
                        anchors.fill: parent
                        clip: true
                        cellWidth: root.cellW
                        cellHeight: root.cellH
                        boundsBehavior: Flickable.StopAtBounds
                        cacheBuffer: root.cellH * 4
                        keyNavigationEnabled: false
                        // a view rewrites currentIndex on model changes, so the
                        // plate would sit on one app while Enter ran another
                        currentIndex: -1
                        highlightFollowsCurrentItem: false
                        model: ScriptModel { values: root.listed }
                        Keys.onPressed: event => {
                            if (win.navKey(event, false))
                                return;
                            if (event.key === Qt.Key_Backspace) {
                                field.forceActiveFocus();
                                field.remove(field.text.length - 1, field.text.length);
                                event.accepted = true;
                            } else if (event.text.length > 0 && event.text.charCodeAt(0) >= 32
                                    && !(event.modifiers & (Qt.ControlModifier | Qt.AltModifier))) {
                                field.forceActiveFocus();
                                field.insert(field.text.length, event.text);
                                event.accepted = true;
                            }
                        }

                        header: Item {
                            id: head
                            width: grid.width
                            height: root.nSug > 0 ? root.headerH : 0
                            visible: root.nSug > 0

                            function cellAt(i) { return suggestions.itemAt(i); }

                            Label {
                                x: 12
                                height: root.sugLabelH
                                role: "label"
                                color: Md.inkMuted
                                text: I18n.tr("Suggested")
                            }

                            Row {
                                y: root.sugLabelH
                                Repeater {
                                    id: suggestions
                                    model: root.suggested
                                    delegate: AppCell {
                                        required property var modelData
                                        required property int index
                                        width: root.cellW
                                        height: root.cellH
                                        entry: modelData
                                        selected: index === root.sel
                                        animate: !root.settling
                                        onActivated: root.launch(modelData, null)
                                        onSecondary: (x, y) => {
                                            root.sel = index;
                                            win.openMenu(modelData, mapToItem(win.contentItem, x, y));
                                        }
                                    }
                                }
                            }

                            Rectangle {
                                x: 12
                                y: root.sugLabelH + root.cellH + 4
                                width: parent.width - 24
                                height: 1
                                color: Md.outlineSoft
                            }
                        }

                        delegate: AppCell {
                            required property var modelData
                            required property int index
                            width: root.cellW
                            height: root.cellH
                            entry: modelData
                            selected: index + root.nSug === root.sel
                            animate: !root.settling
                            onActivated: root.launch(modelData, null)
                            onSecondary: (x, y) => {
                                root.sel = index + root.nSug;
                                win.openMenu(modelData, mapToItem(win.contentItem, x, y));
                            }
                        }
                    }

                    Column {
                        anchors.centerIn: parent
                        width: parent.width - 48
                        visible: root.query.length > 0 && root.listed.length === 0
                        spacing: 8

                        Icon {
                            anchors.horizontalCenter: parent.horizontalCenter
                            name: "search_off"
                            size: 40
                            color: Md.inkFaint
                        }
                        Label {
                            width: parent.width
                            horizontalAlignment: Text.AlignHCenter
                            role: "title"
                            text: I18n.tr("No apps match \"%1\"").replace("%1", root.query)
                        }
                        Label {
                            width: parent.width
                            horizontalAlignment: Text.AlignHCenter
                            role: "body"
                            color: Md.inkMuted
                            wrapMode: Text.Wrap
                            text: root.showHidden
                                ? I18n.tr("Check the spelling or try fewer letters.")
                                : I18n.tr("Check the spelling, or look among hidden apps.")
                        }
                        Item { width: 1; height: 4 }
                        Button {
                            anchors.horizontalCenter: parent.horizontalCenter
                            visible: !root.showHidden
                            variant: "tonal"
                            icon: "visibility"
                            text: I18n.tr("Show hidden apps")
                            onClicked: {
                                root.showHidden = true;
                                root.sel = 0;
                            }
                        }
                    }
                }
            }

            MouseArea {
                anchors.fill: parent
                z: 5
                visible: win.menuEntry !== null
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                onPressed: win.closeMenu()
            }

            AppMenu {
                id: menu
                z: 6
                width: implicitWidth
                height: implicitHeight
                maxHeight: win.height - 16
                visible: win.menuEntry !== null
                entry: win.menuEntry
                onPicked: action => {
                    const entry = win.menuEntry;
                    win.menuEntry = null;
                    root.launch(entry, action);
                }
                onDismissed: win.closeMenu()
            }
        }
    }
}
