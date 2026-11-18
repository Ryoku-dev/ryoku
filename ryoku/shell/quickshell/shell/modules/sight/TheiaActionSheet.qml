pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

FocusScope {
    id: root
    required property var panel
    required property var catalog
    required property var card
    property real s: 1
    property bool open: false
    property real reveal: 0
    property int currentIndex: 0
    property bool pinFocused: false
    readonly property real lithicScale: s
    readonly property var planes: Lithic.planes(Lithic.design)
    readonly property color plane: Lithic.design === "lithic" ? planes.pane : Lithic.design === "fluent" ? Lithic.mix(planes.card, Lithic.sel, 0.025) : Lithic.stone4
    readonly property color stroke: Lithic.design === "material" ? "transparent" : Lithic.design === "adwaita" ? Lithic.alpha(Lithic.fg, Lithic.dark ? 0.10 : 0.16) : Lithic.design === "fluent" ? planes.cardStroke : Lithic.vein
    readonly property var enterCurve: Lithic.design === "material" ? [0.42,1.67,0.21,0.9,1,1] : Lithic.design === "adwaita" ? [0.25,0.46,0.45,0.94,1,1] : Lithic.design === "fluent" ? [0,0,0,1,1,1] : [0,0,0.2,1,1,1]
    readonly property var groups: {
        const source = catalog.available(card);
        const read = [], write = [], translate = [];
        for (let i = 0; i < source.length; ++i) {
            const row = source[i];
            if (row.group === "writing") write.push(row);
            else if (row.group === "translate" || row.id === "translate") translate.push(row);
            else read.push(row);
        }
        const result = [];
        let offset = 0;
        if (read.length) { result.push({label: I18n.tr("Read"), rows: read, offset: offset}); offset += read.length; }
        if (write.length) { result.push({label: I18n.tr("Write"), rows: write, offset: offset}); offset += write.length; }
        if (translate.length) result.push({label: I18n.tr("Translate"), rows: translate, offset: offset});
        return result;
    }
    readonly property var rows: {
        const result = [];
        for (let i = 0; i < groups.length; ++i)
            for (let j = 0; j < groups[i].rows.length; ++j) result.push(groups[i].rows[j]);
        return result;
    }
    readonly property int columns: width >= 840 * s ? 3 : 1
    signal chosen(string id)

    implicitWidth: 400 * s
    implicitHeight: grid.implicitHeight + LithicTokens.space.s3 * 2 * s
    visible: open || reveal > 0
    enabled: open
    opacity: Math.max(0, Math.min(1, reveal))
    scale: 0.97 + 0.03 * reveal
    transformOrigin: Item.TopRight
    activeFocusOnTab: open
    Accessible.role: Accessible.Pane
    Accessible.name: I18n.tr("Theia actions")

    Behavior on reveal {
        NumberAnimation {
            duration: Lithic.duration(root.open ? 140 : 100)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: root.open ? root.enterCurve : [0.3,0,0.8,0.15,1,1]
        }
    }
    onOpenChanged: {
        reveal = open ? 1 : 0;
        if (open) {
            currentIndex = 0;
            pinFocused = false;
            Qt.callLater(function() { if (root.open) { root.forceActiveFocus(Qt.PopupFocusReason); root.ensureCurrentVisible(); } });
        }
    }
    onRowsChanged: if (currentIndex >= rows.length) currentIndex = Math.max(0, rows.length - 1)
    Component.onCompleted: if (open) { reveal = 1; forceActiveFocus(Qt.PopupFocusReason); }

    function close() {
        panel.sheetOpen = false;
        panel.forceActiveFocus(Qt.PopupFocusReason);
    }
    function choose(id) {
        close();
        chosen(id);
    }
    function isPinned(id) { return catalog.pinned.indexOf(id) >= 0; }
    function pinnable(row) { return row && row.group !== "core"; }
    function canPin(row) { return pinnable(row) && (isPinned(row.id) || catalog.pinned.length < 2); }
    function togglePin(row) {
        if (!canPin(row)) return;
        const ids = catalog.pinned.slice();
        const index = ids.indexOf(row.id);
        if (index >= 0) ids.splice(index, 1);
        else if (ids.length < 2) ids.push(row.id);
        catalog.setPinned(ids);
    }
    function move(step) {
        if (!rows.length) return;
        currentIndex = (currentIndex + step + rows.length) % rows.length;
        if (!pinnable(rows[currentIndex])) pinFocused = false;
        ensureCurrentVisible();
    }
    function ensureCurrentVisible() {
        for (let i = 0; i < groupRepeater.count; ++i) {
            const group = groupRepeater.itemAt(i);
            if (!group) continue;
            const local = currentIndex - group.modelData.offset;
            if (local < 0 || local >= group.modelData.rows.length) continue;
            const row = group.rowRepeater.itemAt(local);
            if (!row) return;
            const point = row.mapToItem(grid, 0, 0);
            if (point.y < scroll.contentY) scroll.contentY = point.y;
            else if (point.y + row.height > scroll.contentY + scroll.height) scroll.contentY = point.y + row.height - scroll.height;
            scroll.contentY = Math.max(0, Math.min(scroll.contentY, Math.max(0, scroll.contentHeight - scroll.height)));
            return;
        }
    }
    Keys.priority: Keys.BeforeItem
    Keys.onPressed: event => {
        if (event.key === Qt.Key_Escape) close();
        else if (event.key === Qt.Key_Down) move(1);
        else if (event.key === Qt.Key_Up) move(-1);
        else if (event.key === Qt.Key_Home) { currentIndex = 0; ensureCurrentVisible(); }
        else if (event.key === Qt.Key_End) { currentIndex = Math.max(0, rows.length - 1); ensureCurrentVisible(); }
        else if (event.key === Qt.Key_Left) pinFocused = false;
        else if (event.key === Qt.Key_Right) pinFocused = pinnable(rows[currentIndex]);
        else if (event.key === Qt.Key_Tab || event.key === Qt.Key_Backtab) {
            const backwards = event.key === Qt.Key_Backtab || (event.modifiers & Qt.ShiftModifier);
            if (backwards) {
                if (pinFocused) pinFocused = false;
                else { move(-1); pinFocused = pinnable(rows[currentIndex]); }
            } else if (!pinFocused && pinnable(rows[currentIndex])) pinFocused = true;
            else { pinFocused = false; move(1); }
        } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
            const row = rows[currentIndex];
            if (row) { if (pinFocused) togglePin(row); else choose(row.id); }
        } else return;
        event.accepted = true;
    }

    CutRect { anchors.fill: parent; cut: 8; fill: root.plane; strokeColor: root.stroke; strokeWidth: Lithic.design === "material" ? 0 : 1 }
    MouseArea { anchors.fill: parent; onPressed: root.forceActiveFocus(Qt.MouseFocusReason) }
    Flickable {
        id: scroll
        anchors.fill: parent
        anchors.margins: LithicTokens.space.s3 * root.s
        contentWidth: width
        contentHeight: grid.implicitHeight
        boundsBehavior: Flickable.StopAtBounds
        clip: true
        Grid {
            id: grid
            width: scroll.width
            columns: root.columns
            columnSpacing: LithicTokens.space.s3 * root.s
            rowSpacing: LithicTokens.space.s3 * root.s
            Repeater {
                id: groupRepeater
                model: root.groups
                delegate: Column {
                    id: group
                    required property var modelData
                    property alias rowRepeater: actionRepeater
                    width: (grid.width - grid.columnSpacing * (root.columns - 1)) / root.columns
                    spacing: LithicTokens.space.s1 * root.s
                    Label { text: group.modelData.label; typeRole: "label"; ink: Lithic.fg; leftPadding: LithicTokens.space.s2 * root.s; bottomPadding: LithicTokens.space.s1 * root.s }
                    Repeater {
                        id: actionRepeater
                        model: group.modelData.rows
                        delegate: Item {
                            id: actionRow
                            required property var modelData
                            required property int index
                            readonly property int flatIndex: group.modelData.offset + index
                            readonly property bool current: root.currentIndex === flatIndex && root.activeFocus
                            width: group.width
                            height: Math.max(LithicTokens.size.menuItem * root.s, actionButton.implicitContentHeight + actionButton.topPadding + actionButton.bottomPadding)
                            T.AbstractButton {
                                id: actionButton
                                width: parent.width - (pin.visible ? pin.width + 4 * root.s : 0)
                                height: parent.height
                                padding: LithicTokens.space.s2 * root.s
                                hoverEnabled: true
                                activeFocusOnTab: false
                                Accessible.role: Accessible.Button
                                Accessible.name: actionRow.modelData.label
                                Accessible.description: actionRow.modelData.hint
                                Accessible.onPressAction: root.choose(actionRow.modelData.id)
                                scale: down ? 0.98 : 1
                                Behavior on scale { NumberAnimation { duration: Lithic.duration(90) } }
                                onClicked: root.choose(actionRow.modelData.id)
                                background: Item {
                                    CutRect {
                                        anchors.fill: parent; anchors.margins: LithicTokens.size.focusRingGap * root.s; cut: LithicTokens.cut.small
                                        fill: Lithic.alpha(Lithic.fg, actionButton.down ? 0.12 : 0.08)
                                        opacity: actionButton.hovered || actionButton.down ? 1 : 0
                                        Behavior on opacity { NumberAnimation { duration: Lithic.duration(120) } }
                                    }
                                    FocusRing { anchors.fill: parent; anchors.margins: LithicTokens.size.focusRingGap * root.s; cut: LithicTokens.cut.small; keyboardFocusVisible: actionRow.current && !root.pinFocused }
                                }
                                contentItem: Item {
                                    implicitHeight: actionCopy.implicitHeight
                                    Glyph { id: glyph; anchors.verticalCenter: actionCopy.verticalCenter; name: actionRow.modelData.icon; size: 18; ink: Lithic.fg2 }
                                    Column {
                                        id: actionCopy
                                        anchors.left: glyph.right; anchors.leftMargin: LithicTokens.space.s2 * root.s
                                        anchors.right: parent.right
                                        anchors.verticalCenter: parent.verticalCenter
                                        spacing: LithicTokens.size.settingsDetailGap * root.s
                                        Label { width: parent.width; text: actionRow.modelData.label; typeRole: "row"; ink: Lithic.fg; elide: Text.ElideRight }
                                        Label { width: parent.width; text: actionRow.modelData.hint; typeRole: "detail"; ink: Lithic.fg2; elide: Text.ElideRight }
                                    }
                                }
                            }
                            T.AbstractButton {
                                id: pin
                                anchors.right: parent.right
                                width: 30 * root.s; height: parent.height
                                visible: root.pinnable(actionRow.modelData)
                                enabled: root.canPin(actionRow.modelData)
                                hoverEnabled: true
                                activeFocusOnTab: false
                                opacity: enabled ? 1 : Lithic.disabledOpacity
                                scale: down ? 0.97 : 1
                                Behavior on scale { NumberAnimation { duration: Lithic.duration(90) } }
                                Accessible.role: Accessible.Button
                                Accessible.name: root.isPinned(actionRow.modelData.id) ? I18n.tr("Unpin %1").arg(actionRow.modelData.label) : I18n.tr("Pin %1").arg(actionRow.modelData.label)
                                Accessible.description: I18n.tr("Pin up to two actions")
                                Accessible.checkable: true
                                Accessible.checked: root.isPinned(actionRow.modelData.id)
                                Accessible.onPressAction: root.togglePin(actionRow.modelData)
                                onClicked: {
                                    root.currentIndex = actionRow.flatIndex;
                                    root.pinFocused = true;
                                    root.forceActiveFocus(Qt.MouseFocusReason);
                                    root.togglePin(actionRow.modelData);
                                }
                                background: Item {
                                    CutRect {
                                        anchors.fill: parent; anchors.margins: LithicTokens.size.focusRingGap * root.s; cut: LithicTokens.cut.small
                                        fill: Lithic.alpha(Lithic.fg, pin.down ? 0.12 : 0.08)
                                        opacity: pin.hovered || pin.down || root.isPinned(actionRow.modelData.id) ? 1 : 0
                                        Behavior on opacity { NumberAnimation { duration: Lithic.duration(120) } }
                                    }
                                    FocusRing { anchors.fill: parent; anchors.margins: LithicTokens.size.focusRingGap * root.s; cut: LithicTokens.cut.small; keyboardFocusVisible: actionRow.current && root.pinFocused }
                                }
                                contentItem: Glyph {
                                    name: "keep"; size: 18
                                    ink: root.isPinned(actionRow.modelData.id) ? Lithic.sel : Lithic.fg2
                                    font.variableAxes: ({ "FILL": root.isPinned(actionRow.modelData.id) ? 1 : 0 })
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    ScrollRail { flickable: scroll; anchors.right: parent.right; anchors.top: parent.top; anchors.bottom: parent.bottom; anchors.margins: 4 * root.s }
}
