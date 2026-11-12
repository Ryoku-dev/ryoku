import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property var model: []
    property var titleOf: function(item) { return item && item.title !== undefined ? String(item.title) : "" }
    property var detailOf: function(item) { return item && item.detail !== undefined ? String(item.detail) : "" }
    property string addText: Lithic.tr("Add")
    property string doneText: Lithic.tr("Add")
    property string emptyText: ""
    property Component editor: null
    property Component trailing: null
    property bool reorderable: false
    property color plane: Lithic.slab
    property string accessibleName: Lithic.tr("List")
    property string previewState: ""

    readonly property int count: _items.length
    readonly property int editingIndex: _editIndex
    readonly property bool editorValid: _editor !== null && !!_editor.valid

    signal added(var item)
    signal edited(int index, var item)
    signal removed(int index)
    signal moved(int from, int to)

    readonly property var _items: previewState === "empty" || !model || model.length === undefined ? [] : model
    property int _editIndex: -1
    property var _editor: null
    property int _dragIndex: -1
    property real _dragOffset: 0
    property var _refocus: null
    readonly property real _gap: LithicTokens.size.settingsRowGap
    readonly property color _liftFill: Lithic.mix(plane, Lithic.fg, LithicTokens.alpha.hoverPlane)

    implicitWidth: LithicTokens.size.draftFormMax
    implicitHeight: list.implicitHeight
    padding: 0

    Accessible.role: Accessible.List
    Accessible.name: accessibleName

    function titleAt(index) {
        return index >= 0 && index < count ? titleOf(_items[index]) : "";
    }

    // count opens the editor for a new item, in place of the add row
    function openEditor(index) {
        if (!editor || index < 0 || index > count)
            return;
        _editIndex = -1;
        _editIndex = index;
    }

    // fromKeys: Escape or Return closed the form; otherwise the control that closed it says
    // whether the keys are in use. Focus goes back marked when they are.
    function cancel(fromKeys) {
        const index = _editIndex;
        if (index < 0)
            return;
        const keyboard = fromKeys === true || _keysInUse();
        _editIndex = -1;
        _focusLater(index, index >= count ? "add" : "edit", false, keyboard);
    }

    function done(fromKeys) {
        if (!editorValid)
            return;
        const index = _editIndex;
        const value = _editor.value();
        const keyboard = fromKeys === true || _keysInUse();
        _editIndex = -1;
        _focusLater(index, index >= count ? "add" : "edit", true, keyboard);
        if (index >= count)
            added(value);
        else
            edited(index, value);
    }

    function remove(index) {
        if (index < 0 || index >= count)
            return;
        const keyboard = _keysInUse();
        if (_editIndex === index)
            _editIndex = -1;
        _focusLater(count > 1 ? Math.min(index, count - 2) : -1, count > 1 ? "remove" : "add", true, keyboard);
        removed(index);
    }

    // the move is reported, never applied: the host reorders its model
    function move(from, to) {
        if (!reorderable || from === to || from < 0 || to < 0 || from >= count || to >= count)
            return;
        moved(from, to);
    }

    function _focusLater(index, part, awaitModel, keyboard) {
        _refocus = { index: index, part: part, awaitModel: awaitModel, keyboard: keyboard };
        Qt.callLater(_restoreFocus);
    }

    function _keysInUse() {
        const item = root.Window.activeFocusItem;
        return !!item && item.visualFocus === true;
    }

    // the target may already hold focus with an unmarked reason (a dialog or popup restored it
    // first), so the reason is set as well
    function _focusWithReason(item, reason) {
        item.forceActiveFocus(reason);
        if (item.focusReason !== undefined)
            item.focusReason = reason;
    }

    function _restoreFocus() {
        const target = _refocus;
        if (!target || target.awaitModel)
            return;
        _refocus = null;
        const reason = target.keyboard ? Qt.TabFocusReason : Qt.OtherFocusReason;
        const entry = target.part === "add" || target.index < 0 ? null : rows.itemAt(target.index);
        if (entry)
            entry.focusPart(target.part, reason);
        else if (addPlaque.visible)
            _focusWithReason(addPlaque, reason);
    }

    function _within(item, ancestor) {
        for (let walk = item; walk; walk = walk.parent) {
            if (walk === ancestor)
                return true;
        }
        return false;
    }

    function _focusEditor() {
        const editorItem = _editor;
        if (!editorItem || previewState.length > 0)
            return;
        editorItem.forceActiveFocus(Qt.TabFocusReason);
        if (root.Window.activeFocusItem === editorItem) {
            const next = editorItem.nextItemInFocusChain(true);
            if (next && _within(next, editorItem))
                next.forceActiveFocus(Qt.TabFocusReason);
        }
    }

    function _dropIndex(from, centre) {
        let to = from;
        for (let index = 0; index < count; ++index) {
            const other = rows.itemAt(index);
            if (index === from || !other)
                continue;
            if (index < from && centre < other.boxCentre)
                to = Math.min(to, index);
            else if (index > from && centre > other.boxCentre)
                to = Math.max(to, index);
        }
        return to;
    }

    function _applyPreview() {
        if (previewState === "editing" && count > 0)
            openEditor(0);
        else if (previewState === "adding" || previewState === "editing")
            openEditor(count);
        if (previewState === "dragging" && count > 0) {
            _dragIndex = 0;
            _dragOffset = LithicTokens.space.s4;
        }
    }

    // a new model rebuilds every row, so an open editor closes with it
    on_ItemsChanged: {
        _editIndex = -1;
        _dragIndex = -1;
        if (_refocus) {
            _refocus.awaitModel = false;
            Qt.callLater(_restoreFocus);
        }
    }
    onPreviewStateChanged: Qt.callLater(_applyPreview)
    Component.onCompleted: if (previewState.length > 0) Qt.callLater(_applyPreview)

    Component {
        id: editorPanel

        Item {
            id: panel

            readonly property int editIndex: root._editIndex
            readonly property bool adding: editIndex >= root.count
            readonly property real topGap: parent && parent.gapAbove === true ? root._gap : 0
            readonly property real planeTop: topGap + LithicTokens.space.s2
            readonly property real pad: LithicTokens.space.s3
            // the plane reaches s2 past the row's sides, so its padding leaves the content s1 in
            readonly property real contentInset: pad - LithicTokens.space.s2
            property var editorObject: null

            width: parent ? parent.width : 0
            implicitHeight: planeTop + editorPlane.height
            Accessible.role: Accessible.Grouping
            Accessible.name: adding ? root.addText : Lithic.tr("Edit %1").arg(root.titleAt(editIndex))

            Keys.onEscapePressed: function(event) {
                root.cancel(true);
                event.accepted = true;
            }

            Rectangle {
                visible: panel.topGap > 0
                y: Math.floor((panel.topGap - LithicTokens.size.hairline) / 2)
                width: panel.width
                height: LithicTokens.size.hairline
                color: Lithic.vein
            }

            CutRect {
                id: editorPlane
                x: -LithicTokens.space.s2
                y: panel.planeTop
                width: panel.width + LithicTokens.space.s2 * 2
                height: panel.pad * 3 + editorHost.height + LithicTokens.size.plaqueHeightSmall
                fill: Lithic.field
                cut: LithicTokens.cut.small
            }

            // Enter left unhandled by a single-line field means Done
            Item {
                id: editorHost
                // the form lies on the field plane; a slider in it reads this to keep its range visible
                readonly property color plane: Lithic.field
                x: panel.contentInset
                y: panel.planeTop + panel.pad
                width: panel.width - panel.contentInset * 2
                height: panel.editorObject ? panel.editorObject.implicitHeight : 0

                Keys.onPressed: function(event) {
                    if (event.key !== Qt.Key_Return && event.key !== Qt.Key_Enter)
                        return;
                    if (event.modifiers & (Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier | Qt.ShiftModifier))
                        return;
                    root.done(true);
                    event.accepted = true;
                }
            }

            Row {
                anchors.right: editorHost.right
                y: editorHost.y + editorHost.height + panel.pad
                spacing: LithicTokens.space.s2

                Plaque {
                    small: true
                    kind: "quiet"
                    text: Lithic.tr("Cancel")
                    onClicked: root.cancel()
                }
                Plaque {
                    small: true
                    kind: "line"
                    text: panel.adding ? root.doneText : Lithic.tr("Done")
                    enabled: root.editorValid
                    onClicked: root.done()
                }
            }

            Component.onCompleted: {
                const index = root._editIndex;
                const source = index >= 0 && index < root.count ? root._items[index] : null;
                editorObject = root.editor.createObject(editorHost, { item: source ? Object.assign({}, source) : {} });
                if (!editorObject)
                    return;
                editorObject.width = Qt.binding(function() { return editorHost.width; });
                root._editor = editorObject;
                Qt.callLater(root._focusEditor);
            }
            Component.onDestruction: {
                if (root._editor === editorObject)
                    root._editor = null;
            }
        }
    }

    contentItem: Column {
        id: list
        spacing: 0

        Label {
            id: emptyLine
            width: parent.width
            visible: root.count === 0 && root.emptyText.length > 0
            text: root.emptyText
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Repeater {
            id: rows
            model: root._items

            delegate: Column {
                id: entry

                required property var modelData
                required property int index
                readonly property real gap: index > 0 ? root._gap : 0
                readonly property real boxHeight: Math.max(LithicTokens.size.factsRowHeight,
                                                           copy.implicitHeight, tail.implicitHeight)
                readonly property real boxCentre: y + gap + boxHeight / 2
                readonly property bool dragging: root._dragIndex === index
                readonly property string title: root.titleOf(modelData)

                width: list.width
                z: dragging ? 1 : 0

                function focusedPart() {
                    return handle.activeFocus ? "handle" : removePlaque.activeFocus ? "remove" : "edit";
                }

                function focusPart(part, reason) {
                    const target = part === "handle" && handle.visible ? handle
                                 : part === "remove" ? removePlaque : editPlaque;
                    root._focusWithReason(target, reason);
                }

                Item {
                    id: line
                    width: parent.width
                    height: entry.gap + entry.boxHeight
                    Accessible.role: Accessible.ListItem
                    Accessible.name: entry.title

                    Keys.onPressed: function(event) {
                        if (!root.reorderable || !(event.modifiers & Qt.AltModifier))
                            return;
                        let to = -1;
                        if (event.key === Qt.Key_Up)
                            to = entry.index - 1;
                        else if (event.key === Qt.Key_Down)
                            to = entry.index + 1;
                        else
                            return;
                        event.accepted = true;
                        if (to < 0 || to >= root.count)
                            return;
                        root._focusLater(to, entry.focusedPart(), true, true);
                        root.move(entry.index, to);
                    }

                    Rectangle {
                        visible: entry.index > 0
                        y: Math.floor((entry.gap - LithicTokens.size.hairline) / 2)
                        width: line.width
                        height: LithicTokens.size.hairline
                        color: Lithic.vein
                    }

                    Item {
                        id: lift
                        y: entry.gap
                        width: line.width
                        height: entry.boxHeight
                        transform: Translate { y: entry.dragging ? root._dragOffset : 0 }

                        CutRect {
                            visible: entry.dragging
                            x: -LithicTokens.space.s2
                            y: -LithicTokens.space.s1
                            width: lift.width + LithicTokens.space.s2 * 2
                            height: lift.height + LithicTokens.space.s1 * 2
                            fill: root._liftFill
                            cut: LithicTokens.cut.small
                        }

                        T.AbstractButton {
                            id: handle
                            visible: root.reorderable
                            anchors.verticalCenter: parent.verticalCenter
                            width: LithicTokens.size.iconPlaqueIcon
                            height: LithicTokens.size.iconPlaqueSmall
                            focusPolicy: Qt.StrongFocus
                            Accessible.role: Accessible.Button
                            Accessible.name: Lithic.tr("Move %1").arg(entry.title)
                            Accessible.description: Lithic.tr("Alt+Up and Alt+Down move it")

                            background: FocusRing {
                                x: -outset
                                y: -outset
                                width: handle.width + outset * 2
                                height: handle.height + outset * 2
                                keyboardFocusVisible: handle.visualFocus
                                cut: LithicTokens.cut.small
                            }

                            contentItem: Glyph {
                                name: "drag_indicator"
                                size: LithicTokens.size.iconPlaqueIcon
                                ink: Lithic.fg3
                            }

                            // scene positions, because the row moves under the pointer
                            DragHandler {
                                target: null
                                xAxis.enabled: false
                                cursorShape: Qt.ClosedHandCursor
                                onActiveChanged: {
                                    if (active) {
                                        root._dragIndex = entry.index;
                                        root._dragOffset = 0;
                                    } else if (root._dragIndex === entry.index) {
                                        const to = root._dropIndex(entry.index, entry.boxCentre + root._dragOffset);
                                        root._dragIndex = -1;
                                        root._dragOffset = 0;
                                        root.move(entry.index, to);
                                    }
                                }
                                onCentroidChanged: {
                                    if (active)
                                        root._dragOffset = centroid.scenePosition.y - centroid.scenePressPosition.y;
                                }
                            }
                        }

                        Column {
                            id: copy
                            x: handle.visible ? handle.width + LithicTokens.space.s3 : 0
                            anchors.verticalCenter: parent.verticalCenter
                            width: Math.max(0, tail.x - x - LithicTokens.space.s4)
                            spacing: LithicTokens.size.settingsDetailGap

                            Label {
                                width: parent.width
                                text: entry.title
                                typeRole: "row"
                                wrapMode: Text.Wrap
                                elide: Text.ElideNone
                            }
                            Label {
                                width: parent.width
                                visible: text.length > 0
                                text: root.detailOf(entry.modelData)
                                typeRole: "detail"
                                wrapMode: Text.Wrap
                                elide: Text.ElideNone
                            }
                        }

                        Row {
                            id: tail
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: LithicTokens.space.s2

                            Loader {
                                anchors.verticalCenter: parent.verticalCenter
                                active: root.trailing !== null
                                sourceComponent: root.trailing
                                onLoaded: {
                                    if ("modelData" in item)
                                        item.modelData = Qt.binding(function() { return entry.modelData; });
                                    if ("index" in item)
                                        item.index = Qt.binding(function() { return entry.index; });
                                }
                            }

                            IconPlaque {
                                id: editPlaque
                                anchors.verticalCenter: parent.verticalCenter
                                small: true
                                kind: "quiet"
                                glyph: "edit"
                                enabled: root.editor !== null
                                accessibleName: Lithic.tr("Edit %1").arg(entry.title)
                                onClicked: root.openEditor(entry.index)
                            }
                            IconPlaque {
                                id: removePlaque
                                anchors.verticalCenter: parent.verticalCenter
                                small: true
                                kind: "quiet"
                                glyph: "delete"
                                accessibleName: Lithic.tr("Remove %1").arg(entry.title)
                                onClicked: root.remove(entry.index)
                            }
                        }
                    }
                }

                // an unloaded Loader keeps its last size, so its height follows the panel explicitly
                Loader {
                    width: parent.width
                    height: active && item ? item.implicitHeight : 0
                    visible: active
                    active: root._editIndex === entry.index
                    sourceComponent: editorPanel
                }
            }
        }

        Item {
            id: addRow

            readonly property real topGap: root.count > 0 ? root._gap
                                           : emptyLine.visible ? LithicTokens.space.s2 : 0

            width: parent.width
            height: topGap + LithicTokens.size.factsRowHeight
            visible: root._editIndex !== root.count

            Rectangle {
                visible: root.count > 0
                y: Math.floor((addRow.topGap - LithicTokens.size.hairline) / 2)
                width: addRow.width
                height: LithicTokens.size.hairline
                color: Lithic.vein
            }

            // the glyph lines up with the row titles
            Plaque {
                id: addPlaque
                x: -sideInset
                y: addRow.topGap + (LithicTokens.size.factsRowHeight - height) / 2
                kind: "quiet"
                glyph: "add"
                text: root.addText
                enabled: root.editor !== null
                onClicked: root.openEditor(root.count)
            }
        }

        Loader {
            property bool gapAbove: root.count > 0
            width: parent.width
            height: active && item ? item.implicitHeight : 0
            visible: active
            active: root._editIndex >= 0 && root._editIndex === root.count
            sourceComponent: editorPanel
        }
    }
}
