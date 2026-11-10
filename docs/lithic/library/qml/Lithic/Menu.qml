import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property var items: []
    property real menuWidth: LithicTokens.size.menuWidthMin
    property string previewState: ""
    property int previewIndex: -1
    property int focusedIndex: -1

    signal triggered(int index)

    implicitWidth: Math.max(LithicTokens.size.menuWidthMin,
                            Math.min(LithicTokens.size.menuWidthMax, menuWidth))
    implicitHeight: list.implicitHeight + topPadding + bottomPadding
    topPadding: LithicTokens.size.menuPaddingVertical
    bottomPadding: LithicTokens.size.menuPaddingVertical
    leftPadding: 0
    rightPadding: 0
    activeFocusOnTab: true
    Accessible.role: Accessible.PopupMenu

    function itemEnabled(index) {
        const entry = items[index]
        return entry && !entry.divider && !entry.disabled
    }

    function moveFocus(step) {
        if (items.length === 0)
            return
        let next = focusedIndex
        for (let attempt = 0; attempt < items.length; ++attempt) {
            next = (next + step + items.length) % items.length
            if (itemEnabled(next)) {
                focusedIndex = next
                const item = itemRepeater.itemAt(next)
                if (item)
                    item.forceActiveFocus()
                return
            }
        }
    }

    function activateFocused() {
        if (itemEnabled(focusedIndex))
            triggered(focusedIndex)
    }

    Keys.onDownPressed: moveFocus(1)
    Keys.onUpPressed: moveFocus(-1)
    Keys.onReturnPressed: activateFocused()
    Keys.onSpacePressed: activateFocused()
    onActiveFocusChanged: {
        if (activeFocus && focusedIndex < 0)
            moveFocus(1)
    }

    background: CutRect {
        fill: Lithic.stone4
        strokeColor: Lithic.vein
        strokeWidth: LithicTokens.size.hairline
        cut: LithicTokens.cut.menu
    }

    contentItem: Column {
        id: list
        spacing: 0

        Repeater {
            id: itemRepeater
            model: root.items

            delegate: T.MenuItem {
                id: menuItem

                required property var modelData
                required property int index
                readonly property bool divider: !!modelData.divider
                readonly property bool danger: !!modelData.danger
                readonly property string glyphName: modelData.glyph || ""
                readonly property string shortcutText: modelData.shortcut || ""
                readonly property bool forcedHover: root.previewIndex === index
                                                      && root.previewState === "hover"
                readonly property bool forcedPress: root.previewIndex === index
                                                      && root.previewState === "pressed"

                width: list.width
                height: divider ? LithicTokens.size.hairline : LithicTokens.size.menuItem
                text: divider ? "" : (modelData.label || "")
                enabled: !divider && !modelData.disabled
                hoverEnabled: true
                activeFocusOnTab: enabled
                Accessible.name: text

                onTriggered: root.triggered(index)
                onActiveFocusChanged: {
                    if (activeFocus)
                        root.focusedIndex = index
                }

                background: Item {
                    visible: !menuItem.divider
                    CutRect {
                        anchors.fill: parent
                        fill: menuItem.down || menuItem.forcedPress
                              ? Lithic.pressRow(Lithic.stone4, 1)
                              : menuItem.hovered || menuItem.forcedHover
                                ? Lithic.hoverPlane(Lithic.stone4, 1) : "transparent"
                        cut: 0
                        ChiselColorBehavior on fill {
                            duration: menuItem.hovered || menuItem.forcedHover
                                      ? LithicTokens.motion.hoverIn
                                      : LithicTokens.motion.hoverOut
                        }
                    }
                    FocusRing {
                        x: -outset
                        y: -outset
                        width: parent.width + outset * 2
                        height: parent.height + outset * 2
                        keyboardFocusVisible: menuItem.visualFocus
                        cut: 0
                    }
                }

                contentItem: Item {
                    visible: !menuItem.divider

                    Glyph {
                        id: itemGlyph
                        visible: menuItem.glyphName.length > 0
                        x: LithicTokens.size.menuInset
                        anchors.verticalCenter: parent.verticalCenter
                        name: menuItem.glyphName
                        size: LithicTokens.size.menuIcon
                        ink: menuItem.enabled
                             ? menuItem.danger ? Lithic.jasper : Lithic.fg2
                             : Lithic.fg3
                    }

                    Label {
                        anchors.left: parent.left
                        anchors.leftMargin: LithicTokens.size.menuInset
                                            + (itemGlyph.visible
                                               ? LithicTokens.size.menuIcon
                                                 + LithicTokens.size.menuIconGap : 0)
                        anchors.right: shortcut.left
                        anchors.rightMargin: menuItem.shortcutText.length > 0
                                             ? LithicTokens.size.menuShortcutGap : 0
                        anchors.verticalCenter: parent.verticalCenter
                        text: menuItem.text
                        typeRole: "label"
                        ink: menuItem.enabled
                             ? menuItem.danger ? Lithic.jasper : Lithic.fg
                             : Lithic.fg3
                        elide: Text.ElideRight
                    }

                    Label {
                        id: shortcut
                        anchors.right: parent.right
                        anchors.rightMargin: LithicTokens.size.menuInset
                        anchors.verticalCenter: parent.verticalCenter
                        text: menuItem.shortcutText
                        typeRole: "monoSm"
                        inkRole: "fg3"
                    }
                }

                CutRect {
                    visible: menuItem.divider
                    x: LithicTokens.size.menuDividerInset
                    width: parent.width - LithicTokens.size.menuDividerInset * 2
                    height: LithicTokens.size.hairline
                    fill: Lithic.vein
                    cut: 0
                }
            }
        }
    }
}
