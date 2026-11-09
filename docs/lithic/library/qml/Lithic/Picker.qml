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
                       ? Lithic.type.caption.lineHeight + LithicTokens.space.s1 : 0)
    activeFocusOnTab: true
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
            result.push(typeof option === "string" ? { label: option } : option)
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
            typeRole: "caption"
            inkRole: "fg3"
        }

        CutRect {
            id: field
            x: 0
            y: root.label.length > 0
               ? Lithic.type.caption.lineHeight + LithicTokens.space.s1 : 0
            width: parent.width
            height: LithicTokens.size.pickerHeight
            fill: hover.hovered || root.previewState === "hover"
                  ? Lithic.hoverPlane(Lithic.stone3, 1) : Lithic.stone3
            cut: LithicTokens.cut.mid
            ChiselColorBehavior on fill {
                duration: hover.hovered || root.previewState === "hover"
                          ? LithicTokens.motion.hoverIn
                          : LithicTokens.motion.hoverOut
            }

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
            root.forceActiveFocus()
            root.toggle()
        }
    }
    Keys.onSpacePressed: toggle()
    Keys.onReturnPressed: toggle()
    Keys.onDownPressed: open()
    Keys.onEscapePressed: close()

    Popover {
        id: popover
        parent: root.overlayParent ? root.overlayParent : root.parent
        anchorItem: field
        opener: root
        previewEmbedded: root.previewOpen

        Menu {
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
