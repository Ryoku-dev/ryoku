import QtQuick
import Lithic

// a sheet's running head, as on ryoku.dev: where the sheet belongs at the left, a hairline
// across, and the sheet's mark at the right in mono. Either end may be empty
Item {
    id: head

    property string label: ""
    property string mark: ""

    implicitHeight: Math.max(labelItem.implicitHeight, markItem.implicitHeight)

    // the hairline draws in from the left, ryoku.dev's "blade"; reduced motion leaves it drawn
    function play() {
        if (!Lithic.reduceMotion)
            blade.restart();
    }

    Label {
        id: labelItem
        visible: head.label.length > 0
        anchors.verticalCenter: parent.verticalCenter
        text: head.label
        typeRole: "caption"
        inkRole: "fg2"
    }
    Rectangle {
        id: rule
        readonly property real from: labelItem.visible ? labelItem.width + LithicTokens.space.s3 : 0
        readonly property real to: markItem.visible ? markItem.x - LithicTokens.space.s3 : head.width
        anchors.verticalCenter: parent.verticalCenter
        x: rule.from
        width: Math.max(0, rule.to - rule.from)
        height: LithicTokens.size.hairline
        color: Lithic.vein
        transform: Scale {
            id: draw
            xScale: 1
        }
    }
    Label {
        id: markItem
        visible: head.mark.length > 0
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        text: head.mark
        typeRole: "monoSm"
        inkRole: "fg3"
    }

    NumberAnimation {
        id: blade
        target: draw
        property: "xScale"
        from: 0
        to: 1
        duration: LithicTokens.motion.dialogIn
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
    }
}
