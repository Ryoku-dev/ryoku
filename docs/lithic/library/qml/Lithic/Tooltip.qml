import QtQuick
import QtQuick.Templates as T

T.ToolTip {
    id: root

    property Item anchorItem: null
    property bool previewOpen: false
    property string previewState: ""
    property bool openAbove: false
    property point placementPosition: Qt.point(0, 0)
    property real slideOffset: 0

    component TooltipPlane: CutRect {
        fill: Lithic.stone4
        strokeColor: Lithic.vein
        strokeWidth: LithicTokens.size.hairline
        cut: LithicTokens.cut.tooltip
    }

    parent: anchorItem ? anchorItem.parent : null
    visible: !previewOpen && hover.hovered
    delay: Lithic.duration(LithicTokens.motion.tooltipDelay)
    timeout: -1
    padding: 0
    leftPadding: LithicTokens.size.tooltipPaddingHorizontal
    rightPadding: LithicTokens.size.tooltipPaddingHorizontal
    topPadding: LithicTokens.size.tooltipPaddingVertical
    bottomPadding: LithicTokens.size.tooltipPaddingVertical
    implicitWidth: Math.min(LithicTokens.size.tooltipWidthMax,
                            contentItem.implicitWidth + leftPadding + rightPadding)
    implicitHeight: contentItem.implicitHeight + topPadding + bottomPadding
    closePolicy: T.Popup.NoAutoClose
    enabled: false

    function calculatePlacement() {
        if (!anchorItem || !parent || width <= 0 || height <= 0)
            return null
        const point = anchorItem.mapToItem(parent, 0, 0)
        const gap = LithicTokens.size.tooltipAnchorGap
        const below = point.y + anchorItem.height + gap
        const above = point.y - height - gap
        const placedAbove = below + height > parent.height && above >= 0
        return {
            x: Math.max(0, Math.min(point.x + (anchorItem.width - width) / 2,
                                    parent.width - width)),
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

    onAboutToShow: placeAtAnchor()
    onOpened: placeAtAnchor()
    onWidthChanged: {
        if (visible || previewOpen)
            placeAtAnchor()
    }
    onHeightChanged: {
        if (visible || previewOpen)
            placeAtAnchor()
    }
    onPreviewOpenChanged: {
        if (previewOpen)
            Qt.callLater(placeAtAnchor)
    }

    background: TooltipPlane {
        transform: Translate { y: root.slideOffset }
    }

    contentItem: Label {
        text: root.text
        typeRole: "caption"
        inkRole: "fg"
        wrapMode: Text.Wrap
        transform: Translate { y: root.slideOffset }
    }

    TooltipPlane {
        id: previewPlane
        parent: root.parent
        visible: root.previewOpen
        x: root.placementPosition.x
        y: root.placementPosition.y
        width: root.width
        height: root.height
        z: 100

        Label {
            x: root.leftPadding
            y: root.topPadding
            width: parent.width - root.leftPadding - root.rightPadding
            height: parent.height - root.topPadding - root.bottomPadding
            text: root.text
            typeRole: "caption"
            inkRole: "fg"
            wrapMode: Text.Wrap
        }
    }

    Component.onCompleted: {
        if (previewOpen)
            Qt.callLater(placeAtAnchor)
    }

    HoverHandler {
        id: hover
        parent: root.anchorItem
        enabled: !!root.anchorItem && !root.previewOpen
    }

    enter: Transition {
        ParallelAnimation {
            NumberAnimation {
                target: root
                property: "opacity"
                from: 0
                to: 1
                duration: Lithic.duration(LithicTokens.motion.tooltipIn)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Lithic.curve
            }
            NumberAnimation {
                target: root
                property: "slideOffset"
                from: root.openAbove ? -LithicTokens.size.tooltipTravel
                                     : LithicTokens.size.tooltipTravel
                to: 0
                duration: Lithic.duration(LithicTokens.motion.tooltipIn)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Lithic.curve
            }
        }
    }

    Connections {
        target: root.anchorItem
        ignoreUnknownSignals: true
        function onXChanged() { if (root.visible || root.previewOpen) root.placeAtAnchor() }
        function onYChanged() { if (root.visible || root.previewOpen) root.placeAtAnchor() }
        function onWidthChanged() { if (root.visible || root.previewOpen) root.placeAtAnchor() }
        function onHeightChanged() { if (root.visible || root.previewOpen) root.placeAtAnchor() }
    }

    Connections {
        target: root.parent
        ignoreUnknownSignals: true
        function onWidthChanged() { if (root.visible || root.previewOpen) root.placeAtAnchor() }
        function onHeightChanged() { if (root.visible || root.previewOpen) root.placeAtAnchor() }
    }
}
