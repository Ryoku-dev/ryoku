import QtQuick
import QtQuick.Templates as T

T.Popup {
    id: root

    property Item anchorItem: null
    property Item opener: null
    property string previewState: ""
    property bool previewEmbedded: false
    property string placementSide: "vertical"
    property point placementPosition: Qt.point(0, 0)
    property bool openAbove: false
    property real slideOffset: 0
    // hands focus to a menu inside on open; a host that focuses the menu itself
    // (a Picker on its current choice) turns it off
    property bool menuFocus: true
    // a host whose opener can take a key without showing the mark (a Picker focused by a
    // click, then opened with Down) says the keys opened it, so focus comes back marked
    property bool keysOpened: false
    property bool _markOpener: false
    readonly property bool lithicPopoverMarker: true

    padding: 0
    modal: false
    focus: true
    closePolicy: T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside
    implicitWidth: liveContent.implicitWidth
    implicitHeight: liveContent.implicitHeight
    property var _placement: null
    width: _placement ? _placement.width : implicitWidth
    height: _placement ? _placement.height : implicitHeight

    function calculatePlacement() {
        if (!anchorItem || !parent)
            return null
        const window = liveContent.Window.window
        const viewport = !previewEmbedded && window ? window.contentItem : parent
        const origin = viewport.mapToItem(parent, 0, 0)
        const point = anchorItem.mapToItem(parent, 0, 0)
        const gap = LithicTokens.size.popoverAnchorGap
        const margin = LithicTokens.size.popoverAnchorGap
        const leftEdge = origin.x + margin
        const rightEdge = origin.x + viewport.width - margin
        const topEdge = origin.y + margin
        const bottomEdge = origin.y + viewport.height - margin
        const placedWidth = Math.min(implicitWidth, Math.max(0, rightEdge - leftEdge))
        const clampX = x => Math.max(leftEdge, Math.min(x, rightEdge - placedWidth))
        if (placementSide === "right") {
            const right = point.x + anchorItem.width + gap
            const rightRoom = Math.max(0, rightEdge - right)
            const leftRoom = Math.max(0, point.x - gap - leftEdge)
            const placedLeft = implicitWidth > rightRoom
                               && (implicitWidth <= leftRoom || leftRoom > rightRoom)
            const sideWidth = Math.min(implicitWidth, placedLeft ? leftRoom : rightRoom)
            const placedX = placedLeft ? point.x - gap - sideWidth : right
            const placedHeight = Math.min(implicitHeight, Math.max(0, bottomEdge - topEdge))
            return {
                x: placedX,
                y: Math.max(topEdge, Math.min(point.y + (anchorItem.height - placedHeight) / 2,
                                             bottomEdge - placedHeight)),
                width: sideWidth,
                height: placedHeight,
                above: false
            }
        }
        const below = point.y + anchorItem.height + gap
        const belowRoom = Math.max(0, bottomEdge - below)
        const aboveRoom = Math.max(0, point.y - gap - topEdge)
        const placedAbove = implicitHeight > belowRoom
                            && (implicitHeight <= aboveRoom || aboveRoom > belowRoom)
        const placedHeight = Math.min(implicitHeight, placedAbove ? aboveRoom : belowRoom)
        return {
            x: clampX(point.x),
            y: placedAbove ? point.y - gap - placedHeight : below,
            width: placedWidth,
            height: placedHeight,
            above: placedAbove
        }
    }

    function placeAtAnchor() {
        const placement = calculatePlacement()
        if (!placement)
            return
        _placement = placement
        openAbove = placement.above
        placementPosition = Qt.point(placement.x, placement.y)
        x = placement.x
        y = placement.y
    }

    function closeOtherPopovers() {
        if (!parent)
            return
        const siblings = parent.data
        for (let index = 0; index < siblings.length; ++index) {
            const other = siblings[index]
            if (other !== root && "lithicPopoverMarker" in other
                    && other.lithicPopoverMarker && other.opened)
                other.close()
        }
    }

    onAboutToShow: {
        closeOtherPopovers()
        placeAtAnchor()
        _markOpener = keysOpened || (opener !== null && opener.visualFocus === true)
    }
    onOpened: {
        placeAtAnchor()
        if (menuFocus)
            _focusMenu()
    }

    // A menu inside the popover takes focus on its first enabled item, so the
    // arrows and Return reach it; focus already elsewhere in the content stays where
    // it is. Only a popover the keys opened shows the keyboard ring there; the opener
    // has lost focus to the popup by now, so the mark comes from before it opened.
    function _focusMenu() {
        const menu = _findMenu(liveContent)
        const window = liveContent.Window.window
        for (let item = window ? window.activeFocusItem : null; item; item = item.parent) {
            if (item === menu)
                break
            if (item === liveContent)
                return
        }
        if (menu) {
            const current = Array.isArray(menu.items)
                            ? menu.items.findIndex(entry => entry.checked && !entry.disabled && !entry.divider) : -1
            menu.focusItem(current, _markOpener)
        }
    }

    function _findMenu(item) {
        for (let index = 0; index < item.children.length; ++index) {
            const child = item.children[index]
            if (typeof child.focusItem === "function" && typeof child.moveFocus === "function")
                return child
            const nested = _findMenu(child)
            if (nested)
                return nested
        }
        return null
    }
    // focus goes back to the opener, marked when the keys opened the popover; the popup has
    // usually restored it already with an unmarked reason, so the reason is set as well
    onClosed: {
        if (!opener || !opener.enabled)
            return
        const reason = _markOpener ? Qt.TabFocusReason : Qt.OtherFocusReason
        opener.forceActiveFocus(reason)
        if (opener.focusReason !== undefined)
            opener.focusReason = reason
    }
    onWidthChanged: {
        if (visible || previewEmbedded)
            Qt.callLater(placeAtAnchor)
    }
    onHeightChanged: {
        if (visible || previewEmbedded)
            Qt.callLater(placeAtAnchor)
    }
    onPreviewEmbeddedChanged: {
        if (previewEmbedded)
            Qt.callLater(placeAtAnchor)
    }
    onPlacementSideChanged: placeAtAnchor()
    onImplicitWidthChanged: {
        if (visible || previewEmbedded)
            placeAtAnchor()
    }
    onImplicitHeightChanged: {
        if (visible || previewEmbedded)
            placeAtAnchor()
    }

    background: Item {
        transform: Translate { y: root.slideOffset }
    }

    contentItem: Item {
        id: liveContent
        implicitWidth: children.length > 0 ? children[0].implicitWidth : 0
        implicitHeight: children.length > 0 ? children[0].implicitHeight : 0
        enabled: root.previewEmbedded || root.opened
        transform: Translate { y: root.previewEmbedded ? 0 : root.slideOffset }
    }

    Binding {
        target: liveContent.children.length > 0 ? liveContent.children[0] : null
        property: "height"
        value: liveContent.height
        when: root.visible || root.previewEmbedded
        restoreMode: Binding.RestoreBindingOrValue
    }

    Binding {
        target: liveContent.children.length > 0 ? liveContent.children[0] : null
        property: "width"
        value: liveContent.width
        when: root.visible || root.previewEmbedded
        restoreMode: Binding.RestoreBindingOrValue
    }

    Item {
        id: previewHost
        parent: root.parent
        visible: root.previewEmbedded
        x: root.placementPosition.x
        y: root.placementPosition.y
        width: root.width
        height: root.height
        z: 100
    }

    StateGroup {
        states: State {
        name: "preview"
        when: root.previewEmbedded
        ParentChange {
            target: liveContent
            parent: previewHost
            x: 0
            y: 0
            width: previewHost.width
            height: previewHost.height
        }
        PropertyChanges {
            target: liveContent
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
                duration: Lithic.duration(LithicTokens.motion.menuIn)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Lithic.curve
            }
            NumberAnimation {
                target: root
                property: "slideOffset"
                from: root.openAbove ? -LithicTokens.size.popoverEnterTravel
                                     : LithicTokens.size.popoverEnterTravel
                to: 0
                duration: Lithic.duration(LithicTokens.motion.menuIn)
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
                duration: Lithic.duration(LithicTokens.motion.menuOut)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Lithic.curve
            }
            NumberAnimation {
                target: root
                property: "slideOffset"
                from: 0
                to: root.openAbove ? -LithicTokens.size.popoverExitTravel
                                   : LithicTokens.size.popoverExitTravel
                duration: Lithic.duration(LithicTokens.motion.menuOut)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Lithic.curve
            }
        }
    }

    Component.onCompleted: {
        if (previewEmbedded)
            Qt.callLater(placeAtAnchor)
    }

    Connections {
        target: root.anchorItem
        ignoreUnknownSignals: true
        function onXChanged() { if (root.visible || root.previewEmbedded) root.placeAtAnchor() }
        function onYChanged() { if (root.visible || root.previewEmbedded) root.placeAtAnchor() }
        function onWidthChanged() { if (root.visible || root.previewEmbedded) root.placeAtAnchor() }
        function onHeightChanged() { if (root.visible || root.previewEmbedded) root.placeAtAnchor() }
    }

    Connections {
        target: root.parent
        ignoreUnknownSignals: true
        function onWidthChanged() { if (root.visible || root.previewEmbedded) root.placeAtAnchor() }
        function onHeightChanged() { if (root.visible || root.previewEmbedded) root.placeAtAnchor() }
    }

    Connections {
        target: liveContent.Window.window
        ignoreUnknownSignals: true
        function onWidthChanged() { if (root.visible || root.previewEmbedded) root.placeAtAnchor() }
        function onHeightChanged() { if (root.visible || root.previewEmbedded) root.placeAtAnchor() }
    }
}
