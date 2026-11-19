import QtQuick
import QtQuick.Templates as T
import QtQuick.Effects

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
    readonly property real _itemsHeight: items.reduce((height, entry) =>
        height + (entry.divider ? LithicTokens.size.hairline : LithicTokens.size.menuItem), 0)
    implicitHeight: Math.min(_itemsHeight,
                            LithicTokens.size.menuItem * LithicTokens.size.menuVisibleItemsCount)
                    + topPadding + bottomPadding
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

    // moves come from the keys unless a host says otherwise, so they show the keyboard mark
    function moveFocus(step, reason) {
        if (items.length === 0)
            return
        let next = focusedIndex
        for (let attempt = 0; attempt < items.length; ++attempt) {
            next = (next + step + items.length) % items.length
            if (itemEnabled(next)) {
                focusedIndex = next
                _focusAt(next, reason === undefined ? Qt.TabFocusReason : reason)
                return
            }
        }
    }

    function activateFocused() {
        if (itemEnabled(focusedIndex))
            triggered(focusedIndex)
    }

    // a host that opens the menu starts on its current item; only a menu opened
    // from the keys shows the keyboard mark there, as pointer focus never does
    function focusItem(index, fromKeys) {
        if (!itemEnabled(index)) {
            focusedIndex = -1
            moveFocus(1, fromKeys ? Qt.TabFocusReason : Qt.OtherFocusReason)
            return
        }
        focusedIndex = index
        _focusAt(index, fromKeys ? Qt.TabFocusReason : Qt.OtherFocusReason)
    }

    // an item that kept focus from an earlier open is focused already, so its reason is set as well
    function _focusAt(index, reason) {
        list.currentIndex = index
        list.positionViewAtIndex(index, ListView.Contain)
        list.forceLayout()
        const item = list.itemAtIndex(index)
        if (item) {
            item.forceActiveFocus(reason)
            item.focusReason = reason
        }
    }

    function _pageFocus(direction) {
        const step = Math.max(1, Math.floor(list.height / LithicTokens.size.menuItem))
        let next = Math.max(0, Math.min(items.length - 1, focusedIndex + direction * step))
        while (next >= 0 && next < items.length && !itemEnabled(next))
            next += direction
        if (next < 0 || next >= items.length) {
            next = direction > 0 ? items.length - 1 : 0
            while (next >= 0 && next < items.length && !itemEnabled(next))
                next -= direction
        }
        if (itemEnabled(next))
            focusItem(next, true)
    }

    Keys.onDownPressed: moveFocus(1)
    Keys.onUpPressed: moveFocus(-1)
    Keys.onReturnPressed: activateFocused()
    Keys.onEnterPressed: activateFocused()
    Keys.onSpacePressed: activateFocused()
    // Home and End go to the first and last enabled items, as on the web and in Gio
    Keys.onPressed: event => {
        if (event.key === Qt.Key_Home) {
            focusedIndex = -1
            moveFocus(1)
            event.accepted = true
        } else if (event.key === Qt.Key_End) {
            focusedIndex = 0
            moveFocus(-1)
            event.accepted = true
        } else if (event.key === Qt.Key_PageDown || event.key === Qt.Key_PageUp) {
            _pageFocus(event.key === Qt.Key_PageDown ? 1 : -1)
            event.accepted = true
        }
    }
    // a menu that takes focus itself marks its item only when the keys brought it there
    onActiveFocusChanged: {
        if (activeFocus && focusedIndex < 0)
            moveFocus(1, visualFocus ? Qt.TabFocusReason : Qt.OtherFocusReason)
    }

    background: CutRect {
        fill: Lithic.stone4
        strokeColor: Lithic.vein
        strokeWidth: LithicTokens.size.hairline
        cut: LithicTokens.cut.menu
    }

    contentItem: Item {
        layer.enabled: root.visible
        layer.effect: MultiEffect {
            maskEnabled: true
            maskSource: menuMaskTexture
            maskThresholdMin: 0.5
            maskSpreadAtMin: 1
        }

        CutRect {
            id: menuMask
            width: root.width
            height: root.height
            fill: "white"
            cut: LithicTokens.cut.menu
        }

        ShaderEffectSource {
            id: menuMaskTexture
            sourceItem: menuMask
            sourceRect: Qt.rect(root.leftPadding, root.topPadding,
                                root.availableWidth, root.availableHeight)
            hideSource: true
            visible: false
        }

        ListView {
            id: list
            anchors.fill: parent
            clip: true
            model: root.visible ? root.items : []
            currentIndex: root.focusedIndex
            keyNavigationEnabled: false
            boundsBehavior: Flickable.StopAtBounds
            flickableDirection: Flickable.VerticalFlick
            reuseItems: true

            WheelHandler {
                target: null
                onWheel: event => {
                    const delta = event.pixelDelta.y !== 0
                                  ? event.pixelDelta.y
                                  : event.angleDelta.y / 120 * LithicTokens.size.menuItem * 3
                    list.cancelFlick()
                    list.contentY = Math.max(list.originY,
                        Math.min(list.originY + Math.max(0, list.contentHeight - list.height),
                                 list.contentY - delta))
                    event.accepted = true
                }
            }
            delegate: T.MenuItem {
                id: menuItem

                required property var modelData
                required property int index
                readonly property bool divider: !!modelData.divider
                readonly property bool danger: !!modelData.danger
                readonly property string glyphName: modelData.glyph || ""
                readonly property string shortcutText: modelData.shortcut || ""
                readonly property bool isChecked: !!modelData.checked
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
                Accessible.checkable: isChecked
                Accessible.checked: isChecked

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
                        id: itemLabel
                        anchors.left: parent.left
                        anchors.leftMargin: LithicTokens.size.menuInset
                                            + (itemGlyph.visible
                                               ? LithicTokens.size.menuIcon
                                                 + LithicTokens.size.menuIconGap : 0)
                        anchors.right: menuItem.isChecked ? checkGlyph.left : shortcut.left
                        anchors.rightMargin: menuItem.isChecked || menuItem.shortcutText.length > 0
                                             ? LithicTokens.size.menuShortcutGap : 0
                        y: (parent.height - capMetrics.boundingRect.height) / 2
                           - capMetrics.boundingRect.y - baselineOffset
                        text: menuItem.text
                        typeRole: "label"
                        ink: menuItem.enabled
                             ? menuItem.danger ? Lithic.jasper : Lithic.fg
                             : Lithic.fg3
                        elide: Text.ElideRight
                    }

                    Label {
                        id: shortcut
                        visible: !menuItem.isChecked
                        anchors.right: parent.right
                        anchors.rightMargin: LithicTokens.size.menuInset
                        anchors.verticalCenter: parent.verticalCenter
                        text: menuItem.shortcutText
                        typeRole: "monoSm"
                        inkRole: "fg3"
                    }

                    Glyph {
                        id: checkGlyph
                        visible: menuItem.isChecked
                        anchors.right: parent.right
                        anchors.rightMargin: LithicTokens.size.menuInset
                        anchors.verticalCenter: parent.verticalCenter
                        name: "check"
                        size: LithicTokens.size.menuIcon
                        ink: Lithic.fg2
                    }
                }

                TextMetrics {
                    id: capMetrics
                    font: itemLabel.font
                    text: "H"
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

        ScrollRail {
            anchors.right: parent.right
            height: parent.height
            flickable: list
        }
    }
}
