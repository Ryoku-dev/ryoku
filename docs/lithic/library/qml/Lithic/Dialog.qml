import QtQuick
import QtQuick.Controls as C
import QtQuick.Templates as T

T.Dialog {
    id: root

    default property alias bodyData: bodyHost.data
    property real dialogWidth: LithicTokens.size.dialogWidth
    property var actions: []
    property Item opener: null
    property string previewState: ""
    property bool previewEmbedded: false
    property real slideOffset: 0

    signal actionTriggered(int index)

    component DialogPlane: CutRect {
        fill: Lithic.stone4
        strokeColor: Lithic.vein
        strokeWidth: LithicTokens.size.hairline
        cut: LithicTokens.cut.large
    }

    modal: !previewEmbedded
    dim: !previewEmbedded
    focus: true
    padding: LithicTokens.size.dialogPadding
    closePolicy: T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside
    width: parent
           ? Math.min(dialogWidth, Math.max(0, parent.width - LithicTokens.size.dialogMargin * 2))
           : dialogWidth
    height: implicitHeight
    x: parent ? (parent.width - width) / 2 : 0
    y: parent ? (parent.height - height) / 2 : 0
    implicitHeight: contentColumn.implicitHeight + topPadding + bottomPadding

    C.Overlay.modal: Rectangle { color: Lithic.scrim }

    onClosed: {
        if (opener && opener.enabled)
            opener.forceActiveFocus()
    }
    // like the web, a dialog that opens without focusing a control of its own starts on its first one
    onOpened: {
        if (!previewEmbedded)
            _focusFirstControl()
    }

    function _focusFirstControl() {
        for (let item = contentColumn.Window.activeFocusItem; item; item = item.parent) {
            if (item === contentColumn)
                return
        }
        const first = _firstTabStop(contentColumn)
        if (first)
            first.forceActiveFocus(opener && opener.visualFocus ? Qt.TabFocusReason : Qt.OtherFocusReason)
    }

    function _firstTabStop(item) {
        for (let index = 0; index < item.children.length; ++index) {
            const child = item.children[index]
            if (!child.visible || !child.enabled)
                continue
            if (child.activeFocusOnTab)
                return child
            const inner = _firstTabStop(child)
            if (inner)
                return inner
        }
        return null
    }

    background: DialogPlane {
        transform: Translate { y: root.slideOffset }
    }

    contentItem: Column {
        id: contentColumn
        spacing: 0
        transform: Translate { y: root.slideOffset }

        Label {
            visible: root.title.length > 0
            width: parent.width
            text: root.title
            typeRole: "sentence"
            inkRole: "fg"
            wrapMode: Text.Wrap
            maximumLineCount: 2
            elide: Text.ElideRight
        }

        Item {
            visible: root.title.length > 0
            width: parent.width
            height: visible ? LithicTokens.size.dialogTitleGap : 0
        }

        Item {
            id: bodyHost
            width: parent.width
            implicitWidth: children.length > 0 ? children[0].implicitWidth : 0
            implicitHeight: children.length > 0 ? children[0].implicitHeight : 0
        }

        Item {
            visible: root.actions.length > 0
            width: parent.width
            height: visible ? LithicTokens.size.dialogActionsGap : 0
        }

        Row {
            id: actionRow
            visible: root.actions.length > 0
            width: parent.width
            spacing: LithicTokens.size.dialogActionGap

            Repeater {
                model: root.actions
                delegate: Plaque {
                    required property var modelData
                    required property int index
                    text: modelData.text || ""
                    glyph: modelData.glyph || ""
                    kind: modelData.kind || "quiet"
                    small: !!modelData.small
                    busy: !!modelData.busy
                    enabled: modelData.enabled === undefined || !!modelData.enabled
                    previewState: root.previewState
                    onClicked: {
                        root.actionTriggered(index)
                        if (modelData.closeOnTrigger === undefined || modelData.closeOnTrigger)
                            root.close()
                    }
                }
            }
        }
    }

    DialogPlane {
        id: previewPlane
        parent: root.parent
        visible: root.previewEmbedded
        x: root.x
        y: root.y
        width: root.width
        height: root.implicitHeight
        z: 100
    }

    StateGroup {
        states: State {
        name: "preview"
        when: root.previewEmbedded
        ParentChange {
            target: contentColumn
            parent: previewPlane
            x: root.leftPadding
            y: root.topPadding
            width: previewPlane.width - root.leftPadding - root.rightPadding
        }
        PropertyChanges {
            target: contentColumn
            visible: true
            enabled: true
        }
    }
        }

    enter: Transition {
        ParallelAnimation {
            NumberAnimation {
                target: root
                property: "opacity"
                from: 0
                to: 1
                duration: Lithic.duration(LithicTokens.motion.dialogIn)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Lithic.curve
            }
            NumberAnimation {
                target: root
                property: "slideOffset"
                from: LithicTokens.size.dialogEnterTravel
                to: 0
                duration: Lithic.duration(LithicTokens.motion.dialogIn)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Lithic.curve
            }
        }
    }

    exit: Transition {
        ParallelAnimation {
            NumberAnimation {
                target: root
                property: "opacity"
                from: 1
                to: 0
                duration: Lithic.duration(LithicTokens.motion.dialogOut)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Lithic.curve
            }
            NumberAnimation {
                target: root
                property: "slideOffset"
                from: 0
                to: LithicTokens.size.dialogExitTravel
                duration: Lithic.duration(LithicTokens.motion.dialogOut)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Lithic.curve
            }
        }
    }
}
