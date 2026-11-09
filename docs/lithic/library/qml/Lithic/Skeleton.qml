import QtQuick

Item {
    id: root

    property bool active: false
    property bool compact: false
    property real _sweepProgress: 0

    implicitWidth: LithicTokens.size.listMedium
    implicitHeight: compact
        ? LithicTokens.size.tabletHeightCompact
        : LithicTokens.size.tabletHeight

    component Bone: Item {
        clip: true
        height: LithicTokens.size.skeletonBarHeight

        CutRect {
            anchors.fill: parent
            fill: Lithic.stone3
            cut: LithicTokens.cut.token
        }

        Hatch {
            visible: root.active && !Lithic.reduceMotion
            x: -width + root._sweepProgress * (parent.width + width)
            width: parent.width * LithicTokens.size.bladeIndeterminateFraction
            height: parent.height
        }
    }

    CutRect {
        anchors.fill: parent
        fill: Lithic.stone2
    }

    Column {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: LithicTokens.size.skeletonInset
        anchors.rightMargin: LithicTokens.size.skeletonInset
        anchors.verticalCenter: parent.verticalCenter
        spacing: LithicTokens.size.skeletonBarGap

        Bone {
            width: parent.width * LithicTokens.size.skeletonBarLongFraction
        }
        Bone {
            width: parent.width * LithicTokens.size.skeletonBarShortFraction
        }
    }

    NumberAnimation on _sweepProgress {
        from: 0
        to: 1
        duration: Lithic.duration(LithicTokens.motion.sweep)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
        loops: Animation.Infinite
        running: root.active && !Lithic.reduceMotion && root.visible
    }
}
