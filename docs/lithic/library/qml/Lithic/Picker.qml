import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property string label: ""
    property var options: []
    property int currentIndex: -1
    property Item overlayParent: null
    property string previewState: ""
    property bool previewOpen: false
    readonly property bool opened: previewOpen || popover.opened
    readonly property string value: optionLabel(currentIndex)

    signal activated(int index)

    implicitWidth: Math.max(LithicTokens.size.pickerWidthMin, contentItem.implicitWidth)
    implicitHeight: LithicTokens.size.pickerHeight
                    + (label.length > 0
                       ? Lithic.type.detail.lineHeight + LithicTokens.space.s1 : 0)
    activeFocusOnTab: true
    opacity: enabled ? 1 : Lithic.disabledOpacity
    Accessible.role: Accessible.ComboBox
    Accessible.name: label
    Accessible.description: value

    function optionLabel(index) {
        if (index < 0 || index >= options.length)
            return ""
        const option = options[index]
        return typeof option === "string" ? option : option.label || ""
    }

    function menuItems() {
        const result = []
        for (let index = 0; index < options.length; ++index) {
            const option = options[index]
            const entry = typeof option === "string" ? { label: option } : option
            result.push(Object.assign({}, entry, { checked: index === currentIndex }))
        }
        return result
    }

    function open() {
        if (enabled)
            popover.open()
    }

    function close() {
        popover.close()
    }

    function toggle() {
        if (popover.opened)
            close()
        else
            open()
    }

    background: null

    contentItem: Item {
        implicitWidth: LithicTokens.size.pickerWidthMin
        implicitHeight: root.implicitHeight

        Label {
            visible: root.label.length > 0
            x: 0
            y: 0
            text: root.label
            typeRole: "detail"
        }

        FieldSurface {
            id: field
            x: 0
            y: root.label.length > 0
               ? Lithic.type.detail.lineHeight + LithicTokens.space.s1 : 0
            width: parent.width
            height: LithicTokens.size.pickerHeight
            hovered: hover.hovered || root.previewState === "hover"

            FocusRing {
                x: -outset
                y: -outset
                width: parent.width + outset * 2
                height: parent.height + outset * 2
                keyboardFocusVisible: root.visualFocus || root.previewState === "focus"
                cut: LithicTokens.cut.mid
            }

            Label {
                anchors.left: parent.left
                anchors.leftMargin: LithicTokens.size.pickerInset
                anchors.right: arrow.left
                anchors.rightMargin: LithicTokens.size.pickerArrowGap
                anchors.verticalCenter: parent.verticalCenter
                text: root.value
                typeRole: "body"
                inkRole: "fg"
                elide: Text.ElideRight
            }

            Glyph {
                id: arrow
                anchors.right: parent.right
                anchors.rightMargin: LithicTokens.size.pickerInset
                anchors.verticalCenter: parent.verticalCenter
                name: popover.opened ? "expand_less" : "expand_more"
                size: LithicTokens.size.pickerArrow
                ink: Lithic.fg2
            }
        }
    }

    HoverHandler { id: hover; enabled: root.enabled }
    TapHandler {
        enabled: root.enabled
        onTapped: {
            root._openedFromKeys = false
            root.forceActiveFocus()
            root.toggle()
        }
    }
    property bool _openedFromKeys: false
    function _toggleFromKeys() {
        _openedFromKeys = true
        toggle()
    }
    Keys.onSpacePressed: _toggleFromKeys()
    Keys.onReturnPressed: _toggleFromKeys()
    Keys.onDownPressed: {
        _openedFromKeys = true
        open()
    }
    // a closed picker lets Escape go on, so the dialog or page holding it can close
    Keys.onEscapePressed: event => {
        if (popover.opened)
            close()
        else
            event.accepted = false
    }

    Popover {
        id: popover
        parent: root.overlayParent ? root.overlayParent : root.parent
        anchorItem: field
        opener: root
        previewEmbedded: root.previewOpen
        // the picker, not the popover, hands focus to the menu: on the current choice,
        // with the keyboard mark only when the keys opened it
        menuFocus: false
        onOpened: menu.focusItem(root.currentIndex, root._openedFromKeys)

        Menu {
            id: menu
            width: Math.max(root.width, LithicTokens.size.menuWidthMin)
            items: root.menuItems()
            onTriggered: index => {
                root.currentIndex = index
                root.activated(index)
                popover.close()
            }
        }
    }

}
