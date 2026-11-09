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
    readonly property bool lithicPopoverMarker: true

    padding: 0
    modal: false
    focus: true
    closePolicy: T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutside
    implicitWidth: liveContent.implicitWidth
    implicitHeight: liveContent.implicitHeight

    function calculatePlacement() {
        if (!anchorItem || !parent || width <= 0 || height <= 0)
            return null
        const point = anchorItem.mapToItem(parent, 0, 0)
        const gap = LithicTokens.size.popoverAnchorGap
        if (placementSide === "right") {
            const right = point.x + anchorItem.width + gap
            const left = point.x - width - gap
            const placedX = right + width <= parent.width ? right
                            : left >= 0 ? left
                            : Math.max(0, Math.min(right, parent.width - width))
            return {
                x: placedX,
                y: Math.max(0, Math.min(point.y + (anchorItem.height - height) / 2,
                                        parent.height - height)),
                above: false
            }
        }
        const below = point.y + anchorItem.height + gap
        const above = point.y - height - gap
        const placedAbove = below + height > parent.height && above >= 0
        return {
            x: Math.max(0, Math.min(point.x, parent.width - width)),
            y: placedAbove ? above : Math.max(0, Math.min(below, parent.height - height)),
            above: placedAbove
        }
    }

    function placeAtAnchor() {
        const placement = calculatePlacement()
        if (!placement)
            return
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
    }
    onOpened: placeAtAnchor()
    onClosed: {
        if (opener && opener.enabled)
            opener.forceActiveFocus()
    }
    onWidthChanged: {
        if (visible || previewEmbedded)
            placeAtAnchor()
    }
    onHeightChanged: {
        if (visible || previewEmbedded)
            placeAtAnchor()
    }
    onPreviewEmbeddedChanged: {
        if (previewEmbedded)
            Qt.callLater(placeAtAnchor)
    }
    onPlacementSideChanged: placeAtAnchor()

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
}
