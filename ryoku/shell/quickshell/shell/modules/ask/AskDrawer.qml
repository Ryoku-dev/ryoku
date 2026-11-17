pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services

// A sectioned dropdown for the Ask bar's header buttons (history, model).
// The drawer never takes focus: the field keeps the keyboard, and the surface
// routes Up/Down/Enter here while a drawer is open, so the pointer and the
// keyboard drive the same selection. Rows are plain JS objects:
//   { section: "Asks" }                    a header
//   { label, sub, active, value, ... }     a selectable row
//   { ..., renamable: true }               shows a pencil that edits the label
// The drawer echoes the picked row back through `activated`, and a finished
// rename through `renamed`; renaming is the one time the drawer takes focus.
Rectangle {
    id: drawer

    required property real s
    property var rows: []
    property bool open: false
    property string emptyText: I18n.tr("Nothing here yet")
    property real maxHeight: 260 * s
    readonly property bool motionAllowed: !Tokens.reduceMotion && !Motion.reduce
    property int current: 0
    // The value of the row whose label is being edited, "" when none.
    property string editingValue: ""
    readonly property var selectable: rows.filter(r => !r.section && r.disabled !== true)
    // The natural (unclipped) height, so the owner can reserve room for it.
    readonly property real naturalHeight: contentColumn.implicitHeight + Tokens.s3 * s * 2

    signal activated(var row)
    signal renamed(var row, string title)
    signal editFinished()

    width: parent.width
    height: open ? Math.min(maxHeight, contentColumn.implicitHeight + Tokens.s3 * s * 2) : 0
    visible: height > 0
    opacity: open ? 1 : 0
    scale: open ? 1 : 0.98
    transformOrigin: Item.Top
    radius: Tokens.radius * s
    color: Tokens.paperLift
    border.width: Tokens.border
    border.color: Tokens.lineStrong
    clip: true
    z: 20

    Behavior on height {
        enabled: drawer.motionAllowed
        NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease }
    }
    Behavior on opacity {
        enabled: drawer.motionAllowed
        NumberAnimation { duration: Tokens.swap }
    }
    Behavior on scale {
        enabled: drawer.motionAllowed
        NumberAnimation { duration: Tokens.move; easing.type: Easing.OutBack }
    }

    onOpenChanged: {
        if (open)
            selectFirstActive();
        else
            cancelRename();
    }

    function startRename(row) {
        editingValue = String(row.value || "");
    }

    function finishRename(row, title) {
        editingValue = "";
        const name = String(title || "").trim();
        if (name.length > 0 && name !== String(row.label || ""))
            renamed(row, name);
        editFinished();
    }

    function cancelRename() {
        if (editingValue.length === 0)
            return;
        editingValue = "";
        editFinished();
    }
    onSelectableChanged: current = Math.max(0, Math.min(current, selectable.length - 1))

    function selectFirstActive() {
        for (let i = 0; i < selectable.length; i++)
            if (selectable[i].active) {
                current = i;
                return;
            }
        current = 0;
    }

    function move(delta) {
        if (!open || selectable.length === 0)
            return;
        current = Math.max(0, Math.min(selectable.length - 1, current + delta));
    }

    function activate() {
        if (!open || selectable.length === 0)
            return;
        activated(selectable[Math.max(0, Math.min(current, selectable.length - 1))]);
    }

    Flickable {
        id: scroll
        anchors.fill: parent
        anchors.margins: Tokens.s3 * drawer.s
        contentHeight: contentColumn.implicitHeight
        boundsBehavior: Flickable.StopAtBounds
        interactive: contentHeight > height
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded; visible: scroll.interactive; motionEnabled: drawer.motionAllowed }

        Column {
            id: contentColumn
            width: parent.width
            spacing: 0

            Repeater {
                model: drawer.rows
                delegate: Item {
                    id: row
                    required property var modelData
                    required property int index
                    readonly property bool isSection: Boolean(modelData.section)
                    readonly property bool isDisabled: modelData.disabled === true
                    // Its position among selectable rows, for selection state.
                    readonly property int ordinal: {
                        if (isSection || isDisabled)
                            return -1;
                        let n = 0;
                        for (let i = 0; i < index; i++)
                            if (!drawer.rows[i].section && drawer.rows[i].disabled !== true)
                                n++;
                        return n;
                    }
                    readonly property bool selected: !isSection && !isDisabled && ordinal === drawer.current
                    readonly property bool renamable: !isSection && !isDisabled && modelData.renamable === true
                    readonly property bool editing: renamable && drawer.editingValue.length > 0
                        && String(modelData.value || "") === drawer.editingValue

                    width: contentColumn.width
                    height: isSection ? sectionColumn.implicitHeight : 44 * drawer.s

                    Column {
                        id: sectionColumn
                        visible: row.isSection
                        width: parent.width
                        spacing: Tokens.s1 * drawer.s
                        Item { width: 1; height: row.index === 0 ? 0 : Tokens.s2 * drawer.s }
                        Text {
                            text: String(row.modelData.section)
                            color: Tokens.inkMuted
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fMicro * drawer.s
                            font.letterSpacing: Tokens.trackLabel
                        }
                        Item { width: 1; height: Tokens.s1 * drawer.s }
                    }

                    Rectangle {
                        visible: !row.isSection
                        anchors.fill: parent
                        radius: Tokens.radius * drawer.s
                        color: row.isDisabled ? Tokens.tint5
                            : rowTap.pressed ? Tokens.tint16
                            : row.selected ? Tokens.bone
                            : rowHover.hovered ? Tokens.tint10 : "transparent"
                        scale: rowTap.pressed && drawer.motionAllowed ? 0.985 : 1
                        Behavior on scale {
                            enabled: drawer.motionAllowed
                            NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
                        }
                        Row {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.leftMargin: Tokens.s3 * drawer.s
                            anchors.rightMargin: Tokens.s3 * drawer.s
                            spacing: Tokens.s2 * drawer.s

                            Column {
                                width: parent.width - check.width - parent.spacing
                                    - (editBtn.visible ? editBtn.width + parent.spacing : 0)
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s1 * drawer.s / 2
                                Text {
                                    visible: !row.editing
                                    width: parent.width
                                    text: String(row.modelData.label || "")
                                    color: row.isDisabled ? Tokens.inkMuted
                                        : row.selected ? Tokens.inkOnBone : Tokens.ink
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fSmall * drawer.s
                                    font.weight: row.modelData.active ? Font.DemiBold : Font.Normal
                                    elide: Text.ElideRight
                                }
                                TextInput {
                                    id: renameInput
                                    visible: row.editing
                                    width: parent.width
                                    color: row.selected ? Tokens.inkOnBone : Tokens.ink
                                    selectionColor: Tokens.tint16
                                    selectedTextColor: color
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fSmall * drawer.s
                                    font.weight: Font.DemiBold
                                    clip: true
                                    onVisibleChanged: {
                                        if (!visible)
                                            return;
                                        text = String(row.modelData.label || "");
                                        selectAll();
                                        forceActiveFocus();
                                    }
                                    onAccepted: drawer.finishRename(row.modelData, text)
                                    onActiveFocusChanged: if (!activeFocus && row.editing) drawer.cancelRename()
                                    Keys.onEscapePressed: event => {
                                        event.accepted = true;
                                        drawer.cancelRename();
                                    }
                                }
                                Text {
                                    visible: String(row.modelData.sub || "").length > 0
                                    width: parent.width
                                    text: String(row.modelData.sub || "")
                                    color: row.isDisabled ? Tokens.inkFaint
                                        : row.selected ? Tokens.inkOnBone : Tokens.inkMuted
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fTiny * drawer.s
                                    elide: Text.ElideRight
                                }
                            }
                            Text {
                                id: editBtn
                                anchors.verticalCenter: parent.verticalCenter
                                visible: row.renamable && !row.editing
                                opacity: row.selected || rowHover.hovered ? 1 : 0
                                text: "edit"
                                color: row.selected ? Tokens.inkOnBone : Tokens.inkDim
                                font.family: "Material Symbols Rounded"
                                font.pixelSize: Tokens.fBody * drawer.s
                                Accessible.role: Accessible.Button
                                Accessible.name: I18n.tr("Rename")
                            }
                            Text {
                                id: check
                                anchors.verticalCenter: parent.verticalCenter
                                visible: Boolean(row.modelData.active)
                                text: "check"
                                color: row.selected ? Tokens.inkOnBone : Tokens.inkDim
                                font.family: "Material Symbols Rounded"
                                font.pixelSize: Tokens.fBody * drawer.s
                            }
                        }
                        HoverHandler {
                            enabled: !row.isDisabled
                            id: rowHover
                            cursorShape: Qt.PointingHandCursor
                            onHoveredChanged: if (hovered) drawer.current = row.ordinal
                        }
                        TapHandler {
                            enabled: !row.isDisabled && !row.editing
                            id: rowTap
                            onTapped: eventPoint => {
                                drawer.current = row.ordinal;
                                const p = editBtn.mapFromItem(rowTap.parent, eventPoint.position);
                                if (editBtn.visible && editBtn.contains(p))
                                    drawer.startRename(row.modelData);
                                else
                                    drawer.activated(row.modelData);
                            }
                        }
                    }
                }
            }

            Text {
                visible: drawer.rows.filter(r => !r.section).length === 0
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                topPadding: Tokens.s4 * drawer.s
                bottomPadding: Tokens.s4 * drawer.s
                text: drawer.emptyText
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall * drawer.s
            }
        }
    }
}
