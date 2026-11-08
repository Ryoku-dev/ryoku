// Kairos' launcher: the island bar style's app launcher, opened with Super+Space.
// The pill grows from the resting island to the full panel, so the shortcut reads
// as the island opening; the head is the clock, then the search row and the apps.
//
// The window maps at full size so the pill can morph without buffer resizes.

import QtQuick
import QtQuick.Effects
import QtQuick.Layouts
import Quickshell
import Quickshell.Wayland
import Quickshell.Widgets
import shell.services as Ryoku
import Ryoku.Ui.Singletons
import "../IslandMetrics.js" as Island
import "../components" as Bar

Scope {
    id: root

    // ── launcher contract (docs/launcher.md) ─────────────────────────────────
    property bool openRequested: false
    readonly property bool shown: openRequested || closeSettler.running
    property string monitor: ""
    Timer { id: closeSettler; interval: Math.max(1, Ryoku.Motion.morph) }

    function show(mon) {
        closeSettler.stop();
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
        return {
            open: root.openRequested,
            query: root.query,
            selectedIndex: root.sel,
            resultCount: root.rows.length,
            showHidden: root.showHidden
        };
    }

    // ── geometry ─────────────────────────────────────────────────────────────
    readonly property int panelW: 470
    readonly property int panelRadius: 40
    readonly property int padX: 26
    readonly property int rowH: 38
    readonly property int visibleRows: 6
    readonly property int clockCentreY: 39
    readonly property int searchY: 76
    readonly property int searchH: 40
    readonly property int listY: 126
    readonly property int listH: root.visibleRows * root.rowH
    readonly property int panelH: root.listY + root.listH + 24

    readonly property color fill: Ryoku.Theme.shadow
    readonly property color ink: Ryoku.Theme.ink(fill, 7)
    readonly property color accent: Ryoku.Theme.primary

    // ── apps ─────────────────────────────────────────────────────────────────
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

    readonly property var catalogue: {
        const all = DesktopEntries.applications.values
            .filter(a => root.showHidden || !a.noDisplay);
        all.sort((a, b) => (a.name || "").toLowerCase()
            .localeCompare((b.name || "").toLowerCase()));
        return all;
    }
    readonly property var rows: {
        const all = root.catalogue;
        const q = root.query.trim().toLowerCase();
        if (q.length === 0)
            return all.slice();
        return all.filter(a => (a.name || "").toLowerCase().includes(q)).sort((a, b) => {
            const an = (a.name || "").toLowerCase(), bn = (b.name || "").toLowerCase();
            const ap = an.startsWith(q) ? 0 : 1, bp = bn.startsWith(q) ? 0 : 1;
            return ap !== bp ? ap - bp : an.localeCompare(bn);
        });
    }

    function step(d) {
        if (root.rows.length === 0)
            return;
        root.sel = Math.max(0, Math.min(root.rows.length - 1, root.sel + d));
    }
    function run() {
        const e = root.rows[root.sel];
        if (!e)
            return;
        root.hide();
        Ryoku.AppLaunch.run(e, null);
    }
    // 0 until the morph reaches a band, 1 when it has: the reveal ramp the search
    // row and the list ride in on.
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

            // The morph starts only once the surface is on screen: a Behavior
            // started at the state flip would run its course while the window is
            // still mapping, and the launcher would appear already open. Derived
            // rather than latched, so a quick close-then-open -- the window still
            // mapped, so no visibility change -- grows straight away instead of
            // staying collapsed.
            readonly property bool armed: root.openRequested && win.backingWindowVisible
            property real p: win.armed ? 1 : 0

            Behavior on p {
                enabled: !Ryoku.Motion.reduce
                NumberAnimation { duration: Ryoku.Motion.morph; easing.type: Ryoku.Motion.easeStandard }
            }

            readonly property real startWidth: startClock.implicitWidth + 2 * Island.restPadX
            readonly property real clockPx:
                Island.restFontSize + (Island.launcherFontSize - Island.restFontSize) * p
            readonly property real clockCentre:
                Island.restHeight / 2
                    + (root.clockCentreY - Island.restHeight / 2) * p

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
                color: root.fill
                // Everything below the clock is laid out for the open panel; the
                // clip is what makes it arrive as the island grows.
                clip: true
                MouseArea {
                    anchors.fill: parent
                    acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                    onPressed: event => event.accepted = true
                }

                Bar.Clock { id: clock }

                // Never drawn: holds the resting pill's width, which is where the
                // morph starts.
                Text {
                    id: startClock
                    visible: false
                    text: clock.time
                    font.family: Ryoku.Theme.mono
                    font.pixelSize: Island.restFontSize
                }

                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: Math.round(win.clockCentre - height / 2)
                    text: clock.time
                    color: root.ink
                    font.family: Ryoku.Theme.mono
                    font.pixelSize: Math.round(win.clockPx)
                    font.weight: Font.Medium
                    font.letterSpacing: 1
                }

                Rectangle {
                    id: searchRow
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: root.searchY
                    width: parent.width - 2 * root.padX
                    height: root.searchH
                    radius: height / 2
                    color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.08)
                    opacity: root.revealed(0.35, 0.8, win.p)

                    RowLayout {
                        anchors.fill: parent
                        anchors.leftMargin: 16
                        anchors.rightMargin: 8
                        spacing: 10

                        Text {
                            text: "search"
                            color: root.ink
                            opacity: 0.75
                            font.family: "Material Symbols Rounded"
                            font.pixelSize: 19
                        }
                        TextInput {
                            id: field
                            Layout.fillWidth: true
                            color: root.ink
                            font.family: Ryoku.Theme.fontPrimary
                            font.pixelSize: 15
                            selectByMouse: true
                            onTextChanged: if (text !== root.query) { root.query = text; root.sel = 0; }
                            Connections {
                                target: root
                                function onOpenRequestedChanged() {
                                    if (!root.openRequested)
                                        return;
                                    field.text = "";
                                    field.forceActiveFocus();
                                }
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                visible: field.text.length === 0
                                text: I18n.tr("Search")
                                color: root.ink
                                opacity: 0.45
                                font: field.font
                            }
                            Keys.onEscapePressed: root.hide()
                            Keys.onReturnPressed: root.run()
                            Keys.onEnterPressed: root.run()
                            Keys.onDownPressed: root.step(1)
                            Keys.onUpPressed: root.step(-1)
                            Keys.onPressed: event => {
                                if (event.key === Qt.Key_PageDown) { root.step(6); event.accepted = true; }
                                else if (event.key === Qt.Key_PageUp) { root.step(-6); event.accepted = true; }
                            }
                        }
                        Rectangle {
                            Layout.preferredWidth: 30
                            Layout.preferredHeight: 30
                            radius: height / 2
                            color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b,
                                root.showHidden ? 0.22 : 0.07)
                            Text {
                                anchors.centerIn: parent
                                text: root.showHidden ? "visibility" : "visibility_off"
                                color: root.ink
                                opacity: 0.85
                                font.family: "Material Symbols Rounded"
                                font.pixelSize: 17
                            }
                            MouseArea {
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                onClicked: { root.showHidden = !root.showHidden; root.sel = 0; }
                            }
                        }
                    }
                }

                Item {
                    id: listWrap
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: root.listY
                    width: parent.width - 2 * root.padX
                    height: root.listH
                    opacity: root.revealed(0.5, 1, win.p)

                    Rectangle {
                        anchors.fill: parent
                        radius: 14
                        color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.04)
                    }

                    Text {
                        anchors.centerIn: parent
                        visible: root.rows.length === 0
                        text: I18n.tr("No matches")
                        color: root.ink
                        opacity: 0.5
                        font.family: Ryoku.Theme.fontPrimary
                        font.pixelSize: 13
                    }

                    ListView {
                        id: list
                        anchors.fill: parent
                        anchors.margins: 4
                        clip: true
                        model: ScriptModel { values: root.rows }
                        // root.sel is the selection, not the view's own current item:
                        // a ListView rewrites currentIndex when its model changes or an
                        // ApplyRange pulls it into view, which killed the binding and
                        // left the plate parked on one row while Enter ran another. The
                        // plate is placed from sel below, and the list scrolls to it.
                        currentIndex: -1
                        highlightFollowsCurrentItem: false
                        boundsBehavior: Flickable.StopAtBounds
                        cacheBuffer: root.rowH * 8

                        Connections {
                            target: root
                            function onSelChanged() { list.positionViewAtIndex(root.sel, ListView.Contain) }
                        }
                        highlight: Rectangle {
                            y: root.sel * root.rowH
                            width: list.width
                            height: root.rowH
                            radius: 11
                            color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.10)
                            Behavior on y {
                                enabled: !root.settling
                                NumberAnimation { duration: 200; easing.type: Easing.OutCubic }
                            }
                        }

                        delegate: Item {
                            id: appRow
                            required property int index
                            required property var modelData
                            width: ListView.view.width
                            height: root.rowH
                            readonly property bool on: appRow.index === root.sel

                            RowLayout {
                                anchors.fill: parent
                                anchors.leftMargin: 12 + (appRow.on ? 6 : 0)
                                anchors.rightMargin: 14
                                spacing: 11
                                Behavior on anchors.leftMargin {
                                    enabled: !root.settling
                                    NumberAnimation { duration: 180; easing.type: Easing.InOutQuad }
                                }

                                IconImage {
                                    implicitSize: 20
                                    source: Ryoku.Icons.path((appRow.modelData && appRow.modelData.icon)
                                        || "application-x-executable", true)
                                }
                                Text {
                                    Layout.fillWidth: true
                                    text: appRow.modelData ? appRow.modelData.name : ""
                                    color: root.ink
                                    font.family: Ryoku.Theme.fontPrimary
                                    font.pixelSize: 13
                                    font.weight: appRow.on ? Font.DemiBold : Font.Normal
                                    elide: Text.ElideRight
                                }
                                Text {
                                    visible: appRow.on
                                    text: I18n.tr("Run")
                                    color: root.accent
                                    font.family: Ryoku.Theme.fontPrimary
                                    font.pixelSize: 11
                                }
                            }

                            MouseArea {
                                anchors.fill: parent
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onEntered: root.sel = appRow.index
                                onClicked: { root.sel = appRow.index; root.run(); }
                            }
                        }
                    }
                }
            }
        }
    }
}
