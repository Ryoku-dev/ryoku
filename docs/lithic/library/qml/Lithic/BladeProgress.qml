import QtQuick

Item {
    id: root

    property real value: 0
    property bool active: false
    property bool indeterminate: false
    property real _sweepPosition: -segment.width

    implicitWidth: LithicTokens.size.listMedium
    implicitHeight: LithicTokens.size.bladeTrack
    clip: true

    CutRect {
        anchors.fill: parent
        fill: Lithic.field
    }

    Blade {
        visible: !root.indeterminate
        anchors.fill: parent
        fraction: Math.max(0, Math.min(1, root.value))
        fill: Lithic.lapis
        track: "transparent"
    }

    Blade {
        id: segment
        visible: root.indeterminate
        x: sweep.running
            ? root._sweepPosition
            : (root.width - width) / 2
        width: root.width * LithicTokens.size.bladeIndeterminateFraction
        height: root.height
        explicitWidth: width
        fill: Lithic.lapis
        track: "transparent"
    }

    CutRect {
        visible: root.active && !root.indeterminate && root.value > 0
        x: Math.max(0, Math.min(root.width - width,
                               root.value * root.width - width / 2))
        width: LithicTokens.size.bladeMarker
        height: parent.height
        fill: Lithic.gold
    }

    NumberAnimation {
        id: sweep
        target: root
        property: "_sweepPosition"
        from: -segment.width
        to: root.width
        duration: Lithic.duration(LithicTokens.motion.sweep)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
        loops: Animation.Infinite
        running: root.active && root.indeterminate
            && !Lithic.reduceMotion && root.visible
    }
}
