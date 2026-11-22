pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../ask/look"

Item {
    id: root

    required property real s
    property bool current: false
    property int direction: 1
    property bool keep: false
    property Component sourceComponent: null
    readonly property Item view: loader.item as Item
    readonly property real viewHeight: loader.item ? loader.item.implicitHeight : 0
    readonly property real side: root.current ? root.direction : -root.direction

    property real t: root.current ? 1 : 0
    Behavior on t {
        NumberAnimation {
            duration: QsLook.dur(root.current || Look.viewWipe ? Look.viewMs : Math.round(Look.viewMs * 0.55))
            easing.type: Easing.BezierSpline
            easing.bezierCurve: root.current ? Look.arrive : Look.leave
        }
    }

    readonly property bool wiping: Look.viewWipe && root.current && root.t < 0.999
    readonly property bool fromRight: root.direction < 0

    visible: root.t > 0.001 || root.current
    z: root.current ? 2 : 1
    clip: root.wiping
    x: root.wiping && root.fromRight && root.parent ? root.parent.width - width : 0
    width: !root.parent ? 0 : root.wiping ? Math.ceil(root.parent.width * Math.min(1, root.t)) : root.parent.width
    height: root.viewHeight
    opacity: Look.viewWipe ? 1 : Math.max(0, Math.min(1, root.current ? root.t * 1.4 : root.t * 2 - 1))

    // the wipe covers the view it replaces
    Rectangle {
        visible: root.wiping
        x: -root.x
        width: root.parent ? root.parent.width : 0
        height: parent.height
        color: QsLook.frame
    }

    Loader {
        id: loader
        width: root.parent ? root.parent.width : 0
        x: Look.viewTravel * root.s * (1 - Math.min(1, root.t)) * root.side - root.x
        scale: Look.viewScale + (1 - Look.viewScale) * Math.min(1, root.t)
        transformOrigin: Item.Top
        active: root.keep || root.current || root.t > 0.001
        sourceComponent: root.sourceComponent
    }

    Rectangle {
        visible: root.wiping
        x: root.fromRight ? 0 : root.width - width
        width: Math.max(1, Math.round(root.s))
        height: parent.height
        color: Lithic.sel
    }
}
