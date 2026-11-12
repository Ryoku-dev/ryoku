import QtQuick

Item {
    id: root

    property Flickable flickable: null
    property string previewState: ""

    readonly property bool overflowing: flickable !== null && flickable.contentHeight > flickable.height
    readonly property real _range: flickable ? Math.max(0, flickable.contentHeight - flickable.height) : 0
    readonly property real _minimum: flickable ? flickable.originY : 0
    readonly property real _thumbHeight: overflowing
                                         ? Math.max(LithicTokens.size.scrollThumbMin,
                                                    height * flickable.height / flickable.contentHeight)
                                         : 0
    readonly property real _travel: Math.max(0, height - _thumbHeight)
    readonly property real _thumbY: overflowing && _range > 0
                                    ? Math.max(0, Math.min(_travel,
                                        (flickable.contentY - _minimum) * _travel / _range))
                                    : 0
    readonly property bool _thumbHovered: area.containsMouse && area.mouseY >= _thumbY
                                          && area.mouseY <= _thumbY + _thumbHeight
    readonly property bool _active: area.dragging || _thumbHovered
                                    || previewState === "hover" || previewState === "pressed"
    property real _grabOffset: 0

    implicitWidth: LithicTokens.size.scrollThumb
    visible: overflowing

    Accessible.role: Accessible.ScrollBar
    Accessible.name: Lithic.tr("Scroll")

    function scrollTo(contentY) {
        if (!flickable)
            return;
        flickable.contentY = Math.max(_minimum, Math.min(_minimum + _range, contentY));
    }

    function page(direction) {
        if (!flickable)
            return;
        pageAnimation.stop();
        pageAnimation.from = flickable.contentY;
        pageAnimation.to = Math.max(_minimum, Math.min(_minimum + _range,
                                    flickable.contentY + direction * flickable.height));
        pageAnimation.start();
    }

    NumberAnimation {
        id: pageAnimation
        target: root.flickable
        property: "contentY"
        duration: Lithic.duration(LithicTokens.motion.wheel)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
    }

    CutRect {
        anchors.right: parent.right
        y: root._thumbY
        width: LithicTokens.size.scrollThumb
        height: root._thumbHeight
        fill: root._active ? Lithic.fg3 : Lithic.edge
        cutTopLeft: LithicTokens.cut.token
        cutBottomLeft: LithicTokens.cut.token
        cutTopRight: 0
        cutBottomRight: 0

        ChiselColorBehavior on fill {
            duration: root._active ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
        }
    }

    // the thumb is 3 wide; the pointer target reaches s2 into the pane
    MouseArea {
        id: area

        property bool dragging: false

        anchors.fill: parent
        anchors.leftMargin: -LithicTokens.space.s2
        hoverEnabled: true
        acceptedButtons: Qt.LeftButton

        onPressed: function(mouse) {
            if (mouse.y >= root._thumbY && mouse.y <= root._thumbY + root._thumbHeight) {
                pageAnimation.stop();
                root._grabOffset = mouse.y - root._thumbY;
                dragging = true;
            } else {
                root.page(mouse.y < root._thumbY ? -1 : 1);
            }
        }
        onPositionChanged: function(mouse) {
            if (!dragging || root._travel <= 0)
                return;
            const position = Math.max(0, Math.min(root._travel, mouse.y - root._grabOffset));
            root.scrollTo(root._minimum + position * root._range / root._travel);
        }
        onReleased: dragging = false
        onCanceled: dragging = false
    }
}
